#!/usr/bin/env python3
"""frame_splitter.py — Split raw MJPEG byte stream into individual JPEG frames.

Reads raw MJPEG from stdin (ffmpeg stdout), scans for JPEG SOI/EOI markers,
and writes numbered JPEG files to /tmp/bifrost/frames/ for the HTTP server to serve.

JPEG markers:
  SOI (Start Of Image): 0xFF 0xD8
  EOI (End Of Image):   0xFF 0xD9
"""

import os
import sys
import signal

FRAME_DIR = "/tmp/bifrost/frames"
COUNTER_FILE = os.path.join(FRAME_DIR, "COUNTER")
MAX_FRAMES = 10  # Keep only last N frames

# JPEG markers
SOI = b'\xff\xd8'
EOI = b'\xff\xd9'

def ensure_dir():
    os.makedirs(FRAME_DIR, exist_ok=True)
    with open(COUNTER_FILE, 'w') as f:
        f.write("0")

def cleanup_old_frames(current_num):
    """Remove frames older than the last MAX_FRAMES."""
    keep_min = max(0, current_num - MAX_FRAMES)
    try:
        for fname in os.listdir(FRAME_DIR):
            if fname.endswith('.jpg'):
                fnum = int(fname.replace('.jpg', ''))
                if fnum < keep_min:
                    os.remove(os.path.join(FRAME_DIR, fname))
    except (ValueError, OSError):
        pass

def write_frame(data, counter):
    """Write a JPEG frame and update the counter atomically."""
    fname = os.path.join(FRAME_DIR, f"{counter:08d}.jpg")
    tmpname = fname + ".tmp"
    with open(tmpname, 'wb') as f:
        f.write(data)
    os.rename(tmpname, fname)
    with open(COUNTER_FILE, 'w') as f:
        f.write(str(counter))

def main():
    ensure_dir()
    signal.signal(signal.SIGTERM, lambda *_: sys.exit(0))

    stdin = sys.stdin.buffer
    counter = 0
    buf = b''
    in_frame = False

    while True:
        chunk = stdin.read(65536)
        if not chunk:
            break

        buf += chunk

        while True:
            if not in_frame:
                # Look for SOI marker
                idx = buf.find(SOI)
                if idx == -1:
                    # Keep last byte (could be start of SOI split across reads)
                    buf = buf[-1:] if len(buf) > 1 else buf
                    break
                # Discard everything before SOI
                if idx > 0:
                    buf = buf[idx:]
                in_frame = True

            if in_frame:
                # Look for EOI marker after SOI
                idx = buf.find(EOI, 2)  # Start searching after SOI
                if idx == -1:
                    break
                # Extract complete frame (SOI through EOI inclusive)
                frame = buf[:idx + 2]
                buf = buf[idx + 2:]
                in_frame = False
                counter += 1
                write_frame(frame, counter)
                cleanup_old_frames(counter)

    # Handle any remaining frame in buffer
    if in_frame and len(buf) > 2:
        counter += 1
        write_frame(buf, counter)

if __name__ == '__main__':
    main()

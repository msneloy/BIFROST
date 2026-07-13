#!/usr/bin/env python3
"""Zero-interaction screen capture: Mutter ScreenCast -> PipeWire -> GStreamer.

Outputs:
  - MJPEG to stdout (for HTTP viewer)
  - VP8 RTP to UDP 127.0.0.1:5004 (for WebRTC video)
  - Opus RTP to UDP 127.0.0.1:5005 (for WebRTC audio)
"""
import gi
gi.require_version('Gio', '2.0')
from gi.repository import Gio, GLib
import sys, subprocess, threading

log = lambda m: print(m, file=sys.stderr, flush=True)

bus = Gio.bus_get_sync(Gio.BusType.SESSION, None)
MUTTER = 'org.gnome.Mutter.ScreenCast'
loop = GLib.MainLoop()
nid = [None]

def on_sig(c, s, p, i, sig, params):
    if sig == 'PipeWireStreamAdded':
        nid[0] = params.get_child_value(0).get_uint32()
        loop.quit()

bus.signal_subscribe(None, None, None, None, None, 0, on_sig)

r = bus.call_sync(MUTTER, '/org/gnome/Mutter/ScreenCast', MUTTER, 'CreateSession',
    GLib.Variant('(a{sv})', [{}]), GLib.VariantType('(o)'), Gio.DBusCallFlags.NONE, -1, None)
sess = r.get_child_value(0).get_string()
log(f'Session: {sess}')

r = bus.call_sync(MUTTER, sess, f'{MUTTER}.Session', 'RecordMonitor',
    GLib.Variant('(sa{sv})', ['', {}]), GLib.VariantType('(o)'), Gio.DBusCallFlags.NONE, -1, None)

bus.call_sync(MUTTER, sess, f'{MUTTER}.Session', 'Start',
    GLib.Variant('()', ()), None, Gio.DBusCallFlags.NONE, -1, None)

GLib.timeout_add(10000, loop.quit)
loop.run()

if nid[0] is None:
    log('ERROR: No PipeWire node')
    sys.exit(1)

nid_val = nid[0]
log(f'PipeWire node: {nid_val} — streaming')

def detect_monitor_source():
    """Find PulseAudio monitor source for system audio capture."""
    try:
        out = subprocess.check_output(['pactl', 'list', 'short', 'sources'],
                                       stderr=subprocess.DEVNULL, timeout=3)
        for line in out.decode().splitlines():
            if 'monitor' in line.lower():
                parts = line.split()
                if len(parts) >= 2:
                    return parts[1]
    except Exception:
        pass
    return None

def drain_stderr(proc, label):
    while True:
        line = proc.stderr.readline()
        if not line:
            break
        # Only log errors, not verbose GStreamer output
        text = line.decode(errors='replace').strip()
        if 'error' in text.lower() or 'ERROR' in text:
            log(f'{label} {text}')

# --- Video pipeline: tee to both MJPEG stdout and VP8 RTP ---
video_pipeline = (
    f'pipewiresrc path={nid_val} '
    f'! capsfilter caps=video/x-raw,format=BGRx,width=1920,height=1080 '
    f'! videorate max-rate=15 '
    f'! videoconvert '
    f'! tee name=t '
    f't. ! queue ! jpegenc quality=40 ! filesink location=/dev/stdout '
    f't. ! queue ! vp8enc threads=4 deadline=1 ! rtpvp8pay ! '
    f'udpsink host=127.0.0.1 port=5004 sync=false'
)

log('Video: MJPEG stdout + VP8 RTP → :5004')
video_proc = subprocess.Popen(
    ['gst-launch-1.0', '-v'] + video_pipeline.split(),
    stdout=sys.stdout.buffer, stderr=subprocess.PIPE)
threading.Thread(target=drain_stderr, args=(video_proc, '[video]'), daemon=True).start()

# --- Audio pipeline: PulseAudio monitor → Opus RTP ---
audio_source = detect_monitor_source()
if audio_source:
    log(f'Audio: PulseAudio monitor ({audio_source}) → Opus RTP → :5005')
    audio_pipeline = (
        f'pulsesrc device={audio_source} '
        f'! audioconvert '
        f'! opusenc bitrate=64000 '
        f'! rtpopuspay '
        f'! udpsink host=127.0.0.1 port=5005 sync=false'
    )
    audio_proc = subprocess.Popen(
        ['gst-launch-1.0', '-v'] + audio_pipeline.split(),
        stdout=subprocess.DEVNULL, stderr=subprocess.PIPE)
    threading.Thread(target=drain_stderr, args=(audio_proc, '[audio]'), daemon=True).start()
else:
    log('Audio: no PulseAudio monitor source found — audio disabled')
    audio_proc = None

# Wait for video process (primary)
video_proc.wait()
if audio_proc:
    audio_proc.terminate()

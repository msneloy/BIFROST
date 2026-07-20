#!/usr/bin/env python3
"""Zero-interaction screen capture: Mutter ScreenCast -> PipeWire -> GStreamer.

Single unified pipeline — video and audio share the same GStreamer clock
for perfect RTP timestamp synchronization.

Outputs:
  - MJPEG to stdout (for HTTP viewer)
  - VP8 RTP to UDP 127.0.0.1:5004 (for WebRTC video) [unless --no-webrtc]
  - Opus RTP to UDP 127.0.0.1:5005 (for WebRTC audio) [unless --no-webrtc]
"""
import gi
gi.require_version('Gio', '2.0')
from gi.repository import Gio, GLib
import sys, subprocess, threading, argparse

log = lambda m: print(m, file=sys.stderr, flush=True)

parser = argparse.ArgumentParser()
parser.add_argument('--no-webrtc', action='store_true', help='Disable WebRTC RTP outputs')
args = parser.parse_args()
no_webrtc = args.no_webrtc

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
        text = line.decode(errors='replace').strip()
        if 'error' in text.lower() or 'ERROR' in text:
            log(f'{label} {text}')

# --- Detect audio source ---
audio_source = detect_monitor_source()

# --- Single unified pipeline: video + audio share one GStreamer clock ---
# This ensures RTP timestamps are synchronized for perfect A/V sync in WebRTC.
pipeline_str = (
    f'pipewiresrc path={nid_val} '
    f'! capsfilter caps=video/x-raw,format=BGRx,width=1920,height=1080 '
    f'! videorate max-rate=30 '
    f'! videoconvert '
)

if no_webrtc:
    # MJPEG only — no tee, no VP8, maximum CPU for jpegenc
    pipeline_str += (
        f'! jpegenc quality=40 ! filesink location=/dev/stdout'
    )
else:
    # Tee to MJPEG + VP8 branches
    pipeline_str += (
        f'! tee name=vt '
        f'vt. ! queue max-size-buffers=1 leaky=downstream ! jpegenc quality=40 ! filesink location=/dev/stdout '
        f'vt. ! queue max-size-buffers=1 leaky=downstream ! vp8enc threads=2 deadline=1 cpu-used=8 ! rtpvp8pay ! '
        f'udpsink host=127.0.0.1 port=5004 sync=false'
    )

outputs = ['MJPEG stdout']
if not no_webrtc:
    outputs.append('VP8 RTP :5004')

if audio_source and not no_webrtc:
    pipeline_str += (
        f' pulsesrc device={audio_source} '
        f'! audioconvert '
        f'! opusenc bitrate=64000 audio-type=restricted-lowdelay '
        f'! rtpopuspay ! '
        f'udpsink host=127.0.0.1 port=5005 sync=false'
    )
    outputs.append('Opus RTP :5005')

log(f'Pipeline: {" + ".join(outputs)}')
if audio_source:
    log(f'Audio source: {audio_source}')

proc = subprocess.Popen(
    ['gst-launch-1.0', '-v'] + pipeline_str.split(),
    stdout=sys.stdout.buffer, stderr=subprocess.PIPE)
threading.Thread(target=drain_stderr, args=(proc, '[gst]'), daemon=True).start()

proc.wait()

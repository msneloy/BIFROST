#!/usr/bin/env python3
"""Zero-interaction screen capture: Mutter ScreenCast -> PipeWire -> GStreamer MJPEG stdout."""
import gi
gi.require_version('Gio', '2.0')
from gi.repository import Gio, GLib
import sys, subprocess

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

proc = subprocess.Popen(
    ['gst-launch-1.0', '-v',
     'pipewiresrc', f'path={nid_val}',
     '!', 'capsfilter', 'caps=video/x-raw,format=BGRx,width=1920,height=1080',
     '!', 'videoconvert',
     '!', 'jpegenc', 'quality=40',
     '!', 'filesink', 'location=/dev/stdout'],
    stdout=sys.stdout.buffer, stderr=subprocess.PIPE)

# Log GStreamer errors briefly, then drain stderr
import threading, time
def log_stderr():
    while True:
        line = proc.stderr.readline()
        if not line:
            break
t = threading.Thread(target=log_stderr, daemon=True)
t.start()

proc.wait()

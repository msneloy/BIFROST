# BIFROST — Data Flow

## Primary Video Pipeline

```
┌──────────────┐     ┌─────────────┐     ┌──────────────┐     ┌──────────────┐
│   Display    │────▶│   ffmpeg    │────▶│  Frame Parse │────▶│ Broadcaster  │
│  (X11/Way)   │     │   stdout    │     │  (SOI/EOI)   │     │  .Publish()  │
└──────────────┘     └─────────────┘     └──────────────┘     └──────┬───────┘
                                                                      │
                                              ┌───────────────────────┼───────────────┐
                                              │                       │               │
                                              ▼                       ▼               ▼
                                        ┌──────────┐          ┌──────────┐     ┌──────────┐
                                        │ Client 1 │          │ Client 2 │     │ Client N │
                                        │ channel  │          │ channel  │     │ channel  │
                                        └────┬─────┘          └────┬─────┘     └────┬─────┘
                                             │                      │                │
                                             ▼                      ▼                ▼
                                       /stream MJPEG         /stream MJPEG    /stream MJPEG
                                       multipart response    multipart resp   multipart resp
```

### Frame Lifecycle

1. **Capture:** ffmpeg grabs display, outputs JPEG stream to stdout
2. **Read:** `capture.go` reads 64KB chunks from stdout into a buffer
3. **Parse:** Scans for `0xFF 0xD8` (SOI) and `0xFF 0xD9` (EOI) markers
4. **Extract:** Complete JPEG frame extracted from buffer
5. **Publish:** `Broadcaster.Publish(frame)` fans out to all subscriber channels
6. **Deliver:** Each `/stream` handler reads from its channel and writes multipart MJPEG
7. **Display:** Browser's `<img>` element auto-renders each frame

### Bandwidth Tracking

```
Broadcaster.Publish(frame)
  │
  ▼
For each subscriber channel:
  │
  ├─ fmt.Fprint(w, header)     ──▶ tr.AddBytes(ip, n1)
  ├─ w.Write(frame)             ──▶ tr.AddBytes(ip, n2)
  └─ fmt.Fprint(w, "\r\n")     ──▶ tr.AddBytes(ip, n3)
```

Each write is tracked individually. `Tracker.AddBytes()` increments both per-client and global byte counters.

---

## Browser-Native Bridge Pipeline (Wayland)

```
┌──────────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐
│  getDisplay  │────▶│  Canvas  │────▶│ toBlob() │────▶│ POST     │
│  Media()     │     │ draw     │     │ JPEG     │     │ /push    │
└──────────────┘     └──────────┘     └──────────┘     └────┬─────┘
                                                             │
                                         ┌───────────────────┘
                                         │
                                         ▼
                                  ┌──────────────┐     ┌──────────────┐
                                  │ Set Header   │────▶│ Broadcaster  │
                                  │ "BRIDGE"     │     │ .Publish()   │
                                  └──────────────┘     └──────────────┘
                                         │
                                         ▼
                                  ┌──────────────┐
                                  │ ffmpeg       │
                                  │ capture stops│
                                  │ (sees BRIDGE)│
                                  └──────────────┘
```

### Bridge Activation Sequence

1. Teacher clicks "START BROADCAST" in the web UI
2. Browser requests screen capture via `getDisplayMedia({ video: { frameRate: 15 } })`
3. Video stream drawn to offscreen `<canvas>`
4. `canvas.toBlob()` produces JPEG at 15 FPS (~66ms intervals)
5. Each blob POSTed to `/push`
6. Server sets `Broadcaster.header = "BRIDGE"`
7. Capture goroutine in `capture.go` detects the BRIDGE header and exits
8. All subsequent frames come from the browser bridge

---

## Client Telemetry Flow

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   Browser    │────▶│   /ping      │────▶│  GetClient() │────▶│  ClientInfo  │
│  (every 2s)  │     │   endpoint   │     │  (upsert)    │     │  updated     │
└──────────────┘     └──────────────┘     └──────┬───────┘     └──────────────┘
                                                  │
                                    ┌─────────────┼─────────────┐
                                    │             │             │
                                    ▼             ▼             ▼
                              ┌──────────┐ ┌──────────┐ ┌──────────┐
                              │ DNS      │ │ MAC      │ │ Fields   │
                              │ lookup   │ │ lookup   │ │ written  │
                              │ (async)  │ │ (async)  │ │ directly │
                              └──────────┘ └──────────┘ └──────────┘
```

### Telemetry Data

| Browser sends       | Stored in          | Resolution method       |
| ------------------- | ------------------ | ----------------------- |
| `latency`           | `ClientInfo.Latency` | Direct assignment     |
| `os`                | `ClientInfo.OS`      | Direct assignment     |
| `browser`           | `ClientInfo.Browser`  | Direct assignment     |
| `resolution`        | `ClientInfo.Resolution`| Direct assignment     |
| `device`            | `ClientInfo.DevType`  | Direct assignment     |
| `gpu`               | `ClientInfo.GPU`      | Direct assignment     |
| `battery`           | `ClientInfo.BatPct`   | Direct assignment     |
| `charging`          | `ClientInfo.Charging` | Direct assignment     |
| (implicit)          | `ClientInfo.Hostname` | `net.LookupAddr(ip)`  |
| (implicit)          | `ClientInfo.MAC`      | `/proc/net/arp` parse |

---

## Client Lifecycle State Machine

```
                    ┌─────────┐
         ┌─────────│ UNKNOWN │
         │         └────┬────┘
         │              │ First request to any endpoint
         │              ▼
         │         ┌─────────┐
         │    ┌───▶│ ACTIVE  │◀──── Every /ping, /stream, /frame refresh
         │    │    └────┬────┘
         │    │         │ No activity for > ClientTimeout (30s)
         │    │         ▼
         │    │    ┌──────────┐
         │    └────│ INACTIVE │
         │         └──────────┘
         │              │
         │              │ (remains in Clients map)
         │              ▼
         │         ┌──────────┐
         └─────────│ PRUNED   │
                   └──────────┘
```

**Note:** Clients are never deleted from the map — only marked inactive. The dashboard shows them with `○` (inactive) vs `*` (active).

---

## Dashboard Data Collection Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                     Dashboard Loop (1s)                          │
│                                                                  │
│  ┌──────────────────┐                                            │
│  │ GetSysStats()    │                                            │
│  │                  │                                            │
│  │ /proc/loadavg    │──▶ CPU load (1-min avg / 8 cores)         │
│  │ /proc/cpuinfo    │──▶ CPU frequency (MHz → GHz)              │
│  │ /sys/class/hwmon │──▶ CPU temp, PCH temp, Fan RPM           │
│  │ /proc/meminfo    │──▶ RAM + Swap (total, available)          │
│  │ syscall.Statfs   │──▶ Disk (total, used)                     │
│  │ /sys/class/drm   │──▶ GPU frequency                          │
│  │ /sys/class/power │──▶ Battery (percentage, charging)         │
│  │ /sys/class/net   │──▶ NIC (interface, speed, WiFi/Ethernet)  │
│  └──────────────────┘                                            │
│                                                                  │
│  ┌──────────────────┐                                            │
│  │ Tracker data     │                                            │
│  │                  │                                            │
│  │ Clients map      │──▶ Active count, per-client bandwidth     │
│  │ TotalBytes       │──▶ Total transmitted                      │
│  │ Rejections       │──▶ Rejected client log                    │
│  └──────────────────┘                                            │
│                                                                  │
│  ┌──────────────────┐                                            │
│  │ Broadcaster data │                                            │
│  │                  │                                            │
│  │ Total            │──▶ Total published bytes                  │
│  │ GetPubRate()     │──▶ Current publish rate (reset on read)   │
│  └──────────────────┘                                            │
│                                                                  │
│  ▼                                                                │
│  Render ANSI boxes with stats + client table + rejected log      │
└─────────────────────────────────────────────────────────────────┘
```

### Bandwidth Calculation

```
Per-client MB/s = (Client.Bytes - Client.PrevBytes) / 1024 / 1024
Total MB/s      = sum(all per-client MB/s)
Uplink MB       = Client.Bytes / 1024 / 1024 (cumulative)
```

`PrevBytes` is updated each dashboard tick, making it a delta measurement.

---

## Shutdown Sequence

```
SIGINT/SIGTERM
    │
    ▼
┌───────────────────────────────────────┐
│ 1. srv.Shutdown(ctx)                  │
│    └─ Drains active HTTP connections  │
│    └─ /stream handlers detect         │
│       r.Context().Done() and exit     │
│    └─ 2-second timeout                │
├───────────────────────────────────────┤
│ 2. capturer.Stop()                    │
│    └─ Close done channel              │
│    └─ pkill -9 -P <ffmpeg_pid>        │
│    └─ cmd.Process.Kill()              │
├───────────────────────────────────────┤
│ 3. mDNS cleanup functions             │
│    └─ Kill avahi-publish processes    │
├───────────────────────────────────────┤
│ 4. Exit(0)                            │
└───────────────────────────────────────┘
```

---

## mDNS Registration Flow

```
main.go: mdns.Register(localIP)
    │
    ▼
┌───────────────────────────────────────┐
│ Resolve avahi-publish path:           │
│  1. exec.LookPath("avahi-publish")   │
│  2. vendor/bin/avahi-publish          │
│  3. /opt/bifrost/bin/avahi-publish    │
├───────────────────────────────────────┤
│ For each name ("bifrost.local"):      │
│  exec.Command(avahi, "-a", "-R",      │
│    name, ip)                          │
│  └─ Start process                     │
│  └─ Pipe stderr → log output          │
├───────────────────────────────────────┤
│ Return cleanup functions:             │
│  func() { cmd.Process.Kill() }        │
└───────────────────────────────────────┘
```

The `-a` flag registers an address record, `-R` enables reverse-address lookups.

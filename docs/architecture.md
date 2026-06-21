# BIFROST — Technical Architecture

## System Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                        BIFROST Server (Go)                           │
│                                                                      │
│  ┌──────────┐   ┌────────────┐   ┌────────────────────────────────┐ │
│  │ Capture   │──▶│ Stream     │──▶│ HTTP Server (:8080)            │ │
│  │ (ffmpeg   │   │ Broadcaster│   │  ┌──────┐ ┌────────┐ ┌──────┐ │ │
│  │  stdout)  │   │ (Pub/Sub)  │   │  │ /    │ │/stream │ │/frame│ │ │
│  └──────────┘   └────────────┘   │  │HTML  │ │MJPEG   │ │JPEG  │ │ │
│                                   │  └──────┘ └────────┘ └──────┘ │ │
│  ┌──────────┐                    │  ┌──────┐ ┌──────┐ ┌────────┐ │ │
│  │ Tracker  │◀───────────────────│  │/ping │ │/push │ │/health │ │ │
│  │ (Client  │                    │  │Telem │ │Bridge│ │JSON    │ │ │
│  │  State)  │                    │  └──────┘ └──────┘ └────────┘ │ │
│  └──────────┘                    └────────────────────────────────┘ │
│  ┌──────────┐   ┌───────────┐                                        │
│  │ mDNS     │   │ Dashboard │  ┌──────────┐                         │
│  │(avahi)   │   │ (TUI)     │  │ Watcher  │                         │
│  └──────────┘   └───────────┘  │(dev mode)│                         │
│                                 └──────────┘                         │
└─────────────────────────────────────────────────────────────────────┘
                             │
                    ┌────────▼────────┐
                    │  Student Browser │
                    │  (Linux/Android) │
                    └─────────────────┘
```

## Component Interaction Map

```
main.go
  │
  ├─▶ cleanupOrphans()         kills stale ffmpeg/avahi processes
  ├─▶ getLocalIP()             detects LAN IP
  ├─▶ tracker.New()            creates client registry
  ├─▶ stream.NewBroadcaster()  creates pub/sub hub (video)
  ├─▶ capture.NewCapturer()    creates capture handle
  ├─▶ mdns.Register()          starts avahi-publish
  ├─▶ capturer.Start()         spawns ffmpeg, feeds Broadcaster
  ├─▶ server.New()             builds HTTP mux with all routes
  ├─▶ watcher.Start()          (dev only) polls for source changes
  └─▶ signal.Wait(SIGINT/TERM) graceful shutdown
```

## Capture Pipeline

### X11 Path (Default)

```
ffmpeg -f x11grab -draw_mouse 1 -video_size 1280x720 -framerate 30 -i :0.0
       -f mjpeg -q:v 2 -
```

1. ffmpeg grabs the X11 display at 1280x720
2. Outputs raw JPEG stream to stdout
3. The capture goroutine reads stdout in 64KB chunks
4. JPEG frames are delimited by `0xFF 0xD8` (SOI) and `0xFF 0xD9` (EOI) markers
5. Each complete frame is published to the Broadcaster

### Wayland Path

```
ffmpeg -f kmsgrab -follow_mouse 1 -i /dev/dri/card0
       -vf 'hwdownload,format=bgr0,fps=15,scale=1280:-1'
       -f mjpeg -q:v 2 -
```

1. Uses KMS (kernel mode setting) grab interface via `/dev/dri/card0`
2. Hardware download from GPU buffer, format conversion to BGR0
3. Scaled to 1280px width, maintaining aspect ratio
4. Frame rate capped at 15 FPS to reduce system load
5. Same JPEG frame parsing as X11 path

### Browser-Native Bridge (Wayland Fallback)

When the teacher activates the Wayland bridge from the web UI:

1. Browser calls `navigator.mediaDevices.getDisplayMedia()` to capture the screen
2. Frames are drawn to a `<canvas>` element
3. `canvas.toBlob()` produces JPEG blobs at 15 FPS
4. Each blob is POSTed to `/push`
5. Server sets the Broadcaster header to `"BRIDGE"`, signaling the ffmpeg capture to stop
6. Frames are published to all subscribers via the normal Broadcaster path

## MJPEG Frame Boundary Parsing

The capture loop in `internal/capture/capture.go:86-152` implements a streaming JPEG frame parser:

```
Buffer fills with ffmpeg stdout bytes
  │
  ├─ Scan for 0xFF 0xD8 (JPEG Start of Image)
  │   └─ If not found: keep last byte (boundary alignment), clear buffer
  │
  ├─ From SOI position, scan for 0xFF 0xD9 (JPEG End of Image)
  │   ├─ If EOI found: extract complete frame [SOI..EOI], publish
  │   └─ If EOI not found: retain partial data, wait for more bytes
  │
  └─ On first frame: write debug_capture.jpg for verification
```

Key design decisions:
- Non-blocking reads with `default` case on done channel
- Buffer retains partial data across reads for streaming continuity
- Bridge detection: if Broadcaster header is `"BRIDGE"`, capture goroutine exits to avoid dual-publishing

## Concurrency Model

### Mutexes

| Mutex              | Protects                           | Type        |
| ------------------ | ---------------------------------- | ----------- |
| `Broadcaster.mu`   | `clients` map, `header`, `Total`   | `sync.RWMutex` |
| `Tracker.mu`       | `Clients` map, `Rejections`, `TotalBytes` | `sync.RWMutex` |
| `Capturer.mu`      | `cmd`, `done` channel              | `sync.Mutex`    |

### Channels

| Channel      | Type        | Capacity | Purpose                          |
| ------------ | ----------- | -------- | -------------------------------- |
| `Capturer.done` | `struct{}` | 0       | Signal capture goroutine to stop |
| Subscriber channels | `[]byte` | 10-100  | Per-client frame delivery        |

### Goroutines

| Spawned From          | Lifetime       | Purpose                                    |
| --------------------- | -------------- | ------------------------------------------ |
| `capturer.Start()`    | Until `Stop()` | Read ffmpeg stdout, parse JPEG frames      |
| `capturer.Start()`    | Until `Stop()` | Heartbeat loop (5s tick)                   |
| `tracker.GetClient()` | Short-lived    | Async DNS reverse lookup                   |
| `tracker.GetClient()` | Short-lived    | Async MAC lookup via `/proc/net/arp`       |
| `server.New()`        | Until shutdown | HTTP server accept loop                    |
| `watcher.Start()`     | Until shutdown | Poll filesystem for changes (dev only)     |
| `mdns.Register()`     | Until shutdown | Read avahi-publish stderr                  |
| `main()`              | Blocks         | Wait for SIGINT/SIGTERM                    |

## Client Lifecycle

```
Browser connects to /, /stream, or /frame
  │
  ├─ getIP() extracts client IP from RemoteAddr
  │
  ├─ guard.RejectWindows() checks User-Agent
  │   ├─ Contains "windows" → 403 + denial page + LogRejection()
  │   └─ Passes through
  │
  ├─ tracker.GetClient(ip) returns or creates ClientInfo
  │   ├─ First seen: spawn goroutines for DNS + MAC resolution
  │   └─ Existing: update LastSeen, mark Active
  │
  ├─ tracker.AddBytes(ip, n) increments bandwidth counter
  │
  └─ tracker.Prune(30s) periodically marks inactive clients
      (called in background, runs every ClientTimeout)
```

## System Statistics Collection

`internal/dashboard/dashboard.go` reads directly from procfs/sysfs:

| Stat      | Source                                    | Update Rate |
| --------- | ----------------------------------------- | ----------- |
| CPU usage | `/proc/loadavg` (1-min load / 8 cores)   | 1s          |
| CPU freq  | `/proc/cpuinfo` (`cpu MHz` field)         | 1s          |
| CPU temp  | `/sys/class/hwmon/hwmon*/temp1_input` (coretemp/k10temp) | 1s |
| RAM/Swap  | `/proc/meminfo` (MemTotal, MemAvailable, SwapTotal, SwapFree) | 1s |
| Disk      | `syscall.Statfs("/")`                     | 1s          |
| GPU freq  | `/sys/class/drm/card*/gt_act_freq_mhz`    | 1s          |
| Battery   | `/sys/class/power_supply/BAT*/capacity` + `status` | 1s |
| Fan       | `/sys/class/hwmon/hwmon*/fan1_input`      | 1s          |
| PCH temp  | `/sys/class/hwmon/hwmon*/temp1_input` (pch_skylake/cannonlake) | 1s |
| NIC       | `/sys/class/net/*/operstate`, `speed`, `wireless` | 1s |

## Graceful Shutdown Sequence

```
SIGINT or SIGTERM received
  │
  ├─ srv.Shutdown(ctx)  — drains HTTP connections (2s timeout)
  │   └─ Existing /stream connections close via r.Context().Done()
  │
  ├─ capturer.Stop()    — closes done channel, kills ffmpeg process
  │   └─ ffmpeg killed via pkill -9 -P <pid> + Process.Kill()
  │
  ├─ mDNS cleanup       — kills avahi-publish processes
  │
  └─ Exit
```

## HTTP Server Design

- Uses `http.NewServeMux` (stdlib, no external dependencies)
- `recoverMiddleware` wraps all handlers to catch panics and return 500
- `TCP_NODELAY` set on new connections via `ConnState` callback for reduced latency
- No TLS — designed for trusted LAN deployment
- Port 8080 (configurable via `Port` constant in main.go)

## Data Flow: End-to-End

```
                    ┌─────────────────────────────────┐
                    │        Teacher Machine           │
                    │                                   │
  Display ──▶ ffmpeg ──▶ JPEG stdout ──▶ Frame Parser  │
                                            │           │
                                       Broadcaster      │
                                       .Publish()       │
                                            │           │
                    └───────────────────────┼───────────┘
                                            │
                              HTTP /stream (multipart MJPEG)
                                            │
                    ┌───────────────────────┼───────────┐
                    │                       ▼            │
                    │              Student Browsers      │
                    │              ┌──────────┐          │
                    │              │ <img>    │          │
                    │              │ src=     │          │
                    │              │ /stream  │          │
                    │              └──────────┘          │
                    │                                    │
                    │  Every 2s: GET /ping?latency=N     │
                    │  Browser reports: OS, browser,     │
                    │  GPU, resolution, device, battery  │
                    └────────────────────────────────────┘
```

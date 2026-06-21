# BIFROST — Internal Packages

## `internal/stream` — Pub/Sub Broadcaster

**File:** `internal/stream/stream.go`

A generic, thread-safe publish/subscribe mechanism used for both video (MJPEG frames) and audio (MP3 chunks).

### Types

#### `Broadcaster`

```go
type Broadcaster struct {
    mu        sync.RWMutex
    clients   map[chan []byte]struct{}
    header    []byte
    Total     int64
    PubRate   int64
    lastFrame []byte
}
```

| Field      | Description                                                |
| ---------- | ---------------------------------------------------------- |
| `clients`  | Set of subscriber channels, keyed by channel pointer       |
| `header`   | Optional metadata sent to new subscribers (MJPEG boundary or `"BRIDGE"` marker) |
| `Total`    | Cumulative bytes published across all sessions             |
| `PubRate`  | Bytes published in the current measurement cycle; reset on read |
| `lastFrame`| Copy of the most recently published frame (for `/frame` endpoint) |

#### Methods

- `NewBroadcaster()` — Creates a new Broadcaster with empty client set.
- `SetHeader(header []byte)` — Sets or clears the header metadata. A `nil` header clears it. A header of `"BRIDGE"` signals the capture goroutine to stop.
- `GetHeader() []byte` — Returns current header (thread-safe read).
- `Subscribe(bufferSize int) chan []byte` — Creates a buffered channel, registers it, and immediately sends the current header if present. Returns the channel.
- `Unsubscribe(ch chan []byte)` — Removes the channel from the client set. Does NOT close the channel (avoids panic in concurrent `Publish`).
- `Publish(data []byte)` — Non-blocking fan-out to all subscribers. Updates `Total`, `PubRate`, and `lastFrame`. Skips subscribers with full buffers.
- `GetLastFrame() []byte` — Returns a copy of the last published frame.
- `GetPubRate() int64` — Returns bytes published since the last call and resets the counter.

---

## `internal/capture` — Screen & Audio Capture

**File:** `internal/capture/capture.go`

Manages the ffmpeg subprocess for screen capture and parses JPEG frames from its stdout.

### Types

#### `Capturer`

```go
type Capturer struct {
    fps     int
    quality int
    cmd     *exec.Cmd
    done    chan struct{}
    mu      sync.Mutex
}
```

#### Methods

- `NewCapturer(fps, quality int) *Capturer` — Creates a capturer. Defaults fps to 15 if <= 0.
- `Start(broadcaster *stream.Broadcaster) error` — Detects session type (XDG_SESSION_TYPE), builds the appropriate ffmpeg command, starts it, and spawns goroutines to:
  1. Parse JPEG frames from stdout (SOI/EOI marker detection)
  2. Publish complete frames to the Broadcaster
  3. Monitor for `"BRIDGE"` header to yield to browser capture
- `Stop()` — Closes the done channel, kills the ffmpeg process and its children via `pkill -9 -P <pid>`.
- `DetectPulseAudioMonitor() string` — Stub (returns empty string).

### Frame Parsing Algorithm

```
Read 64KB chunks from ffmpeg stdout into buffer
  │
  ▼
┌─────────────────────────────────────────┐
│ Scan for 0xFF 0xD8 (JPEG SOI)           │
│   └─ Not found → keep last byte, clear  │
│                                         │
│ From SOI, scan for 0xFF 0xD9 (JPEG EOI) │
│   ├─ Found → extract [SOI..EOI], publish│
│   └─ Not found → retain partial, wait   │
└─────────────────────────────────────────┘
```

---

## `internal/tracker` — Client Tracking

**File:** `internal/tracker/tracker.go`

Thread-safe client registry with bandwidth monitoring and async hostname/MAC resolution.

### Types

#### `ClientInfo`

```go
type ClientInfo struct {
    IP         string
    Hostname   string
    MAC        string
    Bytes      int64
    PrevBytes  int64
    LastSeen   time.Time
    Latency    int
    OS         string
    Browser    string
    Resolution string
    DevType    string
    GPU        string
    BatPct     int
    Charging   bool
    Active     bool
}
```

#### `RejectedClient`

```go
type RejectedClient struct {
    IP        string
    OS        string
    Reason    string
    Time      time.Time
    UserAgent string
}
```

#### `Tracker`

```go
type Tracker struct {
    mu         sync.RWMutex
    Clients    map[string]*ClientInfo
    Rejections []RejectedClient
    TotalBytes int64
}
```

#### Methods

- `New() *Tracker` — Creates tracker with empty maps.
- `GetClient(ip string) *ClientInfo` — Returns existing or creates new client. Spawns async goroutines for:
  - DNS reverse lookup (`net.LookupAddr`)
  - MAC resolution (parses `/proc/net/arp`)
- `AddBytes(ip string, n int64)` — Increments per-client and total byte counters.
- `LogRejection(ip, os, reason, ua string)` — Prepends rejection to log (max 5 entries).
- `Prune(timeout time.Duration)` — Marks clients as inactive if `LastSeen` exceeds timeout.
- `GetAllClients() []*ClientInfo` — Returns snapshot of all clients.
- `Lock()/Unlock()/RLock()/RUnlock()` — Expose internal mutex for direct access (used by `dashboard`).

### MAC Resolution

Parses `/proc/net/arp` line by line:
```
IP address       HW type     Flags       HW address            Mask  Device
192.168.1.50     0x1         0x2         AA:BB:CC:DD:EE:FF     *     wlp2s0
```
Matches the target IP in column 0, returns the MAC in column 3.

---

## `internal/server` — HTTP Server

**File:** `internal/server/server.go`

Builds and configures the HTTP server with all routes, middleware, and TCP optimizations.

### Key Functions

- `New(tr, stream, viewerHTML) *http.Server` — Constructs the server with all routes registered.
- `recoverMiddleware(next) http.HandlerFunc` — Wraps handlers with panic recovery.
- `getIP(r) string` — Extracts client IP from `RemoteAddr` (strips port).

### Routes Summary

| Route       | Handler Type | Description                          |
| ----------- | ------------ | ------------------------------------ |
| `/`         | Closure      | Serves viewer HTML                   |
| `/frame`    | Closure      | Single JPEG frame                    |
| `/stream`   | Closure      | MJPEG multipart stream               |
| `/audio`    | Inline       | Returns 410 Gone                     |
| `/ping`     | Closure      | Client telemetry receiver            |
| `/rejected` | Closure      | Rejection logger                     |
| `/push`     | Closure      | Browser bridge frame receiver        |
| `/stats`    | Closure      | JSON stats for teacher dashboard     |
| `/health`   | Closure      | Health check                         |

### TCP Optimization

```go
ConnState: func(conn net.Conn, state http.ConnState) {
    if state == http.StateNew {
        if tc, ok := conn.(*net.TCPConn); ok {
            tc.SetNoDelay(true)
        }
    }
}
```

Disables Nagle's algorithm on all new TCP connections for minimum latency.

---

## `internal/guard` — Windows Rejection

**File:** `internal/guard/guard.go`

HTTP middleware that blocks Windows clients.

### Function

```go
func RejectWindows(tr *tracker.Tracker, next http.HandlerFunc) http.HandlerFunc
```

- Checks `User-Agent` header (lowercased) for `"windows"`
- If matched: logs rejection, returns `403 Forbidden` with styled HTML denial page
- If not matched: passes through to next handler

---

## `internal/mdns` — mDNS Registration

**File:** `internal/mdns/mdns.go`

Registers `.local` hostnames via `avahi-publish`.

### Function

```go
func Register(ip string) []func()
```

- Publishes `bifrost.local` pointing to the server's LAN IP
- Falls back to `vendor/bin/avahi-publish` or `/opt/bifrost/bin/avahi-publish` if not in PATH
- Returns cleanup functions that kill the avahi-publish processes
- Logs stderr from avahi-publish for diagnostics

---

## `internal/dashboard` — Terminal TUI

**File:** `internal/dashboard/dashboard.go`

Live terminal dashboard with system stats and client monitoring.

### Function

```go
func Start(tr *tracker.Tracker, broadcaster *stream.Broadcaster, ip string, version string)
```

Runs an infinite loop (1s refresh) that:

1. Calls `GetSysStats()` to gather system metrics from procfs/sysfs
2. Renders three boxed sections:
   - **Header:** App name, version, connection URLs
   - **System:** CPU, RAM, swap, GPU, disk, NIC, battery, fan
   - **Students:** Connected clients with status, device type, IP, bandwidth, uplink
3. Shows rejected clients if any
4. Footer with uptime, active count, total transferred, log size, time

### Helper Functions

- `bar(pct, width)` — Renders a progress bar using `█` and `░` characters
- `boxTop/Bottom/Row/Separator/Footer` — ANSI-colored box drawing primitives
- `readLine(path)` — Reads first line of a file (for procfs/sysfs)
- `emptyIfBlank(value, fallback)` — Returns fallback if value is empty

---

## `internal/watcher` — Dev Hot-Reload

**File:** `internal/watcher/watcher.go`

Polls source directories for changes and rebuilds/restarts.

### Function

```go
func Start(dirs []string, buildCmd []string)
```

- Watches `cmd/`, `internal/`, `web/` directories
- Monitors `.go`, `.html`, `.tmpl` file extensions
- Polls every 1 second via `time.Ticker`
- On change detected: runs `go build -o bifrost ./cmd/bifrost`
- On success: replaces current process via `syscall.Exec` (Unix) or spawns new process + `os.Exit` (Windows)
- Detects new, modified, and deleted files

---

## `web` — Embedded Assets

**File:** `web/web.go`

```go
//go:embed templates/viewer.html
var ViewerHTML string
```

Uses Go's `//go:embed` directive to compile `viewer.html` into the binary at build time. No external file dependencies at runtime.

### `web/templates/viewer.html`

Single-file web application with:

- **CSS:** Dark theme with red accents, pulsing border animation, HUD overlay positioning
- **Student View:** `<img>` element refreshing from `/frame` at ~30 FPS (33ms interval)
- **Teacher Mode:** Activated via `?teacher=true` query param or localhost detection
  - Stats polling from `/stats` every 2 seconds
  - Active student table with IP, OS, latency, bandwidth
  - Wayland bridge controls (getDisplayMedia → canvas → /push)
  - Rejection log display
- **Client Fingerprinting:** Detects OS, browser, GPU (WebGL), screen resolution, device type, battery level
- **Controls:** SYNC (reload), FULLSCREEN toggle
- **Latency Display:** Measures round-trip time from `/frame` fetch

---

## `debian/` — Packaging

### `bifrost.service`

systemd unit file:
- Runs as `root`
- `Type=simple`
- Depends on `network.target`, `avahi-daemon`, `display-manager.service`
- Sets `XDG_SESSION_TYPE=wayland`
- `Restart=on-failure`

### `control`

Debian package metadata (package name, version, architecture, dependencies).

### `postinst`

Post-installation script for setting up permissions and enabling the service.

# BIFROST

**B**rowser **I**ntegrated **F**eed for **R**emote **O**bservation & **S**creen **T**ransmission

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20Android-lightgrey)](https://github.com/nelobster/BIFROST)

BIFROST is a high-performance, zero-configuration **classroom screen broadcasting server** that streams a teacher's desktop (video + audio) to student browsers over a local network. Built in Go, it delivers near-zero-latency MJPEG video and MP3 audio through a sleek web interface — no client software required.

---

## Features

- **Screen Capture** — Captures the display using `ffmpeg` (X11) or `cosmic-screenshot` (Wayland/COSMIC desktop)
- **Audio Streaming** — Captures and streams system audio via PulseAudio monitor / `ffmpeg` as MP3
- **Web Viewer** — Elegant browser-based viewer with HUD overlay, live status indicators, and latency display
- **mDNS Discovery** — Automatically publishes the stream on the LAN via `avahi-publish` (e.g., `http://bifrost.local:8080`)
- **Terminal Dashboard** — Live TUI dashboard showing:
  - System stats (CPU, GPU, RAM, disk, swap, NIC, battery, fan, temperatures)
  - Connected clients with bandwidth usage and device info
  - Rejected client log
- **Client Tracking** — Monitors each connected client's IP, hostname, MAC address, bandwidth, device type, browser, GPU, battery level, and latency
- **Windows Guard** — Automatically rejects Windows clients with a styled denial page
- **Health Endpoint** — JSON health check at `/health` for monitoring
- **Debian Packaging** — Ready-to-build `.deb` with systemd service for production deployment
- **Development Watch Mode** — `watch.sh` script auto-rebuilds and restarts on source changes

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        BIFROST Server (Go)                          │
│                                                                     │
│  ┌──────────┐   ┌───────────┐   ┌────────────────────────────────┐ │
│  │ Capture   │──>│ Stream    │──>│ HTTP Server (:8080)             │ │
│  │ (Screen   │   │ Broadcaster│  │  ┌──────┐ ┌──────┐ ┌────────┐ │ │
│  │  + Audio) │   │ (Pub/Sub) │  │  │ /    │ │/stream││ /audio │ │ │
│  └──────────┘   └───────────┘  │  │Viewer │ │MJPEG  ││ MP3    │ │ │
│                                 │  │ HTML  │ │Video  ││ Audio  │ │ │
│  ┌──────────┐                   │  └──────┘ └──────┘ └────────┘ │ │
│  │ Tracker  │<──────────────────│  ┌────────┐ ┌───────────────┐ │ │
│  │ (Clients │                   │  │ /ping  │ │ /health       │ │ │
│  │  + Stats)│                   │  │Telemetry│ │ (JSON)        │ │ │
│  └──────────┘                   │  └────────┘ └───────────────┘ │ │
│                                 └────────────────────────────────┘ │
│  ┌──────────┐   ┌───────────┐                                     │
│  │ mDNS     │   │ Dashboard │                                     │
│  │(avahi)   │   │ (TUI)     │                                     │
│  └──────────┘   └───────────┘                                     │
└─────────────────────────────────────────────────────────────────────┘
                            │
                            ▼
                   ┌────────────────┐
                   │  Student       │
                   │  Browser       │
                   │  (Linux/Android)│
                   └────────────────┘
```

---

## Quick Start

### Prerequisites

- **Go 1.22+** — [Install Go](https://go.dev/dl/)
- **ffmpeg** — `sudo apt-get install ffmpeg`
- **avahi-daemon & avahi-utils** — `sudo apt-get install avahi-daemon avahi-utils`
- **PipeWire** — `sudo apt-get install pipewire` (for audio capture)
- **inotify-tools** — `sudo apt-get install inotify-tools` (for watch mode only)
- **cosmic-screenshot** — (optional) Required for native Wayland/COSMIC capture

### Build & Run

```bash
# Build from source
go build -o bifrost ./cmd/bifrost

# Run (may require sudo for privileged ports)
./bifrost
```

On startup, BIFROST will:
1. Display an ASCII art banner and connection URLs
2. Kill any orphaned `ffmpeg` / `gst-launch-1.0` processes
3. Detect your session type (X11 / Wayland)
4. Detect PulseAudio monitor for audio capture
5. Register mDNS service (e.g., `mnkb.local`)
6. Start screen and audio capture
7. Launch the HTTP server on port **8080**
8. Show the live terminal dashboard

Open your browser to `http://bifrost.local:8080` or `http://<local-ip>:8080`.

### Development Watch Mode

```bash
./watch.sh
```

Auto-rebuilds and restarts BIFROST whenever any `.go` or template file changes.

---

## Usage

### Web Viewer

The viewer at `/` provides:
- **Live MJPEG video stream** — auto-updating via multipart response
- **Audio stream** — MP3 audio played automatically
- **HUD overlay** showing:
  - Connection status (LIVE / DISCONNECTED)
  - Latency (ping-based measurement)
- **Controls**:
  - `SYNC` — re-synchronize the video stream
  - `REFRESH` — reload the page
  - `FULLSCREEN` — toggle fullscreen mode
- **Client fingerprinting** — automatically reports OS, browser, GPU, resolution, device type, and battery level

### HTTP Endpoints

| Endpoint       | Method | Description                                          |
|----------------|--------|------------------------------------------------------|
| `/`            | GET    | Serves the web viewer HTML                           |
| `/stream`      | GET    | MJPEG video stream (`multipart/x-mixed-replace`)     |
| `/audio`       | GET    | MP3 audio stream (chunked transfer)                  |
| `/ping`        | GET    | Client telemetry endpoint (latency, OS, browser, etc.)|
| `/health`      | GET    | JSON health check (`{"streaming":true, "clients": N}`)|
| `/rejected`    | GET    | Client-side Windows rejection logging endpoint       |

### Terminal Dashboard

The TUI dashboard refreshes every second and displays:

```
╭─────────────────────────────────────────────────────────────╮
│ ██████╗ ██╗███████╗██████╗  ██████╗ ███████╗████████╗      │
│ ██╔══██╗██║██╔════╝██╔══██╗██╔═══██╗██╔════╝╚══██╔══╝      │
│ ██████╔╝██║█████╗  ██████╔╝██║   ██║███████╗   ██║          │
│ ██╔══██╗██║██╔══╝  ██╔══██╗██║   ██║╚════██║   ██║          │
│ ██████╔╝██║██║     ██║  ██║╚██████╔╝███████║   ██║          │
│  v0.1.0  │ mnkb.local:8080               [12:34:56]          │
│           │ 192.168.1.100:8080 (direct IP)                   │
╰─────────────────────────────────────────────────────────────╯
╭── SYSTEM ───────────────────────────────────────────────────╮
│  CPU: ██████░░░░ 52% 3.2GHz 45°C  RAM:  ████░░░░░░ 4.2/16G │
│  GPU: ░░░░░░░░░░ 1200MHz 55°C         DISK: ██████░░░░ 256G │
│  FAN: ░░░░░░░░░░ 1200 RPM            SWAP: ░░░░░░░░░░ 2.0G  │
│  NIC: wlp2s0  866Mb/s WiFi           BAT:  85% Charging     │
╰─────────────────────────────────────────────────────────────╯
╭── STUDENT STREAM MONITORING ( 3 active) ───────────────────╮
│ S │ # │ DEV │ IP ADDRESS     │ BANDWIDTH      │ UPLINK     │
│ ● │ 1 │ 📱  │ 192.168.1.50  │ ████░░░ 2.1M/s │   45.2M    │
│ ● │ 2 │ 💻  │ 192.168.1.51  │ ██░░░░░ 1.5M/s │   32.1M    │
│ ○ │ 3 │ 💻  │ 192.168.1.52  │ ░░░░░░░ 0.0M/s │   10.8M    │
│ ──────────────────────────────────────────────────────────  │
│ Σ │   │     │                │ ██░░░░░ 3.6M/s │   88.1M    │
╰─────────────────────────────────────────────────────────────╯
```

---

## Project Structure

```
├── cmd/bifrost/main.go          # Application entrypoint & orchestration
├── internal/
│   ├── capture/capture.go       # Screen & audio capture (ffmpeg / cosmic-screenshot)
│   ├── dashboard/dashboard.go   # Terminal TUI dashboard with system stats
│   ├── guard/guard.go           # Windows client rejection middleware
│   ├── mdns/mdns.go             # mDNS service registration (avahi-publish)
│   ├── server/server.go         # HTTP server with routes & middleware
│   ├── stream/stream.go         # Publish/subscribe broadcast mechanism
│   └── tracker/tracker.go       # Client tracking, bandwidth logging, pruning
├── web/
│   ├── web.go                   # Embedded HTML (via //go:embed)
│   └── templates/
│       └── viewer.html          # Web viewer frontend (HTML/CSS/JS)
├── debian/
│   ├── bifrost.service          # systemd service unit
│   ├── control                  # Debian package control file
│   └── postinst                 # Post-installation script
├── go.mod                       # Go module definition (Go 1.22)
├── watch.sh                     # Auto-rebuild development script
├── test.mjpeg                   # Test MJPEG sample
├── .gitattributes               # Git LFS / text normalization
└── README.md                    # This file
```

---

## Internal Packages

### [`cmd/bifrost/main.go`](cmd/bifrost/main.go)

The application entry point that:
- Displays the BIFROST ASCII art banner
- Detects the local IP address
- Kills orphaned `ffmpeg` / `gst-launch-1.0` processes
- Detects session type (Wayland vs X11)
- Initializes all components (Tracker, Stream, mDNS, Capture, Audio, Server)
- Starts the HTTP server and background goroutines (tracker pruning, dashboard)
- Handles graceful shutdown on SIGINT/SIGTERM

### [`internal/capture`](internal/capture/capture.go)

Handles screen and audio capture:

- **`Capturer`** — struct managing the capture lifecycle
  - `NewCapturer(fps)` — creates a new capturer with target frame rate
  - `Start(broadcaster)` — begins screen capture:
    - **Wayland/COSMIC:** Uses `cosmic-screenshot` for per-frame PNG capture, encodes to JPEG, caps at ~6 FPS to avoid DBus rate-limiting
    - **X11:** Uses `ffmpeg` with `x11grab` for efficient MJPEG pipe capture at full frame rate
  - `Stop()` — stops capture and kills the ffmpeg process
- **`DetectPulseAudioMonitor()`** — finds the PulseAudio monitor source for audio loopback
- **`StartAudioBroadcaster(broadcaster)`** — launches `ffmpeg` to capture system audio via PulseAudio and stream MP3 to the broadcaster
- **`getPrimaryDisplayGeometry()`** — parses `cosmic-randr list` output to find the primary display's position and dimensions

### [`internal/stream`](internal/stream/stream.go)

A generic **publish/subscribe** broadcast mechanism:

- **`Broadcaster`** — manages a set of subscriber channels
  - `Subscribe() chan []byte` — creates a buffered channel (capacity 30) and registers it
  - `Unsubscribe(ch)` — removes and closes the channel
  - `Publish(data)` — non-blocking send to all subscribers; drops data if a channel is full

Used separately for video (MJPEG frames) and audio (MP3 chunks).

### [`internal/server`](internal/server/server.go)

HTTP server setup with routes and middleware:

- **`trackingResponseWriter`** — wraps `http.ResponseWriter` to track bytes sent per client
- Routes:
  - `GET /` — serves the web viewer HTML (guarded + tracked)
  - `GET /stream` — MJPEG stream from video broadcaster (guarded + tracked)
  - `GET /audio` — MP3 stream from audio broadcaster (guarded + tracked)
  - `GET /ping` — client telemetry: records latency, OS, browser, resolution, device type, GPU, battery
  - `GET /rejected` — client-side Windows rejection logging
  - `GET /health` — JSON health check (`{"streaming": true, "clients": N}`)
- Applies Windows guard and tracking middleware to most routes
- Sets `TCP_NODELAY` on new connections for reduced latency

### [`internal/tracker`](internal/tracker/tracker.go)

Client connection tracking and bandwidth monitoring:

- **`ClientInfo`** — tracks IP, hostname, MAC, bytes transferred, latency, OS, browser, resolution, device type, GPU, battery, and active status
- **`Tracker`** — thread-safe client registry with:
  - `GetClient(ip)` — returns or creates a client; performs async DNS reverse lookup and MAC resolution via `/proc/net/arp`
  - `AddBytes(ip, n)` — increments byte counters for a client and the total
  - `LogRejection(ip, os, reason, ua)` — logs rejected clients (max 5 kept)
  - `Prune(timeout)` — marks clients as inactive if they haven't been seen within the timeout
  - `GetAllClients()` — returns a snapshot of all clients

### [`internal/guard`](internal/guard/guard.go)

Windows access control middleware:

- **`RejectWindows(tracker, next)`** — HTTP middleware that:
  - Checks the `User-Agent` header for "windows"
  - Logs rejection to the tracker
  - Returns a styled `403 Forbidden` page with "ACCESS DENIED"
  - Passes through to `next` for non-Windows clients

### [`internal/mdns`](internal/mdns/mdns.go)

mDNS service registration:

- **`Register(primaryName, fallbackName, ip)`** — publishes `.local` hostnames via `avahi-publish -a -R`
  - Returns cleanup functions to unpublish on shutdown
  - Handles primary and optional fallback names
  - Logs avahi stderr output

### [`internal/dashboard`](internal/dashboard/dashboard.go)

Terminal TUI dashboard providing:

- **System statistics** gathered from `/proc` and `/sys`:
  - CPU: load average, frequency, temperature
  - GPU: frequency (from DRM)
  - RAM & Swap: total, used, percentage
  - Disk: total, usage percentage
  - NIC: interface name, speed, type (WiFi/Ethernet)
  - Battery: percentage, charging status
  - Fan: RPM
  - PCH temperature
- **Client monitoring** — active/inactive status, bandwidth bars, per-client data usage
- **Rejected clients** — logged with IP, reason, and timestamp
- Uses ANSI escape codes for colors and formatting
- Refreshes every second

### [`web`](web/web.go)

Static asset embedding:

- Uses Go's `//go:embed` directive to embed [`web/templates/viewer.html`](web/templates/viewer.html) into the binary at compile time

### [`web/templates/viewer.html`](web/templates/viewer.html)

Client-side web viewer featuring:

- **Windows detection** — client-side UA check with denial page
- **Video display** — `<img>` element consuming MJPEG from `/stream`
- **Audio playback** — `<audio>` element playing MP3 from `/audio`
- **HUD badges** — connection status (blinking dot) and latency display
- **Control buttons** — SYNC, REFRESH, FULLSCREEN
- **Client fingerprinting** — detects OS, browser, GPU (via WebGL), screen resolution, device type, and battery level (via Battery API)
- **Periodic pinging** — sends telemetry to `/ping` every 2 seconds

### [`debian/bifrost.service`](debian/bifrost.service)

systemd service unit for production deployment:

- Runs as `root`
- Restarts automatically on failure
- Depends on `network.target`, `avahi-daemon`, and `display-manager`
- Sets `XDG_SESSION_TYPE=wayland`

### [`watch.sh`](watch.sh)

Development helper that:

- Watches for Go source file changes using `inotifywait`
- Rebuilds the binary on changes
- Restarts BIFROST automatically
- Retries on build failure, waiting for code fixes

---

## Configuration

Key constants defined in [`cmd/bifrost/main.go`](cmd/bifrost/main.go:28):

| Constant           | Default        | Description                              |
|--------------------|----------------|------------------------------------------|
| `StreamPort`       | `8080`         | HTTP server port                         |
| `StreamFPS`        | `30`           | Target capture frame rate (X11 only)     |
| `JPEGQuality`      | `80`           | JPEG encode quality (Wayland path)       |
| `MaxClientRows`    | `20`           | Max clients shown in dashboard           |
| `MaxRejectedRows`  | `5`            | Max rejection entries kept               |
| `ClientTimeout`    | `30s`          | Seconds before marking client inactive   |
| `DashboardRefresh` | `1s`           | Dashboard refresh interval               |
| `MDNSNamePrimary`  | `"bifrost"`    | Primary mDNS hostname (→ `bifrost.local`) |

---

## Deployment

### Debian Package

```bash
# Build the binary
go build -o bifrost ./cmd/bifrost

# Build the .deb package
dpkg-deb --build bifrost-package bifrost_0.1.0_amd64.deb

# Install
sudo dpkg -i bifrost_0.1.0_amd64.deb

# The systemd service will start automatically
# Access the stream at http://bifrost.local:8080
```

### Manual Installation

```bash
# Copy binary to system path
sudo cp bifrost /usr/local/bin/

# Install and enable avahi
sudo apt-get install avahi-daemon avahi-utils
sudo systemctl enable avahi-daemon
sudo systemctl start avahi-daemon

# Run
sudo /usr/local/bin/bifrost
```

---

## Client Requirements

| Device    | Browser           | Status      |
|-----------|-------------------|-------------|
| Linux     | Chrome/Firefox    | ✅ Supported |
| Android   | Chrome            | ✅ Supported |
| macOS     | Safari/Chrome     | ⚠️ Untested |
| iOS       | Safari            | ⚠️ Untested |
| Windows   | Any               | ❌ Blocked   |

---

## Dependencies

### Runtime
- **ffmpeg** — Screen capture (X11) and audio capture via PulseAudio
- **avahi-daemon / avahi-utils** — mDNS service publication
- **PipeWire** — Audio subsystem dependency
- **cosmic-screenshot** — Optional, for native Wayland/COSMIC capture

### Build
- **Go 1.22+** — No external Go dependencies (stdlib only)

---

## License

This project is licensed under the MIT License.

---

## FAQ

**Q: Why does BIFROST block Windows clients?**  
A: In a classroom environment, Windows devices often introduce compatibility issues with the MJPEG stream and audio playback. The guard ensures a consistent experience for Linux and Android users.

**Q: Why is my frame rate low under Wayland?**  
A: Wayland's security model prevents direct screen access. BIFROST falls back to `cosmic-screenshot`, which is capped at ~6 FPS to avoid DBus rate-limiting. For full frame rate, use an X11 session.

**Q: Do I need to run as root?**  
A: Only if you change `StreamPort` to a privileged port (below 1024). Port 8080 works without root.

**Q: How do I change the mDNS hostname?**  
A: Modify the `MDNSNamePrimary` constant in [`cmd/bifrost/main.go`](cmd/bifrost/main.go:39).

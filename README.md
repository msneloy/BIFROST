# BIFROST

**B**rowser **I**ntegrated **F**eed for **R**emote **O**bservation & **S**creen **T**ransmission

<p align="left">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue?v=2" alt="License"></a>
  <a href="https://github.com/MentorsNoakhali/BIFROST"><img src="https://img.shields.io/badge/platform-Linux-lightgrey?v=2" alt="Platform"></a>
  <a href="https://golang.org/"><img src="https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&v=2" alt="Go"></a>
  <a href="#codebase-structure--line-metrics"><img src="https://img.shields.io/badge/Go_LOC-3233-blue?v=2" alt="Go LOC"></a>
  <a href="#codebase-structure--line-metrics"><img src="https://img.shields.io/badge/Total_LOC-4027-orange?v=2" alt="Total LOC"></a>
</p>

BIFROST is a zero-configuration, lightweight **classroom screen broadcasting server** written in Go. It streams a presenter's desktop (video + system audio) directly to student web browsers over a local network (LAN) using low-latency WebRTC (VP8 + Opus).

No heavy client applications, browser extensions, or cloud dependencies are required. Launch the single static binary to start broadcasting, with a real-time Terminal User Interface (TUI) dashboard to monitor connected viewers and system telemetry.

---

## Key Features

- **Single Static Binary**: Built with Go, zero external dynamic runtime libraries needed (~12MB binary).
- **Synchronized WebRTC Streaming**: Sub-second latency desktop broadcasting with VP8 video and Opus audio.
- **Terminal User Interface (TUI)**: Interactive terminal dashboard built with `bubbletea` and `lipgloss` displaying system stats (CPU, RAM, Disk, GPU, Fan), client links, stream state, and real-time logs.
- **Automatic mDNS Discovery**: Broadcasts hostname `bifrost.local` over LAN via mDNS so viewers don't need to type IP addresses.
- **Zero-Install Viewer**: Pure HTML5/JS frontend embedded in the Go binary; viewers open `http://bifrost.local:8080`.
- **Fault-Tolerant Capture**: Automatic PipeWire node readiness polling and watchdog auto-retry loop for continuous screen capture stability.
- **Cross-Desktop Support**: Native support for Wayland (GNOME / Mutter ScreenCast D-Bus API) and X11.

---

## Codebase Structure & Line Metrics

### 📂 Directory & File Tree

```text
BIFROST/
├── .air.toml               # Air configuration for live-reloading during development
├── Makefile                # Build automation, testing, linting, release, & auto-LOC scripts
├── README.md               # Comprehensive project documentation & architecture guide
├── go.mod                  # Go module definition and dependencies (Pion WebRTC, Lipgloss, Bubbletea)
├── go.sum                  # Cryptographic checksums for Go dependencies
├── main.go                 # Application entry point, CLI orchestration, & auto-retry capture loop
└── internal/               # Core application packages
    ├── capture/            # Screen & audio capture management
    │   └── capture.go      # PipeWire readiness polling, Mutter D-Bus API, separate GStreamer processes
    ├── config/             # Configuration management
    │   ├── config.go       # Command-line flags parsing & default configuration settings
    │   └── config_test.go  # Unit tests for CLI flag parsing and defaults
    ├── mdns/               # Local network discovery
    │   └── mdns.go         # mDNS registration via avahi-publish for bifrost.local
    ├── server/             # HTTP server & web UI
    │   ├── player_page.go  # Embedded HTML5/JS WebRTC player page with click-to-unmute audio
    │   └── server.go       # HTTP routes, WebRTC SDP offer/answer handlers, ICE candidate polling
    ├── stats/              # System telemetry collection
    │   └── stats.go        # Linux /proc and /sys parser (CPU, RAM, Disk, GPU, Swap, Fan, Temp, Load)
    ├── tracker/            # Client state tracking
    │   ├── tracker.go      # Connected client tracking, bandwidth metrics, active sessions
    │   └── tracker_test.go # Unit tests for client connection tracking and timeout eviction
    ├── tui/                # Terminal User Interface
    │   ├── styles.go       # Color palette, Lipgloss styles, borders, and bar formatting
    │   └── tui.go          # Bubbletea dashboard (HUD, telemetry, client list, log feed)
    └── webrtc/             # WebRTC media streaming
        ├── manager.go      # Pion WebRTC PeerConnection manager (VP8/Opus tracks, ICE handling)
        └── rtp_receiver.go # Loopback UDP socket listener (:5004/:5005) & atomic packet counters
```

### 📊 Codebase Metrics

> [!NOTE]
> **Automatic Update**: Metrics below and badges at the top update automatically whenever `make loc`, `make build`, or `make release` is executed.

- **Go Codebase**: 3233 lines
- **Total Codebase**: 4027 lines

| Package / Module | File Path | Description | Lines of Code |
| ---------------- | --------- | ----------- | ------------- |
| **Main** | `main.go` | Entry point, CLI flags, capture loop, OS signal handling | 120 |
| **Capture** | `internal/capture/capture.go` | Wayland/Mutter D-Bus, PipeWire polling, GStreamer audio/video | 310 |
| **Config** | `internal/config/config.go` | CLI flag parser & runtime configuration struct | 98 |
| **Config Tests** | `internal/config/config_test.go` | Unit tests for configuration defaults and flags | 76 |
| **mDNS** | `internal/mdns/mdns.go` | Avahi mDNS publication for `bifrost.local` | 30 |
| **Server** | `internal/server/server.go` | HTTP routes, WebRTC SDP negotiation, ICE polling | 188 |
| **Player Page** | `internal/server/player_page.go` | Embedded HTML5/JS viewer, WebRTC client, click-to-unmute | 633 |
| **Stats** | `internal/stats/stats.go` | `/proc` & `/sys` parser for hardware telemetry | 391 |
| **Tracker** | `internal/tracker/tracker.go` | Client connection tracker & bandwidth estimator | 200 |
| **Tracker Tests** | `internal/tracker/tracker_test.go` | Unit tests for client session eviction & state | 152 |
| **TUI Styles** | `internal/tui/styles.go` | Lipgloss color palette, borders, & HUD styles | 94 |
| **TUI View** | `internal/tui/tui.go` | Bubbletea terminal dashboard & event loops | 604 |
| **WebRTC Manager**| `internal/webrtc/manager.go` | Pion PeerConnection management & track routing | 171 |
| **RTP Receiver** | `internal/webrtc/rtp_receiver.go` | UDP `:5004`/`:5005` socket listener & atomic packet counters | 166 |
| **Modules & Deps**| `go.mod`, `go.sum` | Go module definitions and checksum dependency locks | 180 |
| **Build & Dev** | `Makefile`, `.air.toml` | Build automation, live reload, release packaging, LOC script | 98 |
| **CI/CD & Config**| `.github/workflows/*`, `.goreleaser.yml`, `.gitignore` | GitHub Actions CI/CD workflows and release config | 131 |
| **Documentation** | `README.md` | Comprehensive user manual, Linux guides, architecture spec | 330 |

---

## System Requirements & Prerequisites

### Server Host Requirements
- **OS**: Linux (kernel 4.19+ recommended)
- **Architecture**: `x86_64` (amd64) or `aarch64` (arm64)
- **Display Server**: Wayland (GNOME/Mutter) or X11
- **Sound Server**: PipeWire (with `pipewire-pulse`) or PulseAudio
- **Runtime Dependencies**:
  - `gstreamer-1.0` (with VP8, Opus, and RTP plugins)
  - `pipewire` / `pw-cli` (for Wayland capture)
  - `avahi-daemon` / `avahi-utils` (for mDNS local domain resolution)

### Network Ports
| Port | Protocol | Purpose |
| ---- | -------- | ------- |
| `8080` | TCP | HTTP web server & WebRTC SDP signaling (configurable via `-port`) |
| `5004` | UDP | Local loopback RTP video stream (`gst-launch` → Go RTP receiver) |
| `5005` | UDP | Local loopback RTP audio stream (`gst-launch` → Go RTP receiver) |
| `5353` | UDP | mDNS multicast discovery (`bifrost.local`) |

---

## Distribution-Specific Installation Guides

### 1. Ubuntu / Debian / Pop!_OS / Linux Mint
```bash
# Update package index
sudo apt update

# Install GStreamer core, tools, and codec plugins
sudo apt install -y \
  gstreamer1.0-tools \
  gstreamer1.0-plugins-base \
  gstreamer1.0-plugins-good \
  gstreamer1.0-plugins-bad \
  gstreamer1.0-plugins-ugly

# Install PipeWire CLI tools and mDNS discovery daemon
sudo apt install -y pipewire-bin avahi-daemon avahi-utils
```

### 2. Fedora / RHEL / CentOS Stream / Rocky Linux
```bash
# Install GStreamer tools and plugins
sudo dnf install -y \
  gstreamer1 \
  gstreamer1-plugins-base \
  gstreamer1-plugins-good \
  gstreamer1-plugins-bad-free \
  gstreamer1-plugins-ugly-free

# Install PipeWire tools and Avahi mDNS
sudo dnf install -y pipewire-utils avahi avahi-tools

# Ensure Avahi daemon is active
sudo systemctl enable --now avahi-daemon
```

### 3. Arch Linux / Manjaro / EndeavourOS
```bash
# Install GStreamer, PipeWire, and Avahi packages
sudo pacman -S --needed \
  gstreamer \
  gst-plugins-base \
  gst-plugins-good \
  gst-plugins-bad \
  gst-plugins-ugly \
  pipewire \
  avahi

# Enable and start Avahi service for mDNS
sudo systemctl enable --now avahi-daemon
```

### 4. openSUSE (Leap / Tumbleweed)
```bash
# Install GStreamer stack, PipeWire tools, and Avahi
sudo zypper install -y \
  gstreamer \
  gstreamer-plugins-base \
  gstreamer-plugins-good \
  gstreamer-plugins-bad \
  gstreamer-plugins-ugly \
  pipewire-tools \
  avahi

# Start mDNS daemon
sudo systemctl enable --now avahi-daemon
```

---

## Quick Start

### 1. Download Pre-built Binary
Download the binary for your architecture from the [Releases](https://github.com/MentorsNoakhali/BIFROST/releases) page:

```bash
# Make binary executable
chmod +x bifrost-linux-amd64

# Run BIFROST
./bifrost-linux-amd64
```

### 2. Build from Source
Requirements: Go `1.22` or newer.

```bash
# Clone the repository
git clone https://github.com/MentorsNoakhali/BIFROST.git
cd BIFROST

# Build executable
make build

# Launch server with TUI
./bin/bifrost
```

---

## Command Line Usage

```text
bifrost [flags]

Flags:
  -port int
    	HTTP server port (default 8080)
  -resolution string
    	Capture resolution formatted as WxH (default "1920x1080")
  -fps int
    	Capture frame rate (default 30)
  -no-audio
    	Disable audio capture (video only)
  -headless
    	Run in headless mode without TUI (ideal for systemd/CI)
```

### Examples

```bash
# Custom port and 60 FPS
./bin/bifrost -port 9090 -fps 60

# Run in background / systemd without TUI
./bin/bifrost -headless -no-audio
```

### Interactive TUI Controls

| Key | Action |
| --- | ------ |
| `s` | Toggle stream state (Start / Stop capture) |
| `r` | Refresh TUI dashboard |
| `?` | Toggle help overlay |
| `q` | Exit BIFROST server cleanly |

---

## Architecture & Data Flow

### 1. High-Level Subsystem Diagram

```text
+---------------------------------------------------------------------------------------------------------------+
|                                                HOST LINUX SYSTEM                                              |
|                                                                                                               |
|  +-----------------------+     +-------------------------------+     +-------------------------------------+  |
|  |   DISPLAY SERVER      |     |     GSTREAMER VIDEO PIPELINE  |     |         GO RTP RECEIVER             |  |
|  |  (Wayland / X11)      |---->| pipewiresrc / ximagesrc       |---->| UDP Socket :5004                    |  |
|  |  - Mutter D-Bus API   |     | videoconvert ! vp8enc ! rtp   |     | atomic.Uint64 Video Packet Counter  |  |
|  +-----------------------+     +-------------------------------+     +------------------+------------------+  |
|                                                                                         |                     |
|                                                                                         v                     |  WebRTC PeerConnection  +------------------+
|                                                                              +---------------------+            |  (VP8 Video + Opus)   |                  |
|                                                                              |  Pion WebRTC Track  |------------+----------------------->| STUDENT BROWSERS |
|                                                                              |  (TrackLocalStatic) |            |                       |                  |
|                                                                              +---------------------+            |  HTTP / WebRTC SDP    | http://bifrost   |
|  +-----------------------+     +-------------------------------+                        ^                       |  Signaling & ICE      |       :8080      |
|  |     SOUND SERVER      |     |     GSTREAMER AUDIO PIPELINE  |                        |                       |--------+------------->|                  |
|  | (PipeWire / Pulse)    |---->| pulsesrc (monitor source)     |------------------------+                       |        |              +------------------+
|  |  - pactl source lookup|     | audioconvert ! opusenc ! rtp  |     UDP Socket :5005                           |        |
|  +-----------------------+     +-------------------------------+     atomic.Uint64 Audio Packet Counter         |        v
|                                                                                                               |  +----------------------------------+
|  +---------------------------------------------------------------------------------------------------------+  |  |      HTTP SERVER & WEB UI       |
|  |   SYSTEM TELEMETRY & TUI                                                                                |  |  | - Server: /webrtc/offer, /ice   |
|  |   - /proc & /sys metrics parser (CPU, RAM, Disk, GPU, Temp, Load)                                       |--|->| - Viewer: Embedded Player HTML5 |
|  |   - Bubbletea Elm-architecture TUI dashboard & live logger feed                                        |  |  | - Tracker: Bandwidth & sessions |
|  +---------------------------------------------------------------------------------------------------------+  |  +----------------------------------+
+---------------------------------------------------------------------------------------------------------------+
```

---

### 2. End-to-End Pipeline Specifications

#### A. Screen Capture Pipeline (`internal/capture`)
1. **Wayland D-Bus Session Negotiation**:
   - BIFROST calls `org.gnome.Mutter.ScreenCast.CreateSession` over D-Bus to establish a screen recording session.
   - Invokes `RecordMonitor` specifying the primary display monitor.
   - Starts the session via D-Bus and receives a PipeWire node ID (e.g., `node 123`).
2. **PipeWire Node Readiness Polling (`waitForPipeWireNode`)**:
   - To avoid race conditions where GStreamer attempts to bind before PipeWire negotiates stream formats (`stream error: target not found`), BIFROST polls `pw-cli info <nodeID>` every 200ms for up to 5 seconds.
   - Once confirmed ready in the registry, a 500ms stabilization delay occurs before spawning GStreamer.
3. **Video Encoding Pipeline**:
   - **Command**: `gst-launch-1.0 pipewiresrc path=<nodeID> do-timestamp=true ! videoconvert ! vp8enc deadline=1 cpu-used=8 target-bitrate=2000000 keyframe-max-dist=30 ! rtpvp8pay ! udpsink host=127.0.0.1 port=5004`
   - **X11 Fallback**: `ximagesrc use-damage=false ! videoconvert ! vp8enc ... ! udpsink host=127.0.0.1 port=5004`

#### B. Audio Capture Pipeline (`internal/capture`)
1. **Monitor Source Auto-Detection**:
   - Executes `pactl list short sources` to find the default system audio monitor source (e.g., `alsa_output.pci-0000_00_1b.0.iec958-stereo.monitor`).
2. **Clock Isolation (Independent GStreamer Process)**:
   - **Architecture Decision**: Running `pipewiresrc` (PipeWire clock) and `pulsesrc` (PulseAudio clock) in a single GStreamer pipeline causes internal GStreamer clock desynchronization, stalling audio output.
   - BIFROST launches audio as an independent `gst-launch-1.0` child process with its own dedicated GStreamer clock loop.
3. **Audio Encoding Pipeline**:
   - **Command**: `gst-launch-1.0 pulsesrc device=<monitor> ! audioconvert ! audioresample ! opusenc bitrate=64000 ! rtpopuspay ! udpsink host=127.0.0.1 port=5005`

#### C. RTP Loopback Ingestion & Metrics (`internal/webrtc`)
1. **UDP Socket Binding**:
   - `RTPReceiver` binds local loopback UDP sockets on `:5004` (video) and `:5005` (audio) with `SO_REUSEPORT` enabled.
2. **RTP Packet Forwarding**:
   - `readLoop` parses incoming UDP payloads into `rtp.Packet` structures.
   - Increments atomic packet counters (`videoPkts`, `audioPkts`) for system diagnostics.
   - Writes parsed RTP packets directly into Pion `TrackLocalStaticRTP` video/audio tracks.

#### D. WebRTC Signaling & Client Media Transport (`internal/server`, `internal/webrtc`)
1. **SDP Offer/Answer Exchange**:
   - Student browser requests `GET /` and downloads the embedded single-page player (`player_page.go`).
   - Browser creates an `RTCPeerConnection` (`recvonly`), generates an SDP Offer, and POSTs to `/webrtc/offer`.
   - `webrtc.Manager` attaches the active video (`VP8`) and audio (`Opus`) tracks, generates an SDP Answer, and returns it.
2. **ICE Candidate Exchange**:
   - Client sends local ICE candidates to `/ice` and polls remote server ICE candidates via `/ice/poll`.
3. **Autoplay Policy Handling**:
   - HTML5 `<video>` element starts playing in muted mode to comply with browser autoplay security policies.
   - An overlay prompt (**"🔊 Click to enable audio"**) appears when live tracks are attached.
   - A user click gesture on the stream area programmatically sets `video.muted = false`, enabling unmuted audio.

#### E. Resilience & Fault Tolerance (`main.go`)
1. **Capture Watchdog**:
   - Goroutines monitor child process lifecycles for both video and audio `gst-launch-1.0` commands.
2. **Auto-Retry Loop**:
   - If a screen capture pipeline fails (e.g. PipeWire node churn or display re-configuration), the watchdog transitions `streaming = false`.
   - The main orchestration loop in `main.go` automatically triggers a re-capture attempt every 3 seconds without operator intervention.

---

## Troubleshooting & FAQ

### 1. `address already in use` or Port Conflicts
If BIFROST or previous GStreamer instances were abruptly terminated, residual processes may hold ports `8080`, `5004`, or `5005`.
Kill orphaned instances with:
```bash
pkill -9 -f gst-launch; pkill -9 -f bifrost
```

### 2. Video Works, but No Audio
- **PulseAudio Monitor Source**: Verify system audio output is playing. Run `pactl list short sources` to confirm a `.monitor` device is present and `RUNNING`.
- **Browser Autoplay Policy**: Web browsers block auto-playing unmuting audio. When viewing the stream, click the **"🔊 Click to enable audio"** banner or anywhere on the video area.

### 3. Wayland Stream Errors (`target not found`)
BIFROST automatically polls `pw-cli info <nodeID>` before starting GStreamer. Ensure your PipeWire daemon is running correctly:
```bash
pw-cli info 0
```

---

## Developer Guide

```bash
make build       # Build production binary to ./bin/bifrost (auto-updates LOC in README)
make test        # Execute unit test suite
make lint        # Run go vet static code analysis
make run         # Build and run in headless mode
make tui         # Build and run interactively with full TUI
make dev         # Run with hot-reloading (requires air)
make release     # Cross-compile release binaries and auto-update LOC in README
make loc         # Explicitly recalculate and update LOC metrics in README.md
```

---

## License

Distributed under the MIT License. See `LICENSE` for details.

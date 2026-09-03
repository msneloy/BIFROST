# BIFROST

**B**rowser **I**ntegrated **F**eed for **R**emote **O**bservation & **S**creen **T**ransmission

[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Linux-lightgrey)](https://github.com/nelobster/BIFROST)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://golang.org/)

BIFROST is a zero-configuration, lightweight **classroom screen broadcasting server** written in Go. It streams a teacher's desktop (video + audio) to student browsers over a local network via WebRTC (VP8+Opus). No heavy desktop GUI or client software required — just launch the binary, and the TUI dashboard appears in your terminal.

---

## Features

- **Single Binary** — ~18MB static Go binary. Download and run.
- **Terminal Dashboard (TUI)** — Real-time interface showing system telemetry, connected clients, stream status, and log feed.
- **Synchronized WebRTC Audio/Video** — Low-latency VP8 video + Opus audio via separate GStreamer pipelines for reliable capture.
- **mDNS Network Registration** — Automatic LAN broadcast at `http://bifrost.local`.
- **Student View** — Browser-based viewer with WebRTC connection, live metrics, auto-reconnect, and click-to-unmute audio.
- **System Telemetry** — Real-time CPU, memory, disk, swap, GPU, fan, network, and load monitoring.
- **Auto-Retry Capture** — If the screen capture pipeline fails (e.g. PipeWire node disappears), BIFROST automatically retries every 3 seconds without operator intervention.
- **Pure Go Codebase** — No HTML, Python, or JavaScript files. Everything is Go.

---

## Quick Start

### Download (Recommended)

Download the latest binary from [Releases](https://github.com/msneloy/BIFROST/releases):

```bash
chmod +x bifrost-linux-amd64
./bifrost-linux-amd64
```

### Prerequisites

- **GStreamer** — Screen capture backend (pre-installed on GNOME)
  ```bash
  sudo apt install gstreamer1.0-tools gstreamer1.0-plugins-base gstreamer1.0-plugins-good gstreamer1.0-plugins-bad
  ```
- **avahi-utils** — mDNS for `bifrost.local` discovery (pre-installed on GNOME)
  ```bash
  sudo apt install avahi-daemon avahi-utils
  ```
- **PulseAudio / PipeWire** — System audio capture (usually pre-installed)

### Build from Source

```bash
git clone https://github.com/msneloy/BIFROST.git
cd BIFROST
make build
./bin/bifrost
```

---

## Usage

```
bifrost [OPTIONS]

Options:
  -fps int              Capture frame rate (default 30)
  -headless             Run without TUI (for scripts/CI)
  -no-audio             Disable audio capture
  -port int             HTTP server port (default 8080)
  -resolution string    Capture resolution WxH (default "1920x1080")
```

### TUI Controls

| Key | Action                               |
| --- | ------------------------------------ |
| `s` | Start/stop the screen capture stream |
| `r` | Refresh the display                  |
| `?` | Toggle help                          |
| `q` | Quit BIFROST                         |

---

## Architecture

```
bifrost (single Go binary)
├── main.go                 Entry point, auto-retry capture loop, graceful shutdown
├── internal/
│   ├── config/             CLI flags & config parsing
│   ├── capture/            Screen capture (GStreamer: ximagesrc / pipewiresrc + pulsesrc)
│   │                       Video and audio run as separate gst-launch-1.0 processes
│   │                       to avoid PipeWire/PulseAudio clock conflicts.
│   ├── server/             HTTP server, WebRTC signaling, embedded player page
│   ├── webrtc/             Pion WebRTC peer management (VP8 + Opus), RTP receiver
│   │                       with atomic packet counters for diagnostics
│   ├── tracker/            Client telemetry & bandwidth monitoring
│   ├── stats/              System metrics from /proc and /sys (CPU, mem, disk, GPU, fan)
│   ├── tui/                Terminal dashboard (bubbletea + lipgloss)
│   └── mdns/               mDNS registration via avahi-publish
```

### Audio Pipeline

Audio is captured separately from video to avoid GStreamer clock conflicts between
`pipewiresrc` (PipeWire clock) and `pulsesrc` (PulseAudio clock). Each runs as its
own `gst-launch-1.0` process:

- **Video**: `pipewiresrc → videoconvert → vp8enc → rtpvp8pay → UDP :5004`
- **Audio**: `pulsesrc (monitor) → opusenc → rtpopuspay → UDP :5005`

The Go RTP receiver reads both UDP streams and writes into `TrackLocalStaticRTP`
tracks for WebRTC delivery.

### Player Audio (Browser Autoplay Policy)

Browsers require a user gesture to unmute audio. BIFROST starts playback muted
(for instant autoplay) and shows a **"🔊 Click to enable audio"** overlay.
Clicking anywhere on the video unmutes audio.

---

## HTTP Endpoints

| Endpoint        | Method | Description             |
| --------------- | ------ | ----------------------- |
| `/`             | GET    | Student viewer (WebRTC) |
| `/ping`         | GET    | Client telemetry        |
| `/health`       | GET    | JSON health check       |
| `/webrtc/offer` | POST   | WebRTC SDP signaling    |
| `/ice`          | POST   | ICE candidate exchange  |
| `/ice/poll`     | GET    | Poll for ICE candidates |

---

## Development

```bash
make build       # Compile static binary
make test        # Run all tests
make lint        # Run go vet
make run         # Build & run in headless mode
make tui         # Build & run with full TUI (requires real terminal)
make dev         # Hot-reload in headless mode (requires air)
make release     # Build Linux amd64 + arm64 binaries
make help        # Show all targets
```

### Troubleshooting: Stale Processes

If you see `address already in use` or missing audio/video after restarting,
kill orphaned GStreamer processes from previous runs:

```bash
pkill -9 -f gst-launch; pkill -9 -f bifrost; sleep 1
```

### PipeWire Node Readiness

On Wayland/GNOME, BIFROST uses the Mutter ScreenCast D-Bus API to obtain a
PipeWire node ID. Before launching GStreamer, it polls `pw-cli info <nodeID>`
to confirm the node is registered in the PipeWire registry, preventing the
`stream error: target not found` race condition.

---

## License

MIT

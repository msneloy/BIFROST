# BIFROST

**B**rowser **I**ntegrated **F**eed for **R**emote **O**bservation & **S**creen **T**ransmission

[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Linux-lightgrey)](https://github.com/nelobster/BIFROST)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://golang.org/)

BIFROST is a zero-configuration, lightweight **classroom screen broadcasting server** written in Go. It streams a teacher's desktop (video + audio) to student browsers over a local network via WebRTC (VP8+Opus). No heavy desktop GUI or client software required — just launch the binary, and the TUI dashboard appears in your terminal.

---

## Features

- **Single Binary** — ~18MB static Go binary. Download and run.
- **Terminal Dashboard (TUI)** — bashtop-inspired interface showing system stats, connected clients, and stream status.
- **Synchronized WebRTC Audio/Video** — Low-latency VP8 video + Opus audio for synchronized classroom playback.
- **mDNS Network Registration** — Automatic LAN broadcast at `http://bifrost.local`.
- **Student View** — Browser-based viewer with WebRTC connection, live metrics, and auto-reconnect.
- **System Telemetry** — Real-time CPU, memory, disk, network, GPU, and battery monitoring.
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
  sudo apt install gstreamer1.0-tools gstreamer1.0-plugins-base gstreamer1.0-plugins-good
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

| Key | Action |
|-----|--------|
| `s` | Start/stop the screen capture stream |
| `r` | Refresh the display |
| `?` | Toggle help |
| `q` | Quit BIFROST |

---

## Architecture

```
bifrost (single Go binary)
├── main.go                 Entry point & orchestration
├── internal/
│   ├── config/             CLI flags & config parsing
│   ├── capture/            Screen capture (GStreamer: ximagesrc + pipewiresrc)
│   ├── server/             HTTP server, WebRTC signaling, player page
│   ├── webrtc/             Pion WebRTC peer management (VP8 + Opus)
│   ├── tracker/            Client telemetry & bandwidth monitoring
│   ├── stats/              System metrics from /proc and /sys
│   ├── tui/                Terminal dashboard (bubbletea + lipgloss)
│   └── mdns/               mDNS registration via avahi-publish
```

---

## HTTP Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | GET | Student viewer (WebRTC) |
| `/ping` | GET | Client telemetry |
| `/health` | GET | JSON health check |
| `/webrtc/offer` | POST | WebRTC SDP signaling |
| `/ice` | POST | ICE candidate exchange |
| `/ice/poll` | GET | Poll for ICE candidates |

---

## Development

```bash
make build       # Compile static binary
make test        # Run all tests
make lint        # Run go vet
make run         # Build & run in headless mode
make dev         # Run with hot-reload (requires air)
make release     # Build Linux amd64 + arm64 binaries
make help        # Show all targets
```

---

## License

MIT

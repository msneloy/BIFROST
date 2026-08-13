# BIFROST

**B**rowser **I**ntegrated **F**eed for **R**emote **O**bservation & **S**creen **T**ransmission

[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Linux-lightgrey)](https://github.com/nelobster/BIFROST)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://golang.org/)

BIFROST is a zero-configuration, lightweight **classroom screen broadcasting server** written in Go. It streams a teacher's desktop (video + audio) to student browsers over a local network via WebRTC (VP8+Opus) with automatic MJPEG fallback for 23+ concurrent students. No heavy desktop GUI or client software required — just launch the binary, and the admin interface opens directly in your browser.

---

## Features

- **Lightweight Pure-Go Binary** — ~16MB single binary, zero CGo GUI dependencies.
- **Browser-First Experience** — Auto-launches the admin panel at `http://localhost:8080/admin` upon startup.
- **Synchronized WebRTC Audio/Video** — Low-latency VP8 video + Opus audio stream for synchronized classroom playback.
- **mDNS Network Registration** — Automatic LAN broadcast over `http://bifrost.local:8080/watch`.
- **Teacher Admin Dashboard** — Built-in web panel with live preview, system telemetry, active WebRTC peer counts, and dynamic student QR code generation.
- **Headless Server Mode** — `--no-browser` flag for running BIFROST on headless servers or SSH sessions.
- **Client Telemetry & Guard** — Real-time tracking of IP, OS, browser, GPU, and battery stats.

---

## Quick Start

### Prerequisites

- **ffmpeg** — `sudo apt-get install ffmpeg`
- **PulseAudio / PipeWire** — For system audio capture
- **avahi-daemon & avahi-utils** (optional) — For mDNS discovery (`bifrost.local`)
- **python3 + GStreamer** (optional) — For Wayland capture (`mutter_capture.py`)

### Build & Run

```bash
# Clone the repo
git clone https://github.com/nelobster/BIFROST.git
cd BIFROST

# Build pure-Go binary (~16MB)
go build -o bifrost .

# Run (auto-launches admin panel in default browser)
./bifrost
```

### Options

```bash
bifrost [OPTIONS]

  --port PORT         HTTP server port (default: 8080)
  --fps FPS           Capture frame rate (default: 30)
  --quality Q         JPEG quality 1-100 (default: 40)
  --resolution WxH    Capture resolution (default: 1920x1080)
  --no-browser        Disable automatic browser launch on startup
  --no-audio          Disable audio capture
  --no-webrtc         Disable WebRTC (MJPEG only)
```

---

## Architecture

```
bifrost (Go binary)
├── main.go              Entry point, browser auto-launch & orchestration
├── embed_assets.go      go:embed for web assets
├── internal/
│   ├── config/          CLI flags & config parsing
│   ├── capture/         Wayland/X11 video & audio stream capture
│   ├── server/          HTTP server, WebRTC signaling & API handlers
│   ├── webrtc/          Pion WebRTC peer management (VP8 + Opus)
│   ├── tracker/         Client telemetry & bandwidth monitoring
│   ├── stats/           System metrics from /proc and /sys
│   └── mdns/            mDNS registration via avahi-publish
└── web/
    ├── player.html      Student WebRTC viewer (`/watch`)
    └── admin.html       Teacher Web Admin dashboard (`/admin`)
```

---

## HTTP Endpoints

| Endpoint        | Method | Description                                    |
| --------------- | ------ | ---------------------------------------------- |
| `/watch`        | GET    | Primary Student Viewer (WebRTC + MJPEG)        |
| `/admin`        | GET    | Teacher Admin Dashboard                        |
| `/stream`       | GET    | Multiplexed stream (`multipart/x-mixed-replace`) |
| `/frame`        | GET    | Single latest JPEG snapshot                    |
| `/audio`        | GET    | MP3 audio stream                               |
| `/ping`         | GET    | Client telemetry (latency, OS, browser, battery)|
| `/health`       | GET    | JSON health check                              |
| `/api/stats`    | GET    | System stats & WebRTC peer counts              |
| `/api/clients`  | GET    | Active connected client telemetry              |
| `/webrtc/offer` | POST   | WebRTC SDP signaling                           |

---

## Client Support

| Device  | Browser        | WebRTC | MJPEG |
| ------- | -------------- | ------ | ----- |
| Linux   | Chrome/Firefox/Brave | ✅ | ✅ |
| Android | Chrome         | ✅     | ✅    |
| Windows | Any            | ❌     | ❌    |

---

## License

MIT License

---

## FAQ

**Q: Why does BIFROST block Windows clients?**  
A: In a classroom environment, Windows devices often introduce compatibility issues. The guard ensures a consistent experience for Linux and Android users.

**Q: Why is my frame rate low under Wayland?**  
A: Wayland's security model prevents direct screen access. BIFROST uses GStreamer/PipeWire capture which may have lower throughput than X11's `x11grab`.

**Q: Do I need to run as root?**  
A: Only for privileged ports (below 1024). Port 8080 works without root, but screen capture often needs elevated permissions.

**Q: How does WebRTC work without a TURN server?**  
A: BIFROST uses Pion WebRTC for pure-Go WebRTC. On a LAN, host candidates are sufficient — no STUN/TURN server is needed.

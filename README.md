# BIFROST

**B**rowser **I**ntegrated **F**eed for **R**emote **O**bservation & **S**creen **T**ransmission

[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Linux-lightgrey)](https://github.com/nelobster/BIFROST)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://golang.org/)

BIFROST is a zero-configuration **classroom screen broadcasting server** written in Go. It streams a teacher's desktop (video + audio) to student browsers over a local network via MJPEG and WebRTC. No client software required — just open a browser.

---

## Features

- **Unified Capture** — Single ffmpeg process captures video + audio for perfect sync
- **Dual Streaming** — WebRTC (low-latency, VP8+Opus) with automatic MJPEG fallback
- **Web Viewer** — Browser-based viewer with HUD overlay, sync/refresh/fullscreen controls
- **Admin Dashboard** — Web-based control panel at `/admin` with live preview, system stats, client list
- **mDNS Discovery** — Automatic LAN registration via `avahi-publish` (`bifrost.local`)
- **TUI Dashboard** — Terminal dashboard fallback (`--headless`) with system stats
- **Client Tracking** — IP, hostname, MAC, bandwidth, OS, browser, GPU, battery
- **Windows Guard** — Automatic Windows client rejection with styled 403 page
- **Health Endpoint** — JSON health check at `/health`

---

## Quick Start

### Prerequisites

- **ffmpeg** — `sudo apt-get install ffmpeg`
- **PulseAudio** — For system audio capture (most Linux distros have this)
- **avahi-daemon & avahi-utils** (optional) — For mDNS discovery (`bifrost.local`)
- **python3 + GStreamer** (optional) — For Wayland capture

### Build & Run

```bash
# Clone the repo
git clone https://github.com/nelobster/BIFROST.git
cd BIFROST

# Build
go build -o bifrost .

# Run
./bifrost
```

### Options

```bash
bifrost [OPTIONS]

  --port PORT         HTTP server port (default: 8080)
  --fps FPS           Capture frame rate (default: 30)
  --quality Q         JPEG quality 1-100 (default: 40)
  --resolution WxH    Capture resolution (default: 1920x1080)
  --headless          Skip TUI dashboard
  --no-audio          Disable audio capture
  --no-webrtc         Disable WebRTC (MJPEG only)
```

---

## Architecture

```
bifrost (Go binary)
├── main.go              Entry point & orchestration
├── embed_assets.go      go:embed for web assets
├── internal/
│   ├── config/          CLI flags, config parsing
│   ├── capture/         Unified ffmpeg capture (video + audio + WebRTC RTP)
│   ├── server/          HTTP server & endpoint handlers
│   ├── webrtc/          Pion WebRTC peer management & RTP receiver
│   ├── tracker/         Client tracking & bandwidth monitoring
│   ├── stats/           System metrics from /proc and /sys
│   ├── dashboard/       Terminal TUI (--headless fallback)
│   └── mdns/            mDNS via avahi-publish
└── web/
    ├── player.html      Student viewer (WebRTC + MJPEG fallback)
    └── admin.html       Teacher admin dashboard
```

---

## HTTP Endpoints

| Endpoint        | Method | Description                                    |
| --------------- | ------ | ---------------------------------------------- |
| `/`             | GET    | Student viewer (WebRTC + MJPEG fallback)       |
| `/admin`        | GET    | Teacher admin dashboard                        |
| `/stream`       | GET    | MJPEG video stream (`multipart/x-mixed-replace`) |
| `/frame`        | GET    | Single latest JPEG frame                       |
| `/audio`        | GET    | MP3 audio stream                               |
| `/ping`         | GET    | Client telemetry (latency, OS, browser, GPU)   |
| `/health`       | GET    | JSON health check                              |
| `/stats`        | GET    | JSON system stats                              |
| `/api/clients`  | GET    | JSON client list (admin dashboard)             |
| `/api/stats`    | GET    | JSON system stats (admin dashboard)            |
| `/webrtc/offer` | POST   | WebRTC SDP signaling                           |

---

## TUI Dashboard

The terminal dashboard (BASHTOP-style, available with `--headless` or when no display server is running) displays:

```
╭── BIFROST v0.2.0 ── 192.168.1.100:8080 ── [12:34:56] ── uptime: 3d 12h ──╮
│                                                                             │
╭── SYSTEM ───────────────────────────────────────────────────────────────────╮
│ CPU: ██████░░░░░░░░░ 52%  3200MHz 45°C                                    │
│ RAM: ████░░░░░░░░░░░ 42%  6.7/16.0G                                       │
│ GPU: ░░░░░░░░░░░░░░░ --   0MHz 0°C                                        │
│ DISK: ███████████░░░░ 85%  213/256G                                        │
│ NIC: wlp2s0 866Mb/s WiFi                      SWAP: ░░░░░░░░ 5% 0.5/4.0G │
│ FAN: 1200 RPM                                 BAT:  85% Charging           │
╰─────────────────────────────────────────────────────────────────────────────╯
╭── STUDENTS (3 active) ─────────────────────────────────────────────────────╮
│ S │ # │ DEV │ IP ADDRESS     │ OS/BROWSER    │ BANDWIDTH      │ TOTAL     │
│ ● │ 1 │ 📱  │ 192.168.0.50  │ Linux/Chrome  │ 2.1M/s         │ 88.1M     │
│ ● │ 2 │ 💻  │ 192.168.0.51  │ Linux/Firefox │ 1.5M/s         │ 32.1M     │
│ ○ │ 3 │ 💻  │ 192.168.0.52  │ Android/Chrome│ 0.0M/s         │ 10.8M     │
╰─────────────────────────────────────────────────────────────────────────────╯
```

---

## Dependencies

### Required

| Package | Purpose | Install |
|---------|---------|---------|
| ffmpeg | Screen/audio capture | `apt install ffmpeg` |
| PulseAudio | System audio capture | Pre-installed on most desktop Linux |

### Optional

| Package | Purpose | Install |
|---------|---------|---------|
| avahi-daemon | mDNS discovery | `apt install avahi-daemon avahi-utils` |
| python3 + GStreamer | Wayland capture | `apt install python3 gir1.2-gst-1.0` |

---

## Deployment

### Manual

```bash
./bifrost              # With TUI dashboard
./bifrost --headless   # Without TUI (for SSH/headless)
```

---

## Client Support

| Device  | Browser        | WebRTC | MJPEG |
| ------- | -------------- | ------ | ----- |
| Linux   | Chrome/Firefox | ✅     | ✅    |
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

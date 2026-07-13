# BIFROST

**B**rowser **I**ntegrated **F**eed for **R**emote **O**bservation & **S**creen **T**ransmission

[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Linux-lightgrey)](https://github.com/nelobster/BIFROST)
[![Shell](https://img.shields.io/badge/shell-bash-green)](https://www.gnu.org/software/bash/)

BIFROST is a zero-configuration **classroom screen broadcasting server** written entirely in Bash. It streams a teacher's desktop (video + audio) to student browsers over a local network via MJPEG and WebRTC. No client software required — just open a browser.

---

## Features

- **Screen Capture** — Captures display via `ffmpeg` (X11) or GStreamer/PipeWire (Wayland)
- **Audio Streaming** — System audio via PulseAudio as MP3
- **Dual Streaming** — WebRTC (low-latency) with automatic MJPEG fallback
- **Web Viewer** — Browser-based viewer with HUD overlay, sync/refresh/fullscreen controls
- **mDNS Discovery** — Automatic LAN registration via `avahi-publish` (`bifrost.local`)
- **TUI Dashboard** — BASHTOP-inspired terminal dashboard with:
  - System stats (CPU, RAM, GPU, disk, swap, NIC, battery, fan, temperatures)
  - Connected students with bandwidth, device type, OS/browser
  - Rejected client log
- **Client Tracking** — IP, hostname, MAC, bandwidth, OS, browser, GPU, battery
- **Windows Guard** — Automatic Windows client rejection with styled 403 page
- **Health Endpoint** — JSON health check at `/health`
- **Debian Packaging** — Ready-to-install `.deb` with systemd service

---

## Quick Start

### Prerequisites

- **ffmpeg** — `sudo apt-get install ffmpeg`
- **socat** — `sudo apt-get install socat`
- **python3** — `sudo apt-get install python3`
- **avahi-daemon & avahi-utils** — `sudo apt-get install avahi-daemon avahi-utils`
- **mediamtx** (optional) — For WebRTC support

### Install & Run

```bash
# Clone the repo
git clone https://github.com/nelobster/BIFROST.git
cd BIFROST

# Install (copies to /opt/bifrost, creates /usr/local/bin/bifrost)
sudo bash scripts/install.sh

# Run
sudo bifrost
```

### Run from source (without install)

```bash
sudo bash bifrost.sh
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
bifrost.sh (main)
├── lib/capture.sh       ffmpeg screen/audio capture
├── lib/frame_splitter.py  MJPEG frame extraction
├── lib/stream.sh        socat HTTP server (port 8080)
├── lib/tracker.sh       client tracking (associative arrays)
├── lib/stats.sh         system metrics (/proc, /sys)
├── lib/dashboard.sh     BASHTOP-style TUI
├── lib/guard.sh         Windows rejection
├── lib/mdns.sh          avahi-publish
├── lib/webrtc.sh        mediamtx gateway (optional)
├── lib/common.sh        shared utilities
└── web/player.html      web viewer (WebRTC + MJPEG)
```

---

## HTTP Endpoints

| Endpoint    | Method | Description                                            |
| ----------- | ------ | ------------------------------------------------------ |
| `/`         | GET    | Web viewer HTML (WebRTC + MJPEG fallback)              |
| `/stream`   | GET    | MJPEG video stream (`multipart/x-mixed-replace`)       |
| `/frame`    | GET    | Single latest JPEG frame                               |
| `/audio`    | GET    | MP3 audio stream                                       |
| `/ping`     | GET    | Client telemetry (latency, OS, browser, GPU, etc.)     |
| `/health`   | GET    | JSON health check                                      |
| `/stats`    | GET    | JSON transmission stats                                |
| `/webrtc/offer` | POST | WebRTC SDP signaling (requires mediamtx)          |

---

## TUI Dashboard

The terminal dashboard (BASHTOP-style) displays:

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
| bash 4+ | Runtime | Pre-installed on all Linux |
| ffmpeg | Screen/audio capture | `apt install ffmpeg` |
| socat | HTTP server | `apt install socat` |
| python3 | Frame splitter + Wayland capture | `apt install python3` |
| avahi-daemon | mDNS | `apt install avahi-daemon avahi-utils` |

### Optional

| Package | Purpose | Install |
|---------|---------|---------|
| mediamtx | WebRTC gateway | See [mediamtx docs](https://github.com/bluenviron/mediamtx) |

---

## Deployment

### Debian Package

```bash
sudo bash scripts/install.sh
sudo systemctl enable bifrost
sudo systemctl start bifrost
```

### Manual

```bash
sudo bash bifrost.sh --headless &
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

**Q: What if mediamtx is not installed?**  
A: BIFROST gracefully falls back to MJPEG-only streaming. WebRTC provides lower latency but is not required.

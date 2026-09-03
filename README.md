# BIFROST

**B**rowser **I**ntegrated **F**eed for **R**emote **O**bservation & **S**creen **T**ransmission

[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Linux-lightgrey)](https://github.com/nelobster/BIFROST)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://golang.org/)

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
Download the binary for your architecture from the [Releases](https://github.com/msneloy/BIFROST/releases) page:

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
git clone https://github.com/msneloy/BIFROST.git
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

```text
               +---------------------------------------------------+
               |                    HOST SYSTEM                    |
               |                                                   |
 +----------+  | +------------------+     +----------------------+ |
 | Display  |--->| pipewiresrc/ximage|---->| VP8 Enc / RTP Pay    | |
 | (Wayland/|  | +------------------+     +----------------------+ |
 |   X11)   |  |                                     |             |
 +----------+  |                                UDP 5004           |
               |                                     v             |  WebRTC (VP8+Opus)  +-------------------+
 +----------+  | +------------------+     +----------------------+ |--SDP/ICE via HTTP--->|  Student Browser  |
 | System   |--->| pulsesrc (monitor)|---->| Opus Enc / RTP Pay   | |                    | http://bifrost:8080|
 | Audio    |  | +------------------+     +----------------------+ |                    +-------------------+
 +----------+  |                                     |             |
               |                                UDP 5005           |
               |                                     v             |
               |                        +------------------------+ |
               |                        | Go RTP Receiver (Pion) | |
               |                        +------------------------+ |
               +---------------------------------------------------+
```

1. **Screen Capture (`internal/capture`)**:
   - On Wayland: Negotiates session via GNOME Mutter D-Bus API (`org.gnome.Mutter.ScreenCast`), polls PipeWire registry (`pw-cli info`) for node readiness, and spawns `gst-launch-1.0` with `pipewiresrc`.
   - On X11: Spawns `gst-launch-1.0` with `ximagesrc`.
2. **Audio Capture (`internal/capture`)**:
   - Detects active PulseAudio / PipeWire monitor source (`pactl list short sources`).
   - Spawns a dedicated `gst-launch-1.0` process encoding audio to Opus and outputting to UDP `:5005`. Running audio in a separate process avoids GStreamer clock synchronization stalls between PipeWire and PulseAudio.
3. **WebRTC Delivery (`internal/webrtc`)**:
   - `RTPReceiver` listens on UDP `:5004` (video) and `:5005` (audio).
   - `Manager` handles client SDP offer/answer exchanges and streams WebRTC media tracks to connected browsers.
4. **Embedded Viewer (`internal/server`)**:
   - Serves an HTML5 player page with auto-reconnection and a user-gesture unmute prompt (`🔊 Click to enable audio`) adhering to browser autoplay policies.

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
make build       # Build production binary to ./bin/bifrost
make test        # Execute unit test suite
make lint        # Run go vet static code analysis
make run         # Build and run in headless mode
make tui         # Build and run interactively with full TUI
make dev         # Run with hot-reloading (requires air)
make release     # Cross-compile release binaries (linux-amd64, linux-arm64)
```

---

## License

Distributed under the MIT License. See `LICENSE` for details.

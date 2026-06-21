# Contributing to BIFROST

Thanks for your interest in contributing to BIFROST! This guide covers the development workflow, coding conventions, and submission process.

## Prerequisites

- **Go 1.22+**
- **ffmpeg** — `sudo apt-get install ffmpeg`
- **avahi-daemon & avahi-utils** — `sudo apt-get install avahi-daemon avahi-utils`
- **inotify-tools** — `sudo apt-get install inotify-tools` (for watch mode)
- **PipeWire** — `sudo apt-get install pipewire` (for audio capture)

## Getting Started

```bash
# Clone the repo
git clone https://github.com/nelobster/BIFROST.git
cd BIFROST

# Build
go build -o bifrost ./cmd/bifrost

# Run
./bifrost
```

### Development Watch Mode

```bash
./watch.sh
```

Auto-rebuilds and restarts BIFROST on any `.go` or `.html` file change.

## Project Structure

```
cmd/bifrost/main.go          — Application entrypoint & orchestration
internal/
  capture/capture.go         — Screen & audio capture (ffmpeg)
  dashboard/dashboard.go     — Terminal TUI dashboard with system stats
  guard/guard.go             — Windows client rejection middleware
  mdns/mdns.go               — mDNS service registration (avahi-publish)
  server/server.go           — HTTP server with routes & middleware
  stream/stream.go           — Publish/subscribe broadcast mechanism
  tracker/tracker.go         — Client tracking & bandwidth monitoring
  watcher/watcher.go         — Hot-reload file watcher for dev
web/
  web.go                     — Embedded HTML via go:embed
  templates/viewer.html      — Web viewer frontend (HTML/CSS/JS)
debian/                      — systemd service & .deb packaging
```

## Coding Conventions

### Go Style

- Follow standard Go conventions (`gofmt`, `go vet`).
- Use `camelCase` for unexported, `PascalCase` for exported identifiers.
- Keep functions focused and short. Prefer composition over inheritance.
- Handle errors explicitly — never silently discard them in production code.
- Use `sync.RWMutex` for concurrent map access (see `tracker`, `stream` packages).

### Package Design

- Each `internal/` package owns one concern (capture, tracking, streaming, etc.).
- Packages communicate through exported types and interfaces, not globals.
- The `stream.Broadcaster` is the central pub/sub mechanism — both video and audio use it.
- The `tracker.Tracker` is the source of truth for connected clients.

### Naming

- HTTP endpoints use lowercase: `/stream`, `/ping`, `/health`, `/stats`.
- Struct fields use PascalCase with short, descriptive names (`BatPct`, `DevType`).
- File names match the primary type or function: `capture.go`, `stream.go`, `tracker.go`.

### HTML/CSS/JS

- The viewer (`web/templates/viewer.html`) is embedded via `//go:embed`.
- Keep styling inline (no external CSS/JS dependencies).
- Use CSS custom properties and transitions for HUD elements.
- JavaScript is vanilla ES6+ — no frameworks.

## Testing

```bash
# Run all tests
go test ./...

# Run tests in a specific package
go test ./internal/capture/
go test ./internal/server/
```

- Tests live alongside source files (`*_test.go`).
- Use table-driven tests where appropriate.
- Mock external commands (`ffmpeg`, `avahi-publish`) where feasible.
- Test both success paths and error handling.

## Submitting Changes

### Branching

- Create a feature branch from `main`: `git checkout -b feature/my-change`
- Keep commits focused — one logical change per commit.

### Commit Messages

Use a clear, imperative subject line:

```
Add GPU temperature to dashboard
Fix frame boundary parsing in capture loop
Reject Windows clients at middleware layer
```

### Pull Request Checklist

- [ ] Code compiles without warnings: `go build ./...`
- [ ] Tests pass: `go test ./...`
- [ ] Code is formatted: `gofmt -l .` (no output)
- [ ] No unused imports or variables
- [ ] README.md updated if public API or behavior changed
- [ ] New features include tests where practical

### What to Include in a PR

- **What** changed and **why**.
- Screenshots or terminal output for UI/dashboard changes.
- Any new dependencies or system requirements.

## Reporting Issues

Open an issue with:

1. **Steps to reproduce** — what you did, what happened.
2. **Expected behavior** — what you expected.
3. **Environment** — OS, desktop session (X11/Wayland), Go version, ffmpeg version.
4. **Logs** — relevant output from the terminal or `/tmp/bifrost.log`.

## Code of Conduct

Be respectful and constructive. We're building a tool for classrooms — kindness is the baseline.

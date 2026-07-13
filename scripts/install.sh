#!/bin/bash
# bifrost-install.sh — Universal BIFROST installer
# Installs BIFROST to /opt/bifrost and creates /usr/local/bin/bifrost wrapper.

set -euo pipefail

if [[ "$EUID" -ne 0 ]]; then
    echo "Please run as root: sudo bash install.sh"
    exit 1
fi

INSTALL_DIR="/opt/bifrost"
BIN_LINK="/usr/local/bin/bifrost"
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

echo "Installing BIFROST to $INSTALL_DIR..."

# Create directories
mkdir -p "$INSTALL_DIR/lib" "$INSTALL_DIR/web" "$INSTALL_DIR/scripts"

# Copy files
cp "$SCRIPT_DIR/bifrost.sh" "$INSTALL_DIR/"
cp "$SCRIPT_DIR/lib/"*.sh "$INSTALL_DIR/lib/"
cp "$SCRIPT_DIR/lib/"*.py "$INSTALL_DIR/lib/" 2>/dev/null || true
cp "$SCRIPT_DIR/web/"*.html "$INSTALL_DIR/web/"
cp "$SCRIPT_DIR/scripts/"*.py "$INSTALL_DIR/scripts/" 2>/dev/null || true

# Make executable
chmod +x "$INSTALL_DIR/bifrost.sh"
chmod +x "$INSTALL_DIR/lib/"*.sh
chmod +x "$INSTALL_DIR/lib/"*.py 2>/dev/null || true
chmod +x "$INSTALL_DIR/scripts/"*.py 2>/dev/null || true

# Create wrapper symlink
ln -sf "$INSTALL_DIR/bifrost.sh" "$BIN_LINK"

# Enable avahi-daemon
if command -v systemctl &>/dev/null; then
    systemctl enable avahi-daemon 2>/dev/null || true
    systemctl start avahi-daemon 2>/dev/null || true
fi

echo ""
echo "--------------------------------------------"
echo "BIFROST v0.2.0 installed successfully!"
echo ""
echo "Run anywhere: bifrost"
echo "Or directly:  /opt/bifrost/bifrost.sh"
echo "--------------------------------------------"
echo ""
echo "Quick start:"
echo "  sudo bifrost              # with TUI dashboard"
echo "  sudo bifrost --headless   # without dashboard"
echo "  sudo bifrost --no-webrtc  # MJPEG only"

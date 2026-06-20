#!/bin/bash
# bifrost-install.sh
# Universal, self-contained BIFROST installer.
# No internet, no package manager required.

# Check for root
if [ "$EUID" -ne 0 ]; then
  echo "Please run as root (sudo bash install.sh)"
  exit 1
fi

INSTALL_DIR="/opt/bifrost"
BIN_DIR="/usr/local/bin"
LIB_DIR="$INSTALL_DIR/lib"
GST_DIR="$INSTALL_DIR/gstreamer-1.0"

echo "Installing BIFROST to $INSTALL_DIR..."

mkdir -p "$INSTALL_DIR" "$LIB_DIR" "$GST_DIR" "$INSTALL_DIR/bin"

# Extract embedded archive (if it exists)
# In the final build, the tarball will be appended after exit
ARCHIVE_START=$(awk '/^__ARCHIVE__/{print NR+1; exit}' "$0")
if [ -n "$ARCHIVE_START" ]; then
    tail -n +"$ARCHIVE_START" "$0" | tar -xz -C "$INSTALL_DIR" 2>/dev/null || echo "Info: No embedded archive found (yet)."
fi

# Copy local vendor files if we are running in the repo
if [ -d "vendor" ]; then
    cp -r vendor/lib/* "$LIB_DIR/" 2>/dev/null
    cp -r vendor/gstreamer-1.0/* "$GST_DIR/" 2>/dev/null
    cp -r vendor/bin/* "$INSTALL_DIR/bin/" 2>/dev/null
fi

# Install bifrost binary from current dir if exists
if [ -f "bifrost" ]; then
    cp "bifrost" "$INSTALL_DIR/bin/bifrost-bin"
else
    echo "Warning: binary 'bifrost' not found in current directory."
fi

# Create wrapper script
cat > "$BIN_DIR/bifrost" << EOF
#!/bin/bash
export GST_PLUGIN_PATH=$GST_DIR
export GST_PLUGIN_SYSTEM_PATH=$GST_DIR
export LD_LIBRARY_PATH=$LIB_DIR
export PATH=$INSTALL_DIR/bin:\$PATH
exec $INSTALL_DIR/bin/bifrost-bin "\$@"
EOF
chmod +x "$BIN_DIR/bifrost"

# Register libraries
echo "$LIB_DIR" > /etc/ld.so.conf.d/bifrost.conf
ldconfig

# Enable avahi-daemon
if command -v systemctl &>/dev/null; then
    systemctl enable avahi-daemon 2>/dev/null || true
    systemctl start avahi-daemon 2>/dev/null || true
fi

echo "------------------------------------------------"
echo "BIFROST version 0.1.0 installed successfully!"
echo "Run it anywhere using the command: bifrost"
echo "------------------------------------------------"

exit 0
__ARCHIVE__

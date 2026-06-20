#!/bin/bash
# scripts/bundle_deps.sh
# Collects all runtime dependencies for BIFROST bundling.

set -e

REPO_ROOT=$(pwd)
VENDOR_DIR="$REPO_ROOT/vendor"
LIB_DIR="$VENDOR_DIR/lib"
GST_DIR="$VENDOR_DIR/gstreamer-1.0"
BIN_DIR="$VENDOR_DIR/bin"

mkdir -p "$LIB_DIR" "$GST_DIR" "$BIN_DIR"

echo "Collecting GStreamer core and plugins..."

# Common plugin paths
GST_SYS_DIRS=(
    "/usr/lib/gstreamer-1.0"
    "/usr/lib64/gstreamer-1.0"
    "/usr/lib/x86_64-linux-gnu/gstreamer-1.0"
)

GST_SYS_DIR=""
for d in "${GST_SYS_DIRS[@]}"; do
    if [ -d "$d" ]; then
        GST_SYS_DIR="$d"
        break
    fi
done

if [ -z "$GST_SYS_DIR" ]; then
    echo "Error: GStreamer plugins directory not found."
    exit 1
fi

# Plugins we need
PLUGINS=(
    libgstximagesrc.so
    libgstpipewire.so
    libgstvaapi.so
    libgstx264.so
    libgstopus.so
    libgstisomp4.so
    libgstpulseaudio.so
    libgstautodetect.so
    libgstvideoconvert.so
    libgstvideoscale.so
    libgstvideoconvertscale.so
    libgstvideoparsersbad.so
    libgstaudioconvert.so
    libgstaudioresample.so
    libgstvideorate.so
    libgstapp.so
    libgstcoreelements.so
    libgsttypefindfunctions.so
    libgstplayback.so
)

for p in "${PLUGINS[@]}"; do
    if [ -f "$GST_SYS_DIR/$p" ]; then
        cp "$GST_SYS_DIR/$p" "$GST_DIR/"
    else
        echo "Warning: Plugin $p not found."
    fi
done

echo "Collecting shared libraries..."

# Helper to copy dependencies of a file
copy_deps() {
    local target="$1"
    ldd "$target" | awk '{if ($3 != "") print $3}' | while read -r lib; do
        if [[ $lib == /usr/* ]] || [[ $lib == /lib/* ]]; then
            cp -n "$lib" "$LIB_DIR/" 2>/dev/null || true
        fi
    done
}

# Copy deps of all plugins
for p in "$GST_DIR"/*.so; do
    copy_deps "$p"
done

# Copy avahi-publish
AVAHI=$(which avahi-publish || echo "")
if [ -n "$AVAHI" ]; then
    cp "$AVAHI" "$BIN_DIR/avahi-publish-bin"
    copy_deps "$AVAHI"
    
    # Create wrapper for avahi-publish to use bundled libs
    cat > "$BIN_DIR/avahi-publish" << EOF
#!/bin/bash
export LD_LIBRARY_PATH=$LIB_DIR
exec $BIN_DIR/avahi-publish-bin "\$@"
EOF
    chmod +x "$BIN_DIR/avahi-publish"
fi

# Copy VAAPI libs
cp /usr/lib/libva* "$LIB_DIR/" 2>/dev/null || true

echo "Bundling complete! Files are in vendor/"
echo "Now run: go build -o bifrost ./cmd/bifrost"
echo "Then: cat scripts/install.sh <(tar -cz -C vendor .) > bifrost-installer.sh"

#!/bin/bash
# launch.sh for Platypus (macOS Terminal launcher)

APP_DIR="$(cd "$(dirname "$0")/../Resources" && pwd)"
APP_BIN="$APP_DIR/yeet-tube"

# Make sure the Go binary is executable
chmod +x "$APP_BIN"

# Use AppleScript to tell Terminal.app to open and run your app
osascript <<EOF
tell application "Terminal"
    activate
    do script "$APP_BIN"
end tell
EOF


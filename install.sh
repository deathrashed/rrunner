#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_NAME="Rrunner"
APP_PATH="/Applications/${APP_NAME}.app"
BIN_DIR="$HOME/.local/bin"
BRIDGE_SRC="$REPO_DIR/bin/rrunner"
BRIDGE_DEST="$BIN_DIR/rrunner"

mkdir -p "$BIN_DIR"

chmod +x "$REPO_DIR/bin/rrunner" \
         "$REPO_DIR/bin/rrunner.sh" \
         "$REPO_DIR/bin/md-restore.sh"

# Install bridge only if source and destination are not already the same file.
if [[ -e "$BRIDGE_DEST" ]] && [[ "$(realpath "$BRIDGE_SRC")" == "$(realpath "$BRIDGE_DEST")" ]]; then
  echo "Bridge already installed: $BRIDGE_DEST"
else
  install -m 755 "$BRIDGE_SRC" "$BRIDGE_DEST"
  echo "Installed bridge: $BRIDGE_DEST"
fi

rm -rf "$APP_PATH"

osacompile -o "$APP_PATH" "$REPO_DIR/app/Rrunner.applescript"

mkdir -p "$APP_PATH/Contents/Resources"
cp "$REPO_DIR/app/Rrunner.icns" "$APP_PATH/Contents/Resources/Rrunner.icns"

INFO_PLIST="$APP_PATH/Contents/Info.plist"

/usr/libexec/PlistBuddy -c "Set :CFBundleName Rrunner" "$INFO_PLIST" 2>/dev/null || \
/usr/libexec/PlistBuddy -c "Add :CFBundleName string Rrunner" "$INFO_PLIST"

/usr/libexec/PlistBuddy -c "Set :CFBundleDisplayName Rrunner" "$INFO_PLIST" 2>/dev/null || \
/usr/libexec/PlistBuddy -c "Add :CFBundleDisplayName string Rrunner" "$INFO_PLIST"

/usr/libexec/PlistBuddy -c "Set :CFBundleIdentifier com.deathrashed.Rrunner" "$INFO_PLIST" 2>/dev/null || \
/usr/libexec/PlistBuddy -c "Add :CFBundleIdentifier string com.deathrashed.Rrunner" "$INFO_PLIST"

/usr/libexec/PlistBuddy -c "Set :CFBundleIconFile Rrunner" "$INFO_PLIST" 2>/dev/null || \
/usr/libexec/PlistBuddy -c "Add :CFBundleIconFile string Rrunner" "$INFO_PLIST"

# Replace URL scheme registration.
/usr/libexec/PlistBuddy -c "Delete :CFBundleURLTypes" "$INFO_PLIST" 2>/dev/null || true
/usr/libexec/PlistBuddy -c "Add :CFBundleURLTypes array" "$INFO_PLIST"
/usr/libexec/PlistBuddy -c "Add :CFBundleURLTypes:0 dict" "$INFO_PLIST"
/usr/libexec/PlistBuddy -c "Add :CFBundleURLTypes:0:CFBundleURLName string Rrunner URL Scheme" "$INFO_PLIST"
/usr/libexec/PlistBuddy -c "Add :CFBundleURLTypes:0:CFBundleURLSchemes array" "$INFO_PLIST"
/usr/libexec/PlistBuddy -c "Add :CFBundleURLTypes:0:CFBundleURLSchemes:0 string rrunner" "$INFO_PLIST"

/usr/bin/plutil -lint "$INFO_PLIST" >/dev/null

# Refresh Launch Services registration.
/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister \
  -f "$APP_PATH" >/dev/null 2>&1 || true

echo "Installed $APP_PATH"
echo "Installed bridge at $BRIDGE_DEST"
echo
echo "Test with:"
echo "open 'rrunner://open?url=file:///Users/rd/Scripts/Riley/rrunner/README.md'"

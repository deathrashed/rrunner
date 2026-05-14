#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_NAME="Rrunner"
APP_PATH="/Applications/${APP_NAME}.app"
BRIDGE_DIR="$HOME/.local/bin"

mkdir -p "$BRIDGE_DIR"

install -m 755 "$ROOT/bin/rrunner" "$BRIDGE_DIR/rrunner"

if [[ -d "$APP_PATH" ]]; then
  rm -rf "$APP_PATH"
fi

osacompile -o "$APP_PATH" "$ROOT/app/Rrunner.applescript"

mkdir -p "$APP_PATH/Contents/Resources"
cp "$ROOT/app/Rrunner.icns" "$APP_PATH/Contents/Resources/Rrunner.icns"

PLIST="$APP_PATH/Contents/Info.plist"

/usr/libexec/PlistBuddy -c "Set :CFBundleIdentifier com.deathrashed.Rrunner" "$PLIST" 2>/dev/null || /usr/libexec/PlistBuddy -c "Add :CFBundleIdentifier string com.deathrashed.Rrunner" "$PLIST"
/usr/libexec/PlistBuddy -c "Set :CFBundleName Rrunner" "$PLIST" 2>/dev/null || /usr/libexec/PlistBuddy -c "Add :CFBundleName string Rrunner" "$PLIST"
/usr/libexec/PlistBuddy -c "Set :CFBundleDisplayName Rrunner" "$PLIST" 2>/dev/null || /usr/libexec/PlistBuddy -c "Add :CFBundleDisplayName string Rrunner" "$PLIST"
/usr/libexec/PlistBuddy -c "Set :CFBundleIconFile Rrunner.icns" "$PLIST" 2>/dev/null || /usr/libexec/PlistBuddy -c "Add :CFBundleIconFile string Rrunner.icns" "$PLIST"

/usr/libexec/PlistBuddy -c "Delete :CFBundleURLTypes" "$PLIST" 2>/dev/null || true
/usr/libexec/PlistBuddy -c "Add :CFBundleURLTypes array" "$PLIST"
/usr/libexec/PlistBuddy -c "Add :CFBundleURLTypes:0 dict" "$PLIST"
/usr/libexec/PlistBuddy -c "Add :CFBundleURLTypes:0:CFBundleURLName string Rrunner URL" "$PLIST"
/usr/libexec/PlistBuddy -c "Add :CFBundleURLTypes:0:CFBundleURLSchemes array" "$PLIST"
/usr/libexec/PlistBuddy -c "Add :CFBundleURLTypes:0:CFBundleURLSchemes:0 string rrunner" "$PLIST"

/usr/bin/plutil -lint "$PLIST" >/dev/null

# Register the URL handler with Launch Services.
open "$APP_PATH" >/dev/null 2>&1 || true

cat <<EOF
Installed Rrunner.

App:
  $APP_PATH

Bridge:
  $BRIDGE_DIR/rrunner

URL scheme:
  rrunner://

Try:
  open 'rrunner://launch?app=Ghostty'
EOF

#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_NAME="Rrunner"
APP_PATH="/Applications/${APP_NAME}.app"
BIN_DIR="$HOME/.local/bin"
LIB_DIR="$HOME/.local/lib/rrunner"
BRIDGE_SRC="$REPO_DIR/bin/rrunner"
BRIDGE_DEST="$BIN_DIR/rrunner"
CORE_DEST="$LIB_DIR/rrunner-core"

DRY_RUN=0
CORE_ONLY=0
SKIP_APP=0
VALIDATE_ONLY=0

usage() {
  cat <<'EOF'
Usage: ./install.sh [options]

Options:
  --dry-run        Print planned install steps without writing files.
  --core-only      Build/validate only the Go core; skip bridge and app install.
  --skip-app       Install/build the core and bridge, but do not rebuild Rrunner.app.
  --validate-only  Validate the source/core commands without installing.
  --help           Show this help.
EOF
}

for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=1 ;;
    --core-only) CORE_ONLY=1 ;;
    --skip-app) SKIP_APP=1 ;;
    --validate-only) VALIDATE_ONLY=1 ;;
    --help|-h) usage; exit 0 ;;
    *) echo "Unknown option: $arg" >&2; usage >&2; exit 2 ;;
  esac
done

say() { echo "$*"; }
run() {
  if [[ "$DRY_RUN" == "1" ]]; then
    printf '+ '
    printf '%q ' "$@"
    printf '\n'
  else
    "$@"
  fi
}

validate_source() {
  say "Validating source scripts"
  run bash -n "$REPO_DIR/bin/rrunner" "$REPO_DIR/bin/rrunner.sh" "$REPO_DIR/install.sh" "$REPO_DIR/bin/md-restore.sh"
  if command -v go >/dev/null 2>&1; then
    say "Validating Go core"
    run go test "$REPO_DIR/cmd/rrunner-core"
    run go build -o /tmp/rrunner-core "$REPO_DIR/cmd/rrunner-core"
    if [[ "$DRY_RUN" == "0" ]]; then
      /tmp/rrunner-core --version >/dev/null
      /tmp/rrunner-core --list-actions --json >/dev/null
    fi
  else
    say "Warning: Go not found; Go core validation skipped." >&2
  fi
}

if [[ "$VALIDATE_ONLY" == "1" ]]; then
  validate_source
  exit 0
fi

if [[ "$DRY_RUN" == "1" ]]; then
  say "Dry run: no files will be installed or modified."
fi

run mkdir -p "$BIN_DIR" "$LIB_DIR"

run chmod +x "$REPO_DIR/bin/rrunner" \
             "$REPO_DIR/bin/rrunner.sh" \
             "$REPO_DIR/bin/md-restore.sh"

if command -v go >/dev/null 2>&1; then
  say "Building Go core: $CORE_DEST"
  run go build -o "$CORE_DEST" "$REPO_DIR/cmd/rrunner-core"
  if [[ "$DRY_RUN" == "0" ]]; then
    "$CORE_DEST" --version >/dev/null
    "$CORE_DEST" --list-actions --json >/dev/null
    if ! "$CORE_DEST" --validate-install --json >/dev/null; then
      echo "Warning: Go core installed, but diagnostics reported config/plugin issues." >&2
      "$CORE_DEST" --validate-install || true
    fi
  fi
else
  say "Warning: Go not found; installing shell fallback only." >&2
  say "Install Go and rerun ./install.sh to enable the plugin backend." >&2
fi

if [[ "$CORE_ONLY" == "1" ]]; then
  say "Core-only mode complete."
  exit 0
fi

# Install bridge only if source and destination are not already the same file.
if [[ "$DRY_RUN" == "0" && -e "$BRIDGE_DEST" && "$(realpath "$BRIDGE_SRC")" == "$(realpath "$BRIDGE_DEST")" ]]; then
  say "Bridge already installed: $BRIDGE_DEST"
else
  run install -m 755 "$BRIDGE_SRC" "$BRIDGE_DEST"
  say "Installed bridge: $BRIDGE_DEST"
fi

if [[ "$SKIP_APP" == "1" ]]; then
  say "Skipped app install: $APP_PATH"
  exit 0
fi

run rm -rf "$APP_PATH"
run osacompile -o "$APP_PATH" "$REPO_DIR/app/Rrunner.applescript"

run mkdir -p "$APP_PATH/Contents/Resources"
run cp "$REPO_DIR/app/Rrunner.icns" "$APP_PATH/Contents/Resources/Rrunner.icns"

INFO_PLIST="$APP_PATH/Contents/Info.plist"

run /usr/libexec/PlistBuddy -c "Set :CFBundleName Rrunner" "$INFO_PLIST" 2>/dev/null || \
run /usr/libexec/PlistBuddy -c "Add :CFBundleName string Rrunner" "$INFO_PLIST"

run /usr/libexec/PlistBuddy -c "Set :CFBundleDisplayName Rrunner" "$INFO_PLIST" 2>/dev/null || \
run /usr/libexec/PlistBuddy -c "Add :CFBundleDisplayName string Rrunner" "$INFO_PLIST"

run /usr/libexec/PlistBuddy -c "Set :CFBundleIdentifier com.deathrashed.Rrunner" "$INFO_PLIST" 2>/dev/null || \
run /usr/libexec/PlistBuddy -c "Add :CFBundleIdentifier string com.deathrashed.Rrunner" "$INFO_PLIST"

run /usr/libexec/PlistBuddy -c "Set :CFBundleIconFile Rrunner" "$INFO_PLIST" 2>/dev/null || \
run /usr/libexec/PlistBuddy -c "Add :CFBundleIconFile string Rrunner" "$INFO_PLIST"

# Replace URL scheme registration.
run /usr/libexec/PlistBuddy -c "Delete :CFBundleURLTypes" "$INFO_PLIST" 2>/dev/null || true
run /usr/libexec/PlistBuddy -c "Add :CFBundleURLTypes array" "$INFO_PLIST"
run /usr/libexec/PlistBuddy -c "Add :CFBundleURLTypes:0 dict" "$INFO_PLIST"
run /usr/libexec/PlistBuddy -c "Add :CFBundleURLTypes:0:CFBundleURLName string Rrunner URL Scheme" "$INFO_PLIST"
run /usr/libexec/PlistBuddy -c "Add :CFBundleURLTypes:0:CFBundleURLSchemes array" "$INFO_PLIST"
run /usr/libexec/PlistBuddy -c "Add :CFBundleURLTypes:0:CFBundleURLSchemes:0 string rrunner" "$INFO_PLIST"

run /usr/bin/plutil -lint "$INFO_PLIST" >/dev/null

# Refresh Launch Services registration.
run /System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister \
  -f "$APP_PATH" >/dev/null 2>&1 || true

say "Installed $APP_PATH"
say "Installed bridge at $BRIDGE_DEST"
if [[ "$DRY_RUN" == "1" || -x "$CORE_DEST" ]]; then
  say "Installed Go core at $CORE_DEST"
fi
say
say "Test with:"
say "open 'rrunner://open?url=file:///Users/rd/Scripts/Riley/rrunner/README.md'"

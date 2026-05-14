#!/usr/bin/env bash
set -euo pipefail

export PATH="/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"

RAW_URL="${1:-}"
if [[ -z "$RAW_URL" ]]; then
  echo "Usage: rrunner.sh 'rrunner://action?url=file:///path'" >&2
  exit 2
fi

RESTORE_URL="${RRUNNER_RESTORE_URL:-https://raw.githubusercontent.com/deathrashed/rrunner/main/bin/md-restore.sh}"

notify() {
  /usr/bin/osascript - "$1" <<'APPLESCRIPT' >/dev/null 2>&1 || true
on run argv
  display notification (item 1 of argv) with title "Rrunner"
end run
APPLESCRIPT
}

alert() {
  /usr/bin/osascript - "$1" <<'APPLESCRIPT' >/dev/null 2>&1 || true
on run argv
  display alert "Rrunner" message (item 1 of argv)
end run
APPLESCRIPT
}

url_decode() {
  /usr/bin/python3 - "$1" <<'PY'
import sys, urllib.parse
print(urllib.parse.unquote(sys.argv[1]))
PY
}

file_url_to_path() {
  local value="$1"
  /usr/bin/python3 - "$value" <<'PY'
import sys, urllib.parse
u = sys.argv[1]
p = urllib.parse.urlparse(u)
if p.scheme == "file":
    print(urllib.parse.unquote(p.path))
else:
    print(urllib.parse.unquote(u))
PY
}

query_value() {
  local key="$1"
  /usr/bin/python3 - "$RAW_URL" "$key" <<'PY'
import sys, urllib.parse
url, key = sys.argv[1], sys.argv[2]
p = urllib.parse.urlparse(url)
q = urllib.parse.parse_qs(p.query, keep_blank_values=True)
print(q.get(key, [""])[0])
PY
}

action_from_url() {
  /usr/bin/python3 - "$RAW_URL" <<'PY'
import sys, urllib.parse
p = urllib.parse.urlparse(sys.argv[1])
host = p.netloc or ""
path = p.path.strip("/")
if host:
    print(host)
else:
    print(path.split("/")[0] if path else "")
PY
}

payload_path() {
  local url_value path_value
  url_value="$(query_value url)"
  path_value="$(query_value path)"
  if [[ -n "$url_value" ]]; then
    file_url_to_path "$url_value"
  elif [[ -n "$path_value" ]]; then
    url_decode "$path_value"
  else
    printf ''
  fi
}

quote_for_shell() {
  /usr/bin/python3 - "$1" <<'PY'
import sys, shlex
print(shlex.quote(sys.argv[1]))
PY
}

open_app_with_file() {
  local app="$1"
  local path="$2"
  if [[ -z "$app" ]]; then
    open "$path"
    return
  fi
  if [[ "$app" == *.* ]]; then
    open -b "$app" "$path"
  else
    open -a "$app" "$path"
  fi
}

launch_app() {
  local app="$1"
  if [[ -z "$app" ]]; then
    alert "No app parameter supplied."
    exit 1
  fi
  if [[ "$app" == *.* ]]; then
    open -b "$app"
  else
    open -a "$app"
  fi
}

ghostty_command() {
  local cmd="$1"
  local tmpdir tmp
  tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/rrunner.XXXXXX")"
  tmp="$tmpdir/run.zsh"
  cat > "$tmp" <<EOF
#!/bin/zsh
clear
echo "Rrunner"
echo
$cmd
status=\$?
echo
echo "Exit status: \$status"
echo
echo "Press Return to close..."
read -r _
EOF
  chmod +x "$tmp"

  if command -v ghostty >/dev/null 2>&1; then
    ghostty -e /bin/zsh "$tmp" >/dev/null 2>&1 &
  else
    open -a Ghostty "$tmp" >/dev/null 2>&1 || open -a Terminal "$tmp" >/dev/null 2>&1
  fi
}

run_with() {
  local runner="$1"
  local path="$2"
  if [[ ! -e "$path" ]]; then
    alert "File does not exist: $path"
    exit 1
  fi

  local q
  q="$(quote_for_shell "$path")"

  case "$runner" in
    osascript) ghostty_command "osascript $q" ;;
    bash)      ghostty_command "bash $q" ;;
    zsh)       ghostty_command "zsh $q" ;;
    python)    ghostty_command "python3 $q" ;;
    node)      ghostty_command "node $q" ;;
    ruby)      ghostty_command "ruby $q" ;;
    perl)      ghostty_command "perl $q" ;;
    *)
      alert "Unknown runner: $runner"
      exit 1
      ;;
  esac
}

restore_md() {
  local md="$1"
  if [[ ! -f "$md" ]]; then
    alert "Markdown wrapper does not exist: $md"
    exit 1
  fi

  local cache_dir restore_file
  cache_dir="${XDG_CACHE_HOME:-$HOME/Library/Caches}/Rrunner"
  restore_file="$cache_dir/md-restore.sh"
  mkdir -p "$cache_dir"

  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$RESTORE_URL" -o "$restore_file.tmp"
    chmod +x "$restore_file.tmp"
    mv "$restore_file.tmp" "$restore_file"
  fi

  if [[ ! -x "$restore_file" ]]; then
    alert "Could not fetch md-restore.sh and no cached copy exists."
    exit 1
  fi

  local q
  q="$(quote_for_shell "$md")"
  ghostty_command "bash $(quote_for_shell "$restore_file") restore $q"
}

auto_run() {
  local path="$1"
  local lower="${path##*.}"
  lower="$(printf '%s' "$lower" | tr '[:upper:]' '[:lower:]')"

  case "$lower" in
    applescript|scpt) run_with osascript "$path" ;;
    sh|bash|command)  run_with bash "$path" ;;
    zsh)              run_with zsh "$path" ;;
    py|python)        run_with python "$path" ;;
    js|mjs|cjs)       run_with node "$path" ;;
    rb)               run_with ruby "$path" ;;
    pl|pm)            run_with perl "$path" ;;
    md|markdown)      restore_md "$path" ;;
    *)
      alert "No auto-run handler for: $path"
      exit 1
      ;;
  esac
}

ACTION="$(action_from_url)"
PATH_PAYLOAD="$(payload_path)"
APP_PAYLOAD="$(query_value app)"

case "$ACTION" in
  open)
    [[ -n "$PATH_PAYLOAD" ]] || { alert "No file url/path supplied."; exit 1; }
    open "$PATH_PAYLOAD"
    ;;
  reveal|show)
    [[ -n "$PATH_PAYLOAD" ]] || { alert "No file url/path supplied."; exit 1; }
    open -R "$PATH_PAYLOAD"
    ;;
  openwith|view)
    [[ -n "$PATH_PAYLOAD" ]] || { alert "No file url/path supplied."; exit 1; }
    open_app_with_file "$APP_PAYLOAD" "$PATH_PAYLOAD"
    ;;
  launch)
    launch_app "$APP_PAYLOAD"
    ;;
  auto)
    [[ -n "$PATH_PAYLOAD" ]] || { alert "No file url/path supplied."; exit 1; }
    auto_run "$PATH_PAYLOAD"
    ;;
  osascript|bash|zsh|python|node|ruby|perl)
    [[ -n "$PATH_PAYLOAD" ]] || { alert "No file url/path supplied."; exit 1; }
    run_with "$ACTION" "$PATH_PAYLOAD"
    ;;
  restore)
    [[ -n "$PATH_PAYLOAD" ]] || { alert "No Markdown url/path supplied."; exit 1; }
    restore_md "$PATH_PAYLOAD"
    ;;
  *)
    alert "Unknown Rrunner action: $ACTION"
    exit 1
    ;;
esac

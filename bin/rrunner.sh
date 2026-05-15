#!/usr/bin/env bash
set -euo pipefail

export PATH="/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"

CORE="${RRUNNER_CORE:-$HOME/.local/lib/rrunner/rrunner-core}"
if [[ "${RRUNNER_DISABLE_GO_CORE:-0}" != "1" && -x "$CORE" ]]; then
  exec "$CORE" "$@"
fi

RAW_URL="${1:-}"
if [[ -z "$RAW_URL" ]]; then
  echo "Usage: rrunner.sh [--diagnose|--list-actions|--dry-run] 'rrunner://action?url=file:///path'" >&2
  exit 2
fi

# Defaults. Local config can override these.
RRUNNER_TERMINAL_APP="${RRUNNER_TERMINAL_APP:-Ghostty}"
RRUNNER_KEEP_OPEN="${RRUNNER_KEEP_OPEN:-1}"
RRUNNER_REMOTE_BASE="${RRUNNER_REMOTE_BASE:-https://raw.githubusercontent.com/deathrashed/rrunner/main}"
RRUNNER_RESTORE_URL="${RRUNNER_RESTORE_URL:-}"
RRUNNER_HANDLERS_DIR="${RRUNNER_HANDLERS_DIR:-$HOME/.config/rrunner/handlers}"
RRUNNER_CONFIG_TOML="${RRUNNER_CONFIG_TOML:-$HOME/.config/rrunner/config.toml}"

parse_toml_exports() {
  local mode="$1"
  local file="$2"
  local action="${3:-}"

  [[ -f "$file" ]] || return 0

  /usr/bin/python3 - "$mode" "$file" "$action" <<'PY'
import ast
import re
import shlex
import sys

mode, path, wanted_action = sys.argv[1], sys.argv[2], sys.argv[3]

SECTION_RE = re.compile(r'^\s*\[([^\]]+)\]\s*(?:#.*)?$')
KEY_RE = re.compile(r'^\s*([A-Za-z0-9_-]+)\s*=\s*(.*?)\s*$')


def strip_comment(value):
    quote = None
    escaped = False
    out = []
    for ch in value:
        if quote:
            out.append(ch)
            if escaped:
                escaped = False
            elif ch == '\\':
                escaped = True
            elif ch == quote:
                quote = None
        else:
            if ch in ('"', "'"):
                quote = ch
                out.append(ch)
            elif ch == '#':
                break
            else:
                out.append(ch)
    return ''.join(out).strip()


def parse_value(raw):
    value = strip_comment(raw)
    if not value:
        return ''
    if value[0] in ('"', "'"):
        try:
            return ast.literal_eval(value)
        except Exception:
            return value.strip('"\'')
    lower = value.lower()
    if lower in ('true', 'false'):
        return '1' if lower == 'true' else '0'
    return value


def parse_section(raw):
    parts = [part.strip() for part in raw.split('.')]
    cleaned = []
    for part in parts:
        if len(part) >= 2 and part[0] in ('"', "'") and part[-1] == part[0]:
            try:
                part = ast.literal_eval(part)
            except Exception:
                part = part[1:-1]
        cleaned.append(part)
    return tuple(cleaned)


data = {}
section = ()
try:
    with open(path, 'r', encoding='utf-8') as handle:
        for line in handle:
            stripped = line.strip()
            if not stripped or stripped.startswith('#'):
                continue
            sec = SECTION_RE.match(line)
            if sec:
                section = parse_section(sec.group(1))
                data.setdefault(section, {})
                continue
            key = KEY_RE.match(line)
            if key and section:
                data.setdefault(section, {})[key.group(1).replace('-', '_')] = parse_value(key.group(2))
except OSError:
    sys.exit(0)


def emit(name, value):
    if value is None or value == '':
        return
    print(f'{name}={shlex.quote(str(value))}')

if mode == 'settings':
    settings = data.get(('settings',), {})
    mapping = {
        'terminal_app': 'RRUNNER_TERMINAL_APP',
        'keep_open': 'RRUNNER_KEEP_OPEN',
        'remote_base': 'RRUNNER_REMOTE_BASE',
        'remote_url': 'RRUNNER_REMOTE_URL',
        'restore_url': 'RRUNNER_RESTORE_URL',
        'handlers_dir': 'RRUNNER_HANDLERS_DIR',
        'text_editor': 'RRUNNER_TEXT_EDITOR',
        'code_editor': 'RRUNNER_CODE_EDITOR',
        'markdown_previewer': 'RRUNNER_MARKDOWN_PREVIEWER',
    }
    for key, env_name in mapping.items():
        emit(env_name, settings.get(key))
elif mode == 'action':
    action = data.get(('actions', wanted_action), {})
    if action:
        emit('RRUNNER_TOML_ACTION_FOUND', '1')
        for key in ('type', 'app', 'runner', 'script', 'command'):
            emit(f'RRUNNER_TOML_ACTION_{key.upper()}', action.get(key))
PY
}

parse_legacy_config_exports() {
  local file="$1"
  /usr/bin/python3 - "$file" <<'PY'
import ast, re, shlex, sys
allowed = {
    'RRUNNER_REMOTE_BASE', 'RRUNNER_REMOTE_URL', 'RRUNNER_RESTORE_URL',
    'RRUNNER_HANDLERS_DIR', 'RRUNNER_TERMINAL_APP', 'RRUNNER_KEEP_OPEN'
}
for line in open(sys.argv[1], encoding='utf-8', errors='ignore'):
    m = re.match(r'^\s*(RRUNNER_[A-Z0-9_]+)\s*=\s*(.*?)\s*(?:#.*)?$', line)
    if not m or m.group(1) not in allowed:
        continue
    value = m.group(2).strip()
    if value and value[0] in ('"', "'"):
        try:
            value = ast.literal_eval(value)
        except Exception:
            value = value.strip('"\'')
    print(f'{m.group(1)}={shlex.quote(str(value))}')
PY
}

# Load optional legacy shell config first for compatibility.
# For safety, parse only allowlisted RRUNNER_* assignments by default.
for cfg in "/etc/rrunner.conf" "$HOME/.config/rrunner/config"; do
  if [[ -f "$cfg" ]]; then
    if [[ "${RRUNNER_ALLOW_LEGACY_SOURCE:-0}" == "1" ]]; then
      # shellcheck disable=SC1090
      source "$cfg"
    else
      eval "$(parse_legacy_config_exports "$cfg")"
    fi
  fi
done

# Load the preferred single-file TOML config last so it wins.
if [[ -f "$RRUNNER_CONFIG_TOML" ]]; then
  eval "$(parse_toml_exports settings "$RRUNNER_CONFIG_TOML")"
fi
RRUNNER_RESTORE_URL="${RRUNNER_RESTORE_URL:-$RRUNNER_REMOTE_BASE/bin/md-restore.sh}"

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

expand_path() {
  /usr/bin/python3 - "$1" <<'PY'
import os
import sys
print(os.path.expandvars(os.path.expanduser(sys.argv[1])))
PY
}

configured_env_exports() {
  printf 'export RRUNNER_ACTION=%s\n' "$(quote_for_shell "$ACTION")"
  printf 'export RRUNNER_URL=%s\n' "$(quote_for_shell "$RAW_URL")"
  printf 'export RRUNNER_PATH=%s\n' "$(quote_for_shell "$PATH_PAYLOAD")"
  printf 'export RRUNNER_APP=%s\n' "$(quote_for_shell "$APP_PAYLOAD")"
}

require_payload() {
  if [[ -z "$PATH_PAYLOAD" ]]; then
    alert "No file url/path supplied."
    exit 1
  fi
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

run_command_in_terminal() {
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
if [[ "${RRUNNER_KEEP_OPEN}" == "1" ]]; then
  echo
  echo "Press Return to close..."
  read -r _
fi
exit \$status
EOF
  chmod +x "$tmp"

  if command -v ghostty >/dev/null 2>&1; then
    ghostty -e /bin/zsh "$tmp" >/dev/null 2>&1 &
  else
    open -a "$RRUNNER_TERMINAL_APP" "$tmp" >/dev/null 2>&1 || open -a Terminal "$tmp" >/dev/null 2>&1
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
    osascript) run_command_in_terminal "osascript $q" ;;
    bash)      run_command_in_terminal "bash $q" ;;
    zsh)       run_command_in_terminal "zsh $q" ;;
    python)    run_command_in_terminal "python3 $q" ;;
    node)      run_command_in_terminal "node $q" ;;
    ruby)      run_command_in_terminal "ruby $q" ;;
    perl)      run_command_in_terminal "perl $q" ;;
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
    curl -fsSL "$RRUNNER_RESTORE_URL" -o "$restore_file.tmp"
    chmod +x "$restore_file.tmp"
    mv "$restore_file.tmp" "$restore_file"
  fi

  if [[ ! -x "$restore_file" ]]; then
    alert "Could not fetch md-restore.sh and no cached copy exists."
    exit 1
  fi

  local q
  q="$(quote_for_shell "$md")"
  run_command_in_terminal "bash $(quote_for_shell "$restore_file") restore $q"
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

validate_runner() {
  local runner="$1"
  case "$runner" in
    osascript|bash|zsh|python|node|ruby|perl) return 0 ;;
    *) return 1 ;;
  esac
}

run_configured_script() {
  local script="$1"
  local runner="${2:-}"
  script="$(expand_path "$script")"

  if [[ ! -e "$script" ]]; then
    alert "Configured script does not exist: $script"
    exit 1
  fi

  if [[ -n "$runner" ]] && ! validate_runner "$runner"; then
    alert "Configured action '$ACTION' has unsupported script runner: $runner"
    exit 1
  fi

  local script_q path_arg env_exports command_line
  script_q="$(quote_for_shell "$script")"
  path_arg=""
  if [[ -n "$PATH_PAYLOAD" ]]; then
    path_arg=" $(quote_for_shell "$PATH_PAYLOAD")"
  fi
  env_exports="$(configured_env_exports)"

  if [[ -n "$runner" ]]; then
    command_line="$runner $script_q$path_arg"
  else
    command_line="$script_q$path_arg"
  fi

  run_command_in_terminal "$env_exports
$command_line"
}

run_configured_command() {
  local cmd="$1"
  local env_exports
  env_exports="$(configured_env_exports)"
  notify "Running configured command for rrunner://$ACTION"
  run_command_in_terminal "$env_exports
$cmd"
}

run_toml_action_if_present() {
  RRUNNER_TOML_ACTION_FOUND=""
  RRUNNER_TOML_ACTION_TYPE=""
  RRUNNER_TOML_ACTION_APP=""
  RRUNNER_TOML_ACTION_RUNNER=""
  RRUNNER_TOML_ACTION_SCRIPT=""
  RRUNNER_TOML_ACTION_COMMAND=""

  if [[ -f "$RRUNNER_CONFIG_TOML" ]]; then
    eval "$(parse_toml_exports action "$RRUNNER_CONFIG_TOML" "$ACTION")"
  fi

  [[ "$RRUNNER_TOML_ACTION_FOUND" == "1" ]] || return 0

  local type app runner script command
  type="$(printf '%s' "$RRUNNER_TOML_ACTION_TYPE" | tr '[:upper:]' '[:lower:]')"
  app="${RRUNNER_TOML_ACTION_APP:-$APP_PAYLOAD}"
  runner="$RRUNNER_TOML_ACTION_RUNNER"
  script="$RRUNNER_TOML_ACTION_SCRIPT"
  command="$RRUNNER_TOML_ACTION_COMMAND"

  case "$type" in
    open)
      require_payload
      open "$PATH_PAYLOAD"
      ;;
    reveal|show)
      require_payload
      open -R "$PATH_PAYLOAD"
      ;;
    openwith|view)
      require_payload
      open_app_with_file "$app" "$PATH_PAYLOAD"
      ;;
    launch)
      launch_app "$app"
      ;;
    auto)
      require_payload
      auto_run "$PATH_PAYLOAD"
      ;;
    restore)
      require_payload
      restore_md "$PATH_PAYLOAD"
      ;;
    run)
      require_payload
      if [[ -z "$runner" ]]; then
        alert "Configured action '$ACTION' has type=run but no runner."
        exit 1
      fi
      if ! validate_runner "$runner"; then
        alert "Configured action '$ACTION' has unsupported runner: $runner"
        exit 1
      fi
      run_with "$runner" "$PATH_PAYLOAD"
      ;;
    script)
      if [[ -z "$script" ]]; then
        alert "Configured action '$ACTION' has type=script but no script."
        exit 1
      fi
      run_configured_script "$script" "$runner"
      ;;
    command)
      if [[ -z "$command" ]]; then
        alert "Configured action '$ACTION' has type=command but no command."
        exit 1
      fi
      run_configured_command "$command"
      ;;
    *)
      alert "Configured action '$ACTION' has unknown type: $type"
      exit 1
      ;;
  esac

  exit 0
}

run_custom_handler_if_present() {
  local action="$1"
  local handler="$(expand_path "$RRUNNER_HANDLERS_DIR")/$action"

  if [[ -x "$handler" ]]; then
    export RRUNNER_ACTION="$action"
    export RRUNNER_URL="$RAW_URL"
    export RRUNNER_PATH="$PATH_PAYLOAD"
    export RRUNNER_APP="$APP_PAYLOAD"
    "$handler"
    exit $?
  fi
}

ACTION="$(action_from_url)"
PATH_PAYLOAD="$(payload_path)"
APP_PAYLOAD="$(query_value app)"

# Preferred TOML config actions can override or add actions.
run_toml_action_if_present

# Legacy executable handlers can override or add actions.
run_custom_handler_if_present "$ACTION"

case "$ACTION" in
  open)
    require_payload
    open "$PATH_PAYLOAD"
    ;;
  reveal|show)
    require_payload
    open -R "$PATH_PAYLOAD"
    ;;
  openwith|view)
    require_payload
    open_app_with_file "$APP_PAYLOAD" "$PATH_PAYLOAD"
    ;;
  launch)
    launch_app "$APP_PAYLOAD"
    ;;
  auto)
    require_payload
    auto_run "$PATH_PAYLOAD"
    ;;
  osascript|bash|zsh|python|node|ruby|perl)
    require_payload
    run_with "$ACTION" "$PATH_PAYLOAD"
    ;;
  restore)
    require_payload
    restore_md "$PATH_PAYLOAD"
    ;;
  *)
    alert "Unknown Rrunner action: $ACTION"
    exit 1
    ;;
esac

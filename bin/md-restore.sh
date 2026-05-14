#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat >&2 <<'USAGE'
Usage:
  md-restore.sh restore [--force] /path/to/wrapper.md
  md-restore.sh info /path/to/wrapper.md

Restores the embedded original file from a Markdown wrapper containing:
  source_file: "/absolute/path/to/original"
  <!-- ORIGINAL_FILE_BASE64_BEGIN -->
  ...
  <!-- ORIGINAL_FILE_BASE64_END -->
USAGE
}

cmd="${1:-}"
shift || true

force=0
if [[ "${1:-}" == "--force" ]]; then
  force=1
  shift
fi

md="${1:-}"
[[ -n "$cmd" && -n "$md" ]] || { usage; exit 2; }

if [[ ! -f "$md" ]]; then
  echo "Markdown wrapper not found: $md" >&2
  exit 1
fi

source_file="$(
  awk '
    /^source_file:[[:space:]]*/ {
      sub(/^source_file:[[:space:]]*/, "", $0)
      gsub(/^"/, "", $0)
      gsub(/"$/, "", $0)
      print
      exit
    }
  ' "$md"
)"

if [[ -z "$source_file" ]]; then
  echo "No source_file frontmatter field found in: $md" >&2
  exit 1
fi

case "$cmd" in
  info)
    echo "Wrapper: $md"
    echo "Source:  $source_file"
    if [[ -e "$source_file" ]]; then
      echo "Status:  original exists"
    else
      echo "Status:  original missing"
    fi
    exit 0
    ;;
  restore)
    ;;
  *)
    usage
    exit 2
    ;;
esac

if [[ -e "$source_file" && "$force" -ne 1 ]]; then
  echo "Original already exists; not overwriting:"
  echo "$source_file"
  echo
  echo "Use --force to overwrite."
  exit 0
fi

payload="$(
  awk '
    /ORIGINAL_FILE_BASE64_BEGIN/ {capture=1; next}
    /ORIGINAL_FILE_BASE64_END/ {capture=0}
    capture {print}
  ' "$md" | sed '/^[[:space:]]*$/d'
)"

if [[ -z "$payload" ]]; then
  echo "No embedded Base64 payload found in: $md" >&2
  exit 1
fi

mkdir -p "$(dirname "$source_file")"

tmp="$(mktemp "${TMPDIR:-/tmp}/mdrestore.XXXXXX")"
printf '%s\n' "$payload" > "$tmp"

if base64 --decode "$tmp" > "$source_file" 2>/dev/null; then
  :
elif base64 -D -i "$tmp" -o "$source_file" 2>/dev/null; then
  :
else
  rm -f "$tmp"
  echo "Failed to decode embedded Base64 payload." >&2
  exit 1
fi

rm -f "$tmp"
echo "Restored: $source_file"

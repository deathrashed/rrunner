#!/usr/bin/env bash
set -u

# Finder-selection conversion helpers for the Riley Rrunner plugin.
# The action is chosen from RRUNNER_ACTION, so the same script can back
# multiple rrunner:// actions.

export PATH="/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"

ACTION="${RRUNNER_ACTION:-${1:-}}"
created=0
converted=0
skipped=0
failed=0
deleted=0

notify() {
  /usr/bin/osascript - "$1" <<'APPLESCRIPT' >/dev/null 2>&1 || true
on run argv
  display notification (item 1 of argv) with title "Rrunner Finder Convert"
end run
APPLESCRIPT
}

alert() {
  /usr/bin/osascript - "$1" <<'APPLESCRIPT' >/dev/null 2>&1 || true
on run argv
  display alert "Rrunner Finder Convert" message (item 1 of argv)
end run
APPLESCRIPT
}

selection() {
  if [[ -n "${RRUNNER_PATH:-}" ]]; then
    printf '%s\n' "$RRUNNER_PATH"
    return
  fi
  /usr/bin/osascript <<'APPLESCRIPT'
tell application "Finder"
  set selectedItems to selection
  if selectedItems is {} then return ""
  set outputText to ""
  repeat with selectedItem in selectedItems
    set outputText to outputText & POSIX path of (selectedItem as alias) & linefeed
  end repeat
  return outputText
end tell
APPLESCRIPT
}

lower_ext() {
  local base ext
  base="$(basename "$1")"
  [[ "$base" == *.* ]] || { printf '\n'; return; }
  ext="${base##*.}"
  printf '%s\n' "$(printf '%s' "$ext" | tr '[:upper:]' '[:lower:]')"
}

unique_path() {
  local out="$1" dir base stem ext n
  if [[ ! -e "$out" ]]; then
    printf '%s\n' "$out"
    return
  fi
  dir="$(dirname "$out")"
  base="$(basename "$out")"
  ext=""
  stem="$base"
  if [[ "$base" == *.* ]]; then
    stem="${base%.*}"
    ext=".${base##*.}"
  fi
  n=2
  while [[ -e "$dir/$stem $n$ext" ]]; do
    n=$((n + 1))
  done
  printf '%s\n' "$dir/$stem $n$ext"
}

file_url_for_path() {
  python3 - "$1" <<'PY'
import pathlib, sys
print(pathlib.Path(sys.argv[1]).resolve().as_uri())
PY
}

url_encode() {
  python3 - "$1" <<'PY'
import sys, urllib.parse
print(urllib.parse.quote(sys.argv[1], safe=""))
PY
}

rrunner_link() {
  local action="$1" file_url="$2"
  printf 'rrunner://%s?url=%s\n' "$action" "$file_url"
}

rrunner_openwith() {
  local app="$1" file_url="$2"
  printf 'rrunner://openwith?app=%s&url=%s\n' "$(url_encode "$app")" "$file_url"
}

language_for_ext() {
  case "$1" in
    applescript|scpt) echo applescript ;;
    sh|bash|zsh|command) echo bash ;;
    py|python) echo python ;;
    rb) echo ruby ;;
    pl|pm) echo perl ;;
    js|mjs|cjs) echo javascript ;;
    ts|mts|cts) echo typescript ;;
    json) echo json ;;
    yaml|yml) echo yaml ;;
    toml) echo toml ;;
    xml) echo xml ;;
    html|htm) echo html ;;
    css) echo css ;;
    md|markdown) echo markdown ;;
    *) echo text ;;
  esac
}

finder_items() {
  selection | while IFS= read -r item; do
    [[ -n "$item" ]] && printf '%s\0' "$item"
  done
}

walk_files() {
  local item
  while IFS= read -r -d '' item; do
    if [[ -d "$item" ]]; then
      /usr/bin/find "$item" -type f -print0
    elif [[ -f "$item" ]]; then
      printf '%s\0' "$item"
    else
      skipped=$((skipped + 1))
    fi
  done < <(finder_items)
}

convert_scpt_to_applescript() {
  local file ext dir base stem out
  while IFS= read -r -d '' file; do
    ext="$(lower_ext "$file")"
    if [[ "$ext" != "scpt" ]]; then
      skipped=$((skipped + 1)); continue
    fi
    dir="$(dirname "$file")"; base="$(basename "$file")"; stem="${base%.*}"
    out="$(unique_path "$dir/$stem.applescript")"
    if /usr/bin/osadecompile "$file" > "$out" 2>/dev/null; then
      converted=$((converted + 1))
    else
      failed=$((failed + 1)); rm -f "$out"
    fi
  done < <(walk_files)
  notify "Converted $converted .scpt file(s). Skipped: $skipped. Failed: $failed."
  (( failed == 0 ))
}

write_markdown_wrapper() {
  local file base stem ext lang md source_url md_url fence
  file="$1"; base="$(basename "$file")"; stem="${base%.*}"; [[ "$stem" == "$base" ]] && stem="$base"
  ext="$(lower_ext "$file")"; lang="$(language_for_ext "$ext")"
  md="$(unique_path "$(dirname "$file")/$stem.md")"
  source_url="$(file_url_for_path "$file")"; md_url="$(file_url_for_path "$md")"; fence='```'
  {
    printf '%s\n' '---'
    printf 'title: "%s"\n' "${stem//\"/\\\"}"
    printf 'language: %s\n' "$lang"
    printf 'source_file: "%s"\n' "${file//\"/\\\"}"
    printf 'source_url: "%s"\n' "$source_url"
    printf '%s\n\n' '---'
    printf '# %s\n\n' "$stem"
    printf '> **Path:** `%s`  \n' "$file"
    printf '> **Original file:** [%s](<%s>)\n\n' "$base" "$source_url"
    printf '## Actions\n\n'
    printf '> These links use [Rrunner](https://github.com/deathrashed/rrunner).  \n\n'
    printf -- '- **Original:** [Open](<%s>) · [Reveal](<%s>)\n' "$(rrunner_link open "$source_url")" "$(rrunner_link reveal "$source_url")"
    printf -- '- **Restore:** [Restore original from this Markdown](<%s>)\n' "$(rrunner_link restore "$md_url")"
    printf -- '- **Run:** [Auto-run in Ghostty](<%s>)\n' "$(rrunner_link auto "$source_url")"
    printf -- '- **Edit:** [CotEditor](<%s>) · [VS Code](<%s>)\n\n' "$(rrunner_openwith com.coteditor.CotEditor "$source_url")" "$(rrunner_openwith com.microsoft.VSCode "$source_url")"
    printf '## Contents\n\n%s%s\n' "$fence" "$lang"
    cat "$file"
    printf '\n%s\n\n' "$fence"
    printf '<details>\n<summary>Embedded restore payload (Base64)</summary>\n\n```text\n'
    printf '<!-- ORIGINAL_FILE_BASE64_BEGIN -->\n'
    /usr/bin/base64 < "$file" | fold -w 76
    printf '<!-- ORIGINAL_FILE_BASE64_END -->\n```\n\n</details>\n'
  } > "$md"
  [[ -f "$md" ]] && created=$((created + 1)) || failed=$((failed + 1))
}

convert_files_to_md() {
  local file ext
  while IFS= read -r -d '' file; do
    ext="$(lower_ext "$file")"
    case "$ext" in md|markdown) skipped=$((skipped + 1)); continue ;; esac
    write_markdown_wrapper "$file"
  done < <(walk_files)
  notify "Created $created Markdown wrapper(s). Skipped: $skipped. Failed: $failed."
  (( failed == 0 ))
}

convert_image_to_icns() {
  local file ext dir base stem icon work iconset
  while IFS= read -r -d '' file; do
    ext="$(lower_ext "$file")"
    case "$ext" in png|jpg|jpeg|tif|tiff|gif|bmp|webp) ;; *) skipped=$((skipped + 1)); continue ;; esac
    dir="$(dirname "$file")"; base="$(basename "$file")"; stem="${base%.*}"
    icon="$(unique_path "$dir/$stem.icns")"
    work="$(mktemp -d "${TMPDIR:-/tmp}/rrunner-icns.XXXXXX")"; iconset="$work/icon.iconset"; mkdir -p "$iconset"
    if sips -z 1024 1024 "$file" --out "$iconset/icon_512x512@2x.png" >/dev/null 2>&1; then
      sips -z 512 512 "$iconset/icon_512x512@2x.png" --out "$iconset/icon_512x512.png" >/dev/null 2>&1
      sips -z 512 512 "$iconset/icon_512x512@2x.png" --out "$iconset/icon_256x256@2x.png" >/dev/null 2>&1
      sips -z 256 256 "$iconset/icon_512x512@2x.png" --out "$iconset/icon_256x256.png" >/dev/null 2>&1
      sips -z 256 256 "$iconset/icon_512x512@2x.png" --out "$iconset/icon_128x128@2x.png" >/dev/null 2>&1
      sips -z 128 128 "$iconset/icon_512x512@2x.png" --out "$iconset/icon_128x128.png" >/dev/null 2>&1
      sips -z 32 32 "$iconset/icon_512x512@2x.png" --out "$iconset/icon_16x16@2x.png" >/dev/null 2>&1
      sips -z 16 16 "$iconset/icon_512x512@2x.png" --out "$iconset/icon_16x16.png" >/dev/null 2>&1
      if iconutil -c icns "$iconset" -o "$icon" >/dev/null 2>&1; then converted=$((converted + 1)); else failed=$((failed + 1)); fi
    else
      failed=$((failed + 1))
    fi
    rm -rf "$work"
  done < <(walk_files)
  notify "Created $converted ICNS file(s). Skipped: $skipped. Failed: $failed."
  (( failed == 0 ))
}

convert_image_to_svg() {
  local potrace tmp_list
  potrace="$(command -v potrace || true)"
  [[ -n "$potrace" ]] || { alert "potrace was not found. Install it with Homebrew first."; exit 1; }
  tmp_list="$(mktemp "${TMPDIR:-/tmp}/rrunner-svg-selection.XXXXXX")"
  selection > "$tmp_list"
  export POTRACE_PATH="$potrace"
  export RRUNNER_SELECTION_LIST="$tmp_list"
  python3 - <<'PY'
import os, subprocess, sys, tempfile
from pathlib import Path
try:
    import cv2
except Exception as exc:
    subprocess.run(["osascript", "-e", f'display alert "Rrunner Finder Convert" message "OpenCV/cv2 is not available: {exc}"'])
    sys.exit(1)
paths = [Path(line.strip()) for line in Path(os.environ["RRUNNER_SELECTION_LIST"]).read_text().splitlines() if line.strip()]
converted = skipped = failed = 0
for src in paths:
    if src.is_dir():
        candidates = [p for p in src.rglob("*") if p.suffix.lower() in {".png", ".jpg", ".jpeg"}]
    else:
        candidates = [src]
    for p in candidates:
        if p.suffix.lower() not in {".png", ".jpg", ".jpeg"}:
            skipped += 1; continue
        out = p.with_suffix(".svg")
        i = 2
        while out.exists():
            out = p.with_name(f"{p.stem} {i}.svg"); i += 1
        try:
            img = cv2.imread(str(p), cv2.IMREAD_GRAYSCALE)
            if img is None: raise RuntimeError("OpenCV could not read image")
            _, bw = cv2.threshold(img, 200, 255, cv2.THRESH_BINARY)
            fd, tmp = tempfile.mkstemp(prefix="rrunner-svg-", suffix=".pbm"); os.close(fd)
            try:
                if not cv2.imwrite(tmp, bw): raise RuntimeError("OpenCV could not write PBM")
                subprocess.run([os.environ["POTRACE_PATH"], tmp, "-s", "-o", str(out)], check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
            finally:
                try: os.unlink(tmp)
                except OSError: pass
            converted += 1
        except Exception as exc:
            print(f"Failed {p}: {exc}", file=sys.stderr); failed += 1
subprocess.run(["osascript", "-e", f'display notification "Created {converted} SVG file(s). Skipped: {skipped}. Failed: {failed}." with title "Rrunner Finder Convert"'])
sys.exit(1 if failed else 0)
PY
  local status=$?
  rm -f "$tmp_list"
  return "$status"
}

find_bin() {
  local name="$1" app_glob="${2:-}"
  if command -v "$name" >/dev/null 2>&1; then command -v "$name"; return; fi
  if [[ -n "$app_glob" ]]; then
    /usr/bin/find /Applications -maxdepth 3 -path "$app_glob" -type f -perm +111 2>/dev/null | head -n 1
  fi
}

convert_audio_to_mp3() {
  local xld ffmpeg file ext dir stem out log status
  xld="$(find_bin xld '*/XLD*.app/Contents/MacOS/xld')"; ffmpeg="$(find_bin ffmpeg)"
  [[ -n "$xld" || -n "$ffmpeg" ]] || { alert "Neither xld nor ffmpeg was found."; exit 1; }
  while IFS= read -r -d '' file; do
    ext="$(lower_ext "$file")"
    case "$ext" in flac|wav|m4a) ;; *) skipped=$((skipped + 1)); continue ;; esac
    dir="$(dirname "$file")"; stem="$(basename "${file%.*}")"; out="$dir/$stem.mp3"
    [[ ! -e "$out" ]] || { skipped=$((skipped + 1)); continue; }
    if [[ "$ext" == "m4a" || -z "$xld" ]]; then
      [[ -n "$ffmpeg" ]] || { failed=$((failed + 1)); continue; }
      log="$($ffmpeg -y -i "$file" -b:a 320k -map_metadata 0 -id3v2_version 3 "$out" 2>&1)"; status=$?
    else
      log="$($xld -f mp3 --profile RILEY --keep-timestamp -o "$dir" "$file" 2>&1)"; status=$?
    fi
    if [[ "$status" -eq 0 && -f "$out" ]]; then
      converted=$((converted + 1))
    else
      failed=$((failed + 1)); rm -f "$out"; printf '%s\n' "$log" >&2
    fi
  done < <(walk_files)
  notify "Converted $converted audio file(s) to MP3. Originals kept. Skipped: $skipped. Failed: $failed."
  (( failed == 0 ))
}

convert_video_to_mkv() {
  local mkvmerge file ext dir stem out log status
  mkvmerge="$(find_bin mkvmerge '*/MKVToolNix*.app/Contents/MacOS/mkvmerge')"
  [[ -n "$mkvmerge" ]] || { alert "mkvmerge was not found. Install MKVToolNix first."; exit 1; }
  while IFS= read -r -d '' file; do
    ext="$(lower_ext "$file")"
    case "$ext" in mkv) skipped=$((skipped + 1)); continue ;; mp4|m4v|mov|avi|webm|wmv|flv|ts|m2ts|mts) ;; *) skipped=$((skipped + 1)); continue ;; esac
    dir="$(dirname "$file")"; stem="$(basename "${file%.*}")"; out="$(unique_path "$dir/$stem.mkv")"
    log="$($mkvmerge -o "$out" "$file" 2>&1)"; status=$?
    if [[ "$status" -eq 0 && -f "$out" ]]; then
      converted=$((converted + 1))
    else
      failed=$((failed + 1)); rm -f "$out"; printf '%s\n' "$log" >&2
    fi
  done < <(walk_files)
  notify "Converted $converted video file(s) to MKV. Skipped: $skipped. Failed: $failed."
  (( failed == 0 ))
}

if [[ -z "$(selection)" ]]; then
  notify "Select one or more files or folders in Finder first."
  exit 0
fi

case "$ACTION" in
  finder-scpt-to-applescript|scpt-to-applescript) convert_scpt_to_applescript ;;
  finder-files-to-md|files-to-md)                 convert_files_to_md ;;
  finder-image-to-icns|image-to-icns)             convert_image_to_icns ;;
  finder-image-to-svg|image-to-svg)               convert_image_to_svg ;;
  finder-audio-to-mp3|audio-to-mp3)               convert_audio_to_mp3 ;;
  finder-video-to-mkv|video-to-mkv)               convert_video_to_mkv ;;
  *) alert "Unknown finder conversion action: ${ACTION:-<empty>}"; exit 1 ;;
esac

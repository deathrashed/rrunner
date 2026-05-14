# Rrunner Configuration and Extension Guide

This document explains how to configure and extend **Rrunner**.

## Core idea

Rrunner has two parts:

1. **`/Applications/Rrunner.app`**  
   The tiny macOS app that registers the `rrunner://` URL scheme.

2. **`~/.local/bin/rrunner`**  
   The command-line bridge called by the app. It fetches and runs the public runner script from GitHub.

The public behavior lives here:

```text
https://raw.githubusercontent.com/deathrashed/rrunner/main/bin/rrunner.sh
https://raw.githubusercontent.com/deathrashed/rrunner/main/bin/md-restore.sh
```

That means each Mac only needs the URL scheme installed once, but the runner logic can be updated in the repo.

## Install

```bash
git clone https://github.com/deathrashed/rrunner.git
cd rrunner
chmod +x install.sh bin/rrunner bin/rrunner.sh bin/md-restore.sh
./install.sh
```

## Local configuration

Create the config file:

```bash
mkdir -p ~/.config/rrunner
cp config/rrunner.conf.example ~/.config/rrunner/config
```

Edit it:

```bash
nano ~/.config/rrunner/config
```

Supported settings:

```bash
RRUNNER_TERMINAL_APP="Ghostty"
RRUNNER_KEEP_OPEN=1
RRUNNER_REMOTE_BASE="https://raw.githubusercontent.com/deathrashed/rrunner/main"
RRUNNER_REMOTE_URL="$RRUNNER_REMOTE_BASE/bin/rrunner.sh"
RRUNNER_RESTORE_URL="$RRUNNER_REMOTE_BASE/bin/md-restore.sh"
RRUNNER_HANDLERS_DIR="$HOME/.config/rrunner/handlers"
RRUNNER_TEXT_EDITOR="com.coteditor.CotEditor"
RRUNNER_CODE_EDITOR="com.microsoft.VSCode"
RRUNNER_MARKDOWN_PREVIEWER="com.brettterpstra.marked2"
```

## URL anatomy

```text
rrunner://ACTION?url=file:///path/to/file
rrunner://ACTION?path=/path/to/file
rrunner://openwith?app=APP_ID_OR_NAME&url=file:///path/to/file
rrunner://launch?app=APP_ID_OR_NAME
```

Prefer `url=file://...` for paths with spaces or special characters.

## Built-in actions

| Action | Purpose |
|---|---|
| `open` | Open file with default app |
| `reveal` / `show` | Reveal file in Finder |
| `openwith` / `view` | Open file with a specific app |
| `launch` | Launch an app |
| `auto` | Choose a runner by file extension |
| `osascript` | Run file with `osascript` |
| `bash` | Run file with `bash` |
| `zsh` | Run file with `zsh` |
| `python` | Run file with `python3` |
| `node` | Run file with `node` |
| `ruby` | Run file with `ruby` |
| `perl` | Run file with `perl` |
| `restore` | Restore original file from Markdown wrapper payload |

## Auto-run dispatch

`rrunner://auto?...` chooses based on extension:

```text
.applescript / .scpt       osascript
.sh / .bash / .command     bash
.zsh                       zsh
.py / .python              python3
.js / .mjs / .cjs          node
.rb                        ruby
.pl / .pm                  perl
.md / .markdown            restore
```

## Custom handlers

Custom handlers let you invent new actions without editing the public script.

Create a handler directory:

```bash
mkdir -p ~/.config/rrunner/handlers
```

Create a handler:

```bash
cat > ~/.config/rrunner/handlers/open-project <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

# Available environment variables:
# RRUNNER_ACTION  action from the URL, here: open-project
# RRUNNER_URL     full original URL
# RRUNNER_PATH    decoded file path from url= or path=
# RRUNNER_APP     app= query parameter, if supplied

open -a "Visual Studio Code" "$RRUNNER_PATH"
EOF

chmod +x ~/.config/rrunner/handlers/open-project
```

Use it from Markdown:

```md
[Open Project](<rrunner://open-project?url=file:///Users/rd/Projects/my-project>)
```

## Adding a built-in action

Edit `bin/rrunner.sh`.

Add a function:

```bash
run_swift() {
  local path="$1"
  run_command_in_terminal "swift $(quote_for_shell "$path")"
}
```

Then add it to the `case "$ACTION"` block:

```bash
swift)
  require_payload
  run_swift "$PATH_PAYLOAD"
  ;;
```

## Generated Markdown wrappers

The Keyboard Maestro macro creates wrappers with:

- YAML frontmatter
- source path and source URL
- Rrunner action buttons
- notes and detected metadata
- source contents in a fenced code block
- embedded Base64 restore payload inside a collapsed `<details>` block

Restore button:

```md
[Restore original](<rrunner://restore?url=file:///path/to/wrapper.md>)
```

Run button:

```md
[Run automatically](<rrunner://auto?url=file:///path/to/source.applescript>)
```

## Troubleshooting

### `Rrunner command-line bridge was not found`

Check:

```bash
ls -l ~/.local/bin/rrunner
test -x ~/.local/bin/rrunner && echo OK
```

Reinstall:

```bash
cd /Users/rd/Scripts/Riley/rrunner
./install.sh
```

### URL scheme does not open Rrunner

Re-register the app:

```bash
rm -rf /Applications/Rrunner.app
cd /Users/rd/Scripts/Riley/rrunner
./install.sh
open 'rrunner://launch?app=Ghostty'
```

### Need to test without clicking a link

Call the bridge directly:

```bash
~/.local/bin/rrunner 'rrunner://open?url=file:///Users/rd/Scripts/Riley/rrunner/README.md'
```

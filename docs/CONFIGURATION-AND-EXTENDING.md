# Rrunner Configuration and Extension Guide

This document explains how to configure and extend **Rrunner**.

## Core idea

Rrunner has three runtime layers:

1. **`/Applications/Rrunner.app`**  
   The tiny macOS app that registers the `rrunner://` URL scheme and forwards incoming URLs to the bridge.

2. **`~/.local/bin/rrunner`**  
   The command-line bridge called by the app. It prefers the installed Go core at `~/.local/lib/rrunner/rrunner-core`.

3. **`~/.local/lib/rrunner/rrunner-core`**  
   The primary backend. It loads `config.toml`, discovers plugins, builds the action registry, supports diagnostics/dry-runs, and executes actions.

`bin/rrunner.sh` remains a legacy shell fallback for machines where the Go core is missing or when `RRUNNER_DISABLE_GO_CORE=1` is set. Do not add new primary behavior to the Bash fallback unless you are explicitly maintaining fallback compatibility.

Runtime flow:

```text
Rrunner.app → ~/.local/bin/rrunner → rrunner-core → action plan/execution
                                      ↳ bin/rrunner.sh fallback only
```

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
cp config/rrunner.config.toml.example ~/.config/rrunner/config.toml
```

Edit it:

```bash
nano ~/.config/rrunner/config.toml
```

Supported settings:

```toml
[settings]
terminal_app = "Ghostty"
keep_open = true
remote_base = "https://raw.githubusercontent.com/deathrashed/rrunner/main"
remote_url = "https://raw.githubusercontent.com/deathrashed/rrunner/main/bin/rrunner.sh"
restore_url = "https://raw.githubusercontent.com/deathrashed/rrunner/main/bin/md-restore.sh"
handlers_dir = "~/.config/rrunner/handlers"
text_editor = "com.coteditor.CotEditor"
code_editor = "com.microsoft.VSCode"
markdown_previewer = "com.brettterpstra.marked2"
```

Legacy shell config at `~/.config/rrunner/config` is still supported by the shell fallback, but `config.toml` is preferred. By default, legacy shell config is parsed as allowlisted `RRUNNER_*` assignments only. Setting `RRUNNER_ALLOW_LEGACY_SOURCE=1` re-enables direct shell `source` compatibility, but this is unsafe/deprecated because it executes arbitrary shell code from the config file.

Advanced backend sections:

```toml
[plugins]
enabled = true
dirs = ["~/.config/rrunner/plugins", "~/.local/share/rrunner/plugins"]
disabled = []
trusted = []

[security]
allow_inline_commands = true
allow_plugin_commands = false
require_trusted_plugins_for_commands = true
allow_legacy_handlers = true

[logging]
level = "info"
file = "~/Library/Logs/Rrunner/rrunner.log"
```

Backend commands:

```bash
rrunner --list-actions
rrunner --dry-run 'rrunner://edit?url=file:///path/to/file.md'
rrunner --diagnose
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

## Custom TOML actions

Custom TOML actions let you invent new URL actions without editing the public script or writing a handler file.

Add actions to `~/.config/rrunner/config.toml`:

```toml
[actions.edit]
type = "openwith"
description = "Open a file in CotEditor."
app = "com.coteditor.CotEditor"

[actions.preview]
type = "openwith"
description = "Preview Markdown in Marked."
app = "com.brettterpstra.marked2"

[actions.finder]
type = "reveal"
description = "Reveal the URL payload in Finder."

[actions.run-zsh]
type = "run"
description = "Run the URL payload as a zsh script."
runner = "zsh"
confirm = true

[actions.project-build]
type = "script"
description = "Run the project build script."
script = "~/Scripts/build-project.zsh"
confirm = true

# WARNING: command actions run shell text from this file.
# Only use commands you wrote and trust.
# Use single quotes around shell commands so inner "double quotes" need no escaping.
[actions.quick-command]
type = "command"
description = "Print request context."
command = 'echo "Path: $RRUNNER_PATH"; echo "URL: $RRUNNER_URL"'
confirm = true
```

Use them from Markdown:

```md
[Edit](<rrunner://edit?url=file:///Users/rd/Notes/example.md>)
[Preview](<rrunner://preview?url=file:///Users/rd/Notes/example.md>)
[Build Project](<rrunner://project-build?url=file:///Users/rd/Projects/my-project>)
```

Supported `type` values:

| Type | Required keys | Purpose |
|---|---|---|
| `open` | file URL/path in link | Open with the default app |
| `reveal` / `show` | file URL/path in link | Reveal in Finder |
| `openwith` / `view` | `app` | Open a file with an app name or bundle id |
| `launch` | `app` | Launch an app |
| `auto` | file URL/path in link | Choose runner by extension |
| `restore` | file URL/path in link | Restore Markdown wrapper payload |
| `run` | `runner` | Run the linked file with `osascript`, `bash`, `zsh`, `python`, `node`, `ruby`, or `perl` |
| `script` | `script` | Run a configured script in a terminal; linked path is passed as the first argument when present |
| `command` | `command` | Run inline shell text in a terminal |

`script` and `command` actions receive these environment variables:

```text
RRUNNER_ACTION  action from the URL, e.g. project-build
RRUNNER_URL     full original URL
RRUNNER_PATH    decoded file path from url= or path=
RRUNNER_APP     app= query parameter, if supplied
```

## Action discovery and authoring helpers

Use the Go core to inspect the live registry instead of hand-auditing config files:

```bash
rrunner --list-actions
rrunner --list-actions --markdown --agent-notes
rrunner --export-actions docs/ACTIONS.generated.md --agent-notes
rrunner --explain-action edit --markdown
rrunner --print-url edit README.md --markdown-link "Edit README"
```

Catalogs include action risk markers:

- `passive`: opens/reveals/views/launches.
- `filesystem`: restore/file-modifying behavior.
- `executable`: scripts, commands, runners, or legacy handlers.

For executable actions, prefer `confirm = true` unless the action is intentionally silent automation.

## Plugin manifests

For larger extension sets, create plugin manifests instead of putting everything in the main config. The repo includes ready-to-copy examples under `examples/plugins/basic-workflow/` and `examples/plugins/riley-workflow/`.


```text
~/.config/rrunner/plugins/example.plugin.toml
~/.config/rrunner/plugins/example/plugin.toml
```

Example:

```toml
[plugin]
id = "riley.workflow"
name = "Riley Workflow Launchers"
version = "1.0.0"
enabled = true

[actions.kb]
type = "command"
command = 'open "/Users/rd/.config/typinator/Sets/Includes/Text/KB"'

[actions.project-build]
type = "script"
script = "scripts/build-project.zsh"
runner = "zsh"
path_policy = "directory"
```

Action precedence is: main `config.toml` actions, plugins, legacy executable handlers, then built-ins. Duplicate lower-priority actions are shadowed and visible in `rrunner --diagnose`.

Plugin `command` actions are blocked unless `allow_plugin_commands = true` and, by default, the plugin id is listed under `[plugins].trusted`.

### Legacy executable handlers

Executable handlers in `~/.config/rrunner/handlers/` still work as a fallback. TOML actions are checked first, then plugins, then executable handlers, then built-in actions.

Example fallback handler:

```bash
mkdir -p ~/.config/rrunner/handlers
cat > ~/.config/rrunner/handlers/open-project <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
open -a "Visual Studio Code" "$RRUNNER_PATH"
EOF
chmod +x ~/.config/rrunner/handlers/open-project
```

## Adding a built-in action

Primary built-in actions belong in the Go core, not the Bash fallback.

Edit `cmd/rrunner-core/main.go`:

1. Add the action name in `builtinActions()`.
2. Add or extend handling in `planAction()`.
3. Add tests in `cmd/rrunner-core/main_test.go`.
4. Run `make validate`.

Only edit `bin/rrunner.sh` when you are intentionally changing legacy fallback behavior. The Bash runner is kept for rollback compatibility and should not receive new primary features.

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

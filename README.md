<p align="center">
  <img src="https://github.com/deathrashed/rrunner/blob/main/app/rrunner-icon.png?raw=true" width="160" alt="Rrunner icon">
</p>

# Rrunner

**Rrunner** is a tiny macOS URL-scheme launcher for personal automation links.

It registers the `rrunner://` URL scheme so Markdown files, notes, dashboards, and rendered documents can open files, reveal files, open files in specific apps, run scripts in Ghostty, and restore embedded originals from Markdown wrappers.

Rrunner is designed to replace the OpenAny dependency in these generated Markdown wrappers.

## What it does

```md
[Open](<rrunner://open?url=file:///Users/rd/Scripts/tool.applescript>)
[Reveal](<rrunner://reveal?url=file:///Users/rd/Scripts/tool.applescript>)
[Edit in CotEditor](<rrunner://openwith?app=com.coteditor.CotEditor&url=file:///Users/rd/Scripts/tool.applescript>)
[Run](<rrunner://auto?url=file:///Users/rd/Scripts/tool.applescript>)
[Restore](<rrunner://restore?url=file:///Users/rd/Scripts/tool.md>)
```

## Install

```bash
git clone https://github.com/deathrashed/rrunner.git
cd rrunner
chmod +x install.sh bin/rrunner bin/rrunner.sh bin/md-restore.sh
./install.sh
```

The installer creates:

```text
/Applications/Rrunner.app
~/.local/bin/rrunner
~/.local/lib/rrunner/rrunner-core
```

`Rrunner.app` is the local macOS URL handler. The command-line bridge at `~/.local/bin/rrunner` now prefers the local Go core backend for plugin/config dispatch, with the shell runner kept as a fallback. Set `RRUNNER_DISABLE_GO_CORE=1` to force the legacy shell fallback.

## Test

```bash
make validate
open 'rrunner://open?url=file:///Users/rd/Scripts/Riley/rrunner/README.md'
open 'rrunner://launch?app=Ghostty'
rrunner --list-actions
rrunner --list-actions --markdown --agent-notes
rrunner --explain-action open --markdown
rrunner --print-url open --markdown-link "Open README" README.md
rrunner --dry-run 'rrunner://open?url=file:///Users/rd/Scripts/Riley/rrunner/README.md'
rrunner --diagnose
./install.sh --dry-run
```

## Repo layout

```text
rrunner/
├── README.md
├── gemini.md            # AI/developer repository briefing
├── go.mod
├── Makefile             # local validation targets
├── install.sh
├── cmd/
│   └── rrunner-core/    # primary Go backend for config/plugins/actions
├── bin/
│   ├── rrunner          # tiny local bridge used by Rrunner.app
│   ├── rrunner.sh       # legacy shell fallback when Go core is disabled/missing
│   └── md-restore.sh    # restore embedded Base64 originals from Markdown wrappers
├── app/
│   ├── Rrunner.applescript
│   ├── Rrunner.icns
│   └── rrunner-icon.png
├── config/
│   ├── rrunner.config.toml.example
│   └── rrunner.conf.example      # legacy shell config example
├── docs/
│   ├── CONFIGURATION-AND-EXTENDING.md
│   └── URL-SCHEMES.md
└── examples/
    ├── markdown-links.md
    └── plugins/        # example plugin manifests
```

## Supported URL actions

| Action | Example |
|---|---|
| Open file | `rrunner://open?url=file:///path/to/file.txt` |
| Reveal file | `rrunner://reveal?url=file:///path/to/file.txt` |
| Open with app | `rrunner://openwith?app=com.coteditor.CotEditor&url=file:///path/to/file.txt` |
| Launch app | `rrunner://launch?app=Ghostty` |
| Auto-run by extension | `rrunner://auto?url=file:///path/to/script.applescript` |
| Run AppleScript | `rrunner://osascript?url=file:///path/to/script.applescript` |
| Run Bash | `rrunner://bash?url=file:///path/to/script.sh` |
| Run Zsh | `rrunner://zsh?url=file:///path/to/script.zsh` |
| Run Python | `rrunner://python?url=file:///path/to/script.py` |
| Run Node | `rrunner://node?url=file:///path/to/script.js` |
| Run Ruby | `rrunner://ruby?url=file:///path/to/script.rb` |
| Run Perl | `rrunner://perl?url=file:///path/to/script.pl` |
| Restore Markdown wrapper | `rrunner://restore?url=file:///path/to/wrapper.md` |

## Configure

Create:

```bash
mkdir -p ~/.config/rrunner
cp config/rrunner.config.toml.example ~/.config/rrunner/config.toml
```

Then edit:

```bash
nano ~/.config/rrunner/config.toml
```

Common settings:

```toml
[settings]
terminal_app = "Ghostty"
keep_open = true
remote_base = "https://raw.githubusercontent.com/deathrashed/rrunner/main"
handlers_dir = "~/.config/rrunner/handlers"
```

See [docs/CONFIGURATION-AND-EXTENDING.md](docs/CONFIGURATION-AND-EXTENDING.md).

## Extend

Add custom actions to `~/.config/rrunner/config.toml`:

```toml
[actions.edit]
type = "openwith"
description = "Open a file in CotEditor."
app = "com.coteditor.CotEditor"

[actions.preview]
type = "openwith"
description = "Preview Markdown in Marked."
app = "com.brettterpstra.marked2"

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
# Use single quotes around shell commands so inner "double quotes" need no escaping.
[actions.quick-command]
type = "command"
description = "Print request context."
command = 'echo "Path: $RRUNNER_PATH"'
confirm = true
```

Then use:

```md
[Edit](<rrunner://edit?url=file:///Users/rd/Notes/example.md>)
[Preview](<rrunner://preview?url=file:///Users/rd/Notes/example.md>)
[Build](<rrunner://project-build?url=file:///Users/rd/Projects/my-project>)
```

Executable handlers in `~/.config/rrunner/handlers/` are still supported as a fallback for advanced cases.

Built-in actions belong in the Go core (`cmd/rrunner-core/main.go`). The Bash runner is a compatibility fallback only; do not add new primary behavior there unless you are explicitly changing fallback behavior.

## Plugins and diagnostics

The Go core supports manifest plugins in:

```text
~/.config/rrunner/plugins/*.plugin.toml
~/.config/rrunner/plugins/*/plugin.toml
```

Useful backend commands:

```bash
rrunner --list-actions
rrunner --list-actions --json
rrunner --list-actions --markdown --agent-notes
rrunner --export-actions docs/ACTIONS.generated.md --agent-notes
rrunner --explain-action edit --markdown
rrunner --print-url edit --markdown-link "Edit README" README.md
rrunner --dry-run 'rrunner://edit?url=file:///path/to/file.md'
rrunner --diagnose
```

The repo includes example plugin manifests in `examples/plugins/basic-workflow/` and `examples/plugins/riley-workflow/`.

Plugin `command` actions are blocked by default unless enabled and trusted in `config.toml`; inline commands in your own config remain allowed.

## Markdown wrapper restore

Markdown wrappers generated by the Keyboard Maestro macro can embed the original source file as Base64 inside a collapsed `<details>` block.

Rrunner restores them with:

```md
[Restore original](<rrunner://restore?url=file:///path/to/wrapper.md>)
```

The restore command will not overwrite an existing original unless the restore script is called with `--force`.

## Notes

Rrunner is intentionally local-first. A Mac still needs `Rrunner.app` installed once so macOS knows what `rrunner://` means, but the actual behavior can be updated from this public repo.

# Rrunner URL Schemes

## Backend diagnostics

```md
[Diagnostics](<rrunner://diagnostics>)
[List actions](<rrunner://list-actions>)
[Dry-run open](<rrunner://open?url=file:///path/to/file.txt&_rrunner_dry_run=1>)
```

CLI helpers:

```bash
rrunner --list-actions --markdown
rrunner --list-actions --markdown --agent-notes
rrunner --export-actions docs/ACTIONS.generated.md --agent-notes
rrunner --explain-action edit --markdown
rrunner --print-url edit --markdown-link "Edit README" README.md
```

Action catalog output includes a risk marker:

- `passive` — opens/reveals/views/launches.
- `filesystem` — modifies or restores local files.
- `executable` — runs a script, command, runner, or legacy handler.

## File operations

```md
[Open](<rrunner://open?url=file:///path/to/file.txt>)
[Reveal](<rrunner://reveal?url=file:///path/to/file.txt>)
[Show](<rrunner://show?url=file:///path/to/file.txt>)
```

## Open with a specific app

```md
[CotEditor](<rrunner://openwith?app=com.coteditor.CotEditor&url=file:///path/to/file.txt>)
[VS Code](<rrunner://openwith?app=com.microsoft.VSCode&url=file:///path/to/file.txt>)
[Script Editor](<rrunner://openwith?app=com.apple.ScriptEditor2&url=file:///path/to/script.applescript>)
[Marked](<rrunner://openwith?app=com.brettterpstra.marked2&url=file:///path/to/file.md>)
```

`view` is an alias for `openwith`:

```md
[CotEditor](<rrunner://view?app=com.coteditor.CotEditor&url=file:///path/to/file.txt>)
```

## Launch apps

```md
[Launch Ghostty](<rrunner://launch?app=Ghostty>)
[Launch CotEditor](<rrunner://launch?app=com.coteditor.CotEditor>)
```

## Run scripts in Ghostty

```md
[Auto-run](<rrunner://auto?url=file:///path/to/script.applescript>)
[Run AppleScript](<rrunner://osascript?url=file:///path/to/script.applescript>)
[Run Bash](<rrunner://bash?url=file:///path/to/script.sh>)
[Run Zsh](<rrunner://zsh?url=file:///path/to/script.zsh>)
[Run Python](<rrunner://python?url=file:///path/to/script.py>)
[Run Node](<rrunner://node?url=file:///path/to/script.js>)
[Run Ruby](<rrunner://ruby?url=file:///path/to/script.rb>)
[Run Perl](<rrunner://perl?url=file:///path/to/script.pl>)
```

## Restore Markdown wrapper

```md
[Restore original](<rrunner://restore?url=file:///path/to/wrapper.md>)
```

## Custom TOML actions

Add easy-to-read custom actions in `~/.config/rrunner/config.toml`:

```toml
[actions.edit]
type = "openwith"
app = "com.coteditor.CotEditor"

[actions.preview]
type = "openwith"
app = "com.brettterpstra.marked2"

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

Example links:

```md
[Edit](<rrunner://edit?url=file:///Users/rd/Notes/example.md>)
[Preview](<rrunner://preview?url=file:///Users/rd/Notes/example.md>)
[Build](<rrunner://project-build?url=file:///Users/rd/Projects/my-project>)
```

Supported TOML `type` values: `open`, `reveal`, `show`, `openwith`, `view`, `launch`, `auto`, `restore`, `run`, `script`, and `command`.

Executable handlers in `~/.config/rrunner/handlers/` still work as a fallback for advanced cases.

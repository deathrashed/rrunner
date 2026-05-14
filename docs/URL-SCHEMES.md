# Rrunner URL Schemes

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

## Custom handlers

Any executable in `~/.config/rrunner/handlers` can become a URL action.

Example handler:

```text
~/.config/rrunner/handlers/open-project
```

Example link:

```md
[Open Project](<rrunner://open-project?url=file:///Users/rd/Projects/my-project>)
```

# Rrunner Markdown Link Examples

```md
## File

- [Open](<rrunner://open?url=file:///Users/rd/Scripts/tool.applescript>)
- [Reveal](<rrunner://reveal?url=file:///Users/rd/Scripts/tool.applescript>)

## Edit

- [CotEditor](<rrunner://openwith?app=com.coteditor.CotEditor&url=file:///Users/rd/Scripts/tool.applescript>)
- [VS Code](<rrunner://openwith?app=com.microsoft.VSCode&url=file:///Users/rd/Scripts/tool.applescript>)
- [Script Editor](<rrunner://openwith?app=com.apple.ScriptEditor2&url=file:///Users/rd/Scripts/tool.applescript>)

## Run

- [Auto-run](<rrunner://auto?url=file:///Users/rd/Scripts/tool.applescript>)
- [Run with osascript](<rrunner://osascript?url=file:///Users/rd/Scripts/tool.applescript>)

## Restore

- [Restore original](<rrunner://restore?url=file:///Users/rd/Scripts/tool.md>)

## Custom TOML actions

These assume matching `[actions.<name>]` entries in `~/.config/rrunner/config.toml`.

- [Edit](<rrunner://edit?url=file:///Users/rd/Scripts/tool.applescript>)
- [Preview](<rrunner://preview?url=file:///Users/rd/Scripts/tool.md>)
- [Run Zsh](<rrunner://run-zsh?url=file:///Users/rd/Scripts/tool.zsh>)
- [Build Project](<rrunner://project-build?url=file:///Users/rd/Projects/my-project>)
```

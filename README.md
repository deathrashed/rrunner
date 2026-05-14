# Rrunner

**Rrunner** is a small macOS URL-scheme helper for turning Markdown links into real local actions.

It replaces the need for `openany://` links in generated Markdown wrappers by handling opening, revealing, opening-with-app, restoring embedded files, and running scripts from a single custom scheme:

```text
rrunner://
```

App name casing is intentional:

```text
Rrunner
```

Not `RRunner`, not `rRunner`, not `rrunner`.

## What it does

Rrunner lets Markdown files contain clickable buttons such as:

```md
[Open original](<rrunner://open?url=file:///Users/rd/Scripts/tool.applescript>)
[Reveal original](<rrunner://reveal?url=file:///Users/rd/Scripts/tool.applescript>)
[Edit in CotEditor](<rrunner://openwith?app=com.coteditor.CotEditor&url=file:///Users/rd/Scripts/tool.applescript>)
[Run AppleScript](<rrunner://osascript?url=file:///Users/rd/Scripts/tool.applescript>)
[Restore original](<rrunner://restore?url=file:///Users/rd/Scripts/tool.md>)
```

## Repo layout

```text
rrunner/
├── README.md
├── install.sh
├── bin/
│   ├── rrunner          # tiny local bridge installed to ~/.local/bin/rrunner
│   ├── rrunner.sh       # public runner logic fetched by the bridge
│   └── md-restore.sh    # restores embedded Base64 payloads from .md wrappers
├── app/
│   ├── Rrunner.applescript
│   ├── Rrunner.icns
│   └── Info.plist.example
├── docs/
│   └── URL-SCHEMES.md
└── examples/
    └── markdown-links.md
```

## Install

Clone the repo:

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
```

Then it opens `Rrunner.app` once so macOS registers the `rrunner://` URL scheme.

## Public repo behavior

The installed local helper is intentionally tiny. When a `rrunner://` link is clicked, it calls:

```text
~/.local/bin/rrunner
```

That bridge fetches the latest public runner from:

```text
https://raw.githubusercontent.com/deathrashed/rrunner/main/bin/rrunner.sh
```

The restore action fetches:

```text
https://raw.githubusercontent.com/deathrashed/rrunner/main/bin/md-restore.sh
```

So future behavior can be updated from GitHub without rebuilding the app, while the local app remains only the URL-scheme registrar.

## Supported URL actions

### Open and reveal

```text
rrunner://open?url=file:///path/to/file.txt
rrunner://reveal?url=file:///path/to/file.txt
```

### Open with a specific app

Use a bundle ID:

```text
rrunner://openwith?app=com.coteditor.CotEditor&url=file:///path/to/file.txt
rrunner://openwith?app=com.microsoft.VSCode&url=file:///path/to/project
rrunner://openwith?app=com.apple.ScriptEditor2&url=file:///path/to/script.applescript
```

Or an app name:

```text
rrunner://openwith?app=Ghostty&url=file:///path/to/script.sh
```

### Launch an app

```text
rrunner://launch?app=Ghostty
rrunner://launch?app=com.coteditor.CotEditor
```

### Run scripts in Ghostty

```text
rrunner://auto?url=file:///path/to/script.applescript
rrunner://osascript?url=file:///path/to/script.applescript
rrunner://bash?url=file:///path/to/script.sh
rrunner://zsh?url=file:///path/to/script.zsh
rrunner://python?url=file:///path/to/script.py
rrunner://node?url=file:///path/to/script.js
rrunner://ruby?url=file:///path/to/script.rb
rrunner://perl?url=file:///path/to/script.pl
```

`auto` dispatches by file extension:

```text
.applescript / .scpt      osascript
.sh / .bash / .command    bash
.zsh                      zsh
.py                       python3
.js / .mjs / .cjs         node
.rb                       ruby
.pl / .pm                 perl
.md / .markdown           restore
```

### Restore embedded original files

```text
rrunner://restore?url=file:///path/to/wrapper.md
```

The wrapper must contain:

```md
<!-- ORIGINAL_FILE_BASE64_BEGIN -->
...
<!-- ORIGINAL_FILE_BASE64_END -->
```

and a frontmatter field:

```yaml
source_file: "/absolute/path/to/original.file"
```

## Why not OpenAny?

Rrunner now handles the OpenAny-style actions directly:

| Old OpenAny role | Rrunner replacement |
|---|---|
| `openany://file/open?...` | `rrunner://open?...` |
| `openany://file/reveal?...` | `rrunner://reveal?...` |
| `openany://app/APP/view?...` | `rrunner://openwith?app=APP&...` |
| launch app | `rrunner://launch?app=APP` |

This keeps Markdown buttons dependent on one tool instead of two.

## Notes

- `Rrunner.app` must be installed on each Mac that should understand `rrunner://` links.
- The generated Markdown remains readable anywhere.
- Restore/run buttons are macOS-specific because custom URL schemes are handled by local apps.

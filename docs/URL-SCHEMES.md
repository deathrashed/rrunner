# Rrunner URL Schemes

## Shape

```text
rrunner://ACTION?key=value&key=value
```

The most important payload keys are:

```text
url=file:///absolute/path/to/file
path=/absolute/path/to/file
app=com.example.BundleID
```

Prefer `url=file://...` for paths containing spaces or special characters.

## Actions

### Open

```text
rrunner://open?url=file:///Users/rd/file.txt
```

### Reveal in Finder

```text
rrunner://reveal?url=file:///Users/rd/file.txt
```

### Open with app

```text
rrunner://openwith?app=com.coteditor.CotEditor&url=file:///Users/rd/file.txt
rrunner://openwith?app=Ghostty&url=file:///Users/rd/script.sh
```

### Launch app

```text
rrunner://launch?app=Ghostty
rrunner://launch?app=com.microsoft.VSCode
```

### Run

```text
rrunner://auto?url=file:///Users/rd/script.applescript
rrunner://osascript?url=file:///Users/rd/script.applescript
rrunner://bash?url=file:///Users/rd/script.sh
rrunner://zsh?url=file:///Users/rd/script.zsh
rrunner://python?url=file:///Users/rd/script.py
rrunner://node?url=file:///Users/rd/script.js
rrunner://ruby?url=file:///Users/rd/script.rb
rrunner://perl?url=file:///Users/rd/script.pl
```

### Restore

```text
rrunner://restore?url=file:///Users/rd/wrapper.md
```

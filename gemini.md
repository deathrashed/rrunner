# Rrunner Repository Briefing

## 1. Project Overview & Architecture

* **Purpose:** Rrunner is a macOS `rrunner://` URL-scheme launcher for personal automation links. It lets Markdown, notes, dashboards, and rendered documents open/reveal files, open files in specific apps, launch apps, run scripts, restore embedded originals from Markdown wrappers, and route custom TOML/plugin actions.
* **Tech Stack:**
  * **Go 1.22 module:** `github.com/deathrashed/rrunner`; core CLI in `cmd/rrunner-core`.
  * **Bash:** bridge/fallback runner scripts in `bin/` plus Markdown restore helper.
  * **AppleScript:** `app/Rrunner.applescript` registers/handles macOS URL events through `/Users/rd/.local/bin/rrunner`.
  * **TOML subset:** user config in `~/.config/rrunner/config.toml`; plugin manifests in `~/.config/rrunner/plugins` or `~/.local/share/rrunner/plugins`.
* **Architecture:** Local-first macOS utility with a compiled Go dispatch core, shell bridge/fallback, and AppleScript URL handler. The app receives a URL, calls the bridge, the bridge prefers the Go core, and the Go core loads config/plugins, builds an action registry, plans the requested action, and either dry-runs or executes via macOS `open`, Ghostty/Terminal wrapper scripts, legacy handlers, or restore helper.
* **Architecture Diagram:**

```mermaid
flowchart TD
  Link[Markdown / note / dashboard rrunner:// URL] --> App[Rrunner.app AppleScript URL handler]
  App --> Bridge[~/.local/bin/rrunner]
  Bridge -->|preferred| Core[~/.local/lib/rrunner/rrunner-core]
  Bridge -->|fallback fetch/cache| Shell[bin/rrunner.sh]
  Shell -->|if available| Core
  Shell -->|legacy fallback| BashDispatch[Bash TOML/handler/builtin dispatcher]

  Core --> Config[~/.config/rrunner/config.toml]
  Core --> Plugins[~/.config/rrunner/plugins/*.plugin.toml and */plugin.toml]
  Core --> Handlers[~/.config/rrunner/handlers executables]
  Core --> Registry[Action registry: config > plugins > handlers > built-ins]
  Registry --> Plan[ExecutionPlan / dry-run]
  Plan --> Open[/usr/bin/open file/app actions]
  Plan --> Terminal[Ghostty or Terminal run.zsh wrapper]
  Plan --> Restore[md-restore.sh cached/local helper]
  Core --> Logs[~/Library/Logs/Rrunner/rrunner.log JSONL]
```

## 2. Repository Map (Monorepo Aware)

* **Monorepo status:** Not a monorepo. No workspace manifest, `packages/`, Cargo workspace, Node package, Docker, or CI folder was found.
* **Directory Structure:**
  * `cmd/rrunner-core/` — Go core backend; parses CLI flags, config, plugins, URLs, builds registry/plans, executes actions, logs, and provides diagnostics.
  * `bin/rrunner` — installed command-line bridge used by `Rrunner.app`; prefers `~/.local/lib/rrunner/rrunner-core`, otherwise fetches/caches `bin/rrunner.sh`.
  * `bin/rrunner.sh` — shell fallback and compatibility dispatcher; delegates to Go core when present unless `RRUNNER_DISABLE_GO_CORE=1`.
  * `bin/md-restore.sh` — restores original files from Markdown wrappers containing `source_file:` and Base64 payload markers.
  * `Makefile` — local validation wrapper; `make validate` is the preferred pre-completion check.
  * `app/` — AppleScript app source, icon, and plist example for registering `rrunner://` with macOS.
  * `config/` — example configs: new `rrunner.config.toml.example` and legacy `rrunner.conf.example`.
  * `docs/` — configuration/extension guide and URL scheme examples.
  * `examples/` — Markdown link examples and plugin manifest examples.
* **Key Entry Points:**
  * Runtime URL entry: `app/Rrunner.applescript` `on open location thisURL` → `/Users/rd/.local/bin/rrunner`.
  * Bridge entry: `bin/rrunner`.
  * Go core entry: `cmd/rrunner-core/main.go` `main()` / `run(args []string)`.
  * Installer: `install.sh` builds Go core, installs bridge, compiles `Rrunner.app`, and registers Launch Services.

## 3. Operational Workflows

* **Setup & Environment:**
  * Prerequisites evidenced in repo: macOS, Bash, `/usr/bin/python3` for shell fallback parsers, `curl`, AppleScript tooling (`osacompile`, `osascript`), and optionally `go` for the Go core.
  * Install:
    ```bash
    chmod +x install.sh bin/rrunner bin/rrunner.sh bin/md-restore.sh
    ./install.sh
    ```
  * Installer outputs:
    ```text
    /Applications/Rrunner.app
    ~/.local/bin/rrunner
    ~/.local/lib/rrunner/rrunner-core
    ```
  * User config:
    ```bash
    mkdir -p ~/.config/rrunner
    cp config/rrunner.config.toml.example ~/.config/rrunner/config.toml
    ```
  * Important env toggles:
    * `RRUNNER_CORE` — override Go core path.
    * `RRUNNER_DISABLE_GO_CORE=1` — force shell fallback.
    * `RRUNNER_CONFIG_TOML` — override TOML config path.
    * `RRUNNER_ALLOW_LEGACY_SOURCE=1` — opt into unsafe legacy shell `source` compatibility.
* **Development Server:** Not defined in repository. This is a CLI/macOS app utility, not a server project.
* **Testing & Validation:** Required check before declaring changes complete:
  ```bash
  make validate
  ```
  `make validate` runs:
  ```bash
  gofmt -w cmd/rrunner-core/main.go cmd/rrunner-core/main_test.go
  go test ./...
  go build -o /tmp/rrunner-core ./cmd/rrunner-core
  bash -n bin/rrunner bin/rrunner.sh install.sh bin/md-restore.sh
  /tmp/rrunner-core --validate-install --json
  /tmp/rrunner-core --dry-run 'rrunner://open?url=file:///Users/rd/Scripts/Riley/rrunner/README.md'
  ```
  Useful runtime checks:
  ```bash
    rrunner --version
    rrunner --list-actions
    rrunner --list-actions --json
    rrunner --list-actions --markdown --agent-notes
    rrunner --export-actions docs/ACTIONS.generated.md --agent-notes
    rrunner --explain-action edit --markdown
    rrunner --print-url edit README.md --markdown-link "Edit README"
    rrunner --dry-run 'rrunner://edit?url=file:///path/to/file.md'
    rrunner --diagnose
  ```
* **CI/CD:** Not defined in repository. No `.github/workflows`, Dockerfile, Justfile, Taskfile, or env example was found. Local validation is provided by `Makefile`.

## 4. Technical Conventions & Patterns

* **Coding Standards:**
  * Go core currently uses a single `package main` with small structs (`Config`, `Settings`, `ActionSpec`, `PluginManifest`, `Registry`, `Request`, `ExecutionPlan`) and helper functions grouped by responsibility.
  * Go errors are returned and surfaced with explicit non-zero exit codes; no panics are used for normal failures.
  * CLI flags use the standard library `flag` package; JSON output uses `encoding/json`.
  * No third-party Go dependencies are used.
  * Bash scripts use `#!/usr/bin/env bash` and `set -euo pipefail`.
* **State & Data:**
  * Primary state is local filesystem config:
    * Main config: `~/.config/rrunner/config.toml`.
    * Plugins: `~/.config/rrunner/plugins/*.plugin.toml`, `~/.config/rrunner/plugins/*/plugin.toml`, and the same shapes under `~/.local/share/rrunner/plugins`.
    * Legacy handlers: `~/.config/rrunner/handlers`.
    * Logs: `~/Library/Logs/Rrunner/rrunner.log` JSONL.
    * Cache: `${XDG_CACHE_HOME:-~/Library/Caches}/Rrunner` for shell/restore fallback assets.
  * Action precedence is documented and implemented as: main config actions, plugins, legacy executable handlers, built-ins.
  * The Go core intentionally implements a limited TOML subset: comments, sections, quoted strings, booleans, integers, string arrays, multiline string arrays, `[settings]`, `[plugins]`, `[security]`, `[logging]`, `[plugin]`, and `[actions.<name>]`.
* **Error Handling:**
  * Go core returns exit code `2` for CLI usage/flag errors and `1` for action/config/execution failures.
  * `--validate-install` returns non-zero when config or registry errors exist.
  * Plugin/config parse issues are accumulated in `Config.Errors` / `Registry.Errors` and exposed through diagnostics.
  * User-facing runtime failures call `userError(...)`; action failures are also logged through `logEvent` when logging is configured.
  * Shell fallback uses AppleScript alerts/notifications for user-visible errors.

## 5. AI Agent Operational Guidance (CRITICAL)

* **Safe Modification Boundaries:**
  * Safe to edit: `cmd/rrunner-core/*.go`, `bin/rrunner`, `bin/rrunner.sh`, `bin/md-restore.sh`, `install.sh`, `config/*.example`, `docs/*.md`, `README.md`, `examples/*.md`.
  * Be careful with `app/Rrunner.applescript`: it hardcodes `bridgePath : "/Users/rd/.local/bin/rrunner"`; changing this affects installed app behavior.
  * Do not edit generated installed artifacts directly as source of truth: `/Applications/Rrunner.app`, `~/.local/bin/rrunner`, `~/.local/lib/rrunner/rrunner-core`, or cache files under `~/Library/Caches/Rrunner`. Update repo files and rerun `install.sh` or rebuild/install intentionally.
* **Secrets Management & Security:**
  * No repo-local `.env` or secrets management file is defined. Do not introduce credentials into the repo.
  * Treat `command`, `script`, runner, and legacy-handler actions as high-risk executable actions. Inline commands in the main local config are allowed; plugin `command` actions are blocked unless `allow_plugin_commands = true` and, by default, the plugin id is trusted. Prefer `confirm = true` for executable actions exposed in Markdown.
  * Do not reintroduce unconditional `source "$cfg"` behavior for legacy shell config. The bridge and fallback now parse allowlisted `RRUNNER_*` assignments unless `RRUNNER_ALLOW_LEGACY_SOURCE=1` is explicitly set.
  * Do not template-expand untrusted payload values into shell command strings. The current model passes request data through environment variables such as `RRUNNER_ACTION`, `RRUNNER_URL`, `RRUNNER_PATH`, `RRUNNER_APP`, `RRUNNER_PLUGIN_ID`, and `RRUNNER_PLUGIN_DIR`.
* **Preservation Mandates:**
  * Preserve existing URL forms: `rrunner://ACTION?url=file:///path`, `rrunner://ACTION?path=/path`, `rrunner://openwith?app=...&url=...`, and `rrunner://launch?app=...`.
  * Preserve the rollback path: `RRUNNER_DISABLE_GO_CORE=1` must force shell fallback while `bin/rrunner.sh` exists.
  * Preserve main config compatibility for `[settings]` and `[actions.<name>]`.
  * Preserve plugin action precedence: local config > plugins > legacy handlers > built-ins.
  * Preserve single-quoted TOML command examples so shell double-quotes do not require backslash escaping.
* **Known Pitfalls:**
  * The Go TOML parser is intentionally partial, not a full TOML implementation. Do not document unsupported TOML syntax as supported without adding tests and parser support.
  * GUI URL testing via `open 'rrunner://...'` can launch apps or terminals. Prefer `--dry-run` for automated validation.
  * `install.sh` writes outside the repo (`/Applications`, `~/.local/bin`, `~/.local/lib`) and refreshes Launch Services.
  * `restore` requires a Markdown wrapper with `source_file:` plus `ORIGINAL_FILE_BASE64_BEGIN/END` markers; normal Markdown files are not restorable payloads.
  * The repository currently has uncommitted modifications and untracked Go/config files; check `git status` before assuming baseline state.
* **Required Verifications:**
  * For any Go/core/config/plugin change, run `make validate`.
  * For shell-only changes, still run `bash -n bin/rrunner bin/rrunner.sh install.sh bin/md-restore.sh` plus at least one Go core dry-run if the bridge/core behavior can be affected.
  * For installer changes, run `bash -n install.sh` and inspect paths carefully before executing `./install.sh` because it touches `/Applications` and `~/.local`.

## 6. Project Health Snapshot

* **Current State:** Active refactor toward a Go-backed plugin system. The Go core builds and has unit tests for URL parsing, TOML command parsing, and dry-run planning. Shell fallback is still present for compatibility.
* **Repository status observed:** `main` tracking `origin/main`; modified files include `README.md`, `bin/rrunner`, `bin/rrunner.sh`, docs, examples, and `install.sh`; untracked files include `cmd/`, `go.mod`, `Makefile`, `gemini.md`, and `config/rrunner.config.toml.example`.
* **Coverage:** Low but present. Tests exist only under `cmd/rrunner-core/main_test.go`; no coverage tooling is configured.
* **Operational maturity:** No CI/CD is defined. Local validation is consolidated in `make validate`, which wraps `gofmt`, `go test`, `go build`, `bash -n`, `--validate-install`, and `--dry-run`.
* **Recent major refactor:** Introduction of `~/.local/lib/rrunner/rrunner-core` as the preferred backend, with Bash bridge/fallback retained.

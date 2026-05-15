package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const version = "0.2.0-go-core"

type Settings struct {
	TerminalApp       string
	KeepOpen          bool
	RemoteBase        string
	RemoteURL         string
	RestoreURL        string
	HandlersDir       string
	TextEditor        string
	CodeEditor        string
	MarkdownPreviewer string
}

type PluginSettings struct {
	Enabled  bool
	Dirs     []string
	Disabled map[string]bool
	Trusted  map[string]bool
}

type SecuritySettings struct {
	AllowInlineCommands              bool
	AllowPluginCommands              bool
	RequireTrustedPluginsForCommands bool
	AllowLegacyHandlers              bool
}

type LoggingSettings struct {
	Level string
	File  string
}

type Config struct {
	Path     string
	Settings Settings
	Plugins  PluginSettings
	Security SecuritySettings
	Logging  LoggingSettings
	Actions  map[string]ActionSpec
	Errors   []string
}

type ActionSpec struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Source      string   `json:"source"`
	Description string   `json:"description,omitempty"`
	Aliases     []string `json:"aliases,omitempty"`
	App         string   `json:"app,omitempty"`
	Runner      string   `json:"runner,omitempty"`
	Script      string   `json:"script,omitempty"`
	Command     string   `json:"command,omitempty"`
	PathPolicy  string   `json:"path_policy,omitempty"`
	Confirm     bool     `json:"confirm,omitempty"`
	PluginID    string   `json:"plugin_id,omitempty"`
	PluginDir   string   `json:"plugin_dir,omitempty"`
	Shadowed    bool     `json:"shadowed,omitempty"`
}

type PluginManifest struct {
	ID      string
	Name    string
	Version string
	Path    string
	Dir     string
	Enabled bool
	Actions map[string]ActionSpec
	Errors  []string
}

type PluginEvent struct {
	Path   string `json:"path"`
	ID     string `json:"id,omitempty"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type Registry struct {
	Actions      map[string]ActionSpec `json:"actions"`
	Shadowed     []ActionSpec          `json:"shadowed,omitempty"`
	Plugins      []PluginManifest      `json:"-"`
	PluginEvents []PluginEvent         `json:"plugin_events,omitempty"`
	Errors       []string              `json:"errors,omitempty"`
}

type Request struct {
	Raw    string              `json:"raw"`
	Action string              `json:"action"`
	Path   string              `json:"path,omitempty"`
	App    string              `json:"app,omitempty"`
	Query  map[string][]string `json:"query,omitempty"`
}

type PlanStep struct {
	Kind    string            `json:"kind"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type ExecutionPlan struct {
	Action   string     `json:"action"`
	Source   string     `json:"source"`
	DryRun   bool       `json:"dry_run"`
	Steps    []PlanStep `json:"steps"`
	Warnings []string   `json:"warnings,omitempty"`
}

type miniTOML struct {
	Sections map[string]map[string]any
	Errors   []string
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("rrunner-core", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dryRun := fs.Bool("dry-run", false, "plan without executing")
	jsonOut := fs.Bool("json", false, "print JSON")
	diagnose := fs.Bool("diagnose", false, "print diagnostics")
	listActions := fs.Bool("list-actions", false, "list available actions")
	validateInstall := fs.Bool("validate-install", false, "validate install")
	showVersion := fs.Bool("version", false, "print version")
	markdownOut := fs.Bool("markdown", false, "print Markdown output when supported")
	agentNotes := fs.Bool("agent-notes", false, "include AI-agent guidance in Markdown output")
	exportActions := fs.String("export-actions", "", "write action catalog to path")
	explainAction := fs.String("explain-action", "", "explain one action by name")
	printURL := fs.String("print-url", "", "print an rrunner:// URL for an action")
	markdownLink := fs.String("markdown-link", "", "wrap --print-url output as a Markdown link")
	urlApp := fs.String("app", "", "app name or bundle id for --print-url")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *showVersion {
		fmt.Println(version)
		return 0
	}

	cfg := loadConfig()
	reg := buildRegistry(cfg)

	if *diagnose || *validateInstall {
		report := diagnostics(cfg, reg)
		if *jsonOut {
			printJSON(report)
		} else {
			printDiagnostics(report)
		}
		if len(cfg.Errors)+len(reg.Errors) > 0 && *validateInstall {
			return 1
		}
		return 0
	}

	if *explainAction != "" {
		explained, err := explainActionSpec(reg, *explainAction)
		if err != nil {
			userError(err.Error())
			return 1
		}
		if *jsonOut {
			printJSON(explained)
		} else if *markdownOut {
			fmt.Print(renderActionExplanationMarkdown(explained))
		} else {
			fmt.Print(renderActionExplanationText(explained))
		}
		return 0
	}

	if *listActions || *exportActions != "" {
		items := registryList(reg)
		if *exportActions != "" {
			body := renderActionCatalogMarkdown(items, *agentNotes)
			if err := os.WriteFile(expandPath(*exportActions), []byte(body), 0644); err != nil {
				userError(err.Error())
				return 1
			}
			fmt.Println(expandPath(*exportActions))
			return 0
		}
		if *jsonOut {
			printJSON(items)
		} else if *markdownOut {
			fmt.Print(renderActionCatalogMarkdown(items, *agentNotes))
		} else {
			for _, a := range items {
				shadow := ""
				if a.Shadowed {
					shadow = " (shadowed)"
				}
				fmt.Printf("%-24s %-10s %-12s %-18s%s\n", a.Name, a.Type, actionRisk(a), a.Source, shadow)
			}
		}
		return 0
	}

	pos := fs.Args()
	if *printURL != "" {
		path := ""
		if len(pos) > 0 {
			path = pos[0]
		}
		built, err := buildRrunnerURL(*printURL, path, *urlApp)
		if err != nil {
			userError(err.Error())
			return 1
		}
		if *markdownLink != "" {
			fmt.Printf("[%s](<%s>)\n", *markdownLink, built)
		} else {
			fmt.Println(built)
		}
		return 0
	}

	if len(pos) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: rrunner-core [--dry-run] [--json] 'rrunner://action?url=file:///path'")
		return 2
	}

	req, err := parseRequest(pos[0])
	if err != nil {
		userError(err.Error())
		return 1
	}
	if queryMap(req.Query).GetBool("_rrunner_dry_run") {
		*dryRun = true
	}
	if req.Action == "diagnostics" {
		return showInTerminalOrPrint(cfg, "Rrunner diagnostics", diagnosticsText(diagnostics(cfg, reg)))
	}
	if req.Action == "list-actions" {
		var b strings.Builder
		for _, a := range registryList(reg) {
			fmt.Fprintf(&b, "%-24s %-10s %s\n", a.Name, a.Type, a.Source)
		}
		return showInTerminalOrPrint(cfg, "Rrunner actions", b.String())
	}

	plan, err := planAction(cfg, reg, req, *dryRun)
	if err != nil {
		userError(err.Error())
		logEvent(cfg, "error", "action.failed", map[string]any{"action": req.Action, "error": err.Error()})
		return 1
	}
	if *dryRun {
		if *jsonOut {
			printJSON(plan)
		} else {
			printPlan(plan)
		}
		logEvent(cfg, "info", "action.dry_run", map[string]any{"action": plan.Action, "source": plan.Source})
		return 0
	}
	if spec, ok := reg.Actions[plan.Action]; ok && spec.Confirm {
		if err := confirmAction(plan, spec); err != nil {
			userError(err.Error())
			logEvent(cfg, "info", "action.cancelled", map[string]any{"action": plan.Action, "source": plan.Source, "error": err.Error()})
			return 1
		}
	}
	if err := executePlan(cfg, plan); err != nil {
		userError(err.Error())
		logEvent(cfg, "error", "action.failed", map[string]any{"action": plan.Action, "source": plan.Source, "error": err.Error()})
		return 1
	}
	logEvent(cfg, "info", "action.executed", map[string]any{"action": plan.Action, "source": plan.Source})
	return 0
}

func defaultConfigPath() string {
	if v := os.Getenv("RRUNNER_CONFIG_TOML"); v != "" {
		return expandPath(v)
	}
	return expandPath("~/.config/rrunner/config.toml")
}

func loadConfig() Config {
	cfg := Config{
		Path: defaultConfigPath(),
		Settings: Settings{
			TerminalApp:       "Ghostty",
			KeepOpen:          true,
			RemoteBase:        "https://raw.githubusercontent.com/deathrashed/rrunner/main",
			HandlersDir:       "~/.config/rrunner/handlers",
			TextEditor:        "com.coteditor.CotEditor",
			CodeEditor:        "com.microsoft.VSCode",
			MarkdownPreviewer: "com.brettterpstra.marked2",
		},
		Plugins: PluginSettings{
			Enabled:  true,
			Dirs:     []string{"~/.config/rrunner/plugins", "~/.local/share/rrunner/plugins"},
			Disabled: map[string]bool{},
			Trusted:  map[string]bool{},
		},
		Security: SecuritySettings{AllowInlineCommands: true, AllowPluginCommands: false, RequireTrustedPluginsForCommands: true, AllowLegacyHandlers: true},
		Logging:  LoggingSettings{Level: "info", File: "~/Library/Logs/Rrunner/rrunner.log"},
		Actions:  map[string]ActionSpec{},
	}
	applyEnv(&cfg)
	if _, err := os.Stat(cfg.Path); err != nil {
		return cfg
	}
	mt, err := parseMiniTOMLFile(cfg.Path)
	if err != nil {
		cfg.Errors = append(cfg.Errors, err.Error())
		return cfg
	}
	cfg.Errors = append(cfg.Errors, mt.Errors...)
	applyConfigTOML(&cfg, mt, "local.config", filepath.Dir(cfg.Path))
	return cfg
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("RRUNNER_TERMINAL_APP"); v != "" {
		cfg.Settings.TerminalApp = v
	}
	if v := os.Getenv("RRUNNER_KEEP_OPEN"); v != "" {
		cfg.Settings.KeepOpen = v != "0" && strings.ToLower(v) != "false"
	}
	if v := os.Getenv("RRUNNER_REMOTE_BASE"); v != "" {
		cfg.Settings.RemoteBase = v
	}
	if v := os.Getenv("RRUNNER_REMOTE_URL"); v != "" {
		cfg.Settings.RemoteURL = v
	}
	if v := os.Getenv("RRUNNER_RESTORE_URL"); v != "" {
		cfg.Settings.RestoreURL = v
	}
	if v := os.Getenv("RRUNNER_HANDLERS_DIR"); v != "" {
		cfg.Settings.HandlersDir = v
	}
}

func applyConfigTOML(cfg *Config, mt miniTOML, source, pluginDir string) {
	if s := mt.Sections["settings"]; s != nil {
		if v, ok := strVal(s, "terminal_app"); ok {
			cfg.Settings.TerminalApp = v
		}
		if v, ok := boolVal(s, "keep_open"); ok {
			cfg.Settings.KeepOpen = v
		}
		if v, ok := strVal(s, "remote_base"); ok {
			cfg.Settings.RemoteBase = v
		}
		if v, ok := strVal(s, "remote_url"); ok {
			cfg.Settings.RemoteURL = v
		}
		if v, ok := strVal(s, "restore_url"); ok {
			cfg.Settings.RestoreURL = v
		}
		if v, ok := strVal(s, "handlers_dir"); ok {
			cfg.Settings.HandlersDir = v
		}
		if v, ok := strVal(s, "text_editor"); ok {
			cfg.Settings.TextEditor = v
		}
		if v, ok := strVal(s, "code_editor"); ok {
			cfg.Settings.CodeEditor = v
		}
		if v, ok := strVal(s, "markdown_previewer"); ok {
			cfg.Settings.MarkdownPreviewer = v
		}
	}
	if cfg.Settings.RestoreURL == "" {
		cfg.Settings.RestoreURL = strings.TrimRight(cfg.Settings.RemoteBase, "/") + "/bin/md-restore.sh"
	}
	if s := mt.Sections["plugins"]; s != nil {
		if v, ok := boolVal(s, "enabled"); ok {
			cfg.Plugins.Enabled = v
		}
		if v, ok := strSliceVal(s, "dirs"); ok {
			cfg.Plugins.Dirs = v
		}
		if v, ok := strSliceVal(s, "disabled"); ok {
			cfg.Plugins.Disabled = sliceSet(v)
		}
		if v, ok := strSliceVal(s, "trusted"); ok {
			cfg.Plugins.Trusted = sliceSet(v)
		}
	}
	if s := mt.Sections["security"]; s != nil {
		if v, ok := boolVal(s, "allow_inline_commands"); ok {
			cfg.Security.AllowInlineCommands = v
		}
		if v, ok := boolVal(s, "allow_plugin_commands"); ok {
			cfg.Security.AllowPluginCommands = v
		}
		if v, ok := boolVal(s, "require_trusted_plugins_for_commands"); ok {
			cfg.Security.RequireTrustedPluginsForCommands = v
		}
		if v, ok := boolVal(s, "allow_legacy_handlers"); ok {
			cfg.Security.AllowLegacyHandlers = v
		}
	}
	if s := mt.Sections["logging"]; s != nil {
		if v, ok := strVal(s, "level"); ok {
			cfg.Logging.Level = v
		}
		if v, ok := strVal(s, "file"); ok {
			cfg.Logging.File = v
		}
	}
	for name, s := range mt.Sections {
		if !strings.HasPrefix(name, "actions.") {
			continue
		}
		actionName := strings.TrimPrefix(name, "actions.")
		if isReservedAction(actionName) && source != "rrunner.core" {
			cfg.Errors = append(cfg.Errors, "reserved action ignored: "+actionName)
			continue
		}
		spec := actionFromSection(actionName, s, source, pluginDir)
		cfg.Actions[actionName] = spec
		for _, alias := range spec.Aliases {
			aliasSpec := spec
			aliasSpec.Name = alias
			cfg.Actions[alias] = aliasSpec
		}
	}
}

func actionFromSection(name string, s map[string]any, source, pluginDir string) ActionSpec {
	spec := ActionSpec{Name: name, Source: source, PluginID: source, PluginDir: pluginDir, PathPolicy: ""}
	spec.Type, _ = strVal(s, "type")
	spec.Type = strings.ToLower(spec.Type)
	spec.Description, _ = strVal(s, "description")
	spec.App, _ = strVal(s, "app")
	spec.Runner, _ = strVal(s, "runner")
	spec.Script, _ = strVal(s, "script")
	spec.Command, _ = strVal(s, "command")
	spec.PathPolicy, _ = strVal(s, "path_policy")
	if v, ok := boolVal(s, "confirm"); ok {
		spec.Confirm = v
	}
	spec.Aliases, _ = strSliceVal(s, "aliases")
	return spec
}

func discoverPlugins(cfg Config) []PluginManifest {
	if !cfg.Plugins.Enabled {
		return nil
	}
	var manifests []PluginManifest
	for _, dir := range cfg.Plugins.Dirs {
		dir = expandPath(dir)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, e := range entries {
			p := filepath.Join(dir, e.Name())
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".plugin.toml") {
				if m, ok := loadPluginManifest(p); ok {
					manifests = append(manifests, m)
				}
			}
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			p := filepath.Join(dir, e.Name(), "plugin.toml")
			if _, err := os.Stat(p); err == nil {
				if m, ok := loadPluginManifest(p); ok {
					manifests = append(manifests, m)
				}
			}
		}
	}
	return manifests
}

func loadPluginManifest(path string) (PluginManifest, bool) {
	mt, err := parseMiniTOMLFile(path)
	m := PluginManifest{Path: path, Dir: filepath.Dir(path), Enabled: true, Actions: map[string]ActionSpec{}}
	if err != nil {
		m.Errors = append(m.Errors, err.Error())
		return m, false
	}
	m.Errors = append(m.Errors, mt.Errors...)
	p := mt.Sections["plugin"]
	if p == nil {
		m.Errors = append(m.Errors, "missing [plugin]")
		return m, false
	}
	m.ID, _ = strVal(p, "id")
	m.Name, _ = strVal(p, "name")
	m.Version, _ = strVal(p, "version")
	if enabled, ok := boolVal(p, "enabled"); ok {
		m.Enabled = enabled
	}
	if m.ID == "" {
		m.Errors = append(m.Errors, "plugin id is required")
		return m, false
	}
	for name, s := range mt.Sections {
		if !strings.HasPrefix(name, "actions.") {
			continue
		}
		actionName := strings.TrimPrefix(name, "actions.")
		if isReservedAction(actionName) {
			m.Errors = append(m.Errors, "reserved action ignored: "+actionName)
			continue
		}
		spec := actionFromSection(actionName, s, m.ID, m.Dir)
		m.Actions[actionName] = spec
	}
	return m, true
}

func buildRegistry(cfg Config) Registry {
	reg := Registry{Actions: map[string]ActionSpec{}, Errors: append([]string{}, cfg.Errors...)}
	add := func(spec ActionSpec) {
		if spec.Type == "" {
			reg.Errors = append(reg.Errors, "action missing type: "+spec.Name)
			return
		}
		if _, exists := reg.Actions[spec.Name]; exists {
			spec.Shadowed = true
			reg.Shadowed = append(reg.Shadowed, spec)
			return
		}
		reg.Actions[spec.Name] = spec
	}
	keys := sortedActionKeys(cfg.Actions)
	for _, k := range keys {
		add(cfg.Actions[k])
	}
	seenPlugins := map[string]bool{}
	for _, p := range discoverPlugins(cfg) {
		event := PluginEvent{Path: p.Path, ID: p.ID, Status: "loaded"}
		if len(p.Errors) > 0 {
			event.Reason = strings.Join(p.Errors, "; ")
		}
		if seenPlugins[p.ID] {
			reg.Errors = append(reg.Errors, "duplicate plugin ignored: "+p.ID)
			event.Status = "skipped"
			event.Reason = "duplicate plugin id"
			reg.PluginEvents = append(reg.PluginEvents, event)
			continue
		}
		seenPlugins[p.ID] = true
		if cfg.Plugins.Disabled[p.ID] || !p.Enabled {
			event.Status = "skipped"
			if cfg.Plugins.Disabled[p.ID] {
				event.Reason = "disabled in config"
			} else {
				event.Reason = "plugin enabled=false"
			}
			reg.PluginEvents = append(reg.PluginEvents, event)
			continue
		}
		reg.Plugins = append(reg.Plugins, p)
		for _, k := range sortedActionKeys(p.Actions) {
			s := p.Actions[k]
			if s.Type == "command" && (!cfg.Security.AllowPluginCommands || (cfg.Security.RequireTrustedPluginsForCommands && !cfg.Plugins.Trusted[p.ID])) {
				reason := "plugin command action blocked: " + p.ID + "." + s.Name
				reg.Errors = append(reg.Errors, reason)
				reg.PluginEvents = append(reg.PluginEvents, PluginEvent{Path: p.Path, ID: p.ID, Status: "blocked", Reason: reason})
				continue
			}
			add(s)
		}
		reg.PluginEvents = append(reg.PluginEvents, event)
	}
	if cfg.Security.AllowLegacyHandlers {
		for _, name := range legacyHandlerActions(cfg.Settings.HandlersDir) {
			add(ActionSpec{Name: name, Type: "legacy-handler", Source: "legacy.handlers", PluginID: "legacy.handlers"})
		}
	}
	for _, s := range builtinActions() {
		add(s)
	}
	return reg
}

func builtinActions() []ActionSpec {
	names := []string{"open", "reveal", "show", "openwith", "view", "launch", "auto", "osascript", "bash", "zsh", "python", "node", "ruby", "perl", "restore", "diagnostics", "list-actions"}
	var out []ActionSpec
	for _, n := range names {
		t := n
		if contains([]string{"osascript", "bash", "zsh", "python", "node", "ruby", "perl"}, n) {
			t = "run"
		}
		out = append(out, ActionSpec{Name: n, Type: t, Source: "rrunner.core", PluginID: "rrunner.core"})
	}
	return out
}

func legacyHandlerActions(dir string) []string {
	dir = expandPath(dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if st, err := os.Stat(p); err == nil && st.Mode()&0111 != 0 {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

type queryMap map[string][]string

func (q queryMap) first(k string) string {
	if v := q[k]; len(v) > 0 {
		return v[0]
	}
	return ""
}
func (q queryMap) GetBool(k string) bool {
	v := strings.ToLower(q.first(k))
	return v == "1" || v == "true" || v == "yes"
}

func parseRequest(raw string) (Request, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return Request{}, err
	}
	q := queryMap(u.Query())
	action := u.Host
	if action == "" {
		action = strings.Trim(strings.Split(strings.Trim(u.Path, "/"), "/")[0], " ")
	}
	if action == "" {
		return Request{}, errors.New("no Rrunner action supplied")
	}
	path := ""
	if v := q.first("url"); v != "" {
		path = fileURLToPath(v)
	} else if v := q.first("path"); v != "" {
		path, _ = url.QueryUnescape(v)
	}
	return Request{Raw: raw, Action: action, Path: path, App: q.first("app"), Query: map[string][]string(q)}, nil
}

func fileURLToPath(v string) string {
	u, err := url.Parse(v)
	if err == nil && u.Scheme == "file" {
		p, _ := url.PathUnescape(u.Path)
		return p
	}
	p, _ := url.QueryUnescape(v)
	return p
}

func planAction(cfg Config, reg Registry, req Request, dry bool) (ExecutionPlan, error) {
	spec, ok := reg.Actions[req.Action]
	if !ok {
		return ExecutionPlan{}, fmt.Errorf("unknown Rrunner action: %s", req.Action)
	}
	if spec.Type == "legacy-handler" {
		return planLegacy(cfg, spec, req, dry), nil
	}
	if spec.Source == "local.config" && spec.Type == "command" && !cfg.Security.AllowInlineCommands {
		return ExecutionPlan{}, errors.New("inline command actions are disabled by security config")
	}
	if spec.Source == "rrunner.core" && contains([]string{"osascript", "bash", "zsh", "python", "node", "ruby", "perl"}, spec.Name) {
		spec.Type = "run"
		spec.Runner = spec.Name
	}
	if spec.Type == "auto" {
		return planAuto(cfg, reg, req, dry)
	}
	if err := validatePathPolicy(spec, req); err != nil {
		return ExecutionPlan{}, err
	}
	plan := ExecutionPlan{Action: req.Action, Source: spec.Source, DryRun: dry}
	env := actionEnv(spec, req)
	switch spec.Type {
	case "open":
		plan.Steps = append(plan.Steps, PlanStep{Kind: "open", Command: "/usr/bin/open", Args: []string{req.Path}})
	case "reveal", "show":
		plan.Steps = append(plan.Steps, PlanStep{Kind: "reveal", Command: "/usr/bin/open", Args: []string{"-R", req.Path}})
	case "openwith", "view":
		app := firstNonEmpty(spec.App, req.App)
		if app == "" {
			return plan, errors.New("openwith/view requires app in config or URL")
		}
		args := []string{"-a", app, req.Path}
		if strings.Contains(app, ".") {
			args = []string{"-b", app, req.Path}
		}
		plan.Steps = append(plan.Steps, PlanStep{Kind: "openwith", Command: "/usr/bin/open", Args: args})
	case "launch":
		app := firstNonEmpty(spec.App, req.App)
		if app == "" {
			return plan, errors.New("launch requires app in config or URL")
		}
		args := []string{"-a", app}
		if strings.Contains(app, ".") {
			args = []string{"-b", app}
		}
		plan.Steps = append(plan.Steps, PlanStep{Kind: "launch", Command: "/usr/bin/open", Args: args})
	case "run":
		runner := firstNonEmpty(spec.Runner, spec.Name)
		if !validRunner(runner) {
			return plan, fmt.Errorf("unsupported runner: %s", runner)
		}
		cmd := runnerCommand(runner) + " " + shellQuote(req.Path)
		plan.Steps = append(plan.Steps, PlanStep{Kind: "terminal", Command: "terminal", Args: []string{cmd}, Env: env})
	case "script":
		script := resolveScript(spec)
		if script == "" {
			return plan, errors.New("script action requires script")
		}
		if _, err := os.Stat(script); err != nil {
			return plan, fmt.Errorf("configured script does not exist: %s", script)
		}
		runner := spec.Runner
		if runner != "" && !validRunner(runner) {
			return plan, fmt.Errorf("unsupported script runner: %s", runner)
		}
		cmd := shellQuote(script)
		if runner != "" {
			cmd = runnerCommand(runner) + " " + cmd
		}
		if req.Path != "" {
			cmd += " " + shellQuote(req.Path)
		}
		plan.Steps = append(plan.Steps, PlanStep{Kind: "terminal", Command: "terminal", Args: []string{cmd}, Env: env})
	case "command":
		if spec.Command == "" {
			return plan, errors.New("command action requires command")
		}
		plan.Steps = append(plan.Steps, PlanStep{Kind: "terminal", Command: "terminal", Args: []string{spec.Command}, Env: env})
	case "restore":
		restoreScript := resolveRestoreScript(cfg)
		if !dry {
			localRestore, err := ensureRestoreScript(cfg)
			if err != nil {
				return plan, err
			}
			restoreScript = localRestore
		}
		cmd := "bash " + shellQuote(restoreScript) + " restore " + shellQuote(req.Path)
		plan.Steps = append(plan.Steps, PlanStep{Kind: "terminal", Command: "terminal", Args: []string{cmd}, Env: env})
	default:
		return plan, fmt.Errorf("unsupported action type: %s", spec.Type)
	}
	return plan, nil
}

func planLegacy(cfg Config, spec ActionSpec, req Request, dry bool) ExecutionPlan {
	handler := filepath.Join(expandPath(cfg.Settings.HandlersDir), spec.Name)
	return ExecutionPlan{Action: req.Action, Source: spec.Source, DryRun: dry, Steps: []PlanStep{{Kind: "legacy-handler", Command: handler, Env: actionEnv(spec, req)}}}
}

func planAuto(cfg Config, reg Registry, req Request, dry bool) (ExecutionPlan, error) {
	if req.Path == "" {
		return ExecutionPlan{}, errors.New("auto requires a file payload")
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(req.Path), "."))
	mapping := map[string]string{"applescript": "osascript", "scpt": "osascript", "sh": "bash", "bash": "bash", "command": "bash", "zsh": "zsh", "py": "python", "python": "python", "js": "node", "mjs": "node", "cjs": "node", "rb": "ruby", "pl": "perl", "pm": "perl", "md": "restore", "markdown": "restore"}
	next := mapping[ext]
	if next == "" {
		return ExecutionPlan{}, fmt.Errorf("no auto-run handler for: %s", req.Path)
	}
	req.Action = next
	return planAction(cfg, reg, req, dry)
}

func validatePathPolicy(spec ActionSpec, req Request) error {
	policy := spec.PathPolicy
	if policy == "" {
		switch spec.Type {
		case "launch", "command", "script":
			policy = "any"
		default:
			policy = "exists"
		}
	}
	switch policy {
	case "none":
		return nil
	case "any":
		return nil
	case "exists", "file", "directory":
		if req.Path == "" {
			return fmt.Errorf("action %s requires a file url/path", spec.Name)
		}
		st, err := os.Stat(req.Path)
		if err != nil {
			return fmt.Errorf("path does not exist: %s", req.Path)
		}
		if policy == "file" && st.IsDir() {
			return fmt.Errorf("path is not a file: %s", req.Path)
		}
		if policy == "directory" && !st.IsDir() {
			return fmt.Errorf("path is not a directory: %s", req.Path)
		}
		return nil
	default:
		return fmt.Errorf("unknown path_policy for %s: %s", spec.Name, policy)
	}
}

func executePlan(cfg Config, plan ExecutionPlan) error {
	for _, step := range plan.Steps {
		switch step.Kind {
		case "open", "reveal", "openwith", "launch":
			cmd := exec.Command(step.Command, step.Args...)
			if err := cmd.Start(); err != nil {
				return err
			}
		case "terminal":
			if len(step.Args) == 0 {
				return errors.New("terminal step missing command")
			}
			return runInTerminal(cfg, step.Args[0], step.Env)
		case "legacy-handler":
			cmd := exec.Command(step.Command)
			cmd.Env = append(os.Environ(), envMapToList(step.Env)...)
			return cmd.Run()
		default:
			return fmt.Errorf("unknown plan step kind: %s", step.Kind)
		}
	}
	return nil
}

func runInTerminal(cfg Config, command string, env map[string]string) error {
	dir, err := os.MkdirTemp("", "rrunner.*")
	if err != nil {
		return err
	}
	script := filepath.Join(dir, "run.zsh")
	var b strings.Builder
	b.WriteString("#!/bin/zsh\nclear\necho Rrunner\necho\n")
	for k, v := range env {
		fmt.Fprintf(&b, "export %s=%s\n", k, shellQuote(v))
	}
	b.WriteString(command + "\nstatus=$?\necho\necho \"Exit status: $status\"\n")
	if cfg.Settings.KeepOpen {
		b.WriteString("echo\necho \"Press Return to close...\"\nread -r _\n")
	}
	b.WriteString("exit $status\n")
	if err := os.WriteFile(script, []byte(b.String()), 0700); err != nil {
		return err
	}
	if _, err := exec.LookPath("ghostty"); err == nil {
		return exec.Command("ghostty", "-e", "/bin/zsh", script).Start()
	}
	if err := exec.Command("/usr/bin/open", "-a", cfg.Settings.TerminalApp, script).Start(); err == nil {
		return nil
	}
	return exec.Command("/usr/bin/open", "-a", "Terminal", script).Start()
}

func showInTerminalOrPrint(cfg Config, title, text string) int {
	cmd := "cat " + shellQuote(writeTempText(text))
	if err := runInTerminal(cfg, cmd, map[string]string{}); err != nil {
		fmt.Println(text)
		return 1
	}
	return 0
}
func writeTempText(text string) string {
	f, _ := os.CreateTemp("", "rrunner-report.*.txt")
	_, _ = f.WriteString(text)
	_ = f.Close()
	return f.Name()
}

func parseMiniTOMLFile(path string) (miniTOML, error) {
	f, err := os.Open(path)
	if err != nil {
		return miniTOML{}, err
	}
	defer f.Close()
	mt := miniTOML{Sections: map[string]map[string]any{}}
	section := ""
	s := bufio.NewScanner(f)
	lineNo := 0
	for s.Scan() {
		lineNo++
		line := strings.TrimSpace(stripComment(s.Text()))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.Contains(line, "]") {
			end := strings.Index(line, "]")
			section = strings.TrimSpace(line[1:end])
			mt.Sections[section] = map[string]any{}
			continue
		}
		idx := strings.Index(line, "=")
		if idx < 0 || section == "" {
			mt.Errors = append(mt.Errors, fmt.Sprintf("%s:%d unsupported TOML line", path, lineNo))
			continue
		}
		key := strings.TrimSpace(line[:idx])
		rawValue := strings.TrimSpace(line[idx+1:])
		if strings.HasPrefix(rawValue, "[") && !strings.Contains(rawValue, "]") {
			for s.Scan() {
				lineNo++
				next := strings.TrimSpace(stripComment(s.Text()))
				if next == "" {
					continue
				}
				rawValue += " " + next
				if strings.Contains(next, "]") {
					break
				}
			}
		}
		val, err := parseValue(rawValue)
		if err != nil {
			mt.Errors = append(mt.Errors, fmt.Sprintf("%s:%d %v", path, lineNo, err))
			continue
		}
		mt.Sections[section][strings.ReplaceAll(key, "-", "_")] = val
	}
	return mt, s.Err()
}

func stripComment(line string) string {
	quote := rune(0)
	esc := false
	for i, r := range line {
		if quote != 0 {
			if esc {
				esc = false
				continue
			}
			if r == '\\' && quote == '"' {
				esc = true
				continue
			}
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == '#' {
			return line[:i]
		}
	}
	return line
}

func parseValue(v string) (any, error) {
	v = strings.TrimSpace(v)
	if v == "true" {
		return true, nil
	}
	if v == "false" {
		return false, nil
	}
	if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
		inner := strings.TrimSpace(v[1 : len(v)-1])
		if inner == "" {
			return []string{}, nil
		}
		parts := splitCSV(inner)
		out := []string{}
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			sv, err := parseString(p)
			if err != nil {
				return nil, err
			}
			out = append(out, sv)
		}
		return out, nil
	}
	if strings.HasPrefix(v, "'") || strings.HasPrefix(v, "\"") {
		return parseString(v)
	}
	if i, err := strconv.Atoi(v); err == nil {
		return i, nil
	}
	return v, nil
}

func parseString(v string) (string, error) {
	if len(v) < 2 {
		return "", errors.New("invalid string")
	}
	q := v[0]
	if (q != '\'' && q != '"') || v[len(v)-1] != q {
		return "", errors.New("invalid quoted string")
	}
	body := v[1 : len(v)-1]
	if q == '\'' {
		return body, nil
	}
	return strconv.Unquote(v)
}

func splitCSV(s string) []string {
	var out []string
	start := 0
	quote := rune(0)
	esc := false
	for i, r := range s {
		if quote != 0 {
			if esc {
				esc = false
				continue
			}
			if r == '\\' && quote == '"' {
				esc = true
				continue
			}
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == ',' {
			out = append(out, strings.TrimSpace(s[start:i]))
			start = i + 1
		}
	}
	out = append(out, strings.TrimSpace(s[start:]))
	return out
}

type ActionExplanation struct {
	Action     ActionSpec   `json:"action"`
	Risk       string       `json:"risk"`
	ExampleURL  string       `json:"example_url"`
	ShadowedBy []ActionSpec `json:"shadowed_by,omitempty"`
	Warnings   []string     `json:"warnings,omitempty"`
}

func actionRisk(a ActionSpec) string {
	t := a.Type
	if t == "run" || t == "script" || t == "command" || t == "legacy-handler" || contains([]string{"osascript", "bash", "zsh", "python", "node", "ruby", "perl"}, a.Name) {
		return "executable"
	}
	if t == "restore" {
		return "filesystem"
	}
	return "passive"
}

func buildRrunnerURL(action, path, app string) (string, error) {
	action = strings.TrimSpace(action)
	if action == "" {
		return "", errors.New("action is required")
	}
	u := url.URL{Scheme: "rrunner", Host: action}
	q := url.Values{}
	if path != "" {
		abs := expandPath(path)
		if !strings.HasPrefix(abs, "/") {
			if cwd, err := os.Getwd(); err == nil {
				abs = filepath.Join(cwd, abs)
			}
		}
		fileURL := url.URL{Scheme: "file", Path: abs}
		q.Set("url", fileURL.String())
	}
	if app != "" {
		q.Set("app", app)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func renderActionCatalogMarkdown(items []ActionSpec, agentNotes bool) string {
	var b strings.Builder
	b.WriteString("# Rrunner Action Catalog\n\n")
	b.WriteString("Generated from the live Rrunner action registry. Review local paths before sharing.\n\n")
	if agentNotes {
		b.WriteString("> [!warning] Agent note\n")
		b.WriteString("> Actions marked `executable` can run local scripts or shell commands. Prefer `--dry-run` before executing unfamiliar links.\n\n")
	}
	b.WriteString("| Action | Type | Risk | Source | Description | Example |\n")
	b.WriteString("|---|---|---|---|---|---|\n")
	for _, a := range items {
		example, _ := buildRrunnerURL(a.Name, "/path/to/file", a.App)
		desc := a.Description
		if desc == "" {
			desc = "—"
		}
		b.WriteString(fmt.Sprintf("| `%s` | `%s` | `%s` | `%s` | %s | [`%s`](<%s>) |\n", a.Name, a.Type, actionRisk(a), a.Source, escapeMarkdownCell(desc), example, example))
	}
	return b.String()
}

func escapeMarkdownCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func explainActionSpec(reg Registry, name string) (ActionExplanation, error) {
	spec, ok := reg.Actions[name]
	if !ok {
		return ActionExplanation{}, fmt.Errorf("unknown Rrunner action: %s", name)
	}
	ex := ActionExplanation{Action: spec, Risk: actionRisk(spec)}
	ex.ExampleURL, _ = buildRrunnerURL(spec.Name, "/path/to/file", spec.App)
	for _, s := range reg.Shadowed {
		if s.Name == name {
			ex.ShadowedBy = append(ex.ShadowedBy, s)
		}
	}
	if ex.Risk == "executable" {
		ex.Warnings = append(ex.Warnings, "This action can execute local code. Use --dry-run or confirm=true for safer links.")
	}
	if spec.Confirm {
		ex.Warnings = append(ex.Warnings, "This action asks for confirmation before execution.")
	}
	return ex, nil
}

func renderActionExplanationText(ex ActionExplanation) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Action: %s\n", ex.Action.Name)
	fmt.Fprintf(&b, "Type: %s\n", ex.Action.Type)
	fmt.Fprintf(&b, "Risk: %s\n", ex.Risk)
	fmt.Fprintf(&b, "Source: %s\n", ex.Action.Source)
	if ex.Action.PluginID != "" {
		fmt.Fprintf(&b, "Plugin: %s\n", ex.Action.PluginID)
	}
	if ex.Action.Description != "" {
		fmt.Fprintf(&b, "Description: %s\n", ex.Action.Description)
	}
	fmt.Fprintf(&b, "Example: %s\n", ex.ExampleURL)
	for _, w := range ex.Warnings {
		fmt.Fprintf(&b, "Warning: %s\n", w)
	}
	if len(ex.ShadowedBy) > 0 {
		b.WriteString("Shadowed lower-priority actions:\n")
		for _, s := range ex.ShadowedBy {
			fmt.Fprintf(&b, "- %s from %s\n", s.Name, s.Source)
		}
	}
	return b.String()
}

func renderActionExplanationMarkdown(ex ActionExplanation) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Rrunner Action: `%s`\n\n", ex.Action.Name)
	fmt.Fprintf(&b, "- **Type:** `%s`\n", ex.Action.Type)
	fmt.Fprintf(&b, "- **Risk:** `%s`\n", ex.Risk)
	fmt.Fprintf(&b, "- **Source:** `%s`\n", ex.Action.Source)
	if ex.Action.PluginID != "" {
		fmt.Fprintf(&b, "- **Plugin:** `%s`\n", ex.Action.PluginID)
	}
	if ex.Action.Description != "" {
		fmt.Fprintf(&b, "- **Description:** %s\n", ex.Action.Description)
	}
	fmt.Fprintf(&b, "- **Example:** [`%s`](<%s>)\n", ex.ExampleURL, ex.ExampleURL)
	if len(ex.Warnings) > 0 {
		b.WriteString("\n> [!warning]\n")
		for _, w := range ex.Warnings {
			fmt.Fprintf(&b, "> %s\n", w)
		}
	}
	if len(ex.ShadowedBy) > 0 {
		b.WriteString("\n## Shadowed lower-priority actions\n\n")
		for _, s := range ex.ShadowedBy {
			fmt.Fprintf(&b, "- `%s` from `%s`\n", s.Name, s.Source)
		}
	}
	return b.String()
}

func confirmAction(plan ExecutionPlan, spec ActionSpec) error {
	message := fmt.Sprintf("Run Rrunner action '%s' from %s?", plan.Action, spec.Source)
	script := `display dialog ` + shellQuoteAppleScript(message) + ` buttons {"Cancel", "Run"} default button "Run" cancel button "Cancel" with icon caution`
	cmd := exec.Command("/usr/bin/osascript", "-e", script)
	if err := cmd.Run(); err != nil {
		return errors.New("action cancelled")
	}
	return nil
}

func shellQuoteAppleScript(s string) string {
	return "\"" + strings.ReplaceAll(s, "\"", "\\\"") + "\""
}

func diagnostics(cfg Config, reg Registry) map[string]any {
	return map[string]any{
		"version":        version,
		"core_path":      os.Args[0],
		"config_path":    cfg.Path,
		"config_exists":  fileExists(cfg.Path),
		"terminal_app":   cfg.Settings.TerminalApp,
		"handlers_dir":   expandPath(cfg.Settings.HandlersDir),
		"plugin_dirs":    cfg.Plugins.Dirs,
		"plugins_loaded": len(reg.Plugins),
		"actions":        len(reg.Actions),
		"shadowed":       reg.Shadowed,
		"plugin_events":  reg.PluginEvents,
		"security":       cfg.Security,
		"logging":        cfg.Logging,
		"errors":         append(cfg.Errors, reg.Errors...),
	}
}
func diagnosticsText(m map[string]any) string {
	b, _ := json.MarshalIndent(m, "", "  ")
	return string(b) + "\n"
}
func printDiagnostics(m map[string]any) { fmt.Print(diagnosticsText(m)) }
func printJSON(v any)                   { b, _ := json.MarshalIndent(v, "", "  "); fmt.Println(string(b)) }
func printPlan(p ExecutionPlan) {
	fmt.Printf("Action: %s\nSource: %s\nDry-run: %v\n", p.Action, p.Source, p.DryRun)
	for _, s := range p.Steps {
		fmt.Printf("- %s: %s %s\n", s.Kind, s.Command, strings.Join(s.Args, " "))
	}
}

func registryList(reg Registry) []ActionSpec {
	out := []ActionSpec{}
	for _, k := range sortedActionKeys(reg.Actions) {
		out = append(out, reg.Actions[k])
	}
	out = append(out, reg.Shadowed...)
	return out
}
func sortedActionKeys(m map[string]ActionSpec) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
func sliceSet(xs []string) map[string]bool {
	m := map[string]bool{}
	for _, x := range xs {
		m[x] = true
	}
	return m
}
func strVal(m map[string]any, k string) (string, bool) {
	v, ok := m[k]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}
func boolVal(m map[string]any, k string) (bool, bool) {
	v, ok := m[k]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}
func strSliceVal(m map[string]any, k string) ([]string, bool) {
	v, ok := m[k]
	if !ok {
		return nil, false
	}
	s, ok := v.([]string)
	return s, ok
}
func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}
func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if x != "" {
			return x
		}
	}
	return ""
}
func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }
func isReservedAction(n string) bool {
	return n == "dry-run" || n == "diagnostics" || n == "list-actions"
}
func validRunner(r string) bool {
	return contains([]string{"osascript", "bash", "zsh", "python", "node", "ruby", "perl"}, r)
}
func runnerCommand(r string) string {
	if r == "python" {
		return "python3"
	}
	return r
}
func resolveScript(s ActionSpec) string {
	if s.Script == "" {
		return ""
	}
	if filepath.IsAbs(expandPath(s.Script)) {
		return expandPath(s.Script)
	}
	if s.PluginDir != "" {
		return filepath.Join(s.PluginDir, s.Script)
	}
	return expandPath(s.Script)
}
func resolveRestoreScript(cfg Config) string {
	if strings.HasPrefix(cfg.Settings.RestoreURL, "file://") {
		return fileURLToPath(cfg.Settings.RestoreURL)
	}
	return filepath.Join(cacheDir(), "md-restore.sh")
}

func ensureRestoreScript(cfg Config) (string, error) {
	if strings.HasPrefix(cfg.Settings.RestoreURL, "file://") {
		path := fileURLToPath(cfg.Settings.RestoreURL)
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("restore script does not exist: %s", path)
		}
		return path, nil
	}
	if !strings.HasPrefix(cfg.Settings.RestoreURL, "http://") && !strings.HasPrefix(cfg.Settings.RestoreURL, "https://") {
		if _, err := os.Stat(cfg.Settings.RestoreURL); err != nil {
			return "", fmt.Errorf("restore script does not exist: %s", cfg.Settings.RestoreURL)
		}
		return cfg.Settings.RestoreURL, nil
	}
	cache := filepath.Join(cacheDir(), "md-restore.sh")
	if err := os.MkdirAll(filepath.Dir(cache), 0755); err != nil {
		return "", err
	}
	tmp := cache + ".tmp"
	cmd := exec.Command("curl", "-fsSL", cfg.Settings.RestoreURL, "-o", tmp)
	if err := cmd.Run(); err != nil {
		if fileExists(cache) {
			return cache, nil
		}
		return "", fmt.Errorf("could not fetch restore helper: %w", err)
	}
	if err := os.Chmod(tmp, 0755); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, cache); err != nil {
		return "", err
	}
	return cache, nil
}

func cacheDir() string {
	if v := os.Getenv("XDG_CACHE_HOME"); v != "" {
		return filepath.Join(expandPath(v), "Rrunner")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Caches", "Rrunner")
}
func actionEnv(s ActionSpec, r Request) map[string]string {
	return map[string]string{"RRUNNER_ACTION": r.Action, "RRUNNER_URL": r.Raw, "RRUNNER_PATH": r.Path, "RRUNNER_APP": r.App, "RRUNNER_PLUGIN_ID": s.PluginID, "RRUNNER_PLUGIN_DIR": s.PluginDir}
}
func envMapToList(m map[string]string) []string {
	out := []string{}
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out
}
func expandPath(p string) string {
	if strings.HasPrefix(p, "~") {
		h, _ := os.UserHomeDir()
		p = h + strings.TrimPrefix(p, "~")
	}
	return os.ExpandEnv(p)
}
func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
func userError(msg string)       { fmt.Fprintln(os.Stderr, "Rrunner:", msg) }
func logEvent(cfg Config, level, event string, fields map[string]any) {
	if cfg.Logging.File == "" {
		return
	}
	p := expandPath(cfg.Logging.File)
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	fields["ts"] = time.Now().Format(time.RFC3339)
	fields["level"] = level
	fields["event"] = event
	b, _ := json.Marshal(fields)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		_, _ = f.Write(append(b, '\n'))
	}
}

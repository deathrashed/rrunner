package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseRequestFileURL(t *testing.T) {
	req, err := parseRequest("rrunner://edit?url=file:///tmp/example.md&app=com.example.App")
	if err != nil {
		t.Fatal(err)
	}
	if req.Action != "edit" {
		t.Fatalf("action = %q", req.Action)
	}
	if req.Path != "/tmp/example.md" {
		t.Fatalf("path = %q", req.Path)
	}
	if req.App != "com.example.App" {
		t.Fatalf("app = %q", req.App)
	}
}

func TestMiniTOMLParsesQuoteFriendlyCommands(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`[settings]
terminal_app = "Ghostty"

[plugins]
dirs = [
  "~/.config/rrunner/plugins",
  "~/.local/share/rrunner/plugins",
]

[actions.kb]
type = "command"
command = 'open "/Users/rd/.config/typinator/Sets/Includes/Text/KB"'
`), 0644); err != nil {
		t.Fatal(err)
	}
	mt, err := parseMiniTOMLFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(mt.Errors) != 0 {
		t.Fatalf("unexpected parse errors: %#v", mt.Errors)
	}
	cmd, ok := strVal(mt.Sections["actions.kb"], "command")
	if !ok || cmd != `open "/Users/rd/.config/typinator/Sets/Includes/Text/KB"` {
		t.Fatalf("command = %q", cmd)
	}
}

func TestBuildRrunnerURLEncodesPathAndApp(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "hello world #1.md")
	got, err := buildRrunnerURL("edit", file, "com.example.Editor")
	if err != nil {
		t.Fatal(err)
	}
	req, err := parseRequest(got)
	if err != nil {
		t.Fatal(err)
	}
	if req.Action != "edit" || req.Path != file || req.App != "com.example.Editor" {
		t.Fatalf("built URL parsed as %#v from %q", req, got)
	}
}

func TestActionRiskAndMarkdownCatalog(t *testing.T) {
	items := []ActionSpec{
		{Name: "open", Type: "open", Source: "rrunner.core"},
		{Name: "build", Type: "command", Source: "local.config", Description: "Build | test"},
	}
	md := renderActionCatalogMarkdown(items, true)
	if !strings.Contains(md, "`executable`") || !strings.Contains(md, "Build \\| test") {
		t.Fatalf("catalog missing risk/escaped description:\n%s", md)
	}
}

func TestConfirmParsesFromTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`[actions.danger]
type = "command"
command = 'echo ok'
confirm = true
`), 0644); err != nil {
		t.Fatal(err)
	}
	mt, err := parseMiniTOMLFile(path)
	if err != nil {
		t.Fatal(err)
	}
	spec := actionFromSection("danger", mt.Sections["actions.danger"], "local.config", dir)
	if !spec.Confirm {
		t.Fatal("confirm was not parsed")
	}
}

func TestPluginCommandBlockedAddsEvent(t *testing.T) {
	dir := t.TempDir()
	plugins := filepath.Join(dir, "plugins")
	if err := os.MkdirAll(filepath.Join(plugins, "demo"), 0755); err != nil {
		t.Fatal(err)
	}
	manifest := `[plugin]
id = "demo.plugin"
name = "Demo"

[actions.demo-run]
type = "command"
command = 'echo nope'
`
	if err := os.WriteFile(filepath.Join(plugins, "demo", "plugin.toml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		Settings: Settings{HandlersDir: filepath.Join(dir, "handlers")},
		Plugins:  PluginSettings{Enabled: true, Dirs: []string{plugins}, Disabled: map[string]bool{}, Trusted: map[string]bool{}},
		Security: SecuritySettings{AllowInlineCommands: true, AllowPluginCommands: false, RequireTrustedPluginsForCommands: true},
		Actions:  map[string]ActionSpec{},
	}
	reg := buildRegistry(cfg)
	if _, ok := reg.Actions["demo-run"]; ok {
		t.Fatal("blocked plugin command was registered")
	}
	found := false
	for _, ev := range reg.PluginEvents {
		if ev.Status == "blocked" && strings.Contains(ev.Reason, "demo.plugin.demo-run") {
			found = true
		}
	}
	if !found {
		t.Fatalf("blocked plugin event missing: %#v", reg.PluginEvents)
	}
}

func TestDryRunPlanOpenWith(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "README.md")
	if err := os.WriteFile(file, []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		Settings: Settings{HandlersDir: filepath.Join(dir, "handlers"), TerminalApp: "Terminal", KeepOpen: false},
		Plugins:  PluginSettings{Enabled: false, Disabled: map[string]bool{}, Trusted: map[string]bool{}},
		Security: SecuritySettings{AllowInlineCommands: true, AllowLegacyHandlers: true},
		Actions:  map[string]ActionSpec{"edit": {Name: "edit", Type: "openwith", App: "com.coteditor.CotEditor", Source: "local.config", PluginID: "local.config"}},
	}
	reg := buildRegistry(cfg)
	req, err := parseRequest("rrunner://edit?url=file://" + file)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := planAction(cfg, reg, req, true)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Action != "edit" || len(plan.Steps) != 1 || plan.Steps[0].Kind != "openwith" {
		t.Fatalf("bad plan: %#v", plan)
	}
}

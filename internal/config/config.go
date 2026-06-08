package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Settings represents the core application settings
type Settings struct {
	TerminalApp       string `toml:"terminal_app" json:"terminal_app,omitempty"`
	KeepOpen          bool   `toml:"keep_open" json:"keep_open,omitempty"`
	RemoteBase        string `toml:"remote_base" json:"remote_base,omitempty"`
	RemoteURL         string `toml:"remote_url" json:"remote_url,omitempty"`
	RestoreURL        string `toml:"restore_url" json:"restore_url,omitempty"`
	HandlersDir       string `toml:"handlers_dir" json:"handlers_dir,omitempty"`
	TextEditor        string `toml:"text_editor" json:"text_editor,omitempty"`
	CodeEditor        string `toml:"code_editor" json:"code_editor,omitempty"`
	MarkdownPreviewer string `toml:"markdown_previewer" json:"markdown_previewer,omitempty"`
}

// PluginSettings controls plugin behavior and discovery
type PluginSettings struct {
	Enabled  bool              `toml:"enabled" json:"enabled"`
	Dirs     []string          `toml:"dirs" json:"dirs,omitempty"`
	Disabled map[string]bool   `toml:"disabled" json:"disabled,omitempty"`
	Trusted  map[string]bool   `toml:"trusted" json:"trusted,omitempty"`
	AutoLoad bool              `toml:"auto_load" json:"auto_load,omitempty"`
}

// SecuritySettings controls security-related behavior
type SecuritySettings struct {
	AllowInlineCommands              bool `toml:"allow_inline_commands" json:"allow_inline_commands"`
	AllowPluginCommands              bool `toml:"allow_plugin_commands" json:"allow_plugin_commands"`
	RequireTrustedPluginsForCommands bool `toml:"require_trusted_plugins_for_commands" json:"require_trusted_plugins_for_commands"`
	AllowLegacyHandlers              bool `toml:"allow_legacy_handlers" json:"allow_legacy_handlers"`
	AllowRemotePlugins               bool `toml:"allow_remote_plugins" json:"allow_remote_plugins"`
	SandboxPlugins                   bool `toml:"sandbox_plugins" json:"sandbox_plugins"`
}

// LoggingSettings controls logging behavior
type LoggingSettings struct {
	Level      string `toml:"level" json:"level,omitempty"`
	File       string `toml:"file" json:"file,omitempty"`
	MaxSize    int    `toml:"max_size" json:"max_size,omitempty"`
	MaxBackups int    `toml:"max_backups" json:"max_backups,omitempty"`
	MaxAge     int    `toml:"max_age" json:"max_age,omitempty"`
	Compress   bool   `toml:"compress" json:"compress,omitempty"`
}

// MetricsSettings controls metrics collection and reporting
type MetricsSettings struct {
	Enabled      bool   `toml:"enabled" json:"enabled"`
	Port         int    `toml:"port" json:"port,omitempty"`
	Path         string `toml:"path" json:"path,omitempty"`
	CollectUsage bool   `toml:"collect_usage" json:"collect_usage,omitempty"`
}

// PerformanceSettings controls performance-related behavior
type PerformanceSettings struct {
	MaxConcurrency    int  `toml:"max_concurrency" json:"max_concurrency,omitempty"`
	EnableCaching     bool `toml:"enable_caching" json:"enable_caching,omitempty"`
	CacheSize         int  `toml:"cache_size" json:"cache_size,omitempty"`
	RequestTimeout    int  `toml:"request_timeout" json:"request_timeout,omitempty"`
	PluginTimeout     int  `toml:"plugin_timeout" json:"plugin_timeout,omitempty"`
}

// Config represents the complete configuration structure
type Config struct {
	Path        string                    `json:"path,omitempty"`
	Settings    Settings                  `json:"settings"`
	Plugins     PluginSettings            `json:"plugins"`
	Security    SecuritySettings          `json:"security"`
	Logging     LoggingSettings           `json:"logging"`
	Metrics     MetricsSettings           `json:"metrics"`
	Performance PerformanceSettings       `json:"performance"`
	Actions     map[string]ActionSpec     `json:"actions,omitempty"`
	Errors      []string                  `json:"errors,omitempty"`
	Templates   map[string]TemplateSpec   `json:"templates,omitempty"`
}

// ActionSpec defines a custom action
type ActionSpec struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Source      string   `json:"source,omitempty"`
	Description string   `json:"description,omitempty"`
	Aliases     []string `json:"aliases,omitempty"`
	App         string   `json:"app,omitempty"`
	Runner      string   `json:"runner,omitempty"`
	Script      string   `json:"script,omitempty"`
	Command     string   `json:"command,omitempty"`
	Template    string   `json:"template,omitempty"`
	PathPolicy  string   `json:"path_policy,omitempty"`
	Confirm     bool     `json:"confirm,omitempty"`
	Async       bool     `json:"async,omitempty"`
	Timeout     int      `json:"timeout,omitempty"`
	PluginID    string   `json:"plugin_id,omitempty"`
	PluginDir   string   `json:"plugin_dir,omitempty"`
	Shadowed    bool     `json:"shadowed,omitempty"`
}

// TemplateSpec defines a reusable template
type TemplateSpec struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Content     string            `json:"content"`
	Variables   map[string]string `json:"variables,omitempty"`
	Type        string            `json:"type,omitempty"` // "shell", "applescript", "json", etc.
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() Config {
	homeDir, _ := os.UserHomeDir()
	
	return Config{
		Settings: Settings{
			TerminalApp:       "Terminal",
			KeepOpen:          false,
			HandlersDir:       filepath.Join(homeDir, ".config", "rrunner", "handlers"),
			TextEditor:        "TextEdit",
			CodeEditor:        "Visual Studio Code",
			MarkdownPreviewer: "Marked 2",
		},
		Plugins: PluginSettings{
			Enabled:  true,
			Dirs:     []string{filepath.Join(homeDir, ".config", "rrunner", "plugins")},
			Disabled: make(map[string]bool),
			Trusted:  make(map[string]bool),
			AutoLoad: true,
		},
		Security: SecuritySettings{
			AllowInlineCommands:              true,
			AllowPluginCommands:              false,
			RequireTrustedPluginsForCommands: true,
			AllowLegacyHandlers:              true,
			AllowRemotePlugins:               false,
			SandboxPlugins:                   false,
		},
		Logging: LoggingSettings{
			Level:      "info",
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     28,
			Compress:   true,
		},
		Metrics: MetricsSettings{
			Enabled:      false,
			Port:         8080,
			Path:         "/metrics",
			CollectUsage: true,
		},
		Performance: PerformanceSettings{
			MaxConcurrency: 10,
			EnableCaching:  true,
			CacheSize:      100,
			RequestTimeout: 30,
			PluginTimeout:  60,
		},
		Actions:   make(map[string]ActionSpec),
		Templates: make(map[string]TemplateSpec),
		Errors:    []string{},
	}
}

// Validate checks the configuration for errors and inconsistencies
func (c *Config) Validate() []string {
	var errors []string
	
	// Validate settings
	if c.Settings.TerminalApp == "" {
		errors = append(errors, "terminal_app cannot be empty")
	}
	
	// Validate plugin directories
	for _, dir := range c.Plugins.Dirs {
		expanded := expandPath(dir)
		if _, err := os.Stat(expanded); os.IsNotExist(err) {
			errors = append(errors, fmt.Sprintf("plugin directory does not exist: %s", dir))
		}
	}
	
	// Validate performance settings
	if c.Performance.MaxConcurrency < 1 {
		errors = append(errors, "max_concurrency must be at least 1")
	}
	if c.Performance.RequestTimeout < 1 {
		errors = append(errors, "request_timeout must be at least 1 second")
	}
	
	// Validate logging settings
	validLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLevels, c.Logging.Level) {
		errors = append(errors, fmt.Sprintf("invalid log level: %s (must be one of: %s)", 
			c.Logging.Level, strings.Join(validLevels, ", ")))
	}
	
	// Validate actions
	for name, action := range c.Actions {
		if action.Type == "" {
			errors = append(errors, fmt.Sprintf("action '%s' missing type", name))
		}
		if action.Type == "command" && action.Command == "" {
			errors = append(errors, fmt.Sprintf("command action '%s' missing command", name))
		}
		if action.Type == "script" && action.Script == "" {
			errors = append(errors, fmt.Sprintf("script action '%s' missing script", name))
		}
	}
	
	return errors
}

// ToJSON converts the config to JSON format
func (c *Config) ToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

// Helper functions
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		if homeDir, err := os.UserHomeDir(); err == nil {
			return filepath.Join(homeDir, path[2:])
		}
	}
	return path
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
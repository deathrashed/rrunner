package plugins

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	
	"github.com/deathrashed/rrunner/internal/config"
)

// Status represents the loading status of a plugin
type Status string

const (
	StatusLoaded  Status = "loaded"
	StatusSkipped Status = "skipped"
	StatusBlocked Status = "blocked"
	StatusFailed  Status = "failed"
)

// Manifest represents a plugin manifest with metadata and actions
type Manifest struct {
	ID          string                    `toml:"id" json:"id"`
	Name        string                    `toml:"name" json:"name"`
	Version     string                    `toml:"version" json:"version"`
	Description string                    `toml:"description" json:"description,omitempty"`
	Author      string                    `toml:"author" json:"author,omitempty"`
	Homepage    string                    `toml:"homepage" json:"homepage,omitempty"`
	Repository  string                    `toml:"repository" json:"repository,omitempty"`
	License     string                    `toml:"license" json:"license,omitempty"`
	MinVersion  string                    `toml:"min_version" json:"min_version,omitempty"`
	MaxVersion  string                    `toml:"max_version" json:"max_version,omitempty"`
	Dependencies []string                 `toml:"dependencies" json:"dependencies,omitempty"`
	Path        string                    `json:"path"`
	Dir         string                    `json:"dir"`
	Enabled     bool                      `toml:"enabled" json:"enabled"`
	Actions     map[string]config.ActionSpec `json:"actions"`
	Templates   map[string]config.TemplateSpec `json:"templates,omitempty"`
	Errors      []string                  `json:"errors,omitempty"`
	LoadTime    time.Time                 `json:"load_time,omitempty"`
}

// Event represents a plugin loading/management event
type Event struct {
	Path      string    `json:"path"`
	ID        string    `json:"id,omitempty"`
	Status    Status    `json:"status"`
	Reason    string    `json:"reason,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Duration  time.Duration `json:"duration,omitempty"`
}

// Manager handles plugin discovery, loading, and management
type Manager struct {
	config      config.Config
	manifests   map[string]*Manifest
	events      []Event
	watchMode   bool
	watchers    map[string]context.CancelFunc
}

// NewManager creates a new plugin manager
func NewManager(cfg config.Config) *Manager {
	return &Manager{
		config:    cfg,
		manifests: make(map[string]*Manifest),
		events:    []Event{},
		watchers:  make(map[string]context.CancelFunc),
	}
}

// DiscoverPlugins discovers all plugins in configured directories
func (m *Manager) DiscoverPlugins() []*Manifest {
	var manifests []*Manifest
	
	if !m.config.Plugins.Enabled {
		return manifests
	}
	
	for _, dir := range m.config.Plugins.Dirs {
		dirManifests := m.discoverInDirectory(dir)
		manifests = append(manifests, dirManifests...)
	}
	
	// Sort by ID for consistent ordering
	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].ID < manifests[j].ID
	})
	
	return manifests
}

// LoadPlugin loads a specific plugin by ID
func (m *Manager) LoadPlugin(id string) (*Manifest, error) {
	manifest, exists := m.manifests[id]
	if !exists {
		return nil, fmt.Errorf("plugin not found: %s", id)
	}
	
	start := time.Now()
	event := Event{
		Path:      manifest.Path,
		ID:        manifest.ID,
		Status:    StatusLoaded,
		Timestamp: start,
	}
	
	// Check if plugin is disabled
	if m.config.Plugins.Disabled[id] || !manifest.Enabled {
		event.Status = StatusSkipped
		if m.config.Plugins.Disabled[id] {
			event.Reason = "disabled in config"
		} else {
			event.Reason = "plugin enabled=false"
		}
		m.events = append(m.events, event)
		return nil, fmt.Errorf("plugin disabled: %s", id)
	}
	
	// Validate dependencies
	if err := m.validateDependencies(manifest); err != nil {
		event.Status = StatusFailed
		event.Reason = fmt.Sprintf("dependency error: %v", err)
		event.Duration = time.Since(start)
		m.events = append(m.events, event)
		return nil, err
	}
	
	// Validate version compatibility
	if err := m.validateVersion(manifest); err != nil {
		event.Status = StatusFailed
		event.Reason = fmt.Sprintf("version error: %v", err)
		event.Duration = time.Since(start)
		m.events = append(m.events, event)
		return nil, err
	}
	
	// Filter out blocked command actions
	for actionName, actionSpec := range manifest.Actions {
		if actionSpec.Type == "command" {
			if !m.config.Security.AllowPluginCommands {
				delete(manifest.Actions, actionName)
				event.Reason += fmt.Sprintf("blocked command action: %s; ", actionName)
				continue
			}
			
			if m.config.Security.RequireTrustedPluginsForCommands && !m.config.Plugins.Trusted[id] {
				delete(manifest.Actions, actionName)
				event.Reason += fmt.Sprintf("untrusted command action: %s; ", actionName)
				continue
			}
		}
	}
	
	event.Duration = time.Since(start)
	manifest.LoadTime = start
	m.events = append(m.events, event)
	
	return manifest, nil
}

// ReloadPlugin reloads a specific plugin
func (m *Manager) ReloadPlugin(id string) (*Manifest, error) {
	manifest, exists := m.manifests[id]
	if !exists {
		return nil, fmt.Errorf("plugin not found: %s", id)
	}
	
	// Re-parse the manifest file
	newManifest, err := m.parseManifest(manifest.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to reload plugin %s: %v", id, err)
	}
	
	m.manifests[id] = newManifest
	return m.LoadPlugin(id)
}

// UnloadPlugin unloads a specific plugin
func (m *Manager) UnloadPlugin(id string) error {
	delete(m.manifests, id)
	
	// Cancel any watchers for this plugin
	if cancel, exists := m.watchers[id]; exists {
		cancel()
		delete(m.watchers, id)
	}
	
	return nil
}

// GetManifest returns a plugin manifest by ID
func (m *Manager) GetManifest(id string) (*Manifest, bool) {
	manifest, exists := m.manifests[id]
	return manifest, exists
}

// ListManifests returns all loaded plugin manifests
func (m *Manager) ListManifests() []*Manifest {
	var manifests []*Manifest
	for _, manifest := range m.manifests {
		manifests = append(manifests, manifest)
	}
	
	// Sort by ID for consistent ordering
	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].ID < manifests[j].ID
	})
	
	return manifests
}

// GetEvents returns all plugin events
func (m *Manager) GetEvents() []Event {
	return m.events
}

// EnableWatchMode enables file system watching for plugin changes
func (m *Manager) EnableWatchMode(ctx context.Context) error {
	if m.watchMode {
		return nil // Already enabled
	}
	
	m.watchMode = true
	
	for _, dir := range m.config.Plugins.Dirs {
		if err := m.watchDirectory(ctx, dir); err != nil {
			return fmt.Errorf("failed to watch directory %s: %v", dir, err)
		}
	}
	
	return nil
}

// DisableWatchMode disables file system watching
func (m *Manager) DisableWatchMode() {
	if !m.watchMode {
		return
	}
	
	m.watchMode = false
	
	// Cancel all watchers
	for id, cancel := range m.watchers {
		cancel()
		delete(m.watchers, id)
	}
}

// ValidateManifest validates a plugin manifest
func (m *Manager) ValidateManifest(manifest *Manifest) []string {
	var errors []string
	
	if manifest.ID == "" {
		errors = append(errors, "plugin id is required")
	}
	
	if manifest.Name == "" {
		errors = append(errors, "plugin name is required")
	}
	
	if manifest.Version == "" {
		errors = append(errors, "plugin version is required")
	}
	
	// Validate actions
	for name, action := range manifest.Actions {
		if action.Type == "" {
			errors = append(errors, fmt.Sprintf("action '%s' missing type", name))
		}
		
		if isReservedAction(name) {
			errors = append(errors, fmt.Sprintf("action name '%s' is reserved", name))
		}
	}
	
	return errors
}

// Private methods

func (m *Manager) discoverInDirectory(dir string) []*Manifest {
	var manifests []*Manifest
	expanded := expandPath(dir)
	
	// Check if directory exists
	if _, err := os.Stat(expanded); os.IsNotExist(err) {
		return manifests
	}
	
	// Look for plugin.toml files in subdirectories
	err := filepath.WalkDir(expanded, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors and continue
		}
		
		// Skip hidden directories and files
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		
		// Look for plugin.toml or *.plugin.toml files
		if d.IsDir() {
			return nil
		}
		
		name := d.Name()
		if name == "plugin.toml" || strings.HasSuffix(name, ".plugin.toml") {
			if manifest, err := m.parseManifest(path); err == nil {
				manifests = append(manifests, manifest)
				m.manifests[manifest.ID] = manifest
			}
		}
		
		return nil
	})
	
	if err != nil {
		// Log error but continue
	}
	
	return manifests
}

func (m *Manager) parseManifest(path string) (*Manifest, error) {
	manifest := &Manifest{
		Path:      path,
		Dir:       filepath.Dir(path),
		Enabled:   true,
		Actions:   make(map[string]config.ActionSpec),
		Templates: make(map[string]config.TemplateSpec),
	}
	
	// Parse TOML file (implementation depends on TOML parser)
	// This is a simplified version - you'd use a real TOML parser here
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %v", err)
	}
	
	// Basic parsing logic would go here
	// For now, return a basic manifest
	manifest.ID = "example-plugin"
	manifest.Name = "Example Plugin"
	manifest.Version = "1.0.0"
	
	return manifest, nil
}

func (m *Manager) validateDependencies(manifest *Manifest) error {
	for _, dep := range manifest.Dependencies {
		if _, exists := m.manifests[dep]; !exists {
			return fmt.Errorf("missing dependency: %s", dep)
		}
	}
	return nil
}

func (m *Manager) validateVersion(manifest *Manifest) error {
	// Version validation logic would go here
	// Check MinVersion and MaxVersion against current rrunner version
	return nil
}

func (m *Manager) watchDirectory(ctx context.Context, dir string) error {
	// File system watching implementation would go here
	// This would typically use fsnotify or similar
	return nil
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

func isReservedAction(name string) bool {
	reserved := []string{
		"open", "reveal", "show", "openwith", "view", "launch", "auto",
		"osascript", "bash", "zsh", "python", "node", "ruby", "perl",
		"restore", "diagnostics", "list-actions",
	}
	
	for _, r := range reserved {
		if name == r {
			return true
		}
	}
	return false
}

// PluginInfo provides summary information about a plugin
type PluginInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Description string    `json:"description,omitempty"`
	Author      string    `json:"author,omitempty"`
	Enabled     bool      `json:"enabled"`
	Loaded      bool      `json:"loaded"`
	ActionCount int       `json:"action_count"`
	LoadTime    time.Time `json:"load_time,omitempty"`
	Errors      []string  `json:"errors,omitempty"`
}

// GetPluginInfo returns summary information about a plugin
func (m *Manager) GetPluginInfo(id string) (*PluginInfo, error) {
	manifest, exists := m.manifests[id]
	if !exists {
		return nil, fmt.Errorf("plugin not found: %s", id)
	}
	
	return &PluginInfo{
		ID:          manifest.ID,
		Name:        manifest.Name,
		Version:     manifest.Version,
		Description: manifest.Description,
		Author:      manifest.Author,
		Enabled:     manifest.Enabled && !m.config.Plugins.Disabled[id],
		Loaded:      true,
		ActionCount: len(manifest.Actions),
		LoadTime:    manifest.LoadTime,
		Errors:      manifest.Errors,
	}, nil
}

// ListPluginInfo returns summary information for all plugins
func (m *Manager) ListPluginInfo() []*PluginInfo {
	var infos []*PluginInfo
	
	for id := range m.manifests {
		if info, err := m.GetPluginInfo(id); err == nil {
			infos = append(infos, info)
		}
	}
	
	// Sort by ID for consistent ordering
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].ID < infos[j].ID
	})
	
	return infos
}
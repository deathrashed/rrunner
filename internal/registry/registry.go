package registry

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
	
	"github.com/deathrashed/rrunner/internal/actions"
	"github.com/deathrashed/rrunner/internal/config"
	"github.com/deathrashed/rrunner/internal/plugins"
	"github.com/deathrashed/rrunner/internal/utils"
)

// Registry manages actions, plugins, and request processing
type Registry struct {
	config      config.Config
	pluginMgr   *plugins.Manager
	actionReg   *actions.Registry
	actionSpecs map[string]config.ActionSpec
	cache       *utils.Cache
}

// NewRegistry creates a new registry instance
func NewRegistry(cfg config.Config, pluginMgr *plugins.Manager, actionReg *actions.Registry) *Registry {
	r := &Registry{
		config:      cfg,
		pluginMgr:   pluginMgr,
		actionReg:   actionReg,
		actionSpecs: make(map[string]config.ActionSpec),
		cache:       utils.NewCache(),
	}
	
	// Start cache cleanup
	r.cache.StartCleanupJob(5 * time.Minute)
	
	// Build initial registry
	r.rebuild()
	
	return r
}

// Rebuild rebuilds the action registry from all sources
func (r *Registry) rebuild() {
	// Clear existing actions
	r.actionSpecs = make(map[string]config.ActionSpec)
	
	// Add built-in actions
	builtinActions := r.getBuiltinActions()
	for _, action := range builtinActions {
		r.actionSpecs[action.Name] = action
		
		// Add aliases
		for _, alias := range action.Aliases {
			if _, exists := r.actionSpecs[alias]; !exists {
				aliasAction := action
				aliasAction.Name = alias
				r.actionSpecs[alias] = aliasAction
			}
		}
	}
	
	// Add config actions
	for name, action := range r.config.Actions {
		action.Name = name
		action.Source = "local.config"
		r.actionSpecs[name] = action
		
		// Add aliases
		for _, alias := range action.Aliases {
			if _, exists := r.actionSpecs[alias]; !exists {
				aliasAction := action
				aliasAction.Name = alias
				r.actionSpecs[alias] = aliasAction
			}
		}
	}
	
	// Add plugin actions
	if r.config.Plugins.Enabled {
		manifests := r.pluginMgr.DiscoverPlugins()
		for _, manifest := range manifests {
			loadedManifest, err := r.pluginMgr.LoadPlugin(manifest.ID)
			if err != nil {
				continue // Skip failed plugins
			}
			
			for name, action := range loadedManifest.Actions {
				action.Name = name
				action.Source = "plugin." + manifest.ID
				action.PluginID = manifest.ID
				action.PluginDir = manifest.Dir
				
				// Check for shadowing
				if _, exists := r.actionSpecs[name]; exists {
					action.Shadowed = true
				} else {
					r.actionSpecs[name] = action
				}
				
				// Add aliases
				for _, alias := range action.Aliases {
					if _, exists := r.actionSpecs[alias]; !exists {
						aliasAction := action
						aliasAction.Name = alias
						r.actionSpecs[alias] = aliasAction
					}
				}
			}
		}
	}
	
	// Clear cache after rebuild
	r.cache.Clear()
}

// GetAction retrieves an action by name
func (r *Registry) GetAction(name string) (config.ActionSpec, bool) {
	action, exists := r.actionSpecs[name]
	return action, exists
}

// ListActions returns all available actions
func (r *Registry) ListActions() []config.ActionSpec {
	actions := make([]config.ActionSpec, 0, len(r.actionSpecs))
	seen := make(map[string]bool)
	
	for _, action := range r.actionSpecs {
		if !seen[action.Name+action.Source] {
			actions = append(actions, action)
			seen[action.Name+action.Source] = true
		}
	}
	
	// Sort by name
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].Name < actions[j].Name
	})
	
	return actions
}

// ParseRequest parses a URL request into a structured request
func (r *Registry) ParseRequest(rawURL string) (actions.Request, error) {
	// Check cache first
	if cached, found := r.cache.Get("parse:" + rawURL); found {
		if req, ok := cached.(actions.Request); ok {
			return req, nil
		}
	}
	
	u, err := url.Parse(rawURL)
	if err != nil {
		return actions.Request{}, fmt.Errorf("invalid URL: %v", err)
	}
	
	if u.Scheme != "rrunner" {
		return actions.Request{}, fmt.Errorf("unsupported URL scheme: %s (expected: rrunner)", u.Scheme)
	}
	
	// Extract action from host or path
	action := u.Host
	if action == "" {
		pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(pathParts) > 0 && pathParts[0] != "" {
			action = pathParts[0]
		}
	}
	
	if action == "" {
		return actions.Request{}, fmt.Errorf("no action specified in URL")
	}
	
	// Parse query parameters
	query := u.Query()
	
	// Extract path from URL or path parameter
	path := ""
	if urlParam := query.Get("url"); urlParam != "" {
		path = utils.FileURLToPath(urlParam)
	} else if pathParam := query.Get("path"); pathParam != "" {
		path, _ = url.QueryUnescape(pathParam)
	}
	
	// Create request
	req := actions.Request{
		Raw:    rawURL,
		Action: action,
		Path:   path,
		App:    query.Get("app"),
		Query:  query,
		Context: actions.Context{
			WorkingDir:  getCurrentDir(),
			Environment: getCurrentEnv(),
			User:        getCurrentUser(),
			Timestamp:   time.Now(),
			RequestID:   utils.GenerateRequestID(),
		},
	}
	
	// Cache the result
	r.cache.Set("parse:"+rawURL, req, 5*time.Minute)
	
	return req, nil
}

// PlanExecution creates an execution plan for a request
func (r *Registry) PlanExecution(req actions.Request, dryRun bool) (actions.ExecutionPlan, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("plan:%s:%t", req.Raw, dryRun)
	if cached, found := r.cache.Get(cacheKey); found {
		if plan, ok := cached.(actions.ExecutionPlan); ok {
			return plan, nil
		}
	}
	
	// Get action specification
	actionSpec, exists := r.actionSpecs[req.Action]
	if !exists {
		return actions.ExecutionPlan{}, fmt.Errorf("unknown action: %s", req.Action)
	}
	
	// Create base execution plan
	plan := actions.ExecutionPlan{
		Action:      req.Action,
		Type:        actions.ActionType(actionSpec.Type),
		Path:        req.Path,
		App:         req.App,
		Environment: make(map[string]string),
		DryRun:      dryRun,
		Confirm:     actionSpec.Confirm,
		Async:       actionSpec.Async,
	}
	
	// Set timeout
	if actionSpec.Timeout > 0 {
		plan.Timeout = time.Duration(actionSpec.Timeout) * time.Second
	} else {
		plan.Timeout = time.Duration(r.config.Performance.RequestTimeout) * time.Second
	}
	
	// Handle different action types
	switch actionSpec.Type {
	case "open", "reveal", "show":
		if req.Path == "" {
			return actions.ExecutionPlan{}, fmt.Errorf("path parameter required for %s action", actionSpec.Type)
		}
		plan.Path = req.Path
		
	case "launch":
		if actionSpec.App != "" {
			plan.App = actionSpec.App
		} else if req.App != "" {
			plan.App = req.App
		} else {
			return actions.ExecutionPlan{}, fmt.Errorf("app parameter required for launch action")
		}
		
	case "openwith":
		if req.Path == "" {
			return actions.ExecutionPlan{}, fmt.Errorf("path parameter required for openwith action")
		}
		if actionSpec.App != "" {
			plan.App = actionSpec.App
		} else if req.App != "" {
			plan.App = req.App
		} else {
			return actions.ExecutionPlan{}, fmt.Errorf("app parameter required for openwith action")
		}
		plan.Path = req.Path
		
	case "run":
		if actionSpec.Runner != "" {
			plan.Command = []string{actionSpec.Runner, req.Path}
		} else {
			return actions.ExecutionPlan{}, fmt.Errorf("runner not specified for run action")
		}
		
	case "script":
		if actionSpec.Script == "" {
			return actions.ExecutionPlan{}, fmt.Errorf("script not specified for script action")
		}
		plan.Script = actionSpec.Script
		
	case "command":
		if actionSpec.Command == "" {
			return actions.ExecutionPlan{}, fmt.Errorf("command not specified for command action")
		}
		plan.Script = actionSpec.Command
		
	case "template":
		if actionSpec.Template != "" {
			// TODO: Process template
			plan.Script = actionSpec.Template
		} else {
			return actions.ExecutionPlan{}, fmt.Errorf("template not specified for template action")
		}
		
	case "auto":
		// Auto-detect based on file extension
		autoType, autoRunner := r.detectFileType(req.Path)
		plan.Type = autoType
		if autoRunner != "" {
			plan.Command = []string{autoRunner, req.Path}
		}
		
	default:
		return actions.ExecutionPlan{}, fmt.Errorf("unsupported action type: %s", actionSpec.Type)
	}
	
	// Set environment variables
	plan.Environment["RRUNNER_ACTION"] = req.Action
	plan.Environment["RRUNNER_PATH"] = req.Path
	plan.Environment["RRUNNER_APP"] = req.App
	plan.Environment["RRUNNER_REQUEST_ID"] = req.Context.RequestID
	plan.Environment["RRUNNER_USER"] = req.Context.User
	plan.Environment["RRUNNER_VERSION"] = "0.3.0-enhanced"
	
	// Add query parameters as environment variables
	for key, values := range req.Query {
		if len(values) > 0 {
			envKey := "RRUNNER_PARAM_" + strings.ToUpper(key)
			plan.Environment[envKey] = values[0]
		}
	}
	
	// Set working directory
	if req.Path != "" && plan.Type != actions.ActionTypeOpen && plan.Type != actions.ActionTypeReveal {
		if dir := filepath.Dir(req.Path); dir != "." {
			plan.WorkingDir = dir
		}
	}
	
	// Cache the result
	r.cache.Set(cacheKey, plan, 5*time.Minute)
	
	return plan, nil
}

// Execute executes an execution plan
func (r *Registry) Execute(ctx context.Context, plan actions.ExecutionPlan) actions.Result {
	return r.actionReg.Execute(ctx, plan)
}

// ValidateAction validates an action configuration
func (r *Registry) ValidateAction(actionName string) []string {
	action, exists := r.actionSpecs[actionName]
	if !exists {
		return []string{fmt.Sprintf("action not found: %s", actionName)}
	}
	
	var errors []string
	
	// Validate based on type
	switch action.Type {
	case "command":
		if action.Command == "" {
			errors = append(errors, "command action requires command field")
		}
		
		// Check security settings
		if action.Source == "plugin."+action.PluginID {
			if !r.config.Security.AllowPluginCommands {
				errors = append(errors, "plugin command actions are disabled")
			}
			if r.config.Security.RequireTrustedPluginsForCommands && !r.config.Plugins.Trusted[action.PluginID] {
				errors = append(errors, "plugin is not trusted for command actions")
			}
		}
		
	case "script":
		if action.Script == "" {
			errors = append(errors, "script action requires script field")
		}
		
	case "run":
		if action.Runner == "" {
			errors = append(errors, "run action requires runner field")
		}
		
	case "openwith", "launch":
		if action.App == "" {
			errors = append(errors, fmt.Sprintf("%s action requires app field", action.Type))
		}
	}
	
	return errors
}

// Helper functions
func (r *Registry) getBuiltinActions() []config.ActionSpec {
	builtinSpecs := []struct {
		name        string
		actionType  string
		description string
		aliases     []string
	}{
		{"open", "open", "Open a file with the default application", []string{"view"}},
		{"reveal", "reveal", "Reveal a file in Finder", []string{"show"}},
		{"launch", "launch", "Launch an application", nil},
		{"openwith", "openwith", "Open a file with a specific application", nil},
		{"osascript", "run", "Run an AppleScript file", nil},
		{"bash", "run", "Run a bash script", nil},
		{"zsh", "run", "Run a zsh script", nil},
		{"python", "run", "Run a Python script", []string{"py"}},
		{"node", "run", "Run a Node.js script", []string{"js", "javascript"}},
		{"ruby", "run", "Run a Ruby script", []string{"rb"}},
		{"perl", "run", "Run a Perl script", []string{"pl"}},
		{"restore", "restore", "Restore embedded file from Markdown wrapper", nil},
		{"auto", "auto", "Auto-detect and run based on file extension", nil},
	}
	
	actions := make([]config.ActionSpec, len(builtinSpecs))
	for i, spec := range builtinSpecs {
		action := config.ActionSpec{
			Name:        spec.name,
			Type:        spec.actionType,
			Source:      "rrunner.core",
			Description: spec.description,
			Aliases:     spec.aliases,
			PluginID:    "rrunner.core",
		}
		
		// Set runner for run actions
		if spec.actionType == "run" {
			action.Runner = spec.name
		}
		
		actions[i] = action
	}
	
	return actions
}

func (r *Registry) detectFileType(path string) (actions.ActionType, string) {
	if path == "" {
		return actions.ActionTypeOpen, ""
	}
	
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".applescript", ".scpt":
		return actions.ActionTypeRun, "osascript"
	case ".sh":
		return actions.ActionTypeRun, "bash"
	case ".zsh":
		return actions.ActionTypeRun, "zsh"
	case ".py":
		return actions.ActionTypeRun, "python"
	case ".js":
		return actions.ActionTypeRun, "node"
	case ".rb":
		return actions.ActionTypeRun, "ruby"
	case ".pl":
		return actions.ActionTypeRun, "perl"
	default:
		return actions.ActionTypeOpen, ""
	}
}

func getCurrentDir() string {
	if dir, err := os.Getwd(); err == nil {
		return dir
	}
	return ""
}

func getCurrentEnv() map[string]string {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	return env
}

func getCurrentUser() string {
	if username, _, err := utils.GetUserInfo(); err == nil {
		return username
	}
	return "unknown"
}

// Package imports fix
import (
	"os"
	"path/filepath"
)
package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

// ActionType represents the different types of actions supported
type ActionType string

const (
	ActionTypeOpen         ActionType = "open"
	ActionTypeReveal       ActionType = "reveal"
	ActionTypeShow         ActionType = "show"
	ActionTypeOpenWith     ActionType = "openwith"
	ActionTypeView         ActionType = "view"
	ActionTypeLaunch       ActionType = "launch"
	ActionTypeAuto         ActionType = "auto"
	ActionTypeRun          ActionType = "run"
	ActionTypeScript       ActionType = "script"
	ActionTypeCommand      ActionType = "command"
	ActionTypeTemplate     ActionType = "template"
	ActionTypeRestore      ActionType = "restore"
	ActionTypeHTTP         ActionType = "http"
	ActionTypeJSON         ActionType = "json"
	ActionTypeWebhook      ActionType = "webhook"
	ActionTypeNotify       ActionType = "notify"
	ActionTypeClipboard    ActionType = "clipboard"
	ActionTypeEnvironment  ActionType = "env"
	ActionTypeFile         ActionType = "file"
	ActionTypeDirectory    ActionType = "directory"
	ActionTypeLegacy       ActionType = "legacy-handler"
	ActionTypeDiagnostics  ActionType = "diagnostics"
	ActionTypeListActions  ActionType = "list-actions"
)

// Request represents an action request with its context
type Request struct {
	Raw     string              `json:"raw"`
	Action  string              `json:"action"`
	Path    string              `json:"path,omitempty"`
	App     string              `json:"app,omitempty"`
	Query   map[string][]string `json:"query,omitempty"`
	Context Context             `json:"context,omitempty"`
}

// Context provides additional context for action execution
type Context struct {
	WorkingDir  string            `json:"working_dir,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	User        string            `json:"user,omitempty"`
	Timestamp   time.Time         `json:"timestamp"`
	RequestID   string            `json:"request_id,omitempty"`
}

// Result represents the result of an action execution
type Result struct {
	Success    bool          `json:"success"`
	Output     string        `json:"output,omitempty"`
	Error      string        `json:"error,omitempty"`
	ExitCode   int           `json:"exit_code,omitempty"`
	Duration   time.Duration `json:"duration"`
	Async      bool          `json:"async,omitempty"`
	JobID      string        `json:"job_id,omitempty"`
}

// ExecutionPlan represents a planned action execution
type ExecutionPlan struct {
	Action      string            `json:"action"`
	Type        ActionType        `json:"type"`
	Command     []string          `json:"command,omitempty"`
	Script      string            `json:"script,omitempty"`
	App         string            `json:"app,omitempty"`
	Path        string            `json:"path,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	WorkingDir  string            `json:"working_dir,omitempty"`
	Confirm     bool              `json:"confirm,omitempty"`
	Async       bool              `json:"async,omitempty"`
	Timeout     time.Duration     `json:"timeout,omitempty"`
	DryRun      bool              `json:"dry_run,omitempty"`
}

// Handler represents an action handler interface
type Handler interface {
	Execute(ctx context.Context, plan ExecutionPlan) Result
	CanHandle(actionType ActionType) bool
	Validate(plan ExecutionPlan) error
}

// Registry manages all available action handlers
type Registry struct {
	handlers map[ActionType]Handler
}

// NewRegistry creates a new action handler registry
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[ActionType]Handler),
	}
}

// Register adds a handler for the given action type
func (r *Registry) Register(actionType ActionType, handler Handler) {
	r.handlers[actionType] = handler
}

// GetHandler returns the handler for the given action type
func (r *Registry) GetHandler(actionType ActionType) (Handler, bool) {
	handler, exists := r.handlers[actionType]
	return handler, exists
}

// Execute executes the given plan using the appropriate handler
func (r *Registry) Execute(ctx context.Context, plan ExecutionPlan) Result {
	handler, exists := r.handlers[plan.Type]
	if !exists {
		return Result{
			Success:  false,
			Error:    fmt.Sprintf("no handler registered for action type: %s", plan.Type),
			Duration: 0,
		}
	}
	
	start := time.Now()
	result := handler.Execute(ctx, plan)
	result.Duration = time.Since(start)
	
	return result
}

// BaseHandler provides common functionality for all handlers
type BaseHandler struct {
	name string
}

// NewBaseHandler creates a new base handler
func NewBaseHandler(name string) BaseHandler {
	return BaseHandler{name: name}
}

// SystemHandler handles system-level operations
type SystemHandler struct {
	BaseHandler
}

// NewSystemHandler creates a new system handler
func NewSystemHandler() *SystemHandler {
	return &SystemHandler{
		BaseHandler: NewBaseHandler("system"),
	}
}

// Execute implements the Handler interface for system operations
func (h *SystemHandler) Execute(ctx context.Context, plan ExecutionPlan) Result {
	switch plan.Type {
	case ActionTypeOpen, ActionTypeReveal, ActionTypeShow:
		return h.executeSystemOpen(ctx, plan)
	case ActionTypeLaunch:
		return h.executeSystemLaunch(ctx, plan)
	default:
		return Result{
			Success: false,
			Error:   fmt.Sprintf("unsupported action type for system handler: %s", plan.Type),
		}
	}
}

// CanHandle checks if this handler can handle the given action type
func (h *SystemHandler) CanHandle(actionType ActionType) bool {
	return actionType == ActionTypeOpen || 
		   actionType == ActionTypeReveal || 
		   actionType == ActionTypeShow || 
		   actionType == ActionTypeLaunch
}

// Validate validates the execution plan
func (h *SystemHandler) Validate(plan ExecutionPlan) error {
	if !h.CanHandle(plan.Type) {
		return fmt.Errorf("cannot handle action type: %s", plan.Type)
	}
	
	if plan.Type == ActionTypeLaunch && plan.App == "" {
		return fmt.Errorf("app parameter required for launch action")
	}
	
	if (plan.Type == ActionTypeOpen || plan.Type == ActionTypeReveal || plan.Type == ActionTypeShow) && plan.Path == "" {
		return fmt.Errorf("path parameter required for %s action", plan.Type)
	}
	
	return nil
}

func (h *SystemHandler) executeSystemOpen(ctx context.Context, plan ExecutionPlan) Result {
	if plan.DryRun {
		return Result{
			Success: true,
			Output:  fmt.Sprintf("Would open: %s", plan.Path),
		}
	}
	
	var cmd *exec.Cmd
	switch plan.Type {
	case ActionTypeReveal:
		cmd = exec.CommandContext(ctx, "open", "-R", plan.Path)
	case ActionTypeShow:
		cmd = exec.CommandContext(ctx, "open", "-R", plan.Path)
	default: // ActionTypeOpen
		cmd = exec.CommandContext(ctx, "open", plan.Path)
	}
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return Result{
			Success:  false,
			Error:    fmt.Sprintf("failed to open: %v", err),
			Output:   string(output),
			ExitCode: cmd.ProcessState.ExitCode(),
		}
	}
	
	return Result{
		Success:  true,
		Output:   string(output),
		ExitCode: cmd.ProcessState.ExitCode(),
	}
}

func (h *SystemHandler) executeSystemLaunch(ctx context.Context, plan ExecutionPlan) Result {
	if plan.DryRun {
		return Result{
			Success: true,
			Output:  fmt.Sprintf("Would launch: %s", plan.App),
		}
	}
	
	cmd := exec.CommandContext(ctx, "open", "-a", plan.App)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return Result{
			Success:  false,
			Error:    fmt.Sprintf("failed to launch app: %v", err),
			Output:   string(output),
			ExitCode: cmd.ProcessState.ExitCode(),
		}
	}
	
	return Result{
		Success:  true,
		Output:   string(output),
		ExitCode: cmd.ProcessState.ExitCode(),
	}
}

// ScriptHandler handles script execution
type ScriptHandler struct {
	BaseHandler
}

// NewScriptHandler creates a new script handler
func NewScriptHandler() *ScriptHandler {
	return &ScriptHandler{
		BaseHandler: NewBaseHandler("script"),
	}
}

// Execute implements the Handler interface for script operations
func (h *ScriptHandler) Execute(ctx context.Context, plan ExecutionPlan) Result {
	switch plan.Type {
	case ActionTypeRun, ActionTypeScript, ActionTypeCommand:
		return h.executeScript(ctx, plan)
	default:
		return Result{
			Success: false,
			Error:   fmt.Sprintf("unsupported action type for script handler: %s", plan.Type),
		}
	}
}

// CanHandle checks if this handler can handle the given action type
func (h *ScriptHandler) CanHandle(actionType ActionType) bool {
	return actionType == ActionTypeRun || 
		   actionType == ActionTypeScript || 
		   actionType == ActionTypeCommand
}

// Validate validates the execution plan
func (h *ScriptHandler) Validate(plan ExecutionPlan) error {
	if !h.CanHandle(plan.Type) {
		return fmt.Errorf("cannot handle action type: %s", plan.Type)
	}
	
	if len(plan.Command) == 0 && plan.Script == "" {
		return fmt.Errorf("either command or script must be specified")
	}
	
	return nil
}

func (h *ScriptHandler) executeScript(ctx context.Context, plan ExecutionPlan) Result {
	if plan.DryRun {
		if len(plan.Command) > 0 {
			return Result{
				Success: true,
				Output:  fmt.Sprintf("Would run command: %s", strings.Join(plan.Command, " ")),
			}
		}
		return Result{
			Success: true,
			Output:  fmt.Sprintf("Would run script: %s", plan.Script),
		}
	}
	
	var cmd *exec.Cmd
	if len(plan.Command) > 0 {
		cmd = exec.CommandContext(ctx, plan.Command[0], plan.Command[1:]...)
	} else {
		// Create a temporary script file
		tmpFile, err := os.CreateTemp("", "rrunner-script-*.sh")
		if err != nil {
			return Result{
				Success: false,
				Error:   fmt.Sprintf("failed to create temp script: %v", err),
			}
		}
		defer os.Remove(tmpFile.Name())
		
		if _, err := tmpFile.WriteString(plan.Script); err != nil {
			return Result{
				Success: false,
				Error:   fmt.Sprintf("failed to write script: %v", err),
			}
		}
		tmpFile.Close()
		
		if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
			return Result{
				Success: false,
				Error:   fmt.Sprintf("failed to make script executable: %v", err),
			}
		}
		
		cmd = exec.CommandContext(ctx, "bash", tmpFile.Name())
	}
	
	// Set working directory if specified
	if plan.WorkingDir != "" {
		cmd.Dir = plan.WorkingDir
	}
	
	// Set environment variables
	if plan.Environment != nil {
		env := os.Environ()
		for k, v := range plan.Environment {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	}
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return Result{
			Success:  false,
			Error:    fmt.Sprintf("script execution failed: %v", err),
			Output:   string(output),
			ExitCode: cmd.ProcessState.ExitCode(),
		}
	}
	
	return Result{
		Success:  true,
		Output:   string(output),
		ExitCode: cmd.ProcessState.ExitCode(),
	}
}

// HTTPHandler handles HTTP requests and webhooks
type HTTPHandler struct {
	BaseHandler
	client *http.Client
}

// NewHTTPHandler creates a new HTTP handler
func NewHTTPHandler() *HTTPHandler {
	return &HTTPHandler{
		BaseHandler: NewBaseHandler("http"),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Execute implements the Handler interface for HTTP operations
func (h *HTTPHandler) Execute(ctx context.Context, plan ExecutionPlan) Result {
	switch plan.Type {
	case ActionTypeHTTP, ActionTypeWebhook:
		return h.executeHTTPRequest(ctx, plan)
	default:
		return Result{
			Success: false,
			Error:   fmt.Sprintf("unsupported action type for HTTP handler: %s", plan.Type),
		}
	}
}

// CanHandle checks if this handler can handle the given action type
func (h *HTTPHandler) CanHandle(actionType ActionType) bool {
	return actionType == ActionTypeHTTP || actionType == ActionTypeWebhook
}

// Validate validates the execution plan
func (h *HTTPHandler) Validate(plan ExecutionPlan) error {
	if !h.CanHandle(plan.Type) {
		return fmt.Errorf("cannot handle action type: %s", plan.Type)
	}
	
	if plan.Path == "" {
		return fmt.Errorf("URL is required for HTTP actions")
	}
	
	if _, err := url.ParseRequestURI(plan.Path); err != nil {
		return fmt.Errorf("invalid URL: %v", err)
	}
	
	return nil
}

func (h *HTTPHandler) executeHTTPRequest(ctx context.Context, plan ExecutionPlan) Result {
	if plan.DryRun {
		return Result{
			Success: true,
			Output:  fmt.Sprintf("Would make HTTP request to: %s", plan.Path),
		}
	}
	
	method := "GET"
	if envMethod, exists := plan.Environment["HTTP_METHOD"]; exists {
		method = envMethod
	}
	
	var body io.Reader
	if plan.Script != "" {
		body = strings.NewReader(plan.Script)
	}
	
	req, err := http.NewRequestWithContext(ctx, method, plan.Path, body)
	if err != nil {
		return Result{
			Success: false,
			Error:   fmt.Sprintf("failed to create request: %v", err),
		}
	}
	
	// Add headers from environment
	for k, v := range plan.Environment {
		if strings.HasPrefix(k, "HTTP_HEADER_") {
			headerName := strings.TrimPrefix(k, "HTTP_HEADER_")
			req.Header.Set(headerName, v)
		}
	}
	
	resp, err := h.client.Do(req)
	if err != nil {
		return Result{
			Success: false,
			Error:   fmt.Sprintf("HTTP request failed: %v", err),
		}
	}
	defer resp.Body.Close()
	
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{
			Success: false,
			Error:   fmt.Sprintf("failed to read response: %v", err),
		}
	}
	
	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	result := Result{
		Success:  success,
		Output:   string(respBody),
		ExitCode: resp.StatusCode,
	}
	
	if !success {
		result.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}
	
	return result
}

// TemplateHandler handles template processing
type TemplateHandler struct {
	BaseHandler
}

// NewTemplateHandler creates a new template handler
func NewTemplateHandler() *TemplateHandler {
	return &TemplateHandler{
		BaseHandler: NewBaseHandler("template"),
	}
}

// Execute implements the Handler interface for template operations
func (h *TemplateHandler) Execute(ctx context.Context, plan ExecutionPlan) Result {
	if plan.DryRun {
		return Result{
			Success: true,
			Output:  fmt.Sprintf("Would process template: %s", plan.Script),
		}
	}
	
	tmpl, err := template.New("action").Parse(plan.Script)
	if err != nil {
		return Result{
			Success: false,
			Error:   fmt.Sprintf("template parse error: %v", err),
		}
	}
	
	var buf bytes.Buffer
	data := map[string]interface{}{
		"Path":        plan.Path,
		"App":         plan.App,
		"Environment": plan.Environment,
		"WorkingDir":  plan.WorkingDir,
	}
	
	if err := tmpl.Execute(&buf, data); err != nil {
		return Result{
			Success: false,
			Error:   fmt.Sprintf("template execution error: %v", err),
		}
	}
	
	return Result{
		Success: true,
		Output:  buf.String(),
	}
}

// CanHandle checks if this handler can handle the given action type
func (h *TemplateHandler) CanHandle(actionType ActionType) bool {
	return actionType == ActionTypeTemplate
}

// Validate validates the execution plan
func (h *TemplateHandler) Validate(plan ExecutionPlan) error {
	if !h.CanHandle(plan.Type) {
		return fmt.Errorf("cannot handle action type: %s", plan.Type)
	}
	
	if plan.Script == "" {
		return fmt.Errorf("template content is required")
	}
	
	return nil
}

// Helper functions for template processing
func ProcessTemplate(content string, data interface{}) (string, error) {
	tmpl, err := template.New("template").Parse(content)
	if err != nil {
		return "", err
	}
	
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	
	return buf.String(), nil
}

// CreateDefaultRegistry creates a registry with all default handlers
func CreateDefaultRegistry() *Registry {
	registry := NewRegistry()
	
	// Register system handler for basic operations
	systemHandler := NewSystemHandler()
	registry.Register(ActionTypeOpen, systemHandler)
	registry.Register(ActionTypeReveal, systemHandler)
	registry.Register(ActionTypeShow, systemHandler)
	registry.Register(ActionTypeLaunch, systemHandler)
	
	// Register script handler for script execution
	scriptHandler := NewScriptHandler()
	registry.Register(ActionTypeRun, scriptHandler)
	registry.Register(ActionTypeScript, scriptHandler)
	registry.Register(ActionTypeCommand, scriptHandler)
	
	// Register HTTP handler for web operations
	httpHandler := NewHTTPHandler()
	registry.Register(ActionTypeHTTP, httpHandler)
	registry.Register(ActionTypeWebhook, httpHandler)
	
	// Register template handler
	templateHandler := NewTemplateHandler()
	registry.Register(ActionTypeTemplate, templateHandler)
	
	return registry
}
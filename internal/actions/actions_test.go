package actions

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestActionTypes(t *testing.T) {
	tests := []struct {
		actionType ActionType
		expected   string
	}{
		{ActionTypeOpen, "open"},
		{ActionTypeReveal, "reveal"},
		{ActionTypeRun, "run"},
		{ActionTypeHTTP, "http"},
		{ActionTypeTemplate, "template"},
	}
	
	for _, test := range tests {
		if string(test.actionType) != test.expected {
			t.Errorf("ActionType %v should equal %s", test.actionType, test.expected)
		}
	}
}

func TestRequest(t *testing.T) {
	req := Request{
		Raw:    "rrunner://open?url=file:///test.txt",
		Action: "open",
		Path:   "/test.txt",
		Context: Context{
			WorkingDir: "/tmp",
			User:       "testuser",
			Timestamp:  time.Now(),
			RequestID:  "test-123",
		},
	}
	
	if req.Action != "open" {
		t.Errorf("Expected action to be 'open', got %s", req.Action)
	}
	
	if req.Path != "/test.txt" {
		t.Errorf("Expected path to be '/test.txt', got %s", req.Path)
	}
	
	if req.Context.User != "testuser" {
		t.Errorf("Expected user to be 'testuser', got %s", req.Context.User)
	}
}

func TestExecutionPlan(t *testing.T) {
	plan := ExecutionPlan{
		Action:  "open",
		Type:    ActionTypeOpen,
		Path:    "/test.txt",
		Timeout: 30 * time.Second,
		DryRun:  true,
	}
	
	if plan.Action != "open" {
		t.Errorf("Expected action to be 'open', got %s", plan.Action)
	}
	
	if plan.Type != ActionTypeOpen {
		t.Errorf("Expected type to be ActionTypeOpen, got %v", plan.Type)
	}
	
	if !plan.DryRun {
		t.Error("Expected DryRun to be true")
	}
}

func TestResult(t *testing.T) {
	result := Result{
		Success:  true,
		Output:   "Command executed successfully",
		Duration: 1 * time.Second,
	}
	
	if !result.Success {
		t.Error("Expected result to be successful")
	}
	
	if result.Duration != 1*time.Second {
		t.Errorf("Expected duration to be 1s, got %v", result.Duration)
	}
}

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	if registry == nil {
		t.Fatal("Expected registry to be created")
	}
	
	if registry.handlers == nil {
		t.Error("Expected handlers map to be initialized")
	}
}

func TestRegistryRegisterAndGetHandler(t *testing.T) {
	registry := NewRegistry()
	handler := NewSystemHandler()
	
	registry.Register(ActionTypeOpen, handler)
	
	retrievedHandler, exists := registry.GetHandler(ActionTypeOpen)
	if !exists {
		t.Error("Expected handler to be found")
	}
	
	if retrievedHandler != handler {
		t.Error("Expected retrieved handler to match registered handler")
	}
	
	_, exists = registry.GetHandler(ActionTypeHTTP)
	if exists {
		t.Error("Expected non-registered handler to not be found")
	}
}

func TestSystemHandler(t *testing.T) {
	handler := NewSystemHandler()
	
	// Test CanHandle
	if !handler.CanHandle(ActionTypeOpen) {
		t.Error("Expected system handler to handle open action")
	}
	
	if !handler.CanHandle(ActionTypeLaunch) {
		t.Error("Expected system handler to handle launch action")
	}
	
	if handler.CanHandle(ActionTypeHTTP) {
		t.Error("Expected system handler to not handle HTTP action")
	}
}

func TestSystemHandlerValidation(t *testing.T) {
	handler := NewSystemHandler()
	
	// Valid open plan
	plan := ExecutionPlan{
		Type: ActionTypeOpen,
		Path: "/test.txt",
	}
	
	err := handler.Validate(plan)
	if err != nil {
		t.Errorf("Expected valid plan to pass validation, got error: %v", err)
	}
	
	// Invalid open plan (missing path)
	plan = ExecutionPlan{
		Type: ActionTypeOpen,
		// Path is missing
	}
	
	err = handler.Validate(plan)
	if err == nil {
		t.Error("Expected invalid plan to fail validation")
	}
	
	// Invalid launch plan (missing app)
	plan = ExecutionPlan{
		Type: ActionTypeLaunch,
		// App is missing
	}
	
	err = handler.Validate(plan)
	if err == nil {
		t.Error("Expected invalid launch plan to fail validation")
	}
}

func TestSystemHandlerExecute(t *testing.T) {
	handler := NewSystemHandler()
	ctx := context.Background()
	
	// Test dry run
	plan := ExecutionPlan{
		Type:   ActionTypeOpen,
		Path:   "/test.txt",
		DryRun: true,
	}
	
	result := handler.Execute(ctx, plan)
	if !result.Success {
		t.Error("Expected dry run to succeed")
	}
	
	if !strings.Contains(result.Output, "Would open") {
		t.Error("Expected dry run output to contain 'Would open'")
	}
}

func TestScriptHandler(t *testing.T) {
	handler := NewScriptHandler()
	
	// Test CanHandle
	if !handler.CanHandle(ActionTypeRun) {
		t.Error("Expected script handler to handle run action")
	}
	
	if !handler.CanHandle(ActionTypeScript) {
		t.Error("Expected script handler to handle script action")
	}
	
	if handler.CanHandle(ActionTypeOpen) {
		t.Error("Expected script handler to not handle open action")
	}
}

func TestScriptHandlerValidation(t *testing.T) {
	handler := NewScriptHandler()
	
	// Valid command plan
	plan := ExecutionPlan{
		Type:    ActionTypeCommand,
		Command: []string{"echo", "hello"},
	}
	
	err := handler.Validate(plan)
	if err != nil {
		t.Errorf("Expected valid plan to pass validation, got error: %v", err)
	}
	
	// Invalid plan (no command or script)
	plan = ExecutionPlan{
		Type: ActionTypeCommand,
		// Command and Script are both missing
	}
	
	err = handler.Validate(plan)
	if err == nil {
		t.Error("Expected invalid plan to fail validation")
	}
}

func TestHTTPHandler(t *testing.T) {
	handler := NewHTTPHandler()
	
	// Test CanHandle
	if !handler.CanHandle(ActionTypeHTTP) {
		t.Error("Expected HTTP handler to handle HTTP action")
	}
	
	if !handler.CanHandle(ActionTypeWebhook) {
		t.Error("Expected HTTP handler to handle webhook action")
	}
	
	if handler.CanHandle(ActionTypeOpen) {
		t.Error("Expected HTTP handler to not handle open action")
	}
}

func TestHTTPHandlerValidation(t *testing.T) {
	handler := NewHTTPHandler()
	
	// Valid HTTP plan
	plan := ExecutionPlan{
		Type: ActionTypeHTTP,
		Path: "https://example.com",
	}
	
	err := handler.Validate(plan)
	if err != nil {
		t.Errorf("Expected valid plan to pass validation, got error: %v", err)
	}
	
	// Invalid HTTP plan (missing URL)
	plan = ExecutionPlan{
		Type: ActionTypeHTTP,
		// Path (URL) is missing
	}
	
	err = handler.Validate(plan)
	if err == nil {
		t.Error("Expected invalid plan to fail validation")
	}
	
	// Invalid HTTP plan (invalid URL)
	plan = ExecutionPlan{
		Type: ActionTypeHTTP,
		Path: "not-a-valid-url",
	}
	
	err = handler.Validate(plan)
	if err == nil {
		t.Error("Expected invalid URL plan to fail validation")
	}
}

func TestTemplateHandler(t *testing.T) {
	handler := NewTemplateHandler()
	
	// Test CanHandle
	if !handler.CanHandle(ActionTypeTemplate) {
		t.Error("Expected template handler to handle template action")
	}
	
	if handler.CanHandle(ActionTypeOpen) {
		t.Error("Expected template handler to not handle open action")
	}
}

func TestTemplateHandlerValidation(t *testing.T) {
	handler := NewTemplateHandler()
	
	// Valid template plan
	plan := ExecutionPlan{
		Type:   ActionTypeTemplate,
		Script: "Hello {{.Path}}",
	}
	
	err := handler.Validate(plan)
	if err != nil {
		t.Errorf("Expected valid plan to pass validation, got error: %v", err)
	}
	
	// Invalid template plan (missing content)
	plan = ExecutionPlan{
		Type: ActionTypeTemplate,
		// Script (template content) is missing
	}
	
	err = handler.Validate(plan)
	if err == nil {
		t.Error("Expected invalid plan to fail validation")
	}
}

func TestTemplateHandlerExecute(t *testing.T) {
	handler := NewTemplateHandler()
	ctx := context.Background()
	
	plan := ExecutionPlan{
		Type:   ActionTypeTemplate,
		Script: "Hello {{.Path}}",
		Path:   "/test.txt",
		DryRun: true, // Use dry run to avoid side effects
	}
	
	result := handler.Execute(ctx, plan)
	if !result.Success {
		t.Errorf("Expected template execution to succeed, got error: %s", result.Error)
	}
	
	expected := "Hello /test.txt"
	if result.Output != expected {
		t.Errorf("Expected output %q, got %q", expected, result.Output)
	}
}

func TestProcessTemplate(t *testing.T) {
	template := "Hello {{.Name}}, you have {{.Count}} items"
	data := map[string]interface{}{
		"Name":  "World",
		"Count": 42,
	}
	
	result, err := ProcessTemplate(template, data)
	if err != nil {
		t.Fatalf("Expected template processing to succeed, got error: %v", err)
	}
	
	expected := "Hello World, you have 42 items"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestCreateDefaultRegistry(t *testing.T) {
	registry := CreateDefaultRegistry()
	if registry == nil {
		t.Fatal("Expected default registry to be created")
	}
	
	// Test that system handlers are registered
	_, exists := registry.GetHandler(ActionTypeOpen)
	if !exists {
		t.Error("Expected system handler to be registered for open action")
	}
	
	// Test that script handlers are registered
	_, exists = registry.GetHandler(ActionTypeRun)
	if !exists {
		t.Error("Expected script handler to be registered for run action")
	}
	
	// Test that HTTP handlers are registered
	_, exists = registry.GetHandler(ActionTypeHTTP)
	if !exists {
		t.Error("Expected HTTP handler to be registered for HTTP action")
	}
	
	// Test that template handlers are registered
	_, exists = registry.GetHandler(ActionTypeTemplate)
	if !exists {
		t.Error("Expected template handler to be registered for template action")
	}
}

func TestRegistryExecute(t *testing.T) {
	registry := NewRegistry()
	handler := NewSystemHandler()
	registry.Register(ActionTypeOpen, handler)
	
	ctx := context.Background()
	plan := ExecutionPlan{
		Type:   ActionTypeOpen,
		Path:   "/test.txt",
		DryRun: true,
	}
	
	result := registry.Execute(ctx, plan)
	if !result.Success {
		t.Error("Expected registry execution to succeed")
	}
	
	if result.Duration <= 0 {
		t.Error("Expected execution duration to be recorded")
	}
}

func TestRegistryExecuteUnknownHandler(t *testing.T) {
	registry := NewRegistry()
	
	ctx := context.Background()
	plan := ExecutionPlan{
		Type: ActionTypeHTTP, // No handler registered for this type
	}
	
	result := registry.Execute(ctx, plan)
	if result.Success {
		t.Error("Expected execution with unknown handler to fail")
	}
	
	if !strings.Contains(result.Error, "no handler registered") {
		t.Error("Expected error message about no handler registered")
	}
}
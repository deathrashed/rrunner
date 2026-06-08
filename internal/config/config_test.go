package config

import (
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	
	// Test basic settings
	if cfg.Settings.TerminalApp == "" {
		t.Error("Expected default terminal app to be set")
	}
	
	if cfg.Plugins.Enabled != true {
		t.Error("Expected plugins to be enabled by default")
	}
	
	if cfg.Security.AllowInlineCommands != true {
		t.Error("Expected inline commands to be allowed by default")
	}
	
	if cfg.Logging.Level != "info" {
		t.Error("Expected default log level to be info")
	}
	
	if cfg.Performance.MaxConcurrency < 1 {
		t.Error("Expected max concurrency to be at least 1")
	}
}

func TestConfigValidation(t *testing.T) {
	cfg := DefaultConfig()
	
	// Valid config should have no errors
	errors := cfg.Validate()
	if len(errors) > 0 {
		t.Errorf("Expected valid config to have no errors, got: %v", errors)
	}
	
	// Test invalid log level
	cfg.Logging.Level = "invalid"
	errors = cfg.Validate()
	found := false
	for _, err := range errors {
		if strings.Contains(err, "invalid log level") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected validation error for invalid log level")
	}
	
	// Test invalid max concurrency
	cfg = DefaultConfig()
	cfg.Performance.MaxConcurrency = 0
	errors = cfg.Validate()
	found = false
	for _, err := range errors {
		if strings.Contains(err, "max_concurrency must be at least 1") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected validation error for invalid max concurrency")
	}
}

func TestActionSpecValidation(t *testing.T) {
	cfg := DefaultConfig()
	
	// Add an action without type
	cfg.Actions["test"] = ActionSpec{
		Name: "test",
		// Type is missing
	}
	
	errors := cfg.Validate()
	found := false
	for _, err := range errors {
		if strings.Contains(err, "missing type") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected validation error for action missing type")
	}
	
	// Test command action without command
	cfg.Actions["test"] = ActionSpec{
		Name: "test",
		Type: "command",
		// Command is missing
	}
	
	errors = cfg.Validate()
	found = false
	for _, err := range errors {
		if strings.Contains(err, "missing command") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected validation error for command action missing command")
	}
}

func TestConfigToJSON(t *testing.T) {
	cfg := DefaultConfig()
	
	data, err := cfg.ToJSON()
	if err != nil {
		t.Fatalf("Expected ToJSON to succeed, got error: %v", err)
	}
	
	if len(data) == 0 {
		t.Error("Expected JSON data to be non-empty")
	}
	
	// Check if JSON contains expected fields
	jsonStr := string(data)
	expectedFields := []string{"settings", "plugins", "security", "logging", "metrics", "performance"}
	for _, field := range expectedFields {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("Expected JSON to contain field: %s", field)
		}
	}
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		// Note: ~/path expansion testing would require mocking os.UserHomeDir
	}
	
	for _, test := range tests {
		result := expandPath(test.input)
		if result != test.expected {
			t.Errorf("expandPath(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestContains(t *testing.T) {
	slice := []string{"apple", "banana", "cherry"}
	
	if !contains(slice, "banana") {
		t.Error("Expected contains to return true for 'banana'")
	}
	
	if contains(slice, "orange") {
		t.Error("Expected contains to return false for 'orange'")
	}
}
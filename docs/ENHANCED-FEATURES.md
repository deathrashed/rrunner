# Enhanced Features

This document describes the advanced features and enhancements available in Rrunner Enhanced (v0.3.0+).

## Table of Contents

- [Overview](#overview)
- [Enhanced Architecture](#enhanced-architecture)
- [New Action Types](#new-action-types)
- [Advanced Configuration](#advanced-configuration)
- [Metrics and Monitoring](#metrics-and-monitoring)
- [Structured Logging](#structured-logging)
- [Plugin System Enhancements](#plugin-system-enhancements)
- [Performance Optimizations](#performance-optimizations)
- [Security Enhancements](#security-enhancements)
- [Development Tools](#development-tools)
- [Server Mode](#server-mode)
- [Testing Framework](#testing-framework)

## Overview

Rrunner Enhanced is a complete rewrite and enhancement of the original Rrunner, providing:

- **Modular Architecture**: Well-organized internal packages for better maintainability
- **Advanced Actions**: Support for HTTP requests, templates, webhooks, and more
- **Comprehensive Monitoring**: Built-in metrics collection and health monitoring
- **Enhanced Security**: Granular security controls and validation
- **Development Tools**: Extensive testing framework and development utilities
- **Server Mode**: HTTP API for remote execution and monitoring

## Enhanced Architecture

The enhanced version is organized into logical internal packages:

```
internal/
├── config/         # Configuration management with validation
├── actions/        # Action handlers and execution engine  
├── plugins/        # Advanced plugin system with hot-reload
├── registry/       # Action and plugin registry management
├── metrics/        # Comprehensive metrics collection
├── logging/        # Structured logging with hooks and formatters
├── utils/          # Common utilities, caching, and rate limiting
└── testing/        # Testing framework with mocks and fixtures
```

## New Action Types

### HTTP Actions

Execute HTTP requests directly from URL schemes:

```
rrunner://http?url=https://api.example.com/webhook&method=POST
```

**Features:**
- Support for GET, POST, PUT, DELETE, etc.
- Custom headers via environment variables
- Request body from script parameter
- Response handling and status codes

### Template Actions

Process templates with dynamic data:

```toml
[actions.notify]
type = "template"
description = "Send notification with template"
template = "Alert: {{.Path}} has been modified by {{.User}} at {{.Timestamp}}"
```

**Features:**
- Go template syntax support
- Context data injection (Path, User, Environment, etc.)
- Conditional logic and loops
- Template inheritance

### Webhook Actions

Specialized HTTP actions for webhook integrations:

```
rrunner://webhook?url=https://hooks.slack.com/services/T00/B00/XXX
```

**Features:**
- Pre-configured for common webhook services
- Automatic JSON payload formatting
- Retry logic and error handling
- Rate limiting protection

### File Operations

Enhanced file manipulation capabilities:

```
rrunner://file?action=copy&src=/path/to/source&dest=/path/to/destination
rrunner://file?action=move&src=/path/to/file&dest=/new/location
rrunner://file?action=delete&path=/path/to/file
```

### Directory Operations

Directory management and manipulation:

```
rrunner://directory?action=create&path=/path/to/new/dir
rrunner://directory?action=list&path=/path/to/dir&format=json
```

### Environment Operations

Environment variable management:

```
rrunner://env?action=set&name=MY_VAR&value=my_value
rrunner://env?action=get&name=MY_VAR
rrunner://env?action=list&prefix=RRUNNER_
```

## Advanced Configuration

### Comprehensive Settings

```toml
[settings]
terminal_app = "Ghostty"
keep_open = true
text_editor = "Visual Studio Code"
code_editor = "Cursor"
markdown_previewer = "Marked 2"

[plugins]
enabled = true
auto_load = true
dirs = ["~/.config/rrunner/plugins"]

[security]
allow_inline_commands = true
allow_plugin_commands = false
require_trusted_plugins_for_commands = true
allow_remote_plugins = false
sandbox_plugins = false

[logging]
level = "info"
file = "~/.config/rrunner/logs/rrunner.log"
max_size = 100          # MB
max_backups = 3
max_age = 28           # days
compress = true

[metrics]
enabled = true
port = 9090
path = "/metrics"
collect_usage = true

[performance]
max_concurrency = 10
enable_caching = true
cache_size = 100
request_timeout = 30   # seconds
plugin_timeout = 60    # seconds
```

### Environment-Specific Configuration

Support for different configuration profiles:

```bash
# Development
rrunner --config ~/.config/rrunner/dev.toml

# Production  
rrunner --config ~/.config/rrunner/prod.toml

# Testing
rrunner --config ~/.config/rrunner/test.toml
```

### Configuration Validation

Built-in validation ensures configuration correctness:

```bash
rrunner --validate-config ~/.config/rrunner/config.toml
```

## Metrics and Monitoring

### Built-in Metrics

The enhanced version collects comprehensive metrics:

- **Request Metrics**: Total requests, success/failure rates, duration histograms
- **Action Metrics**: Per-action execution counts, durations, error rates
- **Plugin Metrics**: Plugin load counts, errors, performance
- **System Metrics**: CPU usage, memory usage, goroutine counts
- **Custom Metrics**: User-defined counters, gauges, and histograms

### Metrics Endpoints

Access metrics via HTTP endpoints:

```bash
# Prometheus format
curl http://localhost:8080/metrics

# JSON format  
curl http://localhost:8080/metrics/json

# Health check
curl http://localhost:8080/health
```

### Metric Examples

```
# HELP rrunner_requests_total Total number of requests
# TYPE rrunner_requests_total counter
rrunner_requests_total 1547

# HELP rrunner_request_duration_seconds Request duration distribution
# TYPE rrunner_request_duration_seconds histogram
rrunner_request_duration_seconds_bucket{le="0.1"} 1234
rrunner_request_duration_seconds_bucket{le="0.5"} 1456
rrunner_request_duration_seconds_bucket{le="1.0"} 1500

# HELP rrunner_action_total Number of executions per action
# TYPE rrunner_action_total counter  
rrunner_action_total{action="open"} 523
rrunner_action_total{action="launch"} 234
```

## Structured Logging

### Advanced Logging Features

- **Multiple Output Formats**: Text and JSON formats
- **Log Rotation**: Automatic rotation with size and age limits
- **Hook System**: Extensible hook system for custom log processing
- **Context Logging**: Request ID and component tracking
- **Performance**: High-performance, concurrent-safe logging

### Log Levels

```bash
rrunner --log-level debug   # Detailed debugging information
rrunner --log-level info    # General operational messages  
rrunner --log-level warn    # Warning conditions
rrunner --log-level error   # Error conditions only
```

### Structured Output

JSON format for machine processing:

```json
{
  "time": "2023-12-07T15:04:05.123456789Z",
  "level": "INFO",
  "message": "Action executed successfully",
  "component": "actions",
  "request_id": "20231207150405-abc12345",
  "fields": {
    "action": "open",
    "path": "/path/to/file.txt",
    "duration_ms": 45
  }
}
```

### Log Hooks

Custom log processing with hooks:

```go
// Example: Send error logs to Slack
type SlackHook struct {
    webhookURL string
}

func (h *SlackHook) Fire(entry *logging.Entry) error {
    if entry.Level >= logging.LevelError {
        // Send to Slack
        return sendToSlack(h.webhookURL, entry.Message)
    }
    return nil
}
```

## Plugin System Enhancements

### Hot Reload

Plugins can be reloaded without restarting:

```bash
rrunner --reload-plugin my-plugin-id
```

### Dependency Management

Plugins can declare dependencies on other plugins:

```toml
[plugin]
id = "my-plugin"
name = "My Plugin"
version = "1.0.0"
dependencies = ["base-plugin", "utility-plugin"]
```

### Plugin Versioning

Semantic versioning support with compatibility checks:

```toml
[plugin]
id = "my-plugin"
version = "2.1.0"
min_rrunner_version = "0.3.0"
max_rrunner_version = "0.4.0"
```

### Security Controls

Granular security controls for plugins:

```toml
[security]
allow_plugin_commands = false
require_trusted_plugins_for_commands = true

[plugins.trusted]
"my-safe-plugin" = true
"community-plugin" = false
```

### Plugin Templates

Plugin template system for common patterns:

```toml
[templates.http-webhook]
name = "HTTP Webhook Template"
description = "Template for HTTP webhook integrations"
variables = ["webhook_url", "secret_key"]
```

## Performance Optimizations

### Caching System

Built-in caching for improved performance:

- **Request Parsing Cache**: Cache parsed URL requests
- **Execution Plan Cache**: Cache action execution plans  
- **Plugin Cache**: Cache loaded plugin manifests
- **Template Cache**: Cache compiled templates

### Rate Limiting

Protect against abuse with rate limiting:

```toml
[performance]
rate_limit_enabled = true
rate_limit_requests = 100    # requests per window
rate_limit_window = "1m"     # time window
```

### Concurrency Control

Control concurrent execution:

```toml
[performance]
max_concurrency = 10         # max concurrent actions
enable_async = true          # enable async execution
async_queue_size = 1000      # async queue size
```

### Resource Monitoring

Monitor resource usage:

- CPU usage tracking
- Memory usage monitoring  
- Goroutine leak detection
- File descriptor tracking

## Security Enhancements

### Input Validation

Comprehensive input validation:

- URL scheme validation
- Path traversal protection
- Command injection prevention
- Parameter sanitization

### Execution Sandboxing

Sandbox untrusted code execution:

```toml
[security]
sandbox_plugins = true
sandbox_timeout = "30s"
sandbox_memory_limit = "100MB"
sandbox_network_access = false
```

### Audit Logging

Security event logging:

- All command executions logged
- Plugin loading events tracked
- Security violations recorded
- Access attempts monitored

### Permission System

Fine-grained permission controls:

```toml
[permissions]
allow_file_read = true
allow_file_write = false  
allow_network_access = false
allow_system_commands = true
```

## Development Tools

### Enhanced Makefile

Comprehensive build system:

```bash
make help              # Show all available targets
make dev               # Set up development environment
make test-coverage     # Run tests with coverage
make lint             # Run all linters
make security-scan    # Security analysis
make release          # Build release binaries
make docker-build     # Build Docker image
```

### Testing Framework

Built-in testing framework with:

- **Test Suites**: Organized test execution
- **Mock Handlers**: Mock action handlers for testing
- **Test Fixtures**: Reusable test data and configurations
- **Assertion Helpers**: Common assertion functions
- **Coverage Reports**: HTML and text coverage reports

### Development Configuration

Development-specific settings:

```bash
# Enable debug mode
export RRUNNER_DEBUG=1

# Enable verbose logging
export RRUNNER_LOG_LEVEL=debug

# Enable development features  
export RRUNNER_DEV_MODE=1
```

## Server Mode

### HTTP API

Start Rrunner as an HTTP server:

```bash
rrunner --server --host localhost --port 8080
```

### API Endpoints

Available HTTP endpoints:

```
GET  /                 # Server information
GET  /health          # Health check
GET  /metrics         # Metrics (Prometheus format)
GET  /metrics/json    # Metrics (JSON format)
GET  /actions         # List available actions
POST /execute         # Execute action
GET  /plugins         # List loaded plugins
```

### API Examples

Execute actions via HTTP:

```bash
# List actions
curl http://localhost:8080/actions

# Execute action
curl -X POST http://localhost:8080/execute \
  -H "Content-Type: application/json" \
  -d '{"url": "rrunner://open?url=file:///path/to/file.txt"}'

# Check health
curl http://localhost:8080/health
```

### Web UI

Optional web interface for:

- Action management
- Plugin configuration  
- Metrics visualization
- Log viewing
- System monitoring

## Testing Framework

### Test Suite System

Organize tests into logical suites:

```go
suite := testing.NewTestSuite("Action Tests")
suite.AddTest(testing.TestCase{
    Name: "Open File Action",
    Test: func() error {
        // Test implementation
        return nil
    },
})
results := suite.RunAll()
```

### Mock System

Comprehensive mocking for testing:

```go
mockHandler := testing.NewMockHandler("mock")
mockHandler.SetResponse("open", actions.Result{
    Success: true,
    Output:  "Mock file opened",
})
```

### Test Fixtures

Reusable test data and configurations:

```go
fixtures := testing.NewTestFixtures()
configFile, _ := fixtures.CreateConfig()
testScript, _ := fixtures.CreateExecutableScript("test.sh", "echo hello")
defer fixtures.Cleanup()
```

### Integration Testing

End-to-end testing support:

```bash
make test-integration    # Run integration tests
make test-performance   # Run performance tests
make test-security      # Run security tests
```

## Migration from Legacy

### Compatibility

Enhanced version maintains compatibility with:

- Existing URL schemes
- Configuration files (with extensions)
- Plugin manifests (with enhancements)
- Shell scripts and handlers

### Migration Steps

1. **Backup Configuration**: Save existing configurations
2. **Install Enhanced Version**: Use new installation method
3. **Update Configuration**: Migrate to enhanced format
4. **Test Functionality**: Verify all actions work
5. **Update Plugins**: Upgrade to enhanced plugin format

### Coexistence

Both versions can coexist:

```bash
# Legacy version
~/.local/bin/rrunner

# Enhanced version  
~/.local/bin/rrunner-enhanced
```

## Best Practices

### Configuration

- Use environment-specific configurations
- Enable metrics and logging in production
- Regularly validate configuration files
- Keep security settings restrictive by default

### Plugin Development

- Follow semantic versioning
- Declare all dependencies
- Implement proper error handling
- Include comprehensive tests
- Document plugin capabilities

### Performance

- Enable caching for frequently used actions
- Set appropriate concurrency limits
- Monitor resource usage regularly
- Use async execution for long-running tasks

### Security

- Enable security features by default
- Regularly review plugin permissions
- Monitor audit logs for suspicious activity
- Keep plugins updated to latest versions

## Future Enhancements

Planned features for future releases:

- **Plugin Marketplace**: Central plugin repository
- **Advanced Templates**: More template engines and features
- **Distributed Execution**: Execute actions on remote machines
- **Workflow Engine**: Chain multiple actions together
- **Machine Learning**: Intelligent action suggestions
- **Cloud Integration**: Native cloud service integrations

## Support and Documentation

- **GitHub Repository**: https://github.com/deathrashed/rrunner
- **Documentation**: Comprehensive docs in `/docs` directory
- **Issue Tracker**: Report bugs and request features
- **Discussions**: Community discussions and questions
- **Wiki**: Additional examples and tutorials
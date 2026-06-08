package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"
	
	"github.com/deathrashed/rrunner/internal/actions"
	"github.com/deathrashed/rrunner/internal/config"
	"github.com/deathrashed/rrunner/internal/logging"
	"github.com/deathrashed/rrunner/internal/metrics"
	"github.com/deathrashed/rrunner/internal/plugins"
	"github.com/deathrashed/rrunner/internal/registry"
	"github.com/deathrashed/rrunner/internal/utils"
)

const version = "0.3.0-enhanced"

// CLI represents the command-line interface
type CLI struct {
	config      config.Config
	logger      *logging.Logger
	metrics     *metrics.Metrics
	registry    *registry.Registry
	pluginMgr   *plugins.Manager
	actionReg   *actions.Registry
	startTime   time.Time
	requestID   string
}

// NewCLI creates a new CLI instance
func NewCLI() *CLI {
	return &CLI{
		config:    config.DefaultConfig(),
		startTime: time.Now(),
		requestID: utils.GenerateRequestID(),
	}
}

// Initialize sets up the CLI with configuration and dependencies
func (cli *CLI) Initialize(configPath string) error {
	// Load configuration
	if configPath != "" {
		// TODO: Load configuration from file
		cli.logger.Info("Loading configuration from: %s", configPath)
	}
	
	// Initialize logging
	logging.Initialize(logging.ParseLevel(cli.config.Logging.Level), "rrunner-enhanced")
	cli.logger = logging.GetDefaultLogger().WithComponent("cli").WithRequestID(cli.requestID)
	
	// Add file logging if configured
	if cli.config.Logging.File != "" {
		fileHook, err := logging.NewFileHook(
			utils.ExpandPath(cli.config.Logging.File),
			int64(cli.config.Logging.MaxSize)*1024*1024, // Convert MB to bytes
			cli.config.Logging.MaxBackups,
			cli.config.Logging.MaxAge,
			cli.config.Logging.Compress,
		)
		if err != nil {
			cli.logger.Warn("Failed to create file hook: %v", err)
		} else {
			logging.AddGlobalHook(fileHook)
		}
	}
	
	// Initialize metrics
	cli.metrics = metrics.NewMetrics()
	if cli.config.Metrics.Enabled {
		cli.metrics.Enable()
		cli.logger.Info("Metrics collection enabled")
	}
	
	// Initialize plugin manager
	cli.pluginMgr = plugins.NewManager(cli.config)
	
	// Initialize action registry
	cli.actionReg = actions.CreateDefaultRegistry()
	
	// Initialize registry
	cli.registry = registry.NewRegistry(cli.config, cli.pluginMgr, cli.actionReg)
	
	cli.logger.Info("CLI initialized successfully")
	return nil
}

// Execute runs the CLI with the given arguments
func (cli *CLI) Execute(args []string) error {
	if len(args) == 0 {
		return cli.showHelp()
	}
	
	// Parse flags
	fs := flag.NewFlagSet("rrunner", flag.ExitOnError)
	
	var (
		showVersion       = fs.Bool("version", false, "Show version information")
		showHelp         = fs.Bool("help", false, "Show help information")
		configPath       = fs.String("config", "", "Path to configuration file")
		dryRun           = fs.Bool("dry-run", false, "Show what would be executed without running")
		verbose          = fs.Bool("verbose", false, "Enable verbose output")
		quiet            = fs.Bool("quiet", false, "Suppress output")
		logLevel         = fs.String("log-level", "info", "Log level (debug, info, warn, error)")
		output           = fs.String("output", "text", "Output format (text, json, yaml)")
		timeout          = fs.Duration("timeout", 30*time.Second, "Request timeout")
		
		// Action management
		listActions      = fs.Bool("list-actions", false, "List available actions")
		explainAction    = fs.String("explain-action", "", "Explain a specific action")
		validateAction   = fs.String("validate-action", "", "Validate an action configuration")
		
		// Plugin management
		listPlugins      = fs.Bool("list-plugins", false, "List loaded plugins")
		reloadPlugin     = fs.String("reload-plugin", "", "Reload a specific plugin")
		
		// Diagnostics
		diagnose         = fs.Bool("diagnose", false, "Run system diagnostics")
		healthCheck      = fs.Bool("health", false, "Perform health check")
		showMetrics      = fs.Bool("metrics", false, "Show current metrics")
		
		// Server mode
		server           = fs.Bool("server", false, "Start in server mode")
		serverPort       = fs.Int("port", 8080, "Server port")
		serverHost       = fs.String("host", "localhost", "Server host")
	)
	
	if err := fs.Parse(args); err != nil {
		return err
	}
	
	// Override log level if specified
	if *logLevel != "info" {
		logging.SetGlobalLevel(logging.ParseLevel(*logLevel))
		cli.config.Logging.Level = *logLevel
	}
	
	// Handle quiet/verbose flags
	if *quiet {
		logging.SetGlobalLevel(logging.LevelError)
	} else if *verbose {
		logging.SetGlobalLevel(logging.LevelDebug)
	}
	
	// Initialize CLI with configuration
	if err := cli.Initialize(*configPath); err != nil {
		return fmt.Errorf("failed to initialize CLI: %v", err)
	}
	
	// Handle version
	if *showVersion {
		return cli.showVersion(*output)
	}
	
	// Handle help
	if *showHelp {
		return cli.showHelp()
	}
	
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	
	// Handle various commands
	switch {
	case *listActions:
		return cli.listActions(*output)
	case *explainAction != "":
		return cli.explainAction(*explainAction, *output)
	case *validateAction != "":
		return cli.validateAction(*validateAction, *output)
	case *listPlugins:
		return cli.listPlugins(*output)
	case *reloadPlugin != "":
		return cli.reloadPlugin(*reloadPlugin, *output)
	case *diagnose:
		return cli.diagnose(*output)
	case *healthCheck:
		return cli.healthCheck(*output)
	case *showMetrics:
		return cli.showMetrics(*output)
	case *server:
		return cli.startServer(ctx, *serverHost, *serverPort)
	}
	
	// Handle URL execution (remaining args should be URL)
	remainingArgs := fs.Args()
	if len(remainingArgs) > 0 {
		url := remainingArgs[0]
		return cli.executeURL(ctx, url, *dryRun, *output)
	}
	
	return cli.showHelp()
}

// showVersion displays version information
func (cli *CLI) showVersion(format string) error {
	info := map[string]interface{}{
		"version":    version,
		"build_time": cli.startTime.Format(time.RFC3339),
		"go_version": utils.GetSystemInfo()["go_version"],
		"platform":   utils.GetSystemInfo()["os"] + "/" + utils.GetSystemInfo()["arch"],
	}
	
	switch format {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(info)
	default:
		fmt.Printf("Rrunner Enhanced v%s\n", version)
		fmt.Printf("Build Time: %s\n", cli.startTime.Format(time.RFC3339))
		fmt.Printf("Go Version: %s\n", utils.GetSystemInfo()["go_version"])
		fmt.Printf("Platform: %s\n", utils.GetSystemInfo()["os"]+"/"+utils.GetSystemInfo()["arch"])
	}
	
	return nil
}

// showHelp displays help information
func (cli *CLI) showHelp() error {
	fmt.Printf(`Rrunner Enhanced v%s - Advanced URL scheme launcher

USAGE:
    rrunner [FLAGS] [URL|COMMAND]

FLAGS:
    --version                Show version information
    --help                   Show this help
    --config PATH           Path to configuration file
    --dry-run               Show what would be executed
    --verbose               Enable verbose output
    --quiet                 Suppress output
    --log-level LEVEL       Log level (debug, info, warn, error)
    --output FORMAT         Output format (text, json)
    --timeout DURATION      Request timeout (default: 30s)

ACTION MANAGEMENT:
    --list-actions          List available actions
    --explain-action NAME   Explain a specific action
    --validate-action NAME  Validate action configuration

PLUGIN MANAGEMENT:
    --list-plugins          List loaded plugins
    --reload-plugin ID      Reload a specific plugin

DIAGNOSTICS:
    --diagnose              Run system diagnostics
    --health                Perform health check
    --metrics               Show current metrics

SERVER MODE:
    --server                Start in server mode
    --host HOST            Server host (default: localhost)
    --port PORT            Server port (default: 8080)

EXAMPLES:
    rrunner 'rrunner://open?url=file:///path/to/file.txt'
    rrunner --list-actions --output json
    rrunner --explain-action open
    rrunner --server --port 8080

For more information, see: https://github.com/deathrashed/rrunner
`, version)
	
	return nil
}

// listActions lists all available actions
func (cli *CLI) listActions(format string) error {
	actionSpecs := cli.registry.ListActions()
	
	switch format {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(map[string]interface{}{
			"actions": actionSpecs,
			"count":   len(actionSpecs),
		})
	default:
		fmt.Printf("Available Actions (%d):\n\n", len(actionSpecs))
		
		// Group by source
		bySource := make(map[string][]config.ActionSpec)
		for _, spec := range actionSpecs {
			source := spec.Source
			if source == "" {
				source = "unknown"
			}
			bySource[source] = append(bySource[source], spec)
		}
		
		// Sort sources
		sources := make([]string, 0, len(bySource))
		for source := range bySource {
			sources = append(sources, source)
		}
		sort.Strings(sources)
		
		for _, source := range sources {
			fmt.Printf("%s:\n", strings.ToUpper(source))
			actions := bySource[source]
			sort.Slice(actions, func(i, j int) bool {
				return actions[i].Name < actions[j].Name
			})
			
			for _, action := range actions {
				description := action.Description
				if description == "" {
					description = "No description available"
				}
				fmt.Printf("  %-15s %s\n", action.Name, description)
				if len(action.Aliases) > 0 {
					fmt.Printf("  %15s (aliases: %s)\n", "", strings.Join(action.Aliases, ", "))
				}
			}
			fmt.Println()
		}
	}
	
	return nil
}

// explainAction provides detailed information about an action
func (cli *CLI) explainAction(actionName, format string) error {
	action, exists := cli.registry.GetAction(actionName)
	if !exists {
		return fmt.Errorf("action not found: %s", actionName)
	}
	
	switch format {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(action)
	default:
		fmt.Printf("Action: %s\n", action.Name)
		fmt.Printf("Type: %s\n", action.Type)
		fmt.Printf("Source: %s\n", action.Source)
		
		if action.Description != "" {
			fmt.Printf("Description: %s\n", action.Description)
		}
		
		if len(action.Aliases) > 0 {
			fmt.Printf("Aliases: %s\n", strings.Join(action.Aliases, ", "))
		}
		
		// Show example usage
		fmt.Printf("\nExample Usage:\n")
		fmt.Printf("  rrunner 'rrunner://%s?url=file:///path/to/file'\n", action.Name)
	}
	
	return nil
}

// executeURL executes a URL request
func (cli *CLI) executeURL(ctx context.Context, rawURL string, dryRun bool, format string) error {
	start := time.Now()
	
	cli.logger.Info("Executing URL: %s", rawURL)
	if cli.metrics.IsEnabled() {
		defer func() {
			duration := time.Since(start)
			cli.metrics.RecordRequest(duration, true, "url_execution")
		}()
	}
	
	// Parse the request
	request, err := cli.registry.ParseRequest(rawURL)
	if err != nil {
		cli.logger.Error("Failed to parse request: %v", err)
		return fmt.Errorf("failed to parse request: %v", err)
	}
	
	// Plan the execution
	plan, err := cli.registry.PlanExecution(request, dryRun)
	if err != nil {
		cli.logger.Error("Failed to plan execution: %v", err)
		return fmt.Errorf("failed to plan execution: %v", err)
	}
	
	if dryRun {
		switch format {
		case "json":
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(plan)
		default:
			fmt.Printf("Dry Run - Execution Plan:\n")
			fmt.Printf("Action: %s\n", plan.Action)
			fmt.Printf("Type: %s\n", plan.Type)
			if len(plan.Command) > 0 {
				fmt.Printf("Command: %s\n", strings.Join(plan.Command, " "))
			}
			if plan.Path != "" {
				fmt.Printf("Path: %s\n", plan.Path)
			}
		}
		return nil
	}
	
	// Execute the plan
	result := cli.registry.Execute(ctx, plan)
	
	// Output result
	switch format {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	default:
		if result.Success {
			fmt.Printf("Success: %s\n", request.Action)
			if result.Output != "" {
				fmt.Printf("Output: %s\n", result.Output)
			}
		} else {
			fmt.Printf("Failed: %s\n", request.Action)
			if result.Error != "" {
				fmt.Printf("Error: %s\n", result.Error)
			}
			os.Exit(result.ExitCode)
		}
	}
	
	return nil
}

// startServer starts the HTTP server
func (cli *CLI) startServer(ctx context.Context, host string, port int) error {
	mux := http.NewServeMux()
	
	// Setup routes
	mux.HandleFunc("/", cli.handleIndex)
	mux.HandleFunc("/health", cli.handleHealth)
	mux.HandleFunc("/actions", cli.handleActions)
	mux.HandleFunc("/execute", cli.handleExecute)
	
	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", host, port),
		Handler: mux,
	}
	
	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		
		cli.logger.Info("Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		
		if err := server.Shutdown(shutdownCtx); err != nil {
			cli.logger.Error("Server shutdown error: %v", err)
		}
	}()
	
	cli.logger.Info("Starting server on %s:%d", host, port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %v", err)
	}
	
	return nil
}

// HTTP handlers
func (cli *CLI) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>Rrunner Enhanced v%s</title>
</head>
<body>
    <h1>Rrunner Enhanced v%s</h1>
    <p>Available endpoints:</p>
    <ul>
        <li><a href="/health">Health Check</a></li>
        <li><a href="/actions">Actions</a></li>
    </ul>
</body>
</html>`, version, version)
}

func (cli *CLI) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	health := map[string]interface{}{
		"status":    "ok",
		"version":   version,
		"timestamp": time.Now().UTC(),
		"uptime":    time.Since(cli.startTime).String(),
	}
	
	json.NewEncoder(w).Encode(health)
}

func (cli *CLI) handleActions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	actionSpecs := cli.registry.ListActions()
	response := map[string]interface{}{
		"actions": actionSpecs,
		"count":   len(actionSpecs),
	}
	
	json.NewEncoder(w).Encode(response)
}

func (cli *CLI) handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	
	var request struct {
		URL    string `json:"url"`
		DryRun bool   `json:"dry_run,omitempty"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	// Execute request (simplified for this implementation)
	response := map[string]interface{}{
		"url":     request.URL,
		"dry_run": request.DryRun,
		"status":  "executed",
	}
	
	json.NewEncoder(w).Encode(response)
}

// Additional command implementations
func (cli *CLI) validateAction(actionName, format string) error {
	errors := cli.registry.ValidateAction(actionName)
	
	switch format {
	case "json":
		response := map[string]interface{}{
			"action": actionName,
			"valid":  len(errors) == 0,
			"errors": errors,
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(response)
	default:
		if len(errors) == 0 {
			fmt.Printf("Action '%s' is valid\n", actionName)
		} else {
			fmt.Printf("Action '%s' has validation errors:\n", actionName)
			for _, err := range errors {
				fmt.Printf("  - %s\n", err)
			}
		}
	}
	
	return nil
}

func (cli *CLI) listPlugins(format string) error {
	fmt.Println("Plugin listing not yet implemented in enhanced version")
	return nil
}

func (cli *CLI) reloadPlugin(pluginID, format string) error {
	fmt.Printf("Plugin reloading not yet implemented: %s\n", pluginID)
	return nil
}

func (cli *CLI) diagnose(format string) error {
	fmt.Println("Diagnostics not yet implemented in enhanced version")
	return nil
}

func (cli *CLI) healthCheck(format string) error {
	fmt.Println("Health check not yet implemented in enhanced version")
	return nil
}

func (cli *CLI) showMetrics(format string) error {
	if !cli.metrics.IsEnabled() {
		fmt.Println("Metrics are disabled")
		return nil
	}
	
	switch format {
	case "json":
		data, err := cli.metrics.ExportJSON()
		if err != nil {
			return err
		}
		fmt.Print(string(data))
	default:
		fmt.Print(cli.metrics.ExportPrometheus())
	}
	
	return nil
}

func main() {
	cli := NewCLI()
	
	if err := cli.Execute(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
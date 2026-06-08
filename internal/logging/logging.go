package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Level represents log levels
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

// String returns the string representation of a log level
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel parses a string level into a Level
func ParseLevel(level string) Level {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return LevelDebug
	case "INFO":
		return LevelInfo
	case "WARN", "WARNING":
		return LevelWarn
	case "ERROR":
		return LevelError
	case "FATAL":
		return LevelFatal
	default:
		return LevelInfo
	}
}

// Entry represents a log entry
type Entry struct {
	Time      time.Time              `json:"time"`
	Level     Level                  `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Caller    string                 `json:"caller,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
	Component string                 `json:"component,omitempty"`
}

// String returns a formatted string representation of the entry
func (e *Entry) String() string {
	timestamp := e.Time.Format("2006-01-02 15:04:05.000")
	
	var parts []string
	parts = append(parts, timestamp)
	parts = append(parts, fmt.Sprintf("[%s]", e.Level.String()))
	
	if e.Component != "" {
		parts = append(parts, fmt.Sprintf("(%s)", e.Component))
	}
	
	if e.RequestID != "" {
		parts = append(parts, fmt.Sprintf("[%s]", e.RequestID))
	}
	
	parts = append(parts, e.Message)
	
	if len(e.Fields) > 0 {
		var fields []string
		for k, v := range e.Fields {
			fields = append(fields, fmt.Sprintf("%s=%v", k, v))
		}
		parts = append(parts, fmt.Sprintf("{%s}", strings.Join(fields, ", ")))
	}
	
	if e.Caller != "" {
		parts = append(parts, fmt.Sprintf("@%s", e.Caller))
	}
	
	return strings.Join(parts, " ")
}

// JSON returns a JSON representation of the entry
func (e *Entry) JSON() ([]byte, error) {
	// Create a map for JSON serialization with string level
	jsonEntry := map[string]interface{}{
		"time":    e.Time.Format(time.RFC3339Nano),
		"level":   e.Level.String(),
		"message": e.Message,
	}
	
	if len(e.Fields) > 0 {
		jsonEntry["fields"] = e.Fields
	}
	
	if e.Caller != "" {
		jsonEntry["caller"] = e.Caller
	}
	
	if e.RequestID != "" {
		jsonEntry["request_id"] = e.RequestID
	}
	
	if e.Component != "" {
		jsonEntry["component"] = e.Component
	}
	
	return json.Marshal(jsonEntry)
}

// Logger provides structured logging functionality
type Logger struct {
	mu        sync.RWMutex
	level     Level
	output    io.Writer
	component string
	fields    map[string]interface{}
	formatter Formatter
	hooks     []Hook
}

// Formatter defines the interface for log formatters
type Formatter interface {
	Format(*Entry) ([]byte, error)
}

// TextFormatter formats logs as human-readable text
type TextFormatter struct {
	TimestampFormat string
	DisableColors   bool
	FullTimestamp   bool
}

// Format implements the Formatter interface for TextFormatter
func (f *TextFormatter) Format(entry *Entry) ([]byte, error) {
	output := entry.String()
	
	// Add color coding if enabled and output is a terminal
	if !f.DisableColors && isTerminal(os.Stdout) {
		output = f.colorize(entry.Level, output)
	}
	
	return []byte(output + "\n"), nil
}

func (f *TextFormatter) colorize(level Level, text string) string {
	var color string
	switch level {
	case LevelDebug:
		color = "\033[36m" // Cyan
	case LevelInfo:
		color = "\033[32m" // Green
	case LevelWarn:
		color = "\033[33m" // Yellow
	case LevelError:
		color = "\033[31m" // Red
	case LevelFatal:
		color = "\033[35m" // Magenta
	default:
		return text
	}
	
	return color + text + "\033[0m"
}

// JSONFormatter formats logs as JSON
type JSONFormatter struct{}

// Format implements the Formatter interface for JSONFormatter
func (f *JSONFormatter) Format(entry *Entry) ([]byte, error) {
	data, err := entry.JSON()
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// Hook defines the interface for log hooks
type Hook interface {
	Levels() []Level
	Fire(*Entry) error
}

// FileHook writes logs to a file with rotation support
type FileHook struct {
	filename    string
	maxSize     int64 // in bytes
	maxBackups  int
	maxAge      int // in days
	compress    bool
	file        *os.File
	currentSize int64
	mu          sync.Mutex
}

// NewFileHook creates a new file hook
func NewFileHook(filename string, maxSize int64, maxBackups int, maxAge int, compress bool) (*FileHook, error) {
	// Ensure directory exists
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %v", err)
	}
	
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}
	
	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat log file: %v", err)
	}
	
	return &FileHook{
		filename:    filename,
		maxSize:     maxSize,
		maxBackups:  maxBackups,
		maxAge:      maxAge,
		compress:    compress,
		file:        file,
		currentSize: stat.Size(),
	}, nil
}

// Levels returns the levels this hook handles
func (h *FileHook) Levels() []Level {
	return []Level{LevelDebug, LevelInfo, LevelWarn, LevelError, LevelFatal}
}

// Fire handles the log entry
func (h *FileHook) Fire(entry *Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	// Format the entry
	formatter := &JSONFormatter{} // Always use JSON for file output
	data, err := formatter.Format(entry)
	if err != nil {
		return err
	}
	
	// Check if rotation is needed
	if h.currentSize+int64(len(data)) > h.maxSize {
		if err := h.rotate(); err != nil {
			return err
		}
	}
	
	// Write to file
	n, err := h.file.Write(data)
	if err != nil {
		return err
	}
	
	h.currentSize += int64(n)
	return h.file.Sync()
}

func (h *FileHook) rotate() error {
	h.file.Close()
	
	// Rotate existing files
	for i := h.maxBackups - 1; i >= 0; i-- {
		source := fmt.Sprintf("%s.%d", h.filename, i)
		if i == 0 {
			source = h.filename
		}
		
		dest := fmt.Sprintf("%s.%d", h.filename, i+1)
		
		if _, err := os.Stat(source); err != nil {
			continue // Source doesn't exist, skip
		}
		
		if err := os.Rename(source, dest); err != nil {
			return err
		}
	}
	
	// Create new log file
	file, err := os.Create(h.filename)
	if err != nil {
		return err
	}
	
	h.file = file
	h.currentSize = 0
	
	return nil
}

// Close closes the file hook
func (h *FileHook) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if h.file != nil {
		return h.file.Close()
	}
	return nil
}

// NewLogger creates a new logger instance
func NewLogger(level Level, output io.Writer, component string) *Logger {
	return &Logger{
		level:     level,
		output:    output,
		component: component,
		fields:    make(map[string]interface{}),
		formatter: &TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05.000",
			DisableColors:   false,
			FullTimestamp:   true,
		},
	}
}

// SetLevel sets the log level
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// GetLevel returns the current log level
func (l *Logger) GetLevel() Level {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.level
}

// SetFormatter sets the log formatter
func (l *Logger) SetFormatter(formatter Formatter) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.formatter = formatter
}

// AddHook adds a hook to the logger
func (l *Logger) AddHook(hook Hook) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hooks = append(l.hooks, hook)
}

// WithField creates a new logger with an additional field
func (l *Logger) WithField(key string, value interface{}) *Logger {
	newLogger := l.clone()
	newLogger.fields[key] = value
	return newLogger
}

// WithFields creates a new logger with additional fields
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	newLogger := l.clone()
	for k, v := range fields {
		newLogger.fields[k] = v
	}
	return newLogger
}

// WithComponent creates a new logger with a specific component
func (l *Logger) WithComponent(component string) *Logger {
	newLogger := l.clone()
	newLogger.component = component
	return newLogger
}

// WithRequestID creates a new logger with a request ID
func (l *Logger) WithRequestID(requestID string) *Logger {
	return l.WithField("request_id", requestID)
}

func (l *Logger) clone() *Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	newFields := make(map[string]interface{})
	for k, v := range l.fields {
		newFields[k] = v
	}
	
	return &Logger{
		level:     l.level,
		output:    l.output,
		component: l.component,
		fields:    newFields,
		formatter: l.formatter,
		hooks:     l.hooks,
	}
}

// log writes a log entry
func (l *Logger) log(level Level, msg string, args ...interface{}) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	if level < l.level {
		return
	}
	
	// Format message
	message := msg
	if len(args) > 0 {
		message = fmt.Sprintf(msg, args...)
	}
	
	// Create entry
	entry := &Entry{
		Time:      time.Now(),
		Level:     level,
		Message:   message,
		Fields:    l.fields,
		Component: l.component,
	}
	
	// Add caller information
	if level >= LevelWarn {
		if pc, file, line, ok := runtime.Caller(2); ok {
			funcName := runtime.FuncForPC(pc).Name()
			entry.Caller = fmt.Sprintf("%s:%d (%s)", filepath.Base(file), line, filepath.Base(funcName))
		}
	}
	
	// Add request ID if present in fields
	if requestID, exists := l.fields["request_id"]; exists {
		if id, ok := requestID.(string); ok {
			entry.RequestID = id
		}
	}
	
	// Fire hooks
	for _, hook := range l.hooks {
		for _, hookLevel := range hook.Levels() {
			if hookLevel == level {
				if err := hook.Fire(entry); err != nil {
					// Log hook error to stderr to avoid infinite loops
					fmt.Fprintf(os.Stderr, "Hook error: %v\n", err)
				}
				break
			}
		}
	}
	
	// Format and write to output
	data, err := l.formatter.Format(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Formatting error: %v\n", err)
		return
	}
	
	l.output.Write(data)
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, args ...interface{}) {
	l.log(LevelDebug, msg, args...)
}

// Info logs an info message
func (l *Logger) Info(msg string, args ...interface{}) {
	l.log(LevelInfo, msg, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, args ...interface{}) {
	l.log(LevelWarn, msg, args...)
}

// Error logs an error message
func (l *Logger) Error(msg string, args ...interface{}) {
	l.log(LevelError, msg, args...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, args ...interface{}) {
	l.log(LevelFatal, msg, args...)
	os.Exit(1)
}

// Helper functions
func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return isTerminalFile(f)
	}
	return false
}

func isTerminalFile(f *os.File) bool {
	// This is a simplified check. In a real implementation,
	// you'd want to use the appropriate system calls to check
	// if the file descriptor is a terminal.
	return f == os.Stdout || f == os.Stderr
}

// Default logger instance
var defaultLogger *Logger
var once sync.Once

// Initialize creates the default logger
func Initialize(level Level, component string) {
	once.Do(func() {
		defaultLogger = NewLogger(level, os.Stdout, component)
	})
}

// SetGlobalLevel sets the level for the default logger
func SetGlobalLevel(level Level) {
	if defaultLogger == nil {
		Initialize(level, "rrunner")
	}
	defaultLogger.SetLevel(level)
}

// SetGlobalFormatter sets the formatter for the default logger
func SetGlobalFormatter(formatter Formatter) {
	if defaultLogger == nil {
		Initialize(LevelInfo, "rrunner")
	}
	defaultLogger.SetFormatter(formatter)
}

// AddGlobalHook adds a hook to the default logger
func AddGlobalHook(hook Hook) {
	if defaultLogger == nil {
		Initialize(LevelInfo, "rrunner")
	}
	defaultLogger.AddHook(hook)
}

// GetDefaultLogger returns the default logger
func GetDefaultLogger() *Logger {
	if defaultLogger == nil {
		Initialize(LevelInfo, "rrunner")
	}
	return defaultLogger
}

// Package-level logging functions using the default logger
func Debug(msg string, args ...interface{}) {
	GetDefaultLogger().Debug(msg, args...)
}

func Info(msg string, args ...interface{}) {
	GetDefaultLogger().Info(msg, args...)
}

func Warn(msg string, args ...interface{}) {
	GetDefaultLogger().Warn(msg, args...)
}

func Error(msg string, args ...interface{}) {
	GetDefaultLogger().Error(msg, args...)
}

func Fatal(msg string, args ...interface{}) {
	GetDefaultLogger().Fatal(msg, args...)
}

func WithField(key string, value interface{}) *Logger {
	return GetDefaultLogger().WithField(key, value)
}

func WithFields(fields map[string]interface{}) *Logger {
	return GetDefaultLogger().WithFields(fields)
}

func WithComponent(component string) *Logger {
	return GetDefaultLogger().WithComponent(component)
}

func WithRequestID(requestID string) *Logger {
	return GetDefaultLogger().WithRequestID(requestID)
}
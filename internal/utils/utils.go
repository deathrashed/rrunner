package utils

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ExpandPath expands ~ to home directory and resolves environment variables
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		if homeDir, err := os.UserHomeDir(); err == nil {
			return filepath.Join(homeDir, path[2:])
		}
	}
	return os.ExpandEnv(path)
}

// EnsureDir creates a directory if it doesn't exist
func EnsureDir(path string) error {
	expanded := ExpandPath(path)
	return os.MkdirAll(expanded, 0755)
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	expanded := ExpandPath(path)
	_, err := os.Stat(expanded)
	return !os.IsNotExist(err)
}

// IsExecutable checks if a file is executable
func IsExecutable(path string) bool {
	expanded := ExpandPath(path)
	stat, err := os.Stat(expanded)
	if err != nil {
		return false
	}
	return stat.Mode()&0111 != 0
}

// GenerateID generates a random ID
func GenerateID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// GenerateRequestID generates a request ID with timestamp
func GenerateRequestID() string {
	timestamp := time.Now().Format("20060102150405")
	id := GenerateID()
	return fmt.Sprintf("%s-%s", timestamp, id[:8])
}

// HashString generates an MD5 hash of a string
func HashString(s string) string {
	hasher := md5.New()
	hasher.Write([]byte(s))
	return hex.EncodeToString(hasher.Sum(nil))
}

// Contains checks if a slice contains a string
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ContainsAny checks if a slice contains any of the given items
func ContainsAny(slice []string, items []string) bool {
	for _, item := range items {
		if Contains(slice, item) {
			return true
		}
	}
	return false
}

// RemoveDuplicates removes duplicate strings from a slice
func RemoveDuplicates(slice []string) []string {
	seen := make(map[string]bool)
	result := []string{}
	
	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	
	return result
}

// SortedKeys returns sorted keys from a string map
func SortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// SortedStringKeys returns sorted keys from a string-to-string map
func SortedStringKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// FileURLToPath converts a file:// URL to a local path
func FileURLToPath(fileURL string) string {
	u, err := url.Parse(fileURL)
	if err == nil && u.Scheme == "file" {
		path, _ := url.PathUnescape(u.Path)
		return path
	}
	path, _ := url.QueryUnescape(fileURL)
	return path
}

// PathToFileURL converts a local path to a file:// URL
func PathToFileURL(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	return "file://" + absPath
}

// ValidateURL validates if a string is a valid URL
func ValidateURL(rawURL string) error {
	_, err := url.ParseRequestURI(rawURL)
	return err
}

// SanitizeFileName sanitizes a filename by removing invalid characters
func SanitizeFileName(filename string) string {
	// Replace invalid characters with underscores
	re := regexp.MustCompile(`[<>:"/\\|?*]`)
	sanitized := re.ReplaceAllString(filename, "_")
	
	// Remove leading/trailing dots and spaces
	sanitized = strings.Trim(sanitized, ". ")
	
	// Limit length
	if len(sanitized) > 200 {
		sanitized = sanitized[:200]
	}
	
	return sanitized
}

// GetUserInfo returns information about the current user
func GetUserInfo() (username, homeDir string, err error) {
	currentUser, err := user.Current()
	if err != nil {
		return "", "", err
	}
	return currentUser.Username, currentUser.HomeDir, nil
}

// GetSystemInfo returns system information
func GetSystemInfo() map[string]string {
	return map[string]string{
		"os":           runtime.GOOS,
		"arch":         runtime.GOARCH,
		"go_version":   runtime.Version(),
		"num_cpu":      strconv.Itoa(runtime.NumCPU()),
		"num_goroutine": strconv.Itoa(runtime.NumGoroutine()),
	}
}

// Retry executes a function with retry logic
func Retry(fn func() error, maxAttempts int, delay time.Duration) error {
	var err error
	for i := 0; i < maxAttempts; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		
		if i < maxAttempts-1 {
			time.Sleep(delay)
		}
	}
	return fmt.Errorf("failed after %d attempts: %v", maxAttempts, err)
}

// RetryWithBackoff executes a function with exponential backoff retry
func RetryWithBackoff(fn func() error, maxAttempts int, initialDelay time.Duration) error {
	var err error
	delay := initialDelay
	
	for i := 0; i < maxAttempts; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		
		if i < maxAttempts-1 {
			time.Sleep(delay)
			delay *= 2 // Exponential backoff
		}
	}
	return fmt.Errorf("failed after %d attempts: %v", maxAttempts, err)
}

// TimeoutContext creates a context with timeout
func TimeoutContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

// MapStringToInterface converts map[string]string to map[string]interface{}
func MapStringToInterface(input map[string]string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range input {
		result[k] = v
	}
	return result
}

// SliceStringToInterface converts []string to []interface{}
func SliceStringToInterface(input []string) []interface{} {
	result := make([]interface{}, len(input))
	for i, v := range input {
		result[i] = v
	}
	return result
}

// TruncateString truncates a string to the specified length
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// FormatBytes formats bytes as human-readable string
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatDuration formats duration as human-readable string
func FormatDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%dns", d.Nanoseconds())
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%.1fµs", float64(d.Nanoseconds())/1000)
	}
	if d < time.Second {
		return fmt.Sprintf("%.1fms", float64(d.Nanoseconds())/1000000)
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return d.String()
}

// Cache provides a simple in-memory cache with TTL
type Cache struct {
	mu    sync.RWMutex
	items map[string]*cacheItem
}

type cacheItem struct {
	value  interface{}
	expiry time.Time
}

// NewCache creates a new cache
func NewCache() *Cache {
	return &Cache{
		items: make(map[string]*cacheItem),
	}
}

// Set stores a value in the cache with TTL
func (c *Cache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.items[key] = &cacheItem{
		value:  value,
		expiry: time.Now().Add(ttl),
	}
}

// Get retrieves a value from the cache
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	item, exists := c.items[key]
	if !exists {
		return nil, false
	}
	
	if time.Now().After(item.expiry) {
		delete(c.items, key)
		return nil, false
	}
	
	return item.value, true
}

// Delete removes a value from the cache
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// Clear removes all items from the cache
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*cacheItem)
}

// Cleanup removes expired items from the cache
func (c *Cache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	now := time.Now()
	for key, item := range c.items {
		if now.After(item.expiry) {
			delete(c.items, key)
		}
	}
}

// Size returns the number of items in the cache
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// StartCleanupJob starts a background job to cleanup expired cache items
func (c *Cache) StartCleanupJob(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			c.Cleanup()
		}
	}()
}

// RateLimiter provides rate limiting functionality
type RateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// Allow checks if a request is allowed for the given key
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	windowStart := now.Add(-rl.window)
	
	// Clean up old requests
	requests := rl.requests[key]
	validRequests := []time.Time{}
	for _, reqTime := range requests {
		if reqTime.After(windowStart) {
			validRequests = append(validRequests, reqTime)
		}
	}
	
	// Check if limit is exceeded
	if len(validRequests) >= rl.limit {
		rl.requests[key] = validRequests
		return false
	}
	
	// Add current request
	validRequests = append(validRequests, now)
	rl.requests[key] = validRequests
	
	return true
}

// Reset resets the rate limiter for a specific key
func (rl *RateLimiter) Reset(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.requests, key)
}

// Cleanup removes old entries from the rate limiter
func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	windowStart := now.Add(-rl.window)
	
	for key, requests := range rl.requests {
		validRequests := []time.Time{}
		for _, reqTime := range requests {
			if reqTime.After(windowStart) {
				validRequests = append(validRequests, reqTime)
			}
		}
		
		if len(validRequests) == 0 {
			delete(rl.requests, key)
		} else {
			rl.requests[key] = validRequests
		}
	}
}

// Environment provides environment variable utilities
type Environment struct {
	vars map[string]string
}

// NewEnvironment creates a new environment
func NewEnvironment() *Environment {
	return &Environment{
		vars: make(map[string]string),
	}
}

// Set sets an environment variable
func (e *Environment) Set(key, value string) {
	e.vars[key] = value
}

// Get gets an environment variable with fallback
func (e *Environment) Get(key, fallback string) string {
	if value, exists := e.vars[key]; exists {
		return value
	}
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// GetBool gets a boolean environment variable
func (e *Environment) GetBool(key string, fallback bool) bool {
	value := e.Get(key, "")
	if value == "" {
		return fallback
	}
	
	switch strings.ToLower(value) {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return fallback
	}
}

// GetInt gets an integer environment variable
func (e *Environment) GetInt(key string, fallback int) int {
	value := e.Get(key, "")
	if value == "" {
		return fallback
	}
	
	if intValue, err := strconv.Atoi(value); err == nil {
		return intValue
	}
	
	return fallback
}

// GetDuration gets a duration environment variable
func (e *Environment) GetDuration(key string, fallback time.Duration) time.Duration {
	value := e.Get(key, "")
	if value == "" {
		return fallback
	}
	
	if duration, err := time.ParseDuration(value); err == nil {
		return duration
	}
	
	return fallback
}

// ToMap returns all environment variables as a map
func (e *Environment) ToMap() map[string]string {
	result := make(map[string]string)
	
	// Add OS environment variables
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	
	// Override with custom variables
	for key, value := range e.vars {
		result[key] = value
	}
	
	return result
}

// DefaultEnvironment returns a default environment with common variables
func DefaultEnvironment() *Environment {
	env := NewEnvironment()
	
	// Set common defaults
	env.Set("RRUNNER_VERSION", "0.2.0-go-core")
	env.Set("RRUNNER_LOG_LEVEL", "info")
	env.Set("RRUNNER_METRICS_ENABLED", "false")
	
	return env
}
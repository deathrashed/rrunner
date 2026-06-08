package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Counter represents a monotonically increasing counter
type Counter struct {
	value int64
}

// Inc increments the counter by 1
func (c *Counter) Inc() {
	atomic.AddInt64(&c.value, 1)
}

// Add increments the counter by the given amount
func (c *Counter) Add(delta int64) {
	atomic.AddInt64(&c.value, delta)
}

// Value returns the current value of the counter
func (c *Counter) Value() int64 {
	return atomic.LoadInt64(&c.value)
}

// Gauge represents a value that can go up or down
type Gauge struct {
	value int64
}

// Set sets the gauge to the given value
func (g *Gauge) Set(value int64) {
	atomic.StoreInt64(&g.value, value)
}

// Inc increments the gauge by 1
func (g *Gauge) Inc() {
	atomic.AddInt64(&g.value, 1)
}

// Dec decrements the gauge by 1
func (g *Gauge) Dec() {
	atomic.AddInt64(&g.value, -1)
}

// Add adds the given value to the gauge
func (g *Gauge) Add(delta int64) {
	atomic.AddInt64(&g.value, delta)
}

// Value returns the current value of the gauge
func (g *Gauge) Value() int64 {
	return atomic.LoadInt64(&g.value)
}

// Histogram tracks distribution of values
type Histogram struct {
	mu      sync.RWMutex
	buckets map[string]*Counter
	sum     int64
	count   int64
}

// NewHistogram creates a new histogram with predefined buckets
func NewHistogram() *Histogram {
	h := &Histogram{
		buckets: make(map[string]*Counter),
	}
	
	// Define default buckets (in milliseconds)
	buckets := []string{"1", "5", "10", "25", "50", "100", "250", "500", "1000", "2500", "5000", "10000", "+Inf"}
	for _, bucket := range buckets {
		h.buckets[bucket] = &Counter{}
	}
	
	return h
}

// Observe adds an observation to the histogram
func (h *Histogram) Observe(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	atomic.AddInt64(&h.count, 1)
	atomic.AddInt64(&h.sum, int64(value))
	
	// Increment appropriate buckets
	valueMs := int64(value * 1000) // Convert to milliseconds
	for bucketName, counter := range h.buckets {
		if bucketName == "+Inf" {
			counter.Inc()
			continue
		}
		
		bucketValue := parseBucketValue(bucketName)
		if valueMs <= bucketValue {
			counter.Inc()
		}
	}
}

// Count returns the total number of observations
func (h *Histogram) Count() int64 {
	return atomic.LoadInt64(&h.count)
}

// Sum returns the sum of all observations
func (h *Histogram) Sum() int64 {
	return atomic.LoadInt64(&h.sum)
}

// Buckets returns the current bucket counts
func (h *Histogram) Buckets() map[string]int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	result := make(map[string]int64)
	for name, counter := range h.buckets {
		result[name] = counter.Value()
	}
	return result
}

func parseBucketValue(bucket string) int64 {
	switch bucket {
	case "1":
		return 1
	case "5":
		return 5
	case "10":
		return 10
	case "25":
		return 25
	case "50":
		return 50
	case "100":
		return 100
	case "250":
		return 250
	case "500":
		return 500
	case "1000":
		return 1000
	case "2500":
		return 2500
	case "5000":
		return 5000
	case "10000":
		return 10000
	default:
		return 0
	}
}

// Metrics holds all application metrics
type Metrics struct {
	mu                  sync.RWMutex
	enabled             bool
	startTime           time.Time
	
	// Core metrics
	RequestsTotal       *Counter
	RequestsSuccess     *Counter
	RequestsFailure     *Counter
	RequestDuration     *Histogram
	PluginsLoaded       *Gauge
	PluginErrors        *Counter
	ActionsExecuted     *Counter
	
	// Action-specific metrics
	ActionCounters      map[string]*Counter
	ActionDurations     map[string]*Histogram
	ActionErrors        map[string]*Counter
	
	// System metrics
	CPUUsage            *Gauge
	MemoryUsage         *Gauge
	GoRoutines          *Gauge
	
	// Custom metrics
	CustomCounters      map[string]*Counter
	CustomGauges        map[string]*Gauge
	CustomHistograms    map[string]*Histogram
}

// NewMetrics creates a new metrics instance
func NewMetrics() *Metrics {
	return &Metrics{
		enabled:             true,
		startTime:           time.Now(),
		RequestsTotal:       &Counter{},
		RequestsSuccess:     &Counter{},
		RequestsFailure:     &Counter{},
		RequestDuration:     NewHistogram(),
		PluginsLoaded:       &Gauge{},
		PluginErrors:        &Counter{},
		ActionsExecuted:     &Counter{},
		ActionCounters:      make(map[string]*Counter),
		ActionDurations:     make(map[string]*Histogram),
		ActionErrors:        make(map[string]*Counter),
		CPUUsage:            &Gauge{},
		MemoryUsage:         &Gauge{},
		GoRoutines:          &Gauge{},
		CustomCounters:      make(map[string]*Counter),
		CustomGauges:        make(map[string]*Gauge),
		CustomHistograms:    make(map[string]*Histogram),
	}
}

// Enable enables metrics collection
func (m *Metrics) Enable() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = true
}

// Disable disables metrics collection
func (m *Metrics) Disable() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = false
}

// IsEnabled returns whether metrics collection is enabled
func (m *Metrics) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// RecordRequest records a request execution
func (m *Metrics) RecordRequest(duration time.Duration, success bool, action string) {
	if !m.IsEnabled() {
		return
	}
	
	m.RequestsTotal.Inc()
	m.RequestDuration.Observe(duration.Seconds())
	
	if success {
		m.RequestsSuccess.Inc()
	} else {
		m.RequestsFailure.Inc()
	}
	
	m.RecordAction(action, duration, success)
}

// RecordAction records action-specific metrics
func (m *Metrics) RecordAction(action string, duration time.Duration, success bool) {
	if !m.IsEnabled() {
		return
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Initialize action metrics if not present
	if _, exists := m.ActionCounters[action]; !exists {
		m.ActionCounters[action] = &Counter{}
		m.ActionDurations[action] = NewHistogram()
		m.ActionErrors[action] = &Counter{}
	}
	
	m.ActionsExecuted.Inc()
	m.ActionCounters[action].Inc()
	m.ActionDurations[action].Observe(duration.Seconds())
	
	if !success {
		m.ActionErrors[action].Inc()
	}
}

// RecordPluginLoaded records when a plugin is loaded
func (m *Metrics) RecordPluginLoaded() {
	if !m.IsEnabled() {
		return
	}
	m.PluginsLoaded.Inc()
}

// RecordPluginUnloaded records when a plugin is unloaded
func (m *Metrics) RecordPluginUnloaded() {
	if !m.IsEnabled() {
		return
	}
	m.PluginsLoaded.Dec()
}

// RecordPluginError records a plugin error
func (m *Metrics) RecordPluginError() {
	if !m.IsEnabled() {
		return
	}
	m.PluginErrors.Inc()
}

// GetCounter returns or creates a custom counter
func (m *Metrics) GetCounter(name string) *Counter {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if counter, exists := m.CustomCounters[name]; exists {
		return counter
	}
	
	counter := &Counter{}
	m.CustomCounters[name] = counter
	return counter
}

// GetGauge returns or creates a custom gauge
func (m *Metrics) GetGauge(name string) *Gauge {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if gauge, exists := m.CustomGauges[name]; exists {
		return gauge
	}
	
	gauge := &Gauge{}
	m.CustomGauges[name] = gauge
	return gauge
}

// GetHistogram returns or creates a custom histogram
func (m *Metrics) GetHistogram(name string) *Histogram {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if histogram, exists := m.CustomHistograms[name]; exists {
		return histogram
	}
	
	histogram := NewHistogram()
	m.CustomHistograms[name] = histogram
	return histogram
}

// MetricValue represents a metric value for export
type MetricValue struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Value       interface{}       `json:"value"`
	Labels      map[string]string `json:"labels,omitempty"`
	Description string            `json:"description,omitempty"`
}

// Export exports all metrics as a structured format
func (m *Metrics) Export() []MetricValue {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var metrics []MetricValue
	
	// Core metrics
	metrics = append(metrics, []MetricValue{
		{Name: "rrunner_requests_total", Type: "counter", Value: m.RequestsTotal.Value(), Description: "Total number of requests"},
		{Name: "rrunner_requests_success", Type: "counter", Value: m.RequestsSuccess.Value(), Description: "Number of successful requests"},
		{Name: "rrunner_requests_failure", Type: "counter", Value: m.RequestsFailure.Value(), Description: "Number of failed requests"},
		{Name: "rrunner_plugins_loaded", Type: "gauge", Value: m.PluginsLoaded.Value(), Description: "Number of loaded plugins"},
		{Name: "rrunner_plugin_errors", Type: "counter", Value: m.PluginErrors.Value(), Description: "Number of plugin errors"},
		{Name: "rrunner_actions_executed", Type: "counter", Value: m.ActionsExecuted.Value(), Description: "Total number of actions executed"},
		{Name: "rrunner_uptime_seconds", Type: "counter", Value: time.Since(m.startTime).Seconds(), Description: "Uptime in seconds"},
	}...)
	
	// Request duration histogram
	metrics = append(metrics, MetricValue{
		Name:        "rrunner_request_duration_seconds",
		Type:        "histogram",
		Value:       m.RequestDuration.Buckets(),
		Description: "Request duration distribution",
	})
	
	// Action-specific metrics
	for action, counter := range m.ActionCounters {
		metrics = append(metrics, MetricValue{
			Name:        "rrunner_action_total",
			Type:        "counter",
			Value:       counter.Value(),
			Labels:      map[string]string{"action": action},
			Description: "Number of executions per action",
		})
		
		if histogram, exists := m.ActionDurations[action]; exists {
			metrics = append(metrics, MetricValue{
				Name:        "rrunner_action_duration_seconds",
				Type:        "histogram",
				Value:       histogram.Buckets(),
				Labels:      map[string]string{"action": action},
				Description: "Action duration distribution",
			})
		}
		
		if errorCounter, exists := m.ActionErrors[action]; exists {
			metrics = append(metrics, MetricValue{
				Name:        "rrunner_action_errors_total",
				Type:        "counter",
				Value:       errorCounter.Value(),
				Labels:      map[string]string{"action": action},
				Description: "Number of errors per action",
			})
		}
	}
	
	// Custom counters
	for name, counter := range m.CustomCounters {
		metrics = append(metrics, MetricValue{
			Name:  fmt.Sprintf("rrunner_custom_%s", name),
			Type:  "counter",
			Value: counter.Value(),
		})
	}
	
	// Custom gauges
	for name, gauge := range m.CustomGauges {
		metrics = append(metrics, MetricValue{
			Name:  fmt.Sprintf("rrunner_custom_%s", name),
			Type:  "gauge",
			Value: gauge.Value(),
		})
	}
	
	// Custom histograms
	for name, histogram := range m.CustomHistograms {
		metrics = append(metrics, MetricValue{
			Name:  fmt.Sprintf("rrunner_custom_%s", name),
			Type:  "histogram",
			Value: histogram.Buckets(),
		})
	}
	
	// Sort metrics by name for consistent output
	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].Name < metrics[j].Name
	})
	
	return metrics
}

// ExportJSON exports metrics as JSON
func (m *Metrics) ExportJSON() ([]byte, error) {
	metrics := m.Export()
	return json.MarshalIndent(metrics, "", "  ")
}

// ExportPrometheus exports metrics in Prometheus format
func (m *Metrics) ExportPrometheus() string {
	metrics := m.Export()
	var output []string
	
	for _, metric := range metrics {
		// Add description
		if metric.Description != "" {
			output = append(output, fmt.Sprintf("# HELP %s %s", metric.Name, metric.Description))
		}
		output = append(output, fmt.Sprintf("# TYPE %s %s", metric.Name, metric.Type))
		
		switch metric.Type {
		case "counter", "gauge":
			if len(metric.Labels) > 0 {
				labels := formatPrometheusLabels(metric.Labels)
				output = append(output, fmt.Sprintf("%s{%s} %v", metric.Name, labels, metric.Value))
			} else {
				output = append(output, fmt.Sprintf("%s %v", metric.Name, metric.Value))
			}
			
		case "histogram":
			if buckets, ok := metric.Value.(map[string]int64); ok {
				labels := formatPrometheusLabels(metric.Labels)
				if labels != "" {
					labels = "," + labels
				}
				
				for bucket, count := range buckets {
					output = append(output, fmt.Sprintf("%s_bucket{le=\"%s\"%s} %d", metric.Name, bucket, labels, count))
				}
			}
		}
	}
	
	return fmt.Sprintf("%s\n", fmt.Sprintf("%s", output))
}

func formatPrometheusLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	
	var pairs []string
	for k, v := range labels {
		pairs = append(pairs, fmt.Sprintf("%s=\"%s\"", k, v))
	}
	
	sort.Strings(pairs)
	return fmt.Sprintf("%s", pairs)
}

// Server provides an HTTP metrics endpoint
type Server struct {
	metrics *Metrics
	server  *http.Server
	mux     *http.ServeMux
}

// NewServer creates a new metrics server
func NewServer(metrics *Metrics, port int) *Server {
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
	
	s := &Server{
		metrics: metrics,
		server:  server,
		mux:     mux,
	}
	
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.mux.HandleFunc("/metrics", s.handleMetrics)
	s.mux.HandleFunc("/metrics/json", s.handleMetricsJSON)
	s.mux.HandleFunc("/health", s.handleHealth)
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(s.metrics.ExportPrometheus()))
}

func (s *Server) handleMetricsJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	data, err := s.metrics.ExportJSON()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error exporting metrics: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.Write(data)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	health := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().UTC(),
		"uptime":    time.Since(s.metrics.startTime).String(),
		"version":   "0.2.0-go-core",
	}
	
	json.NewEncoder(w).Encode(health)
}

// Start starts the metrics server
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Stop stops the metrics server gracefully
func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// GlobalMetrics holds the global metrics instance
var GlobalMetrics *Metrics

// Initialize initializes the global metrics instance
func Initialize() {
	GlobalMetrics = NewMetrics()
}

// GetGlobalMetrics returns the global metrics instance
func GetGlobalMetrics() *Metrics {
	if GlobalMetrics == nil {
		Initialize()
	}
	return GlobalMetrics
}
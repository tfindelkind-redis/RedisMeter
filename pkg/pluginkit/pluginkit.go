// Package pluginkit provides utilities for developing RedisMeter plugins.
//
// This package simplifies the creation of custom storage backends, analyzers,
// workloads, and exporters. It provides:
//
//   - Base implementations with common functionality
//   - Testing utilities for plugin validation
//   - Configuration helpers
//   - Lifecycle management
//
// # Creating a Storage Plugin
//
//	type MyStorage struct {
//	    pluginkit.BaseStoragePlugin
//	    db *sql.DB
//	}
//
//	func (s *MyStorage) Init(cfg map[string]interface{}) error {
//	    dsn := pluginkit.GetString(cfg, "dsn", "")
//	    var err error
//	    s.db, err = sql.Open("postgres", dsn)
//	    return err
//	}
//
// # Creating an Analyzer Plugin
//
//	type MyAnalyzer struct {
//	    pluginkit.BaseAnalyzerPlugin
//	}
//
//	func (a *MyAnalyzer) Analyze(ctx context.Context, run *domain.BenchmarkRun) (*plugin.AnalysisResult, error) {
//	    // Custom analysis logic
//	    return &plugin.AnalysisResult{
//	        Score: 0.85,
//	        Findings: []plugin.Finding{{
//	            Type:     "custom",
//	            Severity: "info",
//	            Message:  "Custom analysis complete",
//	        }},
//	    }, nil
//	}
package pluginkit

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ==================== Configuration Helpers ====================

// GetString retrieves a string value from config with a default.
func GetString(cfg map[string]interface{}, key, defaultVal string) string {
	if v, ok := cfg[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultVal
}

// GetInt retrieves an integer value from config with a default.
func GetInt(cfg map[string]interface{}, key string, defaultVal int) int {
	if v, ok := cfg[key]; ok {
		switch i := v.(type) {
		case int:
			return i
		case int64:
			return int(i)
		case float64:
			return int(i)
		}
	}
	return defaultVal
}

// GetFloat retrieves a float value from config with a default.
func GetFloat(cfg map[string]interface{}, key string, defaultVal float64) float64 {
	if v, ok := cfg[key]; ok {
		switch f := v.(type) {
		case float64:
			return f
		case int:
			return float64(f)
		case int64:
			return float64(f)
		}
	}
	return defaultVal
}

// GetBool retrieves a boolean value from config with a default.
func GetBool(cfg map[string]interface{}, key string, defaultVal bool) bool {
	if v, ok := cfg[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return defaultVal
}

// GetDuration retrieves a duration value from config with a default.
func GetDuration(cfg map[string]interface{}, key string, defaultVal time.Duration) time.Duration {
	if v, ok := cfg[key]; ok {
		switch d := v.(type) {
		case string:
			if parsed, err := time.ParseDuration(d); err == nil {
				return parsed
			}
		case time.Duration:
			return d
		case int64:
			return time.Duration(d)
		case float64:
			return time.Duration(d)
		}
	}
	return defaultVal
}

// GetStringSlice retrieves a string slice from config.
func GetStringSlice(cfg map[string]interface{}, key string) []string {
	if v, ok := cfg[key]; ok {
		switch s := v.(type) {
		case []string:
			return s
		case []interface{}:
			result := make([]string, 0, len(s))
			for _, item := range s {
				if str, ok := item.(string); ok {
					result = append(result, str)
				}
			}
			return result
		}
	}
	return nil
}

// GetMap retrieves a nested map from config.
func GetMap(cfg map[string]interface{}, key string) map[string]interface{} {
	if v, ok := cfg[key]; ok {
		if m, ok := v.(map[string]interface{}); ok {
			return m
		}
	}
	return nil
}

// ==================== Base Plugins ====================

// BasePlugin provides common plugin functionality.
type BasePlugin struct {
	name     string
	version  string
	config   map[string]interface{}
	mu       sync.RWMutex
	started  bool
	metadata map[string]string
}

// Name returns the plugin name.
func (p *BasePlugin) Name() string {
	return p.name
}

// Version returns the plugin version.
func (p *BasePlugin) Version() string {
	return p.version
}

// SetInfo sets the plugin name and version.
func (p *BasePlugin) SetInfo(name, version string) {
	p.name = name
	p.version = version
}

// Config returns the plugin configuration.
func (p *BasePlugin) Config() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config
}

// SetConfig sets the plugin configuration.
func (p *BasePlugin) SetConfig(cfg map[string]interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config = cfg
}

// IsStarted returns whether the plugin has been started.
func (p *BasePlugin) IsStarted() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.started
}

// MarkStarted marks the plugin as started.
func (p *BasePlugin) MarkStarted() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.started = true
}

// MarkStopped marks the plugin as stopped.
func (p *BasePlugin) MarkStopped() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.started = false
}

// SetMetadata sets plugin metadata.
func (p *BasePlugin) SetMetadata(key, value string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.metadata == nil {
		p.metadata = make(map[string]string)
	}
	p.metadata[key] = value
}

// GetMetadata retrieves plugin metadata.
func (p *BasePlugin) GetMetadata(key string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.metadata == nil {
		return ""
	}
	return p.metadata[key]
}

// BaseStoragePlugin provides a foundation for storage plugins.
type BaseStoragePlugin struct {
	BasePlugin
}

// Type returns "storage".
func (p *BaseStoragePlugin) Type() string {
	return "storage"
}

// Init is a no-op that can be overridden.
func (p *BaseStoragePlugin) Init(cfg map[string]interface{}) error {
	p.SetConfig(cfg)
	p.MarkStarted()
	return nil
}

// Close is a no-op that can be overridden.
func (p *BaseStoragePlugin) Close() error {
	p.MarkStopped()
	return nil
}

// BaseAnalyzerPlugin provides a foundation for analyzer plugins.
type BaseAnalyzerPlugin struct {
	BasePlugin
}

// Type returns "analyzer".
func (p *BaseAnalyzerPlugin) Type() string {
	return "analyzer"
}

// Init is a no-op that can be overridden.
func (p *BaseAnalyzerPlugin) Init(cfg map[string]interface{}) error {
	p.SetConfig(cfg)
	p.MarkStarted()
	return nil
}

// Close is a no-op that can be overridden.
func (p *BaseAnalyzerPlugin) Close() error {
	p.MarkStopped()
	return nil
}

// BaseExporterPlugin provides a foundation for exporter plugins.
type BaseExporterPlugin struct {
	BasePlugin
}

// Type returns "exporter".
func (p *BaseExporterPlugin) Type() string {
	return "exporter"
}

// Init is a no-op that can be overridden.
func (p *BaseExporterPlugin) Init(cfg map[string]interface{}) error {
	p.SetConfig(cfg)
	p.MarkStarted()
	return nil
}

// Close is a no-op that can be overridden.
func (p *BaseExporterPlugin) Close() error {
	p.MarkStopped()
	return nil
}

// BaseWorkloadPlugin provides a foundation for workload plugins.
type BaseWorkloadPlugin struct {
	BasePlugin
}

// Type returns "workload".
func (p *BaseWorkloadPlugin) Type() string {
	return "workload"
}

// Init is a no-op that can be overridden.
func (p *BaseWorkloadPlugin) Init(cfg map[string]interface{}) error {
	p.SetConfig(cfg)
	p.MarkStarted()
	return nil
}

// Close is a no-op that can be overridden.
func (p *BaseWorkloadPlugin) Close() error {
	p.MarkStopped()
	return nil
}

// ==================== Lifecycle Manager ====================

// LifecycleManager manages plugin lifecycle.
type LifecycleManager struct {
	plugins []interface{ Close() error }
	mu      sync.Mutex
}

// NewLifecycleManager creates a new lifecycle manager.
func NewLifecycleManager() *LifecycleManager {
	return &LifecycleManager{
		plugins: make([]interface{ Close() error }, 0),
	}
}

// Register adds a plugin to be managed.
func (m *LifecycleManager) Register(p interface{ Close() error }) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.plugins = append(m.plugins, p)
}

// CloseAll closes all registered plugins in reverse order.
func (m *LifecycleManager) CloseAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for i := len(m.plugins) - 1; i >= 0; i-- {
		if err := m.plugins[i].Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing plugins: %v", errs)
	}
	return nil
}

// ==================== Health Checker ====================

// HealthStatus represents health check status.
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// HealthCheck is implemented by plugins that support health checks.
type HealthCheck interface {
	HealthCheck(ctx context.Context) (HealthStatus, error)
}

// HealthChecker performs health checks on plugins.
type HealthChecker struct {
	plugins map[string]HealthCheck
	mu      sync.RWMutex
}

// NewHealthChecker creates a new health checker.
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		plugins: make(map[string]HealthCheck),
	}
}

// Register adds a plugin for health checking.
func (h *HealthChecker) Register(name string, p HealthCheck) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.plugins[name] = p
}

// CheckAll performs health checks on all registered plugins.
func (h *HealthChecker) CheckAll(ctx context.Context) map[string]HealthStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()

	results := make(map[string]HealthStatus)
	for name, p := range h.plugins {
		status, err := p.HealthCheck(ctx)
		if err != nil {
			results[name] = HealthStatusUnhealthy
		} else {
			results[name] = status
		}
	}
	return results
}

// ==================== Metrics Collector ====================

// MetricsCollector collects plugin metrics.
type MetricsCollector struct {
	counters map[string]int64
	gauges   map[string]float64
	mu       sync.RWMutex
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		counters: make(map[string]int64),
		gauges:   make(map[string]float64),
	}
}

// IncrCounter increments a counter.
func (m *MetricsCollector) IncrCounter(name string, delta int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[name] += delta
}

// SetGauge sets a gauge value.
func (m *MetricsCollector) SetGauge(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gauges[name] = value
}

// GetCounter returns a counter value.
func (m *MetricsCollector) GetCounter(name string) int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.counters[name]
}

// GetGauge returns a gauge value.
func (m *MetricsCollector) GetGauge(name string) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.gauges[name]
}

// Snapshot returns a snapshot of all metrics.
func (m *MetricsCollector) Snapshot() (map[string]int64, map[string]float64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	counters := make(map[string]int64, len(m.counters))
	for k, v := range m.counters {
		counters[k] = v
	}

	gauges := make(map[string]float64, len(m.gauges))
	for k, v := range m.gauges {
		gauges[k] = v
	}

	return counters, gauges
}

// ==================== Event Emitter ====================

// Event represents a plugin event.
type Event struct {
	Type      string
	Source    string
	Timestamp time.Time
	Data      map[string]interface{}
}

// EventHandler handles plugin events.
type EventHandler func(Event)

// EventEmitter provides event emission for plugins.
type EventEmitter struct {
	handlers map[string][]EventHandler
	mu       sync.RWMutex
}

// NewEventEmitter creates a new event emitter.
func NewEventEmitter() *EventEmitter {
	return &EventEmitter{
		handlers: make(map[string][]EventHandler),
	}
}

// On registers an event handler.
func (e *EventEmitter) On(eventType string, handler EventHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.handlers[eventType] = append(e.handlers[eventType], handler)
}

// Emit emits an event to all registered handlers.
func (e *EventEmitter) Emit(event Event) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if handlers, ok := e.handlers[event.Type]; ok {
		for _, h := range handlers {
			go h(event)
		}
	}

	// Also emit to wildcard handlers
	if handlers, ok := e.handlers["*"]; ok {
		for _, h := range handlers {
			go h(event)
		}
	}
}

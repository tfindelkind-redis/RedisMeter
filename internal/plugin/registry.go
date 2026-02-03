// Package plugin provides the plugin framework for RedisMeter.
// It defines interfaces and registry for extensible components.
package plugin

import (
	"context"
	"fmt"
	"sync"
)

// Type represents the category of a plugin.
type Type string

const (
	TypeStorage   Type = "storage"
	TypeExecutor  Type = "executor"
	TypeExporter  Type = "exporter"
	TypeWorkload  Type = "workload"
	TypeAnalyzer  Type = "analyzer"
	TypeReporter  Type = "reporter"
	TypeNotifier  Type = "notifier"
	TypeCloud     Type = "cloud"
	TypeEnvironment Type = "environment"
)

// Metadata contains information about a plugin.
type Metadata struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Type        Type     `json:"type"`
	Description string   `json:"description"`
	Author      string   `json:"author,omitempty"`
	License     string   `json:"license,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
}

// HealthStatus represents the health state of a plugin.
type HealthStatus struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message,omitempty"`
}

// Plugin is the base interface that all plugins must implement.
type Plugin interface {
	// Metadata returns information about the plugin.
	Metadata() Metadata

	// Initialize sets up the plugin with the provided configuration.
	Initialize(ctx context.Context, config map[string]interface{}) error

	// HealthCheck returns the current health status of the plugin.
	HealthCheck(ctx context.Context) HealthStatus

	// Shutdown gracefully stops the plugin.
	Shutdown(ctx context.Context) error
}

// Registry manages plugin registration and discovery.
type Registry struct {
	mu      sync.RWMutex
	plugins map[Type]map[string]Plugin
}

// NewRegistry creates a new plugin registry.
func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[Type]map[string]Plugin),
	}
}

// Register adds a plugin to the registry.
func (r *Registry) Register(p Plugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	meta := p.Metadata()

	if r.plugins[meta.Type] == nil {
		r.plugins[meta.Type] = make(map[string]Plugin)
	}

	if _, exists := r.plugins[meta.Type][meta.Name]; exists {
		return fmt.Errorf("plugin %s of type %s already registered", meta.Name, meta.Type)
	}

	r.plugins[meta.Type][meta.Name] = p
	return nil
}

// Get retrieves a plugin by type and name.
func (r *Registry) Get(t Type, name string) (Plugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.plugins[t] == nil {
		return nil, fmt.Errorf("no plugins of type %s registered", t)
	}

	p, exists := r.plugins[t][name]
	if !exists {
		return nil, fmt.Errorf("plugin %s of type %s not found", name, t)
	}

	return p, nil
}

// List returns all plugins of a given type.
func (r *Registry) List(t Type) []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Plugin
	for _, p := range r.plugins[t] {
		result = append(result, p)
	}
	return result
}

// ListAll returns all registered plugins.
func (r *Registry) ListAll() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Plugin
	for _, byType := range r.plugins {
		for _, p := range byType {
			result = append(result, p)
		}
	}
	return result
}

// DefaultRegistry is the global plugin registry.
var DefaultRegistry = NewRegistry()

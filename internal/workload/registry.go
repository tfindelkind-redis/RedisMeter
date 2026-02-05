// Package workload provides built-in workload definitions.
package workload

import (
	"fmt"
	"os"
	"sync"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
	"go.yaml.in/yaml/v3"
)

// Registry manages available workloads.
type Registry struct {
	mu        sync.RWMutex
	workloads map[string]*domain.Workload
}

// DefaultRegistry is the global workload registry.
var DefaultRegistry = NewRegistry()

// NewRegistry creates a new workload registry.
func NewRegistry() *Registry {
	r := &Registry{
		workloads: make(map[string]*domain.Workload),
	}
	r.registerBuiltins()
	return r
}

// registerBuiltins adds built-in workload definitions.
func (r *Registry) registerBuiltins() {
	// Cache workload - balanced GET/SET for caching use cases
	r.Register(&domain.Workload{
		Name:        "cache",
		Description: "Balanced GET/SET workload simulating a typical cache usage pattern",
		Type:        "cache",
		Operations: []domain.Operation{
			{Command: "GET", Ratio: 0.8},
			{Command: "SET", Ratio: 0.2},
		},
		KeyPattern: &domain.KeyPattern{
			Prefix:   "cache:",
			Pattern:  "random",
			KeyRange: 1000000,
		},
		DataSize: &domain.DataSize{
			Fixed: 256,
		},
		Threads:  4,
		Clients:  50,
		Duration: "30s",
		Pipeline: 1,
	})

	// Write-heavy workload
	r.Register(&domain.Workload{
		Name:        "write-heavy",
		Description: "Write-heavy workload for testing write performance",
		Type:        "write",
		Operations: []domain.Operation{
			{Command: "GET", Ratio: 0.2},
			{Command: "SET", Ratio: 0.8},
		},
		KeyPattern: &domain.KeyPattern{
			Prefix:   "write:",
			Pattern:  "random",
			KeyRange: 1000000,
		},
		DataSize: &domain.DataSize{
			Fixed: 256,
		},
		Threads:  4,
		Clients:  50,
		Duration: "30s",
		Pipeline: 1,
	})

	// Read-only workload
	r.Register(&domain.Workload{
		Name:        "read-only",
		Description: "Read-only workload for testing read performance (requires pre-populated data)",
		Type:        "read",
		Operations: []domain.Operation{
			{Command: "GET", Ratio: 1.0},
		},
		KeyPattern: &domain.KeyPattern{
			Prefix:   "read:",
			Pattern:  "random",
			KeyRange: 1000000,
		},
		DataSize: &domain.DataSize{
			Fixed: 256,
		},
		Threads:  4,
		Clients:  50,
		Duration: "30s",
		Pipeline: 1,
	})

	// Mixed workload with various operations
	r.Register(&domain.Workload{
		Name:        "mixed",
		Description: "Mixed workload with GET, SET, and various data sizes",
		Type:        "mixed",
		Operations: []domain.Operation{
			{Command: "GET", Ratio: 0.5},
			{Command: "SET", Ratio: 0.5},
		},
		KeyPattern: &domain.KeyPattern{
			Prefix:   "mixed:",
			Pattern:  "random",
			KeyRange: 500000,
		},
		DataSize: &domain.DataSize{
			Min: 64,
			Max: 1024,
		},
		Threads:  4,
		Clients:  50,
		Duration: "30s",
		Pipeline: 1,
	})

	// High throughput workload with pipelining
	r.Register(&domain.Workload{
		Name:        "high-throughput",
		Description: "High throughput workload using pipelining for maximum ops/sec",
		Type:        "throughput",
		Operations: []domain.Operation{
			{Command: "GET", Ratio: 0.8},
			{Command: "SET", Ratio: 0.2},
		},
		KeyPattern: &domain.KeyPattern{
			Prefix:   "ht:",
			Pattern:  "random",
			KeyRange: 1000000,
		},
		DataSize: &domain.DataSize{
			Fixed: 100,
		},
		Threads:  4,
		Clients:  100,
		Duration: "30s",
		Pipeline: 10,
	})

	// Latency-sensitive workload (no pipelining, fewer clients)
	r.Register(&domain.Workload{
		Name:        "low-latency",
		Description: "Latency-sensitive workload optimized for minimal response time",
		Type:        "latency",
		Operations: []domain.Operation{
			{Command: "GET", Ratio: 0.9},
			{Command: "SET", Ratio: 0.1},
		},
		KeyPattern: &domain.KeyPattern{
			Prefix:   "ll:",
			Pattern:  "random",
			KeyRange: 100000,
		},
		DataSize: &domain.DataSize{
			Fixed: 64,
		},
		Threads:  2,
		Clients:  10,
		Duration: "30s",
		Pipeline: 1,
	})

	// Session store workload
	r.Register(&domain.Workload{
		Name:        "session",
		Description: "Session store workload simulating web session management",
		Type:        "session",
		Operations: []domain.Operation{
			{Command: "GET", Ratio: 0.7},
			{Command: "SET", Ratio: 0.3},
		},
		KeyPattern: &domain.KeyPattern{
			Prefix:   "session:",
			Pattern:  "random",
			KeyRange: 100000,
		},
		DataSize: &domain.DataSize{
			Fixed: 512,
		},
		Threads:  4,
		Clients:  50,
		Duration: "30s",
		Pipeline: 1,
	})

	// Large value workload
	r.Register(&domain.Workload{
		Name:        "large-values",
		Description: "Workload with larger values to test bandwidth and serialization",
		Type:        "large",
		Operations: []domain.Operation{
			{Command: "GET", Ratio: 0.5},
			{Command: "SET", Ratio: 0.5},
		},
		KeyPattern: &domain.KeyPattern{
			Prefix:   "large:",
			Pattern:  "random",
			KeyRange: 10000,
		},
		DataSize: &domain.DataSize{
			Min: 4096,
			Max: 16384,
		},
		Threads:  4,
		Clients:  20,
		Duration: "30s",
		Pipeline: 1,
	})

	// Huge read workload - 100% GET with 100KB values
	r.Register(&domain.Workload{
		Name:        "huge-read",
		Description: "100% GET workload with 100KB values for bandwidth/throughput testing",
		Type:        "bandwidth",
		Operations: []domain.Operation{
			{Command: "GET", Ratio: 1.0},
		},
		KeyPattern: &domain.KeyPattern{
			Prefix:   "huge:",
			Pattern:  "random",
			KeyRange: 10000,
		},
		DataSize: &domain.DataSize{
			Fixed: 102400, // 100KB
		},
		Threads:  4,
		Clients:  20,
		Duration: "30s",
		Pipeline: 1,
	})
}

// Register adds a workload to the registry.
func (r *Registry) Register(w *domain.Workload) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.workloads[w.Name]; exists {
		return fmt.Errorf("workload %q already registered", w.Name)
	}

	r.workloads[w.Name] = w
	return nil
}

// Get retrieves a workload by name.
func (r *Registry) Get(name string) (*domain.Workload, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	w, exists := r.workloads[name]
	if !exists {
		return nil, fmt.Errorf("workload %q not found", name)
	}

	// Return a copy to prevent modification
	copy := *w
	return &copy, nil
}

// List returns all registered workload names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.workloads))
	for name := range r.workloads {
		names = append(names, name)
	}
	return names
}

// ListAll returns all registered workloads.
func (r *Registry) ListAll() []*domain.Workload {
	r.mu.RLock()
	defer r.mu.RUnlock()

	workloads := make([]*domain.Workload, 0, len(r.workloads))
	for _, w := range r.workloads {
		copy := *w
		workloads = append(workloads, &copy)
	}
	return workloads
}

// LoadFromFile loads a workload definition from a YAML file.
func LoadFromFile(path string) (*domain.Workload, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read workload file: %w", err)
	}

	var w domain.Workload
	if err := yaml.Unmarshal(data, &w); err != nil {
		return nil, fmt.Errorf("failed to parse workload YAML: %w", err)
	}

	if w.Name == "" {
		return nil, fmt.Errorf("workload must have a name")
	}

	return &w, nil
}

// SaveToFile saves a workload definition to a YAML file.
func SaveToFile(w *domain.Workload, path string) error {
	data, err := yaml.Marshal(w)
	if err != nil {
		return fmt.Errorf("failed to marshal workload: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write workload file: %w", err)
	}

	return nil
}

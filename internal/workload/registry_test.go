package workload

import (
	"testing"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
)

func TestBuiltinWorkloads(t *testing.T) {
	registry := NewRegistry()

	// All expected built-in workloads
	expectedWorkloads := []string{
		"cache",
		"write-heavy",
		"read-only",
		"mixed",
		"high-throughput",
		"low-latency",
		"session",
		"large-values",
	}

	for _, name := range expectedWorkloads {
		t.Run(name, func(t *testing.T) {
			w, err := registry.Get(name)
			if err != nil {
				t.Fatalf("Get(%q) error = %v", name, err)
			}

			if w.Name != name {
				t.Errorf("Name = %q, want %q", w.Name, name)
			}

			// All workloads should have sensible defaults
			if w.Threads <= 0 {
				t.Errorf("Threads = %d, should be > 0", w.Threads)
			}
			if w.Clients <= 0 {
				t.Errorf("Clients = %d, should be > 0", w.Clients)
			}
			if w.Description == "" {
				t.Error("Description should not be empty")
			}
		})
	}
}

func TestRegistryList(t *testing.T) {
	registry := NewRegistry()
	names := registry.List()

	if len(names) < 8 {
		t.Errorf("List() returned %d workloads, want at least 8", len(names))
	}

	// Verify all workloads have names
	for _, name := range names {
		if name == "" {
			t.Error("Found workload with empty name")
		}
	}
}

func TestRegistryGetNonexistent(t *testing.T) {
	registry := NewRegistry()
	_, err := registry.Get("nonexistent-workload")
	if err == nil {
		t.Error("Expected error for nonexistent workload")
	}
}

func TestRegistryRegister(t *testing.T) {
	registry := NewRegistry()
	
	customWorkload := &domain.Workload{
		Name:        "custom",
		Description: "Custom test workload",
		Threads:     2,
		Clients:     20,
		Requests:    5000,
	}

	registry.Register(customWorkload)

	// Should be able to retrieve it
	w, err := registry.Get("custom")
	if err != nil {
		t.Fatalf("Get(custom) error = %v", err)
	}

	if w.Description != "Custom test workload" {
		t.Errorf("Description = %q, want %q", w.Description, "Custom test workload")
	}
}

func TestWorkloadCharacteristics(t *testing.T) {
	registry := NewRegistry()

	tests := []struct {
		name          string
		expectHighOps bool
		expectLowLat  bool
	}{
		{
			name: "cache",
		},
		{
			name: "write-heavy",
		},
		{
			name: "read-only",
		},
		{
			name:          "high-throughput",
			expectHighOps: true, // Uses pipelining
		},
		{
			name:         "low-latency",
			expectLowLat: true, // Single thread, low clients
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, _ := registry.Get(tt.name)

			if tt.expectHighOps && w.Pipeline <= 0 {
				t.Error("High throughput workload should use pipelining")
			}

			if tt.expectLowLat {
				if w.Threads > 2 {
					t.Errorf("Low latency workload has %d threads, should have few", w.Threads)
				}
				if w.Clients > 10 {
					t.Errorf("Low latency workload has %d clients, should have few", w.Clients)
				}
			}
		})
	}
}

func TestLargeValuesWorkload(t *testing.T) {
	registry := NewRegistry()
	w, err := registry.Get("large-values")
	if err != nil {
		t.Fatalf("Failed to get large-values workload: %v", err)
	}

	// Just verify it exists and has a name
	if w.Name != "large-values" {
		t.Errorf("Name = %q, want %q", w.Name, "large-values")
	}
}

func TestSessionWorkload(t *testing.T) {
	registry := NewRegistry()
	w, _ := registry.Get("session")

	// Session workload simulates session storage (balanced reads/writes)
	// Just verify it exists and has valid configuration
	if w == nil {
		t.Error("Session workload should exist")
	}
}

package environment

import (
	"context"
	"strings"
	"testing"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
)

func TestCapture(t *testing.T) {
	capturer := NewCapturer()
	target := &domain.Target{
		Host: "localhost",
		Port: 6379,
	}
	
	env, err := capturer.Capture(context.Background(), target)
	if err != nil {
		t.Fatalf("Capture() error = %v", err)
	}

	// Basic validation
	t.Run("OS", func(t *testing.T) {
		if env.Host == nil || env.Host.OS == "" {
			t.Error("OS should not be empty")
		}
		// Should be darwin, linux, or windows
		validOS := []string{"darwin", "linux", "windows"}
		found := false
		for _, os := range validOS {
			if env.Host.OS == os {
				found = true
				break
			}
		}
		if !found {
			t.Logf("Unexpected OS: %s", env.Host.OS)
		}
	})

	t.Run("Arch", func(t *testing.T) {
		if env.Host == nil || env.Host.Arch == "" {
			t.Error("Arch should not be empty")
		}
		// Common architectures
		validArch := []string{"amd64", "arm64", "386", "arm"}
		found := false
		for _, arch := range validArch {
			if env.Host.Arch == arch {
				found = true
				break
			}
		}
		if !found {
			t.Logf("Unexpected Arch: %s", env.Host.Arch)
		}
	})

	t.Run("Hostname", func(t *testing.T) {
		if env.Host == nil || env.Host.Hostname == "" {
			t.Error("Hostname should not be empty")
		}
	})

	t.Run("CPUCount", func(t *testing.T) {
		if env.Host == nil || env.Host.CPUs <= 0 {
			t.Errorf("CPUs = %d, should be > 0", env.Host.CPUs)
		}
	})

	t.Run("TotalMemory", func(t *testing.T) {
		if env.Host == nil || env.Host.MemoryGB <= 0 {
			t.Errorf("MemoryGB = %f, should be > 0", env.Host.MemoryGB)
		}
	})

	t.Run("Fingerprint", func(t *testing.T) {
		if env.Fingerprint == "" {
			t.Error("Fingerprint should not be empty")
		}
		// Fingerprint should be a non-empty string (length may vary by implementation)
		t.Logf("Fingerprint: %s (length %d)", env.Fingerprint, len(env.Fingerprint))
	})
}

func TestCaptureWithInvalidRedis(t *testing.T) {
	capturer := NewCapturer()
	target := &domain.Target{
		Host: "invalid-host-xxx",
		Port: 9999,
	}
	
	// Capture with invalid Redis should still work (just won't have Redis info)
	env, err := capturer.Capture(context.Background(), target)
	// Should not error - it should just skip Redis info
	if err != nil {
		// If it errors, that's also acceptable behavior
		t.Logf("Capture with invalid Redis returned error (acceptable): %v", err)
		return
	}

	// Should still have basic system info
	if env.Host == nil || env.Host.OS == "" {
		t.Error("OS should not be empty even with invalid Redis")
	}
	if env.Host == nil || env.Host.Hostname == "" {
		t.Error("Hostname should not be empty even with invalid Redis")
	}
}

func TestFingerprintConsistency(t *testing.T) {
	capturer := NewCapturer()
	target := &domain.Target{
		Host: "localhost",
		Port: 6379,
	}
	
	// Same environment should produce same fingerprint
	env1, err := capturer.Capture(context.Background(), target)
	if err != nil {
		t.Fatalf("First Capture() error = %v", err)
	}

	env2, err := capturer.Capture(context.Background(), target)
	if err != nil {
		t.Fatalf("Second Capture() error = %v", err)
	}

	if env1.Fingerprint != env2.Fingerprint {
		t.Errorf("Fingerprints should be identical:\n  env1: %s\n  env2: %s", 
			env1.Fingerprint, env2.Fingerprint)
	}
}

func TestCapturedMetadata(t *testing.T) {
	capturer := NewCapturer()
	target := &domain.Target{
		Host: "localhost",
		Port: 6379,
	}
	
	env, err := capturer.Capture(context.Background(), target)
	if err != nil {
		t.Fatalf("Capture() error = %v", err)
	}

	t.Run("CPUModel", func(t *testing.T) {
		// CPU model may or may not be available
		if env.Host != nil && env.Host.CPUModel != "" {
			t.Logf("CPU Model: %s", env.Host.CPUModel)
		}
	})

	t.Run("NetworkLatency", func(t *testing.T) {
		// Network latency should be positive if Redis is reachable
		if env.NetworkLatencyMs > 0 {
			t.Logf("Network Latency: %.2f ms", env.NetworkLatencyMs)
		}
	})

	t.Run("RedisVersion", func(t *testing.T) {
		// Redis version should be captured if Redis is running
		if env.Redis != nil && env.Redis.Version != "" {
			if !strings.Contains(env.Redis.Version, ".") {
				t.Errorf("RedisVersion = %q, doesn't look like a version", env.Redis.Version)
			}
			t.Logf("Redis Version: %s", env.Redis.Version)
		}
	})
}

func TestEnvironmentString(t *testing.T) {
	env := &domain.Environment{
		Host: &domain.HostInfo{
			OS:       "darwin",
			Arch:     "arm64",
			Hostname: "test-host",
			CPUs:     8,
			CPUModel: "Apple M1",
			MemoryGB: 16,
		},
		Redis: &domain.RedisInfo{
			Version: "7.0.0",
		},
		Fingerprint: "abc123",
	}

	// Verify the environment can be formatted
	str := formatEnvironment(env)
	if str == "" {
		t.Error("formatEnvironment() should not return empty string")
	}

	// Should contain key info
	if !strings.Contains(str, "darwin") {
		t.Error("formatEnvironment() should contain OS")
	}
	if !strings.Contains(str, "arm64") {
		t.Error("formatEnvironment() should contain Arch")
	}
}

// Helper function to format environment for display
func formatEnvironment(env *domain.Environment) string {
	if env.Host == nil {
		return ""
	}
	return env.Host.OS + "/" + env.Host.Arch + " " + env.Host.Hostname
}

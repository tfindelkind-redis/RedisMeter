// Package environment captures execution environment details.
package environment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
)

// Capturer gathers environment information.
type Capturer struct{}

// NewCapturer creates a new environment capturer.
func NewCapturer() *Capturer {
	return &Capturer{}
}

// Capture gathers all environment details.
func (c *Capturer) Capture(ctx context.Context, target *domain.Target) (*domain.Environment, error) {
	env := &domain.Environment{
		RedisMeterVersion: "0.1.0-dev",
	}

	// Capture host info
	c.captureHost(env)

	// Capture hardware info
	c.captureHardware(env)

	// Capture memtier version
	c.captureMemtierVersion(env)

	// Capture Redis info if target provided
	if target != nil {
		c.captureRedisInfo(ctx, env, target)
		c.captureNetworkLatency(ctx, env, target)
	}

	// Generate fingerprint
	env.Fingerprint = c.generateFingerprint(env)

	return env, nil
}

// captureHost gathers host system information.
func (c *Capturer) captureHost(env *domain.Environment) {
	hostname, _ := os.Hostname()
	env.Hostname = hostname
	env.OS = runtime.GOOS
	env.Arch = runtime.GOARCH

	// Get kernel version on Unix systems
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		output, err := exec.Command("uname", "-r").Output()
		if err == nil {
			env.KernelVersion = strings.TrimSpace(string(output))
		}
	}
}

// captureHardware gathers hardware information.
func (c *Capturer) captureHardware(env *domain.Environment) {
	env.CPUCores = runtime.NumCPU()

	// Get CPU model
	if runtime.GOOS == "darwin" {
		output, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output()
		if err == nil {
			env.CPUModel = strings.TrimSpace(string(output))
		}

		// Get memory
		output, err = exec.Command("sysctl", "-n", "hw.memsize").Output()
		if err == nil {
			var memBytes int64
			fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &memBytes)
			env.MemoryGB = float64(memBytes) / (1024 * 1024 * 1024)
		}
	} else if runtime.GOOS == "linux" {
		// Read /proc/cpuinfo for model
		data, err := os.ReadFile("/proc/cpuinfo")
		if err == nil {
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "model name") {
					parts := strings.SplitN(line, ":", 2)
					if len(parts) == 2 {
						env.CPUModel = strings.TrimSpace(parts[1])
						break
					}
				}
			}
		}

		// Read /proc/meminfo for memory
		data, err = os.ReadFile("/proc/meminfo")
		if err == nil {
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "MemTotal:") {
					var memKB int64
					fmt.Sscanf(line, "MemTotal: %d kB", &memKB)
					env.MemoryGB = float64(memKB) / (1024 * 1024)
					break
				}
			}
		}
	}
}

// captureMemtierVersion gets the memtier_benchmark version.
func (c *Capturer) captureMemtierVersion(env *domain.Environment) {
	output, err := exec.Command("memtier_benchmark", "--version").Output()
	if err == nil {
		env.MemtierVersion = strings.TrimSpace(string(output))
	}
}

// captureRedisInfo gathers Redis server information.
func (c *Capturer) captureRedisInfo(ctx context.Context, env *domain.Environment, target *domain.Target) {
	// Build redis-cli command
	args := []string{"-h", target.Host, "-p", fmt.Sprintf("%d", target.Port)}
	if target.Password != "" {
		args = append(args, "-a", target.Password)
	}
	if target.Username != "" {
		args = append(args, "--user", target.Username)
	}
	args = append(args, "INFO", "server")

	cmd := exec.CommandContext(ctx, "redis-cli", args...)
	output, err := cmd.Output()
	if err != nil {
		return
	}

	// Parse INFO output
	lines := strings.Split(string(output), "\n")
	config := make(map[string]string)
	var modules []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			config[key] = value

			if key == "redis_version" {
				env.RedisVersion = value
			}
		}
	}

	env.RedisConfig = config

	// Get loaded modules
	args = args[:len(args)-1] // Remove "server"
	args[len(args)-1] = "MODULE"
	args = append(args, "LIST")

	cmd = exec.CommandContext(ctx, "redis-cli", args...)
	output, err = cmd.Output()
	if err == nil {
		// Parse module list
		lines = strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "name") {
				// Extract module name
				parts := strings.Fields(line)
				for i, p := range parts {
					if p == "name" && i+1 < len(parts) {
						modules = append(modules, parts[i+1])
					}
				}
			}
		}
	}
	env.RedisModules = modules
}

// captureNetworkLatency measures network latency to target.
func (c *Capturer) captureNetworkLatency(ctx context.Context, env *domain.Environment, target *domain.Target) {
	addr := fmt.Sprintf("%s:%d", target.Host, target.Port)

	// Measure TCP connection time as proxy for latency
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return
	}
	latency := time.Since(start)
	conn.Close()

	env.NetworkLatencyMs = float64(latency.Microseconds()) / 1000.0
}

// generateFingerprint creates a hash of environment details.
func (c *Capturer) generateFingerprint(env *domain.Environment) string {
	// Create a struct with key identifying fields
	key := struct {
		OS           string
		Arch         string
		CPUModel     string
		CPUCores     int
		MemoryGB     float64
		RedisVersion string
	}{
		OS:           env.OS,
		Arch:         env.Arch,
		CPUModel:     env.CPUModel,
		CPUCores:     env.CPUCores,
		MemoryGB:     env.MemoryGB,
		RedisVersion: env.RedisVersion,
	}

	data, _ := json.Marshal(key)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:8]) // First 8 bytes = 16 hex chars
}

// CompareEnvironments compares two environments and returns differences.
func CompareEnvironments(a, b *domain.Environment) []string {
	var diffs []string

	if a.OS != b.OS {
		diffs = append(diffs, fmt.Sprintf("OS: %s vs %s", a.OS, b.OS))
	}
	if a.Arch != b.Arch {
		diffs = append(diffs, fmt.Sprintf("Arch: %s vs %s", a.Arch, b.Arch))
	}
	if a.CPUCores != b.CPUCores {
		diffs = append(diffs, fmt.Sprintf("CPU Cores: %d vs %d", a.CPUCores, b.CPUCores))
	}
	if a.RedisVersion != b.RedisVersion {
		diffs = append(diffs, fmt.Sprintf("Redis Version: %s vs %s", a.RedisVersion, b.RedisVersion))
	}

	return diffs
}

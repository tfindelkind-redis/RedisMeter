// Package environment captures execution environment details.
package environment

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
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
		Host:              &domain.HostInfo{},
		Redis:             &domain.RedisInfo{},
	}

	// Capture host info
	c.captureHost(env)

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
	env.Host.Hostname = hostname
	env.Host.OS = runtime.GOOS
	env.Host.Arch = runtime.GOARCH
	env.Host.CPUs = runtime.NumCPU()

	// Get kernel version on Unix systems
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		output, err := exec.Command("uname", "-r").Output()
		if err == nil {
			env.Host.KernelVersion = strings.TrimSpace(string(output))
		}
	}

	// Get CPU model and memory
	if runtime.GOOS == "darwin" {
		output, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output()
		if err == nil {
			env.Host.CPUModel = strings.TrimSpace(string(output))
		}

		output, err = exec.Command("sysctl", "-n", "hw.memsize").Output()
		if err == nil {
			var memBytes int64
			fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &memBytes)
			env.Host.MemoryGB = float64(memBytes) / (1024 * 1024 * 1024)
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
						env.Host.CPUModel = strings.TrimSpace(parts[1])
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
					env.Host.MemoryGB = float64(memKB) / (1024 * 1024)
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

// buildRedisCliArgs builds the redis-cli command arguments for a target.
func (c *Capturer) buildRedisCliArgs(target *domain.Target) []string {
	args := []string{"-h", target.Host, "-p", fmt.Sprintf("%d", target.Port)}
	if target.Password != "" {
		args = append(args, "-a", target.Password)
	}
	if target.Username != "" {
		args = append(args, "--user", target.Username)
	}
	// Add TLS support for secure connections (e.g., Azure Managed Redis)
	if target.TLS != nil && target.TLS.Enabled {
		args = append(args, "--tls")
		if target.TLS.InsecureSkipVerify {
			args = append(args, "--insecure")
		}
		if target.TLS.CertFile != "" {
			args = append(args, "--cert", target.TLS.CertFile)
		}
		if target.TLS.KeyFile != "" {
			args = append(args, "--key", target.TLS.KeyFile)
		}
		if target.TLS.CAFile != "" {
			args = append(args, "--cacert", target.TLS.CAFile)
		}
	}
	return args
}

// captureRedisInfo gathers comprehensive Redis server information.
func (c *Capturer) captureRedisInfo(ctx context.Context, env *domain.Environment, target *domain.Target) {
	baseArgs := c.buildRedisCliArgs(target)

	// Get full INFO (all sections)
	infoArgs := append(baseArgs, "INFO")
	cmd := exec.CommandContext(ctx, "redis-cli", infoArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Fallback: try just INFO server if full INFO fails
		infoArgs = append(baseArgs, "INFO", "server")
		cmd = exec.CommandContext(ctx, "redis-cli", infoArgs...)
		output, _ = cmd.CombinedOutput()
	}

	// Parse INFO output into a map
	infoMap := c.parseInfoOutput(string(output))
	c.populateRedisInfo(env.Redis, infoMap)

	// Get CONFIG GET maxmemory-policy
	configArgs := append(baseArgs, "CONFIG", "GET", "maxmemory-policy")
	cmd = exec.CommandContext(ctx, "redis-cli", configArgs...)
	output, err = cmd.CombinedOutput()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(lines) >= 2 {
			env.Redis.MemoryEvictionPolicy = strings.TrimSpace(lines[1])
		}
	}

	// Get DBSIZE for total keys
	dbsizeArgs := append(baseArgs, "DBSIZE")
	cmd = exec.CommandContext(ctx, "redis-cli", dbsizeArgs...)
	output, err = cmd.CombinedOutput()
	if err == nil {
		// Parse "(integer) 12345" format
		outputStr := strings.TrimSpace(string(output))
		if strings.HasPrefix(outputStr, "(integer)") {
			outputStr = strings.TrimPrefix(outputStr, "(integer)")
			outputStr = strings.TrimSpace(outputStr)
		}
		if val, err := strconv.ParseInt(outputStr, 10, 64); err == nil {
			env.Redis.TotalKeys = val
		}
	}

	// Get loaded modules
	moduleArgs := append(baseArgs, "MODULE", "LIST")
	cmd = exec.CommandContext(ctx, "redis-cli", moduleArgs...)
	output, err = cmd.CombinedOutput()
	if err == nil {
		env.Redis.Modules = c.parseModuleList(string(output))
	}
}

// parseInfoOutput parses Redis INFO output into a key-value map.
func (c *Capturer) parseInfoOutput(output string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			result[key] = value
		}
	}
	return result
}

// populateRedisInfo fills the RedisInfo struct from parsed INFO output.
func (c *Capturer) populateRedisInfo(redis *domain.RedisInfo, info map[string]string) {
	// Store raw config
	redis.Config = info

	// Server info
	redis.Version = info["redis_version"]
	redis.OS = info["os"]
	redis.Arch = info["arch_bits"]
	if uptime, err := strconv.ParseInt(info["uptime_in_seconds"], 10, 64); err == nil {
		redis.Uptime = uptime
	}

	// Determine mode
	if info["cluster_enabled"] == "1" {
		redis.Mode = "cluster"
		redis.ClusterEnabled = true
	} else if info["role"] == "sentinel" {
		redis.Mode = "sentinel"
	} else {
		redis.Mode = "standalone"
	}

	// Memory metrics
	if val, err := strconv.ParseInt(info["used_memory"], 10, 64); err == nil {
		redis.MemoryUsed = val
	}
	if val, err := strconv.ParseInt(info["maxmemory"], 10, 64); err == nil {
		redis.MemoryMax = val
	}
	if val, err := strconv.ParseInt(info["used_memory_peak"], 10, 64); err == nil {
		redis.MemoryPeak = val
	}
	if val, err := strconv.ParseFloat(info["mem_fragmentation_ratio"], 64); err == nil {
		redis.MemoryFragRatio = val
	}

	// Clients
	if val, err := strconv.Atoi(info["connected_clients"]); err == nil {
		redis.ConnectedClients = val
	}
	if val, err := strconv.Atoi(info["blocked_clients"]); err == nil {
		redis.BlockedClients = val
	}

	// Stats
	if val, err := strconv.ParseInt(info["total_connections_received"], 10, 64); err == nil {
		redis.TotalConnectionsReceived = val
	}
	if val, err := strconv.ParseInt(info["total_commands_processed"], 10, 64); err == nil {
		redis.TotalCommandsProcessed = val
	}
	if val, err := strconv.ParseInt(info["instantaneous_ops_per_sec"], 10, 64); err == nil {
		redis.InstantaneousOpsPerSec = val
	}
	if val, err := strconv.ParseInt(info["evicted_keys"], 10, 64); err == nil {
		redis.EvictedKeys = val
	}
	if val, err := strconv.ParseInt(info["expired_keys"], 10, 64); err == nil {
		redis.ExpiredKeys = val
	}
	if val, err := strconv.ParseInt(info["keyspace_hits"], 10, 64); err == nil {
		redis.KeyspaceHits = val
	}
	if val, err := strconv.ParseInt(info["keyspace_misses"], 10, 64); err == nil {
		redis.KeyspaceMisses = val
	}

	// Persistence
	redis.AOFEnabled = info["aof_enabled"] == "1"
	redis.RDBEnabled = info["rdb_last_bgsave_status"] != ""
	if val, err := strconv.ParseInt(info["rdb_last_save_time"], 10, 64); err == nil {
		redis.RDBLastSaveTime = val
	}
	redis.RDBLastBgSaveStatus = info["rdb_last_bgsave_status"]

	// Replication
	redis.Role = info["role"]
	if val, err := strconv.Atoi(info["connected_slaves"]); err == nil {
		redis.ConnectedSlaves = val
	}

	// Cluster
	if val, err := strconv.Atoi(info["cluster_size"]); err == nil {
		redis.ClusterSize = val
	}

	// Keyspace - extract db entries
	keyspace := make(map[string]string)
	var totalExpires int64
	for k, v := range info {
		if strings.HasPrefix(k, "db") {
			keyspace[k] = v
			// Parse "keys=N,expires=M,avg_ttl=X"
			parts := strings.Split(v, ",")
			for _, part := range parts {
				if strings.HasPrefix(part, "expires=") {
					if val, err := strconv.ParseInt(strings.TrimPrefix(part, "expires="), 10, 64); err == nil {
						totalExpires += val
					}
				}
			}
		}
	}
	if len(keyspace) > 0 {
		redis.Keyspace = keyspace
		redis.TotalExpires = totalExpires
	}
}

// parseModuleList extracts module names from MODULE LIST output.
func (c *Capturer) parseModuleList(output string) []string {
	var modules []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "name") {
			parts := strings.Fields(line)
			for i, p := range parts {
				if p == "name" && i+1 < len(parts) {
					modules = append(modules, parts[i+1])
				}
			}
		}
	}
	return modules
}

// captureNetworkLatency measures network latency to target.
func (c *Capturer) captureNetworkLatency(ctx context.Context, env *domain.Environment, target *domain.Target) {
	addr := fmt.Sprintf("%s:%d", target.Host, target.Port)

	// Measure TCP connection time as proxy for latency
	start := time.Now()
	var conn net.Conn
	var err error

	if target.TLS != nil && target.TLS.Enabled {
		// TLS connection for secure Redis
		tlsConfig := &tls.Config{
			InsecureSkipVerify: target.TLS.InsecureSkipVerify,
		}
		conn, err = tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", addr, tlsConfig)
	} else {
		conn, err = net.DialTimeout("tcp", addr, 5*time.Second)
	}

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
		CPUs         int
		MemoryGB     float64
		RedisVersion string
		RedisMode    string
	}{
		OS:           env.Host.OS,
		Arch:         env.Host.Arch,
		CPUModel:     env.Host.CPUModel,
		CPUs:         env.Host.CPUs,
		MemoryGB:     env.Host.MemoryGB,
		RedisVersion: env.Redis.Version,
		RedisMode:    env.Redis.Mode,
	}

	data, _ := json.Marshal(key)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:8]) // First 8 bytes = 16 hex chars
}

// CompareEnvironments compares two environments and returns differences.
func CompareEnvironments(a, b *domain.Environment) []string {
	var diffs []string

	if a.Host != nil && b.Host != nil {
		if a.Host.OS != b.Host.OS {
			diffs = append(diffs, fmt.Sprintf("OS: %s vs %s", a.Host.OS, b.Host.OS))
		}
		if a.Host.Arch != b.Host.Arch {
			diffs = append(diffs, fmt.Sprintf("Arch: %s vs %s", a.Host.Arch, b.Host.Arch))
		}
		if a.Host.CPUs != b.Host.CPUs {
			diffs = append(diffs, fmt.Sprintf("CPU Cores: %d vs %d", a.Host.CPUs, b.Host.CPUs))
		}
	}

	if a.Redis != nil && b.Redis != nil {
		if a.Redis.Version != b.Redis.Version {
			diffs = append(diffs, fmt.Sprintf("Redis Version: %s vs %s", a.Redis.Version, b.Redis.Version))
		}
		if a.Redis.Mode != b.Redis.Mode {
			diffs = append(diffs, fmt.Sprintf("Redis Mode: %s vs %s", a.Redis.Mode, b.Redis.Mode))
		}
	}

	return diffs
}

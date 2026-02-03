// Package cloud provides cloud provider infrastructure management for RedisMeter.
package cloud

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tfindelkind-redis/redismeter/internal/domain"
	"github.com/tfindelkind-redis/redismeter/internal/plugin"
	"golang.org/x/crypto/ssh"
)

// SSHExecutor executes benchmarks on remote hosts via SSH.
type SSHExecutor struct {
	mu          sync.RWMutex
	connections map[string]*sshConnection
	executions  map[string]*sshExecution
	config      SSHConfig
}

// SSHConfig holds SSH executor configuration.
type SSHConfig struct {
	// User is the SSH username.
	User string `json:"user"`

	// PrivateKeyPath is the path to the private key.
	PrivateKeyPath string `json:"private_key_path,omitempty"`

	// PrivateKey is the private key content (alternative to path).
	PrivateKey string `json:"private_key,omitempty"`

	// Password for password authentication (not recommended).
	Password string `json:"password,omitempty"`

	// Port is the SSH port (default 22).
	Port int `json:"port,omitempty"`

	// ConnectTimeout is the connection timeout.
	ConnectTimeout time.Duration `json:"connect_timeout,omitempty"`

	// CommandTimeout is the default command execution timeout.
	CommandTimeout time.Duration `json:"command_timeout,omitempty"`

	// KnownHostsPath is the path to known_hosts file (optional).
	KnownHostsPath string `json:"known_hosts_path,omitempty"`

	// StrictHostKeyChecking enables host key verification.
	StrictHostKeyChecking bool `json:"strict_host_key_checking,omitempty"`

	// MemtierPath is the path to memtier_benchmark on remote hosts.
	MemtierPath string `json:"memtier_path,omitempty"`
}

type sshConnection struct {
	client *ssh.Client
	host   string
	port   int
}

type sshExecution struct {
	id        string
	host      string
	session   *ssh.Session
	state     plugin.ExecutionState
	startTime time.Time
	endTime   time.Time
	output    strings.Builder
	err       error
	metrics   chan *domain.Metrics
	done      chan struct{}
}

// NewSSHExecutor creates a new SSH executor.
func NewSSHExecutor() *SSHExecutor {
	return &SSHExecutor{
		connections: make(map[string]*sshConnection),
		executions:  make(map[string]*sshExecution),
		config: SSHConfig{
			Port:           22,
			ConnectTimeout: 30 * time.Second,
			CommandTimeout: 5 * time.Minute,
			MemtierPath:    "memtier_benchmark",
		},
	}
}

// Metadata returns plugin metadata.
func (e *SSHExecutor) Metadata() plugin.Metadata {
	return plugin.Metadata{
		Name:        "ssh",
		Version:     "1.0.0",
		Type:        plugin.TypeExecutor,
		Description: "Execute benchmarks on remote hosts via SSH",
	}
}

// Initialize sets up the SSH executor.
func (e *SSHExecutor) Initialize(ctx context.Context, config map[string]interface{}) error {
	if user, ok := config["user"].(string); ok {
		e.config.User = user
	}
	if keyPath, ok := config["private_key_path"].(string); ok {
		e.config.PrivateKeyPath = keyPath
	}
	if key, ok := config["private_key"].(string); ok {
		e.config.PrivateKey = key
	}
	if password, ok := config["password"].(string); ok {
		e.config.Password = password
	}
	if port, ok := config["port"].(int); ok {
		e.config.Port = port
	}
	if port, ok := config["port"].(float64); ok {
		e.config.Port = int(port)
	}
	if timeout, ok := config["connect_timeout"].(string); ok {
		if d, err := time.ParseDuration(timeout); err == nil {
			e.config.ConnectTimeout = d
		}
	}
	if path, ok := config["memtier_path"].(string); ok {
		e.config.MemtierPath = path
	}
	if strict, ok := config["strict_host_key_checking"].(bool); ok {
		e.config.StrictHostKeyChecking = strict
	}

	return nil
}

// HealthCheck verifies the executor is healthy.
func (e *SSHExecutor) HealthCheck(ctx context.Context) plugin.HealthStatus {
	// Check if we have valid configuration
	if e.config.User == "" {
		return plugin.HealthStatus{
			Healthy: false,
			Message: "SSH user not configured",
		}
	}

	if e.config.PrivateKeyPath == "" && e.config.PrivateKey == "" && e.config.Password == "" {
		return plugin.HealthStatus{
			Healthy: false,
			Message: "No SSH authentication method configured",
		}
	}

	return plugin.HealthStatus{
		Healthy: true,
		Message: "SSH executor ready",
	}
}

// Shutdown closes all SSH connections.
func (e *SSHExecutor) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, conn := range e.connections {
		if conn.client != nil {
			conn.client.Close()
		}
	}
	e.connections = make(map[string]*sshConnection)

	return nil
}

// Execute starts a benchmark on a remote host.
func (e *SSHExecutor) Execute(ctx context.Context, workload *domain.Workload, target *domain.Target) (*plugin.ExecutionHandle, error) {
	// Validate target has a host
	if target.Host == "" {
		return nil, fmt.Errorf("target host is required")
	}

	// Get SSH host from target labels or use target host
	sshHost := target.Host
	if target.Labels != nil {
		if meta, ok := target.Labels["ssh_host"]; ok && meta != "" {
			sshHost = meta
		}
	}

	// Connect to remote host
	conn, err := e.getOrCreateConnection(ctx, sshHost)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", sshHost, err)
	}

	// Create session
	session, err := conn.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH session: %w", err)
	}

	// Create execution
	execID := uuid.New().String()[:8]
	exec := &sshExecution{
		id:        execID,
		host:      sshHost,
		session:   session,
		state:     plugin.StateQueued,
		startTime: time.Now(),
		metrics:   make(chan *domain.Metrics, 100),
		done:      make(chan struct{}),
	}

	e.mu.Lock()
	e.executions[execID] = exec
	e.mu.Unlock()

	// Build memtier command
	cmd := e.buildMemtierCommand(workload, target)

	// Start execution in background
	go e.runRemoteCommand(ctx, exec, cmd)

	return &plugin.ExecutionHandle{
		ID:           execID,
		ExecutorName: "ssh",
	}, nil
}

// Status returns the status of an execution.
func (e *SSHExecutor) Status(ctx context.Context, handle *plugin.ExecutionHandle) (*plugin.ExecutionStatus, error) {
	e.mu.RLock()
	exec, ok := e.executions[handle.ID]
	e.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("execution not found: %s", handle.ID)
	}

	status := &plugin.ExecutionStatus{
		Handle:    handle,
		State:     exec.state,
		StartTime: exec.startTime.Format(time.RFC3339),
	}

	if exec.state == plugin.StateCompleted || exec.state == plugin.StateFailed {
		status.EndTime = exec.endTime.Format(time.RFC3339)
	}

	if exec.err != nil {
		status.Error = exec.err.Error()
	}

	return status, nil
}

// Stop terminates a running execution.
func (e *SSHExecutor) Stop(ctx context.Context, handle *plugin.ExecutionHandle) error {
	e.mu.RLock()
	exec, ok := e.executions[handle.ID]
	e.mu.RUnlock()

	if !ok {
		return fmt.Errorf("execution not found: %s", handle.ID)
	}

	if exec.session != nil {
		exec.session.Signal(ssh.SIGTERM)
		exec.session.Close()
	}

	exec.state = plugin.StateCancelled
	exec.endTime = time.Now()
	close(exec.done)

	return nil
}

// StreamMetrics returns a channel of real-time metrics.
func (e *SSHExecutor) StreamMetrics(ctx context.Context, handle *plugin.ExecutionHandle) (<-chan *domain.Metrics, error) {
	e.mu.RLock()
	exec, ok := e.executions[handle.ID]
	e.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("execution not found: %s", handle.ID)
	}

	return exec.metrics, nil
}

// GetOutput returns the execution output.
func (e *SSHExecutor) GetOutput(handle *plugin.ExecutionHandle) (string, error) {
	e.mu.RLock()
	exec, ok := e.executions[handle.ID]
	e.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("execution not found: %s", handle.ID)
	}

	return exec.output.String(), nil
}

// Connect establishes an SSH connection to a host.
func (e *SSHExecutor) Connect(ctx context.Context, host string) error {
	_, err := e.getOrCreateConnection(ctx, host)
	return err
}

// RunCommand executes a command on a remote host.
func (e *SSHExecutor) RunCommand(ctx context.Context, host, command string) (string, error) {
	conn, err := e.getOrCreateConnection(ctx, host)
	if err != nil {
		return "", err
	}

	session, err := conn.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}

	return string(output), nil
}

// CopyFile copies a local file to a remote host.
func (e *SSHExecutor) CopyFile(ctx context.Context, host, localPath, remotePath string) error {
	_, err := e.getOrCreateConnection(ctx, host)
	if err != nil {
		return err
	}

	// Read local file
	content, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to read local file: %w", err)
	}

	return e.CopyContent(ctx, host, content, remotePath)
}

// CopyContent copies content to a remote file.
func (e *SSHExecutor) CopyContent(ctx context.Context, host string, content []byte, remotePath string) error {
	hostConn, err := e.getOrCreateConnection(ctx, host)
	if err != nil {
		return err
	}

	session, err := hostConn.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Create remote directory if needed
	dir := filepath.Dir(remotePath)
	if dir != "." && dir != "/" {
		dirSession, _ := hostConn.client.NewSession()
		dirSession.Run(fmt.Sprintf("mkdir -p %s", dir))
		dirSession.Close()
	}

	// Use cat to write file content
	go func() {
		w, _ := session.StdinPipe()
		defer w.Close()
		w.Write(content)
	}()

	if err := session.Run(fmt.Sprintf("cat > %s", remotePath)); err != nil {
		return fmt.Errorf("failed to write remote file: %w", err)
	}

	return nil
}

// getOrCreateConnection gets an existing connection or creates a new one.
func (e *SSHExecutor) getOrCreateConnection(ctx context.Context, host string) (*sshConnection, error) {
	e.mu.RLock()
	conn, ok := e.connections[host]
	e.mu.RUnlock()

	if ok && conn.client != nil {
		// Test if connection is still alive
		_, _, err := conn.client.SendRequest("keepalive", true, nil)
		if err == nil {
			return conn, nil
		}
		// Connection dead, remove it
		e.mu.Lock()
		delete(e.connections, host)
		e.mu.Unlock()
	}

	// Create new connection
	config, err := e.buildSSHConfig()
	if err != nil {
		return nil, err
	}

	addr := fmt.Sprintf("%s:%d", host, e.config.Port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", addr, err)
	}

	conn = &sshConnection{
		client: client,
		host:   host,
		port:   e.config.Port,
	}

	e.mu.Lock()
	e.connections[host] = conn
	e.mu.Unlock()

	return conn, nil
}

// buildSSHConfig creates SSH client configuration.
func (e *SSHExecutor) buildSSHConfig() (*ssh.ClientConfig, error) {
	var authMethods []ssh.AuthMethod

	// Try private key first
	if e.config.PrivateKeyPath != "" {
		key, err := os.ReadFile(e.config.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read private key: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else if e.config.PrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(e.config.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	// Fall back to password
	if e.config.Password != "" {
		authMethods = append(authMethods, ssh.Password(e.config.Password))
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication method configured")
	}

	// Host key callback
	var hostKeyCallback ssh.HostKeyCallback
	if e.config.StrictHostKeyChecking {
		// TODO: Implement proper known_hosts checking
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	} else {
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	return &ssh.ClientConfig{
		User:            e.config.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         e.config.ConnectTimeout,
	}, nil
}

// buildMemtierCommand builds the memtier_benchmark command string.
func (e *SSHExecutor) buildMemtierCommand(workload *domain.Workload, target *domain.Target) string {
	var args []string
	args = append(args, e.config.MemtierPath)

	// Target
	args = append(args, "-s", target.Host)
	args = append(args, "-p", fmt.Sprintf("%d", target.Port))

	// Authentication
	if target.Password != "" {
		args = append(args, "-a", target.Password)
	}

	// Workload parameters
	if workload != nil {
		if workload.Clients > 0 {
			args = append(args, "-c", fmt.Sprintf("%d", workload.Clients))
		}
		if workload.Threads > 0 {
			args = append(args, "-t", fmt.Sprintf("%d", workload.Threads))
		}
		if workload.Requests > 0 {
			args = append(args, "-n", fmt.Sprintf("%d", workload.Requests))
		}
		if workload.Duration != "" {
			args = append(args, "--test-time", workload.Duration)
		}
		// Calculate ratio from operations if defined
		if len(workload.Operations) > 0 {
			var setRatio, getRatio float64
			for _, op := range workload.Operations {
				if op.Command == "SET" {
					setRatio = op.Ratio
				} else if op.Command == "GET" {
					getRatio = op.Ratio
				}
			}
			if setRatio > 0 || getRatio > 0 {
				args = append(args, "--ratio", fmt.Sprintf("%.0f:%.0f", setRatio*10, getRatio*10))
			}
		}
		if workload.DataSize != nil && workload.DataSize.Fixed > 0 {
			args = append(args, "-d", fmt.Sprintf("%d", workload.DataSize.Fixed))
		}
		if workload.KeyPattern != nil && workload.KeyPattern.Pattern != "" {
			args = append(args, "--key-pattern", workload.KeyPattern.Pattern)
		}
		if workload.Pipeline > 0 {
			args = append(args, "--pipeline", fmt.Sprintf("%d", workload.Pipeline))
		}
	}

	// JSON output for parsing
	args = append(args, "--json-out-file=/tmp/memtier_results.json")

	return strings.Join(args, " ")
}

// runRemoteCommand executes the command on the remote host.
func (e *SSHExecutor) runRemoteCommand(ctx context.Context, exec *sshExecution, cmd string) {
	defer close(exec.done)
	defer close(exec.metrics)

	exec.state = plugin.StateRunning

	// Set up stdout/stderr capture
	stdout, err := exec.session.StdoutPipe()
	if err != nil {
		exec.err = fmt.Errorf("failed to get stdout pipe: %w", err)
		exec.state = plugin.StateFailed
		exec.endTime = time.Now()
		return
	}

	stderr, err := exec.session.StderrPipe()
	if err != nil {
		exec.err = fmt.Errorf("failed to get stderr pipe: %w", err)
		exec.state = plugin.StateFailed
		exec.endTime = time.Now()
		return
	}

	// Start command
	if err := exec.session.Start(cmd); err != nil {
		exec.err = fmt.Errorf("failed to start command: %w", err)
		exec.state = plugin.StateFailed
		exec.endTime = time.Now()
		return
	}

	// Read output in goroutines
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		e.readOutput(stdout, exec)
	}()

	go func() {
		defer wg.Done()
		e.readOutput(stderr, exec)
	}()

	// Wait for completion
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- exec.session.Wait()
	}()

	select {
	case err := <-waitCh:
		wg.Wait()
		exec.endTime = time.Now()
		if err != nil {
			exec.err = err
			exec.state = plugin.StateFailed
		} else {
			exec.state = plugin.StateCompleted
		}
	case <-ctx.Done():
		exec.session.Signal(ssh.SIGTERM)
		exec.session.Close()
		exec.endTime = time.Now()
		exec.state = plugin.StateCancelled
		exec.err = ctx.Err()
	}
}

// readOutput reads from a reader and appends to the execution output.
func (e *SSHExecutor) readOutput(r io.Reader, exec *sshExecution) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		exec.output.WriteString(line)
		exec.output.WriteString("\n")
	}
}

// Ensure SSHExecutor implements ExecutorPlugin.
var _ plugin.ExecutorPlugin = (*SSHExecutor)(nil)

// MultiNodeExecutor coordinates benchmark execution across multiple nodes.
type MultiNodeExecutor struct {
	sshExecutor *SSHExecutor
	mu          sync.RWMutex
	executions  map[string]*multiNodeExecution
}

type multiNodeExecution struct {
	id        string
	hosts     []string
	handles   []*plugin.ExecutionHandle
	state     plugin.ExecutionState
	startTime time.Time
	endTime   time.Time
	results   map[string]string // host -> result
	errors    map[string]error
}

// NewMultiNodeExecutor creates a new multi-node executor.
func NewMultiNodeExecutor(sshExecutor *SSHExecutor) *MultiNodeExecutor {
	return &MultiNodeExecutor{
		sshExecutor: sshExecutor,
		executions:  make(map[string]*multiNodeExecution),
	}
}

// ExecuteOnHosts runs a benchmark across multiple hosts simultaneously.
func (m *MultiNodeExecutor) ExecuteOnHosts(ctx context.Context, hosts []string, workload *domain.Workload, target *domain.Target) (string, error) {
	execID := uuid.New().String()[:8]

	exec := &multiNodeExecution{
		id:        execID,
		hosts:     hosts,
		handles:   make([]*plugin.ExecutionHandle, 0, len(hosts)),
		state:     plugin.StateRunning,
		startTime: time.Now(),
		results:   make(map[string]string),
		errors:    make(map[string]error),
	}

	m.mu.Lock()
	m.executions[execID] = exec
	m.mu.Unlock()

	// Start executions on all hosts
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, host := range hosts {
		wg.Add(1)
		go func(h string) {
			defer wg.Done()

				// Set SSH host in target labels
			t := *target
			if t.Labels == nil {
				t.Labels = make(map[string]string)
			}
			t.Labels["ssh_host"] = h

			handle, err := m.sshExecutor.Execute(ctx, workload, &t)
			if err != nil {
				mu.Lock()
				exec.errors[h] = err
				mu.Unlock()
				return
			}

			mu.Lock()
			exec.handles = append(exec.handles, handle)
			mu.Unlock()
		}(host)
	}

	wg.Wait()

	// Check if any started successfully
	if len(exec.handles) == 0 {
		exec.state = plugin.StateFailed
		exec.endTime = time.Now()
		return execID, fmt.Errorf("failed to start on any host")
	}

	return execID, nil
}

// WaitForCompletion waits for all nodes to complete.
func (m *MultiNodeExecutor) WaitForCompletion(ctx context.Context, execID string) error {
	m.mu.RLock()
	exec, ok := m.executions[execID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("execution not found: %s", execID)
	}

	// Poll until all complete
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			allDone := true
			for _, handle := range exec.handles {
				status, err := m.sshExecutor.Status(ctx, handle)
				if err != nil {
					continue
				}
				if status.State == plugin.StateRunning || status.State == plugin.StateQueued {
					allDone = false
					break
				}
			}

			if allDone {
				exec.state = plugin.StateCompleted
				exec.endTime = time.Now()
				return nil
			}
		}
	}
}

// GetAggregatedResults combines results from all nodes.
func (m *MultiNodeExecutor) GetAggregatedResults(execID string) (map[string]string, error) {
	m.mu.RLock()
	exec, ok := m.executions[execID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("execution not found: %s", execID)
	}

	results := make(map[string]string)
	for i, handle := range exec.handles {
		if i < len(exec.hosts) {
			output, err := m.sshExecutor.GetOutput(handle)
			if err == nil {
				results[exec.hosts[i]] = output
			}
		}
	}

	return results, nil
}

// DialWithRetry attempts to connect with retries.
func DialWithRetry(ctx context.Context, network, addr string, config *ssh.ClientConfig, maxRetries int, retryDelay time.Duration) (*ssh.Client, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		conn, err := net.DialTimeout(network, addr, config.Timeout)
		if err != nil {
			lastErr = err
			time.Sleep(retryDelay)
			continue
		}

		c, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
		if err != nil {
			conn.Close()
			lastErr = err
			time.Sleep(retryDelay)
			continue
		}

		return ssh.NewClient(c, chans, reqs), nil
	}

	return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

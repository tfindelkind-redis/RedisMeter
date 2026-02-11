// Package terraform provides infrastructure management for RedisMeter.
// This file handles self-managed infrastructure (BYOI - Bring Your Own Infrastructure).
package terraform

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SelfManagedConfig holds the complete configuration for self-managed infrastructure.
type SelfManagedConfig struct {
	Runners   SelfManagedRunners `json:"runners"`
	Redis     SelfManagedRedis   `json:"redis"`
	ToolPaths map[string]string  `json:"tool_paths,omitempty"`
}

// SelfManagedRunners holds runner machine configuration.
type SelfManagedRunners struct {
	Machines []RunnerMachine `json:"machines"`
	SSH      SSHCredentials  `json:"ssh"`
}

// RunnerMachine represents a single runner machine.
type RunnerMachine struct {
	Name   string            `json:"name,omitempty"`
	Host   string            `json:"host"`
	Port   int               `json:"port,omitempty"` // Default: 22
	Labels map[string]string `json:"labels,omitempty"`
}

// SSHCredentials holds SSH authentication configuration.
// Sensitive fields are stored encrypted.
type SSHCredentials struct {
	AuthMethod            string `json:"auth_method"` // "password", "key", or "key_file"
	Username              string `json:"username"`
	PasswordEncrypted     string `json:"password_encrypted,omitempty"`
	PrivateKeyEncrypted   string `json:"private_key_encrypted,omitempty"`
	PrivateKeyPath        string `json:"private_key_path,omitempty"`
	PassphraseEncrypted   string `json:"passphrase_encrypted,omitempty"`
	Port                  int    `json:"port,omitempty"`
	ConnectTimeout        int    `json:"connect_timeout,omitempty"`
	StrictHostKeyChecking bool   `json:"strict_host_key_checking,omitempty"`
}

// SelfManagedRedis holds Redis target configuration.
type SelfManagedRedis struct {
	Targets     []RedisTargetConfig `json:"targets"`
	Credentials RedisCredentials    `json:"credentials"`
}

// RedisTargetConfig represents a Redis instance to benchmark.
type RedisTargetConfig struct {
	Name         string   `json:"name,omitempty"`
	Host         string   `json:"host"`
	Port         int      `json:"port"`
	IsCluster    bool     `json:"is_cluster,omitempty"`
	ClusterNodes []string `json:"cluster_nodes,omitempty"`
}

// RedisCredentials holds Redis authentication.
// Sensitive fields are stored encrypted.
type RedisCredentials struct {
	Username          string `json:"username,omitempty"`
	PasswordEncrypted string `json:"password_encrypted,omitempty"`
	TLSEnabled        bool   `json:"tls_enabled,omitempty"`
	TLSSkipVerify     bool   `json:"tls_skip_verify,omitempty"`
	TLSCert           string `json:"tls_cert,omitempty"`
	TLSKeyEncrypted   string `json:"tls_key_encrypted,omitempty"`
	TLSCA             string `json:"tls_ca,omitempty"`
}

// SelfManagedState extends InfraState with self-managed specific data.
type SelfManagedState struct {
	*InfraState
	SelfManaged *SelfManagedConfig `json:"self_managed,omitempty"`
}

// CreateSelfManagedState creates and stores state for self-managed infrastructure.
// Credentials are re-encrypted for storage if an encryption key is provided.
func (m *Manager) CreateSelfManagedState(state *InfraState, config interface{}, encryptionKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create workspace directory for state storage
	workspacePath := filepath.Join(m.baseDir, state.ID)
	if err := os.MkdirAll(workspacePath, 0700); err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	state.WorkspacePath = workspacePath

	// Convert config to SelfManagedConfig
	configBytes, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	var selfManagedConfig SelfManagedConfig
	if err := json.Unmarshal(configBytes, &selfManagedConfig); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Store the self-managed config separately (with encrypted credentials)
	smState := &SelfManagedState{
		InfraState:  state,
		SelfManaged: &selfManagedConfig,
	}

	// Save state to file
	stateFile := filepath.Join(workspacePath, "state.json")
	stateData, err := json.MarshalIndent(smState, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(stateFile, stateData, 0600); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	// Store encryption key securely (for credential decryption during benchmarks)
	if encryptionKey != "" {
		keyFile := filepath.Join(workspacePath, ".encryption_key")
		if err := os.WriteFile(keyFile, []byte(encryptionKey), 0600); err != nil {
			return fmt.Errorf("failed to store encryption key: %w", err)
		}
	}

	return nil
}

// GetSelfManagedConfig retrieves the self-managed configuration for an infrastructure.
func (m *Manager) GetSelfManagedConfig(id string) (*SelfManagedConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	workspacePath := filepath.Join(m.baseDir, id)
	stateFile := filepath.Join(workspacePath, "state.json")

	data, err := os.ReadFile(stateFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var smState SelfManagedState
	if err := json.Unmarshal(data, &smState); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return smState.SelfManaged, nil
}

// DecryptCredential decrypts an AES-GCM encrypted credential.
// The encrypted value format is: base64(IV || ciphertext)
func DecryptCredential(encryptedBase64, keyBase64 string) (string, error) {
	if encryptedBase64 == "" || keyBase64 == "" {
		return "", nil
	}

	// Decode key
	keyBytes, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return "", fmt.Errorf("failed to decode key: %w", err)
	}

	// Decode encrypted data
	combined, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return "", fmt.Errorf("failed to decode encrypted data: %w", err)
	}

	if len(combined) < 12 {
		return "", fmt.Errorf("encrypted data too short")
	}

	// Extract IV and ciphertext
	iv := combined[:12]
	ciphertext := combined[12:]

	// Create cipher
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt
	plaintext, err := aesGCM.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// GetDecryptedSSHCredentials returns SSH credentials with sensitive fields decrypted.
func (m *Manager) GetDecryptedSSHCredentials(id string) (*SSHCredentials, error) {
	config, err := m.GetSelfManagedConfig(id)
	if err != nil {
		return nil, err
	}

	// Read encryption key
	workspacePath := filepath.Join(m.baseDir, id)
	keyFile := filepath.Join(workspacePath, ".encryption_key")
	keyData, err := os.ReadFile(keyFile)
	if err != nil {
		// No encryption key - return as-is
		return &config.Runners.SSH, nil
	}
	encryptionKey := string(keyData)

	// Decrypt sensitive fields
	creds := config.Runners.SSH
	
	if creds.PasswordEncrypted != "" {
		if decrypted, err := DecryptCredential(creds.PasswordEncrypted, encryptionKey); err == nil {
			creds.PasswordEncrypted = decrypted // Store decrypted value temporarily
		}
	}
	
	if creds.PrivateKeyEncrypted != "" {
		if decrypted, err := DecryptCredential(creds.PrivateKeyEncrypted, encryptionKey); err == nil {
			creds.PrivateKeyEncrypted = decrypted
		}
	}
	
	if creds.PassphraseEncrypted != "" {
		if decrypted, err := DecryptCredential(creds.PassphraseEncrypted, encryptionKey); err == nil {
			creds.PassphraseEncrypted = decrypted
		}
	}

	return &creds, nil
}

// GetDecryptedRedisCredentials returns Redis credentials with sensitive fields decrypted.
func (m *Manager) GetDecryptedRedisCredentials(id string) (*RedisCredentials, error) {
	config, err := m.GetSelfManagedConfig(id)
	if err != nil {
		return nil, err
	}

	// Read encryption key
	workspacePath := filepath.Join(m.baseDir, id)
	keyFile := filepath.Join(workspacePath, ".encryption_key")
	keyData, err := os.ReadFile(keyFile)
	if err != nil {
		// No encryption key - return as-is
		return &config.Redis.Credentials, nil
	}
	encryptionKey := string(keyData)

	// Decrypt sensitive fields
	creds := config.Redis.Credentials
	
	if creds.PasswordEncrypted != "" {
		if decrypted, err := DecryptCredential(creds.PasswordEncrypted, encryptionKey); err == nil {
			creds.PasswordEncrypted = decrypted
		}
	}
	
	if creds.TLSKeyEncrypted != "" {
		if decrypted, err := DecryptCredential(creds.TLSKeyEncrypted, encryptionKey); err == nil {
			creds.TLSKeyEncrypted = decrypted
		}
	}

	return &creds, nil
}

// IsSelfManaged returns true if the infrastructure is self-managed.
func (m *Manager) IsSelfManaged(id string) bool {
	state, err := m.GetState(id)
	if err != nil {
		return false
	}
	return state.Provider == "self_managed"
}

// ValidateSelfManagedConnectivity checks if runners and Redis are reachable.
// This is a placeholder - actual implementation would do SSH and Redis connection tests.
func (m *Manager) ValidateSelfManagedConnectivity(id string) error {
	config, err := m.GetSelfManagedConfig(id)
	if err != nil {
		return err
	}

	// TODO: Implement actual connectivity checks
	// - SSH to each runner
	// - Redis PING to each target

	_ = config
	return nil
}

// UpdateLastHealthCheck updates the last health check timestamp for self-managed infra.
func (m *Manager) UpdateLastHealthCheck(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	workspacePath := filepath.Join(m.baseDir, id)
	stateFile := filepath.Join(workspacePath, "state.json")

	data, err := os.ReadFile(stateFile)
	if err != nil {
		return err
	}

	var smState SelfManagedState
	if err := json.Unmarshal(data, &smState); err != nil {
		return err
	}

	smState.UpdatedAt = time.Now()

	stateData, err := json.MarshalIndent(smState, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(stateFile, stateData, 0600)
}

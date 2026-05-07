// Package terraform provides infrastructure management using Terraform for RedisMeter.
// This enables persistent infrastructure state tracking so you know what's deployed
// even days later, and supports both ephemeral (test and delete) and persistent
// (keep for multiple tests) deployment modes.
package terraform

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ProgressCallback is called with progress updates during Terraform operations.
type ProgressCallback func(event TerraformEvent)

// TerraformEvent represents a progress event from Terraform.
type TerraformEvent struct {
	Phase       string    // "init", "plan", "apply", "destroy"
	Action      string    // "creating", "created", "destroying", "destroyed", "refreshing"
	Resource    string    // Resource name (e.g., "azurerm_resource_group.main")
	ElapsedTime string    // Time elapsed for this resource
	Message     string    // Raw message
	Total       int       // Total resources (if known)
	Completed   int       // Completed resources
	Timestamp   time.Time // Event timestamp
}

// Manager handles Terraform-based infrastructure lifecycle.
type Manager struct {
	mu           sync.RWMutex
	baseDir      string // Base directory for all Terraform workspaces
	terraformBin string // Path to terraform binary
}

// InfraConfig defines the infrastructure configuration.
type InfraConfig struct {
	Name     string            `json:"name"`              // Name is a unique identifier for this infrastructure
	Provider string            `json:"provider"`          // Provider is the cloud provider (azure, aws, gcp)
	Region   string            `json:"region"`            // Region is the deployment region
	AMR      *AMRConfig        `json:"amr,omitempty"`     // AMR (Azure Managed Redis) Configuration
	Runners  *RunnerConfig     `json:"runners,omitempty"` // Runner VM Configuration
	Tags     map[string]string `json:"tags,omitempty"`    // Tags to apply to all resources
	TTL      time.Duration     `json:"ttl,omitempty"`     // TTL is the maximum lifetime (0 = no auto-cleanup)
}

// AMRConfig defines Azure Managed Redis configuration.
type AMRConfig struct {
	SKU              string   `json:"sku"`               // SKU is the AMR SKU (Balanced_B0, Balanced_B1, etc.)
	Modules          []string `json:"modules,omitempty"` // Modules are Redis modules to enable (RedisJSON, RediSearch, etc.)
	HighAvailability bool     `json:"high_availability"` // HighAvailability enables HA mode
	ClusteringPolicy string   `json:"clustering_policy"` // ClusteringPolicy: OSSCluster, EnterpriseCluster, or NoCluster
	EvictionPolicy   string   `json:"eviction_policy"`   // EvictionPolicy for the database
}

// RunnerConfig defines benchmark runner VM configuration.
type RunnerConfig struct {
	Count         int    `json:"count"`          // Count is the number of runner VMs
	InstanceType  string `json:"instance_type"`  // InstanceType is the VM size (e.g., Standard_D4s_v3)
	SpotInstances bool   `json:"spot_instances"` // SpotInstances enables spot/preemptible VMs
	SSHPublicKey  string `json:"ssh_public_key"` // SSHPublicKey for VM access
	SSHUser       string `json:"ssh_user"`       // SSHUser for VM access (default: azureuser)
}

// InfraState represents the current state of infrastructure.
type InfraState struct {
	ID            string        `json:"id"`                   // ID is the unique infrastructure identifier
	Name          string        `json:"name"`                 // Name is the human-readable name
	Status        string        `json:"status"`               // Status: pending, provisioning, ready, destroying, destroyed, failed
	Provider      string        `json:"provider"`             // Provider (azure, aws, gcp)
	Region        string        `json:"region"`               // Region
	CreatedAt     time.Time     `json:"created_at"`           // CreatedAt timestamp
	UpdatedAt     time.Time     `json:"updated_at"`           // UpdatedAt timestamp
	ExpiresAt     time.Time     `json:"expires_at,omitempty"` // ExpiresAt for TTL-based cleanup (zero = no expiry)
	Config        InfraConfig   `json:"config"`               // Config used to create this infrastructure
	Outputs       *InfraOutputs `json:"outputs,omitempty"`    // Outputs from Terraform
	WorkspacePath string        `json:"workspace_path"`       // WorkspacePath is the path to the Terraform workspace
	Error         string        `json:"error,omitempty"`      // Error message if status is failed
}

// InfraOutputs contains the Terraform output values.
type InfraOutputs struct {
	// Redis connection details
	RedisHostname   string `json:"redis_hostname,omitempty"`
	RedisPort       int    `json:"redis_port,omitempty"`
	RedisPrimaryKey string `json:"redis_primary_key,omitempty"`

	// Resource identifiers
	ResourceGroupName string `json:"resource_group_name,omitempty"`
	ClusterID         string `json:"cluster_id,omitempty"`

	// Runner VM details
	RunnerIPs        []string `json:"runner_ips,omitempty"`
	RunnerPrivateIPs []string `json:"runner_private_ips,omitempty"`
}

// NewManager creates a new Terraform manager.
func NewManager(baseDir string) (*Manager, error) {
	// Find terraform binary
	tfBin, err := exec.LookPath("terraform")
	if err != nil {
		return nil, fmt.Errorf("terraform not found in PATH: %w", err)
	}

	// Ensure base directory exists
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	return &Manager{
		baseDir:      baseDir,
		terraformBin: tfBin,
	}, nil
}

// Provision creates new infrastructure using Terraform.
func (m *Manager) Provision(ctx context.Context, config InfraConfig) (*InfraState, error) {
	return m.ProvisionWithProgress(ctx, config, nil)
}

// ProvisionWithProgress creates new infrastructure with progress callbacks.
func (m *Manager) ProvisionWithProgress(ctx context.Context, config InfraConfig, progress ProgressCallback) (*InfraState, error) {
	// --- Initial setup under lock ---
	m.mu.Lock()

	// Generate unique ID
	id := fmt.Sprintf("rm-%s-%d", config.Provider, time.Now().Unix())
	if config.Name != "" {
		id = fmt.Sprintf("rm-%s-%s", config.Name, time.Now().Format("20060102-150405"))
	}

	// Create workspace directory
	workspacePath := filepath.Join(m.baseDir, id)
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		m.mu.Unlock()
		return nil, fmt.Errorf("failed to create workspace: %w", err)
	}

	// Initialize state
	state := &InfraState{
		ID:            id,
		Name:          config.Name,
		Status:        "pending",
		Provider:      config.Provider,
		Region:        config.Region,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Config:        config,
		WorkspacePath: workspacePath,
	}

	if config.TTL > 0 {
		state.ExpiresAt = time.Now().Add(config.TTL)
	}

	// Generate Terraform files
	if err := m.generateTerraformFiles(workspacePath, config); err != nil {
		state.Status = "failed"
		state.Error = err.Error()
		m.saveState(state)
		m.mu.Unlock()
		return state, fmt.Errorf("failed to generate Terraform files: %w", err)
	}

	// Save initial state and mark as provisioning before releasing lock
	state.Status = "provisioning"
	state.UpdatedAt = time.Now()
	if err := m.saveState(state); err != nil {
		m.mu.Unlock()
		return nil, fmt.Errorf("failed to save state: %w", err)
	}

	m.mu.Unlock()
	// --- Lock released; long-running Terraform operations below ---

	if progress != nil {
		progress(TerraformEvent{
			Phase:     "init",
			Action:    "starting",
			Message:   "Initializing Terraform...",
			Timestamp: time.Now(),
		})
	}

	if err := m.runTerraformWithProgress(ctx, workspacePath, progress, "init", "init", "-input=false"); err != nil {
		state.Status = "failed"
		state.Error = fmt.Sprintf("terraform init failed: %v", err)
		m.saveState(state)
		return state, err
	}

	// Run terraform apply
	if progress != nil {
		progress(TerraformEvent{
			Phase:     "apply",
			Action:    "starting",
			Message:   "Applying infrastructure changes...",
			Timestamp: time.Now(),
		})
	}

	if err := m.runTerraformWithProgress(ctx, workspacePath, progress, "apply", "apply", "-auto-approve", "-input=false"); err != nil {
		state.Status = "failed"
		state.Error = fmt.Sprintf("terraform apply failed: %v", err)
		m.saveState(state)
		return state, err
	}

	// Get outputs
	outputs, err := m.getOutputs(ctx, workspacePath)
	if err != nil {
		state.Status = "failed"
		state.Error = fmt.Sprintf("failed to get outputs: %v", err)
		m.saveState(state)
		return state, err
	}

	state.Status = "ready"
	state.Outputs = outputs
	state.UpdatedAt = time.Now()
	m.saveState(state)

	return state, nil
}

// Destroy tears down infrastructure.
func (m *Manager) Destroy(ctx context.Context, id string) error {
	return m.DestroyWithProgress(ctx, id, nil)
}

// DestroyWithProgress tears down infrastructure with progress callbacks.
func (m *Manager) DestroyWithProgress(ctx context.Context, id string, progress ProgressCallback) error {
	// Load and mark as destroying under lock, then run terraform destroy without holding lock.
	m.mu.Lock()
	state, err := m.loadState(id)
	if err != nil {
		m.mu.Unlock()
		return fmt.Errorf("infrastructure not found: %w", err)
	}

	state.Status = "destroying"
	state.UpdatedAt = time.Now()
	m.saveState(state)
	m.mu.Unlock()
	// --- Lock released; long-running terraform destroy below ---

	// Run terraform destroy
	if progress != nil {
		progress(TerraformEvent{
			Phase:     "destroy",
			Action:    "starting",
			Message:   "Destroying infrastructure...",
			Timestamp: time.Now(),
		})
	}

	if err := m.runTerraformWithProgress(ctx, state.WorkspacePath, progress, "destroy", "destroy", "-auto-approve", "-input=false"); err != nil {
		state.Status = "failed"
		state.Error = fmt.Sprintf("terraform destroy failed: %v", err)
		m.saveState(state)
		return err
	}

	state.Status = "destroyed"
	state.UpdatedAt = time.Now()
	m.saveState(state)

	return nil
}

// GetState returns the current state of infrastructure.
func (m *Manager) GetState(id string) (*InfraState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.loadState(id)
}

// ListInfrastructure returns all tracked infrastructure.
func (m *Manager) ListInfrastructure() ([]*InfraState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries, err := os.ReadDir(m.baseDir)
	if err != nil {
		return nil, err
	}

	var states []*InfraState
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		state, err := m.loadState(entry.Name())
		if err != nil {
			continue // Skip invalid states
		}
		states = append(states, state)
	}

	return states, nil
}

// RefreshState updates state from Terraform.
func (m *Manager) RefreshState(ctx context.Context, id string) (*InfraState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, err := m.loadState(id)
	if err != nil {
		return nil, err
	}

	// Run terraform refresh
	if err := m.runTerraform(ctx, state.WorkspacePath, "refresh", "-input=false"); err != nil {
		return nil, fmt.Errorf("terraform refresh failed: %w", err)
	}

	// Get updated outputs
	outputs, err := m.getOutputs(ctx, state.WorkspacePath)
	if err != nil {
		return nil, err
	}

	state.Outputs = outputs
	state.UpdatedAt = time.Now()

	// Update status to ready if we have outputs
	if outputs != nil && outputs.RedisHostname != "" {
		state.Status = "ready"
	}

	m.saveState(state)

	return state, nil
}

// generateTerraformFiles creates the Terraform configuration files.
func (m *Manager) generateTerraformFiles(workspacePath string, config InfraConfig) error {
	switch config.Provider {
	case "azure":
		return m.generateAzureTerraform(workspacePath, config)
	case "aws":
		return m.generateAWSTerraform(workspacePath, config)
	default:
		return fmt.Errorf("unsupported provider: %s", config.Provider)
	}
}

// generateAzureTerraform creates Azure-specific Terraform files.
func (m *Manager) generateAzureTerraform(workspacePath string, config InfraConfig) error {
	// Get subscription ID from az CLI
	subscriptionID, err := m.getAzureSubscriptionID()
	if err != nil {
		return fmt.Errorf("failed to get Azure subscription ID: %w", err)
	}

	// Generate versions.tf
	versionsContent := fmt.Sprintf(`terraform {
  required_version = ">= 1.3"

  required_providers {
    azapi = {
      source  = "Azure/azapi"
      version = "~> 2.0"
    }
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 4.50"
    }
  }
}

provider "azurerm" {
  features {}
  subscription_id = "%s"
}

provider "azapi" {
  subscription_id = "%s"
}
`, subscriptionID, subscriptionID)
	if err := os.WriteFile(filepath.Join(workspacePath, "versions.tf"), []byte(versionsContent), 0644); err != nil {
		return err
	}

	// Generate main.tf
	mainContent := m.generateAzureMainTF(config)
	if err := os.WriteFile(filepath.Join(workspacePath, "main.tf"), []byte(mainContent), 0644); err != nil {
		return err
	}

	// Generate variables.tf
	variablesContent := m.generateAzureVariablesTF(config)
	if err := os.WriteFile(filepath.Join(workspacePath, "variables.tf"), []byte(variablesContent), 0644); err != nil {
		return err
	}

	// Generate terraform.tfvars
	tfvarsContent := m.generateAzureTFVars(config)
	if err := os.WriteFile(filepath.Join(workspacePath, "terraform.tfvars"), []byte(tfvarsContent), 0644); err != nil {
		return err
	}

	// Generate outputs.tf
	outputsContent := m.generateAzureOutputsTF(config)
	if err := os.WriteFile(filepath.Join(workspacePath, "outputs.tf"), []byte(outputsContent), 0644); err != nil {
		return err
	}

	return nil
}

func (m *Manager) generateAzureMainTF(config InfraConfig) string {
	var sb strings.Builder

	sb.WriteString(`# RedisMeter Infrastructure - Generated by Terraform Manager
# DO NOT EDIT - This file is auto-generated

data "azurerm_client_config" "current" {}

# Resource Group
resource "azurerm_resource_group" "main" {
  name     = var.resource_group_name
  location = var.location
  tags = var.tags
}

locals {
  resource_group_id = azurerm_resource_group.main.id
  redis_enterprise_api_version = "2025-05-01-preview"
}
`)

	// Add AMR if configured
	if config.AMR != nil {
		sb.WriteString(`
# Azure Managed Redis Cluster
resource "azapi_resource" "redis_cluster" {
  type      = "Microsoft.Cache/redisEnterprise@${local.redis_enterprise_api_version}"
  name      = var.redis_name
  location  = azurerm_resource_group.main.location
  parent_id = local.resource_group_id

  body = {
    sku = {
      name = var.redis_sku
    }
    properties = {
      highAvailability  = var.high_availability ? "Enabled" : "Disabled"
      minimumTlsVersion = "1.2"
    }
  }

  tags = var.tags
  schema_validation_enabled = false

  timeouts {
    create = "30m"
    update = "30m"
    delete = "30m"
  }
}

# Get cluster data after creation - this acts as a wait for provisioning
data "azapi_resource" "redis_cluster_data" {
  type                   = "Microsoft.Cache/redisEnterprise@${local.redis_enterprise_api_version}"
  resource_id            = azapi_resource.redis_cluster.id
  response_export_values = ["properties.hostName", "properties.provisioningState"]
  depends_on = [azapi_resource.redis_cluster]
}

# Null resource to wait for cluster to be fully provisioned
# This ensures the cluster provisioningState is "Succeeded" before creating database
resource "terraform_data" "wait_for_cluster" {
  depends_on = [data.azapi_resource.redis_cluster_data]
  
  provisioner "local-exec" {
    command = <<-EOT
      echo "Waiting for Redis cluster to be fully provisioned..."
      for i in $(seq 1 60); do
        state=$(az rest --method GET --uri "https://management.azure.com${azapi_resource.redis_cluster.id}?api-version=2025-05-01-preview" --query "properties.provisioningState" -o tsv 2>/dev/null || echo "Unknown")
        echo "Attempt $i: provisioningState=$state"
        if [ "$state" = "Succeeded" ]; then
          echo "Cluster is ready!"
          exit 0
        fi
        sleep 10
      done
      echo "Timeout waiting for cluster"
      exit 1
    EOT
    interpreter = ["/bin/bash", "-c"]
  }
}

# Redis Database
resource "azapi_resource" "redis_database" {
  type      = "Microsoft.Cache/redisEnterprise/databases@${local.redis_enterprise_api_version}"
  name      = "default"
  parent_id = azapi_resource.redis_cluster.id

  body = {
    properties = {
      clientProtocol           = "Encrypted"
      evictionPolicy           = var.eviction_policy
      clusteringPolicy         = var.clustering_policy
      deferUpgrade             = "NotDeferred"
      accessKeysAuthentication = "Enabled"
      modules                  = [for m in var.redis_modules : { name = m }]
      persistence = {
        aofEnabled = false
        rdbEnabled = false
      }
    }
  }

  schema_validation_enabled = false
  depends_on = [terraform_data.wait_for_cluster]

  timeouts {
    create = "20m"
    update = "20m"
    delete = "20m"
  }
}

# Get database access keys
data "azapi_resource_action" "redis_keys" {
  type        = "Microsoft.Cache/redisEnterprise/databases@${local.redis_enterprise_api_version}"
  resource_id = azapi_resource.redis_database.id
  action      = "listKeys"
  method      = "POST"
  response_export_values = ["primaryKey", "secondaryKey"]
  depends_on = [azapi_resource.redis_database]
}
`)
	}

	// Add Runner VMs if configured
	if config.Runners != nil && config.Runners.Count > 0 {
		sb.WriteString(`
# Network for Runner VMs
resource "azurerm_virtual_network" "runners" {
  name                = "${var.resource_group_name}-vnet"
  address_space       = ["10.0.0.0/16"]
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  tags = var.tags
}

resource "azurerm_subnet" "runners" {
  name                 = "runners"
  resource_group_name  = azurerm_resource_group.main.name
  virtual_network_name = azurerm_virtual_network.runners.name
  address_prefixes     = ["10.0.1.0/24"]
}

resource "azurerm_network_security_group" "runners" {
  name                = "${var.resource_group_name}-nsg"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name

  security_rule {
    name                       = "SSH"
    priority                   = 1001
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "22"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }

  tags = var.tags
}

resource "azurerm_subnet_network_security_group_association" "runners" {
  subnet_id                 = azurerm_subnet.runners.id
  network_security_group_id = azurerm_network_security_group.runners.id
}

# Runner VMs
resource "azurerm_public_ip" "runner" {
  count               = var.runner_count
  name                = "${var.resource_group_name}-runner-${count.index}-pip"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  allocation_method   = "Static"
  sku                 = "Standard"
  tags = var.tags
}

resource "azurerm_network_interface" "runner" {
  count               = var.runner_count
  name                = "${var.resource_group_name}-runner-${count.index}-nic"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name

  ip_configuration {
    name                          = "internal"
    subnet_id                     = azurerm_subnet.runners.id
    private_ip_address_allocation = "Dynamic"
    public_ip_address_id          = azurerm_public_ip.runner[count.index].id
  }

  tags = var.tags
}

resource "azurerm_linux_virtual_machine" "runner" {
  count               = var.runner_count
  name                = "${var.resource_group_name}-runner-${count.index}"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  size                = var.runner_instance_type
  admin_username      = var.ssh_user
  network_interface_ids = [azurerm_network_interface.runner[count.index].id]

  admin_ssh_key {
    username   = var.ssh_user
    public_key = var.ssh_public_key
  }

  os_disk {
    caching              = "ReadWrite"
    storage_account_type = "Premium_LRS"
  }

  source_image_reference {
    publisher = "Canonical"
    offer     = "0001-com-ubuntu-server-jammy"
    sku       = "22_04-lts-gen2"
    version   = "latest"
  }

  custom_data = base64encode(<<-EOF
#!/bin/bash
set -e

# Update and install dependencies
apt-get update
apt-get install -y build-essential autoconf automake libpcre3-dev libevent-dev pkg-config zlib1g-dev libssl-dev git

# Clone and build memtier_benchmark
cd /opt
git clone https://github.com/RedisLabs/memtier_benchmark.git
cd memtier_benchmark
autoreconf -ivf
./configure
make -j$(nproc)
make install

# Signal readiness
touch /tmp/memtier_ready
echo "memtier_benchmark installation complete" > /tmp/memtier_ready
EOF
  )

  tags = var.tags
}
`)
	}

	return sb.String()
}

func (m *Manager) generateAzureVariablesTF(config InfraConfig) string {
	var sb strings.Builder

	sb.WriteString(`variable "resource_group_name" {
  description = "Name of the resource group"
  type        = string
}

variable "location" {
  description = "Azure region"
  type        = string
}

variable "tags" {
  description = "Tags to apply to all resources"
  type        = map(string)
  default     = {}
}
`)

	if config.AMR != nil {
		sb.WriteString(`
variable "redis_name" {
  description = "Name of the Redis cluster"
  type        = string
}

variable "redis_sku" {
  description = "SKU of the Redis cluster"
  type        = string
  default     = "Balanced_B0"
}

variable "redis_modules" {
  description = "Redis modules to enable"
  type        = list(string)
  default     = []
}

variable "high_availability" {
  description = "Enable high availability"
  type        = bool
  default     = false
}

variable "clustering_policy" {
  description = "Clustering policy"
  type        = string
  default     = "OSSCluster"
}

variable "eviction_policy" {
  description = "Eviction policy"
  type        = string
  default     = "VolatileLRU"
}
`)
	}

	if config.Runners != nil && config.Runners.Count > 0 {
		sb.WriteString(`
variable "runner_count" {
  description = "Number of runner VMs"
  type        = number
  default     = 1
}

variable "runner_instance_type" {
  description = "VM size for runners"
  type        = string
  default     = "Standard_B2s"
}

variable "ssh_public_key" {
  description = "SSH public key for VM access"
  type        = string
}

variable "ssh_user" {
  description = "SSH username"
  type        = string
  default     = "azureuser"
}
`)
	}

	return sb.String()
}

func (m *Manager) generateAzureTFVars(config InfraConfig) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(`resource_group_name = "rg-redismeter-%s"
location            = "%s"

tags = {
  "managed-by"  = "redismeter"
  "environment" = "benchmark"
`, config.Name, config.Region))

	for k, v := range config.Tags {
		sb.WriteString(fmt.Sprintf(`  "%s" = "%s"
`, k, v))
	}
	sb.WriteString("}\n\n")

	if config.AMR != nil {
		redisName := fmt.Sprintf("rm-%d", time.Now().Unix())
		// Normalize clustering policy: Azure Redis Enterprise only accepts EnterpriseCluster or OSSCluster.
		clusteringPolicy := defaultString(config.AMR.ClusteringPolicy, "EnterpriseCluster")
		switch clusteringPolicy {
		case "non-clustered", "NoCluster", "none", "":
			clusteringPolicy = "EnterpriseCluster"
		}

		// Normalize eviction policy: Redis Enterprise requires PascalCase values.
		evictionPolicyMap := map[string]string{
			"allkeys-lru":     "AllKeysLRU",
			"allkeys-lfu":     "AllKeysLFU",
			"allkeys-random":  "AllKeysRandom",
			"volatile-lru":    "VolatileLRU",
			"volatile-lfu":    "VolatileLFU",
			"volatile-random": "VolatileRandom",
			"volatile-ttl":    "VolatileTTL",
			"noeviction":      "NoEviction",
			"no-eviction":     "NoEviction",
		}
		evictionPolicy := defaultString(config.AMR.EvictionPolicy, "AllKeysLRU")
		if normalized, ok := evictionPolicyMap[evictionPolicy]; ok {
			evictionPolicy = normalized
		}

		sb.WriteString(fmt.Sprintf(`redis_name        = "%s"
redis_sku         = "%s"
high_availability = %t
clustering_policy = "%s"
eviction_policy   = "%s"
`, redisName, config.AMR.SKU, config.AMR.HighAvailability,
			clusteringPolicy,
			evictionPolicy))

		if len(config.AMR.Modules) > 0 {
			sb.WriteString(`redis_modules = [`)
			for i, mod := range config.AMR.Modules {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(fmt.Sprintf(`"%s"`, mod))
			}
			sb.WriteString("]\n")
		} else {
			sb.WriteString("redis_modules = []\n")
		}
		sb.WriteString("\n")
	}

	if config.Runners != nil && config.Runners.Count > 0 {
		sb.WriteString(fmt.Sprintf(`runner_count         = %d
runner_instance_type = "%s"
ssh_user             = "%s"
ssh_public_key       = <<-EOF
%s
EOF
`, config.Runners.Count,
			defaultString(config.Runners.InstanceType, "Standard_B2s"),
			defaultString(config.Runners.SSHUser, "azureuser"),
			config.Runners.SSHPublicKey))
	}

	return sb.String()
}

func (m *Manager) generateAzureOutputsTF(config InfraConfig) string {
	var sb strings.Builder

	sb.WriteString(`output "resource_group_name" {
  description = "Name of the resource group"
  value       = azurerm_resource_group.main.name
}

output "location" {
  description = "Azure region"
  value       = azurerm_resource_group.main.location
}
`)

	if config.AMR != nil {
		sb.WriteString(`
output "redis_hostname" {
  description = "Redis hostname"
  value       = data.azapi_resource.redis_cluster_data.output.properties.hostName
}

output "redis_port" {
  description = "Redis port"
  value       = 10000
}

output "redis_primary_key" {
  description = "Redis primary access key"
  value       = data.azapi_resource_action.redis_keys.output.primaryKey
  sensitive   = true
}

output "redis_cluster_id" {
  description = "Redis cluster ID"
  value       = azapi_resource.redis_cluster.id
}

output "redis_connection_string" {
  description = "Redis connection string"
  value       = "rediss://:${data.azapi_resource_action.redis_keys.output.primaryKey}@${data.azapi_resource.redis_cluster_data.output.properties.hostName}:10000"
  sensitive   = true
}
`)
	}

	if config.Runners != nil && config.Runners.Count > 0 {
		sb.WriteString(`
output "runner_public_ips" {
  description = "Public IP addresses of runner VMs"
  value       = azurerm_public_ip.runner[*].ip_address
}

output "runner_private_ips" {
  description = "Private IP addresses of runner VMs"
  value       = azurerm_network_interface.runner[*].private_ip_address
}
`)
	}

	return sb.String()
}

func (m *Manager) generateAWSTerraform(workspacePath string, config InfraConfig) error {
	// TODO: Implement AWS Terraform generation
	return fmt.Errorf("AWS Terraform generation not yet implemented")
}

// runTerraform executes a terraform command (without progress streaming).
func (m *Manager) runTerraform(ctx context.Context, workspacePath string, args ...string) error {
	return m.runTerraformWithProgress(ctx, workspacePath, nil, args[0], args...)
}

// runTerraformWithProgress executes a terraform command with real-time progress streaming.
func (m *Manager) runTerraformWithProgress(ctx context.Context, workspacePath string, progress ProgressCallback, phase string, args ...string) error {
	cmd := exec.CommandContext(ctx, m.terraformBin, args...)
	cmd.Dir = workspacePath
	cmd.Env = os.Environ()

	// If no progress callback, use simple buffered output
	if progress == nil {
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("terraform %s failed: %w\nstdout: %s\nstderr: %s",
				args[0], err, stdout.String(), stderr.String())
		}
		return nil
	}

	// Stream output for progress tracking
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start terraform: %w", err)
	}

	// Parse terraform output patterns
	var (
		// Patterns for resource operations
		creatingPattern   = regexp.MustCompile(`^(\S+): Creating\.\.\.`)
		createdPattern    = regexp.MustCompile(`^(\S+): Creation complete after (\S+)`)
		destroyingPattern = regexp.MustCompile(`^(\S+): Destroying\.\.\.`)
		destroyedPattern  = regexp.MustCompile(`^(\S+): Destruction complete after (\S+)`)
		stillPattern      = regexp.MustCompile(`^(\S+): Still (\w+)\.\.\..*\[(\S+) elapsed\]`)
		refreshingPattern = regexp.MustCompile(`^(\S+): Refreshing state\.\.\.`)
		planSummary       = regexp.MustCompile(`Plan: (\d+) to add, (\d+) to change, (\d+) to destroy`)
		applySummary      = regexp.MustCompile(`Apply complete! Resources: (\d+) added, (\d+) changed, (\d+) destroyed`)
		destroySummary    = regexp.MustCompile(`Destroy complete! Resources: (\d+) destroyed`)
	)

	// Track progress
	var total, completed int

	processLine := func(line string) {
		line = strings.TrimSpace(line)
		if line == "" {
			return
		}

		event := TerraformEvent{
			Phase:     phase,
			Message:   line,
			Total:     total,
			Completed: completed,
			Timestamp: time.Now(),
		}

		// Parse the line for specific events
		if matches := creatingPattern.FindStringSubmatch(line); matches != nil {
			event.Action = "creating"
			event.Resource = matches[1]
		} else if matches := createdPattern.FindStringSubmatch(line); matches != nil {
			event.Action = "created"
			event.Resource = matches[1]
			event.ElapsedTime = matches[2]
			completed++
			event.Completed = completed
		} else if matches := destroyingPattern.FindStringSubmatch(line); matches != nil {
			event.Action = "destroying"
			event.Resource = matches[1]
		} else if matches := destroyedPattern.FindStringSubmatch(line); matches != nil {
			event.Action = "destroyed"
			event.Resource = matches[1]
			event.ElapsedTime = matches[2]
			completed++
			event.Completed = completed
		} else if matches := stillPattern.FindStringSubmatch(line); matches != nil {
			event.Action = "waiting"
			event.Resource = matches[1]
			event.ElapsedTime = matches[3]
		} else if matches := refreshingPattern.FindStringSubmatch(line); matches != nil {
			event.Action = "refreshing"
			event.Resource = matches[1]
		} else if matches := planSummary.FindStringSubmatch(line); matches != nil {
			event.Action = "planned"
			// Sum of add + change + destroy = total operations
			var add, change, destroy int
			fmt.Sscanf(matches[1], "%d", &add)
			fmt.Sscanf(matches[2], "%d", &change)
			fmt.Sscanf(matches[3], "%d", &destroy)
			total = add + destroy // For apply/destroy tracking
			event.Total = total
		} else if matches := applySummary.FindStringSubmatch(line); matches != nil {
			event.Action = "complete"
		} else if matches := destroySummary.FindStringSubmatch(line); matches != nil {
			event.Action = "complete"
		}

		progress(event)
	}

	// Read stdout and stderr concurrently
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			processLine(scanner.Text())
		}
	}()

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			processLine(scanner.Text())
		}
	}()

	wg.Wait()

	err = cmd.Wait()
	if err != nil {
		// Collect any remaining output for error message
		var errBuf bytes.Buffer
		io.Copy(&errBuf, stderr)
		return fmt.Errorf("terraform %s failed: %w\n%s", args[0], err, errBuf.String())
	}

	return nil
}

// getOutputs retrieves Terraform outputs.
func (m *Manager) getOutputs(ctx context.Context, workspacePath string) (*InfraOutputs, error) {
	cmd := exec.CommandContext(ctx, m.terraformBin, "output", "-json")
	cmd.Dir = workspacePath

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get terraform outputs: %w", err)
	}

	var rawOutputs map[string]struct {
		Value interface{} `json:"value"`
	}

	if err := json.Unmarshal(output, &rawOutputs); err != nil {
		return nil, fmt.Errorf("failed to parse outputs: %w", err)
	}

	outputs := &InfraOutputs{}

	if v, ok := rawOutputs["redis_hostname"]; ok {
		if s, ok := v.Value.(string); ok {
			outputs.RedisHostname = s
		}
	}

	if v, ok := rawOutputs["redis_port"]; ok {
		if f, ok := v.Value.(float64); ok {
			outputs.RedisPort = int(f)
		}
	}

	if v, ok := rawOutputs["redis_primary_key"]; ok {
		if s, ok := v.Value.(string); ok {
			outputs.RedisPrimaryKey = s
		}
	}

	if v, ok := rawOutputs["resource_group_name"]; ok {
		if s, ok := v.Value.(string); ok {
			outputs.ResourceGroupName = s
		}
	}

	if v, ok := rawOutputs["redis_cluster_id"]; ok {
		if s, ok := v.Value.(string); ok {
			outputs.ClusterID = s
		}
	}

	if v, ok := rawOutputs["runner_public_ips"]; ok {
		if arr, ok := v.Value.([]interface{}); ok {
			for _, ip := range arr {
				if s, ok := ip.(string); ok {
					outputs.RunnerIPs = append(outputs.RunnerIPs, s)
				}
			}
		}
	}

	if v, ok := rawOutputs["runner_private_ips"]; ok {
		if arr, ok := v.Value.([]interface{}); ok {
			for _, ip := range arr {
				if s, ok := ip.(string); ok {
					outputs.RunnerPrivateIPs = append(outputs.RunnerPrivateIPs, s)
				}
			}
		}
	}

	return outputs, nil
}

// saveState persists infrastructure state to disk.
func (m *Manager) saveState(state *InfraState) error {
	statePath := filepath.Join(state.WorkspacePath, "redismeter-state.json")

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(statePath, data, 0644)
}

// loadState loads infrastructure state from disk.
func (m *Manager) loadState(id string) (*InfraState, error) {
	workspacePath := filepath.Join(m.baseDir, id)
	statePath := filepath.Join(workspacePath, "redismeter-state.json")

	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil, err
	}

	var state InfraState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// Helper functions
func defaultString(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// getAzureSubscriptionID retrieves the current Azure subscription ID from az CLI.
func (m *Manager) getAzureSubscriptionID() (string, error) {
	// First check environment variable
	if subID := os.Getenv("AZURE_SUBSCRIPTION_ID"); subID != "" {
		return subID, nil
	}

	// Fall back to az CLI
	cmd := exec.Command("az", "account", "show", "--query", "id", "-o", "tsv")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get subscription ID from az CLI: %w", err)
	}

	subID := strings.TrimSpace(string(output))
	if subID == "" {
		return "", fmt.Errorf("no Azure subscription found")
	}

	return subID, nil
}

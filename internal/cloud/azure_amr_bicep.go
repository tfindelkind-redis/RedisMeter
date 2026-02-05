// Package cloud provides Azure Managed Redis (AMR) provisioning using Bicep templates.
// This file contains the Bicep-based deployment approach for AMR.
package cloud

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

// Embed Bicep templates
// Note: Bicep files are compiled to ARM JSON at build time using `az bicep build`
// For now, we embed the JSON directly. In production, add a build step.
//
//go:embed bicep/amr-deployment.json
var amrDeploymentTemplate string

// AMRBicepDeployer handles AMR deployment via Bicep/ARM templates.
// This approach is more robust than raw HTTP calls because:
// - Azure handles dependency ordering (cluster -> database -> private endpoint)
// - Idempotent deployments with proper state tracking
// - Better error messages from ARM
// - Handles race conditions between cluster and database creation
type AMRBicepDeployer struct {
	subscriptionID    string
	credential        *azidentity.DefaultAzureCredential
	deploymentsClient *armresources.DeploymentsClient
}

// NewAMRBicepDeployer creates a new Bicep-based AMR deployer.
func NewAMRBicepDeployer(subscriptionID string) (*AMRBicepDeployer, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get Azure credentials: %w", err)
	}

	deploymentsClient, err := armresources.NewDeploymentsClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create deployments client: %w", err)
	}

	return &AMRBicepDeployer{
		subscriptionID:    subscriptionID,
		credential:        cred,
		deploymentsClient: deploymentsClient,
	}, nil
}

// AMRBicepDeploymentParams contains parameters for Bicep-based AMR deployment.
type AMRBicepDeploymentParams struct {
	// Redis configuration
	RedisName string `json:"redisName"`
	Location  string `json:"location"`
	SKUName   string `json:"skuName"`

	// Cluster settings
	ClusteringPolicy string `json:"clusteringPolicy"` // OSSCluster, EnterpriseCluster
	HighAvailability bool   `json:"highAvailability"`
	EvictionPolicy   string `json:"evictionPolicy"`

	// Persistence
	RDBEnabled   bool   `json:"rdbEnabled"`
	RDBFrequency string `json:"rdbFrequency"` // 1h, 6h, 12h
	AOFEnabled   bool   `json:"aofEnabled"`
	AOFFrequency string `json:"aofFrequency"` // 1s, always

	// Modules
	EnableRediSearch      bool `json:"enableRediSearch"`
	EnableRedisJSON       bool `json:"enableRedisJSON"`
	EnableRedisTimeSeries bool `json:"enableRedisTimeSeries"`
	EnableRedisBloom      bool `json:"enableRedisBloom"`

	// Private endpoint (optional)
	EnablePrivateEndpoint bool   `json:"enablePrivateEndpoint"`
	VNetID                string `json:"vnetId"`
	SubnetID              string `json:"subnetId"`

	// Tags
	Tags map[string]string `json:"tags"`
}

// AMRBicepDeploymentResult contains the results from a Bicep deployment.
type AMRBicepDeploymentResult struct {
	RedisID           string `json:"redisId"`
	RedisName         string `json:"redisName"`
	RedisHostName     string `json:"redisHostName"`
	RedisDatabaseID   string `json:"redisDatabaseId"`
	RedisPort         int    `json:"redisPort"`
	RedisPrimaryKey   string `json:"redisPrimaryKey"`
	RedisSecondaryKey string `json:"redisSecondaryKey"`
	PrivateEndpointID string `json:"privateEndpointId,omitempty"`
	PrivateEndpointIP string `json:"privateEndpointIp,omitempty"`
}

// DeployAMR deploys Azure Managed Redis using Bicep templates.
// This is the recommended approach for production deployments.
func (d *AMRBicepDeployer) DeployAMR(ctx context.Context, resourceGroup string, params *AMRBicepDeploymentParams) (*AMRBicepDeploymentResult, error) {
	deploymentName := fmt.Sprintf("amr-%s-%d", params.RedisName, time.Now().Unix())

	// Build ARM template parameters - only include non-empty values
	// Empty strings would fail ARM template validation
	armParams := map[string]interface{}{
		"redisName":        map[string]interface{}{"value": params.RedisName},
		"location":         map[string]interface{}{"value": params.Location},
		"skuName":          map[string]interface{}{"value": params.SKUName},
		"clusteringPolicy": map[string]interface{}{"value": params.ClusteringPolicy},
		"highAvailability": map[string]interface{}{"value": params.HighAvailability},
	}

	// Only add evictionPolicy if set
	if params.EvictionPolicy != "" {
		armParams["evictionPolicy"] = map[string]interface{}{"value": params.EvictionPolicy}
	}

	// RDB persistence - only add if explicitly enabled
	armParams["rdbEnabled"] = map[string]interface{}{"value": params.RDBEnabled}
	if params.RDBEnabled && params.RDBFrequency != "" {
		armParams["rdbFrequency"] = map[string]interface{}{"value": params.RDBFrequency}
	}

	// AOF persistence - only add if explicitly enabled
	armParams["aofEnabled"] = map[string]interface{}{"value": params.AOFEnabled}
	if params.AOFEnabled && params.AOFFrequency != "" {
		armParams["aofFrequency"] = map[string]interface{}{"value": params.AOFFrequency}
	}

	// Module flags
	armParams["enableRediSearch"] = map[string]interface{}{"value": params.EnableRediSearch}
	armParams["enableRedisJSON"] = map[string]interface{}{"value": params.EnableRedisJSON}
	armParams["enableRedisTimeSeries"] = map[string]interface{}{"value": params.EnableRedisTimeSeries}
	armParams["enableRedisBloom"] = map[string]interface{}{"value": params.EnableRedisBloom}

	// Private endpoint - only add if enabled with valid subnet
	armParams["enablePrivateEndpoint"] = map[string]interface{}{"value": params.EnablePrivateEndpoint}
	if params.EnablePrivateEndpoint {
		if params.VNetID != "" {
			armParams["vnetId"] = map[string]interface{}{"value": params.VNetID}
		}
		if params.SubnetID != "" {
			armParams["subnetId"] = map[string]interface{}{"value": params.SubnetID}
		}
	}

	// Tags
	if params.Tags != nil {
		armParams["tags"] = map[string]interface{}{"value": params.Tags}
	} else {
		armParams["tags"] = map[string]interface{}{"value": map[string]string{}}
	}

	// Parse the embedded template
	var template map[string]interface{}
	if err := json.Unmarshal([]byte(amrDeploymentTemplate), &template); err != nil {
		return nil, fmt.Errorf("failed to parse ARM template: %w", err)
	}

	// Start deployment
	poller, err := d.deploymentsClient.BeginCreateOrUpdate(ctx, resourceGroup, deploymentName,
		armresources.Deployment{
			Properties: &armresources.DeploymentProperties{
				Template:   template,
				Parameters: armParams,
				Mode:       to.Ptr(armresources.DeploymentModeIncremental),
			},
		}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start deployment: %w", err)
	}

	// Wait for deployment to complete (AMR takes 10-20 minutes)
	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("deployment failed: %w", err)
	}

	// Extract outputs from deployment
	return extractDeploymentOutputs(result.Properties.Outputs)
}

// extractDeploymentOutputs extracts the outputs from an ARM deployment result.
func extractDeploymentOutputs(outputs interface{}) (*AMRBicepDeploymentResult, error) {
	outputMap, ok := outputs.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected outputs format")
	}

	result := &AMRBicepDeploymentResult{}

	// Helper to extract output value
	extractValue := func(key string) string {
		if v, ok := outputMap[key]; ok {
			if vmap, ok := v.(map[string]interface{}); ok {
				if value, ok := vmap["value"]; ok {
					if s, ok := value.(string); ok {
						return s
					}
				}
			}
		}
		return ""
	}

	extractInt := func(key string) int {
		if v, ok := outputMap[key]; ok {
			if vmap, ok := v.(map[string]interface{}); ok {
				if value, ok := vmap["value"]; ok {
					switch n := value.(type) {
					case float64:
						return int(n)
					case int:
						return n
					}
				}
			}
		}
		return 0
	}

	result.RedisID = extractValue("redisId")
	result.RedisName = extractValue("redisName")
	result.RedisHostName = extractValue("redisHostName")
	result.RedisDatabaseID = extractValue("redisDatabaseId")
	result.RedisPort = extractInt("redisPort")
	result.RedisPrimaryKey = extractValue("redisPrimaryKey")
	result.RedisSecondaryKey = extractValue("redisSecondaryKey")
	result.PrivateEndpointID = extractValue("privateEndpointId")
	result.PrivateEndpointIP = extractValue("privateEndpointIP")

	// Default port if not set
	if result.RedisPort == 0 {
		result.RedisPort = 10000
	}

	return result, nil
}

// DeleteAMRDeployment deletes an AMR deployment and all associated resources.
func (d *AMRBicepDeployer) DeleteAMRDeployment(ctx context.Context, resourceGroup, deploymentName string) error {
	poller, err := d.deploymentsClient.BeginDelete(ctx, resourceGroup, deploymentName, nil)
	if err != nil {
		return fmt.Errorf("failed to start deletion: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("deletion failed: %w", err)
	}

	return nil
}

// ConvertTemplateToParams converts an AMRTemplateSpec and SKU to Bicep deployment parameters.
func ConvertTemplateToParams(name, location string, template AMRTemplate, sku string, network *AMRNetworkConfig) (*AMRBicepDeploymentParams, error) {
	// Get template spec
	templates := GetAMRTemplates()
	spec, ok := templates[template]
	if !ok {
		return nil, fmt.Errorf("unknown template: %s", template)
	}

	// Validate SKU
	if err := ValidateAMRSKU(sku); err != nil {
		return nil, err
	}

	params := &AMRBicepDeploymentParams{
		RedisName:        name,
		Location:         location,
		SKUName:          sku,
		ClusteringPolicy: spec.ClusteringPolicy,
		HighAvailability: spec.HighAvailability,
		EvictionPolicy:   spec.EvictionPolicy,
		Tags: map[string]string{
			"managed-by": "redismeter",
			"template":   string(template),
			"sku":        sku,
		},
	}

	// Persistence
	if spec.PersistenceType == "rdb" {
		params.RDBEnabled = true
		params.RDBFrequency = spec.RDBFrequency
	} else if spec.PersistenceType == "aof" {
		params.AOFEnabled = true
		params.AOFFrequency = spec.AOFFrequency
	}

	// Modules
	for _, mod := range spec.Modules {
		switch mod {
		case "RediSearch":
			params.EnableRediSearch = true
		case "RedisJSON":
			params.EnableRedisJSON = true
		case "RedisTimeSeries":
			params.EnableRedisTimeSeries = true
		case "RedisBloom":
			params.EnableRedisBloom = true
		}
	}

	// Network
	if network != nil {
		params.EnablePrivateEndpoint = network.CreatePrivateEndpoint
		params.VNetID = network.ExistingVNetID
		// SubnetID would need to be derived from VNetID + SubnetName
	}

	return params, nil
}

// GetDeploymentStatus retrieves the status of an ongoing deployment.
func (d *AMRBicepDeployer) GetDeploymentStatus(ctx context.Context, resourceGroup, deploymentName string) (string, error) {
	result, err := d.deploymentsClient.Get(ctx, resourceGroup, deploymentName, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get deployment: %w", err)
	}

	if result.Properties.ProvisioningState != nil {
		return string(*result.Properties.ProvisioningState), nil
	}

	return "Unknown", nil
}

// ListDeployments lists all RedisMeter deployments in a resource group.
func (d *AMRBicepDeployer) ListDeployments(ctx context.Context, resourceGroup string) ([]string, error) {
	var deployments []string

	pager := d.deploymentsClient.NewListByResourceGroupPager(resourceGroup, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list deployments: %w", err)
		}

		for _, deployment := range page.Value {
			// Filter to only RedisMeter deployments
			if deployment.Name != nil {
				name := *deployment.Name
				if len(name) > 4 && name[:4] == "amr-" {
					deployments = append(deployments, name)
				}
			}
		}
	}

	return deployments, nil
}

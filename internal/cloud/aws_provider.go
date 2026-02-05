// Package cloud provides cloud provider infrastructure management for RedisMeter.
package cloud

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	"github.com/google/uuid"
	"github.com/tfindelkind-redis/redismeter/internal/plugin"
)

// AWSProvider implements cloud provider operations for AWS.
type AWSProvider struct {
	mu            sync.RWMutex
	ec2Client     *ec2.Client
	pricingClient *pricing.Client
	region        string
	infras        map[string]*Infrastructure
	config        AWSConfig
}

// AWSConfig holds AWS provider configuration.
type AWSConfig struct {
	// Region is the default AWS region.
	Region string `json:"region,omitempty"`

	// Profile is the AWS credentials profile to use.
	Profile string `json:"profile,omitempty"`

	// AccessKeyID for explicit credentials.
	AccessKeyID string `json:"access_key_id,omitempty"`

	// SecretAccessKey for explicit credentials.
	SecretAccessKey string `json:"secret_access_key,omitempty"`

	// DefaultAMI is the default AMI to use (Ubuntu by default).
	DefaultAMI string `json:"default_ami,omitempty"`

	// DefaultInstanceType is the default instance type.
	DefaultInstanceType string `json:"default_instance_type,omitempty"`

	// DefaultKeyName is the default SSH key name.
	DefaultKeyName string `json:"default_key_name,omitempty"`

	// DefaultVPCID is the default VPC to use.
	DefaultVPCID string `json:"default_vpc_id,omitempty"`

	// DefaultSubnetID is the default subnet to use.
	DefaultSubnetID string `json:"default_subnet_id,omitempty"`

	// Tags to apply to all resources.
	Tags map[string]string `json:"tags,omitempty"`
}

// NewAWSProvider creates a new AWS provider.
func NewAWSProvider() *AWSProvider {
	return &AWSProvider{
		infras: make(map[string]*Infrastructure),
		config: AWSConfig{
			Region:              "us-east-1",
			DefaultInstanceType: "t3.medium",
		},
	}
}

// Metadata returns plugin metadata.
func (p *AWSProvider) Metadata() plugin.Metadata {
	return plugin.Metadata{
		Name:        "aws",
		Version:     "1.0.0",
		Type:        plugin.TypeCloud,
		Description: "AWS cloud provider for EC2 and ElastiCache",
	}
}

// Initialize sets up the AWS provider with credentials.
func (p *AWSProvider) Initialize(ctx context.Context, cfg map[string]interface{}) error {
	// Parse configuration
	if region, ok := cfg["region"].(string); ok {
		p.config.Region = region
	}
	if profile, ok := cfg["profile"].(string); ok {
		p.config.Profile = profile
	}
	if ami, ok := cfg["default_ami"].(string); ok {
		p.config.DefaultAMI = ami
	}
	if instanceType, ok := cfg["default_instance_type"].(string); ok {
		p.config.DefaultInstanceType = instanceType
	}
	if keyName, ok := cfg["default_key_name"].(string); ok {
		p.config.DefaultKeyName = keyName
	}
	if vpcID, ok := cfg["default_vpc_id"].(string); ok {
		p.config.DefaultVPCID = vpcID
	}
	if subnetID, ok := cfg["default_subnet_id"].(string); ok {
		p.config.DefaultSubnetID = subnetID
	}
	if tags, ok := cfg["tags"].(map[string]interface{}); ok {
		p.config.Tags = make(map[string]string)
		for k, v := range tags {
			if s, ok := v.(string); ok {
				p.config.Tags[k] = s
			}
		}
	}

	// Load AWS config
	var opts []func(*config.LoadOptions) error
	opts = append(opts, config.WithRegion(p.config.Region))

	if p.config.Profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(p.config.Profile))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	p.ec2Client = ec2.NewFromConfig(awsCfg)
	p.region = p.config.Region

	// Pricing API is only available in us-east-1
	pricingCfg := awsCfg.Copy()
	pricingCfg.Region = "us-east-1"
	p.pricingClient = pricing.NewFromConfig(pricingCfg)

	return nil
}

// HealthCheck verifies AWS connectivity.
func (p *AWSProvider) HealthCheck(ctx context.Context) plugin.HealthStatus {
	if p.ec2Client == nil {
		return plugin.HealthStatus{
			Healthy: false,
			Message: "AWS client not initialized",
		}
	}

	// Try to describe regions to verify connectivity
	_, err := p.ec2Client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
	if err != nil {
		return plugin.HealthStatus{
			Healthy: false,
			Message: fmt.Sprintf("AWS connectivity failed: %v", err),
		}
	}

	return plugin.HealthStatus{
		Healthy: true,
		Message: fmt.Sprintf("AWS provider ready (region: %s)", p.region),
	}
}

// Shutdown cleans up AWS resources.
func (p *AWSProvider) Shutdown(ctx context.Context) error {
	return nil
}

// ListRegions returns available AWS regions.
func (p *AWSProvider) ListRegions(ctx context.Context) ([]Region, error) {
	output, err := p.ec2Client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe regions: %w", err)
	}

	regions := make([]Region, 0, len(output.Regions))
	for _, r := range output.Regions {
		regions = append(regions, Region{
			ID:        aws.ToString(r.RegionName),
			Name:      aws.ToString(r.RegionName),
			Available: true,
		})
	}

	return regions, nil
}

// ListInstanceTypes returns available EC2 instance types.
func (p *AWSProvider) ListInstanceTypes(ctx context.Context, region string) ([]InstanceType, error) {
	// Common instance types for benchmarking
	commonTypes := []string{
		"t3.micro", "t3.small", "t3.medium", "t3.large", "t3.xlarge", "t3.2xlarge",
		"m5.large", "m5.xlarge", "m5.2xlarge", "m5.4xlarge",
		"c5.large", "c5.xlarge", "c5.2xlarge", "c5.4xlarge",
		"r5.large", "r5.xlarge", "r5.2xlarge",
	}

	output, err := p.ec2Client.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{
		InstanceTypes: toInstanceTypes(commonTypes),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe instance types: %w", err)
	}

	instanceTypes := make([]InstanceType, 0, len(output.InstanceTypes))
	for _, it := range output.InstanceTypes {
		memoryMB := int64(0)
		if it.MemoryInfo != nil && it.MemoryInfo.SizeInMiB != nil {
			memoryMB = *it.MemoryInfo.SizeInMiB
		}

		vcpus := int32(0)
		if it.VCpuInfo != nil && it.VCpuInfo.DefaultVCpus != nil {
			vcpus = *it.VCpuInfo.DefaultVCpus
		}

		networkGbps := float64(0)
		if it.NetworkInfo != nil && it.NetworkInfo.NetworkPerformance != nil {
			perf := *it.NetworkInfo.NetworkPerformance
			// Parse network performance string
			if strings.Contains(perf, "Up to") {
				// "Up to 5 Gigabit" -> 5
				fmt.Sscanf(perf, "Up to %f", &networkGbps)
			} else if strings.Contains(perf, "Gigabit") {
				fmt.Sscanf(perf, "%f Gigabit", &networkGbps)
			}
		}

		category := "general"
		name := string(it.InstanceType)
		if strings.HasPrefix(name, "c") {
			category = "compute"
		} else if strings.HasPrefix(name, "r") || strings.HasPrefix(name, "x") {
			category = "memory"
		} else if strings.HasPrefix(name, "t") {
			category = "burstable"
		}

		instanceTypes = append(instanceTypes, InstanceType{
			ID:          name,
			Name:        name,
			VCPUs:       int(vcpus),
			MemoryGB:    float64(memoryMB) / 1024,
			NetworkGbps: networkGbps,
			Category:    category,
			Currency:    "USD",
		})
	}

	return instanceTypes, nil
}

// Provision creates infrastructure according to the specification.
func (p *AWSProvider) Provision(ctx context.Context, spec *InfraSpec) (*Infrastructure, error) {
	infraID := fmt.Sprintf("infra-%s", uuid.New().String()[:8])

	infra := &Infrastructure{
		ID:               infraID,
		Name:             spec.Name,
		Provider:         "aws",
		Region:           spec.Region,
		State:            InfraStateProvisioning,
		CreatedAt:        time.Now(),
		Tags:             spec.Tags,
		ProviderMetadata: make(map[string]interface{}),
	}

	if spec.TTL > 0 {
		expires := time.Now().Add(spec.TTL)
		infra.ExpiresAt = &expires
	}

	// Merge default tags
	if infra.Tags == nil {
		infra.Tags = make(map[string]string)
	}
	infra.Tags["redismeter:infra-id"] = infraID
	infra.Tags["redismeter:managed"] = "true"
	for k, v := range p.config.Tags {
		if _, exists := infra.Tags[k]; !exists {
			infra.Tags[k] = v
		}
	}

	// Use a regional client if needed
	client := p.ec2Client
	if spec.Region != "" && spec.Region != p.region {
		awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(spec.Region))
		if err != nil {
			return nil, fmt.Errorf("failed to create regional client: %w", err)
		}
		client = ec2.NewFromConfig(awsCfg)
	}

	// Create or use existing security group
	sgID, err := p.ensureSecurityGroup(ctx, client, spec, infra)
	if err != nil {
		infra.State = InfraStateFailed
		return infra, fmt.Errorf("failed to create security group: %w", err)
	}

	infra.Network = &NetworkInfo{
		SecurityGroupID: sgID,
	}

	// Provision load generator instances
	if spec.LoadGenerator != nil && spec.LoadGenerator.Count > 0 {
		instances, err := p.provisionInstances(ctx, client, spec, spec.LoadGenerator, "load_generator", infra.Tags)
		if err != nil {
			infra.State = InfraStateFailed
			return infra, fmt.Errorf("failed to provision load generators: %w", err)
		}
		infra.LoadGenerators = instances
	}

	// Handle Redis target
	if spec.RedisTarget != nil {
		switch spec.RedisTarget.Type {
		case RedisTargetExisting:
			infra.RedisEndpoint = spec.RedisTarget.ExistingEndpoint
			// Parse port from endpoint
			if parts := strings.Split(spec.RedisTarget.ExistingEndpoint, ":"); len(parts) == 2 {
				fmt.Sscanf(parts[1], "%d", &infra.RedisPort)
			} else {
				infra.RedisPort = 6379
			}

		case RedisTargetSelfHosted:
			if spec.RedisTarget.SelfHosted != nil && spec.RedisTarget.SelfHosted.Nodes != nil {
				// Provision Redis instances
				// TODO: Implement self-hosted Redis provisioning
				return nil, fmt.Errorf("self-hosted Redis provisioning not yet implemented")
			}

		case RedisTargetManaged:
			// TODO: Implement ElastiCache provisioning
			return nil, fmt.Errorf("managed Redis (ElastiCache) provisioning not yet implemented")
		}
	}

	// Wait for instances to be running
	if len(infra.LoadGenerators) > 0 {
		instanceIDs := make([]string, len(infra.LoadGenerators))
		for i, inst := range infra.LoadGenerators {
			instanceIDs[i] = inst.ID
		}

		if err := p.waitForInstances(ctx, client, instanceIDs); err != nil {
			infra.State = InfraStateFailed
			return infra, fmt.Errorf("failed waiting for instances: %w", err)
		}

		// Refresh instance info to get IPs
		instances, err := p.describeInstances(ctx, client, instanceIDs)
		if err == nil {
			infra.LoadGenerators = instances
		}
	}

	infra.State = InfraStateReady

	// Store infrastructure
	p.mu.Lock()
	p.infras[infraID] = infra
	p.mu.Unlock()

	return infra, nil
}

// Teardown destroys the infrastructure.
func (p *AWSProvider) Teardown(ctx context.Context, infra *Infrastructure) error {
	if infra == nil {
		return fmt.Errorf("infrastructure is nil")
	}

	infra.State = InfraStateTearingDown

	// Get regional client
	client := p.ec2Client
	if infra.Region != "" && infra.Region != p.region {
		awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(infra.Region))
		if err != nil {
			return fmt.Errorf("failed to create regional client: %w", err)
		}
		client = ec2.NewFromConfig(awsCfg)
	}

	// Terminate instances
	var instanceIDs []string
	for _, inst := range infra.LoadGenerators {
		instanceIDs = append(instanceIDs, inst.ID)
	}

	if len(instanceIDs) > 0 {
		_, err := client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
			InstanceIds: instanceIDs,
		})
		if err != nil {
			return fmt.Errorf("failed to terminate instances: %w", err)
		}

		// Wait for termination
		waiter := ec2.NewInstanceTerminatedWaiter(client)
		if err := waiter.Wait(ctx, &ec2.DescribeInstancesInput{
			InstanceIds: instanceIDs,
		}, 5*time.Minute); err != nil {
			// Log but don't fail - instances will terminate eventually
			fmt.Printf("Warning: timeout waiting for instances to terminate: %v\n", err)
		}
	}

	// Delete security group (if we created it)
	if infra.Network != nil && infra.Network.SecurityGroupID != "" {
		// Wait a bit for instances to fully terminate
		time.Sleep(5 * time.Second)

		_, err := client.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{
			GroupId: aws.String(infra.Network.SecurityGroupID),
		})
		if err != nil {
			// Security group may be in use or already deleted
			fmt.Printf("Warning: failed to delete security group: %v\n", err)
		}
	}

	infra.State = InfraStateTerminated

	// Remove from tracked infrastructures
	p.mu.Lock()
	delete(p.infras, infra.ID)
	p.mu.Unlock()

	return nil
}

// GetInstances returns the current state of instances.
func (p *AWSProvider) GetInstances(ctx context.Context, infra *Infrastructure) ([]Instance, error) {
	if infra == nil || len(infra.LoadGenerators) == 0 {
		return nil, nil
	}

	client := p.ec2Client
	if infra.Region != "" && infra.Region != p.region {
		awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(infra.Region))
		if err != nil {
			return nil, fmt.Errorf("failed to create regional client: %w", err)
		}
		client = ec2.NewFromConfig(awsCfg)
	}

	var instanceIDs []string
	for _, inst := range infra.LoadGenerators {
		instanceIDs = append(instanceIDs, inst.ID)
	}

	return p.describeInstances(ctx, client, instanceIDs)
}

// GetInfrastructure retrieves infrastructure by ID.
func (p *AWSProvider) GetInfrastructure(ctx context.Context, id string) (*Infrastructure, error) {
	p.mu.RLock()
	infra, ok := p.infras[id]
	p.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("infrastructure not found: %s", id)
	}

	return infra, nil
}

// EstimateCost provides a cost estimate.
func (p *AWSProvider) EstimateCost(ctx context.Context, spec *InfraSpec, duration time.Duration) (*CostEstimate, error) {
	estimate := &CostEstimate{
		Duration: duration,
		Currency: "USD",
	}

	hours := duration.Hours()

	// Estimate based on instance types (rough pricing)
	instancePricing := map[string]float64{
		"t3.micro":   0.0104,
		"t3.small":   0.0208,
		"t3.medium":  0.0416,
		"t3.large":   0.0832,
		"t3.xlarge":  0.1664,
		"t3.2xlarge": 0.3328,
		"m5.large":   0.096,
		"m5.xlarge":  0.192,
		"m5.2xlarge": 0.384,
		"m5.4xlarge": 0.768,
		"c5.large":   0.085,
		"c5.xlarge":  0.17,
		"c5.2xlarge": 0.34,
		"c5.4xlarge": 0.68,
		"r5.large":   0.126,
		"r5.xlarge":  0.252,
		"r5.2xlarge": 0.504,
	}

	if spec.LoadGenerator != nil {
		instanceType := spec.LoadGenerator.InstanceType
		if instanceType == "" {
			instanceType = p.config.DefaultInstanceType
		}

		hourlyRate := instancePricing[instanceType]
		if hourlyRate == 0 {
			hourlyRate = 0.10 // Default estimate
		}

		if spec.LoadGenerator.SpotInstances {
			hourlyRate *= 0.3 // Spot instances are typically 70% cheaper
		}

		cost := hourlyRate * float64(spec.LoadGenerator.Count) * hours
		estimate.HourlyCost += hourlyRate * float64(spec.LoadGenerator.Count)

		estimate.Breakdown = append(estimate.Breakdown, CostBreakdownItem{
			ResourceType: "EC2 Instances",
			Description:  fmt.Sprintf("%d x %s", spec.LoadGenerator.Count, instanceType),
			Count:        spec.LoadGenerator.Count,
			UnitCost:     hourlyRate,
			TotalCost:    cost,
		})
	}

	estimate.TotalCost = estimate.HourlyCost * hours

	return estimate, nil
}

// ensureSecurityGroup creates or finds a security group for the infrastructure.
func (p *AWSProvider) ensureSecurityGroup(ctx context.Context, client *ec2.Client, spec *InfraSpec, infra *Infrastructure) (string, error) {
	sgName := fmt.Sprintf("redismeter-%s", infra.ID)

	// Determine VPC
	vpcID := p.config.DefaultVPCID
	if spec.Network != nil && spec.Network.VPCID != "" {
		vpcID = spec.Network.VPCID
	}

	// If no VPC specified, get default VPC
	if vpcID == "" {
		vpcs, err := client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
			Filters: []types.Filter{
				{Name: aws.String("is-default"), Values: []string{"true"}},
			},
		})
		if err != nil {
			return "", fmt.Errorf("failed to find default VPC: %w", err)
		}
		if len(vpcs.Vpcs) > 0 {
			vpcID = aws.ToString(vpcs.Vpcs[0].VpcId)
		}
	}

	// Create security group
	createSGOutput, err := client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(sgName),
		Description: aws.String("RedisMeter benchmark security group"),
		VpcId:       aws.String(vpcID),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeSecurityGroup,
				Tags:         mapToEC2Tags(infra.Tags),
			},
		},
	})
	if err != nil {
		return "", err
	}

	sgID := aws.ToString(createSGOutput.GroupId)

	// Add ingress rules
	sshCIDRs := []string{"0.0.0.0/0"}
	if spec.Network != nil && len(spec.Network.AllowSSHFrom) > 0 {
		sshCIDRs = spec.Network.AllowSSHFrom
	}

	// SSH access
	var sshPermissions []types.IpPermission
	for _, cidr := range sshCIDRs {
		sshPermissions = append(sshPermissions, types.IpPermission{
			IpProtocol: aws.String("tcp"),
			FromPort:   aws.Int32(22),
			ToPort:     aws.Int32(22),
			IpRanges:   []types.IpRange{{CidrIp: aws.String(cidr)}},
		})
	}

	// Redis access (internal)
	sshPermissions = append(sshPermissions, types.IpPermission{
		IpProtocol: aws.String("tcp"),
		FromPort:   aws.Int32(6379),
		ToPort:     aws.Int32(6379),
		UserIdGroupPairs: []types.UserIdGroupPair{
			{GroupId: aws.String(sgID)},
		},
	})

	_, err = client.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:       aws.String(sgID),
		IpPermissions: sshPermissions,
	})
	if err != nil {
		return "", fmt.Errorf("failed to add security group rules: %w", err)
	}

	return sgID, nil
}

// provisionInstances launches EC2 instances.
func (p *AWSProvider) provisionInstances(ctx context.Context, client *ec2.Client, spec *InfraSpec, nodeSpec *NodeGroupSpec, role string, tags map[string]string) ([]Instance, error) {
	// Determine AMI - Ubuntu is enforced, only use custom if explicitly configured in provider
	ami := p.config.DefaultAMI
	if ami == "" {
		// Find latest Ubuntu AMI (enforced OS)
		var err error
		ami, err = p.findLatestUbuntuAMI(ctx, client)
		if err != nil {
			return nil, fmt.Errorf("failed to find AMI: %w", err)
		}
	}

	// Determine instance type
	instanceType := nodeSpec.InstanceType
	if instanceType == "" {
		instanceType = p.config.DefaultInstanceType
	}

	// Determine key name
	keyName := nodeSpec.SSHKeyName
	if keyName == "" {
		keyName = p.config.DefaultKeyName
	}

	// Determine subnet
	subnetID := p.config.DefaultSubnetID
	if spec.Network != nil && spec.Network.SubnetID != "" {
		subnetID = spec.Network.SubnetID
	}

	// Build instance tags
	instanceTags := make(map[string]string)
	for k, v := range tags {
		instanceTags[k] = v
	}
	instanceTags["redismeter:role"] = role

	// Build run instances input
	input := &ec2.RunInstancesInput{
		ImageId:      aws.String(ami),
		InstanceType: types.InstanceType(instanceType),
		MinCount:     aws.Int32(int32(nodeSpec.Count)),
		MaxCount:     aws.Int32(int32(nodeSpec.Count)),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeInstance,
				Tags:         mapToEC2Tags(instanceTags),
			},
		},
	}

	if keyName != "" {
		input.KeyName = aws.String(keyName)
	}

	if subnetID != "" {
		input.SubnetId = aws.String(subnetID)
	}

	if nodeSpec.UserData != "" {
		input.UserData = aws.String(nodeSpec.UserData)
	}

	// Spot instances
	if nodeSpec.SpotInstances {
		input.InstanceMarketOptions = &types.InstanceMarketOptionsRequest{
			MarketType: types.MarketTypeSpot,
			SpotOptions: &types.SpotMarketOptions{
				SpotInstanceType: types.SpotInstanceTypeOneTime,
			},
		}
		if nodeSpec.MaxSpotPrice > 0 {
			input.InstanceMarketOptions.SpotOptions.MaxPrice = aws.String(fmt.Sprintf("%.4f", nodeSpec.MaxSpotPrice))
		}
	}

	// Launch instances
	output, err := client.RunInstances(ctx, input)
	if err != nil {
		return nil, err
	}

	instances := make([]Instance, 0, len(output.Instances))
	for _, inst := range output.Instances {
		instances = append(instances, Instance{
			ID:         aws.ToString(inst.InstanceId),
			Name:       aws.ToString(inst.InstanceId),
			Type:       string(inst.InstanceType),
			State:      InstanceStatePending,
			PrivateIP:  aws.ToString(inst.PrivateIpAddress),
			PublicIP:   aws.ToString(inst.PublicIpAddress),
			LaunchTime: aws.ToTime(inst.LaunchTime),
			Role:       role,
		})
	}

	return instances, nil
}

// waitForInstances waits for instances to be running.
func (p *AWSProvider) waitForInstances(ctx context.Context, client *ec2.Client, instanceIDs []string) error {
	waiter := ec2.NewInstanceRunningWaiter(client)
	return waiter.Wait(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: instanceIDs,
	}, 10*time.Minute)
}

// describeInstances gets the current state of instances.
func (p *AWSProvider) describeInstances(ctx context.Context, client *ec2.Client, instanceIDs []string) ([]Instance, error) {
	output, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: instanceIDs,
	})
	if err != nil {
		return nil, err
	}

	var instances []Instance
	for _, reservation := range output.Reservations {
		for _, inst := range reservation.Instances {
			state := InstanceStatePending
			switch inst.State.Name {
			case types.InstanceStateNameRunning:
				state = InstanceStateRunning
			case types.InstanceStateNameStopped:
				state = InstanceStateStopped
			case types.InstanceStateNameTerminated:
				state = InstanceStateTerminated
			case types.InstanceStateNameStopping:
				state = InstanceStateStopping
			}

			// Get name from tags
			name := aws.ToString(inst.InstanceId)
			role := ""
			for _, tag := range inst.Tags {
				if aws.ToString(tag.Key) == "Name" {
					name = aws.ToString(tag.Value)
				}
				if aws.ToString(tag.Key) == "redismeter:role" {
					role = aws.ToString(tag.Value)
				}
			}

			instances = append(instances, Instance{
				ID:         aws.ToString(inst.InstanceId),
				Name:       name,
				Type:       string(inst.InstanceType),
				State:      state,
				PrivateIP:  aws.ToString(inst.PrivateIpAddress),
				PublicIP:   aws.ToString(inst.PublicIpAddress),
				LaunchTime: aws.ToTime(inst.LaunchTime),
				Role:       role,
			})
		}
	}

	return instances, nil
}

// findLatestUbuntuAMI finds the latest Ubuntu 22.04 AMI.
func (p *AWSProvider) findLatestUbuntuAMI(ctx context.Context, client *ec2.Client) (string, error) {
	output, err := client.DescribeImages(ctx, &ec2.DescribeImagesInput{
		Owners: []string{"099720109477"}, // Canonical
		Filters: []types.Filter{
			{Name: aws.String("name"), Values: []string{"ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"}},
			{Name: aws.String("state"), Values: []string{"available"}},
		},
	})
	if err != nil {
		return "", err
	}

	if len(output.Images) == 0 {
		return "", fmt.Errorf("no Ubuntu AMI found")
	}

	// Find the most recent one
	var latestAMI *types.Image
	for i := range output.Images {
		if latestAMI == nil || aws.ToString(output.Images[i].CreationDate) > aws.ToString(latestAMI.CreationDate) {
			latestAMI = &output.Images[i]
		}
	}

	return aws.ToString(latestAMI.ImageId), nil
}

// Helper functions

func toInstanceTypes(names []string) []types.InstanceType {
	result := make([]types.InstanceType, len(names))
	for i, name := range names {
		result[i] = types.InstanceType(name)
	}
	return result
}

func mapToEC2Tags(tags map[string]string) []types.Tag {
	result := make([]types.Tag, 0, len(tags))
	for k, v := range tags {
		result = append(result, types.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}
	return result
}

// Ensure AWSProvider implements ProviderPlugin.
var _ ProviderPlugin = (*AWSProvider)(nil)

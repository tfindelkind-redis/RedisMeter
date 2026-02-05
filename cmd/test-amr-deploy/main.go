// Package main provides a test for Azure Managed Redis Bicep deployment.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/cloud"
)

func main() {
	ctx := context.Background()

	// Get subscription ID from environment or use default
	subscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	if subscriptionID == "" {
		// Try to get it from az CLI
		out, err := exec.Command("az", "account", "show", "--query", "id", "-o", "tsv").Output()
		if err != nil {
			log.Fatalf("AZURE_SUBSCRIPTION_ID not set and failed to get from az CLI: %v", err)
		}
		subscriptionID = string(out)
		subscriptionID = subscriptionID[:len(subscriptionID)-1] // Remove newline
	}

	resourceGroup := os.Getenv("AZURE_RESOURCE_GROUP")
	if resourceGroup == "" {
		resourceGroup = "rg-redismeter-test"
	}

	location := os.Getenv("AZURE_LOCATION")
	if location == "" {
		location = "westus3"
	}

	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║           RedisMeter AMR Deployment Test                       ║")
	fmt.Println("╠════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Subscription: %-48s║\n", subscriptionID[:36])
	fmt.Printf("║  Resource Group: %-44s║\n", resourceGroup)
	fmt.Printf("║  Location: %-50s║\n", location)
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Step 1: Create resource group
	fmt.Println("📦 Step 1: Creating resource group...")
	if err := createResourceGroup(subscriptionID, resourceGroup, location); err != nil {
		log.Fatalf("Failed to create resource group: %v", err)
	}
	fmt.Printf("   ✅ Resource group '%s' ready\n\n", resourceGroup)

	// Step 2: Deploy AMR using Bicep
	fmt.Println("🚀 Step 2: Deploying Azure Managed Redis (this takes 10-20 minutes)...")

	deployer, err := cloud.NewAMRBicepDeployer(subscriptionID)
	if err != nil {
		log.Fatalf("Failed to create deployer: %v", err)
	}

	redisName := fmt.Sprintf("rm-test-%d", time.Now().Unix())

	params := &cloud.AMRBicepDeploymentParams{
		RedisName:        redisName,
		Location:         location,
		SKUName:          "Balanced_B0", // Smallest SKU for testing
		ClusteringPolicy: "OSSCluster",
		HighAvailability: false, // No HA for dev/test
		EvictionPolicy:   "VolatileLRU",
		Tags: map[string]string{
			"environment": "test",
			"managed-by":  "redismeter",
			"purpose":     "deployment-test",
		},
	}

	fmt.Printf("   📋 Redis Name: %s\n", redisName)
	fmt.Printf("   📋 SKU: %s\n", params.SKUName)
	fmt.Printf("   📋 Clustering: %s\n", params.ClusteringPolicy)
	fmt.Printf("   ⏳ Starting deployment...\n\n")

	startTime := time.Now()
	result, err := deployer.DeployAMR(ctx, resourceGroup, params)
	if err != nil {
		log.Fatalf("Deployment failed: %v", err)
	}
	duration := time.Since(startTime)

	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║           ✅ Deployment Successful!                            ║")
	fmt.Println("╠════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Hostname: %-50s║\n", result.RedisHostName)
	fmt.Printf("║  Port: %-54d║\n", result.RedisPort)
	fmt.Printf("║  Duration: %-50s║\n", duration.Round(time.Second))
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Step 3: Test connection info
	fmt.Println("🔑 Connection Details:")
	fmt.Printf("   Host: %s\n", result.RedisHostName)
	fmt.Printf("   Port: %d\n", result.RedisPort)
	fmt.Printf("   TLS: Required (AMR always uses TLS)\n")
	if len(result.RedisPrimaryKey) > 20 {
		fmt.Printf("   Primary Key: %s...\n", result.RedisPrimaryKey[:20])
	} else {
		fmt.Printf("   Primary Key: %s\n", result.RedisPrimaryKey)
	}

	// Step 4: Cleanup prompt
	fmt.Println()
	fmt.Println("⚠️  CLEANUP REQUIRED:")
	fmt.Println("   To delete resources and avoid charges, run:")
	fmt.Printf("   az group delete --name %s --yes --no-wait\n", resourceGroup)
	fmt.Println()
}

func createResourceGroup(subscriptionID, name, location string) error {
	cmd := exec.Command("az", "group", "create",
		"--name", name,
		"--location", location,
		"--subscription", subscriptionID,
		"-o", "none")

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

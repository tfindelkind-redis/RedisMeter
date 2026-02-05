package infraprofile

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileValidation(t *testing.T) {
	tests := []struct {
		name    string
		profile Profile
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid azure profile",
			profile: Profile{
				Name:     "test-azure",
				Provider: ProviderAzure,
				Config: ProfileConfig{
					Azure: &AzureConfig{
						Location: "eastus",
						SKU:      "Basic",
						Family:   "C",
						Capacity: 0,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid aws profile",
			profile: Profile{
				Name:     "test-aws",
				Provider: ProviderAWS,
				Config: ProfileConfig{
					AWS: &AWSConfig{
						Region:        "us-east-1",
						NodeType:      "cache.t3.micro",
						NumCacheNodes: 1,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid local profile",
			profile: Profile{
				Name:     "test-local",
				Provider: ProviderLocal,
				Config: ProfileConfig{
					Local: &LocalConfig{
						Host: "localhost",
						Port: 6379,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			profile: Profile{
				Provider: ProviderAzure,
				Config: ProfileConfig{
					Azure: &AzureConfig{
						Location: "eastus",
						SKU:      "Basic",
						Family:   "C",
					},
				},
			},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name: "missing provider",
			profile: Profile{
				Name: "test",
			},
			wantErr: true,
			errMsg:  "provider is required",
		},
		{
			name: "azure missing config",
			profile: Profile{
				Name:     "test",
				Provider: ProviderAzure,
			},
			wantErr: true,
			errMsg:  "azure configuration is required",
		},
		{
			name: "azure invalid sku",
			profile: Profile{
				Name:     "test",
				Provider: ProviderAzure,
				Config: ProfileConfig{
					Azure: &AzureConfig{
						Location: "eastus",
						SKU:      "Invalid",
						Family:   "C",
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid sku",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.profile.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestAzureConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  AzureConfig
		wantErr bool
	}{
		{
			name: "valid",
			config: AzureConfig{
				Location: "eastus",
				SKU:      "Premium",
				Family:   "P",
				Capacity: 1,
			},
			wantErr: false,
		},
		{
			name:    "missing location",
			config:  AzureConfig{SKU: "Basic", Family: "C"},
			wantErr: true,
		},
		{
			name:    "missing sku",
			config:  AzureConfig{Location: "eastus", Family: "C"},
			wantErr: true,
		},
		{
			name:    "invalid capacity",
			config:  AzureConfig{Location: "eastus", SKU: "Basic", Family: "C", Capacity: 10},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestProfileClone(t *testing.T) {
	original := &Profile{
		ID:       "test-id",
		Name:     "test-profile",
		Provider: ProviderAzure,
		Tags:     []string{"test", "dev"},
		Config: ProfileConfig{
			Azure: &AzureConfig{
				Location: "eastus",
				SKU:      "Basic",
				Family:   "C",
				Capacity: 0,
			},
		},
	}

	clone := original.Clone()

	// Verify it's a deep copy
	if clone.ID != original.ID {
		t.Error("Clone ID doesn't match")
	}

	// Modify clone and ensure original is unchanged
	clone.Name = "modified"
	clone.Tags[0] = "modified"
	clone.Config.Azure.Location = "westus"

	if original.Name == "modified" {
		t.Error("Original name was modified")
	}
	if original.Tags[0] == "modified" {
		t.Error("Original tags were modified")
	}
	if original.Config.Azure.Location == "westus" {
		t.Error("Original config was modified")
	}
}

func TestProfileJSON(t *testing.T) {
	original := &Profile{
		ID:       "test-id",
		Name:     "test-profile",
		Provider: ProviderLocal,
		Config: ProfileConfig{
			Local: &LocalConfig{
				Host: "localhost",
				Port: 6379,
			},
		},
	}

	// Serialize
	data, err := original.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	// Deserialize
	restored, err := FromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	if restored.Name != original.Name {
		t.Error("Name doesn't match after JSON roundtrip")
	}
	if restored.Config.Local.Port != original.Config.Local.Port {
		t.Error("Config doesn't match after JSON roundtrip")
	}
}

func TestFileStore(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "profiles")

	store, err := NewFileStore(storePath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Test Create
	profile := &Profile{
		Name:        "test-profile",
		Description: "Test description",
		Provider:    ProviderLocal,
		Tags:        []string{"test", "dev"},
		Config: ProfileConfig{
			Local: &LocalConfig{
				Host: "localhost",
				Port: 6379,
			},
		},
	}

	if err := store.Create(ctx, profile); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if profile.ID == "" {
		t.Error("ID should be assigned after create")
	}

	// Test Get
	retrieved, err := store.Get(ctx, profile.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Profile not found")
	}
	if retrieved.Name != "test-profile" {
		t.Error("Name doesn't match")
	}

	// Test GetByName
	retrieved, err = store.GetByName(ctx, "test-profile")
	if err != nil {
		t.Fatalf("GetByName failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Profile not found by name")
	}

	// Test List
	profiles, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	// Should include predefined profiles + our test profile
	if len(profiles) < 1 {
		t.Error("Expected at least 1 profile")
	}

	// Test Update
	profile.Description = "Updated description"
	if err := store.Update(ctx, profile); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	updated, _ := store.Get(ctx, profile.ID)
	if updated.Description != "Updated description" {
		t.Error("Update didn't persist")
	}

	// Test RecordUsage
	if err := store.RecordUsage(ctx, profile.ID); err != nil {
		t.Fatalf("RecordUsage failed: %v", err)
	}

	used, _ := store.Get(ctx, profile.ID)
	if used.UseCount != 1 {
		t.Error("UseCount should be 1")
	}
	if used.LastUsedAt == nil {
		t.Error("LastUsedAt should be set")
	}

	// Test Delete
	if err := store.Delete(ctx, profile.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	deleted, _ := store.Get(ctx, profile.ID)
	if deleted != nil {
		t.Error("Profile should be deleted")
	}
}

func TestStoreListByProvider(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "profiles-list")

	store, err := NewFileStore(storePath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create profiles with different providers
	azureProfile := &Profile{
		Name:     "azure-test",
		Provider: ProviderAzure,
		Config: ProfileConfig{
			Azure: &AzureConfig{Location: "eastus", SKU: "Basic", Family: "C"},
		},
	}
	store.Create(ctx, azureProfile)

	localProfile := &Profile{
		Name:     "local-test",
		Provider: ProviderLocal,
		Config: ProfileConfig{
			Local: &LocalConfig{Host: "localhost", Port: 6379},
		},
	}
	store.Create(ctx, localProfile)

	// Test ListByProvider
	azureProfiles, err := store.ListByProvider(ctx, ProviderAzure)
	if err != nil {
		t.Fatalf("ListByProvider failed: %v", err)
	}

	// Count user-created azure profiles
	userAzureCount := 0
	for _, p := range azureProfiles {
		if p.Name == "azure-test" {
			userAzureCount++
		}
	}
	if userAzureCount != 1 {
		t.Errorf("Expected 1 user azure profile, got %d", userAzureCount)
	}
}

func TestStoreListByTag(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "profiles-tags")

	store, err := NewFileStore(storePath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create profiles with tags
	store.Create(ctx, &Profile{
		Name:     "tagged-1",
		Provider: ProviderLocal,
		Tags:     []string{"production", "critical"},
		Config:   ProfileConfig{Local: &LocalConfig{Host: "localhost", Port: 6379}},
	})
	store.Create(ctx, &Profile{
		Name:     "tagged-2",
		Provider: ProviderLocal,
		Tags:     []string{"development"},
		Config:   ProfileConfig{Local: &LocalConfig{Host: "localhost", Port: 6380}},
	})

	// Test ListByTag
	prodProfiles, err := store.ListByTag(ctx, "production")
	if err != nil {
		t.Fatalf("ListByTag failed: %v", err)
	}

	found := false
	for _, p := range prodProfiles {
		if p.Name == "tagged-1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to find tagged-1 profile")
	}
}

func TestStoreStats(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "profiles-stats")

	store, err := NewFileStore(storePath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	stats, err := store.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	// Should have predefined profiles
	if stats.TotalProfiles < len(PredefinedProfiles) {
		t.Errorf("Expected at least %d profiles, got %d", len(PredefinedProfiles), stats.TotalProfiles)
	}
}

func TestStoreExportImport(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "profiles-export")

	store, err := NewFileStore(storePath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create a test profile
	profile := &Profile{
		Name:     "export-test",
		Provider: ProviderLocal,
		Tags:     []string{"export"},
		Config:   ProfileConfig{Local: &LocalConfig{Host: "localhost", Port: 6379}},
	}
	store.Create(ctx, profile)

	// Export
	data, err := store.ExportAll(ctx)
	if err != nil {
		t.Fatalf("ExportAll failed: %v", err)
	}

	// Create new store and import
	newStorePath := filepath.Join(tmpDir, "profiles-import")
	newStore, _ := NewFileStore(newStorePath)
	defer newStore.Close()

	count, err := newStore.ImportProfiles(ctx, data, false)
	if err != nil {
		t.Fatalf("ImportProfiles failed: %v", err)
	}

	if count == 0 {
		t.Error("Expected some profiles to be imported")
	}

	// Verify the profile was imported
	imported, _ := newStore.GetByName(ctx, "export-test")
	if imported == nil {
		t.Error("Expected export-test profile to be imported")
	}
}

func TestPredefinedProfiles(t *testing.T) {
	// Ensure all predefined profiles are valid
	for _, p := range PredefinedProfiles {
		if err := p.Validate(); err != nil {
			t.Errorf("Predefined profile '%s' is invalid: %v", p.Name, err)
		}
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

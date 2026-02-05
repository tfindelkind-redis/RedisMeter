// Package infraprofile provides JSON file-based storage for infrastructure profiles.
package infraprofile

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// FileStore implements Store using JSON files.
type FileStore struct {
	basePath string
	mu       sync.RWMutex
}

// NewFileStore creates a new file-based profile store.
func NewFileStore(basePath string) (*FileStore, error) {
	// Ensure directory exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	store := &FileStore{basePath: basePath}

	// Seed predefined profiles if needed
	if err := store.seedPredefined(); err != nil {
		// Log but don't fail - predefined profiles are optional
		fmt.Printf("Warning: failed to seed predefined profiles: %v\n", err)
	}

	return store, nil
}

// Close is a no-op for file store (implements Store interface)
func (s *FileStore) Close() error {
	return nil
}

func (s *FileStore) profilePath(id string) string {
	return filepath.Join(s.basePath, id+".json")
}

func (s *FileStore) seedPredefined() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, profile := range PredefinedProfiles {
		filePath := s.profilePath(profile.ID)

		// Skip if already exists
		if _, err := os.Stat(filePath); err == nil {
			continue
		}

		// Create predefined profile
		p := profile // Copy
		p.CreatedAt = time.Now()
		p.UpdatedAt = time.Now()

		if err := s.writeProfile(&p); err != nil {
			return fmt.Errorf("failed to seed profile %s: %w", p.ID, err)
		}
	}
	return nil
}

func (s *FileStore) writeProfile(profile *Profile) error {
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize profile: %w", err)
	}

	filePath := s.profilePath(profile.ID)
	return os.WriteFile(filePath, data, 0644)
}

func (s *FileStore) readProfile(id string) (*Profile, error) {
	filePath := s.profilePath(id)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read profile: %w", err)
	}

	var profile Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("failed to parse profile: %w", err)
	}

	return &profile, nil
}

// Create creates a new profile.
func (s *FileStore) Create(ctx context.Context, profile *Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if profile.ID == "" {
		profile.ID = uuid.New().String()
	}
	profile.CreatedAt = time.Now()
	profile.UpdatedAt = time.Now()

	if err := profile.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Check if already exists
	if existing, _ := s.readProfile(profile.ID); existing != nil {
		return fmt.Errorf("profile with ID %s already exists", profile.ID)
	}

	// Check for duplicate name
	profiles, err := s.listAllUnlocked()
	if err != nil {
		return err
	}
	for _, p := range profiles {
		if p.Name == profile.Name {
			return fmt.Errorf("profile with name %s already exists", profile.Name)
		}
	}

	return s.writeProfile(profile)
}

// Get retrieves a profile by ID.
func (s *FileStore) Get(ctx context.Context, id string) (*Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.readProfile(id)
}

// GetByName retrieves a profile by name.
func (s *FileStore) GetByName(ctx context.Context, name string) (*Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	profiles, err := s.listAllUnlocked()
	if err != nil {
		return nil, err
	}

	for _, p := range profiles {
		if p.Name == name {
			return p, nil
		}
	}
	return nil, nil
}

func (s *FileStore) listAllUnlocked() ([]*Profile, error) {
	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read profiles directory: %w", err)
	}

	var profiles []*Profile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		id := strings.TrimSuffix(entry.Name(), ".json")
		profile, err := s.readProfile(id)
		if err != nil {
			continue // Skip invalid profiles
		}
		if profile != nil {
			profiles = append(profiles, profile)
		}
	}

	// Sort by name
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})

	return profiles, nil
}

// List returns all profiles.
func (s *FileStore) List(ctx context.Context) ([]*Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.listAllUnlocked()
}

// ListByProvider returns profiles for a specific provider.
func (s *FileStore) ListByProvider(ctx context.Context, provider Provider) ([]*Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	all, err := s.listAllUnlocked()
	if err != nil {
		return nil, err
	}

	var filtered []*Profile
	for _, p := range all {
		if p.Provider == provider {
			filtered = append(filtered, p)
		}
	}
	return filtered, nil
}

// ListByTag returns profiles with a specific tag.
func (s *FileStore) ListByTag(ctx context.Context, tag string) ([]*Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	all, err := s.listAllUnlocked()
	if err != nil {
		return nil, err
	}

	var filtered []*Profile
	for _, p := range all {
		for _, t := range p.Tags {
			if t == tag {
				filtered = append(filtered, p)
				break
			}
		}
	}
	return filtered, nil
}

// Update updates an existing profile.
func (s *FileStore) Update(ctx context.Context, profile *Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := s.readProfile(profile.ID)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("profile not found: %s", profile.ID)
	}

	profile.UpdatedAt = time.Now()
	profile.CreatedAt = existing.CreatedAt // Preserve original created time
	profile.IsBuiltin = existing.IsBuiltin // Cannot change builtin status

	if err := profile.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	return s.writeProfile(profile)
}

// Delete removes a profile.
func (s *FileStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := s.profilePath(id)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("profile not found: %s", id)
	}

	return os.Remove(filePath)
}

// RecordUsage records that a profile was used.
func (s *FileStore) RecordUsage(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	profile, err := s.readProfile(id)
	if err != nil {
		return err
	}
	if profile == nil {
		return fmt.Errorf("profile not found: %s", id)
	}

	now := time.Now()
	profile.LastUsedAt = &now
	profile.UseCount++
	profile.UpdatedAt = now

	return s.writeProfile(profile)
}

// GetStats returns statistics about stored profiles.
func (s *FileStore) GetStats(ctx context.Context) (*ProfileStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	profiles, err := s.listAllUnlocked()
	if err != nil {
		return nil, err
	}

	stats := &ProfileStats{
		TotalProfiles: len(profiles),
		ByProvider:    make(map[Provider]int),
	}

	var (
		mostUsed       *Profile
		recentlyCreated *Profile
		recentlyUsed   *Profile
	)

	for _, p := range profiles {
		stats.ByProvider[p.Provider]++

		if mostUsed == nil || p.UseCount > mostUsed.UseCount {
			mostUsed = p
		}
		if recentlyCreated == nil || p.CreatedAt.After(recentlyCreated.CreatedAt) {
			recentlyCreated = p
		}
		if p.LastUsedAt != nil && (recentlyUsed == nil || p.LastUsedAt.After(*recentlyUsed.LastUsedAt)) {
			recentlyUsed = p
		}
	}

	stats.MostUsed = mostUsed
	stats.RecentlyCreated = recentlyCreated
	stats.RecentlyUsed = recentlyUsed

	return stats, nil
}

// ExportAll exports all profiles as JSON.
func (s *FileStore) ExportAll(ctx context.Context) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	profiles, err := s.listAllUnlocked()
	if err != nil {
		return nil, err
	}

	return json.MarshalIndent(profiles, "", "  ")
}

// ImportProfiles imports profiles from JSON data.
func (s *FileStore) ImportProfiles(ctx context.Context, data []byte, overwrite bool) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var profiles []Profile
	if err := json.Unmarshal(data, &profiles); err != nil {
		return 0, fmt.Errorf("failed to parse import data: %w", err)
	}

	imported := 0
	for _, profile := range profiles {
		// Don't import built-in profiles
		if profile.IsBuiltin {
			continue
		}

		existing, _ := s.readProfile(profile.ID)
		if existing != nil && !overwrite {
			continue
		}

		if profile.ID == "" {
			profile.ID = uuid.New().String()
		}
		if profile.CreatedAt.IsZero() {
			profile.CreatedAt = time.Now()
		}
		profile.UpdatedAt = time.Now()

		if err := s.writeProfile(&profile); err != nil {
			continue
		}
		imported++
	}

	return imported, nil
}

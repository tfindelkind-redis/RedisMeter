// Package storage provides persistence backends for RedisMeter.
package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Type != StorageTypeFile {
		t.Errorf("Expected default type to be 'file', got %s", config.Type)
	}

	home, _ := os.UserHomeDir()
	expectedPath := filepath.Join(home, ".redismeter")
	if config.Path != expectedPath {
		t.Errorf("Expected default path %s, got %s", expectedPath, config.Path)
	}
}

func TestNewStorage_FileBackend(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "redismeter-factory-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := StorageConfig{
		Type: StorageTypeFile,
		Path: tmpDir,
	}

	storage, err := NewStorage(config)
	if err != nil {
		t.Fatalf("Failed to create file storage: %v", err)
	}

	meta := storage.Metadata()
	if meta.Name != "file" {
		t.Errorf("Expected file storage, got %s", meta.Name)
	}
}

func TestNewStorage_SQLiteBackend(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "redismeter-factory-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := StorageConfig{
		Type: StorageTypeSQLite,
		Path: tmpDir,
	}

	storage, err := NewStorage(config)
	if err != nil {
		t.Fatalf("Failed to create SQLite storage: %v", err)
	}

	meta := storage.Metadata()
	if meta.Name != "sqlite" {
		t.Errorf("Expected sqlite storage, got %s", meta.Name)
	}

	// Verify database file was created
	dbPath := filepath.Join(tmpDir, "redismeter.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}
}

func TestNewStorage_SQLiteWithExplicitPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "redismeter-factory-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "custom.db")
	config := StorageConfig{
		Type: StorageTypeSQLite,
		Path: dbPath,
	}

	storage, err := NewStorage(config)
	if err != nil {
		t.Fatalf("Failed to create SQLite storage: %v", err)
	}

	meta := storage.Metadata()
	if meta.Name != "sqlite" {
		t.Errorf("Expected sqlite storage, got %s", meta.Name)
	}

	// Verify database file was created at explicit path
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created at explicit path")
	}
}

func TestNewStorage_DefaultsToFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "redismeter-factory-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Empty type should default to file
	config := StorageConfig{
		Type: "",
		Path: tmpDir,
	}

	storage, err := NewStorage(config)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	meta := storage.Metadata()
	if meta.Name != "file" {
		t.Errorf("Expected default file storage, got %s", meta.Name)
	}
}

func TestNewStorage_UnsupportedType(t *testing.T) {
	config := StorageConfig{
		Type: "unsupported",
		Path: "/tmp",
	}

	_, err := NewStorage(config)
	if err == nil {
		t.Error("Expected error for unsupported storage type")
	}
}

// mockViperConfig implements the viper interface for testing.
type mockViperConfig struct {
	values map[string]string
}

func (m *mockViperConfig) GetString(key string) string {
	return m.values[key]
}

func TestNewStorageFromViper(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "redismeter-factory-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("uses defaults when no config", func(t *testing.T) {
		v := &mockViperConfig{values: map[string]string{}}
		
		storage, err := NewStorageFromViper(v)
		if err != nil {
			t.Fatalf("Failed to create storage: %v", err)
		}

		meta := storage.Metadata()
		if meta.Name != "file" {
			t.Errorf("Expected default file storage, got %s", meta.Name)
		}
	})

	t.Run("uses viper config", func(t *testing.T) {
		v := &mockViperConfig{values: map[string]string{
			"storage.type": "sqlite",
			"storage.path": tmpDir,
		}}

		storage, err := NewStorageFromViper(v)
		if err != nil {
			t.Fatalf("Failed to create storage: %v", err)
		}

		meta := storage.Metadata()
		if meta.Name != "sqlite" {
			t.Errorf("Expected sqlite storage, got %s", meta.Name)
		}
	})
}

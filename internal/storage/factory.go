// Package storage provides persistence backends for RedisMeter.
package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// StorageType represents the type of storage backend.
type StorageType string

const (
	// StorageTypeFile uses JSON files for storage.
	StorageTypeFile StorageType = "file"
	// StorageTypeSQLite uses SQLite database for storage.
	StorageTypeSQLite StorageType = "sqlite"
)

// StorageConfig holds configuration for storage backends.
type StorageConfig struct {
	// Type specifies the storage backend type (file, sqlite)
	Type StorageType `mapstructure:"type" json:"type"`

	// Path is the base path for storage (directory for file, database path for sqlite)
	Path string `mapstructure:"path" json:"path,omitempty"`
}

// DefaultConfig returns the default storage configuration.
func DefaultConfig() StorageConfig {
	home, _ := os.UserHomeDir()
	return StorageConfig{
		Type: StorageTypeFile,
		Path: filepath.Join(home, ".redismeter"),
	}
}

// NewStorage creates a storage plugin based on the provided configuration.
func NewStorage(config StorageConfig) (RunStorage, error) {
	// Use defaults if not specified
	if config.Type == "" {
		config.Type = StorageTypeFile
	}

	if config.Path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		config.Path = filepath.Join(home, ".redismeter")
	}

	switch config.Type {
	case StorageTypeFile:
		return NewFileStorage(config.Path)

	case StorageTypeSQLite:
		dbPath := config.Path
		// If path is a directory, append default database filename
		info, err := os.Stat(dbPath)
		if err == nil && info.IsDir() {
			dbPath = filepath.Join(dbPath, "redismeter.db")
		} else if !filepath.IsAbs(dbPath) || filepath.Ext(dbPath) == "" {
			// If not absolute or no extension, treat as directory path
			dbPath = filepath.Join(dbPath, "redismeter.db")
		}
		return NewSQLiteStorage(dbPath)

	default:
		return nil, fmt.Errorf("unsupported storage type: %s", config.Type)
	}
}

// NewStorageFromViper creates a storage plugin from viper configuration.
// It reads the following config keys:
//   - storage.type: "file" or "sqlite" (default: "file")
//   - storage.path: base path for storage (default: ~/.redismeter)
func NewStorageFromViper(v interface {
	GetString(key string) string
}) (RunStorage, error) {
	config := DefaultConfig()

	if storageType := v.GetString("storage.type"); storageType != "" {
		config.Type = StorageType(storageType)
	}

	if storagePath := v.GetString("storage.path"); storagePath != "" {
		config.Path = storagePath
	}

	return NewStorage(config)
}

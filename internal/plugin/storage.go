// Package plugin provides the plugin framework for RedisMeter.
package plugin

import (
	"context"
)

// StoragePlugin defines the interface for persistence backends.
type StoragePlugin interface {
	Plugin

	// Save stores an entity and returns its ID.
	Save(ctx context.Context, entityType string, entity interface{}) (string, error)

	// Load retrieves an entity by ID.
	Load(ctx context.Context, entityType string, id string, dest interface{}) error

	// Query finds entities matching the given filter.
	Query(ctx context.Context, entityType string, filter QueryFilter, dest interface{}) error

	// Delete removes an entity by ID.
	Delete(ctx context.Context, entityType string, id string) error
}

// QueryFilter defines criteria for querying entities.
type QueryFilter struct {
	// Conditions maps field names to expected values.
	Conditions map[string]interface{} `json:"conditions,omitempty"`

	// Tags filters by tags (if entity supports tagging).
	Tags []string `json:"tags,omitempty"`

	// TimeRange filters by creation/update time.
	TimeRange *TimeRange `json:"time_range,omitempty"`

	// OrderBy specifies the sort field.
	OrderBy string `json:"order_by,omitempty"`

	// Descending reverses the sort order.
	Descending bool `json:"descending,omitempty"`

	// Limit caps the number of results.
	Limit int `json:"limit,omitempty"`

	// Offset skips the first N results.
	Offset int `json:"offset,omitempty"`
}

// TimeRange specifies a time window for filtering.
type TimeRange struct {
	Start string `json:"start,omitempty"` // RFC3339 format
	End   string `json:"end,omitempty"`   // RFC3339 format
}

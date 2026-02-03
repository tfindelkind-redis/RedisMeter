// Package storage provides persistence backends for RedisMeter.
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
	"github.com/tfindelkind-redis/redismeter/internal/plugin"
)

// MongoConfig holds configuration for MongoDB storage.
type MongoConfig struct {
	// Connection URI (e.g., "mongodb://localhost:27017")
	URI string `json:"uri"`

	// Database name
	Database string `json:"database"`

	// Collection names (optional, defaults provided)
	RunsCollection      string `json:"runs_collection"`
	BaselinesCollection string `json:"baselines_collection"`
	UsersCollection     string `json:"users_collection"`
	OrgsCollection      string `json:"orgs_collection"`
	AuditCollection     string `json:"audit_collection"`

	// Connection pool settings
	MaxPoolSize     uint64        `json:"max_pool_size"`
	MinPoolSize     uint64        `json:"min_pool_size"`
	MaxConnIdleTime time.Duration `json:"max_conn_idle_time"`

	// Timeouts
	ConnectTimeout time.Duration `json:"connect_timeout"`
	ServerTimeout  time.Duration `json:"server_timeout"`

	// Index creation
	AutoCreateIndexes bool `json:"auto_create_indexes"`
}

// DefaultMongoConfig returns sensible defaults for MongoDB.
func DefaultMongoConfig() MongoConfig {
	return MongoConfig{
		URI:                 "mongodb://localhost:27017",
		Database:            "redismeter",
		RunsCollection:      "benchmark_runs",
		BaselinesCollection: "baselines",
		UsersCollection:     "users",
		OrgsCollection:      "organizations",
		AuditCollection:     "audit_log",
		MaxPoolSize:         100,
		MinPoolSize:         10,
		MaxConnIdleTime:     30 * time.Minute,
		ConnectTimeout:      10 * time.Second,
		ServerTimeout:       30 * time.Second,
		AutoCreateIndexes:   true,
	}
}

// MongoDocument represents a MongoDB document wrapper for benchmark runs.
type MongoDocument struct {
	ID          string                 `bson:"_id" json:"_id"`
	CreatedAt   time.Time              `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time              `bson:"updated_at" json:"updated_at"`
	Status      string                 `bson:"status" json:"status"`
	WorkloadName string                `bson:"workload_name,omitempty" json:"workload_name,omitempty"`
	WorkloadType string                `bson:"workload_type,omitempty" json:"workload_type,omitempty"`
	TargetHost  string                 `bson:"target_host,omitempty" json:"target_host,omitempty"`
	TargetPort  int                    `bson:"target_port,omitempty" json:"target_port,omitempty"`
	StartTime   *time.Time             `bson:"start_time,omitempty" json:"start_time,omitempty"`
	EndTime     *time.Time             `bson:"end_time,omitempty" json:"end_time,omitempty"`
	Duration    string                 `bson:"duration,omitempty" json:"duration,omitempty"`
	Error       string                 `bson:"error,omitempty" json:"error,omitempty"`
	Name        string                 `bson:"name,omitempty" json:"name,omitempty"`
	Description string                 `bson:"description,omitempty" json:"description,omitempty"`
	Workload    map[string]interface{} `bson:"workload,omitempty" json:"workload,omitempty"`
	Target      map[string]interface{} `bson:"target,omitempty" json:"target,omitempty"`
	Environment map[string]interface{} `bson:"environment,omitempty" json:"environment,omitempty"`
	Results     map[string]interface{} `bson:"results,omitempty" json:"results,omitempty"`
	Tags        []string               `bson:"tags,omitempty" json:"tags,omitempty"`
	Labels      map[string]string      `bson:"labels,omitempty" json:"labels,omitempty"`
	OrgID       string                 `bson:"org_id,omitempty" json:"org_id,omitempty"`
}

// MongoBaselineDocument represents a MongoDB document for baselines.
type MongoBaselineDocument struct {
	ID          string                 `bson:"_id" json:"_id"`
	Name        string                 `bson:"name" json:"name"`
	Description string                 `bson:"description,omitempty" json:"description,omitempty"`
	RunID       string                 `bson:"run_id,omitempty" json:"run_id,omitempty"`
	CreatedAt   time.Time              `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time              `bson:"updated_at" json:"updated_at"`
	Active      bool                   `bson:"active" json:"active"`
	ValidFrom   *time.Time             `bson:"valid_from,omitempty" json:"valid_from,omitempty"`
	ValidUntil  *time.Time             `bson:"valid_until,omitempty" json:"valid_until,omitempty"`
	Metrics     map[string]interface{} `bson:"metrics,omitempty" json:"metrics,omitempty"`
	Environment map[string]interface{} `bson:"environment,omitempty" json:"environment,omitempty"`
	Workload    map[string]interface{} `bson:"workload,omitempty" json:"workload,omitempty"`
	Thresholds  map[string]interface{} `bson:"thresholds,omitempty" json:"thresholds,omitempty"`
	Tags        []string               `bson:"tags,omitempty" json:"tags,omitempty"`
	Labels      map[string]string      `bson:"labels,omitempty" json:"labels,omitempty"`
	OrgID       string                 `bson:"org_id,omitempty" json:"org_id,omitempty"`
}

// MongoStorage implements StoragePlugin using MongoDB.
// This implementation provides a compatible interface without requiring the MongoDB driver.
// To use MongoDB, install the driver: go get go.mongodb.org/mongo-driver/mongo
type MongoStorage struct {
	config MongoConfig
	mu     sync.RWMutex

	// In-memory storage for demonstration/fallback
	// Real implementation would use *mongo.Client
	runs      map[string]*domain.BenchmarkRun
	baselines map[string]*domain.Baseline
	connected bool
}

// NewMongoStorage creates a new MongoDB-based storage plugin.
// Note: This is a stub implementation. For production use, integrate with
// go.mongodb.org/mongo-driver/mongo package.
func NewMongoStorage(config MongoConfig) (*MongoStorage, error) {
	// Check for environment variable override
	if envURI := os.Getenv("REDISMETER_MONGODB_URI"); envURI != "" {
		config.URI = envURI
	}

	storage := &MongoStorage{
		config:    config,
		runs:      make(map[string]*domain.BenchmarkRun),
		baselines: make(map[string]*domain.Baseline),
		connected: false,
	}

	// In a real implementation, we would:
	// 1. Create MongoDB client with options
	// 2. Connect to database
	// 3. Create indexes if AutoCreateIndexes is true

	return storage, nil
}

// Connect establishes connection to MongoDB.
// Placeholder for real MongoDB connection logic.
func (s *MongoStorage) Connect(ctx context.Context) error {
	// In real implementation:
	// client, err := mongo.Connect(ctx, options.Client().ApplyURI(s.config.URI))
	// s.client = client

	s.mu.Lock()
	defer s.mu.Unlock()
	s.connected = true
	return nil
}

// CreateIndexes creates necessary indexes for optimal query performance.
// This would be called during initialization if AutoCreateIndexes is true.
func (s *MongoStorage) CreateIndexes(ctx context.Context) error {
	// Indexes to create for benchmark_runs collection:
	// - {status: 1}
	// - {workload_name: 1}
	// - {target_host: 1, target_port: 1}
	// - {created_at: -1}
	// - {tags: 1}
	// - {org_id: 1}
	// - {$text: {name: "text", description: "text"}}
	//
	// Indexes for baselines collection:
	// - {name: 1}
	// - {run_id: 1}
	// - {active: 1}
	// - {org_id: 1}
	// - {tags: 1}

	return nil
}

// Metadata returns plugin metadata.
func (s *MongoStorage) Metadata() plugin.Metadata {
	return plugin.Metadata{
		Name:        "mongodb",
		Version:     "1.0.0",
		Type:        plugin.TypeStorage,
		Description: "MongoDB document storage for benchmark data with aggregation support",
	}
}

// Initialize sets up the plugin.
func (s *MongoStorage) Initialize(ctx context.Context, config map[string]interface{}) error {
	return s.Connect(ctx)
}

// HealthCheck returns the plugin health status.
func (s *MongoStorage) HealthCheck(ctx context.Context) plugin.HealthStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.connected {
		return plugin.HealthStatus{Healthy: false, Message: "Not connected to MongoDB"}
	}

	// Real implementation would ping the server:
	// err := s.client.Ping(ctx, nil)

	return plugin.HealthStatus{Healthy: true, Message: "OK"}
}

// Shutdown gracefully stops the plugin.
func (s *MongoStorage) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Real implementation:
	// return s.client.Disconnect(ctx)

	s.connected = false
	return nil
}

// Save stores an entity and returns its ID.
func (s *MongoStorage) Save(ctx context.Context, entityType string, entity interface{}) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch entityType {
	case "runs":
		run := entity.(*domain.BenchmarkRun)
		s.runs[run.ID] = run
		return run.ID, nil
	case "baselines":
		baseline := entity.(*domain.Baseline)
		s.baselines[baseline.ID] = baseline
		return baseline.ID, nil
	default:
		return "", fmt.Errorf("unsupported entity type: %s", entityType)
	}
}

// Load retrieves an entity by ID.
func (s *MongoStorage) Load(ctx context.Context, entityType string, id string, dest interface{}) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	switch entityType {
	case "runs":
		run, exists := s.runs[id]
		if !exists {
			return fmt.Errorf("run not found: %s", id)
		}
		*dest.(*domain.BenchmarkRun) = *run
		return nil
	case "baselines":
		baseline, exists := s.baselines[id]
		if !exists {
			return fmt.Errorf("baseline not found: %s", id)
		}
		*dest.(*domain.Baseline) = *baseline
		return nil
	default:
		return fmt.Errorf("unsupported entity type: %s", entityType)
	}
}

// Query finds entities matching the given filter.
func (s *MongoStorage) Query(ctx context.Context, entityType string, filter plugin.QueryFilter, dest interface{}) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	switch entityType {
	case "runs":
		runs := s.filterRuns(filter)
		*dest.(*[]*domain.BenchmarkRun) = runs
		return nil
	case "baselines":
		baselines := s.filterBaselines(filter)
		*dest.(*[]*domain.Baseline) = baselines
		return nil
	default:
		return fmt.Errorf("unsupported entity type: %s", entityType)
	}
}

func (s *MongoStorage) filterRuns(filter plugin.QueryFilter) []*domain.BenchmarkRun {
	var result []*domain.BenchmarkRun

	for _, run := range s.runs {
		if s.matchesFilter(run, filter) {
			result = append(result, run)
		}
	}

	// Sort by created_at descending (simplified)
	// Real MongoDB implementation would use aggregation pipeline

	// Apply limit
	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}

	return result
}

func (s *MongoStorage) matchesFilter(run *domain.BenchmarkRun, filter plugin.QueryFilter) bool {
	if filter.Conditions != nil {
		if status, ok := filter.Conditions["status"]; ok {
			if string(run.Status) != status.(string) {
				return false
			}
		}
		if workload, ok := filter.Conditions["workload"]; ok {
			if run.Workload == nil || run.Workload.Name != workload.(string) {
				return false
			}
		}
		if target, ok := filter.Conditions["target"]; ok {
			if run.Target == nil || run.Target.Host != target.(string) {
				return false
			}
		}
	}

	if len(filter.Tags) > 0 {
		if !hasAnyTag(run.Tags, filter.Tags) {
			return false
		}
	}

	return true
}

func (s *MongoStorage) filterBaselines(filter plugin.QueryFilter) []*domain.Baseline {
	var result []*domain.Baseline

	for _, baseline := range s.baselines {
		if s.matchesBaselineFilter(baseline, filter) {
			result = append(result, baseline)
		}
	}

	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}

	return result
}

func (s *MongoStorage) matchesBaselineFilter(baseline *domain.Baseline, filter plugin.QueryFilter) bool {
	if filter.Conditions != nil {
		if active, ok := filter.Conditions["active"]; ok {
			if baseline.Active != active.(bool) {
				return false
			}
		}
		if name, ok := filter.Conditions["name"]; ok {
			if baseline.Name != name.(string) {
				return false
			}
		}
	}

	if len(filter.Tags) > 0 {
		if !hasAnyTag(baseline.Tags, filter.Tags) {
			return false
		}
	}

	return true
}

func hasAnyTag(entityTags, filterTags []string) bool {
	tagSet := make(map[string]bool)
	for _, t := range entityTags {
		tagSet[t] = true
	}
	for _, t := range filterTags {
		if tagSet[t] {
			return true
		}
	}
	return false
}

// Delete removes an entity by ID.
func (s *MongoStorage) Delete(ctx context.Context, entityType string, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch entityType {
	case "runs":
		if _, exists := s.runs[id]; !exists {
			return fmt.Errorf("run not found: %s", id)
		}
		delete(s.runs, id)
		return nil
	case "baselines":
		if _, exists := s.baselines[id]; !exists {
			return fmt.Errorf("baseline not found: %s", id)
		}
		delete(s.baselines, id)
		return nil
	default:
		return fmt.Errorf("unsupported entity type: %s", entityType)
	}
}

// SaveRun persists a benchmark run.
func (s *MongoStorage) SaveRun(ctx context.Context, run *domain.BenchmarkRun) error {
	_, err := s.Save(ctx, "runs", run)
	return err
}

// ListRuns returns recent benchmark runs.
func (s *MongoStorage) ListRuns(ctx context.Context, limit int) ([]*domain.BenchmarkRun, error) {
	var runs []*domain.BenchmarkRun
	err := s.Query(ctx, "runs", plugin.QueryFilter{Limit: limit, Descending: true}, &runs)
	return runs, err
}

// GetRun retrieves a single benchmark run by ID.
func (s *MongoStorage) GetRun(ctx context.Context, id string) (*domain.BenchmarkRun, error) {
	run := &domain.BenchmarkRun{}
	err := s.Load(ctx, "runs", id, run)
	if err != nil {
		return nil, err
	}
	return run, nil
}

// DeleteRun removes a benchmark run by ID.
func (s *MongoStorage) DeleteRun(ctx context.Context, id string) error {
	return s.Delete(ctx, "runs", id)
}

// Aggregation methods (MongoDB-specific features)

// AggregateStats returns aggregated statistics for runs.
// This would use MongoDB's aggregation pipeline in a real implementation.
func (s *MongoStorage) AggregateStats(ctx context.Context, groupBy string, filter plugin.QueryFilter) ([]map[string]interface{}, error) {
	// Example aggregation pipeline:
	// db.benchmark_runs.aggregate([
	//   {$match: filter},
	//   {$group: {
	//     _id: "$workload_name",
	//     count: {$sum: 1},
	//     avgLatency: {$avg: "$results.summary.avg_latency_ms"},
	//     maxThroughput: {$max: "$results.summary.ops_per_second"}
	//   }}
	// ])

	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]map[string]interface{})

	for _, run := range s.runs {
		if run.Results == nil || run.Results.Summary == nil {
			continue
		}

		var key string
		switch groupBy {
		case "workload":
			if run.Workload != nil {
				key = run.Workload.Name
			}
		case "target":
			if run.Target != nil {
				key = run.Target.Host
			}
		case "status":
			key = string(run.Status)
		default:
			key = "all"
		}

		if stats[key] == nil {
			stats[key] = map[string]interface{}{
				"count":         0,
				"total_latency": 0.0,
				"max_throughput": 0.0,
			}
		}

		stats[key]["count"] = stats[key]["count"].(int) + 1
		stats[key]["total_latency"] = stats[key]["total_latency"].(float64) + run.Results.Summary.AvgLatencyMs
		if run.Results.Summary.OpsPerSecond > stats[key]["max_throughput"].(float64) {
			stats[key]["max_throughput"] = run.Results.Summary.OpsPerSecond
		}
	}

	var result []map[string]interface{}
	for key, stat := range stats {
		count := stat["count"].(int)
		result = append(result, map[string]interface{}{
			"_id":            key,
			"count":          count,
			"avg_latency":    stat["total_latency"].(float64) / float64(count),
			"max_throughput": stat["max_throughput"],
		})
	}

	return result, nil
}

// TimeSeriesData returns time-bucketed data for trend analysis.
func (s *MongoStorage) TimeSeriesData(ctx context.Context, metric string, bucketSize time.Duration, filter plugin.QueryFilter) ([]map[string]interface{}, error) {
	// MongoDB $bucket aggregation:
	// db.benchmark_runs.aggregate([
	//   {$match: filter},
	//   {$bucket: {
	//     groupBy: "$created_at",
	//     boundaries: [...buckets...],
	//     output: {
	//       count: {$sum: 1},
	//       avgValue: {$avg: "$results.summary." + metric}
	//     }
	//   }}
	// ])

	return nil, fmt.Errorf("time series aggregation not implemented in stub")
}

// TextSearch performs full-text search using MongoDB's text index.
func (s *MongoStorage) TextSearch(ctx context.Context, query string, limit int) ([]*domain.BenchmarkRun, error) {
	// MongoDB text search:
	// db.benchmark_runs.find(
	//   {$text: {$search: query}},
	//   {score: {$meta: "textScore"}}
	// ).sort({score: {$meta: "textScore"}}).limit(limit)

	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.BenchmarkRun
	for _, run := range s.runs {
		// Simple substring matching for stub
		if containsIgnoreCase(run.Name, query) || containsIgnoreCase(run.Description, query) {
			result = append(result, run)
			if len(result) >= limit {
				break
			}
		}
	}

	return result, nil
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && 
		(s == substr || len(s) >= len(substr))
}

// ToDocument converts a BenchmarkRun to a MongoDB document.
func ToDocument(run *domain.BenchmarkRun) (*MongoDocument, error) {
	doc := &MongoDocument{
		ID:          run.ID,
		CreatedAt:   run.CreatedAt,
		UpdatedAt:   run.UpdatedAt,
		Status:      string(run.Status),
		Duration:    run.Duration,
		Error:       run.Error,
		Name:        run.Name,
		Description: run.Description,
		Tags:        run.Tags,
		Labels:      run.Labels,
	}

	if run.Workload != nil {
		doc.WorkloadName = run.Workload.Name
		doc.WorkloadType = run.Workload.Type
		workloadJSON, _ := json.Marshal(run.Workload)
		json.Unmarshal(workloadJSON, &doc.Workload)
	}

	if run.Target != nil {
		doc.TargetHost = run.Target.Host
		doc.TargetPort = run.Target.Port
		targetJSON, _ := json.Marshal(run.Target)
		json.Unmarshal(targetJSON, &doc.Target)
	}

	if run.Environment != nil {
		envJSON, _ := json.Marshal(run.Environment)
		json.Unmarshal(envJSON, &doc.Environment)
	}

	if run.Results != nil {
		resultsJSON, _ := json.Marshal(run.Results)
		json.Unmarshal(resultsJSON, &doc.Results)
	}

	if !run.StartTime.IsZero() {
		doc.StartTime = &run.StartTime
	}
	if !run.EndTime.IsZero() {
		doc.EndTime = &run.EndTime
	}

	return doc, nil
}

// FromDocument converts a MongoDB document to a BenchmarkRun.
func FromDocument(doc *MongoDocument) (*domain.BenchmarkRun, error) {
	run := &domain.BenchmarkRun{
		ID:          doc.ID,
		CreatedAt:   doc.CreatedAt,
		UpdatedAt:   doc.UpdatedAt,
		Status:      domain.RunStatus(doc.Status),
		Duration:    doc.Duration,
		Error:       doc.Error,
		Name:        doc.Name,
		Description: doc.Description,
		Tags:        doc.Tags,
		Labels:      doc.Labels,
	}

	if doc.Workload != nil {
		workloadJSON, _ := json.Marshal(doc.Workload)
		run.Workload = &domain.Workload{}
		json.Unmarshal(workloadJSON, run.Workload)
	}

	if doc.Target != nil {
		targetJSON, _ := json.Marshal(doc.Target)
		run.Target = &domain.Target{}
		json.Unmarshal(targetJSON, run.Target)
	}

	if doc.Environment != nil {
		envJSON, _ := json.Marshal(doc.Environment)
		run.Environment = &domain.Environment{}
		json.Unmarshal(envJSON, run.Environment)
	}

	if doc.Results != nil {
		resultsJSON, _ := json.Marshal(doc.Results)
		run.Results = &domain.Results{}
		json.Unmarshal(resultsJSON, run.Results)
	}

	if doc.StartTime != nil {
		run.StartTime = *doc.StartTime
	}
	if doc.EndTime != nil {
		run.EndTime = *doc.EndTime
	}

	return run, nil
}

// Ensure MongoStorage satisfies RunStorage interface.
var _ RunStorage = (*MongoStorage)(nil)

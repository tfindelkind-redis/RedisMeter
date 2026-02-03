// Package storage provides persistence backends for RedisMeter.
package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
	"github.com/tfindelkind-redis/redismeter/internal/plugin"
)

// SQLiteStorage implements StoragePlugin using SQLite database.
type SQLiteStorage struct {
	db     *sql.DB
	dbPath string
	mu     sync.RWMutex
}

// NewSQLiteStorage creates a new SQLite-based storage plugin.
func NewSQLiteStorage(dbPath string) (*SQLiteStorage, error) {
	if dbPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		dbPath = filepath.Join(home, ".redismeter", "redismeter.db")
	}

	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	storage := &SQLiteStorage{
		db:     db,
		dbPath: dbPath,
	}

	// Initialize schema
	if err := storage.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return storage, nil
}

// initSchema creates the database tables if they don't exist.
func (s *SQLiteStorage) initSchema() error {
	schema := `
	-- Schema version tracking
	CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER PRIMARY KEY,
		applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Benchmark runs table
	CREATE TABLE IF NOT EXISTS benchmark_runs (
		id TEXT PRIMARY KEY,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		status TEXT NOT NULL,
		workload_name TEXT,
		workload_type TEXT,
		target_host TEXT,
		target_port INTEGER,
		target_url TEXT,
		start_time DATETIME,
		end_time DATETIME,
		duration TEXT,
		error TEXT,
		name TEXT,
		description TEXT,
		-- JSON fields for complex data
		workload_json TEXT,
		target_json TEXT,
		environment_json TEXT,
		results_json TEXT,
		tags_json TEXT,
		labels_json TEXT
	);

	-- Indexes for common queries
	CREATE INDEX IF NOT EXISTS idx_runs_status ON benchmark_runs(status);
	CREATE INDEX IF NOT EXISTS idx_runs_workload ON benchmark_runs(workload_name);
	CREATE INDEX IF NOT EXISTS idx_runs_target ON benchmark_runs(target_host, target_port);
	CREATE INDEX IF NOT EXISTS idx_runs_created ON benchmark_runs(created_at);

	-- Tags table for efficient tag queries
	CREATE TABLE IF NOT EXISTS run_tags (
		run_id TEXT NOT NULL,
		tag TEXT NOT NULL,
		PRIMARY KEY (run_id, tag),
		FOREIGN KEY (run_id) REFERENCES benchmark_runs(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_tags_tag ON run_tags(tag);

	-- Baselines table
	CREATE TABLE IF NOT EXISTS baselines (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT,
		run_id TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		active INTEGER DEFAULT 1,
		valid_from DATETIME,
		valid_until DATETIME,
		metrics_json TEXT,
		environment_json TEXT,
		workload_json TEXT,
		thresholds_json TEXT,
		tags_json TEXT,
		labels_json TEXT,
		FOREIGN KEY (run_id) REFERENCES benchmark_runs(id) ON DELETE SET NULL
	);

	CREATE INDEX IF NOT EXISTS idx_baselines_name ON baselines(name);
	CREATE INDEX IF NOT EXISTS idx_baselines_run ON baselines(run_id);
	CREATE INDEX IF NOT EXISTS idx_baselines_active ON baselines(active);

	-- Insert initial schema version
	INSERT OR IGNORE INTO schema_version (version) VALUES (1);
	`

	_, err := s.db.Exec(schema)
	return err
}

// Metadata returns plugin metadata.
func (s *SQLiteStorage) Metadata() plugin.Metadata {
	return plugin.Metadata{
		Name:        "sqlite",
		Version:     "1.0.0",
		Type:        plugin.TypeStorage,
		Description: "SQLite database storage for benchmark data",
	}
}

// Initialize sets up the plugin.
func (s *SQLiteStorage) Initialize(ctx context.Context, config map[string]interface{}) error {
	return nil
}

// HealthCheck returns the plugin health status.
func (s *SQLiteStorage) HealthCheck(ctx context.Context) plugin.HealthStatus {
	if err := s.db.PingContext(ctx); err != nil {
		return plugin.HealthStatus{Healthy: false, Message: err.Error()}
	}
	return plugin.HealthStatus{Healthy: true, Message: "OK"}
}

// Shutdown gracefully stops the plugin.
func (s *SQLiteStorage) Shutdown(ctx context.Context) error {
	return s.db.Close()
}

// Save stores an entity and returns its ID.
func (s *SQLiteStorage) Save(ctx context.Context, entityType string, entity interface{}) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch entityType {
	case "runs":
		return s.saveRun(ctx, entity.(*domain.BenchmarkRun))
	case "baselines":
		return s.saveBaseline(ctx, entity.(*domain.Baseline))
	default:
		return "", fmt.Errorf("unsupported entity type: %s", entityType)
	}
}

// saveRun saves a benchmark run to the database.
func (s *SQLiteStorage) saveRun(ctx context.Context, run *domain.BenchmarkRun) (string, error) {
	workloadJSON, _ := json.Marshal(run.Workload)
	targetJSON, _ := json.Marshal(run.Target)
	environmentJSON, _ := json.Marshal(run.Environment)
	resultsJSON, _ := json.Marshal(run.Results)
	tagsJSON, _ := json.Marshal(run.Tags)
	labelsJSON, _ := json.Marshal(run.Labels)

	var workloadName, workloadType string
	if run.Workload != nil {
		workloadName = run.Workload.Name
		workloadType = run.Workload.Type
	}

	var targetHost, targetURL string
	var targetPort int
	if run.Target != nil {
		targetHost = run.Target.Host
		targetPort = run.Target.Port
		targetURL = run.Target.URL
	}

	query := `
	INSERT OR REPLACE INTO benchmark_runs (
		id, created_at, updated_at, status,
		workload_name, workload_type,
		target_host, target_port, target_url,
		start_time, end_time, duration, error,
		name, description,
		workload_json, target_json, environment_json, results_json, tags_json, labels_json
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query,
		run.ID, run.CreatedAt, time.Now(), run.Status,
		workloadName, workloadType,
		targetHost, targetPort, targetURL,
		run.StartTime, run.EndTime, run.Duration, run.Error,
		run.Name, run.Description,
		string(workloadJSON), string(targetJSON), string(environmentJSON),
		string(resultsJSON), string(tagsJSON), string(labelsJSON),
	)
	if err != nil {
		return "", fmt.Errorf("failed to save run: %w", err)
	}

	// Save tags
	if len(run.Tags) > 0 {
		// Delete existing tags
		s.db.ExecContext(ctx, "DELETE FROM run_tags WHERE run_id = ?", run.ID)

		// Insert new tags
		for _, tag := range run.Tags {
			_, err := s.db.ExecContext(ctx, "INSERT INTO run_tags (run_id, tag) VALUES (?, ?)", run.ID, tag)
			if err != nil {
				return "", fmt.Errorf("failed to save tag: %w", err)
			}
		}
	}

	return run.ID, nil
}

// saveBaseline saves a baseline to the database.
func (s *SQLiteStorage) saveBaseline(ctx context.Context, baseline *domain.Baseline) (string, error) {
	metricsJSON, _ := json.Marshal(baseline.Metrics)
	environmentJSON, _ := json.Marshal(baseline.Environment)
	workloadJSON, _ := json.Marshal(baseline.Workload)
	thresholdsJSON, _ := json.Marshal(baseline.Thresholds)
	tagsJSON, _ := json.Marshal(baseline.Tags)
	labelsJSON, _ := json.Marshal(baseline.Labels)

	query := `
	INSERT OR REPLACE INTO baselines (
		id, name, description, run_id, created_at, updated_at,
		active, valid_from, valid_until,
		metrics_json, environment_json, workload_json, thresholds_json,
		tags_json, labels_json
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var active int
	if baseline.Active {
		active = 1
	}

	// Convert empty run_id to nil for proper NULL handling
	var runID interface{}
	if baseline.RunID != "" {
		runID = baseline.RunID
	}

	_, err := s.db.ExecContext(ctx, query,
		baseline.ID, baseline.Name, baseline.Description, runID,
		baseline.CreatedAt, time.Now(),
		active, baseline.ValidFrom, baseline.ValidUntil,
		string(metricsJSON), string(environmentJSON), string(workloadJSON), string(thresholdsJSON),
		string(tagsJSON), string(labelsJSON),
	)
	if err != nil {
		return "", fmt.Errorf("failed to save baseline: %w", err)
	}

	return baseline.ID, nil
}

// Load retrieves an entity by ID.
func (s *SQLiteStorage) Load(ctx context.Context, entityType string, id string, dest interface{}) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	switch entityType {
	case "runs":
		return s.loadRun(ctx, id, dest.(*domain.BenchmarkRun))
	case "baselines":
		return s.loadBaseline(ctx, id, dest.(*domain.Baseline))
	default:
		return fmt.Errorf("unsupported entity type: %s", entityType)
	}
}

// loadRun loads a benchmark run from the database.
func (s *SQLiteStorage) loadRun(ctx context.Context, id string, run *domain.BenchmarkRun) error {
	query := `
	SELECT id, created_at, updated_at, status,
		start_time, end_time, duration, error,
		name, description,
		workload_json, target_json, environment_json, results_json, tags_json, labels_json
	FROM benchmark_runs WHERE id = ?
	`

	var workloadJSON, targetJSON, environmentJSON, resultsJSON, tagsJSON, labelsJSON sql.NullString
	var startTime, endTime sql.NullTime

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&run.ID, &run.CreatedAt, &run.UpdatedAt, &run.Status,
		&startTime, &endTime, &run.Duration, &run.Error,
		&run.Name, &run.Description,
		&workloadJSON, &targetJSON, &environmentJSON, &resultsJSON, &tagsJSON, &labelsJSON,
	)
	if err == sql.ErrNoRows {
		return fmt.Errorf("run not found: %s", id)
	}
	if err != nil {
		return fmt.Errorf("failed to load run: %w", err)
	}

	if startTime.Valid {
		run.StartTime = startTime.Time
	}
	if endTime.Valid {
		run.EndTime = endTime.Time
	}

	if workloadJSON.Valid {
		run.Workload = &domain.Workload{}
		json.Unmarshal([]byte(workloadJSON.String), run.Workload)
	}
	if targetJSON.Valid {
		run.Target = &domain.Target{}
		json.Unmarshal([]byte(targetJSON.String), run.Target)
	}
	if environmentJSON.Valid {
		run.Environment = &domain.Environment{}
		json.Unmarshal([]byte(environmentJSON.String), run.Environment)
	}
	if resultsJSON.Valid {
		run.Results = &domain.Results{}
		json.Unmarshal([]byte(resultsJSON.String), run.Results)
	}
	if tagsJSON.Valid {
		json.Unmarshal([]byte(tagsJSON.String), &run.Tags)
	}
	if labelsJSON.Valid {
		json.Unmarshal([]byte(labelsJSON.String), &run.Labels)
	}

	return nil
}

// loadBaseline loads a baseline from the database.
func (s *SQLiteStorage) loadBaseline(ctx context.Context, id string, baseline *domain.Baseline) error {
	query := `
	SELECT id, name, description, run_id, created_at, updated_at,
		active, valid_from, valid_until,
		metrics_json, environment_json, workload_json, thresholds_json,
		tags_json, labels_json
	FROM baselines WHERE id = ?
	`

	var metricsJSON, environmentJSON, workloadJSON, thresholdsJSON, tagsJSON, labelsJSON sql.NullString
	var runID sql.NullString
	var validFrom, validUntil sql.NullTime
	var active int

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&baseline.ID, &baseline.Name, &baseline.Description, &runID,
		&baseline.CreatedAt, &baseline.UpdatedAt,
		&active, &validFrom, &validUntil,
		&metricsJSON, &environmentJSON, &workloadJSON, &thresholdsJSON,
		&tagsJSON, &labelsJSON,
	)
	if err == sql.ErrNoRows {
		return fmt.Errorf("baseline not found: %s", id)
	}
	if err != nil {
		return fmt.Errorf("failed to load baseline: %w", err)
	}

	if runID.Valid {
		baseline.RunID = runID.String
	}
	baseline.Active = active == 1
	if validFrom.Valid {
		baseline.ValidFrom = validFrom.Time
	}
	if validUntil.Valid {
		baseline.ValidUntil = validUntil.Time
	}

	if metricsJSON.Valid {
		baseline.Metrics = &domain.SummaryMetrics{}
		json.Unmarshal([]byte(metricsJSON.String), baseline.Metrics)
	}
	if environmentJSON.Valid {
		baseline.Environment = &domain.EnvironmentConstraints{}
		json.Unmarshal([]byte(environmentJSON.String), baseline.Environment)
	}
	if workloadJSON.Valid {
		baseline.Workload = &domain.Workload{}
		json.Unmarshal([]byte(workloadJSON.String), baseline.Workload)
	}
	if thresholdsJSON.Valid {
		baseline.Thresholds = &domain.BaselineThresholds{}
		json.Unmarshal([]byte(thresholdsJSON.String), baseline.Thresholds)
	}
	if tagsJSON.Valid {
		json.Unmarshal([]byte(tagsJSON.String), &baseline.Tags)
	}
	if labelsJSON.Valid {
		json.Unmarshal([]byte(labelsJSON.String), &baseline.Labels)
	}

	return nil
}

// Query finds entities matching the given filter.
func (s *SQLiteStorage) Query(ctx context.Context, entityType string, filter plugin.QueryFilter, dest interface{}) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	switch entityType {
	case "runs":
		return s.queryRuns(ctx, filter, dest)
	case "baselines":
		return s.queryBaselines(ctx, filter, dest)
	default:
		return fmt.Errorf("unsupported entity type: %s", entityType)
	}
}

// queryRuns queries benchmark runs with filtering.
func (s *SQLiteStorage) queryRuns(ctx context.Context, filter plugin.QueryFilter, dest interface{}) error {
	query := `
	SELECT id, created_at, updated_at, status,
		start_time, end_time, duration, error,
		name, description,
		workload_json, target_json, environment_json, results_json, tags_json, labels_json
	FROM benchmark_runs
	WHERE 1=1
	`

	args := []interface{}{}

	// Apply conditions filter
	if filter.Conditions != nil {
		if status, ok := filter.Conditions["status"].(string); ok && status != "" {
			query += " AND status = ?"
			args = append(args, status)
		}
		if workload, ok := filter.Conditions["workload"].(string); ok && workload != "" {
			query += " AND workload_name = ?"
			args = append(args, workload)
		}
		if target, ok := filter.Conditions["target"].(string); ok && target != "" {
			query += " AND (target_host LIKE ? OR target_url LIKE ?)"
			args = append(args, "%"+target+"%", "%"+target+"%")
		}
	}

	// Apply tag filter
	if len(filter.Tags) > 0 {
		placeholders := make([]string, len(filter.Tags))
		for i, tag := range filter.Tags {
			placeholders[i] = "?"
			args = append(args, tag)
		}
		query += fmt.Sprintf(` AND id IN (SELECT run_id FROM run_tags WHERE tag IN (%s))`, strings.Join(placeholders, ","))
	}

	// Apply time range filter
	if filter.TimeRange != nil {
		if filter.TimeRange.Start != "" {
			query += " AND created_at >= ?"
			args = append(args, filter.TimeRange.Start)
		}
		if filter.TimeRange.End != "" {
			query += " AND created_at <= ?"
			args = append(args, filter.TimeRange.End)
		}
	}

	// Order by
	orderBy := "created_at"
	if filter.OrderBy != "" {
		// Map allowed fields
		allowedFields := map[string]string{
			"created_at": "created_at",
			"updated_at": "updated_at",
			"status":     "status",
			"workload":   "workload_name",
			"duration":   "duration",
		}
		if field, ok := allowedFields[filter.OrderBy]; ok {
			orderBy = field
		}
	}
	if filter.Descending {
		query += fmt.Sprintf(" ORDER BY %s DESC", orderBy)
	} else {
		query += fmt.Sprintf(" ORDER BY %s DESC", orderBy) // Default to DESC for newest first
	}

	// Apply limit
	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}

	// Apply offset
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to query runs: %w", err)
	}
	defer rows.Close()

	var results []*domain.BenchmarkRun

	for rows.Next() {
		run := &domain.BenchmarkRun{}
		var workloadJSON, targetJSON, environmentJSON, resultsJSON, tagsJSON, labelsJSON sql.NullString
		var startTime, endTime sql.NullTime

		err := rows.Scan(
			&run.ID, &run.CreatedAt, &run.UpdatedAt, &run.Status,
			&startTime, &endTime, &run.Duration, &run.Error,
			&run.Name, &run.Description,
			&workloadJSON, &targetJSON, &environmentJSON, &resultsJSON, &tagsJSON, &labelsJSON,
		)
		if err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		if startTime.Valid {
			run.StartTime = startTime.Time
		}
		if endTime.Valid {
			run.EndTime = endTime.Time
		}

		if workloadJSON.Valid {
			run.Workload = &domain.Workload{}
			json.Unmarshal([]byte(workloadJSON.String), run.Workload)
		}
		if targetJSON.Valid {
			run.Target = &domain.Target{}
			json.Unmarshal([]byte(targetJSON.String), run.Target)
		}
		if environmentJSON.Valid {
			run.Environment = &domain.Environment{}
			json.Unmarshal([]byte(environmentJSON.String), run.Environment)
		}
		if resultsJSON.Valid {
			run.Results = &domain.Results{}
			json.Unmarshal([]byte(resultsJSON.String), run.Results)
		}
		if tagsJSON.Valid {
			json.Unmarshal([]byte(tagsJSON.String), &run.Tags)
		}
		if labelsJSON.Valid {
			json.Unmarshal([]byte(labelsJSON.String), &run.Labels)
		}

		results = append(results, run)
	}

	// Copy to destination
	if runsPtr, ok := dest.(*[]*domain.BenchmarkRun); ok {
		*runsPtr = results
	}

	return nil
}

// queryBaselines queries baselines with filtering.
func (s *SQLiteStorage) queryBaselines(ctx context.Context, filter plugin.QueryFilter, dest interface{}) error {
	query := `
	SELECT id, name, description, run_id, created_at, updated_at,
		active, valid_from, valid_until,
		metrics_json, environment_json, workload_json, thresholds_json,
		tags_json, labels_json
	FROM baselines
	WHERE 1=1
	`

	args := []interface{}{}

	// Apply conditions
	if filter.Conditions != nil {
		if name, ok := filter.Conditions["name"].(string); ok && name != "" {
			query += " AND name LIKE ?"
			args = append(args, "%"+name+"%")
		}
		if active, ok := filter.Conditions["active"].(bool); ok {
			activeVal := 0
			if active {
				activeVal = 1
			}
			query += " AND active = ?"
			args = append(args, activeVal)
		}
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to query baselines: %w", err)
	}
	defer rows.Close()

	var results []*domain.Baseline

	for rows.Next() {
		baseline := &domain.Baseline{}
		var metricsJSON, environmentJSON, workloadJSON, thresholdsJSON, tagsJSON, labelsJSON sql.NullString
		var runID sql.NullString
		var validFrom, validUntil sql.NullTime
		var active int

		err := rows.Scan(
			&baseline.ID, &baseline.Name, &baseline.Description, &runID,
			&baseline.CreatedAt, &baseline.UpdatedAt,
			&active, &validFrom, &validUntil,
			&metricsJSON, &environmentJSON, &workloadJSON, &thresholdsJSON,
			&tagsJSON, &labelsJSON,
		)
		if err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		if runID.Valid {
			baseline.RunID = runID.String
		}
		baseline.Active = active == 1
		if validFrom.Valid {
			baseline.ValidFrom = validFrom.Time
		}
		if validUntil.Valid {
			baseline.ValidUntil = validUntil.Time
		}

		if metricsJSON.Valid {
			baseline.Metrics = &domain.SummaryMetrics{}
			json.Unmarshal([]byte(metricsJSON.String), baseline.Metrics)
		}
		if environmentJSON.Valid {
			baseline.Environment = &domain.EnvironmentConstraints{}
			json.Unmarshal([]byte(environmentJSON.String), baseline.Environment)
		}
		if workloadJSON.Valid {
			baseline.Workload = &domain.Workload{}
			json.Unmarshal([]byte(workloadJSON.String), baseline.Workload)
		}
		if thresholdsJSON.Valid {
			baseline.Thresholds = &domain.BaselineThresholds{}
			json.Unmarshal([]byte(thresholdsJSON.String), baseline.Thresholds)
		}
		if tagsJSON.Valid {
			json.Unmarshal([]byte(tagsJSON.String), &baseline.Tags)
		}
		if labelsJSON.Valid {
			json.Unmarshal([]byte(labelsJSON.String), &baseline.Labels)
		}

		results = append(results, baseline)
	}

	// Copy to destination
	if baselinesPtr, ok := dest.(*[]*domain.Baseline); ok {
		*baselinesPtr = results
	}

	return nil
}

// Delete removes an entity by ID.
func (s *SQLiteStorage) Delete(ctx context.Context, entityType string, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var query string
	switch entityType {
	case "runs":
		query = "DELETE FROM benchmark_runs WHERE id = ?"
	case "baselines":
		query = "DELETE FROM baselines WHERE id = ?"
	default:
		return fmt.Errorf("unsupported entity type: %s", entityType)
	}

	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("entity not found: %s/%s", entityType, id)
	}

	return nil
}

// ListRuns returns all benchmark runs (convenience method).
func (s *SQLiteStorage) ListRuns(ctx context.Context, limit int) ([]*domain.BenchmarkRun, error) {
	var runs []*domain.BenchmarkRun
	filter := plugin.QueryFilter{Limit: limit}
	if err := s.Query(ctx, "runs", filter, &runs); err != nil {
		return nil, err
	}
	return runs, nil
}

// GetRun retrieves a single benchmark run by ID (convenience method).
func (s *SQLiteStorage) GetRun(ctx context.Context, id string) (*domain.BenchmarkRun, error) {
	var run domain.BenchmarkRun
	if err := s.Load(ctx, "runs", id, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

// DeleteRun removes a benchmark run by ID (convenience method).
func (s *SQLiteStorage) DeleteRun(ctx context.Context, id string) error {
	return s.Delete(ctx, "runs", id)
}

// SaveRun stores a benchmark run (convenience method).
func (s *SQLiteStorage) SaveRun(ctx context.Context, run *domain.BenchmarkRun) error {
	_, err := s.Save(ctx, "runs", run)
	return err
}

// GetStats returns storage statistics.
func (s *SQLiteStorage) GetStats(ctx context.Context) (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]interface{})

	// Count runs by status
	rows, err := s.db.QueryContext(ctx, "SELECT status, COUNT(*) FROM benchmark_runs GROUP BY status")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	runsByStatus := make(map[string]int)
	totalRuns := 0
	for rows.Next() {
		var status string
		var count int
		rows.Scan(&status, &count)
		runsByStatus[status] = count
		totalRuns += count
	}
	stats["runs_by_status"] = runsByStatus
	stats["total_runs"] = totalRuns

	// Count baselines
	var baselineCount int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM baselines").Scan(&baselineCount)
	stats["total_baselines"] = baselineCount

	// Database file size
	if info, err := os.Stat(s.dbPath); err == nil {
		stats["db_size_bytes"] = info.Size()
	}

	return stats, nil
}

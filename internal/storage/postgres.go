// Package storage provides persistence backends for RedisMeter.
package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
	"github.com/tfindelkind-redis/redismeter/internal/plugin"
)

// PostgresConfig holds configuration for PostgreSQL storage.
type PostgresConfig struct {
	// Connection string (e.g., "postgres://user:pass@localhost:5432/dbname?sslmode=disable")
	ConnectionString string `json:"connection_string"`

	// Or individual parameters
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
	SSLMode  string `json:"ssl_mode"` // disable, require, verify-ca, verify-full

	// Connection pool settings
	MaxOpenConns    int           `json:"max_open_conns"`
	MaxIdleConns    int           `json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `json:"conn_max_idle_time"`

	// Schema settings
	Schema          string `json:"schema"` // defaults to "redismeter"
	AutoMigrate     bool   `json:"auto_migrate"`
	MigrationTable  string `json:"migration_table"`
}

// DefaultPostgresConfig returns sensible defaults for PostgreSQL.
func DefaultPostgresConfig() PostgresConfig {
	return PostgresConfig{
		Host:            "localhost",
		Port:            5432,
		SSLMode:         "disable",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
		Schema:          "redismeter",
		AutoMigrate:     true,
		MigrationTable:  "schema_migrations",
	}
}

// PostgresStorage implements StoragePlugin using PostgreSQL database.
type PostgresStorage struct {
	db     *sql.DB
	config PostgresConfig
	mu     sync.RWMutex
}

// NewPostgresStorage creates a new PostgreSQL-based storage plugin.
func NewPostgresStorage(config PostgresConfig) (*PostgresStorage, error) {
	connStr := config.ConnectionString
	if connStr == "" {
		connStr = buildPostgresConnString(config)
	}

	// Check for environment variable override
	if envConn := os.Getenv("REDISMETER_POSTGRES_URL"); envConn != "" {
		connStr = envConn
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	if config.MaxOpenConns > 0 {
		db.SetMaxOpenConns(config.MaxOpenConns)
	}
	if config.MaxIdleConns > 0 {
		db.SetMaxIdleConns(config.MaxIdleConns)
	}
	if config.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(config.ConnMaxLifetime)
	}
	if config.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(config.ConnMaxIdleTime)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	storage := &PostgresStorage{
		db:     db,
		config: config,
	}

	// Initialize schema
	if config.AutoMigrate {
		if err := storage.migrate(ctx); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}
	}

	return storage, nil
}

func buildPostgresConnString(config PostgresConfig) string {
	port := config.Port
	if port == 0 {
		port = 5432
	}

	sslMode := config.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host, port, config.User, config.Password, config.Database, sslMode)
}

// migrate runs database migrations.
func (s *PostgresStorage) migrate(ctx context.Context) error {
	schema := s.config.Schema
	if schema == "" {
		schema = "redismeter"
	}

	migrations := []struct {
		version int
		sql     string
	}{
		{1, fmt.Sprintf(`
			-- Create schema
			CREATE SCHEMA IF NOT EXISTS %s;

			-- Migration tracking table
			CREATE TABLE IF NOT EXISTS %s.%s (
				version INTEGER PRIMARY KEY,
				applied_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
			);

			-- Benchmark runs table
			CREATE TABLE IF NOT EXISTS %s.benchmark_runs (
				id TEXT PRIMARY KEY,
				created_at TIMESTAMPTZ NOT NULL,
				updated_at TIMESTAMPTZ NOT NULL,
				status TEXT NOT NULL,
				workload_name TEXT,
				workload_type TEXT,
				target_host TEXT,
				target_port INTEGER,
				target_url TEXT,
				start_time TIMESTAMPTZ,
				end_time TIMESTAMPTZ,
				duration TEXT,
				error TEXT,
				name TEXT,
				description TEXT,
				workload JSONB,
				target JSONB,
				environment JSONB,
				results JSONB,
				tags TEXT[],
				labels JSONB
			);

			-- Indexes for benchmark_runs
			CREATE INDEX IF NOT EXISTS idx_runs_status ON %s.benchmark_runs(status);
			CREATE INDEX IF NOT EXISTS idx_runs_workload ON %s.benchmark_runs(workload_name);
			CREATE INDEX IF NOT EXISTS idx_runs_target ON %s.benchmark_runs(target_host, target_port);
			CREATE INDEX IF NOT EXISTS idx_runs_created ON %s.benchmark_runs(created_at DESC);
			CREATE INDEX IF NOT EXISTS idx_runs_tags ON %s.benchmark_runs USING GIN(tags);
			CREATE INDEX IF NOT EXISTS idx_runs_labels ON %s.benchmark_runs USING GIN(labels);

			-- Baselines table
			CREATE TABLE IF NOT EXISTS %s.baselines (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				description TEXT,
				run_id TEXT REFERENCES %s.benchmark_runs(id) ON DELETE SET NULL,
				created_at TIMESTAMPTZ NOT NULL,
				updated_at TIMESTAMPTZ NOT NULL,
				active BOOLEAN DEFAULT true,
				valid_from TIMESTAMPTZ,
				valid_until TIMESTAMPTZ,
				metrics JSONB,
				environment JSONB,
				workload JSONB,
				thresholds JSONB,
				tags TEXT[],
				labels JSONB
			);

			-- Indexes for baselines
			CREATE INDEX IF NOT EXISTS idx_baselines_name ON %s.baselines(name);
			CREATE INDEX IF NOT EXISTS idx_baselines_run ON %s.baselines(run_id);
			CREATE INDEX IF NOT EXISTS idx_baselines_active ON %s.baselines(active) WHERE active = true;
			CREATE INDEX IF NOT EXISTS idx_baselines_tags ON %s.baselines USING GIN(tags);
		`, schema, schema, s.config.MigrationTable,
			schema,
			schema, schema, schema, schema, schema, schema,
			schema, schema,
			schema, schema, schema, schema)},
		{2, fmt.Sprintf(`
			-- Add full-text search support
			ALTER TABLE %s.benchmark_runs ADD COLUMN IF NOT EXISTS search_vector tsvector;
			
			CREATE OR REPLACE FUNCTION %s.update_run_search_vector() RETURNS trigger AS $$
			BEGIN
				NEW.search_vector := to_tsvector('english',
					COALESCE(NEW.name, '') || ' ' ||
					COALESCE(NEW.description, '') || ' ' ||
					COALESCE(NEW.workload_name, '') || ' ' ||
					COALESCE(NEW.target_host, '')
				);
				RETURN NEW;
			END;
			$$ LANGUAGE plpgsql;

			DROP TRIGGER IF EXISTS trigger_update_run_search ON %s.benchmark_runs;
			CREATE TRIGGER trigger_update_run_search
				BEFORE INSERT OR UPDATE ON %s.benchmark_runs
				FOR EACH ROW EXECUTE FUNCTION %s.update_run_search_vector();

			CREATE INDEX IF NOT EXISTS idx_runs_search ON %s.benchmark_runs USING GIN(search_vector);

			-- Update existing rows
			UPDATE %s.benchmark_runs SET search_vector = to_tsvector('english',
				COALESCE(name, '') || ' ' ||
				COALESCE(description, '') || ' ' ||
				COALESCE(workload_name, '') || ' ' ||
				COALESCE(target_host, '')
			) WHERE search_vector IS NULL;
		`, schema, schema, schema, schema, schema, schema, schema)},
		{3, fmt.Sprintf(`
			-- Add organizations and teams support
			CREATE TABLE IF NOT EXISTS %s.organizations (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL UNIQUE,
				description TEXT,
				created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
				settings JSONB DEFAULT '{}'::jsonb
			);

			CREATE TABLE IF NOT EXISTS %s.teams (
				id TEXT PRIMARY KEY,
				org_id TEXT NOT NULL REFERENCES %s.organizations(id) ON DELETE CASCADE,
				name TEXT NOT NULL,
				description TEXT,
				created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(org_id, name)
			);

			CREATE TABLE IF NOT EXISTS %s.users (
				id TEXT PRIMARY KEY,
				email TEXT NOT NULL UNIQUE,
				name TEXT,
				password_hash TEXT,
				created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
				last_login TIMESTAMPTZ,
				settings JSONB DEFAULT '{}'::jsonb,
				active BOOLEAN DEFAULT true
			);

			CREATE TABLE IF NOT EXISTS %s.team_members (
				team_id TEXT NOT NULL REFERENCES %s.teams(id) ON DELETE CASCADE,
				user_id TEXT NOT NULL REFERENCES %s.users(id) ON DELETE CASCADE,
				role TEXT NOT NULL DEFAULT 'member',
				joined_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY(team_id, user_id)
			);

			CREATE TABLE IF NOT EXISTS %s.org_members (
				org_id TEXT NOT NULL REFERENCES %s.organizations(id) ON DELETE CASCADE,
				user_id TEXT NOT NULL REFERENCES %s.users(id) ON DELETE CASCADE,
				role TEXT NOT NULL DEFAULT 'member',
				joined_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY(org_id, user_id)
			);

			-- Add org_id to benchmark_runs for multi-tenancy
			ALTER TABLE %s.benchmark_runs ADD COLUMN IF NOT EXISTS org_id TEXT REFERENCES %s.organizations(id);
			ALTER TABLE %s.baselines ADD COLUMN IF NOT EXISTS org_id TEXT REFERENCES %s.organizations(id);

			CREATE INDEX IF NOT EXISTS idx_runs_org ON %s.benchmark_runs(org_id);
			CREATE INDEX IF NOT EXISTS idx_baselines_org ON %s.baselines(org_id);
		`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema,
			schema, schema, schema, schema, schema, schema)},
		{4, fmt.Sprintf(`
			-- API keys for programmatic access
			CREATE TABLE IF NOT EXISTS %s.api_keys (
				id TEXT PRIMARY KEY,
				user_id TEXT NOT NULL REFERENCES %s.users(id) ON DELETE CASCADE,
				name TEXT NOT NULL,
				key_hash TEXT NOT NULL UNIQUE,
				prefix TEXT NOT NULL,
				scopes TEXT[] DEFAULT ARRAY['read'],
				created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
				expires_at TIMESTAMPTZ,
				last_used_at TIMESTAMPTZ,
				active BOOLEAN DEFAULT true
			);

			CREATE INDEX IF NOT EXISTS idx_api_keys_user ON %s.api_keys(user_id);
			CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON %s.api_keys(prefix);

			-- Audit log
			CREATE TABLE IF NOT EXISTS %s.audit_log (
				id BIGSERIAL PRIMARY KEY,
				timestamp TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
				user_id TEXT,
				action TEXT NOT NULL,
				resource_type TEXT,
				resource_id TEXT,
				details JSONB,
				ip_address INET,
				user_agent TEXT
			);

			CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON %s.audit_log(timestamp DESC);
			CREATE INDEX IF NOT EXISTS idx_audit_user ON %s.audit_log(user_id);
			CREATE INDEX IF NOT EXISTS idx_audit_resource ON %s.audit_log(resource_type, resource_id);
		`, schema, schema, schema, schema, schema, schema, schema, schema)},
	}

	migrationTable := s.config.MigrationTable
	if migrationTable == "" {
		migrationTable = "schema_migrations"
	}

	for _, m := range migrations {
		// Check if migration already applied
		var exists bool
		err := s.db.QueryRowContext(ctx, fmt.Sprintf(
			"SELECT EXISTS(SELECT 1 FROM %s.%s WHERE version = $1)",
			schema, migrationTable), m.version).Scan(&exists)

		// Table might not exist yet
		if err != nil && m.version == 1 {
			exists = false
		} else if err != nil {
			return fmt.Errorf("failed to check migration %d: %w", m.version, err)
		}

		if exists {
			continue
		}

		// Run migration
		if _, err := s.db.ExecContext(ctx, m.sql); err != nil {
			return fmt.Errorf("failed to run migration %d: %w", m.version, err)
		}

		// Record migration
		if m.version > 1 {
			_, err = s.db.ExecContext(ctx, fmt.Sprintf(
				"INSERT INTO %s.%s (version) VALUES ($1)",
				schema, migrationTable), m.version)
			if err != nil {
				return fmt.Errorf("failed to record migration %d: %w", m.version, err)
			}
		}
	}

	return nil
}

// Metadata returns plugin metadata.
func (s *PostgresStorage) Metadata() plugin.Metadata {
	return plugin.Metadata{
		Name:        "postgres",
		Version:     "1.0.0",
		Type:        plugin.TypeStorage,
		Description: "PostgreSQL database storage with full enterprise features",
	}
}

// Initialize sets up the plugin.
func (s *PostgresStorage) Initialize(ctx context.Context, config map[string]interface{}) error {
	return nil
}

// HealthCheck returns the plugin health status.
func (s *PostgresStorage) HealthCheck(ctx context.Context) plugin.HealthStatus {
	if err := s.db.PingContext(ctx); err != nil {
		return plugin.HealthStatus{Healthy: false, Message: err.Error()}
	}
	return plugin.HealthStatus{Healthy: true, Message: "OK"}
}

// Shutdown gracefully stops the plugin.
func (s *PostgresStorage) Shutdown(ctx context.Context) error {
	return s.db.Close()
}

// Save stores an entity and returns its ID.
func (s *PostgresStorage) Save(ctx context.Context, entityType string, entity interface{}) (string, error) {
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

func (s *PostgresStorage) saveRun(ctx context.Context, run *domain.BenchmarkRun) (string, error) {
	schema := s.config.Schema
	if schema == "" {
		schema = "redismeter"
	}

	workloadJSON, _ := json.Marshal(run.Workload)
	targetJSON, _ := json.Marshal(run.Target)
	environmentJSON, _ := json.Marshal(run.Environment)
	resultsJSON, _ := json.Marshal(run.Results)
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

	query := fmt.Sprintf(`
		INSERT INTO %s.benchmark_runs (
			id, created_at, updated_at, status,
			workload_name, workload_type,
			target_host, target_port, target_url,
			start_time, end_time, duration, error,
			name, description,
			workload, target, environment, results, tags, labels
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21
		)
		ON CONFLICT (id) DO UPDATE SET
			updated_at = EXCLUDED.updated_at,
			status = EXCLUDED.status,
			start_time = EXCLUDED.start_time,
			end_time = EXCLUDED.end_time,
			duration = EXCLUDED.duration,
			error = EXCLUDED.error,
			results = EXCLUDED.results,
			tags = EXCLUDED.tags,
			labels = EXCLUDED.labels
	`, schema)

	_, err := s.db.ExecContext(ctx, query,
		run.ID, run.CreatedAt, time.Now(), run.Status,
		workloadName, workloadType,
		targetHost, targetPort, targetURL,
		nullTime(run.StartTime), nullTime(run.EndTime), run.Duration, run.Error,
		run.Name, run.Description,
		string(workloadJSON), string(targetJSON), string(environmentJSON),
		string(resultsJSON), run.Tags, string(labelsJSON),
	)
	if err != nil {
		return "", fmt.Errorf("failed to save run: %w", err)
	}

	return run.ID, nil
}

func (s *PostgresStorage) saveBaseline(ctx context.Context, baseline *domain.Baseline) (string, error) {
	schema := s.config.Schema
	if schema == "" {
		schema = "redismeter"
	}

	metricsJSON, _ := json.Marshal(baseline.Metrics)
	environmentJSON, _ := json.Marshal(baseline.Environment)
	workloadJSON, _ := json.Marshal(baseline.Workload)
	thresholdsJSON, _ := json.Marshal(baseline.Thresholds)
	labelsJSON, _ := json.Marshal(baseline.Labels)

	query := fmt.Sprintf(`
		INSERT INTO %s.baselines (
			id, name, description, run_id, created_at, updated_at,
			active, valid_from, valid_until,
			metrics, environment, workload, thresholds, tags, labels
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
		)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			updated_at = EXCLUDED.updated_at,
			active = EXCLUDED.active,
			valid_from = EXCLUDED.valid_from,
			valid_until = EXCLUDED.valid_until,
			metrics = EXCLUDED.metrics,
			thresholds = EXCLUDED.thresholds,
			tags = EXCLUDED.tags,
			labels = EXCLUDED.labels
	`, schema)

	var runID interface{}
	if baseline.RunID != "" {
		runID = baseline.RunID
	}

	_, err := s.db.ExecContext(ctx, query,
		baseline.ID, baseline.Name, baseline.Description, runID,
		baseline.CreatedAt, time.Now(),
		baseline.Active, nullTime(baseline.ValidFrom), nullTime(baseline.ValidUntil),
		string(metricsJSON), string(environmentJSON), string(workloadJSON), string(thresholdsJSON),
		baseline.Tags, string(labelsJSON),
	)
	if err != nil {
		return "", fmt.Errorf("failed to save baseline: %w", err)
	}

	return baseline.ID, nil
}

// Load retrieves an entity by ID.
func (s *PostgresStorage) Load(ctx context.Context, entityType string, id string, dest interface{}) error {
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

func (s *PostgresStorage) loadRun(ctx context.Context, id string, run *domain.BenchmarkRun) error {
	schema := s.config.Schema
	if schema == "" {
		schema = "redismeter"
	}

	query := fmt.Sprintf(`
		SELECT id, created_at, updated_at, status,
			start_time, end_time, duration, error,
			name, description,
			workload, target, environment, results, tags, labels
		FROM %s.benchmark_runs WHERE id = $1
	`, schema)

	var workloadJSON, targetJSON, environmentJSON, resultsJSON, labelsJSON sql.NullString
	var startTime, endTime sql.NullTime
	var tags []string

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&run.ID, &run.CreatedAt, &run.UpdatedAt, &run.Status,
		&startTime, &endTime, &run.Duration, &run.Error,
		&run.Name, &run.Description,
		&workloadJSON, &targetJSON, &environmentJSON, &resultsJSON, &tags, &labelsJSON,
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
	run.Tags = tags

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
	if labelsJSON.Valid {
		json.Unmarshal([]byte(labelsJSON.String), &run.Labels)
	}

	return nil
}

func (s *PostgresStorage) loadBaseline(ctx context.Context, id string, baseline *domain.Baseline) error {
	schema := s.config.Schema
	if schema == "" {
		schema = "redismeter"
	}

	query := fmt.Sprintf(`
		SELECT id, name, description, run_id, created_at, updated_at,
			active, valid_from, valid_until,
			metrics, environment, workload, thresholds, tags, labels
		FROM %s.baselines WHERE id = $1
	`, schema)

	var metricsJSON, environmentJSON, workloadJSON, thresholdsJSON, labelsJSON sql.NullString
	var runID sql.NullString
	var validFrom, validUntil sql.NullTime
	var tags []string

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&baseline.ID, &baseline.Name, &baseline.Description, &runID,
		&baseline.CreatedAt, &baseline.UpdatedAt,
		&baseline.Active, &validFrom, &validUntil,
		&metricsJSON, &environmentJSON, &workloadJSON, &thresholdsJSON, &tags, &labelsJSON,
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
	if validFrom.Valid {
		baseline.ValidFrom = validFrom.Time
	}
	if validUntil.Valid {
		baseline.ValidUntil = validUntil.Time
	}
	baseline.Tags = tags

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
		json.Unmarshal([]byte(thresholdsJSON.String), &baseline.Thresholds)
	}
	if labelsJSON.Valid {
		json.Unmarshal([]byte(labelsJSON.String), &baseline.Labels)
	}

	return nil
}

// Query finds entities matching the given filter.
func (s *PostgresStorage) Query(ctx context.Context, entityType string, filter plugin.QueryFilter, dest interface{}) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	switch entityType {
	case "runs":
		runs, err := s.queryRuns(ctx, filter)
		if err != nil {
			return err
		}
		*dest.(*[]*domain.BenchmarkRun) = runs
		return nil
	case "baselines":
		baselines, err := s.queryBaselines(ctx, filter)
		if err != nil {
			return err
		}
		*dest.(*[]*domain.Baseline) = baselines
		return nil
	default:
		return fmt.Errorf("unsupported entity type: %s", entityType)
	}
}

func (s *PostgresStorage) queryRuns(ctx context.Context, filter plugin.QueryFilter) ([]*domain.BenchmarkRun, error) {
	schema := s.config.Schema
	if schema == "" {
		schema = "redismeter"
	}

	query := fmt.Sprintf(`
		SELECT id, created_at, updated_at, status,
			start_time, end_time, duration, error,
			name, description,
			workload, target, environment, results, tags, labels
		FROM %s.benchmark_runs
	`, schema)

	var conditions []string
	var args []interface{}
	argNum := 1

	// Build WHERE clause
	if filter.Conditions != nil {
		for field, value := range filter.Conditions {
			switch field {
			case "status":
				conditions = append(conditions, fmt.Sprintf("status = $%d", argNum))
				args = append(args, value)
				argNum++
			case "workload":
				conditions = append(conditions, fmt.Sprintf("workload_name = $%d", argNum))
				args = append(args, value)
				argNum++
			case "target":
				conditions = append(conditions, fmt.Sprintf("target_host = $%d", argNum))
				args = append(args, value)
				argNum++
			case "org_id":
				conditions = append(conditions, fmt.Sprintf("org_id = $%d", argNum))
				args = append(args, value)
				argNum++
			case "search":
				conditions = append(conditions, fmt.Sprintf("search_vector @@ plainto_tsquery('english', $%d)", argNum))
				args = append(args, value)
				argNum++
			}
		}
	}

	if len(filter.Tags) > 0 {
		conditions = append(conditions, fmt.Sprintf("tags && $%d", argNum))
		args = append(args, filter.Tags)
		argNum++
	}

	if filter.TimeRange != nil {
		if filter.TimeRange.Start != "" {
			conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argNum))
			args = append(args, filter.TimeRange.Start)
			argNum++
		}
		if filter.TimeRange.End != "" {
			conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argNum))
			args = append(args, filter.TimeRange.End)
			argNum++
		}
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	// ORDER BY
	orderBy := "created_at"
	if filter.OrderBy != "" {
		orderBy = filter.OrderBy
	}
	direction := "DESC"
	if !filter.Descending {
		direction = "ASC"
	}
	query += fmt.Sprintf(" ORDER BY %s %s", orderBy, direction)

	// LIMIT and OFFSET
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query runs: %w", err)
	}
	defer rows.Close()

	var runs []*domain.BenchmarkRun
	for rows.Next() {
		run := &domain.BenchmarkRun{}
		var workloadJSON, targetJSON, environmentJSON, resultsJSON, labelsJSON sql.NullString
		var startTime, endTime sql.NullTime
		var tags []string

		err := rows.Scan(
			&run.ID, &run.CreatedAt, &run.UpdatedAt, &run.Status,
			&startTime, &endTime, &run.Duration, &run.Error,
			&run.Name, &run.Description,
			&workloadJSON, &targetJSON, &environmentJSON, &resultsJSON, &tags, &labelsJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan run: %w", err)
		}

		if startTime.Valid {
			run.StartTime = startTime.Time
		}
		if endTime.Valid {
			run.EndTime = endTime.Time
		}
		run.Tags = tags

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
		if labelsJSON.Valid {
			json.Unmarshal([]byte(labelsJSON.String), &run.Labels)
		}

		runs = append(runs, run)
	}

	return runs, nil
}

func (s *PostgresStorage) queryBaselines(ctx context.Context, filter plugin.QueryFilter) ([]*domain.Baseline, error) {
	schema := s.config.Schema
	if schema == "" {
		schema = "redismeter"
	}

	query := fmt.Sprintf(`
		SELECT id, name, description, run_id, created_at, updated_at,
			active, valid_from, valid_until,
			metrics, environment, workload, thresholds, tags, labels
		FROM %s.baselines
	`, schema)

	var conditions []string
	var args []interface{}
	argNum := 1

	if filter.Conditions != nil {
		for field, value := range filter.Conditions {
			switch field {
			case "active":
				conditions = append(conditions, fmt.Sprintf("active = $%d", argNum))
				args = append(args, value)
				argNum++
			case "name":
				conditions = append(conditions, fmt.Sprintf("name = $%d", argNum))
				args = append(args, value)
				argNum++
			case "org_id":
				conditions = append(conditions, fmt.Sprintf("org_id = $%d", argNum))
				args = append(args, value)
				argNum++
			}
		}
	}

	if len(filter.Tags) > 0 {
		conditions = append(conditions, fmt.Sprintf("tags && $%d", argNum))
		args = append(args, filter.Tags)
		argNum++
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	orderBy := "created_at"
	if filter.OrderBy != "" {
		orderBy = filter.OrderBy
	}
	direction := "DESC"
	if !filter.Descending {
		direction = "ASC"
	}
	query += fmt.Sprintf(" ORDER BY %s %s", orderBy, direction)

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query baselines: %w", err)
	}
	defer rows.Close()

	var baselines []*domain.Baseline
	for rows.Next() {
		baseline := &domain.Baseline{}
		var metricsJSON, environmentJSON, workloadJSON, thresholdsJSON, labelsJSON sql.NullString
		var runID sql.NullString
		var validFrom, validUntil sql.NullTime
		var tags []string

		err := rows.Scan(
			&baseline.ID, &baseline.Name, &baseline.Description, &runID,
			&baseline.CreatedAt, &baseline.UpdatedAt,
			&baseline.Active, &validFrom, &validUntil,
			&metricsJSON, &environmentJSON, &workloadJSON, &thresholdsJSON, &tags, &labelsJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan baseline: %w", err)
		}

		if runID.Valid {
			baseline.RunID = runID.String
		}
		if validFrom.Valid {
			baseline.ValidFrom = validFrom.Time
		}
		if validUntil.Valid {
			baseline.ValidUntil = validUntil.Time
		}
		baseline.Tags = tags

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
			json.Unmarshal([]byte(thresholdsJSON.String), &baseline.Thresholds)
		}
		if labelsJSON.Valid {
			json.Unmarshal([]byte(labelsJSON.String), &baseline.Labels)
		}

		baselines = append(baselines, baseline)
	}

	return baselines, nil
}

// Delete removes an entity by ID.
func (s *PostgresStorage) Delete(ctx context.Context, entityType string, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	schema := s.config.Schema
	if schema == "" {
		schema = "redismeter"
	}

	var table string
	switch entityType {
	case "runs":
		table = "benchmark_runs"
	case "baselines":
		table = "baselines"
	default:
		return fmt.Errorf("unsupported entity type: %s", entityType)
	}

	query := fmt.Sprintf("DELETE FROM %s.%s WHERE id = $1", schema, table)
	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete %s: %w", entityType, err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("%s not found: %s", entityType, id)
	}

	return nil
}

// SaveRun persists a benchmark run.
func (s *PostgresStorage) SaveRun(ctx context.Context, run *domain.BenchmarkRun) error {
	_, err := s.Save(ctx, "runs", run)
	return err
}

// ListRuns returns recent benchmark runs.
func (s *PostgresStorage) ListRuns(ctx context.Context, limit int) ([]*domain.BenchmarkRun, error) {
	var runs []*domain.BenchmarkRun
	err := s.Query(ctx, "runs", plugin.QueryFilter{Limit: limit, Descending: true}, &runs)
	return runs, err
}

// GetRun retrieves a single benchmark run by ID.
func (s *PostgresStorage) GetRun(ctx context.Context, id string) (*domain.BenchmarkRun, error) {
	run := &domain.BenchmarkRun{}
	err := s.Load(ctx, "runs", id, run)
	if err != nil {
		return nil, err
	}
	return run, nil
}

// DeleteRun removes a benchmark run by ID.
func (s *PostgresStorage) DeleteRun(ctx context.Context, id string) error {
	return s.Delete(ctx, "runs", id)
}

// FullTextSearch performs a full-text search across benchmark runs.
func (s *PostgresStorage) FullTextSearch(ctx context.Context, query string, limit int) ([]*domain.BenchmarkRun, error) {
	var runs []*domain.BenchmarkRun
	err := s.Query(ctx, "runs", plugin.QueryFilter{
		Conditions: map[string]interface{}{"search": query},
		Limit:      limit,
		Descending: true,
	}, &runs)
	return runs, err
}

// Helper functions

func nullTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t
}

// Ensure PostgresStorage satisfies RunStorage interface.
var _ RunStorage = (*PostgresStorage)(nil)

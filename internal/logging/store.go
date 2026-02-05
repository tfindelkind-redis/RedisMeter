package logging

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStore implements Store using SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite log store.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.init(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// init creates the logs table if it doesn't exist.
func (s *SQLiteStore) init() error {
	schema := `
	CREATE TABLE IF NOT EXISTS logs (
		id TEXT PRIMARY KEY,
		timestamp DATETIME NOT NULL,
		level TEXT NOT NULL,
		source TEXT NOT NULL,
		operation TEXT NOT NULL,
		message TEXT NOT NULL,
		context TEXT,
		benchmark_id TEXT,
		infra_id TEXT,
		error TEXT,
		duration_ns INTEGER
	);
	
	CREATE INDEX IF NOT EXISTS idx_logs_timestamp ON logs(timestamp);
	CREATE INDEX IF NOT EXISTS idx_logs_level ON logs(level);
	CREATE INDEX IF NOT EXISTS idx_logs_source ON logs(source);
	CREATE INDEX IF NOT EXISTS idx_logs_benchmark_id ON logs(benchmark_id);
	CREATE INDEX IF NOT EXISTS idx_logs_infra_id ON logs(infra_id);
	`

	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// Save stores a log entry.
func (s *SQLiteStore) Save(ctx context.Context, entry *Entry) error {
	var contextJSON []byte
	if entry.Context != nil {
		var err error
		contextJSON, err = json.Marshal(entry.Context)
		if err != nil {
			return fmt.Errorf("failed to marshal context: %w", err)
		}
	}

	query := `
	INSERT INTO logs (id, timestamp, level, source, operation, message, context, benchmark_id, infra_id, error, duration_ns)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query,
		entry.ID,
		entry.Timestamp,
		entry.Level,
		entry.Source,
		entry.Operation,
		entry.Message,
		string(contextJSON),
		entry.BenchmarkID,
		entry.InfraID,
		entry.Error,
		entry.Duration.Nanoseconds(),
	)

	if err != nil {
		return fmt.Errorf("failed to insert log entry: %w", err)
	}

	return nil
}

// Query retrieves log entries matching the filter.
func (s *SQLiteStore) Query(ctx context.Context, filter *QueryFilter) ([]*Entry, error) {
	query := "SELECT id, timestamp, level, source, operation, message, context, benchmark_id, infra_id, error, duration_ns FROM logs WHERE 1=1"
	args := []interface{}{}

	if filter != nil {
		if filter.Level != "" {
			query += " AND level = ?"
			args = append(args, filter.Level)
		}
		if filter.Source != "" {
			query += " AND source = ?"
			args = append(args, filter.Source)
		}
		if filter.Operation != "" {
			query += " AND operation LIKE ?"
			args = append(args, "%"+filter.Operation+"%")
		}
		if filter.BenchmarkID != "" {
			query += " AND benchmark_id = ?"
			args = append(args, filter.BenchmarkID)
		}
		if filter.InfraID != "" {
			query += " AND infra_id = ?"
			args = append(args, filter.InfraID)
		}
		if filter.Since != nil && !filter.Since.IsZero() {
			query += " AND timestamp >= ?"
			args = append(args, *filter.Since)
		}
		if filter.Until != nil && !filter.Until.IsZero() {
			query += " AND timestamp <= ?"
			args = append(args, *filter.Until)
		}
		if filter.Search != "" {
			query += " AND (message LIKE ? OR operation LIKE ? OR context LIKE ?)"
			searchTerm := "%" + filter.Search + "%"
			args = append(args, searchTerm, searchTerm, searchTerm)
		}
	}

	query += " ORDER BY timestamp DESC"

	if filter != nil {
		if filter.Limit > 0 {
			query += fmt.Sprintf(" LIMIT %d", filter.Limit)
		}
		if filter.Offset > 0 {
			query += fmt.Sprintf(" OFFSET %d", filter.Offset)
		}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %w", err)
	}
	defer rows.Close()

	var entries []*Entry
	for rows.Next() {
		var entry Entry
		var contextJSON sql.NullString
		var benchmarkID, infraID, errorStr sql.NullString
		var durationNs sql.NullInt64

		err := rows.Scan(
			&entry.ID,
			&entry.Timestamp,
			&entry.Level,
			&entry.Source,
			&entry.Operation,
			&entry.Message,
			&contextJSON,
			&benchmarkID,
			&infraID,
			&errorStr,
			&durationNs,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if contextJSON.Valid && contextJSON.String != "" {
			if err := json.Unmarshal([]byte(contextJSON.String), &entry.Context); err != nil {
				// Log but don't fail - context is optional
				entry.Context = map[string]interface{}{"_raw": contextJSON.String}
			}
		}

		if benchmarkID.Valid {
			entry.BenchmarkID = benchmarkID.String
		}
		if infraID.Valid {
			entry.InfraID = infraID.String
		}
		if errorStr.Valid {
			entry.Error = errorStr.String
		}
		if durationNs.Valid {
			entry.Duration = time.Duration(durationNs.Int64)
		}

		entries = append(entries, &entry)
	}

	return entries, nil
}

// Count returns the number of entries matching the filter.
func (s *SQLiteStore) Count(ctx context.Context, filter *QueryFilter) (int64, error) {
	query := "SELECT COUNT(*) FROM logs WHERE 1=1"
	args := []interface{}{}

	if filter != nil {
		if filter.Level != "" {
			query += " AND level = ?"
			args = append(args, filter.Level)
		}
		if filter.Source != "" {
			query += " AND source = ?"
			args = append(args, filter.Source)
		}
		if filter.BenchmarkID != "" {
			query += " AND benchmark_id = ?"
			args = append(args, filter.BenchmarkID)
		}
		if filter.InfraID != "" {
			query += " AND infra_id = ?"
			args = append(args, filter.InfraID)
		}
		if filter.Since != nil && !filter.Since.IsZero() {
			query += " AND timestamp >= ?"
			args = append(args, *filter.Since)
		}
		if filter.Until != nil && !filter.Until.IsZero() {
			query += " AND timestamp <= ?"
			args = append(args, *filter.Until)
		}
		if filter.Search != "" {
			query += " AND (message LIKE ? OR operation LIKE ? OR context LIKE ?)"
			searchTerm := "%" + filter.Search + "%"
			args = append(args, searchTerm, searchTerm, searchTerm)
		}
	}

	var count int64
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count logs: %w", err)
	}

	return count, nil
}

// Delete removes entries older than the given time.
func (s *SQLiteStore) Delete(ctx context.Context, before time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, "DELETE FROM logs WHERE timestamp < ?", before)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old logs: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get affected rows: %w", err)
	}

	return affected, nil
}

// Export exports logs to a writer in JSONL format.
func (s *SQLiteStore) Export(ctx context.Context, filter *QueryFilter, w io.Writer) error {
	entries, err := s.Query(ctx, filter)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(w)
	for _, entry := range entries {
		if err := encoder.Encode(entry); err != nil {
			return fmt.Errorf("failed to encode entry: %w", err)
		}
	}

	return nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// GetStats returns statistics about the log store.
func (s *SQLiteStore) GetStats(ctx context.Context) (*StoreStats, error) {
	stats := &StoreStats{
		LevelCounts:  make(map[Level]int64),
		SourceCounts: make(map[Source]int64),
	}

	// Total count
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM logs").Scan(&stats.TotalEntries); err != nil {
		return nil, err
	}

	// Oldest entry - use string scanning for compatibility with modernc.org/sqlite
	var oldestStr sql.NullString
	if err := s.db.QueryRowContext(ctx, "SELECT MIN(timestamp) FROM logs").Scan(&oldestStr); err != nil {
		return nil, err
	}
	if oldestStr.Valid && oldestStr.String != "" {
		if t, err := time.Parse(time.RFC3339Nano, oldestStr.String); err == nil {
			stats.OldestEntry = t
		} else if t, err := time.Parse("2006-01-02 15:04:05.999999999-07:00", oldestStr.String); err == nil {
			stats.OldestEntry = t
		} else if t, err := time.Parse("2006-01-02T15:04:05.999999999Z", oldestStr.String); err == nil {
			stats.OldestEntry = t
		}
	}

	// Newest entry
	var newestStr sql.NullString
	if err := s.db.QueryRowContext(ctx, "SELECT MAX(timestamp) FROM logs").Scan(&newestStr); err != nil {
		return nil, err
	}
	if newestStr.Valid && newestStr.String != "" {
		if t, err := time.Parse(time.RFC3339Nano, newestStr.String); err == nil {
			stats.NewestEntry = t
		} else if t, err := time.Parse("2006-01-02 15:04:05.999999999-07:00", newestStr.String); err == nil {
			stats.NewestEntry = t
		} else if t, err := time.Parse("2006-01-02T15:04:05.999999999Z", newestStr.String); err == nil {
			stats.NewestEntry = t
		}
	}

	// Count by level
	rows, err := s.db.QueryContext(ctx, "SELECT level, COUNT(*) FROM logs GROUP BY level")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var level string
		var count int64
		if err := rows.Scan(&level, &count); err != nil {
			return nil, err
		}
		stats.LevelCounts[Level(level)] = count
	}

	// Count by source
	rows, err = s.db.QueryContext(ctx, "SELECT source, COUNT(*) FROM logs GROUP BY source")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var source string
		var count int64
		if err := rows.Scan(&source, &count); err != nil {
			return nil, err
		}
		stats.SourceCounts[Source(source)] = count
	}

	// Database size (approximate)
	var pageCount, pageSize int64
	if err := s.db.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pageCount); err != nil {
		return nil, err
	}
	if err := s.db.QueryRowContext(ctx, "PRAGMA page_size").Scan(&pageSize); err != nil {
		return nil, err
	}
	stats.DatabaseSize = pageCount * pageSize

	return stats, nil
}

// StoreStats holds statistics about the log store.
type StoreStats struct {
	TotalEntries int64            `json:"total_entries"`
	OldestEntry  time.Time        `json:"oldest_entry"`
	NewestEntry  time.Time        `json:"newest_entry"`
	LevelCounts  map[Level]int64  `json:"level_counts"`
	SourceCounts map[Source]int64 `json:"source_counts"`
	DatabaseSize int64            `json:"database_size_bytes"`
}

// Vacuum optimizes the database.
func (s *SQLiteStore) Vacuum(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "VACUUM")
	return err
}

// GetEntriesForBenchmark retrieves all log entries for a specific benchmark.
func (s *SQLiteStore) GetEntriesForBenchmark(ctx context.Context, benchmarkID string) ([]*Entry, error) {
	return s.Query(ctx, &QueryFilter{BenchmarkID: benchmarkID})
}

// GetEntriesForInfra retrieves all log entries for a specific infrastructure.
func (s *SQLiteStore) GetEntriesForInfra(ctx context.Context, infraID string) ([]*Entry, error) {
	return s.Query(ctx, &QueryFilter{InfraID: infraID})
}

// GetRecentErrors retrieves recent error entries.
func (s *SQLiteStore) GetRecentErrors(ctx context.Context, limit int) ([]*Entry, error) {
	return s.Query(ctx, &QueryFilter{Level: LevelError, Limit: limit})
}

// SearchLogs performs a full-text search on logs.
func (s *SQLiteStore) SearchLogs(ctx context.Context, searchTerm string, limit int) ([]*Entry, error) {
	return s.Query(ctx, &QueryFilter{Search: searchTerm, Limit: limit})
}

// ExportToJSON exports logs to JSON array format.
func (s *SQLiteStore) ExportToJSON(ctx context.Context, filter *QueryFilter, w io.Writer) error {
	entries, err := s.Query(ctx, filter)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(entries)
}

// ImportFromJSONL imports logs from JSONL format.
func (s *SQLiteStore) ImportFromJSONL(ctx context.Context, r io.Reader) (int, error) {
	decoder := json.NewDecoder(r)
	count := 0

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	for decoder.More() {
		var entry Entry
		if err := decoder.Decode(&entry); err != nil {
			return count, fmt.Errorf("failed to decode entry %d: %w", count+1, err)
		}

		// Ensure unique ID
		if entry.ID == "" {
			entry.ID = fmt.Sprintf("imported-%d-%d", time.Now().UnixNano(), count)
		} else {
			// Prefix to avoid conflicts
			entry.ID = "imported-" + entry.ID
		}

		if err := s.saveInTx(ctx, tx, &entry); err != nil {
			// Log conflict but continue
			if strings.Contains(err.Error(), "UNIQUE constraint") {
				continue
			}
			return count, err
		}
		count++
	}

	if err := tx.Commit(); err != nil {
		return count, err
	}

	return count, nil
}

// saveInTx saves an entry within a transaction.
func (s *SQLiteStore) saveInTx(ctx context.Context, tx *sql.Tx, entry *Entry) error {
	var contextJSON []byte
	if entry.Context != nil {
		var err error
		contextJSON, err = json.Marshal(entry.Context)
		if err != nil {
			return fmt.Errorf("failed to marshal context: %w", err)
		}
	}

	query := `
	INSERT INTO logs (id, timestamp, level, source, operation, message, context, benchmark_id, infra_id, error, duration_ns)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := tx.ExecContext(ctx, query,
		entry.ID,
		entry.Timestamp,
		entry.Level,
		entry.Source,
		entry.Operation,
		entry.Message,
		string(contextJSON),
		entry.BenchmarkID,
		entry.InfraID,
		entry.Error,
		entry.Duration.Nanoseconds(),
	)

	return err
}

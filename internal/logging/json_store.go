package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// JSONStore implements Store using JSON files.
// Logs are stored as JSONL (one entry per line) files, rotated daily.
type JSONStore struct {
	dir       string
	mu        sync.RWMutex
	cache     []*Entry      // In-memory cache for fast queries
	cacheMax  int           // Maximum entries to keep in memory
	writeBuf  chan *Entry   // Buffered writes
	done      chan struct{} // Shutdown signal
	wg        sync.WaitGroup
}

// NewJSONStore creates a new JSON-based log store.
func NewJSONStore(dir string) (*JSONStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	store := &JSONStore{
		dir:      dir,
		cacheMax: 10000, // Keep last 10k entries in memory
		writeBuf: make(chan *Entry, 1000),
		done:     make(chan struct{}),
	}

	// Load recent entries into cache
	if err := store.loadRecentEntries(); err != nil {
		// Non-fatal - start fresh
		fmt.Fprintf(os.Stderr, "Warning: Could not load existing logs: %v\n", err)
	}

	// Start background writer
	store.wg.Add(1)
	go store.backgroundWriter()

	return store, nil
}

// backgroundWriter handles async writes to reduce I/O impact.
func (s *JSONStore) backgroundWriter() {
	defer s.wg.Done()

	var pending []*Entry
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	flush := func() {
		if len(pending) == 0 {
			return
		}

		s.mu.Lock()
		defer s.mu.Unlock()

		// Group entries by date
		byDate := make(map[string][]*Entry)
		for _, e := range pending {
			date := e.Timestamp.Format("2006-01-02")
			byDate[date] = append(byDate[date], e)
		}

		// Write each date's entries
		for date, entries := range byDate {
			filename := filepath.Join(s.dir, fmt.Sprintf("logs-%s.jsonl", date))
			f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
				continue
			}

			encoder := json.NewEncoder(f)
			for _, e := range entries {
				if err := encoder.Encode(e); err != nil {
					fmt.Fprintf(os.Stderr, "Error writing log entry: %v\n", err)
				}
			}
			f.Close()
		}

		// Update cache
		s.cache = append(s.cache, pending...)
		if len(s.cache) > s.cacheMax {
			s.cache = s.cache[len(s.cache)-s.cacheMax:]
		}

		pending = nil
	}

	for {
		select {
		case entry := <-s.writeBuf:
			pending = append(pending, entry)
			if len(pending) >= 100 {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-s.done:
			// Drain remaining entries
			close(s.writeBuf)
			for entry := range s.writeBuf {
				pending = append(pending, entry)
			}
			flush()
			return
		}
	}
}

// loadRecentEntries loads entries from recent log files.
func (s *JSONStore) loadRecentEntries() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	files, err := filepath.Glob(filepath.Join(s.dir, "logs-*.jsonl"))
	if err != nil {
		return err
	}

	// Sort files by date (newest first)
	sort.Sort(sort.Reverse(sort.StringSlice(files)))

	// Load from most recent files up to cache limit
	for _, file := range files {
		entries, err := s.loadFile(file)
		if err != nil {
			continue // Skip corrupt files
		}

		s.cache = append(entries, s.cache...)
		if len(s.cache) >= s.cacheMax {
			s.cache = s.cache[:s.cacheMax]
			break
		}
	}

	return nil
}

// loadFile reads entries from a single JSONL file.
func (s *JSONStore) loadFile(filename string) ([]*Entry, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []*Entry
	decoder := json.NewDecoder(f)

	for decoder.More() {
		var entry Entry
		if err := decoder.Decode(&entry); err != nil {
			continue // Skip malformed entries
		}
		entries = append(entries, &entry)
	}

	return entries, nil
}

// Save stores a log entry.
func (s *JSONStore) Save(ctx context.Context, entry *Entry) error {
	select {
	case s.writeBuf <- entry:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Buffer full - write synchronously
		return s.saveSync(entry)
	}
}

// saveSync writes an entry immediately (fallback when buffer is full).
func (s *JSONStore) saveSync(entry *Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	date := entry.Timestamp.Format("2006-01-02")
	filename := filepath.Join(s.dir, fmt.Sprintf("logs-%s.jsonl", date))

	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	if err := encoder.Encode(entry); err != nil {
		return fmt.Errorf("failed to encode log entry: %w", err)
	}

	// Update cache
	s.cache = append(s.cache, entry)
	if len(s.cache) > s.cacheMax {
		s.cache = s.cache[1:]
	}

	return nil
}

// Query retrieves log entries matching the filter.
func (s *JSONStore) Query(ctx context.Context, filter *QueryFilter) ([]*Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*Entry

	// If we need entries outside cache, load from files
	if filter != nil && filter.Since != nil && len(s.cache) > 0 {
		oldest := s.cache[0].Timestamp
		if filter.Since.Before(oldest) {
			entries, err := s.loadEntriesInRange(filter.Since, filter.Until)
			if err != nil {
				return nil, err
			}
			results = s.filterEntries(entries, filter)
		} else {
			results = s.filterEntries(s.cache, filter)
		}
	} else {
		results = s.filterEntries(s.cache, filter)
	}

	// Sort by timestamp descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})

	// Apply limit and offset
	if filter != nil {
		if filter.Offset > 0 && filter.Offset < len(results) {
			results = results[filter.Offset:]
		} else if filter.Offset >= len(results) {
			return []*Entry{}, nil
		}

		if filter.Limit > 0 && filter.Limit < len(results) {
			results = results[:filter.Limit]
		}
	}

	return results, nil
}

// loadEntriesInRange loads entries from files within a date range.
func (s *JSONStore) loadEntriesInRange(since, until *time.Time) ([]*Entry, error) {
	files, err := filepath.Glob(filepath.Join(s.dir, "logs-*.jsonl"))
	if err != nil {
		return nil, err
	}

	var allEntries []*Entry
	for _, file := range files {
		// Extract date from filename
		base := filepath.Base(file)
		dateStr := strings.TrimPrefix(base, "logs-")
		dateStr = strings.TrimSuffix(dateStr, ".jsonl")
		fileDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		// Check if file is in range
		if since != nil && fileDate.Before(since.Truncate(24*time.Hour).Add(-24*time.Hour)) {
			continue
		}
		if until != nil && fileDate.After(until.Truncate(24*time.Hour).Add(24*time.Hour)) {
			continue
		}

		entries, err := s.loadFile(file)
		if err != nil {
			continue
		}
		allEntries = append(allEntries, entries...)
	}

	return allEntries, nil
}

// filterEntries applies filter criteria to entries.
func (s *JSONStore) filterEntries(entries []*Entry, filter *QueryFilter) []*Entry {
	if filter == nil {
		result := make([]*Entry, len(entries))
		copy(result, entries)
		return result
	}

	var results []*Entry
	for _, entry := range entries {
		if s.matchesFilter(entry, filter) {
			results = append(results, entry)
		}
	}
	return results
}

// matchesFilter checks if an entry matches the filter criteria.
func (s *JSONStore) matchesFilter(entry *Entry, filter *QueryFilter) bool {
	if filter.Level != "" && entry.Level != filter.Level {
		return false
	}
	if filter.Source != "" && entry.Source != filter.Source {
		return false
	}
	if filter.Operation != "" && !strings.Contains(entry.Operation, filter.Operation) {
		return false
	}
	if filter.BenchmarkID != "" && entry.BenchmarkID != filter.BenchmarkID {
		return false
	}
	if filter.InfraID != "" && entry.InfraID != filter.InfraID {
		return false
	}
	if filter.Since != nil && entry.Timestamp.Before(*filter.Since) {
		return false
	}
	if filter.Until != nil && entry.Timestamp.After(*filter.Until) {
		return false
	}
	if filter.Search != "" {
		search := strings.ToLower(filter.Search)
		if !strings.Contains(strings.ToLower(entry.Message), search) &&
			!strings.Contains(strings.ToLower(entry.Operation), search) {
			// Check context
			contextStr, _ := json.Marshal(entry.Context)
			if !strings.Contains(strings.ToLower(string(contextStr)), search) {
				return false
			}
		}
	}
	return true
}

// Count returns the number of entries matching the filter.
func (s *JSONStore) Count(ctx context.Context, filter *QueryFilter) (int64, error) {
	entries, err := s.Query(ctx, filter)
	if err != nil {
		return 0, err
	}
	return int64(len(entries)), nil
}

// Delete removes entries older than the given time.
func (s *JSONStore) Delete(ctx context.Context, before time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var deleted int64

	// Remove from cache
	var newCache []*Entry
	for _, e := range s.cache {
		if e.Timestamp.Before(before) {
			deleted++
		} else {
			newCache = append(newCache, e)
		}
	}
	s.cache = newCache

	// Remove or truncate old log files
	files, err := filepath.Glob(filepath.Join(s.dir, "logs-*.jsonl"))
	if err != nil {
		return deleted, err
	}

	cutoffDate := before.Format("2006-01-02")
	for _, file := range files {
		base := filepath.Base(file)
		dateStr := strings.TrimPrefix(base, "logs-")
		dateStr = strings.TrimSuffix(dateStr, ".jsonl")

		if dateStr < cutoffDate {
			// Entire file is old - remove it
			count, _ := s.countEntriesInFile(file)
			if err := os.Remove(file); err == nil {
				deleted += int64(count)
			}
		} else if dateStr == cutoffDate {
			// Partial file - need to filter entries
			d, err := s.truncateFile(file, before)
			if err == nil {
				deleted += d
			}
		}
	}

	return deleted, nil
}

// countEntriesInFile counts entries in a file without loading them all.
func (s *JSONStore) countEntriesInFile(filename string) (int, error) {
	entries, err := s.loadFile(filename)
	if err != nil {
		return 0, err
	}
	return len(entries), nil
}

// truncateFile removes entries older than the given time from a file.
func (s *JSONStore) truncateFile(filename string, before time.Time) (int64, error) {
	entries, err := s.loadFile(filename)
	if err != nil {
		return 0, err
	}

	var keep []*Entry
	var deleted int64
	for _, e := range entries {
		if e.Timestamp.Before(before) {
			deleted++
		} else {
			keep = append(keep, e)
		}
	}

	if len(keep) == 0 {
		return deleted, os.Remove(filename)
	}

	// Rewrite file with remaining entries
	f, err := os.Create(filename)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	for _, e := range keep {
		if err := encoder.Encode(e); err != nil {
			return deleted, err
		}
	}

	return deleted, nil
}

// Export exports logs to a writer in JSONL format.
func (s *JSONStore) Export(ctx context.Context, filter *QueryFilter, w io.Writer) error {
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

// Close closes the store and flushes pending writes.
func (s *JSONStore) Close() error {
	close(s.done)
	s.wg.Wait()
	return nil
}

// GetStats returns statistics about the log store.
func (s *JSONStore) GetStats(ctx context.Context) (*StoreStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &StoreStats{
		LevelCounts:  make(map[Level]int64),
		SourceCounts: make(map[Source]int64),
	}

	// Count from cache (represents recent activity)
	for _, e := range s.cache {
		stats.TotalEntries++
		stats.LevelCounts[e.Level]++
		stats.SourceCounts[e.Source]++

		if stats.OldestEntry.IsZero() || e.Timestamp.Before(stats.OldestEntry) {
			stats.OldestEntry = e.Timestamp
		}
		if e.Timestamp.After(stats.NewestEntry) {
			stats.NewestEntry = e.Timestamp
		}
	}

	// Calculate total size from all log files
	files, _ := filepath.Glob(filepath.Join(s.dir, "logs-*.jsonl"))
	for _, file := range files {
		if info, err := os.Stat(file); err == nil {
			stats.DatabaseSize += info.Size()
		}
	}

	return stats, nil
}

// GetEntriesForBenchmark retrieves all log entries for a specific benchmark.
func (s *JSONStore) GetEntriesForBenchmark(ctx context.Context, benchmarkID string) ([]*Entry, error) {
	return s.Query(ctx, &QueryFilter{BenchmarkID: benchmarkID})
}

// GetEntriesForInfra retrieves all log entries for a specific infrastructure.
func (s *JSONStore) GetEntriesForInfra(ctx context.Context, infraID string) ([]*Entry, error) {
	return s.Query(ctx, &QueryFilter{InfraID: infraID})
}

// GetRecentErrors retrieves recent error entries.
func (s *JSONStore) GetRecentErrors(ctx context.Context, limit int) ([]*Entry, error) {
	return s.Query(ctx, &QueryFilter{Level: LevelError, Limit: limit})
}

// SearchLogs performs a search on logs.
func (s *JSONStore) SearchLogs(ctx context.Context, searchTerm string, limit int) ([]*Entry, error) {
	return s.Query(ctx, &QueryFilter{Search: searchTerm, Limit: limit})
}

// ExportToJSON exports logs to JSON array format.
func (s *JSONStore) ExportToJSON(ctx context.Context, filter *QueryFilter, w io.Writer) error {
	entries, err := s.Query(ctx, filter)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(entries)
}

// ImportFromJSONL imports logs from JSONL format.
func (s *JSONStore) ImportFromJSONL(ctx context.Context, r io.Reader) (int, error) {
	decoder := json.NewDecoder(r)
	count := 0

	for decoder.More() {
		var entry Entry
		if err := decoder.Decode(&entry); err != nil {
			return count, fmt.Errorf("failed to decode entry %d: %w", count+1, err)
		}

		// Ensure unique ID
		if entry.ID == "" {
			entry.ID = fmt.Sprintf("imported-%d-%d", time.Now().UnixNano(), count)
		}

		if err := s.Save(ctx, &entry); err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}

// Vacuum is a no-op for JSON store (kept for interface compatibility).
func (s *JSONStore) Vacuum(ctx context.Context) error {
	return nil
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

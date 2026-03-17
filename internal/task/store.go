package task

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
)

// Store provides persistent storage for tasks using JSON files.
// Each task is stored in its own file for easy inspection and recovery.
// File naming: {task_id}.json in the tasks directory.
type Store struct {
	dir   string
	mu    sync.RWMutex
	cache map[string]*Task // In-memory cache for fast access
}

// NewStore creates a new JSON-based task store.
func NewStore(dir string) (*Store, error) {
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		dir = filepath.Join(home, ".redismeter", "tasks")
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create tasks directory: %w", err)
	}

	store := &Store{
		dir:   dir,
		cache: make(map[string]*Task),
	}

	// Load existing tasks into cache
	if err := store.loadAll(); err != nil {
		// Non-fatal - start fresh
		fmt.Fprintf(os.Stderr, "Warning: Could not load existing tasks: %v\n", err)
	}

	return store, nil
}

// loadAll loads all tasks from disk into the cache.
func (s *Store) loadAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	files, err := filepath.Glob(filepath.Join(s.dir, "*.json"))
	if err != nil {
		return err
	}

	for _, file := range files {
		task, err := s.loadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not load task %s: %v\n", file, err)
			continue
		}
		s.cache[task.ID] = task
	}

	return nil
}

// loadFile loads a single task from disk.
func (s *Store) loadFile(path string) (*Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var task Task
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, err
	}

	return &task, nil
}

// Save persists a task to disk.
func (s *Store) Save(ctx context.Context, task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task.LastUpdatedAt = time.Now()

	// Marshal with indentation for human readability
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	path := filepath.Join(s.dir, task.ID+".json")

	// Write to temp file first, then rename for atomic operation
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write task file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // Clean up temp file
		return fmt.Errorf("failed to rename task file: %w", err)
	}

	// Update cache
	s.cache[task.ID] = task

	return nil
}

// Get retrieves a task by ID.
func (s *Store) Get(ctx context.Context, id string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.cache[id]
	if !ok {
		return nil, fmt.Errorf("task %s not found", id)
	}

	// Return a copy to prevent external modification
	return s.copyTask(task), nil
}

// Delete removes a task.
func (s *Store) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, id+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete task file: %w", err)
	}

	delete(s.cache, id)
	return nil
}

// List returns all tasks, optionally filtered.
func (s *Store) List(ctx context.Context, filter *TaskFilter) ([]*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var tasks []*Task
	for _, task := range s.cache {
		if filter == nil || filter.Matches(task) {
			tasks = append(tasks, s.copyTask(task))
		}
	}

	// Sort by created time, newest first
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})

	// Apply limit if specified
	if filter != nil && filter.Limit > 0 && len(tasks) > filter.Limit {
		tasks = tasks[:filter.Limit]
	}

	return tasks, nil
}

// ListActive returns tasks that are currently running or paused.
func (s *Store) ListActive(ctx context.Context) ([]*Task, error) {
	return s.List(ctx, &TaskFilter{
		Statuses: []TaskStatus{StatusPending, StatusRunning, StatusPaused},
	})
}

// ListResumable returns tasks that can be resumed.
func (s *Store) ListResumable(ctx context.Context) ([]*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var tasks []*Task
	for _, task := range s.cache {
		if task.CanResume() {
			tasks = append(tasks, s.copyTask(task))
		}
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].LastUpdatedAt.After(tasks[j].LastUpdatedAt)
	})

	return tasks, nil
}

// GetByTool returns tasks for a specific tool.
func (s *Store) GetByTool(ctx context.Context, tool ToolType) ([]*Task, error) {
	return s.List(ctx, &TaskFilter{Tool: tool})
}

// copyTask creates a deep copy of a task.
func (s *Store) copyTask(t *Task) *Task {
	// Use JSON round-trip for deep copy (simple and reliable)
	data, _ := json.Marshal(t)
	var copy Task
	json.Unmarshal(data, &copy)
	return &copy
}

// Archive moves completed/failed tasks to an archive directory.
// This helps keep the active tasks directory small.
func (s *Store) Archive(ctx context.Context, olderThan time.Duration) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	archiveDir := filepath.Join(s.dir, "archive")
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create archive directory: %w", err)
	}

	cutoff := time.Now().Add(-olderThan)
	archived := 0

	for id, task := range s.cache {
		if task.IsTerminal() && task.LastUpdatedAt.Before(cutoff) {
			srcPath := filepath.Join(s.dir, id+".json")
			dstPath := filepath.Join(archiveDir, id+".json")

			if err := os.Rename(srcPath, dstPath); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Could not archive task %s: %v\n", id, err)
				continue
			}

			delete(s.cache, id)
			archived++
		}
	}

	return archived, nil
}

// Cleanup removes very old archived tasks.
func (s *Store) Cleanup(ctx context.Context, olderThan time.Duration) (int, error) {
	archiveDir := filepath.Join(s.dir, "archive")
	files, err := filepath.Glob(filepath.Join(archiveDir, "*.json"))
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-olderThan)
	cleaned := 0

	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			if err := os.Remove(file); err == nil {
				cleaned++
			}
		}
	}

	return cleaned, nil
}

// TaskFilter specifies criteria for filtering tasks.
type TaskFilter struct {
	Statuses []TaskStatus
	Tool     ToolType
	Tags     []string
	Limit    int
}

// Matches returns true if the task matches the filter criteria.
func (f *TaskFilter) Matches(t *Task) bool {
	if f == nil {
		return true
	}

	// Check status
	if len(f.Statuses) > 0 {
		found := false
		for _, s := range f.Statuses {
			if t.Status == s {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check tool
	if f.Tool != "" && t.Tool != f.Tool {
		return false
	}

	// Check tags (all specified tags must be present)
	if len(f.Tags) > 0 {
		for _, tag := range f.Tags {
			found := false
			for _, tt := range t.Tags {
				if strings.EqualFold(tt, tag) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	return true
}

// Stats returns statistics about stored tasks.
type Stats struct {
	Total     int            `json:"total"`
	ByStatus  map[string]int `json:"by_status"`
	ByTool    map[string]int `json:"by_tool"`
	Active    int            `json:"active"`
	Resumable int            `json:"resumable"`
}

// GetStats returns statistics about stored tasks.
func (s *Store) GetStats(ctx context.Context) (*Stats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &Stats{
		Total:    len(s.cache),
		ByStatus: make(map[string]int),
		ByTool:   make(map[string]int),
	}

	for _, task := range s.cache {
		stats.ByStatus[string(task.Status)]++
		stats.ByTool[string(task.Tool)]++

		if task.Status == StatusRunning || task.Status == StatusPaused {
			stats.Active++
		}
		if task.CanResume() {
			stats.Resumable++
		}
	}

	return stats, nil
}

// WatchCallback is called when a task changes.
type WatchCallback func(task *Task, event string)

// Watcher watches for task changes.
type Watcher struct {
	store     *Store
	callbacks []WatchCallback
	done      chan struct{}
	mu        sync.RWMutex
}

// Watch creates a watcher for task changes.
// Note: This is a polling-based implementation. For production,
// consider using fsnotify for file system events.
func (s *Store) Watch(interval time.Duration) *Watcher {
	w := &Watcher{
		store: s,
		done:  make(chan struct{}),
	}

	go w.poll(interval)
	return w
}

func (w *Watcher) poll(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	lastState := make(map[string]time.Time)

	// Initialize with current state
	w.store.mu.RLock()
	for id, task := range w.store.cache {
		lastState[id] = task.LastUpdatedAt
	}
	w.store.mu.RUnlock()

	for {
		select {
		case <-ticker.C:
			w.store.mu.RLock()
			// Check for new or updated tasks
			for id, task := range w.store.cache {
				last, existed := lastState[id]
				if !existed {
					w.notify(task, "created")
				} else if task.LastUpdatedAt.After(last) {
					w.notify(task, "updated")
				}
				lastState[id] = task.LastUpdatedAt
			}

			// Check for deleted tasks
			for id := range lastState {
				if _, ok := w.store.cache[id]; !ok {
					delete(lastState, id)
					// Can't notify deleted task, but could track IDs
				}
			}
			w.store.mu.RUnlock()

		case <-w.done:
			return
		}
	}
}

// OnChange registers a callback for task changes.
func (w *Watcher) OnChange(cb WatchCallback) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.callbacks = append(w.callbacks, cb)
}

func (w *Watcher) notify(task *Task, event string) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	for _, cb := range w.callbacks {
		cb(task, event)
	}
}

// Stop stops the watcher.
func (w *Watcher) Stop() {
	close(w.done)
}

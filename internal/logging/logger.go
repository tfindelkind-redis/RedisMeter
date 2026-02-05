// Package logging provides centralized logging for RedisMeter operations.
package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Level represents log severity levels.
type Level string

const (
	LevelDebug Level = "DEBUG"
	LevelInfo  Level = "INFO"
	LevelWarn  Level = "WARN"
	LevelError Level = "ERROR"
)

// Source identifies where the log originated.
type Source string

const (
	SourceMemtier    Source = "memtier"
	SourceTerraform  Source = "terraform"
	SourceAPI        Source = "api"
	SourceStorage    Source = "storage"
	SourceAnalysis   Source = "analysis"
	SourceEngine     Source = "engine"
	SourceInfra      Source = "infrastructure"
	SourceExport     Source = "export"
	SourceImport     Source = "import"
	SourceInternal   Source = "internal"
)

// Entry represents a single log entry.
type Entry struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	Level       Level                  `json:"level"`
	Source      Source                 `json:"source"`
	Operation   string                 `json:"operation"`
	Message     string                 `json:"message"`
	Context     map[string]interface{} `json:"context,omitempty"`
	BenchmarkID string                 `json:"benchmark_id,omitempty"`
	InfraID     string                 `json:"infra_id,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Duration    time.Duration          `json:"duration,omitempty"`
}

// Store is the interface for log persistence.
type Store interface {
	// Save stores a log entry.
	Save(ctx context.Context, entry *Entry) error

	// Query retrieves log entries matching the filter.
	Query(ctx context.Context, filter *QueryFilter) ([]*Entry, error)

	// Count returns the number of entries matching the filter.
	Count(ctx context.Context, filter *QueryFilter) (int64, error)

	// Delete removes entries older than the given time.
	Delete(ctx context.Context, before time.Time) (int64, error)

	// Export exports logs to a writer.
	Export(ctx context.Context, filter *QueryFilter, w io.Writer) error

	// Close closes the store.
	Close() error
}

// QueryFilter defines criteria for querying logs.
type QueryFilter struct {
	Level       Level      `json:"level,omitempty"`
	Source      Source     `json:"source,omitempty"`
	Operation   string     `json:"operation,omitempty"`
	BenchmarkID string     `json:"benchmark_id,omitempty"`
	InfraID     string     `json:"infra_id,omitempty"`
	Since       *time.Time `json:"since,omitempty"`
	Until       *time.Time `json:"until,omitempty"`
	Search      string     `json:"search,omitempty"`
	Limit       int        `json:"limit,omitempty"`
	Offset      int        `json:"offset,omitempty"`
}

// Logger is the main logging interface.
type Logger interface {
	// Log writes a log entry.
	Log(level Level, source Source, operation, message string, ctx map[string]interface{})

	// Debug logs a debug message.
	Debug(operation, message string, ctx map[string]interface{})

	// Info logs an info message.
	Info(operation, message string, ctx map[string]interface{})

	// Warn logs a warning message.
	Warn(operation, message string, ctx map[string]interface{})

	// Error logs an error message.
	Error(operation, message string, ctx map[string]interface{})

	// WithBenchmark returns a logger that includes benchmark ID in all entries.
	WithBenchmark(benchmarkID string) Logger

	// WithInfra returns a logger that includes infrastructure ID in all entries.
	WithInfra(infraID string) Logger

	// Query retrieves log entries.
	Query(ctx context.Context, filter *QueryFilter) ([]*Entry, error)

	// Export exports logs.
	Export(ctx context.Context, filter *QueryFilter, w io.Writer) error
}

// DefaultLogger is the default logger implementation.
type DefaultLogger struct {
	store       Store
	minLevel    Level
	benchmarkID string
	infraID     string
	source      Source
	mu          sync.RWMutex
	console     bool
	idCounter   uint64
}

// Config holds logger configuration.
type Config struct {
	Store    Store
	MinLevel Level
	Console  bool // Also output to console
	Source   Source
}

// NewLogger creates a new logger.
func NewLogger(cfg Config) *DefaultLogger {
	if cfg.MinLevel == "" {
		cfg.MinLevel = LevelInfo
	}
	return &DefaultLogger{
		store:    cfg.Store,
		minLevel: cfg.MinLevel,
		console:  cfg.Console,
		source:   cfg.Source,
	}
}

// generateID generates a unique log entry ID.
func (l *DefaultLogger) generateID() string {
	l.mu.Lock()
	l.idCounter++
	id := l.idCounter
	l.mu.Unlock()
	return fmt.Sprintf("log-%d-%d", time.Now().UnixNano(), id)
}

// shouldLog checks if the message should be logged based on level.
func (l *DefaultLogger) shouldLog(level Level) bool {
	levelOrder := map[Level]int{
		LevelDebug: 0,
		LevelInfo:  1,
		LevelWarn:  2,
		LevelError: 3,
	}
	return levelOrder[level] >= levelOrder[l.minLevel]
}

// Log writes a log entry.
func (l *DefaultLogger) Log(level Level, source Source, operation, message string, ctx map[string]interface{}) {
	if !l.shouldLog(level) {
		return
	}

	entry := &Entry{
		ID:          l.generateID(),
		Timestamp:   time.Now(),
		Level:       level,
		Source:      source,
		Operation:   operation,
		Message:     message,
		Context:     ctx,
		BenchmarkID: l.benchmarkID,
		InfraID:     l.infraID,
	}

	// Console output
	if l.console {
		l.printToConsole(entry)
	}

	// Persist to store
	if l.store != nil {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := l.store.Save(bgCtx, entry); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save log entry: %v\n", err)
		}
	}
}

// printToConsole outputs the entry to console.
func (l *DefaultLogger) printToConsole(entry *Entry) {
	timestamp := entry.Timestamp.Format("15:04:05.000")
	levelColor := map[Level]string{
		LevelDebug: "\033[36m", // Cyan
		LevelInfo:  "\033[32m", // Green
		LevelWarn:  "\033[33m", // Yellow
		LevelError: "\033[31m", // Red
	}
	reset := "\033[0m"
	color := levelColor[entry.Level]
	fmt.Printf("%s %s%-5s%s [%s] %s: %s",
		timestamp, color, entry.Level, reset,
		entry.Source, entry.Operation, entry.Message)
	if entry.Error != "" {
		fmt.Printf(" error=%s", entry.Error)
	}
	if entry.Duration > 0 {
		fmt.Printf(" duration=%v", entry.Duration)
	}
	fmt.Println()
}

// Debug logs a debug message.
func (l *DefaultLogger) Debug(operation, message string, ctx map[string]interface{}) {
	source := l.source
	if source == "" {
		source = SourceInternal
	}
	l.Log(LevelDebug, source, operation, message, ctx)
}

// Info logs an info message.
func (l *DefaultLogger) Info(operation, message string, ctx map[string]interface{}) {
	source := l.source
	if source == "" {
		source = SourceInternal
	}
	l.Log(LevelInfo, source, operation, message, ctx)
}

// Warn logs a warning message.
func (l *DefaultLogger) Warn(operation, message string, ctx map[string]interface{}) {
	source := l.source
	if source == "" {
		source = SourceInternal
	}
	l.Log(LevelWarn, source, operation, message, ctx)
}

// Error logs an error message.
func (l *DefaultLogger) Error(operation, message string, ctx map[string]interface{}) {
	source := l.source
	if source == "" {
		source = SourceInternal
	}
	l.Log(LevelError, source, operation, message, ctx)
}

// WithBenchmark returns a logger that includes benchmark ID.
func (l *DefaultLogger) WithBenchmark(benchmarkID string) Logger {
	return &DefaultLogger{
		store:       l.store,
		minLevel:    l.minLevel,
		benchmarkID: benchmarkID,
		infraID:     l.infraID,
		source:      l.source,
		console:     l.console,
	}
}

// WithInfra returns a logger that includes infrastructure ID.
func (l *DefaultLogger) WithInfra(infraID string) Logger {
	return &DefaultLogger{
		store:       l.store,
		minLevel:    l.minLevel,
		benchmarkID: l.benchmarkID,
		infraID:     infraID,
		source:      l.source,
		console:     l.console,
	}
}

// Query retrieves log entries.
func (l *DefaultLogger) Query(ctx context.Context, filter *QueryFilter) ([]*Entry, error) {
	if l.store == nil {
		return nil, fmt.Errorf("no log store configured")
	}
	return l.store.Query(ctx, filter)
}

// Export exports logs.
func (l *DefaultLogger) Export(ctx context.Context, filter *QueryFilter, w io.Writer) error {
	if l.store == nil {
		return fmt.Errorf("no log store configured")
	}
	return l.store.Export(ctx, filter, w)
}

// CommandLog captures command execution details.
type CommandLog struct {
	Command   string        `json:"command"`
	Args      []string      `json:"args"`
	Stdout    string        `json:"stdout,omitempty"`
	Stderr    string        `json:"stderr,omitempty"`
	ExitCode  int           `json:"exit_code"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"duration"`
	Error     string        `json:"error,omitempty"`
	Output    string        `json:"output,omitempty"`
}

// LogCommand logs a command execution.
func (l *DefaultLogger) LogCommand(source Source, operation string, cmdLog *CommandLog) {
	ctx := map[string]interface{}{
		"command":    cmdLog.Command,
		"args":       cmdLog.Args,
		"exit_code":  cmdLog.ExitCode,
		"start_time": cmdLog.StartTime,
		"end_time":   cmdLog.EndTime,
		"duration":   cmdLog.Duration.String(),
	}

	// Include stdout/stderr in context if not too large
	if len(cmdLog.Stdout) > 0 && len(cmdLog.Stdout) < 10000 {
		ctx["stdout"] = cmdLog.Stdout
	} else if len(cmdLog.Stdout) >= 10000 {
		ctx["stdout_truncated"] = cmdLog.Stdout[:10000] + "... (truncated)"
	}
	if len(cmdLog.Stderr) > 0 && len(cmdLog.Stderr) < 10000 {
		ctx["stderr"] = cmdLog.Stderr
	} else if len(cmdLog.Stderr) >= 10000 {
		ctx["stderr_truncated"] = cmdLog.Stderr[:10000] + "... (truncated)"
	}

	level := LevelInfo
	message := fmt.Sprintf("Command completed: %s", cmdLog.Command)
	if cmdLog.ExitCode != 0 {
		level = LevelError
		message = fmt.Sprintf("Command failed with exit code %d: %s", cmdLog.ExitCode, cmdLog.Command)
	}

	l.Log(level, source, operation, message, ctx)
}

// MarshalJSON implements json.Marshaler for Entry.
func (e *Entry) MarshalJSON() ([]byte, error) {
	type Alias Entry
	return json.Marshal(&struct {
		*Alias
		Duration string `json:"duration,omitempty"`
	}{
		Alias:    (*Alias)(e),
		Duration: e.Duration.String(),
	})
}

// Global logger instance
var globalLogger Logger

// SetGlobalLogger sets the global logger.
func SetGlobalLogger(l Logger) {
	globalLogger = l
}

// GetGlobalLogger returns the global logger.
func GetGlobalLogger() Logger {
	return globalLogger
}

// Global logging functions (convenience wrappers)

// GlobalDebug logs a debug message using the global logger.
func GlobalDebug(source Source, operation, message string, ctx map[string]interface{}) {
	if globalLogger != nil {
		globalLogger.Log(LevelDebug, source, operation, message, ctx)
	}
}

// GlobalInfo logs an info message using the global logger.
func GlobalInfo(source Source, operation, message string, ctx map[string]interface{}) {
	if globalLogger != nil {
		globalLogger.Log(LevelInfo, source, operation, message, ctx)
	}
}

// GlobalWarn logs a warning message using the global logger.
func GlobalWarn(source Source, operation, message string, ctx map[string]interface{}) {
	if globalLogger != nil {
		globalLogger.Log(LevelWarn, source, operation, message, ctx)
	}
}

// GlobalError logs an error message using the global logger.
func GlobalError(source Source, operation, message string, ctx map[string]interface{}) {
	if globalLogger != nil {
		globalLogger.Log(LevelError, source, operation, message, ctx)
	}
}

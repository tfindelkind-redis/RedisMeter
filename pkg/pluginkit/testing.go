// Package pluginkit provides testing utilities for RedisMeter plugin development.
package pluginkit

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// ==================== Test Utilities ====================

// PluginTester provides testing utilities for plugins.
type PluginTester struct {
	t *testing.T
}

// NewPluginTester creates a new plugin tester.
func NewPluginTester(t *testing.T) *PluginTester {
	return &PluginTester{t: t}
}

// AssertNoError fails if err is not nil.
func (pt *PluginTester) AssertNoError(err error, msg string) {
	pt.t.Helper()
	if err != nil {
		pt.t.Fatalf("%s: %v", msg, err)
	}
}

// AssertError fails if err is nil.
func (pt *PluginTester) AssertError(err error, msg string) {
	pt.t.Helper()
	if err == nil {
		pt.t.Fatalf("%s: expected error, got nil", msg)
	}
}

// AssertEqual fails if expected != actual.
func (pt *PluginTester) AssertEqual(expected, actual interface{}, msg string) {
	pt.t.Helper()
	if expected != actual {
		pt.t.Fatalf("%s: expected %v, got %v", msg, expected, actual)
	}
}

// AssertContains fails if s does not contain substr.
func (pt *PluginTester) AssertContains(s, substr string, msg string) {
	pt.t.Helper()
	if !strings.Contains(s, substr) {
		pt.t.Fatalf("%s: expected %q to contain %q", msg, s, substr)
	}
}

// ==================== Mock Implementations ====================

// MockBenchmarkRun creates a mock benchmark run for testing.
type MockBenchmarkRun struct {
	ID          string
	Name        string
	Status      string
	Duration    time.Duration
	OpsPerSec   float64
	LatencyP50  float64
	LatencyP99  float64
	Errors      int64
	Tags        []string
	Labels      map[string]string
	CreatedAt   time.Time
	CompletedAt time.Time
}

// NewMockBenchmarkRun creates a mock run with default values.
func NewMockBenchmarkRun() *MockBenchmarkRun {
	now := time.Now()
	return &MockBenchmarkRun{
		ID:          "test-run-001",
		Name:        "Test Benchmark",
		Status:      "completed",
		Duration:    30 * time.Second,
		OpsPerSec:   100000.0,
		LatencyP50:  150.0,
		LatencyP99:  450.0,
		Errors:      0,
		Tags:        []string{"test"},
		Labels:      map[string]string{"env": "test"},
		CreatedAt:   now.Add(-time.Minute),
		CompletedAt: now,
	}
}

// ToMap converts the mock run to a map (for storage testing).
func (m *MockBenchmarkRun) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"id":           m.ID,
		"name":         m.Name,
		"status":       m.Status,
		"duration":     m.Duration.String(),
		"ops_per_sec":  m.OpsPerSec,
		"latency_p50":  m.LatencyP50,
		"latency_p99":  m.LatencyP99,
		"errors":       m.Errors,
		"tags":         m.Tags,
		"labels":       m.Labels,
		"created_at":   m.CreatedAt,
		"completed_at": m.CompletedAt,
	}
}

// MockBaseline creates a mock baseline for testing.
type MockBaseline struct {
	ID         string
	Name       string
	RunID      string
	Workload   string
	Active     bool
	Thresholds map[string]float64
	Metrics    map[string]float64
	CreatedAt  time.Time
}

// NewMockBaseline creates a mock baseline with default values.
func NewMockBaseline() *MockBaseline {
	return &MockBaseline{
		ID:       "test-baseline-001",
		Name:     "Test Baseline",
		RunID:    "test-run-001",
		Workload: "mixed",
		Active:   true,
		Thresholds: map[string]float64{
			"latency_p99": 0.10,
			"throughput":  0.05,
		},
		Metrics: map[string]float64{
			"ops_per_sec":  100000.0,
			"latency_p50":  150.0,
			"latency_p99":  450.0,
			"latency_p999": 1200.0,
		},
		CreatedAt: time.Now(),
	}
}

// ==================== Storage Plugin Tester ====================

// StoragePluginTest defines the interface for testing storage plugins.
type StoragePluginTest interface {
	Save(ctx context.Context, run *MockBenchmarkRun) error
	Load(ctx context.Context, id string) (*MockBenchmarkRun, error)
	Delete(ctx context.Context, id string) error
	Query(ctx context.Context, opts map[string]interface{}) ([]*MockBenchmarkRun, error)
}

// StorageTestSuite runs a standard suite of tests on a storage plugin.
type StorageTestSuite struct {
	t       *testing.T
	storage StoragePluginTest
	cleanup func()
}

// NewStorageTestSuite creates a new storage test suite.
func NewStorageTestSuite(t *testing.T, storage StoragePluginTest, cleanup func()) *StorageTestSuite {
	return &StorageTestSuite{
		t:       t,
		storage: storage,
		cleanup: cleanup,
	}
}

// Run executes all storage tests.
func (s *StorageTestSuite) Run() {
	s.t.Cleanup(s.cleanup)

	s.t.Run("SaveAndLoad", func(t *testing.T) { s.testSaveAndLoad(t) })
	s.t.Run("Delete", func(t *testing.T) { s.testDelete(t) })
	s.t.Run("Query", func(t *testing.T) { s.testQuery(t) })
	s.t.Run("NotFound", func(t *testing.T) { s.testNotFound(t) })
}

func (s *StorageTestSuite) testSaveAndLoad(t *testing.T) {
	ctx := context.Background()
	run := NewMockBenchmarkRun()

	// Save
	err := s.storage.Save(ctx, run)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load
	loaded, err := s.storage.Load(ctx, run.ID)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.ID != run.ID {
		t.Errorf("ID mismatch: expected %s, got %s", run.ID, loaded.ID)
	}
	if loaded.Name != run.Name {
		t.Errorf("Name mismatch: expected %s, got %s", run.Name, loaded.Name)
	}
}

func (s *StorageTestSuite) testDelete(t *testing.T) {
	ctx := context.Background()
	run := NewMockBenchmarkRun()
	run.ID = "delete-test-run"

	// Save
	if err := s.storage.Save(ctx, run); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Delete
	if err := s.storage.Delete(ctx, run.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	_, err := s.storage.Load(ctx, run.ID)
	if err == nil {
		t.Error("Expected error loading deleted run")
	}
}

func (s *StorageTestSuite) testQuery(t *testing.T) {
	ctx := context.Background()

	// Save multiple runs
	for i := 0; i < 5; i++ {
		run := NewMockBenchmarkRun()
		run.ID = fmt.Sprintf("query-test-run-%d", i)
		run.Tags = []string{"query-test"}
		if err := s.storage.Save(ctx, run); err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	// Query
	runs, err := s.storage.Query(ctx, map[string]interface{}{
		"tags": []string{"query-test"},
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(runs) < 5 {
		t.Errorf("Expected at least 5 runs, got %d", len(runs))
	}
}

func (s *StorageTestSuite) testNotFound(t *testing.T) {
	ctx := context.Background()
	_, err := s.storage.Load(ctx, "nonexistent-run-id")
	if err == nil {
		t.Error("Expected error for nonexistent run")
	}
}

// ==================== Analyzer Plugin Tester ====================

// AnalyzerResult represents an analysis result for testing.
type AnalyzerResult struct {
	Score           float64
	Findings        []Finding
	Recommendations []Recommendation
}

// Finding represents an analysis finding.
type Finding struct {
	Type     string
	Severity string
	Message  string
}

// Recommendation represents a recommendation.
type Recommendation struct {
	Title       string
	Description string
	Priority    string
}

// AnalyzerPluginTest defines the interface for testing analyzer plugins.
type AnalyzerPluginTest interface {
	Analyze(ctx context.Context, run *MockBenchmarkRun) (*AnalyzerResult, error)
}

// AnalyzerTestSuite runs tests on an analyzer plugin.
type AnalyzerTestSuite struct {
	t        *testing.T
	analyzer AnalyzerPluginTest
}

// NewAnalyzerTestSuite creates a new analyzer test suite.
func NewAnalyzerTestSuite(t *testing.T, analyzer AnalyzerPluginTest) *AnalyzerTestSuite {
	return &AnalyzerTestSuite{
		t:        t,
		analyzer: analyzer,
	}
}

// Run executes all analyzer tests.
func (s *AnalyzerTestSuite) Run() {
	s.t.Run("AnalyzeNormalRun", s.testAnalyzeNormal)
	s.t.Run("AnalyzeHighLatency", s.testAnalyzeHighLatency)
	s.t.Run("AnalyzeWithErrors", s.testAnalyzeWithErrors)
}

func (s *AnalyzerTestSuite) testAnalyzeNormal(t *testing.T) {
	ctx := context.Background()
	run := NewMockBenchmarkRun()

	result, err := s.analyzer.Analyze(ctx, run)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.Score < 0 || result.Score > 1 {
		t.Errorf("Score out of range: %f", result.Score)
	}
}

func (s *AnalyzerTestSuite) testAnalyzeHighLatency(t *testing.T) {
	ctx := context.Background()
	run := NewMockBenchmarkRun()
	run.LatencyP99 = 10000.0 // 10ms - very high

	result, err := s.analyzer.Analyze(ctx, run)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// High latency should result in findings
	hasLatencyFinding := false
	for _, f := range result.Findings {
		if strings.Contains(strings.ToLower(f.Message), "latency") {
			hasLatencyFinding = true
			break
		}
	}

	if !hasLatencyFinding {
		t.Log("Note: analyzer did not flag high latency")
	}
}

func (s *AnalyzerTestSuite) testAnalyzeWithErrors(t *testing.T) {
	ctx := context.Background()
	run := NewMockBenchmarkRun()
	run.Errors = 1000 // High error count

	result, err := s.analyzer.Analyze(ctx, run)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Errors should affect score
	if result.Score > 0.9 {
		t.Log("Note: analyzer did not penalize errors")
	}
}

// ==================== Benchmark Utilities ====================

// BenchmarkHelper provides utilities for plugin benchmarking.
type BenchmarkHelper struct {
	b *testing.B
}

// NewBenchmarkHelper creates a new benchmark helper.
func NewBenchmarkHelper(b *testing.B) *BenchmarkHelper {
	return &BenchmarkHelper{b: b}
}

// TimeOperation measures the time to execute an operation.
func (h *BenchmarkHelper) TimeOperation(name string, fn func() error) {
	h.b.Helper()
	h.b.Run(name, func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if err := fn(); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// ReportMetric reports a custom metric.
func (h *BenchmarkHelper) ReportMetric(name string, value float64, unit string) {
	h.b.ReportMetric(value, name+"/"+unit)
}

// ==================== Validation Utilities ====================

// ValidationError represents a validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ConfigValidator validates plugin configuration.
type ConfigValidator struct {
	errors []ValidationError
}

// NewConfigValidator creates a new config validator.
func NewConfigValidator() *ConfigValidator {
	return &ConfigValidator{
		errors: make([]ValidationError, 0),
	}
}

// RequireString validates that a string field is present and non-empty.
func (v *ConfigValidator) RequireString(cfg map[string]interface{}, field string) {
	val := GetString(cfg, field, "")
	if val == "" {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: "required string field is missing or empty",
		})
	}
}

// RequireInt validates that an integer field is present.
func (v *ConfigValidator) RequireInt(cfg map[string]interface{}, field string) {
	if _, ok := cfg[field]; !ok {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: "required integer field is missing",
		})
	}
}

// RequirePositiveInt validates that an integer field is positive.
func (v *ConfigValidator) RequirePositiveInt(cfg map[string]interface{}, field string) {
	val := GetInt(cfg, field, 0)
	if val <= 0 {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: "field must be a positive integer",
		})
	}
}

// ValidateURL validates that a field contains a valid URL.
func (v *ConfigValidator) ValidateURL(cfg map[string]interface{}, field string) {
	val := GetString(cfg, field, "")
	if val != "" && !strings.HasPrefix(val, "http://") && !strings.HasPrefix(val, "https://") {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: "field must be a valid HTTP(S) URL",
		})
	}
}

// HasErrors returns true if there are validation errors.
func (v *ConfigValidator) HasErrors() bool {
	return len(v.errors) > 0
}

// Errors returns all validation errors.
func (v *ConfigValidator) Errors() []ValidationError {
	return v.errors
}

// Error returns a combined error message.
func (v *ConfigValidator) Error() error {
	if !v.HasErrors() {
		return nil
	}

	msgs := make([]string, len(v.errors))
	for i, e := range v.errors {
		msgs[i] = e.Error()
	}
	return fmt.Errorf("configuration validation failed: %s", strings.Join(msgs, "; "))
}

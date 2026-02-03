// Package analysis provides performance analysis and comparison capabilities.
package analysis

import (
	"context"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
)

// Analyzer is the interface for performance analyzers.
type Analyzer interface {
	// Name returns the analyzer's name.
	Name() string

	// Description returns a brief description of what this analyzer does.
	Description() string

	// Analyze performs analysis on a benchmark run.
	Analyze(ctx context.Context, run *domain.BenchmarkRun) (*AnalysisReport, error)

	// AnalyzeMultiple analyzes multiple runs together (for trend analysis).
	AnalyzeMultiple(ctx context.Context, runs []*domain.BenchmarkRun) (*AnalysisReport, error)
}

// AnalysisReport contains the results of an analysis.
type AnalysisReport struct {
	// Analyzer name
	Analyzer string `json:"analyzer"`

	// Overall score (0-100, higher is better)
	Score int `json:"score"`

	// Status: "good", "warning", "critical"
	Status string `json:"status"`

	// Summary of findings
	Summary string `json:"summary"`

	// Detailed findings
	Findings []Finding `json:"findings"`

	// Recommendations for improvement
	Recommendations []Recommendation `json:"recommendations"`

	// Additional metrics computed by the analyzer
	Metrics map[string]interface{} `json:"metrics,omitempty"`
}

// Finding represents a specific observation from analysis.
type Finding struct {
	// Category of finding
	Category string `json:"category"`

	// Severity: "info", "warning", "error"
	Severity string `json:"severity"`

	// Title of the finding
	Title string `json:"title"`

	// Detailed description
	Description string `json:"description"`

	// Associated metric name (if applicable)
	Metric string `json:"metric,omitempty"`

	// Actual value observed
	Value interface{} `json:"value,omitempty"`

	// Expected/threshold value
	Expected interface{} `json:"expected,omitempty"`
}

// Recommendation provides actionable advice.
type Recommendation struct {
	// Priority: "high", "medium", "low"
	Priority string `json:"priority"`

	// Title of the recommendation
	Title string `json:"title"`

	// Detailed description of what to do
	Description string `json:"description"`

	// Expected impact if implemented
	Impact string `json:"impact,omitempty"`
}

// AnalyzerRegistry manages available analyzers.
type AnalyzerRegistry struct {
	analyzers map[string]Analyzer
}

// DefaultRegistry is the global analyzer registry.
var DefaultRegistry = NewAnalyzerRegistry()

// NewAnalyzerRegistry creates a new analyzer registry.
func NewAnalyzerRegistry() *AnalyzerRegistry {
	return &AnalyzerRegistry{
		analyzers: make(map[string]Analyzer),
	}
}

// Register adds an analyzer to the registry.
func (r *AnalyzerRegistry) Register(analyzer Analyzer) {
	r.analyzers[analyzer.Name()] = analyzer
}

// Get retrieves an analyzer by name.
func (r *AnalyzerRegistry) Get(name string) (Analyzer, bool) {
	a, ok := r.analyzers[name]
	return a, ok
}

// List returns all registered analyzer names.
func (r *AnalyzerRegistry) List() []string {
	names := make([]string, 0, len(r.analyzers))
	for name := range r.analyzers {
		names = append(names, name)
	}
	return names
}

// All returns all registered analyzers.
func (r *AnalyzerRegistry) All() []Analyzer {
	analyzers := make([]Analyzer, 0, len(r.analyzers))
	for _, a := range r.analyzers {
		analyzers = append(analyzers, a)
	}
	return analyzers
}

// AnalyzeAll runs all registered analyzers on a run.
func (r *AnalyzerRegistry) AnalyzeAll(ctx context.Context, run *domain.BenchmarkRun) ([]*AnalysisReport, error) {
	var reports []*AnalysisReport
	for _, analyzer := range r.analyzers {
		report, err := analyzer.Analyze(ctx, run)
		if err != nil {
			continue // Log but don't fail on individual analyzer errors
		}
		reports = append(reports, report)
	}
	return reports, nil
}

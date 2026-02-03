package reporter

import (
	"context"
	"io"

	"github.com/tfindelkind-redis/redismeter/internal/analysis"
	"github.com/tfindelkind-redis/redismeter/internal/domain"
)

// Reporter is the interface for generating benchmark reports.
type Reporter interface {
	// Name returns the reporter's name.
	Name() string

	// Description returns a description of the report format.
	Description() string

	// ContentType returns the MIME type of the output.
	ContentType() string

	// FileExtension returns the default file extension.
	FileExtension() string

	// Generate generates a report and writes it to the writer.
	Generate(ctx context.Context, report *Report, w io.Writer) error
}

// Report contains all data needed for report generation.
type Report struct {
	// Title of the report
	Title string

	// Description of the report
	Description string

	// Generation timestamp
	GeneratedAt string

	// Single run report
	Run *domain.BenchmarkRun

	// Multiple runs for comparison
	Runs []*domain.BenchmarkRun

	// Comparison results
	Comparison *analysis.RunComparison

	// Baseline comparison
	BaselineComparison *domain.ComparisonResult

	// Analysis results
	Analysis []*analysis.AnalysisReport

	// Baseline for comparison
	Baseline *domain.Baseline

	// Custom metadata
	Metadata map[string]interface{}
}

// ReportOptions configures report generation.
type ReportOptions struct {
	// Title override
	Title string

	// Include charts/visualizations
	IncludeCharts bool

	// Include raw data tables
	IncludeRawData bool

	// Include recommendations
	IncludeRecommendations bool

	// Include environment details
	IncludeEnvironment bool

	// Include comparison to baseline
	IncludeBaselineComparison bool

	// Custom CSS for HTML reports
	CustomCSS string

	// Template override
	TemplatePath string

	// Logo path for branded reports
	LogoPath string
}

// DefaultOptions returns default report options.
func DefaultOptions() ReportOptions {
	return ReportOptions{
		IncludeCharts:             true,
		IncludeRawData:            true,
		IncludeRecommendations:    true,
		IncludeEnvironment:        true,
		IncludeBaselineComparison: true,
	}
}

// Registry manages available reporters.
type Registry struct {
	reporters map[string]Reporter
}

// NewRegistry creates a new reporter registry.
func NewRegistry() *Registry {
	r := &Registry{
		reporters: make(map[string]Reporter),
	}

	// Register built-in reporters
	r.Register(&HTMLReporter{})
	r.Register(&MarkdownReporter{})
	r.Register(&JSONReporter{})
	r.Register(&TextReporter{})

	return r
}

// Register adds a reporter to the registry.
func (r *Registry) Register(reporter Reporter) {
	r.reporters[reporter.Name()] = reporter
}

// Get retrieves a reporter by name.
func (r *Registry) Get(name string) (Reporter, bool) {
	reporter, ok := r.reporters[name]
	return reporter, ok
}

// List returns all registered reporters.
func (r *Registry) List() []Reporter {
	reporters := make([]Reporter, 0, len(r.reporters))
	for _, reporter := range r.reporters {
		reporters = append(reporters, reporter)
	}
	return reporters
}

// Names returns the names of all registered reporters.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.reporters))
	for name := range r.reporters {
		names = append(names, name)
	}
	return names
}

// DefaultRegistry is the default reporter registry.
var DefaultRegistry = NewRegistry()

// Package redismeter provides a Go client for the RedisMeter API.
//
// # Quick Start
//
//	client := redismeter.NewClient("http://localhost:8080",
//	    redismeter.WithAPIKey("your-api-key"),
//	)
//
//	run, err := client.RunBenchmark(ctx, &redismeter.RunBenchmarkRequest{
//	    Target: &redismeter.Target{Host: "localhost", Port: 6379},
//	    Workload: "mixed",
//	    Duration: "30s",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	fmt.Printf("Throughput: %.2f ops/sec\n", run.Results.Summary.OpsPerSec)
package redismeter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client is a RedisMeter API client.
type Client struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
	headers    map[string]string
}

// ClientOption configures the client.
type ClientOption func(*Client)

// WithAPIKey sets the API key for authentication.
func WithAPIKey(key string) ClientOption {
	return func(c *Client) {
		c.apiKey = key
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = client
	}
}

// WithTimeout sets the request timeout.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// WithHeaders sets additional headers to include in requests.
func WithHeaders(headers map[string]string) ClientOption {
	return func(c *Client) {
		for k, v := range headers {
			c.headers[k] = v
		}
	}
}

// NewClient creates a new RedisMeter API client.
func NewClient(baseURL string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		headers: map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
			"User-Agent":   "redismeter-go/1.0.0",
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.apiKey != "" {
		c.headers["X-API-Key"] = c.apiKey
	}

	return c
}

// Error represents an API error.
type Error struct {
	StatusCode int
	Code       string
	Message    string
	Details    interface{}
}

func (e *Error) Error() string {
	return fmt.Sprintf("redismeter: %s (status=%d, code=%s)", e.Message, e.StatusCode, e.Code)
}

// IsNotFound returns true if the error is a 404 Not Found.
func IsNotFound(err error) bool {
	if e, ok := err.(*Error); ok {
		return e.StatusCode == 404
	}
	return false
}

// IsAuthError returns true if the error is an authentication error.
func IsAuthError(err error) bool {
	if e, ok := err.(*Error); ok {
		return e.StatusCode == 401
	}
	return false
}

// IsRateLimitError returns true if the error is a rate limit error.
func IsRateLimitError(err error) bool {
	if e, ok := err.(*Error); ok {
		return e.StatusCode == 429
	}
	return false
}

func (c *Client) request(ctx context.Context, method, path string, query url.Values, body interface{}) ([]byte, error) {
	u := c.baseURL + path
	if query != nil && len(query) > 0 {
		u += "?" + query.Encode()
	}

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Message string      `json:"message"`
			Code    string      `json:"code"`
			Details interface{} `json:"details"`
		}
		json.Unmarshal(respBody, &errResp)

		return nil, &Error{
			StatusCode: resp.StatusCode,
			Code:       errResp.Code,
			Message:    errResp.Message,
			Details:    errResp.Details,
		}
	}

	return respBody, nil
}

// ==================== Models ====================

// BenchmarkRun represents a benchmark run.
type BenchmarkRun struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Status      string            `json:"status"`
	Workload    *Workload         `json:"workload"`
	Target      *Target           `json:"target"`
	Results     *Results          `json:"results"`
	Tags        []string          `json:"tags"`
	Labels      map[string]string `json:"labels"`
	CreatedAt   time.Time         `json:"created_at"`
	StartedAt   time.Time         `json:"started_at"`
	CompletedAt time.Time         `json:"completed_at"`
	Duration    string            `json:"duration"`
	Error       string            `json:"error"`
}

// Workload represents a benchmark workload.
type Workload struct {
	Name       string       `json:"name"`
	Type       string       `json:"type"`
	Duration   string       `json:"duration"`
	Clients    int          `json:"clients"`
	Threads    int          `json:"threads"`
	Operations []*Operation `json:"operations"`
}

// Operation represents a workload operation.
type Operation struct {
	Command string                 `json:"command"`
	Ratio   float64                `json:"ratio"`
	Args    map[string]interface{} `json:"args"`
}

// Target represents a Redis target.
type Target struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Password string `json:"password,omitempty"`
	TLS      bool   `json:"tls"`
	Database int    `json:"database"`
}

// Results represents benchmark results.
type Results struct {
	Summary          *SummaryMetrics    `json:"summary"`
	LatencyHistogram []*HistogramBucket `json:"latency_histogram"`
	TimeSeries       []*TimeSeriesPoint `json:"time_series"`
}

// SummaryMetrics represents summary statistics.
type SummaryMetrics struct {
	TotalOps     int64   `json:"total_ops"`
	OpsPerSec    float64 `json:"ops_per_sec"`
	TotalBytes   int64   `json:"total_bytes"`
	BytesPerSec  float64 `json:"bytes_per_sec"`
	LatencyAvgUs float64 `json:"latency_avg_us"`
	LatencyMinUs float64 `json:"latency_min_us"`
	LatencyMaxUs float64 `json:"latency_max_us"`
	LatencyP50Us float64 `json:"latency_p50_us"`
	LatencyP95Us float64 `json:"latency_p95_us"`
	LatencyP99Us float64 `json:"latency_p99_us"`
	LatencyP999Us float64 `json:"latency_p999_us"`
	Errors       int64   `json:"errors"`
	ErrorRate    float64 `json:"error_rate"`
}

// HistogramBucket represents a latency histogram bucket.
type HistogramBucket struct {
	LowerBound float64 `json:"lower_bound"`
	UpperBound float64 `json:"upper_bound"`
	Count      int64   `json:"count"`
}

// TimeSeriesPoint represents a time series data point.
type TimeSeriesPoint struct {
	Timestamp   time.Time `json:"timestamp"`
	OpsPerSec   float64   `json:"ops_per_sec"`
	LatencyP50  float64   `json:"latency_p50"`
	LatencyP99  float64   `json:"latency_p99"`
	ErrorCount  int64     `json:"error_count"`
	Connections int       `json:"connections"`
}

// Baseline represents a performance baseline.
type Baseline struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	RunID       string             `json:"run_id"`
	Workload    string             `json:"workload"`
	Metrics     *SummaryMetrics    `json:"metrics"`
	Thresholds  map[string]float64 `json:"thresholds"`
	Active      bool               `json:"active"`
	Tags        []string           `json:"tags"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
}

// ComparisonResult represents a comparison between runs.
type ComparisonResult struct {
	BaseRun            *BenchmarkRun    `json:"base_run"`
	CompareRun         *BenchmarkRun    `json:"compare_run"`
	Baseline           *Baseline        `json:"baseline,omitempty"`
	Differences        []*MetricDiff    `json:"differences"`
	RegressionDetected bool             `json:"regression_detected"`
	Regressions        []*MetricDiff    `json:"regressions"`
	Improvements       []*MetricDiff    `json:"improvements"`
}

// MetricDiff represents a difference in a metric.
type MetricDiff struct {
	Metric        string  `json:"metric"`
	BaseValue     float64 `json:"base_value"`
	CompareValue  float64 `json:"compare_value"`
	Difference    float64 `json:"difference"`
	ChangePercent float64 `json:"change_percent"`
	IsRegression  bool    `json:"is_regression"`
}

// AnalysisResult represents an analysis result.
type AnalysisResult struct {
	Analyzer        string                 `json:"analyzer"`
	Score           float64                `json:"score"`
	Findings        []*Finding             `json:"findings"`
	Recommendations []*Recommendation      `json:"recommendations"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// Finding represents an analysis finding.
type Finding struct {
	Type     string                 `json:"type"`
	Severity string                 `json:"severity"`
	Message  string                 `json:"message"`
	Details  map[string]interface{} `json:"details"`
}

// Recommendation represents a recommendation.
type Recommendation struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	Impact      string `json:"impact"`
}

// ==================== Runs ====================

// ListRunsOptions specifies options for listing runs.
type ListRunsOptions struct {
	Limit    int
	Offset   int
	Status   string
	Workload string
	Tags     []string
}

// ListRuns lists benchmark runs.
func (c *Client) ListRuns(ctx context.Context, opts *ListRunsOptions) ([]*BenchmarkRun, error) {
	query := url.Values{}
	if opts != nil {
		if opts.Limit > 0 {
			query.Set("limit", strconv.Itoa(opts.Limit))
		}
		if opts.Offset > 0 {
			query.Set("offset", strconv.Itoa(opts.Offset))
		}
		if opts.Status != "" {
			query.Set("status", opts.Status)
		}
		if opts.Workload != "" {
			query.Set("workload", opts.Workload)
		}
		if len(opts.Tags) > 0 {
			query.Set("tags", strings.Join(opts.Tags, ","))
		}
	}

	data, err := c.request(ctx, "GET", "/api/v1/runs", query, nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Runs []*BenchmarkRun `json:"runs"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return resp.Runs, nil
}

// GetRun gets a benchmark run by ID.
func (c *Client) GetRun(ctx context.Context, runID string) (*BenchmarkRun, error) {
	data, err := c.request(ctx, "GET", "/api/v1/runs/"+runID, nil, nil)
	if err != nil {
		return nil, err
	}

	var run BenchmarkRun
	if err := json.Unmarshal(data, &run); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &run, nil
}

// DeleteRun deletes a benchmark run.
func (c *Client) DeleteRun(ctx context.Context, runID string) error {
	_, err := c.request(ctx, "DELETE", "/api/v1/runs/"+runID, nil, nil)
	return err
}

// RunBenchmarkRequest specifies parameters for running a benchmark.
type RunBenchmarkRequest struct {
	Target      *Target           `json:"target"`
	Workload    interface{}       `json:"workload"` // string name or Workload struct
	Duration    string            `json:"duration,omitempty"`
	Clients     int               `json:"clients,omitempty"`
	Threads     int               `json:"threads,omitempty"`
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// RunBenchmarkOptions specifies options for running a benchmark.
type RunBenchmarkOptions struct {
	Wait         bool
	PollInterval time.Duration
	Timeout      time.Duration
}

// RunBenchmark runs a benchmark.
func (c *Client) RunBenchmark(ctx context.Context, req *RunBenchmarkRequest, opts *RunBenchmarkOptions) (*BenchmarkRun, error) {
	data, err := c.request(ctx, "POST", "/api/v1/benchmark", nil, req)
	if err != nil {
		return nil, err
	}

	var run BenchmarkRun
	if err := json.Unmarshal(data, &run); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if opts == nil || !opts.Wait {
		return &run, nil
	}

	// Poll for completion
	pollInterval := opts.PollInterval
	if pollInterval == 0 {
		pollInterval = time.Second
	}

	startTime := time.Now()
	for run.Status == "pending" || run.Status == "running" {
		if opts.Timeout > 0 && time.Since(startTime) > opts.Timeout {
			return &run, fmt.Errorf("timeout waiting for benchmark completion")
		}

		select {
		case <-ctx.Done():
			return &run, ctx.Err()
		case <-time.After(pollInterval):
		}

		updatedRun, err := c.GetRun(ctx, run.ID)
		if err != nil {
			return &run, fmt.Errorf("poll run status: %w", err)
		}
		run = *updatedRun
	}

	return &run, nil
}

// ==================== Baselines ====================

// ListBaselinesOptions specifies options for listing baselines.
type ListBaselinesOptions struct {
	Limit      int
	Offset     int
	ActiveOnly bool
	Name       string
}

// ListBaselines lists baselines.
func (c *Client) ListBaselines(ctx context.Context, opts *ListBaselinesOptions) ([]*Baseline, error) {
	query := url.Values{}
	if opts != nil {
		if opts.Limit > 0 {
			query.Set("limit", strconv.Itoa(opts.Limit))
		}
		if opts.Offset > 0 {
			query.Set("offset", strconv.Itoa(opts.Offset))
		}
		if opts.ActiveOnly {
			query.Set("active", "true")
		}
		if opts.Name != "" {
			query.Set("name", opts.Name)
		}
	}

	data, err := c.request(ctx, "GET", "/api/v1/baselines", query, nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Baselines []*Baseline `json:"baselines"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return resp.Baselines, nil
}

// GetBaseline gets a baseline by ID.
func (c *Client) GetBaseline(ctx context.Context, baselineID string) (*Baseline, error) {
	data, err := c.request(ctx, "GET", "/api/v1/baselines/"+baselineID, nil, nil)
	if err != nil {
		return nil, err
	}

	var baseline Baseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &baseline, nil
}

// CreateBaselineRequest specifies parameters for creating a baseline.
type CreateBaselineRequest struct {
	Name        string             `json:"name"`
	RunID       string             `json:"run_id"`
	Description string             `json:"description,omitempty"`
	Thresholds  map[string]float64 `json:"thresholds,omitempty"`
	Tags        []string           `json:"tags,omitempty"`
}

// CreateBaseline creates a baseline from a benchmark run.
func (c *Client) CreateBaseline(ctx context.Context, req *CreateBaselineRequest) (*Baseline, error) {
	data, err := c.request(ctx, "POST", "/api/v1/baselines", nil, req)
	if err != nil {
		return nil, err
	}

	var baseline Baseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &baseline, nil
}

// DeleteBaseline deletes a baseline.
func (c *Client) DeleteBaseline(ctx context.Context, baselineID string) error {
	_, err := c.request(ctx, "DELETE", "/api/v1/baselines/"+baselineID, nil, nil)
	return err
}

// SetActiveBaseline sets a baseline as active.
func (c *Client) SetActiveBaseline(ctx context.Context, baselineID string) (*Baseline, error) {
	data, err := c.request(ctx, "PUT", "/api/v1/baselines/"+baselineID+"/active", nil, nil)
	if err != nil {
		return nil, err
	}

	var baseline Baseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &baseline, nil
}

// ==================== Comparison ====================

// CompareRuns compares two benchmark runs.
func (c *Client) CompareRuns(ctx context.Context, runID, otherRunID string) (*ComparisonResult, error) {
	query := url.Values{}
	query.Set("run_id", runID)
	query.Set("other_run_id", otherRunID)

	data, err := c.request(ctx, "GET", "/api/v1/compare", query, nil)
	if err != nil {
		return nil, err
	}

	var result ComparisonResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &result, nil
}

// CompareToBaseline compares a run to a baseline.
func (c *Client) CompareToBaseline(ctx context.Context, runID string, baselineID string) (*ComparisonResult, error) {
	query := url.Values{}
	query.Set("run_id", runID)
	if baselineID != "" {
		query.Set("baseline_id", baselineID)
	}

	data, err := c.request(ctx, "GET", "/api/v1/compare/baseline", query, nil)
	if err != nil {
		return nil, err
	}

	var result ComparisonResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &result, nil
}

// ==================== Analysis ====================

// AnalyzeRun analyzes a benchmark run.
func (c *Client) AnalyzeRun(ctx context.Context, runID string, analyzers []string) ([]*AnalysisResult, error) {
	query := url.Values{}
	query.Set("run_id", runID)
	if len(analyzers) > 0 {
		query.Set("analyzers", strings.Join(analyzers, ","))
	}

	data, err := c.request(ctx, "GET", "/api/v1/analyze", query, nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Results []*AnalysisResult `json:"results"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return resp.Results, nil
}

// ==================== Workloads ====================

// WorkloadInfo represents workload metadata.
type WorkloadInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// ListWorkloads lists available workloads.
func (c *Client) ListWorkloads(ctx context.Context) ([]*WorkloadInfo, error) {
	data, err := c.request(ctx, "GET", "/api/v1/workloads", nil, nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Workloads []*WorkloadInfo `json:"workloads"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return resp.Workloads, nil
}

// GetWorkload gets a workload by name.
func (c *Client) GetWorkload(ctx context.Context, name string) (*Workload, error) {
	data, err := c.request(ctx, "GET", "/api/v1/workloads/"+name, nil, nil)
	if err != nil {
		return nil, err
	}

	var workload Workload
	if err := json.Unmarshal(data, &workload); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &workload, nil
}

// ==================== Health ====================

// HealthStatus represents the health status.
type HealthStatus struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
}

// Health checks the API health.
func (c *Client) Health(ctx context.Context) (*HealthStatus, error) {
	data, err := c.request(ctx, "GET", "/health", nil, nil)
	if err != nil {
		return nil, err
	}

	var status HealthStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &status, nil
}

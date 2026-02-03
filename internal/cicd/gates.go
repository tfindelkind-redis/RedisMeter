// Package cicd provides CI/CD integration utilities.
package cicd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/tfindelkind-redis/redismeter/internal/analysis"
	"github.com/tfindelkind-redis/redismeter/internal/domain"
)

// Environment represents the detected CI/CD environment.
type Environment string

const (
	EnvironmentGitHub     Environment = "github"
	EnvironmentGitLab     Environment = "gitlab"
	EnvironmentJenkins    Environment = "jenkins"
	EnvironmentCircleCI   Environment = "circleci"
	EnvironmentAzureDevOps Environment = "azure_devops"
	EnvironmentUnknown    Environment = "unknown"
	EnvironmentLocal      Environment = "local"
)

// CIInfo contains information about the CI environment.
type CIInfo struct {
	Environment Environment `json:"environment"`
	Branch      string      `json:"branch"`
	Commit      string      `json:"commit"`
	CommitShort string      `json:"commit_short"`
	BuildNumber string      `json:"build_number"`
	BuildURL    string      `json:"build_url"`
	Repository  string      `json:"repository"`
	PullRequest string      `json:"pull_request"`
	Author      string      `json:"author"`
	Message     string      `json:"message"`
}

// DetectEnvironment detects the current CI/CD environment.
func DetectEnvironment() Environment {
	switch {
	case os.Getenv("GITHUB_ACTIONS") == "true":
		return EnvironmentGitHub
	case os.Getenv("GITLAB_CI") == "true":
		return EnvironmentGitLab
	case os.Getenv("JENKINS_URL") != "":
		return EnvironmentJenkins
	case os.Getenv("CIRCLECI") == "true":
		return EnvironmentCircleCI
	case os.Getenv("TF_BUILD") == "True":
		return EnvironmentAzureDevOps
	case os.Getenv("CI") == "true":
		return EnvironmentUnknown
	default:
		return EnvironmentLocal
	}
}

// GetCIInfo retrieves CI information from environment variables.
func GetCIInfo() *CIInfo {
	env := DetectEnvironment()
	info := &CIInfo{
		Environment: env,
	}

	switch env {
	case EnvironmentGitHub:
		info.Branch = os.Getenv("GITHUB_REF_NAME")
		info.Commit = os.Getenv("GITHUB_SHA")
		if len(info.Commit) > 7 {
			info.CommitShort = info.Commit[:7]
		}
		info.BuildNumber = os.Getenv("GITHUB_RUN_NUMBER")
		info.BuildURL = fmt.Sprintf("%s/%s/actions/runs/%s",
			os.Getenv("GITHUB_SERVER_URL"),
			os.Getenv("GITHUB_REPOSITORY"),
			os.Getenv("GITHUB_RUN_ID"))
		info.Repository = os.Getenv("GITHUB_REPOSITORY")
		info.PullRequest = os.Getenv("GITHUB_EVENT_NAME")
		info.Author = os.Getenv("GITHUB_ACTOR")

	case EnvironmentGitLab:
		info.Branch = os.Getenv("CI_COMMIT_REF_NAME")
		info.Commit = os.Getenv("CI_COMMIT_SHA")
		if len(info.Commit) > 7 {
			info.CommitShort = info.Commit[:7]
		}
		info.BuildNumber = os.Getenv("CI_PIPELINE_ID")
		info.BuildURL = os.Getenv("CI_PIPELINE_URL")
		info.Repository = os.Getenv("CI_PROJECT_PATH")
		info.PullRequest = os.Getenv("CI_MERGE_REQUEST_IID")
		info.Author = os.Getenv("GITLAB_USER_LOGIN")
		info.Message = os.Getenv("CI_COMMIT_MESSAGE")

	case EnvironmentJenkins:
		info.Branch = os.Getenv("GIT_BRANCH")
		info.Commit = os.Getenv("GIT_COMMIT")
		if len(info.Commit) > 7 {
			info.CommitShort = info.Commit[:7]
		}
		info.BuildNumber = os.Getenv("BUILD_NUMBER")
		info.BuildURL = os.Getenv("BUILD_URL")
		info.Repository = os.Getenv("GIT_URL")
		info.PullRequest = os.Getenv("CHANGE_ID")

	case EnvironmentCircleCI:
		info.Branch = os.Getenv("CIRCLE_BRANCH")
		info.Commit = os.Getenv("CIRCLE_SHA1")
		if len(info.Commit) > 7 {
			info.CommitShort = info.Commit[:7]
		}
		info.BuildNumber = os.Getenv("CIRCLE_BUILD_NUM")
		info.BuildURL = os.Getenv("CIRCLE_BUILD_URL")
		info.Repository = os.Getenv("CIRCLE_REPOSITORY_URL")
		info.PullRequest = os.Getenv("CIRCLE_PULL_REQUEST")
		info.Author = os.Getenv("CIRCLE_USERNAME")

	case EnvironmentAzureDevOps:
		info.Branch = os.Getenv("BUILD_SOURCEBRANCHNAME")
		info.Commit = os.Getenv("BUILD_SOURCEVERSION")
		if len(info.Commit) > 7 {
			info.CommitShort = info.Commit[:7]
		}
		info.BuildNumber = os.Getenv("BUILD_BUILDNUMBER")
		info.BuildURL = fmt.Sprintf("%s%s/_build/results?buildId=%s",
			os.Getenv("SYSTEM_COLLECTIONURI"),
			os.Getenv("SYSTEM_TEAMPROJECT"),
			os.Getenv("BUILD_BUILDID"))
		info.Repository = os.Getenv("BUILD_REPOSITORY_NAME")
		info.PullRequest = os.Getenv("SYSTEM_PULLREQUEST_PULLREQUESTID")
		info.Author = os.Getenv("BUILD_REQUESTEDFOR")
		info.Message = os.Getenv("BUILD_SOURCEVERSIONMESSAGE")
	}

	return info
}

// PerformanceGate defines thresholds for pass/fail criteria.
type PerformanceGate struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Checks      []GateCheck        `json:"checks"`
	Baseline    *BaselineReference `json:"baseline,omitempty"`
}

// GateCheck defines a single check condition.
type GateCheck struct {
	// Metric to check: throughput, avg_latency, p99_latency, error_rate, etc.
	Metric string `json:"metric"`

	// Operator: gt, lt, gte, lte, eq, within_percent
	Operator string `json:"operator"`

	// Value threshold
	Value float64 `json:"value"`

	// Required determines if this check must pass
	Required bool `json:"required"`

	// Description of the check
	Description string `json:"description"`
}

// BaselineReference references a baseline for comparison.
type BaselineReference struct {
	// Type: "previous", "tag", "branch", "run_id"
	Type string `json:"type"`

	// Value depends on Type
	Value string `json:"value"`

	// MaxDeviation is the maximum allowed deviation percentage
	MaxDeviation float64 `json:"max_deviation"`
}

// GateResult contains the result of a gate evaluation.
type GateResult struct {
	Gate         *PerformanceGate `json:"gate"`
	Passed       bool             `json:"passed"`
	CheckResults []CheckResult    `json:"check_results"`
	Comparison   *analysis.RunComparison `json:"comparison,omitempty"`
	Summary      string           `json:"summary"`
}

// CheckResult contains the result of a single check.
type CheckResult struct {
	Check       GateCheck `json:"check"`
	Passed      bool      `json:"passed"`
	ActualValue float64   `json:"actual_value"`
	Message     string    `json:"message"`
}

// GateEvaluator evaluates performance gates.
type GateEvaluator struct {
	gates []PerformanceGate
}

// NewGateEvaluator creates a new gate evaluator.
func NewGateEvaluator() *GateEvaluator {
	return &GateEvaluator{
		gates: []PerformanceGate{},
	}
}

// AddGate adds a gate to evaluate.
func (e *GateEvaluator) AddGate(gate PerformanceGate) {
	e.gates = append(e.gates, gate)
}

// LoadGates loads gates from a configuration.
func (e *GateEvaluator) LoadGates(gates []PerformanceGate) {
	e.gates = gates
}

// Evaluate evaluates all gates against a benchmark run.
func (e *GateEvaluator) Evaluate(ctx context.Context, run *domain.BenchmarkRun) ([]GateResult, error) {
	var results []GateResult

	for _, gate := range e.gates {
		result := e.evaluateGate(ctx, gate, run)
		results = append(results, result)
	}

	return results, nil
}

// EvaluateWithBaseline evaluates gates with baseline comparison.
func (e *GateEvaluator) EvaluateWithBaseline(ctx context.Context, run *domain.BenchmarkRun, baseline *domain.BenchmarkRun) ([]GateResult, error) {
	var results []GateResult

	// Run comparison if baseline provided
	var comparison *analysis.RunComparison
	if baseline != nil {
		comparator := analysis.NewComparator(nil)
		comparison = comparator.Compare(run, baseline)
	}

	for _, gate := range e.gates {
		result := e.evaluateGate(ctx, gate, run)
		result.Comparison = comparison

		// Check baseline deviation if configured
		if gate.Baseline != nil && comparison != nil {
			baselineResult := e.checkBaselineDeviation(gate, comparison)
			if !baselineResult.Passed {
				result.Passed = false
				result.CheckResults = append(result.CheckResults, baselineResult)
			}
		}

		results = append(results, result)
	}

	return results, nil
}

func (e *GateEvaluator) evaluateGate(ctx context.Context, gate PerformanceGate, run *domain.BenchmarkRun) GateResult {
	result := GateResult{
		Gate:   &gate,
		Passed: true,
	}

	if run.Results == nil || run.Results.Summary == nil {
		result.Passed = false
		result.Summary = "No results available"
		return result
	}

	for _, check := range gate.Checks {
		checkResult := e.evaluateCheck(check, run)
		result.CheckResults = append(result.CheckResults, checkResult)

		if !checkResult.Passed && check.Required {
			result.Passed = false
		}
	}

	if result.Passed {
		result.Summary = fmt.Sprintf("All checks passed (%d/%d)", len(result.CheckResults), len(gate.Checks))
	} else {
		failedCount := 0
		for _, cr := range result.CheckResults {
			if !cr.Passed {
				failedCount++
			}
		}
		result.Summary = fmt.Sprintf("Failed: %d/%d checks", failedCount, len(gate.Checks))
	}

	return result
}

func (e *GateEvaluator) evaluateCheck(check GateCheck, run *domain.BenchmarkRun) CheckResult {
	result := CheckResult{
		Check: check,
	}

	summary := run.Results.Summary
	var actualValue float64

	switch check.Metric {
	case "throughput", "ops_per_second":
		actualValue = summary.OpsPerSecond
	case "avg_latency", "avg_latency_ms":
		actualValue = summary.AvgLatencyMs
	case "p50_latency", "p50_latency_ms":
		actualValue = summary.P50LatencyMs
	case "p90_latency", "p90_latency_ms":
		actualValue = summary.P90LatencyMs
	case "p95_latency", "p95_latency_ms":
		actualValue = summary.P95LatencyMs
	case "p99_latency", "p99_latency_ms":
		actualValue = summary.P99LatencyMs
	case "error_rate":
		actualValue = summary.ErrorRate
	case "errors":
		actualValue = float64(summary.Errors)
	default:
		result.Message = fmt.Sprintf("Unknown metric: %s", check.Metric)
		result.Passed = false
		return result
	}

	result.ActualValue = actualValue
	result.Passed = e.compareValue(actualValue, check.Operator, check.Value)

	if result.Passed {
		result.Message = fmt.Sprintf("%s: %.2f %s %.2f ✓",
			check.Metric, actualValue, check.Operator, check.Value)
	} else {
		result.Message = fmt.Sprintf("%s: %.2f %s %.2f ✗ (expected %s %.2f)",
			check.Metric, actualValue, "not", check.Value, check.Operator, check.Value)
	}

	return result
}

func (e *GateEvaluator) compareValue(actual float64, operator string, threshold float64) bool {
	switch operator {
	case "gt", ">":
		return actual > threshold
	case "lt", "<":
		return actual < threshold
	case "gte", ">=":
		return actual >= threshold
	case "lte", "<=":
		return actual <= threshold
	case "eq", "==":
		return actual == threshold
	case "neq", "!=":
		return actual != threshold
	default:
		return false
	}
}

func (e *GateEvaluator) checkBaselineDeviation(gate PerformanceGate, comparison *analysis.RunComparison) CheckResult {
	result := CheckResult{
		Check: GateCheck{
			Metric:      "baseline_deviation",
			Description: "Baseline deviation check",
			Required:    true,
		},
		Passed: true,
	}

	maxDeviation := gate.Baseline.MaxDeviation
	if maxDeviation == 0 {
		maxDeviation = 10.0 // Default 10%
	}

	// Check throughput regression (run2 vs run1 - negative means regression)
	if comparison.Metrics != nil {
		throughputChange := comparison.Metrics.ThroughputDiff
		if throughputChange < -maxDeviation {
			result.Passed = false
			result.Message = fmt.Sprintf("Throughput regression: %.1f%% (max allowed: %.1f%%)",
				throughputChange, maxDeviation)
			result.ActualValue = throughputChange
			return result
		}

		// Check latency regression (positive change means worse)
		latencyChanges := map[string]float64{
			"avg_latency": comparison.Metrics.AvgLatencyDiff,
			"p50_latency": comparison.Metrics.P50LatencyDiff,
			"p99_latency": comparison.Metrics.P99LatencyDiff,
		}
		for metric, change := range latencyChanges {
			if change > maxDeviation {
				result.Passed = false
				result.Message = fmt.Sprintf("%s increased by %.1f%% (max allowed: %.1f%%)",
					metric, change, maxDeviation)
				result.ActualValue = change
				return result
			}
		}
	}

	result.Message = fmt.Sprintf("Within baseline deviation (max %.1f%%)", maxDeviation)
	return result
}

// AllPassed returns true if all gates passed.
func AllPassed(results []GateResult) bool {
	for _, r := range results {
		if !r.Passed {
			return false
		}
	}
	return true
}

// GitHubOutput writes results in GitHub Actions format.
type GitHubOutput struct {
	writer *os.File
}

// NewGitHubOutput creates output for GitHub Actions.
func NewGitHubOutput() *GitHubOutput {
	outputFile := os.Getenv("GITHUB_OUTPUT")
	if outputFile == "" {
		return &GitHubOutput{}
	}

	f, err := os.OpenFile(outputFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return &GitHubOutput{}
	}

	return &GitHubOutput{writer: f}
}

// SetOutput sets a GitHub Actions output variable.
func (g *GitHubOutput) SetOutput(name, value string) {
	if g.writer == nil {
		return
	}
	fmt.Fprintf(g.writer, "%s=%s\n", name, value)
}

// SetMultilineOutput sets a multiline GitHub Actions output.
func (g *GitHubOutput) SetMultilineOutput(name, value string) {
	if g.writer == nil {
		return
	}
	delimiter := "EOF"
	fmt.Fprintf(g.writer, "%s<<%s\n%s\n%s\n", name, delimiter, value, delimiter)
}

// Close closes the output file.
func (g *GitHubOutput) Close() {
	if g.writer != nil {
		g.writer.Close()
	}
}

// WriteGateResultsToGitHub writes gate results to GitHub Actions.
func WriteGateResultsToGitHub(results []GateResult) {
	output := NewGitHubOutput()
	defer output.Close()

	passed := AllPassed(results)
	output.SetOutput("passed", strconv.FormatBool(passed))

	var summaries []string
	for _, r := range results {
		status := "✓"
		if !r.Passed {
			status = "✗"
		}
		summaries = append(summaries, fmt.Sprintf("%s %s: %s", status, r.Gate.Name, r.Summary))
	}

	output.SetMultilineOutput("summary", strings.Join(summaries, "\n"))

	// JSON results
	jsonResults, _ := json.Marshal(results)
	output.SetOutput("results_json", string(jsonResults))
}

// WritePRComment formats results for a PR comment.
func WritePRComment(results []GateResult, run *domain.BenchmarkRun) string {
	var sb strings.Builder

	passed := AllPassed(results)

	if passed {
		sb.WriteString("## ✅ Performance Check Passed\n\n")
	} else {
		sb.WriteString("## ❌ Performance Check Failed\n\n")
	}

	// Summary table
	if run.Results != nil && run.Results.Summary != nil {
		summary := run.Results.Summary
		sb.WriteString("### Benchmark Results\n\n")
		sb.WriteString("| Metric | Value |\n")
		sb.WriteString("|--------|-------|\n")
		sb.WriteString(fmt.Sprintf("| Throughput | %.2f ops/sec |\n", summary.OpsPerSecond))
		sb.WriteString(fmt.Sprintf("| Avg Latency | %.2f ms |\n", summary.AvgLatencyMs))
		sb.WriteString(fmt.Sprintf("| P99 Latency | %.2f ms |\n", summary.P99LatencyMs))
		sb.WriteString(fmt.Sprintf("| Error Rate | %.4f%% |\n", summary.ErrorRate*100))
		sb.WriteString("\n")
	}

	// Gate results
	sb.WriteString("### Performance Gates\n\n")
	for _, r := range results {
		status := "✅"
		if !r.Passed {
			status = "❌"
		}
		sb.WriteString(fmt.Sprintf("#### %s %s\n\n", status, r.Gate.Name))

		if len(r.CheckResults) > 0 {
			sb.WriteString("| Check | Result | Details |\n")
			sb.WriteString("|-------|--------|--------|\n")
			for _, cr := range r.CheckResults {
				checkStatus := "✅"
				if !cr.Passed {
					checkStatus = "❌"
				}
				sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n",
					cr.Check.Description, checkStatus, cr.Message))
			}
			sb.WriteString("\n")
		}

		// Comparison if available
		if r.Comparison != nil && r.Comparison.Metrics != nil {
			sb.WriteString("**Comparison with Baseline:**\n")
			sb.WriteString(fmt.Sprintf("- Throughput Change: %+.1f%%\n", r.Comparison.Metrics.ThroughputDiff))
			sb.WriteString(fmt.Sprintf("- Avg Latency Change: %+.1f%%\n", r.Comparison.Metrics.AvgLatencyDiff))
			sb.WriteString(fmt.Sprintf("- P99 Latency Change: %+.1f%%\n", r.Comparison.Metrics.P99LatencyDiff))
			sb.WriteString("\n")
		}
	}

	// CI info
	ciInfo := GetCIInfo()
	sb.WriteString("<details>\n<summary>CI Details</summary>\n\n")
	sb.WriteString(fmt.Sprintf("- Environment: %s\n", ciInfo.Environment))
	sb.WriteString(fmt.Sprintf("- Branch: %s\n", ciInfo.Branch))
	sb.WriteString(fmt.Sprintf("- Commit: %s\n", ciInfo.CommitShort))
	sb.WriteString(fmt.Sprintf("- Build: %s\n", ciInfo.BuildNumber))
	sb.WriteString("\n</details>\n")

	return sb.String()
}

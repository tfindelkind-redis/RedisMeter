// Package alerting provides notification and alerting capabilities.
package alerting

import (
	"context"
	"encoding/json"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/analysis"
	"github.com/tfindelkind-redis/redismeter/internal/domain"
)

// Alert represents an alert to be sent.
type Alert struct {
	// ID is a unique identifier for the alert
	ID string `json:"id"`

	// Type of alert: "anomaly", "threshold", "baseline_deviation", "benchmark_failed"
	Type string `json:"type"`

	// Severity: "info", "warning", "critical"
	Severity string `json:"severity"`

	// Title is a short description
	Title string `json:"title"`

	// Message is the detailed alert message
	Message string `json:"message"`

	// Run associated with this alert (if applicable)
	Run *domain.BenchmarkRun `json:"run,omitempty"`

	// Anomaly details (if type is "anomaly")
	Anomaly *analysis.Anomaly `json:"anomaly,omitempty"`

	// Comparison details (if type is "baseline_deviation")
	Comparison *analysis.RunComparison `json:"comparison,omitempty"`

	// Timestamp when the alert was created
	Timestamp time.Time `json:"timestamp"`

	// Tags for categorization
	Tags map[string]string `json:"tags,omitempty"`

	// Source identifies where the alert originated
	Source string `json:"source"`
}

// Notifier sends alerts to external systems.
type Notifier interface {
	// Name returns the notifier name
	Name() string

	// Send sends an alert
	Send(ctx context.Context, alert *Alert) error

	// SendBatch sends multiple alerts
	SendBatch(ctx context.Context, alerts []*Alert) error

	// HealthCheck verifies the notifier is working
	HealthCheck(ctx context.Context) error
}

// NotifierConfig contains common notifier configuration.
type NotifierConfig struct {
	// Enabled determines if the notifier is active
	Enabled bool `json:"enabled"`

	// MinSeverity is the minimum severity to notify (info, warning, critical)
	MinSeverity string `json:"min_severity"`

	// RateLimit limits notifications per hour
	RateLimit int `json:"rate_limit"`

	// RetryAttempts for failed notifications
	RetryAttempts int `json:"retry_attempts"`

	// RetryDelay between retry attempts
	RetryDelay time.Duration `json:"retry_delay"`
}

// DefaultNotifierConfig returns sensible defaults.
func DefaultNotifierConfig() NotifierConfig {
	return NotifierConfig{
		Enabled:       true,
		MinSeverity:   "warning",
		RateLimit:     100,
		RetryAttempts: 3,
		RetryDelay:    5 * time.Second,
	}
}

// NotifierRegistry manages available notifiers.
type NotifierRegistry struct {
	notifiers map[string]Notifier
}

// NewNotifierRegistry creates a new notifier registry.
func NewNotifierRegistry() *NotifierRegistry {
	return &NotifierRegistry{
		notifiers: make(map[string]Notifier),
	}
}

// Register adds a notifier to the registry.
func (r *NotifierRegistry) Register(notifier Notifier) {
	r.notifiers[notifier.Name()] = notifier
}

// Get retrieves a notifier by name.
func (r *NotifierRegistry) Get(name string) (Notifier, bool) {
	n, ok := r.notifiers[name]
	return n, ok
}

// List returns all registered notifier names.
func (r *NotifierRegistry) List() []string {
	names := make([]string, 0, len(r.notifiers))
	for name := range r.notifiers {
		names = append(names, name)
	}
	return names
}

// All returns all registered notifiers.
func (r *NotifierRegistry) All() []Notifier {
	notifiers := make([]Notifier, 0, len(r.notifiers))
	for _, n := range r.notifiers {
		notifiers = append(notifiers, n)
	}
	return notifiers
}

// DefaultRegistry is the global notifier registry.
var DefaultRegistry = NewNotifierRegistry()

// AlertManager coordinates alert delivery across notifiers.
type AlertManager struct {
	registry *NotifierRegistry
	config   AlertManagerConfig
}

// AlertManagerConfig configures the alert manager.
type AlertManagerConfig struct {
	// DefaultNotifiers to use if none specified
	DefaultNotifiers []string `json:"default_notifiers"`

	// GroupByRun groups alerts by run ID before sending
	GroupByRun bool `json:"group_by_run"`

	// Deduplicate prevents sending duplicate alerts
	Deduplicate bool `json:"deduplicate"`

	// DeduplicationWindow is the time window for deduplication
	DeduplicationWindow time.Duration `json:"deduplication_window"`
}

// NewAlertManager creates a new alert manager.
func NewAlertManager(registry *NotifierRegistry, config AlertManagerConfig) *AlertManager {
	if registry == nil {
		registry = DefaultRegistry
	}
	return &AlertManager{
		registry: registry,
		config:   config,
	}
}

// SendAlert sends an alert through configured notifiers.
func (m *AlertManager) SendAlert(ctx context.Context, alert *Alert, notifierNames ...string) error {
	if alert.Timestamp.IsZero() {
		alert.Timestamp = time.Now()
	}

	if len(notifierNames) == 0 {
		notifierNames = m.config.DefaultNotifiers
	}

	if len(notifierNames) == 0 {
		notifierNames = m.registry.List()
	}

	var lastErr error
	for _, name := range notifierNames {
		notifier, ok := m.registry.Get(name)
		if !ok {
			continue
		}
		if err := notifier.Send(ctx, alert); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// SendAlerts sends multiple alerts through configured notifiers.
func (m *AlertManager) SendAlerts(ctx context.Context, alerts []*Alert, notifierNames ...string) error {
	if len(notifierNames) == 0 {
		notifierNames = m.config.DefaultNotifiers
	}

	if len(notifierNames) == 0 {
		notifierNames = m.registry.List()
	}

	var lastErr error
	for _, name := range notifierNames {
		notifier, ok := m.registry.Get(name)
		if !ok {
			continue
		}
		if err := notifier.SendBatch(ctx, alerts); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// AlertRule defines when to create an alert.
type AlertRule struct {
	// Name of the rule
	Name string `json:"name"`

	// Description of what the rule checks
	Description string `json:"description"`

	// Type: "threshold", "baseline_deviation", "anomaly", "custom"
	Type string `json:"type"`

	// Enabled determines if the rule is active
	Enabled bool `json:"enabled"`

	// Severity when rule triggers
	Severity string `json:"severity"`

	// Conditions for the rule
	Conditions []RuleCondition `json:"conditions"`

	// Actions when rule triggers
	Actions []RuleAction `json:"actions"`

	// Tags to add to generated alerts
	Tags map[string]string `json:"tags,omitempty"`
}

// RuleCondition defines a condition for an alert rule.
type RuleCondition struct {
	// Metric to check
	Metric string `json:"metric"`

	// Operator: "gt", "lt", "gte", "lte", "eq", "neq"
	Operator string `json:"operator"`

	// Value to compare against
	Value float64 `json:"value"`

	// ValueType: "absolute", "percentage", "baseline_deviation"
	ValueType string `json:"value_type"`
}

// RuleAction defines what to do when a rule triggers.
type RuleAction struct {
	// Type: "notify", "webhook", "log"
	Type string `json:"type"`

	// Notifiers to use (for notify action)
	Notifiers []string `json:"notifiers,omitempty"`

	// WebhookURL (for webhook action)
	WebhookURL string `json:"webhook_url,omitempty"`

	// Template for message customization
	Template string `json:"template,omitempty"`
}

// RuleEngine evaluates alert rules against benchmark runs.
type RuleEngine struct {
	rules   []AlertRule
	manager *AlertManager
}

// NewRuleEngine creates a new rule engine.
func NewRuleEngine(manager *AlertManager) *RuleEngine {
	return &RuleEngine{
		rules:   []AlertRule{},
		manager: manager,
	}
}

// AddRule adds a rule to the engine.
func (e *RuleEngine) AddRule(rule AlertRule) {
	e.rules = append(e.rules, rule)
}

// RemoveRule removes a rule by name.
func (e *RuleEngine) RemoveRule(name string) {
	newRules := make([]AlertRule, 0, len(e.rules))
	for _, r := range e.rules {
		if r.Name != name {
			newRules = append(newRules, r)
		}
	}
	e.rules = newRules
}

// ListRules returns all rules.
func (e *RuleEngine) ListRules() []AlertRule {
	return e.rules
}

// Evaluate evaluates all rules against a benchmark run.
func (e *RuleEngine) Evaluate(ctx context.Context, run *domain.BenchmarkRun) ([]*Alert, error) {
	var alerts []*Alert

	for _, rule := range e.rules {
		if !rule.Enabled {
			continue
		}

		triggered, err := e.evaluateRule(ctx, rule, run)
		if err != nil {
			continue
		}

		if triggered {
			alert := &Alert{
				ID:        generateAlertID(),
				Type:      rule.Type,
				Severity:  rule.Severity,
				Title:     rule.Name,
				Message:   rule.Description,
				Run:       run,
				Timestamp: time.Now(),
				Tags:      rule.Tags,
				Source:    "rule_engine",
			}
			alerts = append(alerts, alert)

			// Execute actions
			for _, action := range rule.Actions {
				if err := e.executeAction(ctx, action, alert); err != nil {
					// Log but continue
				}
			}
		}
	}

	return alerts, nil
}

func (e *RuleEngine) evaluateRule(ctx context.Context, rule AlertRule, run *domain.BenchmarkRun) (bool, error) {
	if run.Results == nil || run.Results.Summary == nil {
		return false, nil
	}

	for _, cond := range rule.Conditions {
		value := e.getMetricValue(run, cond.Metric)
		if !e.evaluateCondition(cond, value) {
			return false, nil
		}
	}

	return len(rule.Conditions) > 0, nil
}

func (e *RuleEngine) getMetricValue(run *domain.BenchmarkRun, metric string) float64 {
	if run.Results == nil || run.Results.Summary == nil {
		return 0
	}

	switch metric {
	case "throughput", "ops_per_second":
		return run.Results.Summary.OpsPerSecond
	case "avg_latency", "avg_latency_ms":
		return run.Results.Summary.AvgLatencyMs
	case "p50_latency", "p50_latency_ms":
		return run.Results.Summary.P50LatencyMs
	case "p90_latency", "p90_latency_ms":
		return run.Results.Summary.P90LatencyMs
	case "p95_latency", "p95_latency_ms":
		return run.Results.Summary.P95LatencyMs
	case "p99_latency", "p99_latency_ms":
		return run.Results.Summary.P99LatencyMs
	case "error_rate":
		return run.Results.Summary.ErrorRate
	case "errors":
		return float64(run.Results.Summary.Errors)
	default:
		return 0
	}
}

func (e *RuleEngine) evaluateCondition(cond RuleCondition, value float64) bool {
	switch cond.Operator {
	case "gt":
		return value > cond.Value
	case "lt":
		return value < cond.Value
	case "gte":
		return value >= cond.Value
	case "lte":
		return value <= cond.Value
	case "eq":
		return value == cond.Value
	case "neq":
		return value != cond.Value
	default:
		return false
	}
}

func (e *RuleEngine) executeAction(ctx context.Context, action RuleAction, alert *Alert) error {
	switch action.Type {
	case "notify":
		return e.manager.SendAlert(ctx, alert, action.Notifiers...)
	case "webhook":
		// Webhook execution handled separately
		return nil
	case "log":
		// Logging handled by caller
		return nil
	default:
		return nil
	}
}

var alertCounter int64

func generateAlertID() string {
	alertCounter++
	return "alert-" + time.Now().Format("20060102150405") + "-" + intToStr(alertCounter)
}

func intToStr(n int64) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n = n / 10
	}
	return s
}

// MarshalJSON implements json.Marshaler for Alert.
func (a *Alert) MarshalJSON() ([]byte, error) {
	type Alias Alert
	return json.Marshal(&struct {
		*Alias
		TimestampStr string `json:"timestamp_str"`
	}{
		Alias:        (*Alias)(a),
		TimestampStr: a.Timestamp.Format(time.RFC3339),
	})
}

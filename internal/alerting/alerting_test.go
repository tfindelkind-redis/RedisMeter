package alerting

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/analysis"
	"github.com/tfindelkind-redis/redismeter/internal/domain"
)

func TestAlert_MarshalJSON(t *testing.T) {
	alert := &Alert{
		ID:        "test-alert-1",
		Type:      "anomaly",
		Severity:  "warning",
		Title:     "Test Alert",
		Message:   "Test message",
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Source:    "test",
	}

	data, err := alert.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("Expected non-empty JSON")
	}
}

func TestNotifierRegistry(t *testing.T) {
	registry := NewNotifierRegistry()

	// Register a console notifier
	config := DefaultNotifierConfig()
	notifier := NewConsoleNotifier(config, &bytes.Buffer{})
	registry.Register(notifier)

	// Get should work
	n, ok := registry.Get("console")
	if !ok {
		t.Error("Expected to find console notifier")
	}
	if n.Name() != "console" {
		t.Errorf("Expected name 'console', got %s", n.Name())
	}

	// List should include console
	names := registry.List()
	found := false
	for _, name := range names {
		if name == "console" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'console' in list")
	}

	// All should return the notifier
	all := registry.All()
	if len(all) != 1 {
		t.Errorf("Expected 1 notifier, got %d", len(all))
	}
}

func TestAlertManager_SendAlert(t *testing.T) {
	registry := NewNotifierRegistry()
	buf := &bytes.Buffer{}
	config := DefaultNotifierConfig()
	notifier := NewConsoleNotifier(config, buf)
	registry.Register(notifier)

	manager := NewAlertManager(registry, AlertManagerConfig{
		DefaultNotifiers: []string{"console"},
	})

	alert := &Alert{
		ID:        "test-1",
		Type:      "test",
		Severity:  "info",
		Title:     "Test Alert",
		Message:   "This is a test",
		Timestamp: time.Now(),
		Source:    "test",
	}

	err := manager.SendAlert(context.Background(), alert)
	if err != nil {
		t.Errorf("SendAlert failed: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("Expected output from console notifier")
	}
}

func TestConsoleNotifier(t *testing.T) {
	buf := &bytes.Buffer{}
	config := DefaultNotifierConfig()
	notifier := NewConsoleNotifier(config, buf)

	if notifier.Name() != "console" {
		t.Errorf("Expected name 'console', got %s", notifier.Name())
	}

	// HealthCheck should pass
	err := notifier.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("HealthCheck failed: %v", err)
	}

	// Send alert
	alert := &Alert{
		ID:        "test-1",
		Type:      "anomaly",
		Severity:  "warning",
		Title:     "Test Warning",
		Message:   "Something happened",
		Timestamp: time.Now(),
		Source:    "test",
		Anomaly: &analysis.Anomaly{
			Metric:    "throughput",
			Value:     8000,
			Expected:  10000,
			Deviation: -0.2,
		},
	}

	err = notifier.Send(context.Background(), alert)
	if err != nil {
		t.Errorf("Send failed: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("Expected non-empty output")
	}

	// Check output contains expected content
	if !bytes.Contains(buf.Bytes(), []byte("WARNING")) {
		t.Error("Expected output to contain 'WARNING'")
	}
	if !bytes.Contains(buf.Bytes(), []byte("Test Warning")) {
		t.Error("Expected output to contain 'Test Warning'")
	}
}

func TestConsoleNotifier_Disabled(t *testing.T) {
	buf := &bytes.Buffer{}
	config := DefaultNotifierConfig()
	config.Enabled = false
	notifier := NewConsoleNotifier(config, buf)

	alert := &Alert{
		ID:        "test-1",
		Type:      "test",
		Severity:  "info",
		Title:     "Test",
		Message:   "Test",
		Timestamp: time.Now(),
		Source:    "test",
	}

	err := notifier.Send(context.Background(), alert)
	if err != nil {
		t.Errorf("Send failed: %v", err)
	}

	if buf.Len() != 0 {
		t.Error("Expected no output when disabled")
	}
}

func TestSlackNotifier_HealthCheck(t *testing.T) {
	config := DefaultSlackConfig()
	config.WebhookURL = ""
	notifier := NewSlackNotifier(config)

	err := notifier.HealthCheck(context.Background())
	if err == nil {
		t.Error("Expected error for missing webhook URL")
	}

	config.WebhookURL = "https://hooks.slack.com/test"
	notifier = NewSlackNotifier(config)
	err = notifier.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("HealthCheck failed: %v", err)
	}
}

func TestWebhookNotifier_HealthCheck(t *testing.T) {
	config := DefaultWebhookConfig()
	config.URL = ""
	notifier := NewWebhookNotifier(config)

	err := notifier.HealthCheck(context.Background())
	if err == nil {
		t.Error("Expected error for missing URL")
	}

	config.URL = "https://example.com/webhook"
	notifier = NewWebhookNotifier(config)
	err = notifier.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("HealthCheck failed: %v", err)
	}
}

func TestPagerDutyNotifier_HealthCheck(t *testing.T) {
	config := DefaultPagerDutyConfig()
	config.RoutingKey = ""
	notifier := NewPagerDutyNotifier(config)

	err := notifier.HealthCheck(context.Background())
	if err == nil {
		t.Error("Expected error for missing routing key")
	}

	config.RoutingKey = "test-routing-key"
	notifier = NewPagerDutyNotifier(config)
	err = notifier.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("HealthCheck failed: %v", err)
	}
}

func TestRuleEngine(t *testing.T) {
	registry := NewNotifierRegistry()
	buf := &bytes.Buffer{}
	config := DefaultNotifierConfig()
	notifier := NewConsoleNotifier(config, buf)
	registry.Register(notifier)

	manager := NewAlertManager(registry, AlertManagerConfig{})
	engine := NewRuleEngine(manager)

	// Add a rule
	rule := AlertRule{
		Name:        "low_throughput",
		Description: "Throughput below threshold",
		Type:        "threshold",
		Enabled:     true,
		Severity:    "warning",
		Conditions: []RuleCondition{
			{
				Metric:   "throughput",
				Operator: "lt",
				Value:    10000,
			},
		},
		Actions: []RuleAction{
			{
				Type:      "notify",
				Notifiers: []string{"console"},
			},
		},
	}
	engine.AddRule(rule)

	rules := engine.ListRules()
	if len(rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(rules))
	}

	// Create a run with low throughput
	run := &domain.BenchmarkRun{
		ID: "test-run-1",
		Results: &domain.Results{
			Summary: &domain.SummaryMetrics{
				OpsPerSecond: 5000, // Below threshold
			},
		},
	}

	alerts, err := engine.Evaluate(context.Background(), run)
	if err != nil {
		t.Errorf("Evaluate failed: %v", err)
	}

	if len(alerts) != 1 {
		t.Errorf("Expected 1 alert, got %d", len(alerts))
	}

	// Test removing rule
	engine.RemoveRule("low_throughput")
	rules = engine.ListRules()
	if len(rules) != 0 {
		t.Errorf("Expected 0 rules after removal, got %d", len(rules))
	}
}

func TestRuleEngine_PassingCondition(t *testing.T) {
	manager := NewAlertManager(NewNotifierRegistry(), AlertManagerConfig{})
	engine := NewRuleEngine(manager)

	rule := AlertRule{
		Name:        "low_throughput",
		Description: "Throughput below threshold",
		Type:        "threshold",
		Enabled:     true,
		Severity:    "warning",
		Conditions: []RuleCondition{
			{
				Metric:   "throughput",
				Operator: "lt",
				Value:    10000,
			},
		},
	}
	engine.AddRule(rule)

	// Create a run with sufficient throughput
	run := &domain.BenchmarkRun{
		ID: "test-run-1",
		Results: &domain.Results{
			Summary: &domain.SummaryMetrics{
				OpsPerSecond: 15000, // Above threshold
			},
		},
	}

	alerts, err := engine.Evaluate(context.Background(), run)
	if err != nil {
		t.Errorf("Evaluate failed: %v", err)
	}

	if len(alerts) != 0 {
		t.Errorf("Expected 0 alerts for passing condition, got %d", len(alerts))
	}
}

func TestRuleEngine_DisabledRule(t *testing.T) {
	manager := NewAlertManager(NewNotifierRegistry(), AlertManagerConfig{})
	engine := NewRuleEngine(manager)

	rule := AlertRule{
		Name:        "disabled_rule",
		Description: "This rule is disabled",
		Type:        "threshold",
		Enabled:     false,
		Severity:    "warning",
		Conditions: []RuleCondition{
			{
				Metric:   "throughput",
				Operator: "lt",
				Value:    10000,
			},
		},
	}
	engine.AddRule(rule)

	run := &domain.BenchmarkRun{
		ID: "test-run-1",
		Results: &domain.Results{
			Summary: &domain.SummaryMetrics{
				OpsPerSecond: 5000, // Would trigger if enabled
			},
		},
	}

	alerts, err := engine.Evaluate(context.Background(), run)
	if err != nil {
		t.Errorf("Evaluate failed: %v", err)
	}

	if len(alerts) != 0 {
		t.Errorf("Expected 0 alerts for disabled rule, got %d", len(alerts))
	}
}

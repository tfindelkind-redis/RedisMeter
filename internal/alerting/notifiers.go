package alerting

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SlackNotifier sends alerts to Slack.
type SlackNotifier struct {
	config      SlackConfig
	client      *http.Client
	rateLimiter *rateLimiter
}

// SlackConfig configures the Slack notifier.
type SlackConfig struct {
	NotifierConfig

	// WebhookURL is the Slack webhook URL
	WebhookURL string `json:"webhook_url"`

	// Channel to post to (optional, uses webhook default)
	Channel string `json:"channel"`

	// Username to display as
	Username string `json:"username"`

	// IconEmoji to use
	IconEmoji string `json:"icon_emoji"`

	// IncludeDetails adds extra alert details
	IncludeDetails bool `json:"include_details"`
}

// DefaultSlackConfig returns sensible defaults.
func DefaultSlackConfig() SlackConfig {
	return SlackConfig{
		NotifierConfig: DefaultNotifierConfig(),
		Username:       "RedisMeter",
		IconEmoji:      ":chart_with_upwards_trend:",
		IncludeDetails: true,
	}
}

// NewSlackNotifier creates a new Slack notifier.
func NewSlackNotifier(config SlackConfig) *SlackNotifier {
	return &SlackNotifier{
		config: config,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		rateLimiter: newRateLimiter(config.RateLimit),
	}
}

// Name returns the notifier name.
func (n *SlackNotifier) Name() string {
	return "slack"
}

// Send sends an alert to Slack.
func (n *SlackNotifier) Send(ctx context.Context, alert *Alert) error {
	if !n.config.Enabled {
		return nil
	}

	if !n.meetsMinSeverity(alert.Severity) {
		return nil
	}

	if !n.rateLimiter.allow() {
		return fmt.Errorf("rate limit exceeded")
	}

	payload := n.buildPayload(alert)
	return n.sendWithRetry(ctx, payload)
}

// SendBatch sends multiple alerts to Slack.
func (n *SlackNotifier) SendBatch(ctx context.Context, alerts []*Alert) error {
	for _, alert := range alerts {
		if err := n.Send(ctx, alert); err != nil {
			// Continue sending other alerts
			continue
		}
	}
	return nil
}

// HealthCheck verifies Slack connectivity.
func (n *SlackNotifier) HealthCheck(ctx context.Context) error {
	if n.config.WebhookURL == "" {
		return fmt.Errorf("webhook URL not configured")
	}
	return nil
}

func (n *SlackNotifier) buildPayload(alert *Alert) map[string]interface{} {
	color := n.severityColor(alert.Severity)

	attachment := map[string]interface{}{
		"color":  color,
		"title":  alert.Title,
		"text":   alert.Message,
		"ts":     alert.Timestamp.Unix(),
		"footer": "RedisMeter Alert",
		"fields": []map[string]interface{}{
			{
				"title": "Severity",
				"value": alert.Severity,
				"short": true,
			},
			{
				"title": "Type",
				"value": alert.Type,
				"short": true,
			},
		},
	}

	if n.config.IncludeDetails {
		if alert.Run != nil {
			workloadName := ""
			if alert.Run.Workload != nil {
				workloadName = alert.Run.Workload.Name
			}
			attachment["fields"] = append(attachment["fields"].([]map[string]interface{}),
				map[string]interface{}{
					"title": "Workload",
					"value": workloadName,
					"short": true,
				},
				map[string]interface{}{
					"title": "Run ID",
					"value": alert.Run.ID,
					"short": true,
				},
			)
		}

		if alert.Anomaly != nil {
			attachment["fields"] = append(attachment["fields"].([]map[string]interface{}),
				map[string]interface{}{
					"title": "Metric",
					"value": alert.Anomaly.Metric,
					"short": true,
				},
				map[string]interface{}{
					"title": "Deviation",
					"value": fmt.Sprintf("%.2f%%", alert.Anomaly.Deviation*100),
					"short": true,
				},
			)
		}
	}

	payload := map[string]interface{}{
		"attachments": []map[string]interface{}{attachment},
	}

	if n.config.Channel != "" {
		payload["channel"] = n.config.Channel
	}
	if n.config.Username != "" {
		payload["username"] = n.config.Username
	}
	if n.config.IconEmoji != "" {
		payload["icon_emoji"] = n.config.IconEmoji
	}

	return payload
}

func (n *SlackNotifier) sendWithRetry(ctx context.Context, payload map[string]interface{}) error {
	var lastErr error
	for attempt := 0; attempt <= n.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(n.config.RetryDelay):
			}
		}

		if err := n.send(ctx, payload); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return lastErr
}

func (n *SlackNotifier) send(ctx context.Context, payload map[string]interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.config.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slack returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (n *SlackNotifier) severityColor(severity string) string {
	switch severity {
	case "critical":
		return "danger"
	case "warning":
		return "warning"
	default:
		return "good"
	}
}

func (n *SlackNotifier) meetsMinSeverity(severity string) bool {
	severityLevel := map[string]int{
		"info":     0,
		"warning":  1,
		"critical": 2,
	}
	return severityLevel[severity] >= severityLevel[n.config.MinSeverity]
}

// WebhookNotifier sends alerts to generic webhooks.
type WebhookNotifier struct {
	config      WebhookConfig
	client      *http.Client
	rateLimiter *rateLimiter
}

// WebhookConfig configures the webhook notifier.
type WebhookConfig struct {
	NotifierConfig

	// URL is the webhook endpoint
	URL string `json:"url"`

	// Method is the HTTP method (POST, PUT)
	Method string `json:"method"`

	// Headers to include
	Headers map[string]string `json:"headers"`

	// AuthType: "none", "basic", "bearer"
	AuthType string `json:"auth_type"`

	// AuthToken for bearer auth
	AuthToken string `json:"auth_token"`

	// Username for basic auth
	Username string `json:"username"`

	// Password for basic auth
	Password string `json:"password"`
}

// DefaultWebhookConfig returns sensible defaults.
func DefaultWebhookConfig() WebhookConfig {
	return WebhookConfig{
		NotifierConfig: DefaultNotifierConfig(),
		Method:         http.MethodPost,
		AuthType:       "none",
		Headers:        map[string]string{"Content-Type": "application/json"},
	}
}

// NewWebhookNotifier creates a new webhook notifier.
func NewWebhookNotifier(config WebhookConfig) *WebhookNotifier {
	return &WebhookNotifier{
		config: config,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		rateLimiter: newRateLimiter(config.RateLimit),
	}
}

// Name returns the notifier name.
func (n *WebhookNotifier) Name() string {
	return "webhook"
}

// Send sends an alert via webhook.
func (n *WebhookNotifier) Send(ctx context.Context, alert *Alert) error {
	if !n.config.Enabled {
		return nil
	}

	if !n.meetsMinSeverity(alert.Severity) {
		return nil
	}

	if !n.rateLimiter.allow() {
		return fmt.Errorf("rate limit exceeded")
	}

	return n.sendWithRetry(ctx, alert)
}

// SendBatch sends multiple alerts via webhook.
func (n *WebhookNotifier) SendBatch(ctx context.Context, alerts []*Alert) error {
	for _, alert := range alerts {
		if err := n.Send(ctx, alert); err != nil {
			continue
		}
	}
	return nil
}

// HealthCheck verifies webhook connectivity.
func (n *WebhookNotifier) HealthCheck(ctx context.Context) error {
	if n.config.URL == "" {
		return fmt.Errorf("webhook URL not configured")
	}
	return nil
}

func (n *WebhookNotifier) sendWithRetry(ctx context.Context, alert *Alert) error {
	var lastErr error
	for attempt := 0; attempt <= n.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(n.config.RetryDelay):
			}
		}

		if err := n.send(ctx, alert); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return lastErr
}

func (n *WebhookNotifier) send(ctx context.Context, alert *Alert) error {
	body, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("marshal alert: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, n.config.Method, n.config.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	for key, value := range n.config.Headers {
		req.Header.Set(key, value)
	}

	switch n.config.AuthType {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+n.config.AuthToken)
	case "basic":
		req.SetBasicAuth(n.config.Username, n.config.Password)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (n *WebhookNotifier) meetsMinSeverity(severity string) bool {
	severityLevel := map[string]int{
		"info":     0,
		"warning":  1,
		"critical": 2,
	}
	return severityLevel[severity] >= severityLevel[n.config.MinSeverity]
}

// PagerDutyNotifier sends alerts to PagerDuty.
type PagerDutyNotifier struct {
	config      PagerDutyConfig
	client      *http.Client
	rateLimiter *rateLimiter
}

// PagerDutyConfig configures the PagerDuty notifier.
type PagerDutyConfig struct {
	NotifierConfig

	// RoutingKey is the PagerDuty integration key
	RoutingKey string `json:"routing_key"`

	// ServiceKey (deprecated, use RoutingKey)
	ServiceKey string `json:"service_key"`

	// DedupKey function
	DedupKeyPrefix string `json:"dedup_key_prefix"`
}

// DefaultPagerDutyConfig returns sensible defaults.
func DefaultPagerDutyConfig() PagerDutyConfig {
	return PagerDutyConfig{
		NotifierConfig: DefaultNotifierConfig(),
		DedupKeyPrefix: "redismeter",
	}
}

// NewPagerDutyNotifier creates a new PagerDuty notifier.
func NewPagerDutyNotifier(config PagerDutyConfig) *PagerDutyNotifier {
	return &PagerDutyNotifier{
		config: config,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		rateLimiter: newRateLimiter(config.RateLimit),
	}
}

// Name returns the notifier name.
func (n *PagerDutyNotifier) Name() string {
	return "pagerduty"
}

// Send sends an alert to PagerDuty.
func (n *PagerDutyNotifier) Send(ctx context.Context, alert *Alert) error {
	if !n.config.Enabled {
		return nil
	}

	if !n.meetsMinSeverity(alert.Severity) {
		return nil
	}

	if !n.rateLimiter.allow() {
		return fmt.Errorf("rate limit exceeded")
	}

	return n.sendWithRetry(ctx, alert)
}

// SendBatch sends multiple alerts to PagerDuty.
func (n *PagerDutyNotifier) SendBatch(ctx context.Context, alerts []*Alert) error {
	for _, alert := range alerts {
		if err := n.Send(ctx, alert); err != nil {
			continue
		}
	}
	return nil
}

// HealthCheck verifies PagerDuty connectivity.
func (n *PagerDutyNotifier) HealthCheck(ctx context.Context) error {
	if n.config.RoutingKey == "" && n.config.ServiceKey == "" {
		return fmt.Errorf("routing key not configured")
	}
	return nil
}

func (n *PagerDutyNotifier) sendWithRetry(ctx context.Context, alert *Alert) error {
	var lastErr error
	for attempt := 0; attempt <= n.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(n.config.RetryDelay):
			}
		}

		if err := n.send(ctx, alert); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return lastErr
}

func (n *PagerDutyNotifier) send(ctx context.Context, alert *Alert) error {
	routingKey := n.config.RoutingKey
	if routingKey == "" {
		routingKey = n.config.ServiceKey
	}

	payload := map[string]interface{}{
		"routing_key":  routingKey,
		"event_action": "trigger",
		"dedup_key":    n.config.DedupKeyPrefix + "-" + alert.ID,
		"payload": map[string]interface{}{
			"summary":   alert.Title + ": " + alert.Message,
			"severity":  n.pdSeverity(alert.Severity),
			"source":    alert.Source,
			"timestamp": alert.Timestamp.Format(time.RFC3339),
			"custom_details": map[string]interface{}{
				"type":    alert.Type,
				"alert":   alert,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://events.pagerduty.com/v2/enqueue", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pagerduty returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (n *PagerDutyNotifier) pdSeverity(severity string) string {
	switch severity {
	case "critical":
		return "critical"
	case "warning":
		return "warning"
	default:
		return "info"
	}
}

func (n *PagerDutyNotifier) meetsMinSeverity(severity string) bool {
	severityLevel := map[string]int{
		"info":     0,
		"warning":  1,
		"critical": 2,
	}
	return severityLevel[severity] >= severityLevel[n.config.MinSeverity]
}

// ConsoleNotifier writes alerts to console (for testing/debugging).
type ConsoleNotifier struct {
	config NotifierConfig
	writer io.Writer
}

// NewConsoleNotifier creates a console notifier.
func NewConsoleNotifier(config NotifierConfig, writer io.Writer) *ConsoleNotifier {
	return &ConsoleNotifier{
		config: config,
		writer: writer,
	}
}

// Name returns the notifier name.
func (n *ConsoleNotifier) Name() string {
	return "console"
}

// Send writes an alert to the console.
func (n *ConsoleNotifier) Send(ctx context.Context, alert *Alert) error {
	if !n.config.Enabled {
		return nil
	}

	severityIcon := map[string]string{
		"info":     "ℹ️",
		"warning":  "⚠️",
		"critical": "🚨",
	}

	icon := severityIcon[alert.Severity]
	if icon == "" {
		icon = "📢"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n%s [%s] %s\n", icon, strings.ToUpper(alert.Severity), alert.Title))
	sb.WriteString(fmt.Sprintf("   %s\n", alert.Message))
	sb.WriteString(fmt.Sprintf("   Type: %s | Source: %s | Time: %s\n",
		alert.Type, alert.Source, alert.Timestamp.Format(time.RFC3339)))

	if alert.Run != nil {
		workloadName := ""
		if alert.Run.Workload != nil {
			workloadName = alert.Run.Workload.Name
		}
		sb.WriteString(fmt.Sprintf("   Run: %s | Workload: %s\n",
			alert.Run.ID, workloadName))
	}

	if alert.Anomaly != nil {
		sb.WriteString(fmt.Sprintf("   Anomaly: %s = %.2f (expected: %.2f, deviation: %.1f%%)\n",
			alert.Anomaly.Metric, alert.Anomaly.Value, alert.Anomaly.Expected, alert.Anomaly.Deviation*100))
	}

	_, err := fmt.Fprint(n.writer, sb.String())
	return err
}

// SendBatch writes multiple alerts to console.
func (n *ConsoleNotifier) SendBatch(ctx context.Context, alerts []*Alert) error {
	for _, alert := range alerts {
		if err := n.Send(ctx, alert); err != nil {
			continue
		}
	}
	return nil
}

// HealthCheck always returns nil for console.
func (n *ConsoleNotifier) HealthCheck(ctx context.Context) error {
	return nil
}

// rateLimiter implements a simple token bucket rate limiter.
type rateLimiter struct {
	tokens     int
	maxTokens  int
	lastRefill time.Time
}

func newRateLimiter(maxPerHour int) *rateLimiter {
	return &rateLimiter{
		tokens:     maxPerHour,
		maxTokens:  maxPerHour,
		lastRefill: time.Now(),
	}
}

func (r *rateLimiter) allow() bool {
	// Refill tokens
	now := time.Now()
	elapsed := now.Sub(r.lastRefill)
	tokensToAdd := int(elapsed.Hours() * float64(r.maxTokens))
	if tokensToAdd > 0 {
		r.tokens += tokensToAdd
		if r.tokens > r.maxTokens {
			r.tokens = r.maxTokens
		}
		r.lastRefill = now
	}

	if r.tokens > 0 {
		r.tokens--
		return true
	}
	return false
}

package pluginkit

import (
	"context"
	"testing"
	"time"
)

func TestGetString(t *testing.T) {
	cfg := map[string]interface{}{
		"host": "localhost",
		"port": 5432,
	}

	val := GetString(cfg, "host", "default")
	if val != "localhost" {
		t.Errorf("expected 'localhost', got '%s'", val)
	}

	val = GetString(cfg, "missing", "default")
	if val != "default" {
		t.Errorf("expected 'default', got '%s'", val)
	}

	// Wrong type should return default
	val = GetString(cfg, "port", "default")
	if val != "default" {
		t.Errorf("expected 'default' for wrong type, got '%s'", val)
	}
}

func TestGetInt(t *testing.T) {
	cfg := map[string]interface{}{
		"port":    5432,
		"port64":  int64(6379),
		"portf":   float64(8080),
		"invalid": "not a number",
	}

	val := GetInt(cfg, "port", 0)
	if val != 5432 {
		t.Errorf("expected 5432, got %d", val)
	}

	val = GetInt(cfg, "port64", 0)
	if val != 6379 {
		t.Errorf("expected 6379, got %d", val)
	}

	val = GetInt(cfg, "portf", 0)
	if val != 8080 {
		t.Errorf("expected 8080, got %d", val)
	}

	val = GetInt(cfg, "missing", 9999)
	if val != 9999 {
		t.Errorf("expected 9999, got %d", val)
	}

	val = GetInt(cfg, "invalid", 9999)
	if val != 9999 {
		t.Errorf("expected 9999 for invalid type, got %d", val)
	}
}

func TestGetFloat(t *testing.T) {
	cfg := map[string]interface{}{
		"rate":    0.95,
		"count":   100,
		"count64": int64(200),
	}

	val := GetFloat(cfg, "rate", 0)
	if val != 0.95 {
		t.Errorf("expected 0.95, got %f", val)
	}

	val = GetFloat(cfg, "count", 0)
	if val != 100 {
		t.Errorf("expected 100, got %f", val)
	}

	val = GetFloat(cfg, "missing", 1.5)
	if val != 1.5 {
		t.Errorf("expected 1.5, got %f", val)
	}
}

func TestGetBool(t *testing.T) {
	cfg := map[string]interface{}{
		"enabled":  true,
		"disabled": false,
		"invalid":  "true",
	}

	val := GetBool(cfg, "enabled", false)
	if val != true {
		t.Error("expected true")
	}

	val = GetBool(cfg, "disabled", true)
	if val != false {
		t.Error("expected false")
	}

	val = GetBool(cfg, "missing", true)
	if val != true {
		t.Error("expected true for missing")
	}

	val = GetBool(cfg, "invalid", false)
	if val != false {
		t.Error("expected false for invalid type")
	}
}

func TestGetDuration(t *testing.T) {
	cfg := map[string]interface{}{
		"timeout":  "30s",
		"interval": time.Minute,
		"nanos":    int64(time.Second),
	}

	val := GetDuration(cfg, "timeout", 0)
	if val != 30*time.Second {
		t.Errorf("expected 30s, got %v", val)
	}

	val = GetDuration(cfg, "interval", 0)
	if val != time.Minute {
		t.Errorf("expected 1m, got %v", val)
	}

	val = GetDuration(cfg, "nanos", 0)
	if val != time.Second {
		t.Errorf("expected 1s, got %v", val)
	}

	val = GetDuration(cfg, "missing", 5*time.Second)
	if val != 5*time.Second {
		t.Errorf("expected 5s, got %v", val)
	}
}

func TestGetStringSlice(t *testing.T) {
	cfg := map[string]interface{}{
		"tags":    []string{"a", "b", "c"},
		"mixed":   []interface{}{"x", "y", "z"},
		"invalid": "not a slice",
	}

	val := GetStringSlice(cfg, "tags")
	if len(val) != 3 || val[0] != "a" {
		t.Errorf("expected [a b c], got %v", val)
	}

	val = GetStringSlice(cfg, "mixed")
	if len(val) != 3 || val[0] != "x" {
		t.Errorf("expected [x y z], got %v", val)
	}

	val = GetStringSlice(cfg, "missing")
	if val != nil {
		t.Errorf("expected nil, got %v", val)
	}
}

func TestGetMap(t *testing.T) {
	cfg := map[string]interface{}{
		"nested": map[string]interface{}{
			"key": "value",
		},
	}

	val := GetMap(cfg, "nested")
	if val == nil || val["key"] != "value" {
		t.Errorf("expected map with key=value, got %v", val)
	}

	val = GetMap(cfg, "missing")
	if val != nil {
		t.Errorf("expected nil, got %v", val)
	}
}

func TestBasePlugin(t *testing.T) {
	p := &BasePlugin{}
	p.SetInfo("test-plugin", "1.0.0")

	if p.Name() != "test-plugin" {
		t.Errorf("expected name 'test-plugin', got '%s'", p.Name())
	}
	if p.Version() != "1.0.0" {
		t.Errorf("expected version '1.0.0', got '%s'", p.Version())
	}

	cfg := map[string]interface{}{"key": "value"}
	p.SetConfig(cfg)
	if p.Config()["key"] != "value" {
		t.Error("config not set correctly")
	}

	if p.IsStarted() {
		t.Error("plugin should not be started")
	}

	p.MarkStarted()
	if !p.IsStarted() {
		t.Error("plugin should be started")
	}

	p.MarkStopped()
	if p.IsStarted() {
		t.Error("plugin should be stopped")
	}

	p.SetMetadata("author", "test")
	if p.GetMetadata("author") != "test" {
		t.Error("metadata not set correctly")
	}
}

func TestBaseStoragePlugin(t *testing.T) {
	p := &BaseStoragePlugin{}
	p.SetInfo("storage", "1.0.0")

	if p.Type() != "storage" {
		t.Errorf("expected type 'storage', got '%s'", p.Type())
	}

	err := p.Init(map[string]interface{}{"test": true})
	if err != nil {
		t.Errorf("Init failed: %v", err)
	}

	if !p.IsStarted() {
		t.Error("plugin should be started after Init")
	}

	err = p.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	if p.IsStarted() {
		t.Error("plugin should be stopped after Close")
	}
}

func TestBaseAnalyzerPlugin(t *testing.T) {
	p := &BaseAnalyzerPlugin{}
	p.SetInfo("analyzer", "1.0.0")

	if p.Type() != "analyzer" {
		t.Errorf("expected type 'analyzer', got '%s'", p.Type())
	}
}

func TestLifecycleManager(t *testing.T) {
	mgr := NewLifecycleManager()

	closed := make([]string, 0)
	
	p1 := &mockCloseable{name: "p1", closeFn: func() { closed = append(closed, "p1") }}
	p2 := &mockCloseable{name: "p2", closeFn: func() { closed = append(closed, "p2") }}
	p3 := &mockCloseable{name: "p3", closeFn: func() { closed = append(closed, "p3") }}

	mgr.Register(p1)
	mgr.Register(p2)
	mgr.Register(p3)

	err := mgr.CloseAll()
	if err != nil {
		t.Errorf("CloseAll failed: %v", err)
	}

	// Should be closed in reverse order
	if len(closed) != 3 {
		t.Errorf("expected 3 closed, got %d", len(closed))
	}
	if closed[0] != "p3" || closed[1] != "p2" || closed[2] != "p1" {
		t.Errorf("wrong close order: %v", closed)
	}
}

type mockCloseable struct {
	name    string
	closeFn func()
}

func (m *mockCloseable) Close() error {
	m.closeFn()
	return nil
}

func TestHealthChecker(t *testing.T) {
	checker := NewHealthChecker()

	healthy := &mockHealthCheck{status: HealthStatusHealthy}
	degraded := &mockHealthCheck{status: HealthStatusDegraded}
	unhealthy := &mockHealthCheck{err: context.DeadlineExceeded}

	checker.Register("healthy", healthy)
	checker.Register("degraded", degraded)
	checker.Register("unhealthy", unhealthy)

	results := checker.CheckAll(context.Background())

	if results["healthy"] != HealthStatusHealthy {
		t.Errorf("expected healthy, got %s", results["healthy"])
	}
	if results["degraded"] != HealthStatusDegraded {
		t.Errorf("expected degraded, got %s", results["degraded"])
	}
	if results["unhealthy"] != HealthStatusUnhealthy {
		t.Errorf("expected unhealthy, got %s", results["unhealthy"])
	}
}

type mockHealthCheck struct {
	status HealthStatus
	err    error
}

func (m *mockHealthCheck) HealthCheck(ctx context.Context) (HealthStatus, error) {
	return m.status, m.err
}

func TestMetricsCollector(t *testing.T) {
	metrics := NewMetricsCollector()

	metrics.IncrCounter("requests", 1)
	metrics.IncrCounter("requests", 5)

	if metrics.GetCounter("requests") != 6 {
		t.Errorf("expected 6, got %d", metrics.GetCounter("requests"))
	}

	metrics.SetGauge("connections", 10.5)
	if metrics.GetGauge("connections") != 10.5 {
		t.Errorf("expected 10.5, got %f", metrics.GetGauge("connections"))
	}

	counters, gauges := metrics.Snapshot()
	if counters["requests"] != 6 {
		t.Error("snapshot counters incorrect")
	}
	if gauges["connections"] != 10.5 {
		t.Error("snapshot gauges incorrect")
	}
}

func TestEventEmitter(t *testing.T) {
	emitter := NewEventEmitter()

	received := make(chan Event, 10)

	emitter.On("test", func(e Event) {
		received <- e
	})

	emitter.On("*", func(e Event) {
		received <- e
	})

	emitter.Emit(Event{
		Type:      "test",
		Source:    "test",
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"key": "value"},
	})

	// Should receive 2 events (specific + wildcard)
	time.Sleep(100 * time.Millisecond)

	if len(received) != 2 {
		t.Errorf("expected 2 events, got %d", len(received))
	}
}

func TestConfigValidator(t *testing.T) {
	cfg := map[string]interface{}{
		"host":     "localhost",
		"port":     5432,
		"url":      "https://example.com",
		"bad_url":  "not-a-url",
		"empty":    "",
		"negative": -1,
	}

	v := NewConfigValidator()

	v.RequireString(cfg, "host")
	if v.HasErrors() {
		t.Error("host should be valid")
	}

	v = NewConfigValidator()
	v.RequireString(cfg, "missing")
	if !v.HasErrors() {
		t.Error("missing should be invalid")
	}

	v = NewConfigValidator()
	v.RequireString(cfg, "empty")
	if !v.HasErrors() {
		t.Error("empty should be invalid")
	}

	v = NewConfigValidator()
	v.RequireInt(cfg, "port")
	if v.HasErrors() {
		t.Error("port should be valid")
	}

	v = NewConfigValidator()
	v.RequireInt(cfg, "missing")
	if !v.HasErrors() {
		t.Error("missing int should be invalid")
	}

	v = NewConfigValidator()
	v.RequirePositiveInt(cfg, "port")
	if v.HasErrors() {
		t.Error("port should be positive")
	}

	v = NewConfigValidator()
	v.RequirePositiveInt(cfg, "negative")
	if !v.HasErrors() {
		t.Error("negative should be invalid")
	}

	v = NewConfigValidator()
	v.ValidateURL(cfg, "url")
	if v.HasErrors() {
		t.Error("url should be valid")
	}

	v = NewConfigValidator()
	v.ValidateURL(cfg, "bad_url")
	if !v.HasErrors() {
		t.Error("bad_url should be invalid")
	}
}

func TestValidationError(t *testing.T) {
	err := &ValidationError{
		Field:   "host",
		Message: "required",
	}

	expected := "host: required"
	if err.Error() != expected {
		t.Errorf("expected '%s', got '%s'", expected, err.Error())
	}
}

func TestMockBenchmarkRun(t *testing.T) {
	run := NewMockBenchmarkRun()

	if run.ID == "" {
		t.Error("expected ID to be set")
	}
	if run.Status != "completed" {
		t.Errorf("expected status 'completed', got '%s'", run.Status)
	}
	if run.OpsPerSec <= 0 {
		t.Error("expected positive OpsPerSec")
	}

	m := run.ToMap()
	if m["id"] != run.ID {
		t.Error("ToMap id mismatch")
	}
}

func TestMockBaseline(t *testing.T) {
	baseline := NewMockBaseline()

	if baseline.ID == "" {
		t.Error("expected ID to be set")
	}
	if !baseline.Active {
		t.Error("expected active to be true")
	}
	if len(baseline.Thresholds) == 0 {
		t.Error("expected thresholds to be set")
	}
}

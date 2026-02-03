# RedisMeter Plugin Development Kit

This package provides utilities for developing custom RedisMeter plugins including storage backends, analyzers, workloads, and exporters.

## Installation

```go
import "github.com/redismeter/redismeter/pkg/pluginkit"
```

## Quick Start

### Creating a Storage Plugin

```go
package myplugin

import (
    "context"
    
    "github.com/redismeter/redismeter/internal/domain"
    "github.com/redismeter/redismeter/pkg/pluginkit"
)

type MyStorage struct {
    pluginkit.BaseStoragePlugin
    db *sql.DB
}

func NewMyStorage() *MyStorage {
    s := &MyStorage{}
    s.SetInfo("my-storage", "1.0.0")
    return s
}

func (s *MyStorage) Init(cfg map[string]interface{}) error {
    // Use config helpers
    dsn := pluginkit.GetString(cfg, "dsn", "localhost:5432")
    maxConns := pluginkit.GetInt(cfg, "max_connections", 10)
    timeout := pluginkit.GetDuration(cfg, "timeout", 30*time.Second)
    
    // Validate configuration
    validator := pluginkit.NewConfigValidator()
    validator.RequireString(cfg, "dsn")
    validator.RequirePositiveInt(cfg, "max_connections")
    if err := validator.Error(); err != nil {
        return err
    }
    
    // Initialize database
    var err error
    s.db, err = sql.Open("postgres", dsn)
    if err != nil {
        return err
    }
    s.db.SetMaxOpenConns(maxConns)
    
    s.MarkStarted()
    return nil
}

func (s *MyStorage) Save(ctx context.Context, run *domain.BenchmarkRun) error {
    // Implementation
}

func (s *MyStorage) Load(ctx context.Context, id string) (*domain.BenchmarkRun, error) {
    // Implementation
}

func (s *MyStorage) Close() error {
    s.MarkStopped()
    return s.db.Close()
}
```

### Creating an Analyzer Plugin

```go
package myplugin

import (
    "context"
    
    "github.com/redismeter/redismeter/internal/domain"
    "github.com/redismeter/redismeter/internal/plugin"
    "github.com/redismeter/redismeter/pkg/pluginkit"
)

type MyAnalyzer struct {
    pluginkit.BaseAnalyzerPlugin
    threshold float64
}

func NewMyAnalyzer() *MyAnalyzer {
    a := &MyAnalyzer{}
    a.SetInfo("my-analyzer", "1.0.0")
    return a
}

func (a *MyAnalyzer) Init(cfg map[string]interface{}) error {
    a.threshold = pluginkit.GetFloat(cfg, "threshold", 0.8)
    a.MarkStarted()
    return nil
}

func (a *MyAnalyzer) Analyze(ctx context.Context, run *domain.BenchmarkRun) (*plugin.AnalysisResult, error) {
    score := calculateScore(run)
    
    result := &plugin.AnalysisResult{
        Analyzer: a.Name(),
        Score:    score,
        Findings: []plugin.Finding{},
    }
    
    if score < a.threshold {
        result.Findings = append(result.Findings, plugin.Finding{
            Type:     "performance",
            Severity: "warning",
            Message:  "Performance below threshold",
        })
    }
    
    return result, nil
}
```

## Configuration Helpers

The plugin kit provides type-safe configuration accessors:

```go
// String with default
dsn := pluginkit.GetString(cfg, "dsn", "localhost:5432")

// Integer with default
maxConns := pluginkit.GetInt(cfg, "max_connections", 10)

// Float with default
threshold := pluginkit.GetFloat(cfg, "threshold", 0.95)

// Boolean with default
debug := pluginkit.GetBool(cfg, "debug", false)

// Duration with default
timeout := pluginkit.GetDuration(cfg, "timeout", 30*time.Second)

// String slice
tags := pluginkit.GetStringSlice(cfg, "tags")

// Nested map
nested := pluginkit.GetMap(cfg, "advanced")
```

## Configuration Validation

```go
validator := pluginkit.NewConfigValidator()

// Required fields
validator.RequireString(cfg, "dsn")
validator.RequireInt(cfg, "port")
validator.RequirePositiveInt(cfg, "max_connections")

// Format validation
validator.ValidateURL(cfg, "webhook_url")

// Check for errors
if validator.HasErrors() {
    for _, err := range validator.Errors() {
        log.Printf("Validation error: %s - %s", err.Field, err.Message)
    }
    return validator.Error()
}
```

## Lifecycle Management

```go
// Create a lifecycle manager
lifecycle := pluginkit.NewLifecycleManager()

// Register plugins
lifecycle.Register(storage)
lifecycle.Register(analyzer)
lifecycle.Register(exporter)

// On shutdown, close all plugins in reverse order
defer lifecycle.CloseAll()
```

## Health Checks

Implement the `HealthCheck` interface for plugins that need health monitoring:

```go
func (s *MyStorage) HealthCheck(ctx context.Context) (pluginkit.HealthStatus, error) {
    if err := s.db.PingContext(ctx); err != nil {
        return pluginkit.HealthStatusUnhealthy, err
    }
    return pluginkit.HealthStatusHealthy, nil
}

// Use the health checker
healthChecker := pluginkit.NewHealthChecker()
healthChecker.Register("my-storage", storage)

// Check all plugins
statuses := healthChecker.CheckAll(ctx)
for name, status := range statuses {
    fmt.Printf("%s: %s\n", name, status)
}
```

## Metrics Collection

```go
metrics := pluginkit.NewMetricsCollector()

// Increment counters
metrics.IncrCounter("queries_total", 1)
metrics.IncrCounter("errors_total", 1)

// Set gauges
metrics.SetGauge("connection_pool_size", 10)
metrics.SetGauge("cache_hit_rate", 0.95)

// Get values
queries := metrics.GetCounter("queries_total")
poolSize := metrics.GetGauge("connection_pool_size")

// Get all metrics
counters, gauges := metrics.Snapshot()
```

## Event Emission

```go
emitter := pluginkit.NewEventEmitter()

// Register handlers
emitter.On("run.completed", func(e pluginkit.Event) {
    log.Printf("Run completed: %s", e.Data["run_id"])
})

// Wildcard handler for all events
emitter.On("*", func(e pluginkit.Event) {
    log.Printf("Event: %s from %s", e.Type, e.Source)
})

// Emit events
emitter.Emit(pluginkit.Event{
    Type:      "run.completed",
    Source:    "my-plugin",
    Timestamp: time.Now(),
    Data: map[string]interface{}{
        "run_id": "run-123",
        "status": "success",
    },
})
```

## Testing Plugins

### Storage Plugin Testing

```go
func TestMyStorage(t *testing.T) {
    storage := NewMyStorage()
    err := storage.Init(map[string]interface{}{
        "dsn": "postgres://test:test@localhost/test",
    })
    if err != nil {
        t.Fatal(err)
    }
    
    // Use the storage test suite
    suite := pluginkit.NewStorageTestSuite(t, 
        &storageAdapter{storage},  // Adapter implementing StoragePluginTest
        func() { storage.Close() },
    )
    suite.Run()
}
```

### Analyzer Plugin Testing

```go
func TestMyAnalyzer(t *testing.T) {
    analyzer := NewMyAnalyzer()
    analyzer.Init(nil)
    
    suite := pluginkit.NewAnalyzerTestSuite(t,
        &analyzerAdapter{analyzer},
    )
    suite.Run()
}
```

### Using Mock Data

```go
// Create mock run
run := pluginkit.NewMockBenchmarkRun()
run.OpsPerSec = 150000.0
run.LatencyP99 = 500.0

// Create mock baseline
baseline := pluginkit.NewMockBaseline()
baseline.Metrics["ops_per_sec"] = 100000.0

// Test with mocks
result, err := analyzer.Analyze(ctx, run)
```

### Benchmarking

```go
func BenchmarkMyStorage(b *testing.B) {
    storage := NewMyStorage()
    storage.Init(testConfig)
    defer storage.Close()
    
    helper := pluginkit.NewBenchmarkHelper(b)
    
    run := pluginkit.NewMockBenchmarkRun()
    
    helper.TimeOperation("Save", func() error {
        return storage.Save(context.Background(), run)
    })
    
    helper.TimeOperation("Load", func() error {
        _, err := storage.Load(context.Background(), run.ID)
        return err
    })
}
```

## Plugin Registration

To register your plugin with RedisMeter:

```go
import "github.com/redismeter/redismeter/internal/plugin"

func init() {
    plugin.RegisterStorage("my-storage", func() plugin.StoragePlugin {
        return NewMyStorage()
    })
    
    plugin.RegisterAnalyzer("my-analyzer", func() plugin.AnalyzerPlugin {
        return NewMyAnalyzer()
    })
}
```

## Best Practices

1. **Always validate configuration** - Use `ConfigValidator` to validate required fields
2. **Implement health checks** - Enable monitoring of plugin health
3. **Use lifecycle management** - Ensure proper cleanup on shutdown
4. **Collect metrics** - Expose operational metrics for observability
5. **Write tests** - Use the test suites to validate plugin behavior
6. **Handle context cancellation** - Respect context deadlines and cancellation
7. **Log appropriately** - Use structured logging with appropriate levels

## License

MIT License

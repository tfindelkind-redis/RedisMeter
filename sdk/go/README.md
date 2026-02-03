# RedisMeter Go SDK

Go client library for the RedisMeter benchmarking tool.

## Installation

```bash
go get github.com/redismeter/redismeter/sdk/go/redismeter
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/redismeter/redismeter/sdk/go/redismeter"
)

func main() {
    ctx := context.Background()

    // Create a client
    client := redismeter.NewClient("http://localhost:8080",
        redismeter.WithAPIKey("your-api-key"),
    )

    // Run a benchmark
    run, err := client.RunBenchmark(ctx, &redismeter.RunBenchmarkRequest{
        Target: &redismeter.Target{
            Host: "localhost",
            Port: 6379,
        },
        Workload: "mixed",
        Duration: "30s",
        Clients:  50,
    }, &redismeter.RunBenchmarkOptions{
        Wait: true,
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Run completed: %s\n", run.ID)
    fmt.Printf("Throughput: %.2f ops/sec\n", run.Results.Summary.OpsPerSec)
    fmt.Printf("P99 Latency: %.2f µs\n", run.Results.Summary.LatencyP99Us)

    // Compare to baseline
    comparison, err := client.CompareToBaseline(ctx, run.ID, "")
    if err != nil {
        log.Fatal(err)
    }

    if comparison.RegressionDetected {
        fmt.Println("⚠️  Regression detected!")
        for _, reg := range comparison.Regressions {
            fmt.Printf("  %s: %.1f%%\n", reg.Metric, reg.ChangePercent)
        }
    }
}
```

## Client Options

```go
// With API key
client := redismeter.NewClient(baseURL, redismeter.WithAPIKey("key"))

// With custom timeout
client := redismeter.NewClient(baseURL, redismeter.WithTimeout(60*time.Second))

// With custom HTTP client
httpClient := &http.Client{
    Transport: &http.Transport{
        MaxIdleConns: 100,
    },
}
client := redismeter.NewClient(baseURL, redismeter.WithHTTPClient(httpClient))

// With custom headers
client := redismeter.NewClient(baseURL, redismeter.WithHeaders(map[string]string{
    "X-Custom-Header": "value",
}))
```

## API Reference

### Runs

```go
// List runs
runs, err := client.ListRuns(ctx, &redismeter.ListRunsOptions{
    Limit:    20,
    Status:   "completed",
    Workload: "mixed",
    Tags:     []string{"ci"},
})

// Get a specific run
run, err := client.GetRun(ctx, "run-id")

// Delete a run
err := client.DeleteRun(ctx, "run-id")

// Run a benchmark
run, err := client.RunBenchmark(ctx, &redismeter.RunBenchmarkRequest{
    Target: &redismeter.Target{
        Host:     "localhost",
        Port:     6379,
        Password: "secret",
        TLS:      true,
    },
    Workload:    "mixed",
    Duration:    "30s",
    Clients:     50,
    Name:        "CI Benchmark",
    Description: "Automated benchmark from CI",
    Tags:        []string{"ci", "nightly"},
    Labels:      map[string]string{"env": "staging"},
}, &redismeter.RunBenchmarkOptions{
    Wait:         true,
    PollInterval: time.Second,
    Timeout:      5 * time.Minute,
})
```

### Baselines

```go
// List baselines
baselines, err := client.ListBaselines(ctx, &redismeter.ListBaselinesOptions{
    ActiveOnly: true,
})

// Get a baseline
baseline, err := client.GetBaseline(ctx, "baseline-id")

// Create a baseline
baseline, err := client.CreateBaseline(ctx, &redismeter.CreateBaselineRequest{
    Name:        "v1.0 baseline",
    RunID:       "run-id",
    Description: "Baseline for version 1.0",
    Thresholds: map[string]float64{
        "latency_p99": 0.10,
        "throughput":  0.05,
    },
    Tags: []string{"release"},
})

// Delete a baseline
err := client.DeleteBaseline(ctx, "baseline-id")

// Set active baseline
baseline, err := client.SetActiveBaseline(ctx, "baseline-id")
```

### Comparison & Analysis

```go
// Compare two runs
result, err := client.CompareRuns(ctx, "run1-id", "run2-id")

// Compare to baseline
result, err := client.CompareToBaseline(ctx, "run-id", "baseline-id")
// Or use active baseline:
result, err := client.CompareToBaseline(ctx, "run-id", "")

// Analyze a run
results, err := client.AnalyzeRun(ctx, "run-id", []string{"latency", "throughput"})
```

### Error Handling

```go
run, err := client.GetRun(ctx, "run-id")
if err != nil {
    if redismeter.IsNotFound(err) {
        fmt.Println("Run not found")
    } else if redismeter.IsAuthError(err) {
        fmt.Println("Authentication failed")
    } else if redismeter.IsRateLimitError(err) {
        fmt.Println("Rate limited, try again later")
    } else {
        // Other error
        if apiErr, ok := err.(*redismeter.Error); ok {
            fmt.Printf("API error: %s (code=%s)\n", apiErr.Message, apiErr.Code)
        }
    }
}
```

## CI/CD Integration

### GitHub Actions

```yaml
- name: Run Benchmark
  run: |
    go run ./benchmark/main.go
  env:
    REDISMETER_URL: ${{ secrets.REDISMETER_URL }}
    REDISMETER_KEY: ${{ secrets.REDISMETER_KEY }}
```

```go
// benchmark/main.go
package main

import (
    "context"
    "log"
    "os"

    "github.com/redismeter/redismeter/sdk/go/redismeter"
)

func main() {
    client := redismeter.NewClient(
        os.Getenv("REDISMETER_URL"),
        redismeter.WithAPIKey(os.Getenv("REDISMETER_KEY")),
    )

    run, err := client.RunBenchmark(context.Background(), 
        &redismeter.RunBenchmarkRequest{
            Target:   &redismeter.Target{Host: "localhost", Port: 6379},
            Workload: "mixed",
            Duration: "30s",
        },
        &redismeter.RunBenchmarkOptions{Wait: true},
    )
    if err != nil {
        log.Fatal(err)
    }

    result, err := client.CompareToBaseline(context.Background(), run.ID, "")
    if err != nil {
        log.Fatal(err)
    }

    if result.RegressionDetected {
        log.Fatal("Regression detected!")
    }
}
```

## License

MIT License

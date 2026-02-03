package memtier

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFixMemtierJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "removes trailing comma in object",
			input:    `{"key": "value",}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "removes trailing comma in array",
			input:    `["a", "b",]`,
			expected: `["a", "b"]`,
		},
		{
			name:     "removes multiple trailing commas",
			input:    `{"arr": [1, 2,], "obj": {"a": 1,},}`,
			expected: `{"arr": [1, 2], "obj": {"a": 1}}`,
		},
		{
			name:     "handles whitespace before closing bracket",
			input:    `{"key": "value" , }`,
			expected: `{"key": "value"  }`,
		},
		{
			name:     "valid JSON unchanged",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fixMemtierJSON(tt.input)
			if result != tt.expected {
				t.Errorf("fixMemtierJSON() = %q, want %q", result, tt.expected)
			}

			// Verify result is valid JSON
			var v interface{}
			if err := json.Unmarshal([]byte(result), &v); err != nil {
				t.Errorf("fixMemtierJSON() produced invalid JSON: %v", err)
			}
		})
	}
}

func TestParserParse(t *testing.T) {
	// Sample memtier JSON output
	sampleJSON := `{
		"ALL STATS": {
			"Runtime": {
				"Start time": 1000,
				"Finish time": 2000,
				"Total duration": 1000000000,
				"Time unit": "Microseconds"
			},
			"Sets": {
				"Count": 1000,
				"Ops/sec": 100.5,
				"Hits/sec": 0.0,
				"Misses/sec": 0.0,
				"Latency": 1.234,
				"KB/sec": 50.0
			},
			"Gets": {
				"Count": 4000,
				"Ops/sec": 400.5,
				"Hits/sec": 380.0,
				"Misses/sec": 20.5,
				"Latency": 0.987,
				"KB/sec": 200.0
			}
		}
	}`

	parser := NewParser()
	results, err := parser.Parse([]byte(sampleJSON))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Verify we have operation metrics
	if results.ByOperation == nil {
		t.Fatal("ByOperation should not be nil")
	}

	// Find and verify SET metrics
	setMetrics, ok := results.ByOperation["SET"]
	if !ok {
		t.Error("SET metrics not found")
	} else {
		if setMetrics.OpsPerSecond != 100.5 {
			t.Errorf("SET OpsPerSecond = %f, want 100.5", setMetrics.OpsPerSecond)
		}
	}

	// Find and verify GET metrics
	getMetrics, ok := results.ByOperation["GET"]
	if !ok {
		t.Error("GET metrics not found")
	} else {
		if getMetrics.OpsPerSecond != 400.5 {
			t.Errorf("GET OpsPerSecond = %f, want 400.5", getMetrics.OpsPerSecond)
		}
	}
}

func TestParserParseInvalidJSON(t *testing.T) {
	parser := NewParser()
	_, err := parser.Parse([]byte("not json"))
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestParserParseEmptyStats(t *testing.T) {
	emptyJSON := `{
		"ALL STATS": {
			"Runtime": {
				"Start time": 1000,
				"Finish time": 2000
			}
		}
	}`

	parser := NewParser()
	results, err := parser.Parse([]byte(emptyJSON))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if results == nil {
		t.Error("Results should not be nil")
	}
}

func TestFixMemtierJSONRealWorld(t *testing.T) {
	// This simulates the actual memtier output format with trailing commas
	realWorldJSON := `{
		"ALL STATS": {
			"Sets": {
				"Count": 1000,
				"Ops/sec": 100.0,
			},
			"Gets": {
				"Count": 4000,
				"Ops/sec": 400.0,
			},
		},
	}`

	fixed := fixMemtierJSON(realWorldJSON)

	// Should not contain trailing commas
	if strings.Contains(fixed, ",}") || strings.Contains(fixed, ",]") {
		t.Errorf("Fixed JSON still contains trailing commas: %s", fixed)
	}

	// Should be valid JSON
	var v interface{}
	if err := json.Unmarshal([]byte(fixed), &v); err != nil {
		t.Errorf("Fixed JSON is not valid: %v\nJSON: %s", err, fixed)
	}
}

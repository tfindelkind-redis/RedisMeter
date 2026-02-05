package memtier

import (
	"strings"
	"testing"
)

func TestCommandBuilder(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		wantArgs []string
	}{
		{
			name: "basic command",
			config: &Config{
				Host:     "localhost",
				Port:     6379,
				Threads:  4,
				Clients:  50,
				Requests: 10000,
				Ratio:    "1:4",
				DataSize: 64,
			},
			wantArgs: []string{
				"-s", "localhost",
				"-p", "6379",
				"-t", "4",
				"-c", "50",
				"-n", "10000",
				"--ratio", "1:4",
				"-d", "64",
			},
		},
		{
			name: "with pipeline",
			config: &Config{
				Host:     "localhost",
				Port:     6379,
				Threads:  8,
				Clients:  100,
				Pipeline: 10,
				Ratio:    "1:4",
			},
			wantArgs: []string{
				"--pipeline", "10",
			},
		},
		{
			name: "with authentication",
			config: &Config{
				Host:     "localhost",
				Port:     6379,
				Password: "secret",
				Threads:  1,
				Clients:  1,
				Requests: 100,
				Ratio:    "1:4",
			},
			wantArgs: []string{
				"-a", "secret",
			},
		},
		{
			name: "with TLS",
			config: &Config{
				Host:          "localhost",
				Port:          6379,
				TLS:           true,
				TLSSkipVerify: true,
				Threads:       1,
				Clients:       1,
				Ratio:         "1:4",
			},
			wantArgs: []string{
				"--tls",
				"--tls-skip-verify",
			},
		},
		{
			name: "with key prefix",
			config: &Config{
				Host:      "localhost",
				Port:      6379,
				Threads:   1,
				Clients:   1,
				Ratio:     "1:4",
				KeyPrefix: "test:",
			},
			wantArgs: []string{
				"--key-prefix", "test:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewCommandBuilder(tt.config)
			args := builder.Build()

			cmdStr := strings.Join(args, " ")
			for i := 0; i < len(tt.wantArgs); i += 2 {
				if i+1 < len(tt.wantArgs) {
					// Check flag and value pair
					flag := tt.wantArgs[i]
					value := tt.wantArgs[i+1]
					// Check they appear adjacent
					if !strings.Contains(cmdStr, flag+" "+value) && !strings.Contains(cmdStr, flag+value) {
						t.Errorf("buildCommand() missing %q %q in args: %v", flag, value, args)
					}
				} else {
					// Just check flag exists
					if !strings.Contains(cmdStr, tt.wantArgs[i]) {
						t.Errorf("buildCommand() missing %q in args: %v", tt.wantArgs[i], args)
					}
				}
			}
		})
	}
}

func TestExecutor(t *testing.T) {
	e := NewExecutor()
	if e == nil {
		t.Error("NewExecutor() returned nil")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Host != "localhost" {
		t.Errorf("Host = %q, want %q", cfg.Host, "localhost")
	}
	if cfg.Port != 6379 {
		t.Errorf("Port = %d, want 6379", cfg.Port)
	}
	if cfg.Threads <= 0 {
		t.Errorf("Threads = %d, should be > 0", cfg.Threads)
	}
	if cfg.Clients <= 0 {
		t.Errorf("Clients = %d, should be > 0", cfg.Clients)
	}
	if !cfg.JSONOutput {
		t.Error("JSONOutput should be true by default")
	}
}

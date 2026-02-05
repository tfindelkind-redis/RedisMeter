package memtier

import (
	"strings"
	"testing"
	"time"
)

// Helper function to check if args contain a specific flag-value pair
func containsFlagValue(args []string, flag, value string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}

// Helper function to check if args contain a specific flag (boolean)
func containsFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag {
			return true
		}
	}
	return false
}

// Helper function to get value for a flag
func getFlagValue(args []string, flag string) (string, bool) {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag {
			return args[i+1], true
		}
	}
	return "", false
}

// =============================================================================
// CONNECTION TESTS (15 tests)
// =============================================================================

func TestBuild_Connection_Host_Default(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-s", "localhost") {
		t.Errorf("Expected -s localhost, got %v", args)
	}
}

func TestBuild_Connection_Host_Custom(t *testing.T) {
	cfg := &Config{Host: "redis.example.com", Port: 6379, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-s", "redis.example.com") {
		t.Errorf("Expected -s redis.example.com, got %v", args)
	}
}

func TestBuild_Connection_Host_IP(t *testing.T) {
	cfg := &Config{Host: "192.168.1.100", Port: 6379, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-s", "192.168.1.100") {
		t.Errorf("Expected -s 192.168.1.100, got %v", args)
	}
}

func TestBuild_Connection_Port_Default(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-p", "6379") {
		t.Errorf("Expected -p 6379, got %v", args)
	}
}

func TestBuild_Connection_Port_Custom(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6380, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-p", "6380") {
		t.Errorf("Expected -p 6380, got %v", args)
	}
}

func TestBuild_Connection_Port_TLS(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6443, Ratio: "1:1", KeyPattern: "R", TLS: true}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-p", "6443") {
		t.Errorf("Expected -p 6443, got %v", args)
	}
}

func TestBuild_Connection_Password_Empty(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Password: "", Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if containsFlag(args, "-a") {
		t.Errorf("Should not have -a flag when password is empty")
	}
}

func TestBuild_Connection_Password_Set(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Password: "secretpassword", Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-a", "secretpassword") {
		t.Errorf("Expected -a secretpassword, got %v", args)
	}
}

func TestBuild_Connection_Password_SpecialChars(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Password: "p@ss!word#123", Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-a", "p@ss!word#123") {
		t.Errorf("Expected password with special chars, got %v", args)
	}
}

func TestBuild_Connection_Username_Empty(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Username: "", Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if containsFlag(args, "--user") {
		t.Errorf("Should not have --user flag when username is empty")
	}
}

func TestBuild_Connection_Username_Set(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Username: "admin", Password: "secret", Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--user", "admin") {
		t.Errorf("Expected --user admin, got %v", args)
	}
}

func TestBuild_Connection_TLS_Disabled(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, TLS: false, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if containsFlag(args, "--tls") {
		t.Errorf("Should not have --tls flag when TLS is disabled")
	}
}

func TestBuild_Connection_TLS_Enabled(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, TLS: true, TLSSkipVerify: true, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlag(args, "--tls") {
		t.Errorf("Expected --tls flag, got %v", args)
	}
	if !containsFlag(args, "--tls-skip-verify") {
		t.Errorf("Expected --tls-skip-verify flag, got %v", args)
	}
}

func TestBuild_Connection_Cluster_Disabled(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Cluster: false, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if containsFlag(args, "--cluster-mode") {
		t.Errorf("Should not have --cluster-mode flag when cluster is disabled")
	}
}

func TestBuild_Connection_Cluster_Enabled(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Cluster: true, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlag(args, "--cluster-mode") {
		t.Errorf("Expected --cluster-mode flag, got %v", args)
	}
}

// =============================================================================
// PROTOCOL TESTS (6 tests)
// =============================================================================

func TestBuild_Protocol_Redis_Default(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Protocol: "redis", Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	// redis is default, should not add -P flag
	if containsFlag(args, "-P") {
		t.Errorf("Should not have -P flag for default redis protocol")
	}
}

func TestBuild_Protocol_Redis_Empty(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Protocol: "", Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if containsFlag(args, "-P") {
		t.Errorf("Should not have -P flag when protocol is empty")
	}
}

func TestBuild_Protocol_RESP2(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Protocol: "resp2", Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-P", "resp2") {
		t.Errorf("Expected -P resp2, got %v", args)
	}
}

func TestBuild_Protocol_RESP3(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Protocol: "resp3", Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-P", "resp3") {
		t.Errorf("Expected -P resp3, got %v", args)
	}
}

func TestBuild_Protocol_SelectDB_Zero(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, SelectDB: 0, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if containsFlag(args, "--select-db") {
		t.Errorf("Should not have --select-db flag when DB is 0")
	}
}

func TestBuild_Protocol_SelectDB_Custom(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, SelectDB: 5, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--select-db", "5") {
		t.Errorf("Expected --select-db 5, got %v", args)
	}
}

// =============================================================================
// KEY PATTERN TESTS (20 tests)
// =============================================================================

func TestBuild_KeyPattern_Random(t *testing.T) {
	// Single-letter patterns are auto-converted to X:X format for memtier compatibility
	cfg := &Config{Host: "localhost", Port: 6379, KeyPattern: "R", Ratio: "1:1"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--key-pattern", "R:R") {
		t.Errorf("Expected --key-pattern R:R (auto-converted from R), got %v", args)
	}
}

func TestBuild_KeyPattern_Sequential(t *testing.T) {
	// Single-letter patterns are auto-converted to X:X format for memtier compatibility
	cfg := &Config{Host: "localhost", Port: 6379, KeyPattern: "S", Ratio: "1:1"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--key-pattern", "S:S") {
		t.Errorf("Expected --key-pattern S:S (auto-converted from S), got %v", args)
	}
}

func TestBuild_KeyPattern_Gaussian(t *testing.T) {
	// Single-letter patterns are auto-converted to X:X format for memtier compatibility
	cfg := &Config{Host: "localhost", Port: 6379, KeyPattern: "G", Ratio: "1:1"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--key-pattern", "G:G") {
		t.Errorf("Expected --key-pattern G:G (auto-converted from G), got %v", args)
	}
}

func TestBuild_KeyPattern_Parallel(t *testing.T) {
	// Single-letter patterns are auto-converted to X:X format for memtier compatibility
	cfg := &Config{Host: "localhost", Port: 6379, KeyPattern: "P", Ratio: "1:1"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--key-pattern", "P:P") {
		t.Errorf("Expected --key-pattern P:P (auto-converted from P), got %v", args)
	}
}

func TestBuild_KeyPattern_Zipf(t *testing.T) {
	// Single-letter patterns are auto-converted to X:X format for memtier compatibility
	cfg := &Config{Host: "localhost", Port: 6379, KeyPattern: "Z", Ratio: "1:1"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--key-pattern", "Z:Z") {
		t.Errorf("Expected --key-pattern Z:Z (auto-converted from Z), got %v", args)
	}
}

func TestBuild_KeyPattern_Combined_SG(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, KeyPattern: "S:G", Ratio: "1:1"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--key-pattern", "S:G") {
		t.Errorf("Expected --key-pattern S:G, got %v", args)
	}
}

func TestBuild_KeyPattern_Combined_RR(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, KeyPattern: "R:R", Ratio: "1:1"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--key-pattern", "R:R") {
		t.Errorf("Expected --key-pattern R:R, got %v", args)
	}
}

func TestBuild_KeyRange_Minimum_Zero(t *testing.T) {
	// KeyMinimum 0 is auto-converted to 1 because memtier requires key-minimum > 0
	cfg := &Config{Host: "localhost", Port: 6379, KeyMinimum: 0, KeyMaximum: 1000, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--key-minimum", "1") {
		t.Errorf("Expected --key-minimum 1 (auto-converted from 0), got %v", args)
	}
}

func TestBuild_KeyRange_Minimum_Custom(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, KeyMinimum: 1000, KeyMaximum: 10000, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--key-minimum", "1000") {
		t.Errorf("Expected --key-minimum 1000, got %v", args)
	}
}

func TestBuild_KeyRange_Maximum_Small(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, KeyMinimum: 0, KeyMaximum: 100, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--key-maximum", "100") {
		t.Errorf("Expected --key-maximum 100, got %v", args)
	}
}

func TestBuild_KeyRange_Maximum_Large(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, KeyMinimum: 0, KeyMaximum: 100000000, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--key-maximum", "100000000") {
		t.Errorf("Expected --key-maximum 100000000, got %v", args)
	}
}

func TestBuild_KeyPrefix_Empty(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, KeyPrefix: "", Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if containsFlag(args, "--key-prefix") {
		t.Errorf("Should not have --key-prefix when empty")
	}
}

func TestBuild_KeyPrefix_Simple(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, KeyPrefix: "test:", Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--key-prefix", "test:") {
		t.Errorf("Expected --key-prefix test:, got %v", args)
	}
}

func TestBuild_KeyPrefix_Complex(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, KeyPrefix: "app:v1:user:", Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--key-prefix", "app:v1:user:") {
		t.Errorf("Expected --key-prefix app:v1:user:, got %v", args)
	}
}

func TestBuild_KeyStddev_Zero(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, KeyStddev: 0, Ratio: "1:1", KeyPattern: "G"}
	args := NewCommandBuilder(cfg).Build()
	if containsFlag(args, "--key-stddev") {
		t.Errorf("Should not have --key-stddev when 0")
	}
}

func TestBuild_KeyStddev_Set(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, KeyStddev: 0.1, KeyPattern: "G", Ratio: "1:1"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--key-stddev", "0.1") {
		t.Errorf("Expected --key-stddev 0.1, got %v", args)
	}
}

func TestBuild_KeyMedian_Zero(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, KeyMedian: 0, Ratio: "1:1", KeyPattern: "G"}
	args := NewCommandBuilder(cfg).Build()
	if containsFlag(args, "--key-median") {
		t.Errorf("Should not have --key-median when 0")
	}
}

func TestBuild_KeyMedian_Set(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, KeyMedian: 50000, KeyPattern: "G", Ratio: "1:1"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--key-median", "50000") {
		t.Errorf("Expected --key-median 50000, got %v", args)
	}
}

func TestBuild_ZipfExponent_Zero(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, ZipfExponent: 0, Ratio: "1:1", KeyPattern: "Z"}
	args := NewCommandBuilder(cfg).Build()
	if containsFlag(args, "--key-zipf-exp") {
		t.Errorf("Should not have --key-zipf-exp when 0")
	}
}

func TestBuild_ZipfExponent_Set(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, ZipfExponent: 1.5, KeyPattern: "Z", Ratio: "1:1"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--key-zipf-exp", "1.5") {
		t.Errorf("Expected --key-zipf-exp 1.5, got %v", args)
	}
}

// =============================================================================
// DATA SIZE TESTS (15 tests)
// =============================================================================

func TestBuild_DataSize_Fixed_Small(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, DataSize: 32, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-d", "32") {
		t.Errorf("Expected -d 32, got %v", args)
	}
}

func TestBuild_DataSize_Fixed_Medium(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, DataSize: 1024, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-d", "1024") {
		t.Errorf("Expected -d 1024, got %v", args)
	}
}

func TestBuild_DataSize_Fixed_Large(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, DataSize: 1048576, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-d", "1048576") {
		t.Errorf("Expected -d 1048576, got %v", args)
	}
}

func TestBuild_DataSize_Range(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, DataSizeMin: 64, DataSizeMax: 1024, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--data-size-range", "64-1024") {
		t.Errorf("Expected --data-size-range 64-1024, got %v", args)
	}
}

func TestBuild_DataSize_Range_Wide(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, DataSizeMin: 8, DataSizeMax: 10240, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--data-size-range", "8-10240") {
		t.Errorf("Expected --data-size-range 8-10240, got %v", args)
	}
}

func TestBuild_DataSize_Range_WithPattern_Random(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, DataSizeMin: 64, DataSizeMax: 256, DataPattern: "R", Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--data-size-pattern", "R") {
		t.Errorf("Expected --data-size-pattern R, got %v", args)
	}
}

func TestBuild_DataSize_Range_WithPattern_Sequential(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, DataSizeMin: 64, DataSizeMax: 256, DataPattern: "S", Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--data-size-pattern", "S") {
		t.Errorf("Expected --data-size-pattern S, got %v", args)
	}
}

func TestBuild_DataSize_List(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, DataSizeList: "32:80,64:15,128:5", Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--data-size-list", "32:80,64:15,128:5") {
		t.Errorf("Expected --data-size-list 32:80,64:15,128:5, got %v", args)
	}
}

func TestBuild_DataSize_List_TakesPrecedence(t *testing.T) {
	// DataSizeList should take precedence over DataSize and DataSizeRange
	cfg := &Config{Host: "localhost", Port: 6379, DataSize: 100, DataSizeMin: 50, DataSizeMax: 200, DataSizeList: "64:50,128:50", Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--data-size-list", "64:50,128:50") {
		t.Errorf("Expected --data-size-list to be used, got %v", args)
	}
	if containsFlag(args, "-d") {
		t.Errorf("Should not have -d when DataSizeList is set")
	}
	if containsFlag(args, "--data-size-range") {
		t.Errorf("Should not have --data-size-range when DataSizeList is set")
	}
}

func TestBuild_DataOffset_Zero(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, DataOffset: 0, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if containsFlag(args, "--data-offset") {
		t.Errorf("Should not have --data-offset when 0")
	}
}

func TestBuild_DataOffset_Set(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, DataOffset: 100, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--data-offset", "100") {
		t.Errorf("Expected --data-offset 100, got %v", args)
	}
}

func TestBuild_RandomData_Disabled(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, RandomData: false, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if containsFlag(args, "-R") {
		t.Errorf("Should not have -R when RandomData is false")
	}
}

func TestBuild_RandomData_Enabled(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, RandomData: true, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlag(args, "-R") {
		t.Errorf("Expected -R flag, got %v", args)
	}
}

func TestBuild_DataSize_Zero_NoFlag(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, DataSize: 0, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if containsFlag(args, "-d") {
		t.Errorf("Should not have -d when DataSize is 0")
	}
}

func TestBuild_DataSize_Range_TakesPrecedence(t *testing.T) {
	// DataSizeRange should take precedence over DataSize
	cfg := &Config{Host: "localhost", Port: 6379, DataSize: 100, DataSizeMin: 64, DataSizeMax: 256, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--data-size-range", "64-256") {
		t.Errorf("Expected --data-size-range 64-256, got %v", args)
	}
	if containsFlag(args, "-d") {
		t.Errorf("Should not have -d when DataSizeRange is set")
	}
}

// =============================================================================
// COMMAND RATIO TESTS (12 tests)
// =============================================================================

func TestBuild_Ratio_Equal(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--ratio", "1:1") {
		t.Errorf("Expected --ratio 1:1, got %v", args)
	}
}

func TestBuild_Ratio_ReadHeavy(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Ratio: "9:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--ratio", "9:1") {
		t.Errorf("Expected --ratio 9:1, got %v", args)
	}
}

func TestBuild_Ratio_WriteHeavy(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Ratio: "1:9", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--ratio", "1:9") {
		t.Errorf("Expected --ratio 1:9, got %v", args)
	}
}

func TestBuild_Ratio_GetOnly(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Ratio: "1:0", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--ratio", "1:0") {
		t.Errorf("Expected --ratio 1:0, got %v", args)
	}
}

func TestBuild_Ratio_SetOnly(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Ratio: "0:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--ratio", "0:1") {
		t.Errorf("Expected --ratio 0:1, got %v", args)
	}
}

func TestBuild_Ratio_8020(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Ratio: "4:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--ratio", "4:1") {
		t.Errorf("Expected --ratio 4:1, got %v", args)
	}
}

func TestBuild_Ratio_LargeNumbers(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Ratio: "99:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--ratio", "99:1") {
		t.Errorf("Expected --ratio 99:1, got %v", args)
	}
}

func TestBuild_Ratio_5050(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Ratio: "50:50", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--ratio", "50:50") {
		t.Errorf("Expected --ratio 50:50, got %v", args)
	}
}

func TestBuild_Ratio_7030(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Ratio: "7:3", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--ratio", "7:3") {
		t.Errorf("Expected --ratio 7:3, got %v", args)
	}
}

func TestBuild_Ratio_INCR(t *testing.T) {
	// For INCR-only workload, ratio should still be set (though might use custom command)
	cfg := &Config{Host: "localhost", Port: 6379, Ratio: "0:0", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--ratio", "0:0") {
		t.Errorf("Expected --ratio 0:0, got %v", args)
	}
}

func TestBuild_Ratio_Decimal(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Ratio: "3:2", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--ratio", "3:2") {
		t.Errorf("Expected --ratio 3:2, got %v", args)
	}
}

func TestBuild_Ratio_Complex(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Ratio: "17:3", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--ratio", "17:3") {
		t.Errorf("Expected --ratio 17:3, got %v", args)
	}
}

// =============================================================================
// CUSTOM COMMAND TESTS (8 tests)
// =============================================================================

func TestBuild_CustomCommand_Single(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Ratio: "0:0", KeyPattern: "R", CustomCommands: []CustomCommand{
		{Command: "INCR __key__", Ratio: 1},
	}}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--command", "INCR __key__") {
		t.Errorf("Expected --command INCR __key__, got %v", args)
	}
}

func TestBuild_CustomCommand_WithRatio(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Ratio: "0:0", KeyPattern: "R", CustomCommands: []CustomCommand{
		{Command: "LPUSH __key__ __value__", Ratio: 5},
	}}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--command-ratio", "5") {
		t.Errorf("Expected --command-ratio 5, got %v", args)
	}
}

func TestBuild_CustomCommand_WithKeyPattern(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Ratio: "0:0", KeyPattern: "R", CustomCommands: []CustomCommand{
		{Command: "HSET __key__ field __value__", KeyPattern: "S"},
	}}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--command-key-pattern", "S") {
		t.Errorf("Expected --command-key-pattern S, got %v", args)
	}
}

func TestBuild_CustomCommand_ZADD(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Ratio: "0:0", KeyPattern: "R", CustomCommands: []CustomCommand{
		{Command: "ZADD __key__ __key__ __value__", Ratio: 1},
	}}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--command", "ZADD __key__ __key__ __value__") {
		t.Errorf("Expected ZADD command, got %v", args)
	}
}

func TestBuild_CustomCommand_SADD(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Ratio: "0:0", KeyPattern: "R", CustomCommands: []CustomCommand{
		{Command: "SADD __key__ __value__", Ratio: 1},
	}}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--command", "SADD __key__ __value__") {
		t.Errorf("Expected SADD command, got %v", args)
	}
}

func TestBuild_CustomCommand_Multiple(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Ratio: "0:0", KeyPattern: "R", CustomCommands: []CustomCommand{
		{Command: "INCR __key__", Ratio: 3},
		{Command: "LPUSH list __value__", Ratio: 1},
	}}
	args := NewCommandBuilder(cfg).Build()
	cmdStr := strings.Join(args, " ")
	if !strings.Contains(cmdStr, "--command INCR __key__") {
		t.Errorf("Expected INCR command, got %v", args)
	}
	if !strings.Contains(cmdStr, "--command LPUSH list __value__") {
		t.Errorf("Expected LPUSH command, got %v", args)
	}
}

func TestBuild_CustomCommand_NoRatio(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Ratio: "0:0", KeyPattern: "R", CustomCommands: []CustomCommand{
		{Command: "PING", Ratio: 0},
	}}
	args := NewCommandBuilder(cfg).Build()
	// Should have command but not command-ratio when ratio is 0
	if !containsFlagValue(args, "--command", "PING") {
		t.Errorf("Expected --command PING, got %v", args)
	}
}

func TestBuild_CustomCommand_XADD(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Ratio: "0:0", KeyPattern: "R", CustomCommands: []CustomCommand{
		{Command: "XADD __key__ * field __value__", Ratio: 1},
	}}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--command", "XADD __key__ * field __value__") {
		t.Errorf("Expected XADD stream command, got %v", args)
	}
}

// =============================================================================
// PARALLELISM TESTS (10 tests)
// =============================================================================

func TestBuild_Threads_Single(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-t", "1") {
		t.Errorf("Expected -t 1, got %v", args)
	}
}

func TestBuild_Threads_Default(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Threads: 4, Clients: 50, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-t", "4") {
		t.Errorf("Expected -t 4, got %v", args)
	}
}

func TestBuild_Threads_High(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Threads: 16, Clients: 200, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-t", "16") {
		t.Errorf("Expected -t 16, got %v", args)
	}
}

func TestBuild_Threads_Max(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Threads: 64, Clients: 500, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-t", "64") {
		t.Errorf("Expected -t 64, got %v", args)
	}
}

func TestBuild_Clients_Single(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-c", "1") {
		t.Errorf("Expected -c 1, got %v", args)
	}
}

func TestBuild_Clients_Default(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Threads: 4, Clients: 50, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-c", "50") {
		t.Errorf("Expected -c 50, got %v", args)
	}
}

func TestBuild_Clients_High(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Threads: 8, Clients: 200, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-c", "200") {
		t.Errorf("Expected -c 200, got %v", args)
	}
}

func TestBuild_Clients_Max(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Threads: 16, Clients: 1000, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-c", "1000") {
		t.Errorf("Expected -c 1000, got %v", args)
	}
}

func TestBuild_ThreadsClients_LowLatency(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Threads: 2, Clients: 10, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-t", "2") || !containsFlagValue(args, "-c", "10") {
		t.Errorf("Expected -t 2 -c 10, got %v", args)
	}
}

func TestBuild_ThreadsClients_HighThroughput(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Threads: 16, Clients: 500, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-t", "16") || !containsFlagValue(args, "-c", "500") {
		t.Errorf("Expected -t 16 -c 500, got %v", args)
	}
}

// =============================================================================
// DURATION/REQUESTS TESTS (10 tests)
// =============================================================================

func TestBuild_Duration_Short(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Duration: 5 * time.Second, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--test-time", "5") {
		t.Errorf("Expected --test-time 5, got %v", args)
	}
}

func TestBuild_Duration_Medium(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Duration: 30 * time.Second, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--test-time", "30") {
		t.Errorf("Expected --test-time 30, got %v", args)
	}
}

func TestBuild_Duration_Long(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Duration: 300 * time.Second, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--test-time", "300") {
		t.Errorf("Expected --test-time 300, got %v", args)
	}
}

func TestBuild_Duration_OneMinute(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Duration: 1 * time.Minute, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--test-time", "60") {
		t.Errorf("Expected --test-time 60, got %v", args)
	}
}

func TestBuild_Requests_Small(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Requests: 1000, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-n", "1000") {
		t.Errorf("Expected -n 1000, got %v", args)
	}
}

func TestBuild_Requests_Medium(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Requests: 100000, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-n", "100000") {
		t.Errorf("Expected -n 100000, got %v", args)
	}
}

func TestBuild_Requests_Large(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Requests: 10000000, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-n", "10000000") {
		t.Errorf("Expected -n 10000000, got %v", args)
	}
}

func TestBuild_Requests_OverridesDuration(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Requests: 10000, Duration: 30 * time.Second, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-n", "10000") {
		t.Errorf("Expected -n 10000, got %v", args)
	}
	// Duration should NOT be set when requests is specified
	if containsFlag(args, "--test-time") {
		t.Errorf("Should not have --test-time when requests is set")
	}
}

func TestBuild_RunCount_Single(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, RunCount: 1, Duration: 10 * time.Second, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if containsFlag(args, "-x") {
		t.Errorf("Should not have -x when RunCount is 1")
	}
}

func TestBuild_RunCount_Multiple(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, RunCount: 5, Duration: 10 * time.Second, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-x", "5") {
		t.Errorf("Expected -x 5, got %v", args)
	}
}

// =============================================================================
// PIPELINE TESTS (6 tests)
// =============================================================================

func TestBuild_Pipeline_Disabled(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Pipeline: 1, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--pipeline", "1") {
		t.Errorf("Expected --pipeline 1, got %v", args)
	}
}

func TestBuild_Pipeline_Small(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Pipeline: 5, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--pipeline", "5") {
		t.Errorf("Expected --pipeline 5, got %v", args)
	}
}

func TestBuild_Pipeline_Medium(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Pipeline: 10, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--pipeline", "10") {
		t.Errorf("Expected --pipeline 10, got %v", args)
	}
}

func TestBuild_Pipeline_Large(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Pipeline: 50, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--pipeline", "50") {
		t.Errorf("Expected --pipeline 50, got %v", args)
	}
}

func TestBuild_Pipeline_Aggressive(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Pipeline: 100, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--pipeline", "100") {
		t.Errorf("Expected --pipeline 100, got %v", args)
	}
}

func TestBuild_Pipeline_WithRateLimit(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Pipeline: 10, RateLimit: 5000, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--pipeline", "10") {
		t.Errorf("Expected --pipeline 10, got %v", args)
	}
	if !containsFlagValue(args, "--rate-limiting", "5000") {
		t.Errorf("Expected --rate-limiting 5000, got %v", args)
	}
}

// =============================================================================
// RATE LIMITING TESTS (5 tests)
// =============================================================================

func TestBuild_RateLimit_Disabled(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, RateLimit: 0, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if containsFlag(args, "--rate-limiting") {
		t.Errorf("Should not have --rate-limiting when disabled")
	}
}

func TestBuild_RateLimit_Low(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, RateLimit: 1000, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--rate-limiting", "1000") {
		t.Errorf("Expected --rate-limiting 1000, got %v", args)
	}
}

func TestBuild_RateLimit_Medium(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, RateLimit: 10000, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--rate-limiting", "10000") {
		t.Errorf("Expected --rate-limiting 10000, got %v", args)
	}
}

func TestBuild_RateLimit_High(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, RateLimit: 100000, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--rate-limiting", "100000") {
		t.Errorf("Expected --rate-limiting 100000, got %v", args)
	}
}

func TestBuild_RateLimit_VeryHigh(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, RateLimit: 500000, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--rate-limiting", "500000") {
		t.Errorf("Expected --rate-limiting 500000, got %v", args)
	}
}

// =============================================================================
// EXPIRY TESTS (6 tests)
// =============================================================================

func TestBuild_Expiry_Disabled(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, ExpiryMin: 0, ExpiryMax: 0, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if containsFlag(args, "--expiry-range") {
		t.Errorf("Should not have --expiry-range when disabled")
	}
}

func TestBuild_Expiry_Fixed(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, ExpiryMin: 60, ExpiryMax: 60, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--expiry-range", "60-60") {
		t.Errorf("Expected --expiry-range 60-60, got %v", args)
	}
}

func TestBuild_Expiry_Range(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, ExpiryMin: 30, ExpiryMax: 120, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--expiry-range", "30-120") {
		t.Errorf("Expected --expiry-range 30-120, got %v", args)
	}
}

func TestBuild_Expiry_MinOnly(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, ExpiryMin: 60, ExpiryMax: 0, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--expiry-range", "60-60") {
		t.Errorf("Expected --expiry-range 60-60 (min used for both), got %v", args)
	}
}

func TestBuild_Expiry_MaxOnly(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, ExpiryMin: 0, ExpiryMax: 120, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--expiry-range", "120-120") {
		t.Errorf("Expected --expiry-range 120-120 (max used for both), got %v", args)
	}
}

func TestBuild_Expiry_WideRange(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, ExpiryMin: 1, ExpiryMax: 3600, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--expiry-range", "1-3600") {
		t.Errorf("Expected --expiry-range 1-3600, got %v", args)
	}
}

// =============================================================================
// OUTPUT TESTS (6 tests)
// =============================================================================

func TestBuild_JSONOutput_Enabled(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, JSONOutput: true, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--json-out-file", "/dev/stdout") {
		t.Errorf("Expected --json-out-file /dev/stdout, got %v", args)
	}
}

func TestBuild_JSONOutput_Disabled(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, JSONOutput: false, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if containsFlag(args, "--json-out-file") {
		t.Errorf("Should not have --json-out-file when disabled")
	}
}

func TestBuild_HideHistogram_Enabled(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, HideHistogram: true, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlag(args, "--hide-histogram") {
		t.Errorf("Expected --hide-histogram, got %v", args)
	}
}

func TestBuild_HideHistogram_Disabled(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, HideHistogram: false, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if containsFlag(args, "--hide-histogram") {
		t.Errorf("Should not have --hide-histogram when disabled")
	}
}

func TestBuild_PrintPercentiles_Set(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, PrintPercentiles: []float64{50, 90, 99, 99.9}, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--print-percentiles", "50,90,99,99.9") {
		t.Errorf("Expected --print-percentiles 50,90,99,99.9, got %v", args)
	}
}

func TestBuild_PrintPercentiles_Empty(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, PrintPercentiles: []float64{}, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if containsFlag(args, "--print-percentiles") {
		t.Errorf("Should not have --print-percentiles when empty")
	}
}

// =============================================================================
// RANDOMIZATION TESTS (4 tests)
// =============================================================================

func TestBuild_DistinctClientSeed_Enabled(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, DistinctClientSeed: true, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlag(args, "--distinct-client-seed") {
		t.Errorf("Expected --distinct-client-seed, got %v", args)
	}
}

func TestBuild_DistinctClientSeed_Disabled(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, DistinctClientSeed: false, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if containsFlag(args, "--distinct-client-seed") {
		t.Errorf("Should not have --distinct-client-seed when disabled")
	}
}

func TestBuild_RandomizeSeed_Enabled(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, RandomizeSeed: true, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlag(args, "--randomize") {
		t.Errorf("Expected --randomize, got %v", args)
	}
}

func TestBuild_RandomizeSeed_Disabled(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, RandomizeSeed: false, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if containsFlag(args, "--randomize") {
		t.Errorf("Should not have --randomize when disabled")
	}
}

// =============================================================================
// MULTI-KEY TESTS (4 tests)
// =============================================================================

func TestBuild_MultiKeyGet_Disabled(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, MultiKeyGet: 0, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if containsFlag(args, "--multi-key-get") {
		t.Errorf("Should not have --multi-key-get when disabled")
	}
}

func TestBuild_MultiKeyGet_Small(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, MultiKeyGet: 5, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--multi-key-get", "5") {
		t.Errorf("Expected --multi-key-get 5, got %v", args)
	}
}

func TestBuild_MultiKeyGet_Medium(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, MultiKeyGet: 10, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--multi-key-get", "10") {
		t.Errorf("Expected --multi-key-get 10, got %v", args)
	}
}

func TestBuild_MultiKeyGet_Large(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, MultiKeyGet: 100, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--multi-key-get", "100") {
		t.Errorf("Expected --multi-key-get 100, got %v", args)
	}
}

// =============================================================================
// RECONNECT TESTS (3 tests)
// =============================================================================

func TestBuild_ReconnectInterval_Disabled(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, ReconnectInterval: 0, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if containsFlag(args, "--reconnect-interval") {
		t.Errorf("Should not have --reconnect-interval when disabled")
	}
}

func TestBuild_ReconnectInterval_Small(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, ReconnectInterval: 1000, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--reconnect-interval", "1000") {
		t.Errorf("Expected --reconnect-interval 1000, got %v", args)
	}
}

func TestBuild_ReconnectInterval_Large(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, ReconnectInterval: 10000, Threads: 1, Clients: 1, Ratio: "1:1", KeyPattern: "R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--reconnect-interval", "10000") {
		t.Errorf("Expected --reconnect-interval 10000, got %v", args)
	}
}

// =============================================================================
// EDGE CASES & INTEGRATION TESTS (10 tests)
// =============================================================================

func TestBuild_FullConfig_AllParameters(t *testing.T) {
	cfg := &Config{
		// Connection
		Host:     "redis.example.com",
		Port:     6380,
		Password: "secret",
		Username: "admin",
		TLS:      true,
		Cluster:  true,
		// Protocol
		Protocol: "resp3",
		SelectDB: 1,
		// Key settings
		Ratio:        "4:1",
		KeyPattern:   "G",
		KeyMinimum:   1000,
		KeyMaximum:   1000000,
		KeyPrefix:    "myapp:",
		KeyStddev:    0.2,
		KeyMedian:    500000,
		ZipfExponent: 0, // Not set (using Gaussian)
		// Data settings
		DataSize:   256,
		RandomData: true,
		DataOffset: 10,
		ExpiryMin:  60,
		ExpiryMax:  300,
		// Execution
		Threads:            8,
		Clients:            100,
		Requests:           100000,
		Pipeline:           10,
		RateLimit:          50000,
		RunCount:           3,
		ReconnectInterval:  5000,
		DistinctClientSeed: true,
		RandomizeSeed:      true,
		MultiKeyGet:        5,
		// Output
		JSONOutput:       true,
		HideHistogram:    true,
		PrintPercentiles: []float64{50, 90, 99},
	}
	args := NewCommandBuilder(cfg).Build()
	cmdStr := strings.Join(args, " ")

	// Verify key parameters are present
	expectedParts := []string{
		"-s redis.example.com",
		"-p 6380",
		"-a secret",
		"--user admin",
		"--tls",
		"--cluster-mode",
		"-P resp3",
		"--select-db 1",
		"--ratio 4:1",
		"--key-pattern G",
		"--key-minimum 1000",
		"--key-maximum 1000000",
		"--key-prefix myapp:",
		"--key-stddev 0.2",
		"--key-median 500000",
		"-d 256",
		"-R",
		"--data-offset 10",
		"--expiry-range 60-300",
		"-t 8",
		"-c 100",
		"-n 100000",
		"--pipeline 10",
		"--rate-limiting 50000",
		"-x 3",
		"--reconnect-interval 5000",
		"--distinct-client-seed",
		"--randomize",
		"--multi-key-get 5",
		"--json-out-file /dev/stdout",
		"--hide-histogram",
		"--print-percentiles 50,90,99",
	}

	for _, part := range expectedParts {
		if !strings.Contains(cmdStr, part) {
			t.Errorf("Missing expected part %q in command: %s", part, cmdStr)
		}
	}
}

func TestBuild_MinimalConfig(t *testing.T) {
	cfg := &Config{
		Host:       "localhost",
		Port:       6379,
		Threads:    1,
		Clients:    1,
		Ratio:      "1:1",
		KeyPattern: "R",
	}
	args := NewCommandBuilder(cfg).Build()

	// Should have minimum required args
	if len(args) < 10 {
		t.Errorf("Expected at least 10 args for minimal config, got %d", len(args))
	}
}

func TestBuild_DefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	args := NewCommandBuilder(cfg).Build()

	// Verify defaults are applied
	if !containsFlagValue(args, "-s", "localhost") {
		t.Errorf("Expected default host localhost")
	}
	if !containsFlagValue(args, "-p", "6379") {
		t.Errorf("Expected default port 6379")
	}
	if !containsFlagValue(args, "-t", "4") {
		t.Errorf("Expected default threads 4")
	}
	if !containsFlagValue(args, "-c", "50") {
		t.Errorf("Expected default clients 50")
	}
	if !containsFlagValue(args, "--pipeline", "1") {
		t.Errorf("Expected default pipeline 1")
	}
}

func TestBuild_CloudBenchmarkConfig(t *testing.T) {
	// Simulate cloud benchmark with TLS and cluster
	cfg := &Config{
		Host:       "redis-cluster.redis.cache.windows.net",
		Port:       6380,
		Password:   "access-key-here",
		TLS:        true,
		Cluster:    true,
		Threads:    4,
		Clients:    50,
		Requests:   100000,
		Ratio:      "4:1",
		KeyPattern: "R",
		Pipeline:   10,
		DataSize:   256,
	}
	args := NewCommandBuilder(cfg).Build()

	if !containsFlag(args, "--tls") {
		t.Errorf("Expected --tls for cloud benchmark")
	}
	if !containsFlag(args, "--cluster-mode") {
		t.Errorf("Expected --cluster-mode for cloud benchmark")
	}
}

func TestBuild_LatencyTestConfig(t *testing.T) {
	// Low latency measurement configuration
	cfg := &Config{
		Host:             "localhost",
		Port:             6379,
		Threads:          2,
		Clients:          10,
		Duration:         30 * time.Second,
		Ratio:            "1:1",
		KeyPattern:       "R",
		Pipeline:         1,
		PrintPercentiles: []float64{50, 90, 95, 99, 99.5, 99.9},
	}
	args := NewCommandBuilder(cfg).Build()

	if !containsFlagValue(args, "--pipeline", "1") {
		t.Errorf("Expected pipeline 1 for latency test")
	}
	if !containsFlag(args, "--print-percentiles") {
		t.Errorf("Expected percentiles for latency test")
	}
}

func TestBuild_ThroughputTestConfig(t *testing.T) {
	// High throughput configuration
	cfg := &Config{
		Host:       "localhost",
		Port:       6379,
		Threads:    16,
		Clients:    500,
		Duration:   60 * time.Second,
		Ratio:      "1:1",
		KeyPattern: "R",
		Pipeline:   50,
		DataSize:   32,
	}
	args := NewCommandBuilder(cfg).Build()

	if !containsFlagValue(args, "-t", "16") {
		t.Errorf("Expected 16 threads for throughput test")
	}
	if !containsFlagValue(args, "-c", "500") {
		t.Errorf("Expected 500 clients for throughput test")
	}
	if !containsFlagValue(args, "--pipeline", "50") {
		t.Errorf("Expected pipeline 50 for throughput test")
	}
}

func TestBuild_StressTestConfig(t *testing.T) {
	// Stress test configuration
	cfg := &Config{
		Host:       "localhost",
		Port:       6379,
		Threads:    32,
		Clients:    1000,
		Duration:   300 * time.Second,
		Ratio:      "1:1",
		KeyPattern: "R",
		Pipeline:   100,
		DataSize:   1024,
		RunCount:   5,
	}
	args := NewCommandBuilder(cfg).Build()

	if !containsFlagValue(args, "-t", "32") {
		t.Errorf("Expected 32 threads for stress test")
	}
	if !containsFlagValue(args, "-x", "5") {
		t.Errorf("Expected 5 iterations for stress test")
	}
}

func TestBuild_GaussianDistributionConfig(t *testing.T) {
	// Gaussian/hot-key distribution
	cfg := &Config{
		Host:       "localhost",
		Port:       6379,
		Threads:    4,
		Clients:    50,
		Duration:   30 * time.Second,
		Ratio:      "4:1",
		KeyPattern: "G:G",
		KeyMinimum: 0,
		KeyMaximum: 1000000,
		KeyStddev:  0.1,
		KeyMedian:  500000,
		DataSize:   256,
	}
	args := NewCommandBuilder(cfg).Build()

	if !containsFlagValue(args, "--key-pattern", "G:G") {
		t.Errorf("Expected Gaussian key pattern")
	}
	if !containsFlagValue(args, "--key-stddev", "0.1") {
		t.Errorf("Expected key stddev 0.1")
	}
	if !containsFlagValue(args, "--key-median", "500000") {
		t.Errorf("Expected key median 500000")
	}
}

func TestBuild_ZipfDistributionConfig(t *testing.T) {
	// Zipf distribution for realistic access patterns
	cfg := &Config{
		Host:         "localhost",
		Port:         6379,
		Threads:      4,
		Clients:      50,
		Duration:     30 * time.Second,
		Ratio:        "4:1",
		KeyPattern:   "Z:Z",
		KeyMinimum:   0,
		KeyMaximum:   1000000,
		ZipfExponent: 1.2,
		DataSize:     256,
	}
	args := NewCommandBuilder(cfg).Build()

	if !containsFlagValue(args, "--key-pattern", "Z:Z") {
		t.Errorf("Expected Zipf key pattern")
	}
	if !containsFlagValue(args, "--key-zipf-exp", "1.2") {
		t.Errorf("Expected Zipf exponent 1.2")
	}
}

func TestBuild_VariableDataSizeConfig(t *testing.T) {
	// Variable data size with weighted distribution
	cfg := &Config{
		Host:         "localhost",
		Port:         6379,
		Threads:      4,
		Clients:      50,
		Duration:     30 * time.Second,
		Ratio:        "1:1",
		KeyPattern:   "R",
		DataSizeList: "32:50,128:30,512:15,1024:5",
	}
	args := NewCommandBuilder(cfg).Build()

	if !containsFlagValue(args, "--data-size-list", "32:50,128:30,512:15,1024:5") {
		t.Errorf("Expected data-size-list with weighted distribution")
	}
}

// =============================================================================
// NEW CONNECTION TESTS - Unix Socket, URI, IPv4/IPv6
// =============================================================================

func TestBuild_Connection_UnixSocket(t *testing.T) {
	cfg := &Config{UnixSocket: "/var/run/redis.sock", Ratio: "1:1", KeyPattern: "R:R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-S", "/var/run/redis.sock") {
		t.Errorf("Expected -S /var/run/redis.sock, got %v", args)
	}
	// Should not have -s or -p when using socket
	if containsFlag(args, "-s") {
		t.Errorf("Should not have -s flag when using Unix socket")
	}
}

func TestBuild_Connection_URI(t *testing.T) {
	cfg := &Config{URI: "redis://user:pass@localhost:6379/0", Ratio: "1:1", KeyPattern: "R:R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-u", "redis://user:pass@localhost:6379/0") {
		t.Errorf("Expected -u with URI, got %v", args)
	}
}

func TestBuild_Connection_ForceIPv4(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, ForceIPv4: true, Ratio: "1:1", KeyPattern: "R:R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlag(args, "-4") {
		t.Errorf("Expected -4 flag, got %v", args)
	}
}

func TestBuild_Connection_ForceIPv6(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, ForceIPv6: true, Ratio: "1:1", KeyPattern: "R:R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlag(args, "-6") {
		t.Errorf("Expected -6 flag, got %v", args)
	}
}

// =============================================================================
// NEW TLS TESTS - Certificates, protocols, SNI
// =============================================================================

func TestBuild_TLS_WithCert(t *testing.T) {
	cfg := &Config{
		Host:       "localhost",
		Port:       6379,
		TLS:        true,
		TLSCert:    "/path/to/cert.pem",
		Ratio:      "1:1",
		KeyPattern: "R:R",
	}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlag(args, "--tls") {
		t.Errorf("Expected --tls flag")
	}
	if !containsFlagValue(args, "--cert", "/path/to/cert.pem") {
		t.Errorf("Expected --cert flag with path, got %v", args)
	}
}

func TestBuild_TLS_WithKey(t *testing.T) {
	cfg := &Config{
		Host:       "localhost",
		Port:       6379,
		TLS:        true,
		TLSKey:     "/path/to/key.pem",
		Ratio:      "1:1",
		KeyPattern: "R:R",
	}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--key", "/path/to/key.pem") {
		t.Errorf("Expected --key flag with path, got %v", args)
	}
}

func TestBuild_TLS_WithCACert(t *testing.T) {
	cfg := &Config{
		Host:       "localhost",
		Port:       6379,
		TLS:        true,
		TLSCACert:  "/path/to/ca.pem",
		Ratio:      "1:1",
		KeyPattern: "R:R",
	}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--cacert", "/path/to/ca.pem") {
		t.Errorf("Expected --cacert flag with path, got %v", args)
	}
}

func TestBuild_TLS_Protocols(t *testing.T) {
	cfg := &Config{
		Host:         "localhost",
		Port:         6379,
		TLS:          true,
		TLSProtocols: "TLSv1.2,TLSv1.3",
		Ratio:        "1:1",
		KeyPattern:   "R:R",
	}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--tls-protocols", "TLSv1.2,TLSv1.3") {
		t.Errorf("Expected --tls-protocols flag, got %v", args)
	}
}

func TestBuild_TLS_SNI(t *testing.T) {
	cfg := &Config{
		Host:       "localhost",
		Port:       6379,
		TLS:        true,
		TLSSNI:     "redis.example.com",
		Ratio:      "1:1",
		KeyPattern: "R:R",
	}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--sni", "redis.example.com") {
		t.Errorf("Expected --sni flag, got %v", args)
	}
}

func TestBuild_TLS_FullConfig(t *testing.T) {
	cfg := &Config{
		Host:          "localhost",
		Port:          6379,
		TLS:           true,
		TLSCert:       "/path/to/cert.pem",
		TLSKey:        "/path/to/key.pem",
		TLSCACert:     "/path/to/ca.pem",
		TLSSkipVerify: false,
		TLSProtocols:  "TLSv1.3",
		TLSSNI:        "redis.example.com",
		Ratio:         "1:1",
		KeyPattern:    "R:R",
	}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlag(args, "--tls") {
		t.Errorf("Expected --tls flag")
	}
	if !containsFlagValue(args, "--cert", "/path/to/cert.pem") {
		t.Errorf("Expected --cert flag")
	}
	if !containsFlagValue(args, "--key", "/path/to/key.pem") {
		t.Errorf("Expected --key flag")
	}
	if !containsFlagValue(args, "--cacert", "/path/to/ca.pem") {
		t.Errorf("Expected --cacert flag")
	}
	if containsFlag(args, "--tls-skip-verify") {
		t.Errorf("Should not have --tls-skip-verify when TLSSkipVerify is false")
	}
}

// =============================================================================
// DATA IMPORT TESTS
// =============================================================================

func TestBuild_DataImport_Basic(t *testing.T) {
	cfg := &Config{
		Host:       "localhost",
		Port:       6379,
		DataImport: "/path/to/data.rdb",
		Ratio:      "1:1",
		KeyPattern: "R:R",
	}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--data-import", "/path/to/data.rdb") {
		t.Errorf("Expected --data-import flag, got %v", args)
	}
}

func TestBuild_DataImport_WithVerify(t *testing.T) {
	cfg := &Config{
		Host:       "localhost",
		Port:       6379,
		DataImport: "/path/to/data.rdb",
		DataVerify: true,
		Ratio:      "1:1",
		KeyPattern: "R:R",
	}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--data-import", "/path/to/data.rdb") {
		t.Errorf("Expected --data-import flag")
	}
	if !containsFlag(args, "--data-verify") {
		t.Errorf("Expected --data-verify flag, got %v", args)
	}
}

func TestBuild_DataImport_VerifyOnly(t *testing.T) {
	cfg := &Config{
		Host:       "localhost",
		Port:       6379,
		DataImport: "/path/to/data.rdb",
		VerifyOnly: true,
		Ratio:      "1:1",
		KeyPattern: "R:R",
	}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlag(args, "--verify-only") {
		t.Errorf("Expected --verify-only flag, got %v", args)
	}
}

func TestBuild_DataImport_GenerateKeys(t *testing.T) {
	cfg := &Config{
		Host:         "localhost",
		Port:         6379,
		DataImport:   "/path/to/data.rdb",
		GenerateKeys: true,
		Ratio:        "1:1",
		KeyPattern:   "R:R",
	}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlag(args, "--generate-keys") {
		t.Errorf("Expected --generate-keys flag, got %v", args)
	}
}

func TestBuild_DataImport_NoExpiry(t *testing.T) {
	cfg := &Config{
		Host:       "localhost",
		Port:       6379,
		DataImport: "/path/to/data.rdb",
		NoExpiry:   true,
		Ratio:      "1:1",
		KeyPattern: "R:R",
	}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlag(args, "--no-expiry") {
		t.Errorf("Expected --no-expiry flag, got %v", args)
	}
}

func TestBuild_DataImport_FullConfig(t *testing.T) {
	cfg := &Config{
		Host:         "localhost",
		Port:         6379,
		DataImport:   "/path/to/data.rdb",
		DataVerify:   true,
		GenerateKeys: true,
		NoExpiry:     true,
		Ratio:        "1:1",
		KeyPattern:   "R:R",
	}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--data-import", "/path/to/data.rdb") {
		t.Errorf("Expected --data-import flag")
	}
	if !containsFlag(args, "--data-verify") {
		t.Errorf("Expected --data-verify flag")
	}
	if !containsFlag(args, "--generate-keys") {
		t.Errorf("Expected --generate-keys flag")
	}
	if !containsFlag(args, "--no-expiry") {
		t.Errorf("Expected --no-expiry flag")
	}
}

// =============================================================================
// WAIT OPTIONS TESTS (replication)
// =============================================================================

func TestBuild_Wait_Ratio(t *testing.T) {
	cfg := &Config{
		Host:       "localhost",
		Port:       6379,
		WaitRatio:  "1:1",
		Ratio:      "1:1",
		KeyPattern: "R:R",
	}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--wait-ratio", "1:1") {
		t.Errorf("Expected --wait-ratio flag, got %v", args)
	}
}

func TestBuild_Wait_NumSlaves(t *testing.T) {
	cfg := &Config{
		Host:         "localhost",
		Port:         6379,
		NumSlavesMin: 1,
		NumSlavesMax: 2,
		Ratio:        "1:1",
		KeyPattern:   "R:R",
	}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--num-slaves", "1-2") {
		t.Errorf("Expected --num-slaves 1-2, got %v", args)
	}
}

func TestBuild_Wait_Timeout(t *testing.T) {
	cfg := &Config{
		Host:           "localhost",
		Port:           6379,
		WaitTimeoutMin: 100,
		WaitTimeoutMax: 500,
		Ratio:          "1:1",
		KeyPattern:     "R:R",
	}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--wait-timeout", "100-500") {
		t.Errorf("Expected --wait-timeout 100-500, got %v", args)
	}
}

// =============================================================================
// OUTPUT OPTIONS TESTS
// =============================================================================

func TestBuild_Output_Debug(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Debug: true, Ratio: "1:1", KeyPattern: "R:R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlag(args, "-D") {
		t.Errorf("Expected -D flag for debug, got %v", args)
	}
}

func TestBuild_Output_ShowConfig(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, ShowConfig: true, Ratio: "1:1", KeyPattern: "R:R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlag(args, "--show-config") {
		t.Errorf("Expected --show-config flag, got %v", args)
	}
}

func TestBuild_Output_OutFile(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, OutFile: "/tmp/output.txt", Ratio: "1:1", KeyPattern: "R:R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-o", "/tmp/output.txt") {
		t.Errorf("Expected -o /tmp/output.txt, got %v", args)
	}
}

func TestBuild_Output_JSONOutFile(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, JSONOutFile: "/tmp/results.json", Ratio: "1:1", KeyPattern: "R:R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--json-out-file", "/tmp/results.json") {
		t.Errorf("Expected --json-out-file /tmp/results.json, got %v", args)
	}
}

func TestBuild_Output_HdrFilePrefix(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, HdrFilePrefix: "/tmp/hdr_", Ratio: "1:1", KeyPattern: "R:R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--hdr-file-prefix", "/tmp/hdr_") {
		t.Errorf("Expected --hdr-file-prefix, got %v", args)
	}
}

func TestBuild_Output_ClientStats(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, ClientStats: "/tmp/client_stats.csv", Ratio: "1:1", KeyPattern: "R:R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--client-stats", "/tmp/client_stats.csv") {
		t.Errorf("Expected --client-stats flag, got %v", args)
	}
}

func TestBuild_Output_PrintAllRuns(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, PrintAllRuns: true, RunCount: 3, Ratio: "1:1", KeyPattern: "R:R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlag(args, "--print-all-runs") {
		t.Errorf("Expected --print-all-runs flag, got %v", args)
	}
}

// =============================================================================
// PROTOCOL TESTS - resp2, resp3 (additional)
// =============================================================================

func TestBuild_Protocol_RESP2_New(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Protocol: "resp2", Ratio: "1:1", KeyPattern: "R:R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-P", "resp2") {
		t.Errorf("Expected -P resp2, got %v", args)
	}
}

func TestBuild_Protocol_RESP3_New(t *testing.T) {
	cfg := &Config{Host: "localhost", Port: 6379, Protocol: "resp3", Ratio: "1:1", KeyPattern: "R:R"}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-P", "resp3") {
		t.Errorf("Expected -P resp3, got %v", args)
	}
}

// =============================================================================
// REQUEST COUNT TEST (instead of duration)
// =============================================================================

func TestBuild_Requests_Count(t *testing.T) {
	cfg := &Config{
		Host:       "localhost",
		Port:       6379,
		Requests:   10000,
		Ratio:      "1:1",
		KeyPattern: "R:R",
	}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-n", "10000") {
		t.Errorf("Expected -n 10000, got %v", args)
	}
}

func TestBuild_Requests_Allkeys(t *testing.T) {
	// Note: memtier supports "allkeys" but we use int64, so this tests large number
	cfg := &Config{
		Host:       "localhost",
		Port:       6379,
		Requests:   1000000,
		KeyMaximum: 1000000,
		Ratio:      "1:1",
		KeyPattern: "R:R",
	}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "-n", "1000000") {
		t.Errorf("Expected -n 1000000, got %v", args)
	}
}

// =============================================================================
// ZIPF EXPONENT TEST
// =============================================================================

func TestBuild_ZipfExponent(t *testing.T) {
	cfg := &Config{
		Host:         "localhost",
		Port:         6379,
		KeyPattern:   "Z:Z",
		ZipfExponent: 1.5,
		Ratio:        "1:1",
	}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--key-zipf-exp", "1.5") {
		t.Errorf("Expected --key-zipf-exp 1.5, got %v", args)
	}
}

func TestBuild_ZipfExponent_High(t *testing.T) {
	cfg := &Config{
		Host:         "localhost",
		Port:         6379,
		KeyPattern:   "Z:Z",
		ZipfExponent: 2.5,
		Ratio:        "1:1",
	}
	args := NewCommandBuilder(cfg).Build()
	if !containsFlagValue(args, "--key-zipf-exp", "2.5") {
		t.Errorf("Expected --key-zipf-exp 2.5, got %v", args)
	}
}

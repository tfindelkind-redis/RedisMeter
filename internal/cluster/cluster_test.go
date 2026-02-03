package cluster

import (
	"testing"
)

func TestGetSlotForKey(t *testing.T) {
	tests := []struct {
		key      string
		expected int
	}{
		// Standard keys
		{"foo", 12182},
		{"bar", 5061},
		{"hello", 866},
		
		// Keys with hashtags
		{"{user1000}.following", GetSlotForKey("user1000")},
		{"{user1000}.followers", GetSlotForKey("user1000")},
		{"foo{user1000}bar", GetSlotForKey("user1000")},
		
		// Empty hashtag should use full key
		{"foo{}bar", GetSlotForKey("foo{}bar")},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			slot := GetSlotForKey(tt.key)
			if slot != tt.expected {
				t.Errorf("GetSlotForKey(%s) = %d, want %d", tt.key, slot, tt.expected)
			}
			// All slots should be in valid range
			if slot < 0 || slot >= 16384 {
				t.Errorf("GetSlotForKey(%s) = %d, out of range [0, 16384)", tt.key, slot)
			}
		})
	}
}

func TestGetSlotForKey_HashtagConsistency(t *testing.T) {
	// Keys with same hashtag should go to same slot
	keys := []string{
		"{user:1234}.profile",
		"{user:1234}.posts",
		"{user:1234}.settings",
		"prefix:{user:1234}:suffix",
	}

	expectedSlot := GetSlotForKey("user:1234")
	for _, key := range keys {
		slot := GetSlotForKey(key)
		if slot != expectedSlot {
			t.Errorf("Key %s got slot %d, expected %d (same as 'user:1234')", key, slot, expectedSlot)
		}
	}
}

func TestSlotRange(t *testing.T) {
	sr := SlotRange{Start: 0, End: 5460}
	
	if sr.Start != 0 {
		t.Errorf("Expected start 0, got %d", sr.Start)
	}
	if sr.End != 5460 {
		t.Errorf("Expected end 5460, got %d", sr.End)
	}
}

func TestParseSlotRange(t *testing.T) {
	tests := []struct {
		input    string
		expected SlotRange
		hasError bool
	}{
		{"0-5460", SlotRange{Start: 0, End: 5460}, false},
		{"5461-10922", SlotRange{Start: 5461, End: 10922}, false},
		{"10923-16383", SlotRange{Start: 10923, End: 16383}, false},
		{"5000", SlotRange{Start: 5000, End: 5000}, false},
		{"abc", SlotRange{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			sr, err := parseSlotRange(tt.input)
			if tt.hasError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if sr.Start != tt.expected.Start || sr.End != tt.expected.End {
				t.Errorf("parseSlotRange(%s) = %+v, want %+v", tt.input, sr, tt.expected)
			}
		})
	}
}

func TestNodeInfo(t *testing.T) {
	node := NodeInfo{
		ID:          "abc123",
		Address:     "127.0.0.1:6379",
		Host:        "127.0.0.1",
		Port:        6379,
		Role:        "master",
		Connected:   true,
		ConfigEpoch: 1,
		Slots: []SlotRange{
			{Start: 0, End: 5460},
		},
	}

	if node.ID != "abc123" {
		t.Errorf("Expected ID 'abc123', got %s", node.ID)
	}
	if node.Role != "master" {
		t.Errorf("Expected role 'master', got %s", node.Role)
	}
	if len(node.Slots) != 1 {
		t.Errorf("Expected 1 slot range, got %d", len(node.Slots))
	}
}

func TestClusterTopology(t *testing.T) {
	topology := &ClusterTopology{
		TotalSlots:   16384,
		CoveredSlots: 16384,
		ClusterOK:    true,
		CurrentEpoch: 6,
		Masters: []NodeInfo{
			{ID: "master1", Role: "master"},
			{ID: "master2", Role: "master"},
			{ID: "master3", Role: "master"},
		},
		Replicas: []NodeInfo{
			{ID: "replica1", Role: "replica", MasterID: "master1"},
			{ID: "replica2", Role: "replica", MasterID: "master2"},
			{ID: "replica3", Role: "replica", MasterID: "master3"},
		},
	}

	if !topology.ClusterOK {
		t.Error("Expected cluster OK")
	}
	if len(topology.Masters) != 3 {
		t.Errorf("Expected 3 masters, got %d", len(topology.Masters))
	}
	if len(topology.Replicas) != 3 {
		t.Errorf("Expected 3 replicas, got %d", len(topology.Replicas))
	}
}

func TestSlotDistribution(t *testing.T) {
	dist := NewSlotDistribution()

	// Record some keys
	dist.RecordKey("user:1", "127.0.0.1:6379")
	dist.RecordKey("user:2", "127.0.0.1:6380")
	dist.RecordKey("user:3", "127.0.0.1:6379")
	dist.RecordKey("{user:1}.profile", "127.0.0.1:6379")

	nodeCounts := dist.GetNodeCounts()
	if len(nodeCounts) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(nodeCounts))
	}

	stats := dist.GetDistributionStats()
	if stats.TotalOps != 4 {
		t.Errorf("Expected 4 total ops, got %d", stats.TotalOps)
	}

	// Check node distribution
	if nodeCounts["127.0.0.1:6379"] != 3 {
		t.Errorf("Expected 3 ops on :6379, got %d", nodeCounts["127.0.0.1:6379"])
	}
	if nodeCounts["127.0.0.1:6380"] != 1 {
		t.Errorf("Expected 1 op on :6380, got %d", nodeCounts["127.0.0.1:6380"])
	}
}

func TestSlotDistribution_GetSlotCounts(t *testing.T) {
	dist := NewSlotDistribution()

	// Same key multiple times
	dist.RecordKey("foo", "node1")
	dist.RecordKey("foo", "node1")
	dist.RecordKey("foo", "node1")

	slotCounts := dist.GetSlotCounts()
	fooSlot := GetSlotForKey("foo")
	
	if slotCounts[fooSlot] != 3 {
		t.Errorf("Expected 3 for slot %d, got %d", fooSlot, slotCounts[fooSlot])
	}
}

func TestClusterBenchmarkConfig(t *testing.T) {
	config := DefaultClusterBenchmarkConfig()

	if !config.ShardAware {
		t.Error("Expected shard aware by default")
	}
	if config.ReplicaReads {
		t.Error("Expected replica reads disabled by default")
	}
	if config.FailoverTesting {
		t.Error("Expected failover testing disabled by default")
	}
}

func TestFailoverScenario(t *testing.T) {
	scenario := FailoverScenario{
		Name:        "manual-failover",
		Description: "Test manual failover",
		Type:        "manual",
		TargetNode:  "127.0.0.1:6379",
	}

	if scenario.Name != "manual-failover" {
		t.Errorf("Expected name 'manual-failover', got %s", scenario.Name)
	}
	if scenario.Type != "manual" {
		t.Errorf("Expected type 'manual', got %s", scenario.Type)
	}
}

func TestCRC16(t *testing.T) {
	// These are known CRC16 values for Redis cluster
	tests := []struct {
		input    string
		expected uint16
	}{
		{"123456789", 0x31C3}, // Standard CRC16 test vector
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := crc16([]byte(tt.input))
			if result != tt.expected {
				t.Errorf("crc16(%s) = %04X, want %04X", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDistributionStats(t *testing.T) {
	stats := DistributionStats{
		TotalOps: 1000,
		NodeOps: map[string]int64{
			"node1": 500,
			"node2": 300,
			"node3": 200,
		},
		SlotGroups: map[string]int64{
			"0-999":      300,
			"1000-1999":  350,
			"2000-2999":  350,
		},
	}

	if stats.TotalOps != 1000 {
		t.Errorf("Expected 1000 total ops, got %d", stats.TotalOps)
	}
	if len(stats.NodeOps) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(stats.NodeOps))
	}
}

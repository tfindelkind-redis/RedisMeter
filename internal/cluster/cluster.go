// Package cluster provides Redis cluster testing capabilities.
package cluster

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// NodeInfo represents information about a cluster node.
type NodeInfo struct {
	ID         string            `json:"id"`
	Address    string            `json:"address"`
	Host       string            `json:"host"`
	Port       int               `json:"port"`
	Role       string            `json:"role"` // master, slave/replica
	MasterID   string            `json:"master_id,omitempty"`
	Slots      []SlotRange       `json:"slots,omitempty"`
	Flags      []string          `json:"flags"`
	Connected  bool              `json:"connected"`
	PingSent   int64             `json:"ping_sent"`
	PongRecv   int64             `json:"pong_recv"`
	ConfigEpoch int64            `json:"config_epoch"`
	LinkState  string            `json:"link_state"`
	Replicas   []string          `json:"replicas,omitempty"`
}

// SlotRange represents a range of hash slots.
type SlotRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// ClusterTopology represents the cluster topology.
type ClusterTopology struct {
	Nodes       []NodeInfo        `json:"nodes"`
	Masters     []NodeInfo        `json:"masters"`
	Replicas    []NodeInfo        `json:"replicas"`
	TotalSlots  int               `json:"total_slots"`
	CoveredSlots int              `json:"covered_slots"`
	ClusterOK   bool              `json:"cluster_ok"`
	CurrentEpoch int64            `json:"current_epoch"`
	MyEpoch     int64             `json:"my_epoch"`
	StateChange time.Time         `json:"state_change"`
}

// ClusterClient interface for cluster operations.
type ClusterClient interface {
	Do(ctx context.Context, args ...interface{}) (interface{}, error)
	DoOnNode(ctx context.Context, address string, args ...interface{}) (interface{}, error)
	GetNodeAddresses() []string
}

// ClusterAnalyzer analyzes cluster topology and health.
type ClusterAnalyzer struct {
	client ClusterClient
	mu     sync.RWMutex
	topology *ClusterTopology
}

// NewClusterAnalyzer creates a new cluster analyzer.
func NewClusterAnalyzer(client ClusterClient) *ClusterAnalyzer {
	return &ClusterAnalyzer{
		client: client,
	}
}

// GetTopology retrieves the current cluster topology.
func (a *ClusterAnalyzer) GetTopology(ctx context.Context) (*ClusterTopology, error) {
	result, err := a.client.Do(ctx, "CLUSTER", "NODES")
	if err != nil {
		return nil, fmt.Errorf("cluster nodes: %w", err)
	}

	nodesStr, ok := result.(string)
	if !ok {
		return nil, fmt.Errorf("unexpected result type")
	}

	topology := &ClusterTopology{
		TotalSlots: 16384,
	}

	lines := strings.Split(strings.TrimSpace(nodesStr), "\n")
	coveredSlots := make(map[int]bool)

	for _, line := range lines {
		if line == "" {
			continue
		}

		node, err := a.parseNodeLine(line)
		if err != nil {
			continue
		}

		topology.Nodes = append(topology.Nodes, node)

		if node.Role == "master" {
			topology.Masters = append(topology.Masters, node)
			for _, sr := range node.Slots {
				for slot := sr.Start; slot <= sr.End; slot++ {
					coveredSlots[slot] = true
				}
			}
		} else {
			topology.Replicas = append(topology.Replicas, node)
		}
	}

	topology.CoveredSlots = len(coveredSlots)
	topology.ClusterOK = topology.CoveredSlots == topology.TotalSlots

	// Get cluster info
	infoResult, err := a.client.Do(ctx, "CLUSTER", "INFO")
	if err == nil {
		if infoStr, ok := infoResult.(string); ok {
			a.parseClusterInfo(infoStr, topology)
		}
	}

	a.mu.Lock()
	a.topology = topology
	a.mu.Unlock()

	return topology, nil
}

func (a *ClusterAnalyzer) parseNodeLine(line string) (NodeInfo, error) {
	parts := strings.Fields(line)
	if len(parts) < 8 {
		return NodeInfo{}, fmt.Errorf("invalid node line")
	}

	node := NodeInfo{
		ID:        parts[0],
		Address:   parts[1],
		LinkState: parts[7],
	}

	// Parse address
	addrParts := strings.Split(strings.Split(parts[1], "@")[0], ":")
	if len(addrParts) == 2 {
		node.Host = addrParts[0]
		node.Port, _ = strconv.Atoi(addrParts[1])
	}

	// Parse flags
	node.Flags = strings.Split(parts[2], ",")
	for _, flag := range node.Flags {
		switch flag {
		case "master":
			node.Role = "master"
		case "slave", "replica":
			node.Role = "replica"
		case "fail":
			node.Connected = false
		}
	}
	if node.Role == "" {
		node.Role = "unknown"
	}

	// Master ID for replicas
	if parts[3] != "-" {
		node.MasterID = parts[3]
	}

	// Parse ping/pong
	node.PingSent, _ = strconv.ParseInt(parts[4], 10, 64)
	node.PongRecv, _ = strconv.ParseInt(parts[5], 10, 64)
	node.ConfigEpoch, _ = strconv.ParseInt(parts[6], 10, 64)

	// Check connected status
	node.Connected = parts[7] == "connected"

	// Parse slots (for masters)
	if len(parts) > 8 && node.Role == "master" {
		for _, slotStr := range parts[8:] {
			if strings.HasPrefix(slotStr, "[") {
				continue // Skip importing/migrating slots
			}
			sr, err := parseSlotRange(slotStr)
			if err == nil {
				node.Slots = append(node.Slots, sr)
			}
		}
	}

	return node, nil
}

func parseSlotRange(s string) (SlotRange, error) {
	if strings.Contains(s, "-") {
		parts := strings.Split(s, "-")
		if len(parts) == 2 {
			start, _ := strconv.Atoi(parts[0])
			end, _ := strconv.Atoi(parts[1])
			return SlotRange{Start: start, End: end}, nil
		}
	}

	slot, err := strconv.Atoi(s)
	if err != nil {
		return SlotRange{}, err
	}
	return SlotRange{Start: slot, End: slot}, nil
}

func (a *ClusterAnalyzer) parseClusterInfo(info string, topology *ClusterTopology) {
	lines := strings.Split(info, "\n")
	for _, line := range lines {
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) != 2 {
			continue
		}
		key, value := parts[0], parts[1]

		switch key {
		case "cluster_state":
			topology.ClusterOK = value == "ok"
		case "cluster_current_epoch":
			topology.CurrentEpoch, _ = strconv.ParseInt(value, 10, 64)
		case "cluster_my_epoch":
			topology.MyEpoch, _ = strconv.ParseInt(value, 10, 64)
		}
	}
}

// GetSlotForKey returns the hash slot for a given key.
func GetSlotForKey(key string) int {
	// Extract hashtag if present
	start := strings.Index(key, "{")
	if start != -1 {
		end := strings.Index(key[start:], "}")
		if end > 1 {
			key = key[start+1 : start+end]
		}
	}

	return int(crc16([]byte(key)) % 16384)
}

// crc16 calculates CRC16 for slot calculation (XMODEM).
func crc16(data []byte) uint16 {
	crc := uint16(0)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// GetNodeForSlot returns the node responsible for a given slot.
func (a *ClusterAnalyzer) GetNodeForSlot(slot int) (*NodeInfo, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.topology == nil {
		return nil, fmt.Errorf("topology not loaded")
	}

	for _, master := range a.topology.Masters {
		for _, sr := range master.Slots {
			if slot >= sr.Start && slot <= sr.End {
				return &master, nil
			}
		}
	}

	return nil, fmt.Errorf("no node found for slot %d", slot)
}

// ClusterBenchmarkConfig configures cluster benchmark settings.
type ClusterBenchmarkConfig struct {
	// ShardAware distributes load across shards based on key distribution
	ShardAware bool `json:"shard_aware"`

	// TargetNodes specifies which nodes to benchmark (empty = all)
	TargetNodes []string `json:"target_nodes,omitempty"`

	// ReplicaReads enables reading from replicas
	ReplicaReads bool `json:"replica_reads"`

	// FailoverTesting enables failover scenario testing
	FailoverTesting bool `json:"failover_testing"`

	// FailoverDelay is the delay between failover operations
	FailoverDelay time.Duration `json:"failover_delay"`
}

// DefaultClusterBenchmarkConfig returns default cluster benchmark config.
func DefaultClusterBenchmarkConfig() ClusterBenchmarkConfig {
	return ClusterBenchmarkConfig{
		ShardAware:      true,
		ReplicaReads:    false,
		FailoverTesting: false,
		FailoverDelay:   10 * time.Second,
	}
}

// SlotDistribution tracks key distribution across slots.
type SlotDistribution struct {
	mu          sync.RWMutex
	counts      map[int]int64
	nodeCount   map[string]int64
}

// NewSlotDistribution creates a new slot distribution tracker.
func NewSlotDistribution() *SlotDistribution {
	return &SlotDistribution{
		counts:    make(map[int]int64),
		nodeCount: make(map[string]int64),
	}
}

// RecordKey records a key access.
func (d *SlotDistribution) RecordKey(key string, nodeAddress string) {
	slot := GetSlotForKey(key)

	d.mu.Lock()
	defer d.mu.Unlock()

	d.counts[slot]++
	d.nodeCount[nodeAddress]++
}

// GetSlotCounts returns the distribution across slots.
func (d *SlotDistribution) GetSlotCounts() map[int]int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make(map[int]int64, len(d.counts))
	for k, v := range d.counts {
		result[k] = v
	}
	return result
}

// GetNodeCounts returns the distribution across nodes.
func (d *SlotDistribution) GetNodeCounts() map[string]int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make(map[string]int64, len(d.nodeCount))
	for k, v := range d.nodeCount {
		result[k] = v
	}
	return result
}

// GetDistributionStats returns distribution statistics.
func (d *SlotDistribution) GetDistributionStats() DistributionStats {
	d.mu.RLock()
	defer d.mu.RUnlock()

	stats := DistributionStats{
		TotalOps:   0,
		NodeOps:    make(map[string]int64),
		SlotGroups: make(map[string]int64),
	}

	for _, count := range d.counts {
		stats.TotalOps += count
	}

	// Node distribution
	for node, count := range d.nodeCount {
		stats.NodeOps[node] = count
	}

	// Group slots into ranges for summary
	for slot, count := range d.counts {
		group := fmt.Sprintf("%d-%d", (slot/1000)*1000, (slot/1000)*1000+999)
		stats.SlotGroups[group] += count
	}

	return stats
}

// DistributionStats contains distribution statistics.
type DistributionStats struct {
	TotalOps   int64            `json:"total_ops"`
	NodeOps    map[string]int64 `json:"node_ops"`
	SlotGroups map[string]int64 `json:"slot_groups"`
}

// FailoverScenario represents a failover test scenario.
type FailoverScenario struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Type        string        `json:"type"` // "manual", "automatic", "crash"
	TargetNode  string        `json:"target_node,omitempty"`
	Duration    time.Duration `json:"duration"`
}

// FailoverTester tests failover scenarios.
type FailoverTester struct {
	analyzer *ClusterAnalyzer
	client   ClusterClient
}

// NewFailoverTester creates a new failover tester.
func NewFailoverTester(client ClusterClient) *FailoverTester {
	return &FailoverTester{
		analyzer: NewClusterAnalyzer(client),
		client:   client,
	}
}

// TriggerManualFailover triggers a manual failover.
func (t *FailoverTester) TriggerManualFailover(ctx context.Context, replicaAddress string) error {
	_, err := t.client.DoOnNode(ctx, replicaAddress, "CLUSTER", "FAILOVER")
	return err
}

// TriggerForceFailover triggers a forced failover.
func (t *FailoverTester) TriggerForceFailover(ctx context.Context, replicaAddress string) error {
	_, err := t.client.DoOnNode(ctx, replicaAddress, "CLUSTER", "FAILOVER", "FORCE")
	return err
}

// WaitForFailover waits for a failover to complete.
func (t *FailoverTester) WaitForFailover(ctx context.Context, expectedMaster string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		topology, err := t.analyzer.GetTopology(ctx)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		for _, master := range topology.Masters {
			if master.Address == expectedMaster || master.ID == expectedMaster {
				return nil
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("failover timeout: %s did not become master", expectedMaster)
}

// RunFailoverScenario runs a failover scenario.
func (t *FailoverTester) RunFailoverScenario(ctx context.Context, scenario FailoverScenario) (*FailoverResult, error) {
	result := &FailoverResult{
		Scenario:  scenario,
		StartTime: time.Now(),
	}

	// Get initial topology
	initialTopology, err := t.analyzer.GetTopology(ctx)
	if err != nil {
		return nil, fmt.Errorf("get initial topology: %w", err)
	}
	result.InitialTopology = initialTopology

	// Find target node
	var targetNode *NodeInfo
	if scenario.TargetNode != "" {
		for _, node := range initialTopology.Nodes {
			if node.Address == scenario.TargetNode || node.ID == scenario.TargetNode {
				targetNode = &node
				break
			}
		}
	} else {
		// Pick a random master with replicas
		for _, master := range initialTopology.Masters {
			if len(master.Replicas) > 0 {
				targetNode = &master
				break
			}
		}
	}

	if targetNode == nil {
		return nil, fmt.Errorf("no suitable target node found")
	}
	result.TargetNode = *targetNode

	// Find a replica for the target master
	var replica *NodeInfo
	for _, r := range initialTopology.Replicas {
		if r.MasterID == targetNode.ID {
			replica = &r
			break
		}
	}

	if replica == nil {
		return nil, fmt.Errorf("no replica found for master %s", targetNode.ID)
	}

	// Trigger failover
	failoverStart := time.Now()
	switch scenario.Type {
	case "manual":
		err = t.TriggerManualFailover(ctx, replica.Address)
	case "force":
		err = t.TriggerForceFailover(ctx, replica.Address)
	default:
		err = t.TriggerManualFailover(ctx, replica.Address)
	}

	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	// Wait for failover
	err = t.WaitForFailover(ctx, replica.Address, scenario.Duration)
	result.FailoverDuration = time.Since(failoverStart)

	if err != nil {
		result.Error = err.Error()
	} else {
		result.Success = true
	}

	// Get final topology
	finalTopology, _ := t.analyzer.GetTopology(ctx)
	result.FinalTopology = finalTopology
	result.EndTime = time.Now()

	return result, err
}

// FailoverResult contains failover test results.
type FailoverResult struct {
	Scenario         FailoverScenario  `json:"scenario"`
	Success          bool              `json:"success"`
	StartTime        time.Time         `json:"start_time"`
	EndTime          time.Time         `json:"end_time"`
	FailoverDuration time.Duration     `json:"failover_duration"`
	TargetNode       NodeInfo          `json:"target_node"`
	InitialTopology  *ClusterTopology  `json:"initial_topology"`
	FinalTopology    *ClusterTopology  `json:"final_topology"`
	Error            string            `json:"error,omitempty"`
}

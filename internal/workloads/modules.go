// Package workloads provides advanced workload definitions for Redis modules.
package workloads

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
)

// ModuleWorkload represents a workload for a Redis module.
type ModuleWorkload interface {
	// Name returns the workload name
	Name() string

	// Description returns the workload description
	Description() string

	// Module returns the Redis module name
	Module() string

	// GenerateOperation generates a single operation
	GenerateOperation(ctx context.Context) Operation

	// Validate checks if the module is available
	Validate(ctx context.Context, client RedisClient) error
}

// Operation represents a Redis operation.
type Operation struct {
	Command string
	Args    []interface{}
}

// RedisClient interface for module operations.
type RedisClient interface {
	Do(ctx context.Context, args ...interface{}) (interface{}, error)
}

// RediSearchWorkload generates RediSearch operations.
type RediSearchWorkload struct {
	config RediSearchConfig
	rng    *rand.Rand
}

// RediSearchConfig configures RediSearch workload.
type RediSearchConfig struct {
	IndexName    string   `json:"index_name"`
	Schema       []string `json:"schema"`
	DocumentCount int     `json:"document_count"`
	QueryTerms   []string `json:"query_terms"`
	ReadRatio    float64  `json:"read_ratio"`  // 0.0-1.0
	WriteRatio   float64  `json:"write_ratio"` // 0.0-1.0
}

// DefaultRediSearchConfig returns default RediSearch configuration.
func DefaultRediSearchConfig() RediSearchConfig {
	return RediSearchConfig{
		IndexName:     "benchmark_idx",
		Schema:        []string{"title", "TEXT", "body", "TEXT", "score", "NUMERIC"},
		DocumentCount: 10000,
		QueryTerms:    []string{"redis", "benchmark", "performance", "test", "data"},
		ReadRatio:     0.7,
		WriteRatio:    0.3,
	}
}

// NewRediSearchWorkload creates a new RediSearch workload.
func NewRediSearchWorkload(config RediSearchConfig) *RediSearchWorkload {
	return &RediSearchWorkload{
		config: config,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (w *RediSearchWorkload) Name() string {
	return "redisearch"
}

func (w *RediSearchWorkload) Description() string {
	return "RediSearch full-text search and indexing workload"
}

func (w *RediSearchWorkload) Module() string {
	return "search"
}

func (w *RediSearchWorkload) GenerateOperation(ctx context.Context) Operation {
	ratio := w.rng.Float64()

	if ratio < w.config.ReadRatio {
		return w.generateSearchOperation()
	}
	return w.generateIndexOperation()
}

func (w *RediSearchWorkload) generateSearchOperation() Operation {
	term := w.config.QueryTerms[w.rng.Intn(len(w.config.QueryTerms))]
	query := fmt.Sprintf("@title:%s|@body:%s", term, term)

	return Operation{
		Command: "FT.SEARCH",
		Args:    []interface{}{w.config.IndexName, query, "LIMIT", 0, 10},
	}
}

func (w *RediSearchWorkload) generateIndexOperation() Operation {
	docID := fmt.Sprintf("doc:%d", w.rng.Intn(w.config.DocumentCount))
	title := w.generateText(5)
	body := w.generateText(50)
	score := w.rng.Float64() * 100

	return Operation{
		Command: "FT.ADD",
		Args: []interface{}{
			w.config.IndexName, docID, 1.0, "FIELDS",
			"title", title,
			"body", body,
			"score", score,
		},
	}
}

func (w *RediSearchWorkload) generateText(words int) string {
	parts := make([]string, words)
	for i := 0; i < words; i++ {
		parts[i] = w.config.QueryTerms[w.rng.Intn(len(w.config.QueryTerms))]
	}
	return strings.Join(parts, " ")
}

func (w *RediSearchWorkload) Validate(ctx context.Context, client RedisClient) error {
	_, err := client.Do(ctx, "FT._LIST")
	if err != nil {
		return fmt.Errorf("RediSearch module not available: %w", err)
	}
	return nil
}

// RedisJSONWorkload generates RedisJSON operations.
type RedisJSONWorkload struct {
	config RedisJSONConfig
	rng    *rand.Rand
}

// RedisJSONConfig configures RedisJSON workload.
type RedisJSONConfig struct {
	KeyPrefix     string  `json:"key_prefix"`
	KeyCount      int     `json:"key_count"`
	DocumentDepth int     `json:"document_depth"`
	ReadRatio     float64 `json:"read_ratio"`
	WriteRatio    float64 `json:"write_ratio"`
	PathRatio     float64 `json:"path_ratio"` // Ratio of path-based operations
}

// DefaultRedisJSONConfig returns default RedisJSON configuration.
func DefaultRedisJSONConfig() RedisJSONConfig {
	return RedisJSONConfig{
		KeyPrefix:     "json:",
		KeyCount:      10000,
		DocumentDepth: 3,
		ReadRatio:     0.6,
		WriteRatio:    0.4,
		PathRatio:     0.5,
	}
}

// NewRedisJSONWorkload creates a new RedisJSON workload.
func NewRedisJSONWorkload(config RedisJSONConfig) *RedisJSONWorkload {
	return &RedisJSONWorkload{
		config: config,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (w *RedisJSONWorkload) Name() string {
	return "redisjson"
}

func (w *RedisJSONWorkload) Description() string {
	return "RedisJSON document storage and manipulation workload"
}

func (w *RedisJSONWorkload) Module() string {
	return "ReJSON"
}

func (w *RedisJSONWorkload) GenerateOperation(ctx context.Context) Operation {
	ratio := w.rng.Float64()

	if ratio < w.config.ReadRatio {
		return w.generateReadOperation()
	}
	return w.generateWriteOperation()
}

func (w *RedisJSONWorkload) generateReadOperation() Operation {
	key := fmt.Sprintf("%s%d", w.config.KeyPrefix, w.rng.Intn(w.config.KeyCount))

	if w.rng.Float64() < w.config.PathRatio {
		path := w.generatePath()
		return Operation{
			Command: "JSON.GET",
			Args:    []interface{}{key, path},
		}
	}

	return Operation{
		Command: "JSON.GET",
		Args:    []interface{}{key},
	}
}

func (w *RedisJSONWorkload) generateWriteOperation() Operation {
	key := fmt.Sprintf("%s%d", w.config.KeyPrefix, w.rng.Intn(w.config.KeyCount))

	if w.rng.Float64() < w.config.PathRatio {
		path := w.generatePath()
		value := w.generateValue()
		return Operation{
			Command: "JSON.SET",
			Args:    []interface{}{key, path, value},
		}
	}

	doc := w.generateDocument()
	jsonDoc, _ := json.Marshal(doc)
	return Operation{
		Command: "JSON.SET",
		Args:    []interface{}{key, "$", string(jsonDoc)},
	}
}

func (w *RedisJSONWorkload) generatePath() string {
	paths := []string{"$.name", "$.value", "$.data.count", "$.nested.field"}
	return paths[w.rng.Intn(len(paths))]
}

func (w *RedisJSONWorkload) generateValue() string {
	values := []interface{}{
		w.rng.Int63(),
		w.rng.Float64() * 1000,
		fmt.Sprintf("value_%d", w.rng.Int()),
		true,
	}
	val, _ := json.Marshal(values[w.rng.Intn(len(values))])
	return string(val)
}

func (w *RedisJSONWorkload) generateDocument() map[string]interface{} {
	return map[string]interface{}{
		"name":      fmt.Sprintf("item_%d", w.rng.Int()),
		"value":     w.rng.Float64() * 1000,
		"timestamp": time.Now().Unix(),
		"data": map[string]interface{}{
			"count": w.rng.Intn(1000),
			"active": w.rng.Float64() > 0.5,
		},
		"nested": map[string]interface{}{
			"field": fmt.Sprintf("nested_%d", w.rng.Int()),
		},
	}
}

func (w *RedisJSONWorkload) Validate(ctx context.Context, client RedisClient) error {
	_, err := client.Do(ctx, "JSON.SET", "__test__", "$", "{}")
	if err != nil {
		return fmt.Errorf("RedisJSON module not available: %w", err)
	}
	client.Do(ctx, "DEL", "__test__")
	return nil
}

// RedisTimeSeriesWorkload generates RedisTimeSeries operations.
type RedisTimeSeriesWorkload struct {
	config RedisTimeSeriesConfig
	rng    *rand.Rand
}

// RedisTimeSeriesConfig configures RedisTimeSeries workload.
type RedisTimeSeriesConfig struct {
	KeyPrefix     string            `json:"key_prefix"`
	KeyCount      int               `json:"key_count"`
	Labels        map[string]string `json:"labels"`
	RetentionMs   int64             `json:"retention_ms"`
	ChunkSize     int64             `json:"chunk_size"`
	ReadRatio     float64           `json:"read_ratio"`
	WriteRatio    float64           `json:"write_ratio"`
	AggregateRatio float64          `json:"aggregate_ratio"`
}

// DefaultRedisTimeSeriesConfig returns default RedisTimeSeries configuration.
func DefaultRedisTimeSeriesConfig() RedisTimeSeriesConfig {
	return RedisTimeSeriesConfig{
		KeyPrefix:      "ts:",
		KeyCount:       100,
		Labels:         map[string]string{"sensor": "temp", "region": "us-west"},
		RetentionMs:    86400000, // 24 hours
		ChunkSize:      4096,
		ReadRatio:      0.5,
		WriteRatio:     0.4,
		AggregateRatio: 0.1,
	}
}

// NewRedisTimeSeriesWorkload creates a new RedisTimeSeries workload.
func NewRedisTimeSeriesWorkload(config RedisTimeSeriesConfig) *RedisTimeSeriesWorkload {
	return &RedisTimeSeriesWorkload{
		config: config,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (w *RedisTimeSeriesWorkload) Name() string {
	return "redistimeseries"
}

func (w *RedisTimeSeriesWorkload) Description() string {
	return "RedisTimeSeries time-series data workload"
}

func (w *RedisTimeSeriesWorkload) Module() string {
	return "timeseries"
}

func (w *RedisTimeSeriesWorkload) GenerateOperation(ctx context.Context) Operation {
	ratio := w.rng.Float64()

	if ratio < w.config.WriteRatio {
		return w.generateAddOperation()
	} else if ratio < w.config.WriteRatio+w.config.AggregateRatio {
		return w.generateAggregateOperation()
	}
	return w.generateRangeOperation()
}

func (w *RedisTimeSeriesWorkload) generateAddOperation() Operation {
	key := fmt.Sprintf("%s%d", w.config.KeyPrefix, w.rng.Intn(w.config.KeyCount))
	timestamp := time.Now().UnixMilli()
	value := w.rng.Float64() * 100

	return Operation{
		Command: "TS.ADD",
		Args:    []interface{}{key, timestamp, value},
	}
}

func (w *RedisTimeSeriesWorkload) generateRangeOperation() Operation {
	key := fmt.Sprintf("%s%d", w.config.KeyPrefix, w.rng.Intn(w.config.KeyCount))
	now := time.Now().UnixMilli()
	fromTime := now - int64(w.rng.Intn(3600))*1000 // Last hour

	return Operation{
		Command: "TS.RANGE",
		Args:    []interface{}{key, fromTime, now},
	}
}

func (w *RedisTimeSeriesWorkload) generateAggregateOperation() Operation {
	key := fmt.Sprintf("%s%d", w.config.KeyPrefix, w.rng.Intn(w.config.KeyCount))
	now := time.Now().UnixMilli()
	fromTime := now - int64(w.rng.Intn(3600))*1000
	aggregations := []string{"AVG", "MIN", "MAX", "SUM", "COUNT"}
	agg := aggregations[w.rng.Intn(len(aggregations))]

	return Operation{
		Command: "TS.RANGE",
		Args:    []interface{}{key, fromTime, now, "AGGREGATION", agg, 60000},
	}
}

func (w *RedisTimeSeriesWorkload) Validate(ctx context.Context, client RedisClient) error {
	_, err := client.Do(ctx, "TS.CREATE", "__ts_test__", "RETENTION", 1000)
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("RedisTimeSeries module not available: %w", err)
	}
	client.Do(ctx, "DEL", "__ts_test__")
	return nil
}

// RedisBloomWorkload generates RedisBloom operations.
type RedisBloomWorkload struct {
	config RedisBloomConfig
	rng    *rand.Rand
}

// RedisBloomConfig configures RedisBloom workload.
type RedisBloomConfig struct {
	BloomPrefix   string  `json:"bloom_prefix"`
	CuckooPrefix  string  `json:"cuckoo_prefix"`
	CMSPrefix     string  `json:"cms_prefix"`
	TopKPrefix    string  `json:"topk_prefix"`
	KeyCount      int     `json:"key_count"`
	ItemCount     int     `json:"item_count"`
	BloomRatio    float64 `json:"bloom_ratio"`
	CuckooRatio   float64 `json:"cuckoo_ratio"`
	CMSRatio      float64 `json:"cms_ratio"`
	TopKRatio     float64 `json:"topk_ratio"`
}

// DefaultRedisBloomConfig returns default RedisBloom configuration.
func DefaultRedisBloomConfig() RedisBloomConfig {
	return RedisBloomConfig{
		BloomPrefix:  "bf:",
		CuckooPrefix: "cf:",
		CMSPrefix:    "cms:",
		TopKPrefix:   "topk:",
		KeyCount:     100,
		ItemCount:    100000,
		BloomRatio:   0.4,
		CuckooRatio:  0.2,
		CMSRatio:     0.2,
		TopKRatio:    0.2,
	}
}

// NewRedisBloomWorkload creates a new RedisBloom workload.
func NewRedisBloomWorkload(config RedisBloomConfig) *RedisBloomWorkload {
	return &RedisBloomWorkload{
		config: config,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (w *RedisBloomWorkload) Name() string {
	return "redisbloom"
}

func (w *RedisBloomWorkload) Description() string {
	return "RedisBloom probabilistic data structures workload"
}

func (w *RedisBloomWorkload) Module() string {
	return "bf"
}

func (w *RedisBloomWorkload) GenerateOperation(ctx context.Context) Operation {
	ratio := w.rng.Float64()
	cumulative := 0.0

	cumulative += w.config.BloomRatio
	if ratio < cumulative {
		return w.generateBloomOperation()
	}

	cumulative += w.config.CuckooRatio
	if ratio < cumulative {
		return w.generateCuckooOperation()
	}

	cumulative += w.config.CMSRatio
	if ratio < cumulative {
		return w.generateCMSOperation()
	}

	return w.generateTopKOperation()
}

func (w *RedisBloomWorkload) generateBloomOperation() Operation {
	key := fmt.Sprintf("%s%d", w.config.BloomPrefix, w.rng.Intn(w.config.KeyCount))
	item := fmt.Sprintf("item_%d", w.rng.Intn(w.config.ItemCount))

	if w.rng.Float64() < 0.5 {
		return Operation{
			Command: "BF.ADD",
			Args:    []interface{}{key, item},
		}
	}
	return Operation{
		Command: "BF.EXISTS",
		Args:    []interface{}{key, item},
	}
}

func (w *RedisBloomWorkload) generateCuckooOperation() Operation {
	key := fmt.Sprintf("%s%d", w.config.CuckooPrefix, w.rng.Intn(w.config.KeyCount))
	item := fmt.Sprintf("item_%d", w.rng.Intn(w.config.ItemCount))

	if w.rng.Float64() < 0.5 {
		return Operation{
			Command: "CF.ADD",
			Args:    []interface{}{key, item},
		}
	}
	return Operation{
		Command: "CF.EXISTS",
		Args:    []interface{}{key, item},
	}
}

func (w *RedisBloomWorkload) generateCMSOperation() Operation {
	key := fmt.Sprintf("%s%d", w.config.CMSPrefix, w.rng.Intn(w.config.KeyCount))
	item := fmt.Sprintf("item_%d", w.rng.Intn(w.config.ItemCount))

	if w.rng.Float64() < 0.5 {
		return Operation{
			Command: "CMS.INCRBY",
			Args:    []interface{}{key, item, w.rng.Intn(10) + 1},
		}
	}
	return Operation{
		Command: "CMS.QUERY",
		Args:    []interface{}{key, item},
	}
}

func (w *RedisBloomWorkload) generateTopKOperation() Operation {
	key := fmt.Sprintf("%s%d", w.config.TopKPrefix, w.rng.Intn(w.config.KeyCount))
	item := fmt.Sprintf("item_%d", w.rng.Intn(w.config.ItemCount))

	if w.rng.Float64() < 0.7 {
		return Operation{
			Command: "TOPK.ADD",
			Args:    []interface{}{key, item},
		}
	}
	return Operation{
		Command: "TOPK.LIST",
		Args:    []interface{}{key},
	}
}

func (w *RedisBloomWorkload) Validate(ctx context.Context, client RedisClient) error {
	_, err := client.Do(ctx, "BF.RESERVE", "__bf_test__", 0.01, 1000)
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("RedisBloom module not available: %w", err)
	}
	client.Do(ctx, "DEL", "__bf_test__")
	return nil
}

// WorkloadRegistry manages available module workloads.
type WorkloadRegistry struct {
	workloads map[string]ModuleWorkload
}

// NewWorkloadRegistry creates a new workload registry.
func NewWorkloadRegistry() *WorkloadRegistry {
	r := &WorkloadRegistry{
		workloads: make(map[string]ModuleWorkload),
	}

	// Register default workloads
	r.Register(NewRediSearchWorkload(DefaultRediSearchConfig()))
	r.Register(NewRedisJSONWorkload(DefaultRedisJSONConfig()))
	r.Register(NewRedisTimeSeriesWorkload(DefaultRedisTimeSeriesConfig()))
	r.Register(NewRedisBloomWorkload(DefaultRedisBloomConfig()))

	return r
}

// Register adds a workload to the registry.
func (r *WorkloadRegistry) Register(w ModuleWorkload) {
	r.workloads[w.Name()] = w
}

// Get retrieves a workload by name.
func (r *WorkloadRegistry) Get(name string) (ModuleWorkload, bool) {
	w, ok := r.workloads[name]
	return w, ok
}

// List returns all registered workload names.
func (r *WorkloadRegistry) List() []string {
	names := make([]string, 0, len(r.workloads))
	for name := range r.workloads {
		names = append(names, name)
	}
	return names
}

// ToWorkload converts a module workload to a domain Workload.
func ToWorkload(w ModuleWorkload) *domain.Workload {
	return &domain.Workload{
		Name:        w.Name(),
		Description: w.Description(),
		Type:        w.Module(),
	}
}

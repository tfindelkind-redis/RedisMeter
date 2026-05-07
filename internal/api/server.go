package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
	"github.com/tfindelkind-redis/redismeter/internal/plugin"
	"github.com/tfindelkind-redis/redismeter/internal/terraform"
)

// Server is the REST API server for RedisMeter.
type Server struct {
	addr    string
	storage plugin.StoragePlugin
	engine  BenchmarkEngine
	mux     *http.ServeMux
	server  *http.Server

	// Active benchmarks
	activeBenchmarks sync.Map // map[string]*ActiveBenchmark

	// WebSocket connections for streaming
	wsConnections sync.Map // map[string][]*websocketConn

	// Infrastructure management
	infraManager *terraform.Manager

	// Active infrastructure provisioning
	activeInfraOps sync.Map // map[string]*ActiveInfraOp

	// Static file serving
	webDir string // Directory containing built frontend

	// Extended features (set via Register* methods)
	logStore          LogStore           // Log storage
	infraProfileStore InfraProfileStore  // Infrastructure profile storage
	bundleProvider    BundleDataProvider // Export/import data provider
	bundleVersion     string             // Application version for bundle manifest
}

// ActiveInfraOp tracks an active infrastructure operation.
type ActiveInfraOp struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Progress  int       `json:"progress"`
	StartTime time.Time `json:"start_time"`
	Cancel    context.CancelFunc
}

// BenchmarkEngine interface for running benchmarks.
type BenchmarkEngine interface {
	Run(ctx context.Context, cfg interface{}) (*domain.BenchmarkRun, error)
}

// ActiveBenchmark tracks a running benchmark.
type ActiveBenchmark struct {
	ID        string           `json:"id"`
	Status    string           `json:"status"`
	Progress  float64          `json:"progress"`
	StartTime time.Time        `json:"start_time"`
	StageKey  string           `json:"stage_key,omitempty"`
	Config    *BenchmarkConfig `json:"config"`
	Cancel    context.CancelFunc
	ExpectedDurationSeconds int64 `json:"expected_duration_seconds,omitempty"`
}

// ServerConfig holds server configuration.
type ServerConfig struct {
	Addr           string
	Storage        plugin.StoragePlugin
	Engine         BenchmarkEngine
	EnableCORS     bool
	AllowedOrigins []string
	APIKey         string // Simple API key auth (optional)
	WebDir         string // Directory containing the built frontend (optional)
}

// NewServer creates a new API server.
func NewServer(cfg ServerConfig) *Server {
	s := &Server{
		addr:    cfg.Addr,
		storage: cfg.Storage,
		engine:  cfg.Engine,
		mux:     http.NewServeMux(),
	}

	// Initialize infrastructure manager
	homeDir, _ := os.UserHomeDir()
	tfDir := filepath.Join(homeDir, ".redismeter", "terraform")
	if infraMgr, err := terraform.NewManager(tfDir); err == nil {
		s.infraManager = infraMgr
	} else {
		log.Printf("Warning: Infrastructure management disabled: %v", err)
	}

	// Setup routes
	s.setupRoutes(cfg)

	return s
}

func (s *Server) setupRoutes(cfg ServerConfig) {
	// Middleware chain
	var handler http.Handler = s.mux

	// Add CORS if enabled
	if cfg.EnableCORS {
		handler = corsMiddleware(handler, cfg.AllowedOrigins)
	}

	// Add API key auth if configured
	if cfg.APIKey != "" {
		handler = apiKeyMiddleware(handler, cfg.APIKey)
	}

	// Add logging
	handler = loggingMiddleware(handler)

	// Store webDir for use in handleRoot
	s.webDir = cfg.WebDir

	// Register routes
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/api/v1/runs", s.handleRuns)
	s.mux.HandleFunc("/api/v1/runs/", s.handleRun)
	s.mux.HandleFunc("/api/v1/baselines", s.handleBaselines)
	s.mux.HandleFunc("/api/v1/baselines/", s.handleBaseline)
	s.mux.HandleFunc("/api/v1/workloads", s.handleWorkloads)
	s.mux.HandleFunc("/api/v1/workloads/", s.handleWorkload)
	s.mux.HandleFunc("/api/v1/run-profiles", s.handleRunProfiles)
	s.mux.HandleFunc("/api/v1/run-profiles/", s.handleRunProfile)
	s.mux.HandleFunc("/api/v1/benchmark", s.handleBenchmark)
	s.mux.HandleFunc("/api/v1/benchmark/", s.handleBenchmarkStatus)
	s.mux.HandleFunc("/api/v1/compare", s.handleCompare)
	s.mux.HandleFunc("/api/v1/analyze", s.handleAnalyze)
	s.mux.HandleFunc("/api/v1/ws", s.handleWebSocket)

	// Infrastructure management routes
	s.mux.HandleFunc("/api/v1/infrastructures", s.handleInfrastructures)
	s.mux.HandleFunc("/api/v1/infrastructures/", s.handleInfrastructure)
	s.mux.HandleFunc("/api/v1/cloud/benchmark", s.handleCloudBenchmark)
	s.mux.HandleFunc("/api/v1/docs", s.handleDocs)
	s.mux.HandleFunc("/api/v1/docs/", s.handleDoc)

	// Root handler - serves API info or static frontend
	s.mux.HandleFunc("/", s.handleRootOrStatic)

	if cfg.WebDir != "" {
		log.Printf("Serving frontend from %s", cfg.WebDir)
	}

	s.server = &http.Server{
		Addr:         s.addr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}

// Start starts the API server.
func (s *Server) Start() error {
	log.Printf("Starting API server on %s", s.addr)
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// --- Route Handlers ---

func (s *Server) handleRootOrStatic(w http.ResponseWriter, r *http.Request) {
	// If no frontend is configured, show API info for root path
	if s.webDir == "" {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		s.handleRoot(w, r)
		return
	}

	// Serve static frontend
	// Try to serve the actual file first
	filePath := filepath.Join(s.webDir, r.URL.Path)
	if stat, err := os.Stat(filePath); err == nil && !stat.IsDir() {
		if strings.HasSuffix(filePath, "index.html") {
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
		}
		http.ServeFile(w, r, filePath)
		return
	}

	// For SPA routing, serve index.html for all unmatched paths
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	http.ServeFile(w, r, filepath.Join(s.webDir, "index.html"))
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"name":    "RedisMeter API",
		"version": "1.0.0",
		"endpoints": map[string]string{
			"health":          "/health",
			"runs":            "/api/v1/runs",
			"baselines":       "/api/v1/baselines",
			"workloads":       "/api/v1/workloads",
			"benchmark":       "/api/v1/benchmark",
			"compare":         "/api/v1/compare",
			"analyze":         "/api/v1/analyze",
			"websocket":       "/api/v1/ws",
			"infrastructures": "/api/v1/infrastructures",
			"cloud_benchmark": "/api/v1/cloud/benchmark",
			"docs":            "/api/v1/docs",
		},
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	// Check storage health
	if s.storage != nil {
		health := s.storage.HealthCheck(r.Context())
		status["storage"] = map[string]interface{}{
			"healthy": health.Healthy,
			"message": health.Message,
		}
	}

	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleRuns(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listRuns(w, r)
	case http.MethodPost:
		s.createRun(w, r)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) listRuns(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters into plugin.QueryFilter
	filter := plugin.QueryFilter{
		Conditions: make(map[string]interface{}),
	}

	if status := r.URL.Query().Get("status"); status != "" {
		filter.Conditions["status"] = status
	}
	if workload := r.URL.Query().Get("workload"); workload != "" {
		filter.Conditions["workload"] = workload
	}
	if target := r.URL.Query().Get("target"); target != "" {
		filter.Conditions["target"] = target
	}
	if tags := r.URL.Query().Get("tags"); tags != "" {
		filter.Tags = strings.Split(tags, ",")
	}
	if limit := r.URL.Query().Get("limit"); limit != "" {
		if n, err := strconv.Atoi(limit); err == nil {
			filter.Limit = n
		}
	}
	if offset := r.URL.Query().Get("offset"); offset != "" {
		if n, err := strconv.Atoi(offset); err == nil {
			filter.Offset = n
		}
	}
	if orderBy := r.URL.Query().Get("order_by"); orderBy != "" {
		filter.OrderBy = orderBy
	}
	if r.URL.Query().Get("desc") == "true" {
		filter.Descending = true
	}

	var runs []*domain.BenchmarkRun
	if err := s.storage.Query(ctx, "runs", filter, &runs); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to query runs: "+err.Error())
		return
	}

	for _, run := range runs {
		s.enrichRunForDisplay(run)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"runs":  runs,
		"count": len(runs),
	})
}

func (s *Server) createRun(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var run domain.BenchmarkRun
	if err := json.NewDecoder(r.Body).Decode(&run); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	id, err := s.storage.Save(ctx, "runs", &run)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to save run: "+err.Error())
		return
	}
	run.ID = id

	writeJSON(w, http.StatusCreated, run)
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	// Extract run ID from path: /api/v1/runs/{id}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/runs/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Missing run ID")
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getRun(w, r, id)
	case http.MethodDelete:
		s.deleteRun(w, r, id)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) getRun(w http.ResponseWriter, r *http.Request, id string) {
	ctx := r.Context()

	var run domain.BenchmarkRun
	if err := s.storage.Load(ctx, "runs", id, &run); err != nil {
		writeError(w, http.StatusNotFound, "Run not found: "+err.Error())
		return
	}

	s.enrichRunForDisplay(&run)

	writeJSON(w, http.StatusOK, run)
}

func (s *Server) enrichRunForDisplay(run *domain.BenchmarkRun) {
	if run == nil {
		return
	}

	if run.CreatedAt.IsZero() {
		if !run.StartTime.IsZero() {
			run.CreatedAt = run.StartTime
		} else {
			run.CreatedAt = time.Now().UTC()
		}
	}

	if run.UpdatedAt.IsZero() {
		switch {
		case !run.EndTime.IsZero():
			run.UpdatedAt = run.EndTime
		default:
			run.UpdatedAt = run.CreatedAt
		}
	}

	if run.Environment == nil {
		run.Environment = deriveEnvironmentFromRunMetadata(run)
	}
}

func deriveEnvironmentFromRunMetadata(run *domain.BenchmarkRun) *domain.Environment {
	if run == nil {
		return nil
	}

	provider := ""
	region := ""
	if run.Labels != nil {
		provider = strings.TrimSpace(run.Labels["provider"])
		region = strings.TrimSpace(run.Labels["region"])
	}

	if provider == "" && region == "" && run.Target == nil {
		return nil
	}

	redisMode := "standalone"
	clusterEnabled := false
	if run.Labels != nil {
		clustering := strings.ToLower(strings.TrimSpace(run.Labels["amr_clustering"]))
		if strings.Contains(clustering, "cluster") && clustering != "non-clustered" {
			redisMode = "cluster"
			clusterEnabled = true
		}
	}

	redisConfig := map[string]string{}
	for _, k := range []string{
		"amr_sku",
		"amr_clustering",
		"amr_high_availability",
		"amr_eviction_policy",
		"amr_modules",
		"amr_access_keys_authentication",
		"amr_entra_authentication",
		"amr_minimum_tls_version",
		"runner_count",
	} {
		if run.Labels != nil {
			if v := strings.TrimSpace(run.Labels[k]); v != "" {
				redisConfig[k] = v
			}
		}
	}

	hostname := ""
	if run.Target != nil {
		hostname = run.Target.Host
	}

	return &domain.Environment{
		Host: &domain.HostInfo{Hostname: hostname},
		Redis: &domain.RedisInfo{
			Mode:           redisMode,
			ClusterEnabled: clusterEnabled,
			Config:         redisConfig,
		},
		Cloud: &domain.CloudEnvironment{
			Provider: provider,
			Region:   region,
		},
	}
}

func (s *Server) deleteRun(w http.ResponseWriter, r *http.Request, id string) {
	ctx := r.Context()

	if err := s.storage.Delete(ctx, "runs", id); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete run: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": id})
}

func (s *Server) handleBaselines(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listBaselines(w, r)
	case http.MethodPost:
		s.createBaseline(w, r)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) listBaselines(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var baselines []*domain.Baseline
	filter := plugin.QueryFilter{}
	if err := s.storage.Query(ctx, "baseline", filter, &baselines); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list baselines: "+err.Error())
		return
	}

	for _, baseline := range baselines {
		s.enrichBaselineFromRun(ctx, baseline)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"baselines": baselines,
		"count":     len(baselines),
	})
}

func (s *Server) createBaseline(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var baseline domain.Baseline
	if err := json.NewDecoder(r.Body).Decode(&baseline); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	now := time.Now().UTC()
	if baseline.CreatedAt.IsZero() {
		baseline.CreatedAt = now
	}
	if baseline.UpdatedAt.IsZero() {
		baseline.UpdatedAt = now
	}

	s.enrichBaselineFromRun(ctx, &baseline)

	id, err := s.storage.Save(ctx, "baseline", &baseline)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to save baseline: "+err.Error())
		return
	}
	baseline.ID = id

	writeJSON(w, http.StatusCreated, baseline)
}

func (s *Server) enrichBaselineFromRun(ctx context.Context, baseline *domain.Baseline) {
	if baseline == nil || baseline.RunID == "" {
		return
	}

	var run domain.BenchmarkRun
	if err := s.storage.Load(ctx, "runs", baseline.RunID, &run); err != nil {
		return
	}

	if baseline.Metrics == nil && run.Results != nil {
		baseline.Metrics = run.Results.Summary
	}
	if baseline.Workload == nil {
		baseline.Workload = run.Workload
	}
	if baseline.CreatedAt.IsZero() {
		switch {
		case !run.CreatedAt.IsZero():
			baseline.CreatedAt = run.CreatedAt
		case !run.StartTime.IsZero():
			baseline.CreatedAt = run.StartTime
		default:
			baseline.CreatedAt = time.Now().UTC()
		}
	}
	if baseline.UpdatedAt.IsZero() {
		baseline.UpdatedAt = baseline.CreatedAt
	}

	if baseline.Labels == nil {
		baseline.Labels = map[string]string{}
	}

	if baseline.Labels["provider"] == "" && run.Labels != nil {
		baseline.Labels["provider"] = run.Labels["provider"]
	}
	if baseline.Labels["region"] == "" && run.Labels != nil {
		baseline.Labels["region"] = run.Labels["region"]
	}
	if baseline.Labels["environment"] == "" {
		baseline.Labels["environment"] = inferBaselineEnvironmentCategory(&run)
	}
}

func inferBaselineEnvironmentCategory(run *domain.BenchmarkRun) string {
	if run == nil {
		return "unknown"
	}

	if run.Labels != nil {
		if provider := strings.TrimSpace(run.Labels["provider"]); provider != "" {
			return provider
		}
		if cloud := strings.TrimSpace(run.Labels["cloud"]); cloud == "true" {
			return "cloud"
		}
	}

	for _, tag := range run.Tags {
		t := strings.ToLower(strings.TrimSpace(tag))
		switch t {
		case "azure", "aws", "gcp", "local":
			return t
		}
	}

	if strings.HasPrefix(run.ID, "cloud-") {
		return "cloud"
	}

	return "local"
}

func (s *Server) handleBaseline(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/baselines/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Missing baseline ID")
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getBaseline(w, r, id)
	case http.MethodDelete:
		s.deleteBaseline(w, r, id)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) getBaseline(w http.ResponseWriter, r *http.Request, id string) {
	ctx := r.Context()

	var baseline domain.Baseline
	if err := s.storage.Load(ctx, "baseline", id, &baseline); err != nil {
		writeError(w, http.StatusNotFound, "Baseline not found: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, baseline)
}

func (s *Server) deleteBaseline(w http.ResponseWriter, r *http.Request, id string) {
	ctx := r.Context()

	if err := s.storage.Delete(ctx, "baseline", id); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete baseline: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": id})
}

func (s *Server) handleWorkloads(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listWorkloads(w, r)
	case http.MethodPost:
		s.createWorkload(w, r)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) listWorkloads(w http.ResponseWriter, r *http.Request) {
	full := r.URL.Query().Get("full") == "true"

	// Get built-in workloads from registry
	builtinWorkloads := []map[string]interface{}{
		{"name": "cache", "description": "Standard cache workload with GET/SET operations", "is_builtin": true,
			"operations": []map[string]interface{}{{"command": "GET", "ratio": 0.8}, {"command": "SET", "ratio": 0.2}},
			"threads":    4, "clients": 50, "duration": "30s", "pipeline": 1,
			"key_pattern": map[string]interface{}{"prefix": "cache:", "pattern": "random", "key_range": 1000000},
			"data_size":   map[string]interface{}{"fixed": 256}},
		{"name": "mixed", "description": "Mixed workload with various operation types", "is_builtin": true,
			"operations": []map[string]interface{}{{"command": "GET", "ratio": 0.5}, {"command": "SET", "ratio": 0.5}},
			"threads":    4, "clients": 50, "duration": "30s", "pipeline": 1,
			"key_pattern": map[string]interface{}{"prefix": "mixed:", "pattern": "random", "key_range": 500000},
			"data_size":   map[string]interface{}{"min": 64, "max": 1024}},
		{"name": "read-heavy", "description": "Read-heavy workload (90% GET, 10% SET)", "is_builtin": true,
			"operations": []map[string]interface{}{{"command": "GET", "ratio": 0.9}, {"command": "SET", "ratio": 0.1}},
			"threads":    4, "clients": 50, "duration": "30s", "pipeline": 1,
			"key_pattern": map[string]interface{}{"prefix": "read:", "pattern": "random", "key_range": 1000000},
			"data_size":   map[string]interface{}{"fixed": 256}},
		{"name": "write-heavy", "description": "Write-heavy workload (10% GET, 90% SET)", "is_builtin": true,
			"operations": []map[string]interface{}{{"command": "GET", "ratio": 0.1}, {"command": "SET", "ratio": 0.9}},
			"threads":    4, "clients": 50, "duration": "30s", "pipeline": 1,
			"key_pattern": map[string]interface{}{"prefix": "write:", "pattern": "random", "key_range": 1000000},
			"data_size":   map[string]interface{}{"fixed": 256}},
		{"name": "pipeline", "description": "Pipeline workload for bulk operations", "is_builtin": true,
			"operations": []map[string]interface{}{{"command": "GET", "ratio": 0.8}, {"command": "SET", "ratio": 0.2}},
			"threads":    4, "clients": 100, "duration": "30s", "pipeline": 10,
			"key_pattern": map[string]interface{}{"prefix": "pipe:", "pattern": "random", "key_range": 1000000},
			"data_size":   map[string]interface{}{"fixed": 100}},
		{"name": "large-values", "description": "Large value workload (1KB-10KB)", "is_builtin": true,
			"operations": []map[string]interface{}{{"command": "GET", "ratio": 0.5}, {"command": "SET", "ratio": 0.5}},
			"threads":    4, "clients": 20, "duration": "30s", "pipeline": 1,
			"key_pattern": map[string]interface{}{"prefix": "large:", "pattern": "random", "key_range": 10000},
			"data_size":   map[string]interface{}{"min": 1024, "max": 10240}},
		{"name": "small-values", "description": "Small value workload (8-64 bytes)", "is_builtin": true,
			"operations": []map[string]interface{}{{"command": "GET", "ratio": 0.8}, {"command": "SET", "ratio": 0.2}},
			"threads":    4, "clients": 50, "duration": "30s", "pipeline": 1,
			"key_pattern": map[string]interface{}{"prefix": "small:", "pattern": "random", "key_range": 1000000},
			"data_size":   map[string]interface{}{"min": 8, "max": 64}},
		{"name": "scan-heavy", "description": "SCAN operation heavy workload", "is_builtin": true,
			"operations": []map[string]interface{}{{"command": "SCAN", "ratio": 0.7}, {"command": "GET", "ratio": 0.3}},
			"threads":    2, "clients": 10, "duration": "30s", "pipeline": 1,
			"key_pattern": map[string]interface{}{"prefix": "scan:", "pattern": "random", "key_range": 100000},
			"data_size":   map[string]interface{}{"fixed": 256}},
		{"name": "huge-read", "description": "100% GET workload with 100KB values", "is_builtin": true,
			"operations": []map[string]interface{}{{"command": "GET", "ratio": 1.0}},
			"threads":    4, "clients": 20, "duration": "30s", "pipeline": 1,
			"key_pattern": map[string]interface{}{"prefix": "huge:", "pattern": "random", "key_range": 10000},
			"data_size":   map[string]interface{}{"fixed": 102400}},
	}

	// Get custom workloads from storage
	customWorkloads := s.getCustomWorkloads()

	// Combine workloads
	allWorkloads := append(builtinWorkloads, customWorkloads...)

	if !full {
		// Return simplified list for backward compatibility
		simplified := make([]map[string]string, len(allWorkloads))
		for i, w := range allWorkloads {
			simplified[i] = map[string]string{
				"name":        w["name"].(string),
				"description": w["description"].(string),
			}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"workloads": simplified})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"workloads": allWorkloads})
}

func (s *Server) getCustomWorkloads() []map[string]interface{} {
	// Load custom workloads from file
	workloadsFile := filepath.Join(os.Getenv("HOME"), ".redismeter", "custom_workloads.json")
	data, err := os.ReadFile(workloadsFile)
	if err != nil {
		return []map[string]interface{}{}
	}

	var workloads []map[string]interface{}
	if err := json.Unmarshal(data, &workloads); err != nil {
		log.Printf("Failed to parse custom workloads: %v", err)
		return []map[string]interface{}{}
	}

	return workloads
}

func (s *Server) saveCustomWorkloads(workloads []map[string]interface{}) error {
	workloadsFile := filepath.Join(os.Getenv("HOME"), ".redismeter", "custom_workloads.json")

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(workloadsFile), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(workloads, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(workloadsFile, data, 0644)
}

func (s *Server) createWorkload(w http.ResponseWriter, r *http.Request) {
	var workload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&workload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	name, ok := workload["name"].(string)
	if !ok || name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	// Check if name conflicts with built-in
	builtinNames := []string{"cache", "mixed", "read-heavy", "write-heavy", "pipeline", "large-values", "small-values", "scan-heavy", "huge-read"}
	for _, bn := range builtinNames {
		if name == bn {
			writeError(w, http.StatusConflict, "Cannot use built-in workload name")
			return
		}
	}

	workload["is_builtin"] = false

	customWorkloads := s.getCustomWorkloads()

	// Check for duplicate
	for _, cw := range customWorkloads {
		if cw["name"] == name {
			writeError(w, http.StatusConflict, "Workload already exists")
			return
		}
	}

	customWorkloads = append(customWorkloads, workload)

	if err := s.saveCustomWorkloads(customWorkloads); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to save workload: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, workload)
}

func (s *Server) handleWorkload(w http.ResponseWriter, r *http.Request) {
	// Extract workload name from path: /api/v1/workloads/{name}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/workloads/")
	name := strings.TrimSuffix(path, "/")

	if name == "" {
		writeError(w, http.StatusBadRequest, "workload name required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getWorkload(w, r, name)
	case http.MethodPut:
		s.updateWorkload(w, r, name)
	case http.MethodDelete:
		s.deleteWorkload(w, r, name)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) getWorkload(w http.ResponseWriter, r *http.Request, name string) {
	customWorkloads := s.getCustomWorkloads()

	for _, cw := range customWorkloads {
		if cw["name"] == name {
			writeJSON(w, http.StatusOK, cw)
			return
		}
	}

	writeError(w, http.StatusNotFound, "Workload not found")
}

func (s *Server) updateWorkload(w http.ResponseWriter, r *http.Request, name string) {
	var workload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&workload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	customWorkloads := s.getCustomWorkloads()

	found := false
	for i, cw := range customWorkloads {
		if cw["name"] == name {
			workload["name"] = name // Preserve name
			workload["is_builtin"] = false
			customWorkloads[i] = workload
			found = true
			break
		}
	}

	if !found {
		writeError(w, http.StatusNotFound, "Workload not found")
		return
	}

	if err := s.saveCustomWorkloads(customWorkloads); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to save workload: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, workload)
}

func (s *Server) deleteWorkload(w http.ResponseWriter, r *http.Request, name string) {
	customWorkloads := s.getCustomWorkloads()

	found := false
	newWorkloads := make([]map[string]interface{}, 0)
	for _, cw := range customWorkloads {
		if cw["name"] == name {
			found = true
			continue
		}
		newWorkloads = append(newWorkloads, cw)
	}

	if !found {
		writeError(w, http.StatusNotFound, "Workload not found")
		return
	}

	if err := s.saveCustomWorkloads(newWorkloads); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to save workloads: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "name": name})
}

// ===================== Run Profile Handlers =====================

func (s *Server) handleRunProfiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listRunProfiles(w, r)
	case http.MethodPost:
		s.createRunProfile(w, r)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) listRunProfiles(w http.ResponseWriter, r *http.Request) {
	full := r.URL.Query().Get("full") == "true"

	// Built-in run profiles
	builtinProfiles := []map[string]interface{}{
		{
			"name": "default", "description": "Default execution profile - balanced settings",
			"is_builtin": true, "threads": 4, "clients": 50, "duration": "30s", "pipeline": 1,
			"run_count": 1, "protocol": "redis",
		},
		{
			"name": "quick-test", "description": "Quick test - short duration for validation",
			"is_builtin": true, "threads": 2, "clients": 10, "duration": "10s", "pipeline": 1,
			"run_count": 1, "protocol": "redis",
		},
		{
			"name": "high-load", "description": "High load test - maximum parallelism",
			"is_builtin": true, "threads": 8, "clients": 100, "duration": "60s", "pipeline": 10,
			"run_count": 1, "protocol": "redis",
		},
		{
			"name": "low-latency", "description": "Low latency measurement - minimal pipelining",
			"is_builtin": true, "threads": 2, "clients": 10, "duration": "30s", "pipeline": 1,
			"run_count": 3, "protocol": "redis",
		},
		{
			"name": "throughput", "description": "Throughput focused - aggressive pipelining",
			"is_builtin": true, "threads": 4, "clients": 100, "duration": "60s", "pipeline": 20,
			"run_count": 1, "protocol": "redis",
		},
		{
			"name": "stress", "description": "Stress test - extended duration with high load",
			"is_builtin": true, "threads": 8, "clients": 200, "duration": "300s", "pipeline": 10,
			"run_count": 1, "protocol": "redis",
		},
		{
			"name": "rate-limited", "description": "Rate limited - controlled request rate",
			"is_builtin": true, "threads": 4, "clients": 50, "duration": "30s", "pipeline": 1,
			"run_count": 1, "rate_limit": 10000, "protocol": "redis",
		},
		{
			"name": "request-based", "description": "Request based - fixed number of requests",
			"is_builtin": true, "threads": 4, "clients": 50, "requests": 100000, "pipeline": 1,
			"run_count": 1, "protocol": "redis",
		},
	}

	// Get custom run profiles
	customProfiles := s.getCustomRunProfiles()
	allProfiles := append(builtinProfiles, customProfiles...)

	if !full {
		simplified := make([]map[string]string, len(allProfiles))
		for i, p := range allProfiles {
			simplified[i] = map[string]string{
				"name":        p["name"].(string),
				"description": p["description"].(string),
			}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"run_profiles": simplified})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"run_profiles": allProfiles})
}

func (s *Server) getCustomRunProfiles() []map[string]interface{} {
	profilesFile := filepath.Join(os.Getenv("HOME"), ".redismeter", "custom_run_profiles.json")
	data, err := os.ReadFile(profilesFile)
	if err != nil {
		return []map[string]interface{}{}
	}

	var profiles []map[string]interface{}
	if err := json.Unmarshal(data, &profiles); err != nil {
		log.Printf("Failed to parse custom run profiles: %v", err)
		return []map[string]interface{}{}
	}

	return profiles
}

func (s *Server) saveCustomRunProfiles(profiles []map[string]interface{}) error {
	profilesFile := filepath.Join(os.Getenv("HOME"), ".redismeter", "custom_run_profiles.json")

	if err := os.MkdirAll(filepath.Dir(profilesFile), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(profilesFile, data, 0644)
}

func (s *Server) createRunProfile(w http.ResponseWriter, r *http.Request) {
	var profile map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	name, ok := profile["name"].(string)
	if !ok || name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	builtinNames := []string{"default", "quick-test", "high-load", "low-latency", "throughput", "stress", "rate-limited", "request-based"}
	for _, bn := range builtinNames {
		if name == bn {
			writeError(w, http.StatusConflict, "Cannot use built-in run profile name")
			return
		}
	}

	profile["is_builtin"] = false
	customProfiles := s.getCustomRunProfiles()

	for _, cp := range customProfiles {
		if cp["name"] == name {
			writeError(w, http.StatusConflict, "Run profile already exists")
			return
		}
	}

	customProfiles = append(customProfiles, profile)

	if err := s.saveCustomRunProfiles(customProfiles); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to save run profile: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, profile)
}

func (s *Server) handleRunProfile(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/run-profiles/")
	name := strings.TrimSuffix(path, "/")

	if name == "" {
		writeError(w, http.StatusBadRequest, "run profile name required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getRunProfile(w, r, name)
	case http.MethodPut:
		s.updateRunProfile(w, r, name)
	case http.MethodDelete:
		s.deleteRunProfile(w, r, name)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) getRunProfile(w http.ResponseWriter, r *http.Request, name string) {
	customProfiles := s.getCustomRunProfiles()

	for _, cp := range customProfiles {
		if cp["name"] == name {
			writeJSON(w, http.StatusOK, cp)
			return
		}
	}

	writeError(w, http.StatusNotFound, "Run profile not found")
}

func (s *Server) updateRunProfile(w http.ResponseWriter, r *http.Request, name string) {
	var profile map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	customProfiles := s.getCustomRunProfiles()

	found := false
	for i, cp := range customProfiles {
		if cp["name"] == name {
			profile["name"] = name
			profile["is_builtin"] = false
			customProfiles[i] = profile
			found = true
			break
		}
	}

	if !found {
		writeError(w, http.StatusNotFound, "Run profile not found")
		return
	}

	if err := s.saveCustomRunProfiles(customProfiles); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to save run profile: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) deleteRunProfile(w http.ResponseWriter, r *http.Request, name string) {
	customProfiles := s.getCustomRunProfiles()

	found := false
	newProfiles := make([]map[string]interface{}, 0)
	for _, cp := range customProfiles {
		if cp["name"] == name {
			found = true
			continue
		}
		newProfiles = append(newProfiles, cp)
	}

	if !found {
		writeError(w, http.StatusNotFound, "Run profile not found")
		return
	}

	if err := s.saveCustomRunProfiles(newProfiles); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to save run profiles: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "name": name})
}

// BenchmarkConfig represents a benchmark configuration.
type BenchmarkConfig struct {
	Workload   string `json:"workload"`
	RunProfile string `json:"run_profile,omitempty"` // Optional run profile name
	Target     string `json:"target"`
	Password   string `json:"password,omitempty"`
	// Legacy fields - kept for backwards compatibility, overridden by run profile if specified
	Duration string   `json:"duration,omitempty"`
	Requests int      `json:"requests,omitempty"`
	Clients  int      `json:"clients,omitempty"`
	Threads  int      `json:"threads,omitempty"`
	Pipeline int      `json:"pipeline,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

func (s *Server) handleBenchmark(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	var cfg BenchmarkConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	// Validate required fields
	if cfg.Workload == "" {
		writeError(w, http.StatusBadRequest, "workload is required")
		return
	}
	if cfg.Target == "" {
		writeError(w, http.StatusBadRequest, "target is required")
		return
	}

	// Generate benchmark ID
	benchID := fmt.Sprintf("bench-%d", time.Now().UnixNano())

	// Create context with cancel
	ctx, cancel := context.WithCancel(r.Context())

	// Track active benchmark
	active := &ActiveBenchmark{
		ID:        benchID,
		Status:    "starting",
		StartTime: time.Now(),
		StageKey:  "starting",
		Config:    &cfg,
		Cancel:    cancel,
	}
	s.activeBenchmarks.Store(benchID, active)

	// Start benchmark in background
	go s.runBenchmark(ctx, benchID, &cfg)

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"id":         benchID,
		"status":     "starting",
		"message":    "Benchmark started",
		"status_url": fmt.Sprintf("/api/v1/benchmark/%s", benchID),
	})
}

func (s *Server) runBenchmark(ctx context.Context, id string, cfg *BenchmarkConfig) {
	active, ok := s.activeBenchmarks.Load(id)
	if !ok {
		return
	}
	ab := active.(*ActiveBenchmark)
	ab.Status = "running"
	ab.StageKey = "running"

	// Run the actual benchmark
	// This would integrate with the engine
	// For now, we'll simulate
	defer func() {
		ab.Status = "completed"
		ab.StageKey = "completed"
	}()

	// Simulate progress updates
	for i := 0; i <= 100; i += 10 {
		select {
		case <-ctx.Done():
			ab.Status = "cancelled"
			ab.StageKey = "cancelled"
			return
		case <-time.After(time.Second):
			ab.Progress = float64(i) / 100.0
			// Broadcast progress to WebSocket clients
			s.broadcastProgress(id, ab.Progress)
		}
	}
}

func (s *Server) handleBenchmarkStatus(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/benchmark/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Missing benchmark ID")
		return
	}

	switch r.Method {
	case http.MethodGet:
		active, ok := s.activeBenchmarks.Load(id)
		if !ok {
			writeError(w, http.StatusNotFound, "Benchmark not found")
			return
		}
		ab := active.(*ActiveBenchmark)
		stage := resolveBenchmarkStage(ab.StageKey, ab.Status)
		elapsedSeconds := int64(time.Since(ab.StartTime).Seconds())
		if elapsedSeconds < 0 {
			elapsedSeconds = 0
		}
		estimatedRemainingSeconds := estimateBenchmarkRemainingSeconds(ab, stage)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"id":         ab.ID,
			"status":     ab.Status,
			"progress":   ab.Progress,
			"start_time": ab.StartTime,
			"stage_key":  stage.Key,
			"stage_label": stage.Label,
			"stage_index": stage.Index,
			"total_stages": stage.Total,
			"elapsed_seconds": elapsedSeconds,
			"expected_duration_seconds": ab.ExpectedDurationSeconds,
			"estimated_remaining_seconds": estimatedRemainingSeconds,
			"config":     ab.Config,
		})

	case http.MethodDelete:
		active, ok := s.activeBenchmarks.Load(id)
		if !ok {
			writeError(w, http.StatusNotFound, "Benchmark not found")
			return
		}
		ab := active.(*ActiveBenchmark)
		ab.Cancel()
		ab.Status = "cancelled"
		ab.StageKey = "cancelled"
		writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled", "id": id})

	default:
		methodNotAllowed(w)
	}
}

// CompareRequest represents a comparison request.
type CompareRequest struct {
	RunID1     string `json:"run_id_1"`
	RunID2     string `json:"run_id_2"`
	BaselineID string `json:"baseline_id,omitempty"`
}

func (s *Server) handleCompare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	var req CompareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	ctx := r.Context()

	// Load runs
	var run1 domain.BenchmarkRun
	if err := s.storage.Load(ctx, "runs", req.RunID1, &run1); err != nil {
		writeError(w, http.StatusNotFound, "Run 1 not found: "+err.Error())
		return
	}

	var run2 domain.BenchmarkRun
	if err := s.storage.Load(ctx, "runs", req.RunID2, &run2); err != nil {
		writeError(w, http.StatusNotFound, "Run 2 not found: "+err.Error())
		return
	}

	// Perform comparison
	comparison := compareRuns(&run1, &run2)
	writeJSON(w, http.StatusOK, comparison)
}

// AnalyzeRequest represents an analysis request.
type AnalyzeRequest struct {
	RunID     string   `json:"run_id"`
	Analyzers []string `json:"analyzers,omitempty"`
}

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	ctx := r.Context()

	var run domain.BenchmarkRun
	if err := s.storage.Load(ctx, "runs", req.RunID, &run); err != nil {
		writeError(w, http.StatusNotFound, "Run not found: "+err.Error())
		return
	}

	// Perform analysis
	analysisResult := analyzeRun(&run, req.Analyzers)
	writeJSON(w, http.StatusOK, analysisResult)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// WebSocket upgrade would happen here
	// For now, return info about WebSocket endpoint
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "WebSocket endpoint",
		"usage":   "Connect via WebSocket for real-time updates",
		"events": []string{
			"benchmark.started",
			"benchmark.progress",
			"benchmark.completed",
			"benchmark.failed",
		},
	})
}

func (s *Server) broadcastProgress(benchID string, progress float64) {
	// Broadcast to all WebSocket connections subscribed to this benchmark
	// Implementation would use actual WebSocket connections
}

// --- Helper Functions ---

func compareRuns(run1, run2 *domain.BenchmarkRun) map[string]interface{} {
	run1Summary := getSummary(run1)
	run2Summary := getSummary(run2)

	throughputChange := 0.0
	if run1Summary.OpsPerSecond > 0 {
		throughputChange = (run2Summary.OpsPerSecond - run1Summary.OpsPerSecond) / run1Summary.OpsPerSecond * 100
	}

	latencyChange := 0.0
	if run1Summary.AvgLatencyMs > 0 {
		latencyChange = (run2Summary.AvgLatencyMs - run1Summary.AvgLatencyMs) / run1Summary.AvgLatencyMs * 100
	}

	workloadBlockers, workloadWarnings, workloadDetails := compareWorkloadCompatibility(run1, run2)
	profileBlockers, profileWarnings, profileDetails := compareExecutionProfileCompatibility(run1, run2)
	targetBlockers, targetWarnings, targetDetails := compareTargetCompatibility(run1, run2)
	envBlockers, envWarnings, envDetails := compareEnvironmentCompatibility(run1, run2)

	blockers := append([]string{}, workloadBlockers...)
	blockers = append(blockers, profileBlockers...)
	blockers = append(blockers, targetBlockers...)
	blockers = append(blockers, envBlockers...)

	warnings := append([]string{}, workloadWarnings...)
	warnings = append(warnings, profileWarnings...)
	warnings = append(warnings, targetWarnings...)
	warnings = append(warnings, envWarnings...)

	comparable := len(blockers) == 0
	quality := "valid"
	if !comparable {
		quality = "invalid"
	} else if len(warnings) > 0 {
		quality = "questionable"
	}

	verdict := "pass"
	if !comparable {
		verdict = "invalid"
	} else if throughputChange < 0 || latencyChange > 0 {
		verdict = "warning"
	}

	return map[string]interface{}{
		"run1_id": run1.ID,
		"run2_id": run2.ID,
		"comparable":          comparable,
		"comparison_quality":  quality,
		"blocking_differences": blockers,
		"warnings":            warnings,
		"verdict":             verdict,
		"compatibility": map[string]interface{}{
			"workload":          workloadDetails,
			"execution_profile": profileDetails,
			"target_infra":      targetDetails,
			"environment":       envDetails,
		},
		"metrics": map[string]interface{}{
			"throughput": map[string]interface{}{
				"run1":       run1Summary.OpsPerSecond,
				"run2":       run2Summary.OpsPerSecond,
				"change_pct": throughputChange,
				"improved":   throughputChange > 0,
			},
			"avg_latency": map[string]interface{}{
				"run1":       run1Summary.AvgLatencyMs,
				"run2":       run2Summary.AvgLatencyMs,
				"change_pct": latencyChange,
				"improved":   latencyChange < 0,
			},
			"p99_latency": map[string]interface{}{
				"run1": run1Summary.P99LatencyMs,
				"run2": run2Summary.P99LatencyMs,
			},
		},
	}
}

func getSummary(run *domain.BenchmarkRun) *domain.SummaryMetrics {
	if run.Results != nil && run.Results.Summary != nil {
		return run.Results.Summary
	}
	return &domain.SummaryMetrics{}
}

func compareWorkloadCompatibility(run1, run2 *domain.BenchmarkRun) ([]string, []string, map[string]interface{}) {
	blockers := []string{}
	warnings := []string{}
	details := map[string]interface{}{}

	w1 := run1.Workload
	w2 := run2.Workload
	if w1 == nil || w2 == nil {
		blockers = append(blockers, "missing workload definition in one or both runs")
		details["status"] = "incompatible"
		details["reason"] = "missing workload definition"
		return blockers, warnings, details
	}

	details["run1_name"] = w1.Name
	details["run2_name"] = w2.Name
	details["run1_type"] = w1.Type
	details["run2_type"] = w2.Type

	if w1.Name != w2.Name {
		blockers = append(blockers, fmt.Sprintf("different workload names: %s vs %s", w1.Name, w2.Name))
	}
	if w1.Type != "" && w2.Type != "" && w1.Type != w2.Type {
		blockers = append(blockers, fmt.Sprintf("different workload types: %s vs %s", w1.Type, w2.Type))
	}

	op1 := operationSignature(w1.Operations)
	op2 := operationSignature(w2.Operations)
	details["run1_operations"] = op1
	details["run2_operations"] = op2
	if !operationSignaturesEqual(op1, op2) {
		blockers = append(blockers, "different workload operations/ratios")
	}

	if len(blockers) > 0 {
		details["status"] = "incompatible"
	} else {
		details["status"] = "compatible"
	}

	return blockers, warnings, details
}

func compareExecutionProfileCompatibility(run1, run2 *domain.BenchmarkRun) ([]string, []string, map[string]interface{}) {
	blockers := []string{}
	warnings := []string{}
	details := map[string]interface{}{}

	p1 := effectiveExecutionProfile(run1)
	p2 := effectiveExecutionProfile(run2)
	details["run1"] = p1
	details["run2"] = p2

	if p1["source"] != p2["source"] {
		warnings = append(warnings, fmt.Sprintf("different profile sources: %v vs %v", p1["source"], p2["source"]))
	}

	compareKeys := []string{"profile_name", "threads", "clients", "pipeline", "duration", "requests", "rate_limit"}
	for _, key := range compareKeys {
		v1, ok1 := p1[key]
		v2, ok2 := p2[key]
		if ok1 != ok2 {
			blockers = append(blockers, fmt.Sprintf("execution profile field %s missing on one run", key))
			continue
		}
		if ok1 && fmt.Sprintf("%v", v1) != fmt.Sprintf("%v", v2) {
			blockers = append(blockers, fmt.Sprintf("different execution profile %s: %v vs %v", key, v1, v2))
		}
	}

	if len(blockers) > 0 {
		details["status"] = "incompatible"
	} else {
		details["status"] = "compatible"
	}

	return blockers, warnings, details
}

func compareTargetCompatibility(run1, run2 *domain.BenchmarkRun) ([]string, []string, map[string]interface{}) {
	blockers := []string{}
	warnings := []string{}
	details := map[string]interface{}{}

	t1 := run1.Target
	t2 := run2.Target
	if t1 == nil || t2 == nil {
		warnings = append(warnings, "missing target metadata in one or both runs")
		details["status"] = "unknown"
		return blockers, warnings, details
	}

	details["run1_host"] = t1.Host
	details["run2_host"] = t2.Host
	details["run1_port"] = t1.Port
	details["run2_port"] = t2.Port
	details["run1_cluster"] = t1.Cluster
	details["run2_cluster"] = t2.Cluster
	details["run1_database"] = t1.Database
	details["run2_database"] = t2.Database

	if t1.Host != t2.Host || t1.Port != t2.Port {
		blockers = append(blockers, "different Redis target endpoint (host/port)")
	}
	if t1.Cluster != t2.Cluster {
		blockers = append(blockers, "different Redis topology (cluster vs standalone)")
	}
	if t1.Database != t2.Database {
		warnings = append(warnings, fmt.Sprintf("different Redis database index: %d vs %d", t1.Database, t2.Database))
	}

	tls1 := false
	tls2 := false
	if t1.TLS != nil {
		tls1 = t1.TLS.Enabled
	}
	if t2.TLS != nil {
		tls2 = t2.TLS.Enabled
	}
	details["run1_tls"] = tls1
	details["run2_tls"] = tls2
	if tls1 != tls2 {
		blockers = append(blockers, "different TLS mode")
	}

	if len(blockers) > 0 {
		details["status"] = "incompatible"
	} else {
		details["status"] = "compatible"
	}

	return blockers, warnings, details
}

func compareEnvironmentCompatibility(run1, run2 *domain.BenchmarkRun) ([]string, []string, map[string]interface{}) {
	blockers := []string{}
	warnings := []string{}
	details := map[string]interface{}{}

	e1 := run1.Environment
	e2 := run2.Environment
	if e1 == nil || e2 == nil {
		warnings = append(warnings, "environment capture missing; hardware/infra comparability cannot be validated")
		details["status"] = "unknown"
		return blockers, warnings, details
	}

	details["run1_fingerprint"] = e1.Fingerprint
	details["run2_fingerprint"] = e2.Fingerprint
	if e1.Fingerprint != "" && e2.Fingerprint != "" && e1.Fingerprint != e2.Fingerprint {
		warnings = append(warnings, "different environment fingerprint")
	}

	if e1.Host != nil && e2.Host != nil {
		details["run1_host"] = e1.Host
		details["run2_host"] = e2.Host
		if e1.Host.CPUs != 0 && e2.Host.CPUs != 0 && e1.Host.CPUs != e2.Host.CPUs {
			blockers = append(blockers, fmt.Sprintf("different CPU count: %d vs %d", e1.Host.CPUs, e2.Host.CPUs))
		}
		if e1.Host.CPUModel != "" && e2.Host.CPUModel != "" && e1.Host.CPUModel != e2.Host.CPUModel {
			blockers = append(blockers, "different CPU model")
		}
		if e1.Host.MemoryGB > 0 && e2.Host.MemoryGB > 0 && e1.Host.MemoryGB != e2.Host.MemoryGB {
			warnings = append(warnings, fmt.Sprintf("different host memory: %.2fGB vs %.2fGB", e1.Host.MemoryGB, e2.Host.MemoryGB))
		}
		if e1.Host.OS != "" && e2.Host.OS != "" && e1.Host.OS != e2.Host.OS {
			warnings = append(warnings, fmt.Sprintf("different host OS: %s vs %s", e1.Host.OS, e2.Host.OS))
		}
	}

	if e1.Redis != nil && e2.Redis != nil {
		details["run1_redis"] = map[string]interface{}{"version": e1.Redis.Version, "mode": e1.Redis.Mode}
		details["run2_redis"] = map[string]interface{}{"version": e2.Redis.Version, "mode": e2.Redis.Mode}
		if e1.Redis.Version != "" && e2.Redis.Version != "" && e1.Redis.Version != e2.Redis.Version {
			blockers = append(blockers, fmt.Sprintf("different Redis version: %s vs %s", e1.Redis.Version, e2.Redis.Version))
		}
		if e1.Redis.Mode != "" && e2.Redis.Mode != "" && e1.Redis.Mode != e2.Redis.Mode {
			blockers = append(blockers, fmt.Sprintf("different Redis mode: %s vs %s", e1.Redis.Mode, e2.Redis.Mode))
		}
	}

	if e1.Cloud != nil && e2.Cloud != nil {
		details["run1_cloud"] = e1.Cloud
		details["run2_cloud"] = e2.Cloud
		if e1.Cloud.Provider != "" && e2.Cloud.Provider != "" && e1.Cloud.Provider != e2.Cloud.Provider {
			blockers = append(blockers, fmt.Sprintf("different cloud provider: %s vs %s", e1.Cloud.Provider, e2.Cloud.Provider))
		}
		if e1.Cloud.InstanceType != "" && e2.Cloud.InstanceType != "" && e1.Cloud.InstanceType != e2.Cloud.InstanceType {
			blockers = append(blockers, fmt.Sprintf("different instance type: %s vs %s", e1.Cloud.InstanceType, e2.Cloud.InstanceType))
		}
	}

	if len(blockers) > 0 {
		details["status"] = "incompatible"
	} else if len(warnings) > 0 {
		details["status"] = "warning"
	} else {
		details["status"] = "compatible"
	}

	return blockers, warnings, details
}

func operationSignature(ops []domain.Operation) map[string]float64 {
	result := map[string]float64{}
	for _, op := range ops {
		result[strings.ToUpper(op.Command)] += op.Ratio
	}
	return result
}

func operationSignaturesEqual(a, b map[string]float64) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || v != bv {
			return false
		}
	}
	return true
}

func effectiveExecutionProfile(run *domain.BenchmarkRun) map[string]interface{} {
	profile := map[string]interface{}{}
	if run.RunProfile != nil {
		profile["source"] = "run_profile"
		profile["profile_name"] = run.RunProfile.Name
		profile["threads"] = run.RunProfile.Threads
		profile["clients"] = run.RunProfile.Clients
		profile["pipeline"] = run.RunProfile.Pipeline
		if run.RunProfile.Duration != "" {
			profile["duration"] = run.RunProfile.Duration
		}
		if run.RunProfile.Requests > 0 {
			profile["requests"] = run.RunProfile.Requests
		}
		if run.RunProfile.RateLimit > 0 {
			profile["rate_limit"] = run.RunProfile.RateLimit
		}
		return profile
	}

	profile["source"] = "workload_legacy"
	if run.Workload != nil {
		if run.Workload.Threads > 0 {
			profile["threads"] = run.Workload.Threads
		}
		if run.Workload.Clients > 0 {
			profile["clients"] = run.Workload.Clients
		}
		if run.Workload.Pipeline > 0 {
			profile["pipeline"] = run.Workload.Pipeline
		}
		if run.Workload.Duration != "" {
			profile["duration"] = run.Workload.Duration
		}
		if run.Workload.Requests > 0 {
			profile["requests"] = run.Workload.Requests
		}
		if run.Workload.RateLimiting != nil && run.Workload.RateLimiting.RequestsPerSecond > 0 {
			profile["rate_limit"] = run.Workload.RateLimiting.RequestsPerSecond
		}
	}
	return profile
}

func analyzeRun(run *domain.BenchmarkRun, analyzers []string) map[string]interface{} {
	summary := getSummary(run)

	analysisResult := map[string]interface{}{
		"run_id": run.ID,
		"summary": map[string]interface{}{
			"throughput":  summary.OpsPerSecond,
			"avg_latency": summary.AvgLatencyMs,
			"p99_latency": summary.P99LatencyMs,
		},
		"recommendations": []string{},
	}

	// Add basic recommendations based on metrics
	recommendations := []string{}

	if summary.P99LatencyMs > 10*summary.AvgLatencyMs && summary.AvgLatencyMs > 0 {
		recommendations = append(recommendations, "High latency variance detected - consider investigating tail latency")
	}

	if summary.OpsPerSecond < 10000 && summary.OpsPerSecond > 0 {
		recommendations = append(recommendations, "Low throughput - consider increasing clients or pipeline depth")
	}

	analysisResult["recommendations"] = recommendations

	return analysisResult
}

// --- Middleware ---

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func corsMiddleware(next http.Handler, origins []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Check if origin is allowed
		allowed := len(origins) == 0 || contains(origins, "*") || contains(origins, origin)

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func apiKeyMiddleware(next http.Handler, apiKey string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health check
		if r.URL.Path == "/health" || r.URL.Path == "/" {
			next.ServeHTTP(w, r)
			return
		}

		key := r.Header.Get("X-API-Key")
		if key == "" {
			key = r.URL.Query().Get("api_key")
		}

		if key != apiKey {
			writeError(w, http.StatusUnauthorized, "Invalid or missing API key")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// --- Response Helpers ---

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// --- Infrastructure Handlers ---

// InfraCreateRequest is the request body for creating infrastructure.
type InfraCreateRequest struct {
	Name          string             `json:"name"`
	Provider      string             `json:"provider"`
	Region        string             `json:"region"`
	TTL           string             `json:"ttl,omitempty"`
	Tags          map[string]string  `json:"tags,omitempty"`
	AMR           *InfraAMRConfig    `json:"amr,omitempty"`
	Runners       *InfraRunnerConfig `json:"runners,omitempty"`
	SelfManaged   *SelfManagedConfig `json:"self_managed,omitempty"`
	EncryptionKey string             `json:"encryption_key,omitempty"` // For decrypting self-managed credentials
}

// SelfManagedConfig holds configuration for self-managed (BYOI) infrastructure.
type SelfManagedConfig struct {
	Runners   SelfManagedRunners `json:"runners"`
	Redis     SelfManagedRedis   `json:"redis"`
	ToolPaths map[string]string  `json:"tool_paths,omitempty"`
}

// SelfManagedRunners holds runner machine configuration.
type SelfManagedRunners struct {
	Machines []RunnerMachine `json:"machines"`
	SSH      SSHConfig       `json:"ssh"`
}

// RunnerMachine represents a single runner machine.
type RunnerMachine struct {
	Name   string            `json:"name,omitempty"`
	Host   string            `json:"host"`
	Port   int               `json:"port,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}

// SSHConfig holds SSH authentication configuration (credentials are encrypted).
type SSHConfig struct {
	AuthMethod            string `json:"auth_method"` // "password", "key", or "key_file"
	Username              string `json:"username"`
	Password              string `json:"password,omitempty"`         // Encrypted
	PrivateKey            string `json:"private_key,omitempty"`      // Encrypted
	PrivateKeyPath        string `json:"private_key_path,omitempty"` // Path on server
	Passphrase            string `json:"passphrase,omitempty"`       // Encrypted
	Port                  int    `json:"port,omitempty"`             // Default: 22
	ConnectTimeout        int    `json:"connect_timeout,omitempty"`  // Default: 30
	StrictHostKeyChecking bool   `json:"strict_host_key_checking,omitempty"`
}

// SelfManagedRedis holds Redis target configuration.
type SelfManagedRedis struct {
	Targets     []RedisTarget    `json:"targets"`
	Credentials RedisCredentials `json:"credentials"`
}

// RedisTarget represents a Redis instance to benchmark.
type RedisTarget struct {
	Name         string   `json:"name,omitempty"`
	Host         string   `json:"host"`
	Port         int      `json:"port"`
	IsCluster    bool     `json:"is_cluster,omitempty"`
	ClusterNodes []string `json:"cluster_nodes,omitempty"`
}

// RedisCredentials holds Redis authentication (password is encrypted).
type RedisCredentials struct {
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"` // Encrypted
	TLSEnabled    bool   `json:"tls_enabled,omitempty"`
	TLSSkipVerify bool   `json:"tls_skip_verify,omitempty"`
	TLSCert       string `json:"tls_cert,omitempty"`
	TLSKey        string `json:"tls_key,omitempty"` // Encrypted
	TLSCA         string `json:"tls_ca,omitempty"`
}

// InfraAMRConfig is the AMR configuration for infrastructure.
type InfraAMRConfig struct {
	SKU              string   `json:"sku"`
	Modules          []string `json:"modules,omitempty"`
	HighAvailability bool     `json:"high_availability"`
	ClusteringPolicy string   `json:"clustering_policy"`
	EvictionPolicy   string   `json:"eviction_policy"`
}

// InfraRunnerConfig is the runner configuration for infrastructure.
type InfraRunnerConfig struct {
	Count         int    `json:"count"`
	InstanceType  string `json:"instance_type"`
	SpotInstances bool   `json:"spot_instances"`
	SSHPublicKey  string `json:"ssh_public_key"`
	SSHUser       string `json:"ssh_user"`
}

// InfraResponse is the response for infrastructure operations.
type InfraResponse struct {
	ID        string              `json:"id"`
	Name      string              `json:"name"`
	Status    string              `json:"status"`
	Provider  string              `json:"provider"`
	Region    string              `json:"region"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`
	ExpiresAt *time.Time          `json:"expires_at,omitempty"`
	Outputs   *InfraOutputs       `json:"outputs,omitempty"`
	Error     string              `json:"error,omitempty"`
	Config    *InfraCreateRequest `json:"config,omitempty"`
}

// InfraOutputs contains the infrastructure outputs.
type InfraOutputs struct {
	RedisHostname     string   `json:"redis_hostname,omitempty"`
	RedisPort         int      `json:"redis_port,omitempty"`
	RedisPrimaryKey   string   `json:"redis_primary_key,omitempty"`
	ResourceGroupName string   `json:"resource_group_name,omitempty"`
	ClusterID         string   `json:"cluster_id,omitempty"`
	RunnerIPs         []string `json:"runner_ips,omitempty"`
	RunnerPrivateIPs  []string `json:"runner_private_ips,omitempty"`
}

func (s *Server) handleInfrastructures(w http.ResponseWriter, r *http.Request) {
	// Check if infrastructure management is available
	if s.infraManager == nil {
		writeError(w, http.StatusServiceUnavailable, "Infrastructure management not available (terraform not found)")
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.listInfrastructures(w, r)
	case http.MethodPost:
		s.createInfrastructure(w, r)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) listInfrastructures(w http.ResponseWriter, r *http.Request) {
	infras, err := s.infraManager.ListInfrastructure()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list infrastructures: "+err.Error())
		return
	}

	// Convert to response format
	responses := make([]InfraResponse, 0, len(infras))
	for _, infra := range infras {
		resp := infraStateToResponse(infra)
		responses = append(responses, resp)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"infrastructures": responses,
		"count":           len(responses),
	})
}

func (s *Server) createInfrastructure(w http.ResponseWriter, r *http.Request) {
	var req InfraCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	// Validate required fields
	if req.Provider == "" {
		writeError(w, http.StatusBadRequest, "provider is required")
		return
	}
	if req.Region == "" {
		writeError(w, http.StatusBadRequest, "region is required")
		return
	}

	// Handle self-managed infrastructure specially
	if req.Provider == "self_managed" {
		s.createSelfManagedInfrastructure(w, req)
		return
	}

	// Build terraform config
	config := terraform.InfraConfig{
		Name:     req.Name,
		Provider: req.Provider,
		Region:   req.Region,
		Tags:     req.Tags,
	}

	// Parse TTL
	if req.TTL != "" {
		ttl, err := time.ParseDuration(req.TTL)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid TTL format: "+err.Error())
			return
		}
		config.TTL = ttl
	}

	// AMR config
	if req.AMR != nil {
		config.AMR = &terraform.AMRConfig{
			SKU:              req.AMR.SKU,
			Modules:          req.AMR.Modules,
			HighAvailability: req.AMR.HighAvailability,
			ClusteringPolicy: req.AMR.ClusteringPolicy,
			EvictionPolicy:   req.AMR.EvictionPolicy,
		}
	}

	// Runner config
	if req.Runners != nil {
		// If no SSH key provided, try to read from default location
		sshKey := req.Runners.SSHPublicKey
		if sshKey == "" {
			homeDir, _ := os.UserHomeDir()
			sshKeyPath := filepath.Join(homeDir, ".ssh", "id_ed25519.pub")
			if keyData, err := os.ReadFile(sshKeyPath); err == nil {
				sshKey = strings.TrimSpace(string(keyData))
			} else {
				// Try RSA key
				sshKeyPath = filepath.Join(homeDir, ".ssh", "id_rsa.pub")
				if keyData, err := os.ReadFile(sshKeyPath); err == nil {
					sshKey = strings.TrimSpace(string(keyData))
				}
			}
		}

		config.Runners = &terraform.RunnerConfig{
			Count:         req.Runners.Count,
			InstanceType:  req.Runners.InstanceType,
			SpotInstances: req.Runners.SpotInstances,
			SSHPublicKey:  sshKey,
			SSHUser:       req.Runners.SSHUser,
		}
		if config.Runners.SSHUser == "" {
			config.Runners.SSHUser = "azureuser"
		}
		if config.Runners.Count == 0 {
			config.Runners.Count = 1
		}
	}

	// Start provisioning in background
	ctx, cancel := context.WithCancel(context.Background())

	// Track the operation
	op := &ActiveInfraOp{
		ID:        fmt.Sprintf("pending-%d", time.Now().Unix()),
		Status:    "provisioning",
		Progress:  0,
		StartTime: time.Now(),
		Cancel:    cancel,
	}

	// Start provisioning asynchronously
	go func() {
		state, err := s.infraManager.ProvisionWithProgress(ctx, config, func(event terraform.TerraformEvent) {
			// Update progress based on events
			if opVal, ok := s.activeInfraOps.Load(op.ID); ok {
				activeOp := opVal.(*ActiveInfraOp)
				switch event.Phase {
				case "init":
					activeOp.Progress = 10
				case "apply":
					if event.Total > 0 {
						activeOp.Progress = 20 + (70 * event.Completed / event.Total)
					} else {
						activeOp.Progress = 20
					}
				}
			}
		})

		// Remove from active ops
		s.activeInfraOps.Delete(op.ID)

		if err != nil {
			log.Printf("Infrastructure provisioning failed: %v", err)
		} else {
			log.Printf("Infrastructure %s provisioned successfully", state.ID)
		}
	}()

	// Store the operation
	s.activeInfraOps.Store(op.ID, op)

	// Return immediately with pending status
	// The client should poll for status
	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"id":      op.ID,
		"status":  "provisioning",
		"message": "Infrastructure provisioning started. Poll /api/v1/infrastructures for status.",
	})
}

// createSelfManagedInfrastructure handles self-managed (BYOI) infrastructure registration.
// Unlike cloud providers, self-managed infra doesn't require provisioning - it's immediately ready.
func (s *Server) createSelfManagedInfrastructure(w http.ResponseWriter, req InfraCreateRequest) {
	// Validate self-managed config
	if req.SelfManaged == nil {
		writeError(w, http.StatusBadRequest, "self_managed configuration is required for self-managed provider")
		return
	}

	if len(req.SelfManaged.Runners.Machines) == 0 {
		writeError(w, http.StatusBadRequest, "at least one runner machine is required")
		return
	}

	if len(req.SelfManaged.Redis.Targets) == 0 {
		writeError(w, http.StatusBadRequest, "at least one Redis target is required")
		return
	}

	// Validate SSH credentials
	ssh := req.SelfManaged.Runners.SSH
	if ssh.Username == "" {
		writeError(w, http.StatusBadRequest, "SSH username is required")
		return
	}

	switch ssh.AuthMethod {
	case "password":
		if ssh.Password == "" {
			writeError(w, http.StatusBadRequest, "SSH password is required for password authentication")
			return
		}
	case "key":
		if ssh.PrivateKey == "" {
			writeError(w, http.StatusBadRequest, "SSH private key is required for key authentication")
			return
		}
	case "key_file":
		if ssh.PrivateKeyPath == "" {
			writeError(w, http.StatusBadRequest, "SSH private key path is required for key_file authentication")
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "Invalid SSH auth method: "+ssh.AuthMethod)
		return
	}

	// Generate unique ID
	id := fmt.Sprintf("rm-self-%s-%d", strings.ReplaceAll(req.Name, " ", "-"), time.Now().Unix())

	// Create state - self-managed infra is immediately ready
	now := time.Now()
	state := &terraform.InfraState{
		ID:        id,
		Name:      req.Name,
		Status:    "ready", // Immediately ready - no provisioning needed
		Provider:  "self_managed",
		Region:    "custom",
		CreatedAt: now,
		UpdatedAt: now,
		Config: terraform.InfraConfig{
			Name:     req.Name,
			Provider: "self_managed",
			Region:   "custom",
			Tags:     req.Tags,
		},
		Outputs: &terraform.InfraOutputs{
			// For self-managed, we populate outputs from the provided config
			RedisHostname:    req.SelfManaged.Redis.Targets[0].Host,
			RedisPort:        req.SelfManaged.Redis.Targets[0].Port,
			RunnerIPs:        make([]string, 0, len(req.SelfManaged.Runners.Machines)),
			RunnerPrivateIPs: make([]string, 0, len(req.SelfManaged.Runners.Machines)),
		},
	}

	// Populate runner IPs
	for _, machine := range req.SelfManaged.Runners.Machines {
		state.Outputs.RunnerIPs = append(state.Outputs.RunnerIPs, machine.Host)
		state.Outputs.RunnerPrivateIPs = append(state.Outputs.RunnerPrivateIPs, machine.Host)
	}

	// Store the state
	if s.infraManager != nil {
		if err := s.infraManager.CreateSelfManagedState(state, req.SelfManaged, req.EncryptionKey); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to store infrastructure state: "+err.Error())
			return
		}
	}

	// Return success immediately
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":      id,
		"name":    req.Name,
		"status":  "ready",
		"message": "Self-managed infrastructure registered successfully",
		"outputs": state.Outputs,
	})
}

func (s *Server) handleInfrastructure(w http.ResponseWriter, r *http.Request) {
	if s.infraManager == nil {
		writeError(w, http.StatusServiceUnavailable, "Infrastructure management not available")
		return
	}

	// Extract ID from path: /api/v1/infrastructures/{id}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/infrastructures/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Missing infrastructure ID")
		return
	}

	// Handle sub-routes
	parts := strings.SplitN(id, "/", 2)
	id = parts[0]

	switch r.Method {
	case http.MethodGet:
		s.getInfrastructure(w, r, id)
	case http.MethodDelete:
		s.destroyInfrastructure(w, r, id)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) getInfrastructure(w http.ResponseWriter, r *http.Request, id string) {
	state, err := s.infraManager.GetState(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Infrastructure not found: "+err.Error())
		return
	}

	resp := infraStateToResponse(state)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) destroyInfrastructure(w http.ResponseWriter, r *http.Request, id string) {
	// Start destroy in background
	ctx := context.Background()

	go func() {
		err := s.infraManager.DestroyWithProgress(ctx, id, func(event terraform.TerraformEvent) {
			log.Printf("Destroy progress: %s - %s", event.Phase, event.Message)
		})
		if err != nil {
			log.Printf("Infrastructure destroy failed: %v", err)
		} else {
			log.Printf("Infrastructure %s destroyed successfully", id)
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"id":      id,
		"status":  "destroying",
		"message": "Infrastructure destroy started",
	})
}

func (s *Server) handleCloudBenchmark(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	// Parse request
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Failed to read request body: "+err.Error())
		return
	}

	var req CloudBenchmarkRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Defensive fallback: capture fields from raw JSON if strict decoding paths miss them.
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Description) == "" || len(req.Tags) == 0 {
		var raw map[string]interface{}
		if err := json.Unmarshal(body, &raw); err == nil {
			if strings.TrimSpace(req.Name) == "" {
				if v, ok := raw["name"].(string); ok {
					req.Name = strings.TrimSpace(v)
				}
			}
			if strings.TrimSpace(req.Description) == "" {
				if v, ok := raw["description"].(string); ok {
					req.Description = strings.TrimSpace(v)
				}
			}
			if len(req.Tags) == 0 {
				if rawTags, ok := raw["tags"].([]interface{}); ok {
					tags := make([]string, 0, len(rawTags))
					for _, item := range rawTags {
						if tag, ok := item.(string); ok {
							tag = strings.TrimSpace(tag)
							if tag != "" {
								tags = append(tags, tag)
							}
						}
					}
					req.Tags = tags
				}
			}
		}
	}

	// Validate required fields
	if req.InfrastructureID == "" {
		writeError(w, http.StatusBadRequest, "infrastructure_id is required")
		return
	}
	if req.Workload == "" {
		req.Workload = "cache" // default workload
	}

	// Get infrastructure
	if s.infraManager == nil {
		writeError(w, http.StatusServiceUnavailable, "Infrastructure manager not available")
		return
	}

	infraStates, err := s.infraManager.ListInfrastructure()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list infrastructure: "+err.Error())
		return
	}

	var infraState *terraform.InfraState
	for _, state := range infraStates {
		if state.ID == req.InfrastructureID {
			infraState = state
			break
		}
	}

	if infraState == nil {
		writeError(w, http.StatusNotFound, "Infrastructure not found: "+req.InfrastructureID)
		return
	}

	if infraState.Status != "ready" {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Infrastructure is not ready (status: %s)", infraState.Status))
		return
	}

	// Check if we have runner IPs
	if infraState.Outputs == nil || len(infraState.Outputs.RunnerIPs) == 0 {
		writeError(w, http.StatusBadRequest, "Infrastructure has no runner VMs")
		return
	}

	// Generate benchmark ID
	benchmarkID := fmt.Sprintf("cloud-%d", time.Now().UnixNano())

	// Start benchmark in background
	ctx, cancel := context.WithCancel(context.Background())

	// Store active benchmark
	s.activeBenchmarks.Store(benchmarkID, &ActiveBenchmark{
		ID:        benchmarkID,
		Status:    "running",
		StartTime: time.Now(),
		StageKey:  "starting",
		Cancel:    cancel,
		ExpectedDurationSeconds: int64(computeExpectedBenchmarkDuration(req.Duration).Seconds()),
	})

	// Start benchmark goroutine
	go s.runCloudBenchmark(ctx, benchmarkID, infraState, req)

	// Return accepted with benchmark ID
	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"benchmark_id":      benchmarkID,
		"infrastructure_id": req.InfrastructureID,
		"status":            "running",
		"message":           "Cloud benchmark started",
	})
}

// CloudBenchmarkRequest represents a request to run a benchmark on cloud infrastructure.
type CloudBenchmarkRequest struct {
	InfrastructureID string `json:"infrastructure_id"`
	Name             string `json:"name,omitempty"`
	Description      string `json:"description,omitempty"`
	Workload         string `json:"workload"`
	Clients          int    `json:"clients,omitempty"`
	Threads          int    `json:"threads,omitempty"`
	Duration         string `json:"duration,omitempty"`
	Requests         int    `json:"requests,omitempty"`
	Pipeline         int    `json:"pipeline,omitempty"`
	Tags             []string `json:"tags,omitempty"`
}

// runCloudBenchmark executes a benchmark on cloud infrastructure in the background.
func (s *Server) runCloudBenchmark(ctx context.Context, benchmarkID string, state *terraform.InfraState, req CloudBenchmarkRequest) {
	startTime := time.Now()
	sshCommandTimeout := computeCloudBenchmarkTimeout(req.Duration)

	// Update status helper
	updateStatus := func(status string, stageKey string, progress float64) {
		if ab, ok := s.activeBenchmarks.Load(benchmarkID); ok {
			active := ab.(*ActiveBenchmark)
			active.Status = status
			active.StageKey = stageKey
			active.Progress = progress
		}
	}

	// Create cleanup function
	defer func() {
		if r := recover(); r != nil {
			updateStatus("failed", "failed", 0)
			log.Printf("Cloud benchmark %s panicked: %v", benchmarkID, r)
		}
	}()

	// Build target from infrastructure
	target := &domain.Target{
		Host:     state.Outputs.RedisHostname,
		Port:     state.Outputs.RedisPort,
		Password: state.Outputs.RedisPrimaryKey,
		TLS:      &domain.TLSConfig{Enabled: true, InsecureSkipVerify: true},
	}

	// Determine clustering from config
	if state.Config.AMR != nil && state.Config.AMR.ClusteringPolicy == "OSSCluster" {
		target.Cluster = true
	}

	// Load workload (for now, use defaults)
	wl := &domain.Workload{
		Name:     req.Workload,
		Clients:  50,
		Threads:  4,
		Requests: 100000,
	}
	if req.Clients > 0 {
		wl.Clients = req.Clients
	}
	if req.Threads > 0 {
		wl.Threads = req.Threads
	}
	if strings.TrimSpace(req.Duration) != "" {
		wl.Duration = strings.TrimSpace(req.Duration)
		wl.Requests = 0
	}
	if req.Requests > 0 {
		wl.Requests = int64(req.Requests)
		wl.Duration = ""
	}

	// Set up default operations
	wl.Operations = []domain.Operation{
		{Command: "SET", Ratio: 0.2},
		{Command: "GET", Ratio: 0.8},
	}

	updateStatus("connecting", "connecting", 10)

	// Get SSH user
	sshUser := "azureuser"
	if state.Config.Runners != nil && state.Config.Runners.SSHUser != "" {
		sshUser = state.Config.Runners.SSHUser
	}

	runnerIPs := state.Outputs.RunnerIPs
	log.Printf("Starting cloud benchmark %s on %d runners", benchmarkID, len(runnerIPs))

	// Build memtier command with JSON output
	jsonOutFile := "/tmp/memtier_results.json"
	memtierLogFile := "/tmp/memtier_output.log"
	memtierArgs := buildCloudMemtierCommand(wl, target, jsonOutFile)
	memtierCmd := "memtier_benchmark " + strings.Join(memtierArgs, " ")

	// Redirect memtier stdout/stderr to a log file so the only SSH output is the JSON.
	// This avoids marker extraction problems with large/interleaved output.
	syncCmd := fmt.Sprintf(`
		%s > %s 2>&1
		cat %s 2>/dev/null || echo "{}"
	`, memtierCmd, memtierLogFile, jsonOutFile)

	updateStatus("running", "executing", 30)

	// Run on all runners in parallel
	type runnerResult struct {
		ip       string
		jsonData []byte
		err      error
	}

	resultsCh := make(chan runnerResult, len(runnerIPs))
	var wg sync.WaitGroup

	for _, ip := range runnerIPs {
		wg.Add(1)
		go func(runnerIP string) {
			defer wg.Done()
			runnerCtx, cancel := context.WithTimeout(ctx, sshCommandTimeout)
			defer cancel()

			output, err := runSSHCommand(runnerCtx, runnerIP, sshUser, syncCmd)

			var jsonData []byte
			if err == nil {
				// The SSH command outputs JSON file content directly (no markers).
				// Try direct parse first; then extract a JSON object from mixed output;
				// finally fall back to marker extraction for compatibility.
				trimmed := strings.TrimSpace(output)
				if strings.HasPrefix(trimmed, "{") {
					jsonData = []byte(trimmed)
				} else {
					start := strings.Index(trimmed, "{")
					end := strings.LastIndex(trimmed, "}")
					if start >= 0 && end > start {
						jsonData = []byte(trimmed[start : end+1])
					} else {
						jsonData = extractJSONFromOutput(output)
					}
				}
			}

			resultsCh <- runnerResult{
				ip:       runnerIP,
				jsonData: jsonData,
				err:      err,
			}
		}(ip)
	}

	// Wait for completion
	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	// Collect results
	var allJSONData [][]byte
	var successCount int
	for result := range resultsCh {
		if result.err != nil {
			log.Printf("Runner %s failed: %v", result.ip, result.err)
			continue
		}
		successCount++
		if len(result.jsonData) > 0 {
			allJSONData = append(allJSONData, result.jsonData)
		}
	}

	endTime := time.Now()
	updateStatus("processing", "collecting_results", 80)

	if successCount == 0 {
		updateStatus("failed", "failed", 0)
		log.Printf("Cloud benchmark %s failed: all runners failed", benchmarkID)
		return
	}

	// Parse and aggregate results
	aggregatedResults, err := parseAndAggregateCloudResults(allJSONData)
	if err != nil {
		log.Printf("Cloud benchmark %s: failed to parse results: %v", benchmarkID, err)
		updateStatus("failed", "failed", 0)
		return
	}

	updateStatus("saving", "saving", 90)

	environment := captureCloudRunEnvironment(ctx, state, target, runnerIPs, sshUser)

	// Save to storage
	run := &domain.BenchmarkRun{
		ID:        benchmarkID,
		CreatedAt: startTime,
		UpdatedAt: endTime,
		Name:      strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		Workload:  wl,
		Target:    target,
		Environment: environment,
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  endTime.Sub(startTime).String(),
		Status:    domain.RunStatusCompleted,
		Results:   aggregatedResults,
		Tags:      append([]string{"cloud", state.Provider, state.Region}, req.Tags...),
		Labels: map[string]string{
			"cloud":          "true",
			"provider":       state.Provider,
			"region":         state.Region,
			"infrastructure": state.ID,
			"runner_count":   fmt.Sprintf("%d", len(runnerIPs)),
		},
	}

	if state.Config.AMR != nil {
		run.Labels["amr_sku"] = state.Config.AMR.SKU
		if state.Config.AMR.ClusteringPolicy != "" {
			run.Labels["amr_clustering"] = state.Config.AMR.ClusteringPolicy
		}
		run.Labels["amr_high_availability"] = strconv.FormatBool(state.Config.AMR.HighAvailability)
		if state.Config.AMR.EvictionPolicy != "" {
			run.Labels["amr_eviction_policy"] = state.Config.AMR.EvictionPolicy
		}
		if len(state.Config.AMR.Modules) > 0 {
			run.Labels["amr_modules"] = strings.Join(state.Config.AMR.Modules, ",")
		}
		run.Labels["amr_access_keys_authentication"] = "Enabled"
		run.Labels["amr_entra_authentication"] = "Disabled"
		run.Labels["amr_minimum_tls_version"] = "1.2"
	}

	if _, err := s.storage.Save(ctx, "runs", run); err != nil {
		log.Printf("Cloud benchmark %s: failed to save: %v", benchmarkID, err)
		updateStatus("failed", "failed", 0)
		return
	}

	updateStatus("completed", "completed", 100)
	log.Printf("Cloud benchmark %s completed: %.2f ops/sec", benchmarkID, aggregatedResults.Summary.OpsPerSecond)

	// Clean up active benchmark after a delay
	go func() {
		time.Sleep(5 * time.Minute)
		s.activeBenchmarks.Delete(benchmarkID)
	}()
}

// buildCloudMemtierCommand builds the memtier command for cloud execution.
func buildCloudMemtierCommand(wl *domain.Workload, target *domain.Target, jsonOutFile string) []string {
	args := []string{
		"-s", target.Host,
		"-p", fmt.Sprintf("%d", target.Port),
		"-c", fmt.Sprintf("%d", wl.Clients),
		"-t", fmt.Sprintf("%d", wl.Threads),
	}

	if d, err := time.ParseDuration(strings.TrimSpace(wl.Duration)); err == nil && d > 0 {
		args = append(args, "--test-time", fmt.Sprintf("%d", int(d.Seconds())))
	} else {
		args = append(args, "-n", fmt.Sprintf("%d", wl.Requests))
	}

	if len(wl.Operations) > 0 {
		var getRatio, setRatio float64
		for _, op := range wl.Operations {
			switch op.Command {
			case "GET":
				getRatio = op.Ratio
			case "SET":
				setRatio = op.Ratio
			}
		}
		if getRatio > 0 || setRatio > 0 {
			getInt := int(getRatio * 10)
			setInt := int(setRatio * 10)
			if getInt > 0 || setInt > 0 {
				args = append(args, "--ratio", fmt.Sprintf("%d:%d", setInt, getInt))
			}
		}
	}

	if target.Password != "" {
		args = append(args, "-a", target.Password)
	}

	if target.TLS != nil && target.TLS.Enabled {
		args = append(args, "--tls")
		if target.TLS.InsecureSkipVerify {
			args = append(args, "--tls-skip-verify")
		}
	}

	if jsonOutFile != "" {
		args = append(args, "--json-out-file", jsonOutFile)
	}

	return args
}

// runSSHCommand executes a command on a remote host via SSH.
func runSSHCommand(ctx context.Context, host, user, command string) (string, error) {
	sshArgs := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=30",
		fmt.Sprintf("%s@%s", user, host),
		command,
	}

	cmd := execCommandContext(ctx, "ssh", sshArgs...)
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return "", fmt.Errorf("SSH command timed out/cancelled: %w", ctx.Err())
	}
	if err != nil {
		return "", fmt.Errorf("SSH command failed: %w, output: %s", err, string(output))
	}

	return string(output), nil
}

func computeCloudBenchmarkTimeout(duration string) time.Duration {
	if d, err := time.ParseDuration(strings.TrimSpace(duration)); err == nil && d > 0 {
		if d < 2*time.Minute {
			return 5 * time.Minute
		}
		return d + 3*time.Minute
	}

	// Fallback for request-count based runs where no duration is specified.
	return 20 * time.Minute
}

func computeExpectedBenchmarkDuration(duration string) time.Duration {
	d, err := time.ParseDuration(strings.TrimSpace(duration))
	if err != nil || d <= 0 {
		return 0
	}

	return d
}

func captureCloudRunEnvironment(ctx context.Context, state *terraform.InfraState, target *domain.Target, runnerIPs []string, sshUser string) *domain.Environment {
	env := &domain.Environment{
		Host:  &domain.HostInfo{},
		Redis: &domain.RedisInfo{Config: map[string]string{}},
		Cloud: &domain.CloudEnvironment{},
	}

	if state != nil {
		env.Cloud.Provider = state.Provider
		env.Cloud.Region = state.Region
		if state.Config.Runners != nil {
			env.Cloud.InstanceType = state.Config.Runners.InstanceType
		}

		if state.Config.AMR != nil {
			env.Redis.Config["amr_sku"] = state.Config.AMR.SKU
			env.Redis.Config["amr_clustering"] = state.Config.AMR.ClusteringPolicy
			env.Redis.Config["amr_high_availability"] = strconv.FormatBool(state.Config.AMR.HighAvailability)
			env.Redis.Config["amr_eviction_policy"] = state.Config.AMR.EvictionPolicy
			env.Redis.Config["amr_modules"] = strings.Join(state.Config.AMR.Modules, ",")
			env.Redis.Config["amr_access_keys_authentication"] = "Enabled"
			env.Redis.Config["amr_entra_authentication"] = "Disabled"
			env.Redis.Config["amr_minimum_tls_version"] = "1.2"

			clusterPolicy := strings.ToLower(strings.TrimSpace(state.Config.AMR.ClusteringPolicy))
			env.Redis.ClusterEnabled = strings.Contains(clusterPolicy, "cluster") && clusterPolicy != "non-clustered"
			if env.Redis.ClusterEnabled {
				env.Redis.Mode = "cluster"
			} else {
				env.Redis.Mode = "standalone"
			}
		}
	}

	if len(runnerIPs) > 0 {
		env.Host.Hostname = runnerIPs[0]
		if hostInfo := collectRunnerHostInfo(ctx, runnerIPs[0], sshUser); hostInfo != nil {
			env.Host = hostInfo
		}
	}

	if target != nil {
		env.Redis.Config["target_host"] = target.Host
		env.Redis.Config["target_port"] = fmt.Sprintf("%d", target.Port)
	}

	return env
}

func collectRunnerHostInfo(ctx context.Context, runnerIP string, sshUser string) *domain.HostInfo {
	if runnerIP == "" || sshUser == "" {
		return nil
	}

	runnerCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	cmd := `bash -lc 'echo HOSTNAME=$(hostname); echo OS=$(uname -s); echo ARCH=$(uname -m); echo KERNEL=$(uname -r); echo CPUS=$(nproc); echo MEM_KB=$(awk "\/MemTotal\/ {print $2}" /proc/meminfo); echo CPU_MODEL=$(awk -F":" "\/model name\/ {gsub(/^ +/, \"\", $2); print $2; exit}" /proc/cpuinfo)'`
	out, err := runSSHCommand(runnerCtx, runnerIP, sshUser, cmd)
	if err != nil {
		return &domain.HostInfo{Hostname: runnerIP}
	}

	host := &domain.HostInfo{Hostname: runnerIP}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		switch k {
		case "HOSTNAME":
			if v != "" {
				host.Hostname = v
			}
		case "OS":
			host.OS = strings.ToLower(v)
		case "ARCH":
			host.Arch = v
		case "KERNEL":
			host.KernelVersion = v
		case "CPUS":
			if n, err := strconv.Atoi(v); err == nil {
				host.CPUs = n
			}
		case "MEM_KB":
			if kb, err := strconv.ParseFloat(v, 64); err == nil {
				host.MemoryGB = kb / (1024.0 * 1024.0)
			}
		case "CPU_MODEL":
			host.CPUModel = v
		}
	}

	return host
}

type benchmarkStage struct {
	Key   string
	Label string
	Index int
	Total int
}

func resolveBenchmarkStage(stageKey string, status string) benchmarkStage {
	key := strings.TrimSpace(stageKey)
	if key == "" {
		key = strings.TrimSpace(status)
	}

	switch key {
	case "starting":
		return benchmarkStage{Key: "starting", Label: "Preparing Benchmark", Index: 1, Total: 6}
	case "connecting":
		return benchmarkStage{Key: "connecting", Label: "Connecting to Runners", Index: 2, Total: 6}
	case "running", "executing":
		return benchmarkStage{Key: "executing", Label: "Executing Workload", Index: 3, Total: 6}
	case "processing", "collecting_results":
		return benchmarkStage{Key: "collecting_results", Label: "Collecting Results", Index: 4, Total: 6}
	case "saving":
		return benchmarkStage{Key: "saving", Label: "Saving Run", Index: 5, Total: 6}
	case "completed":
		return benchmarkStage{Key: "completed", Label: "Completed", Index: 6, Total: 6}
	case "failed":
		return benchmarkStage{Key: "failed", Label: "Failed", Index: 6, Total: 6}
	case "cancelled":
		return benchmarkStage{Key: "cancelled", Label: "Cancelled", Index: 6, Total: 6}
	default:
		return benchmarkStage{Key: key, Label: "Running", Index: 3, Total: 6}
	}
}

func estimateBenchmarkRemainingSeconds(ab *ActiveBenchmark, stage benchmarkStage) int64 {
	if ab == nil {
		return 0
	}

	status := strings.ToLower(strings.TrimSpace(ab.Status))
	if status == "completed" || status == "failed" || status == "cancelled" {
		return 0
	}

	elapsed := time.Since(ab.StartTime)
	if elapsed < 0 {
		elapsed = 0
	}

	if ab.ExpectedDurationSeconds > 0 {
		target := time.Duration(ab.ExpectedDurationSeconds) * time.Second
		overhead := 20 * time.Second
		switch stage.Key {
		case "starting", "connecting":
			overhead = 30 * time.Second
		case "collecting_results", "saving":
			overhead = 8 * time.Second
		}

		remaining := (target + overhead) - elapsed
		if remaining < 0 {
			return 0
		}
		return int64(remaining.Seconds())
	}

	if ab.Progress > 0 {
		totalEstimate := time.Duration(float64(elapsed) / (ab.Progress / 100.0))
		remaining := totalEstimate - elapsed
		if remaining < 0 {
			return 0
		}
		return int64(remaining.Seconds())
	}

	return 0
}

// extractJSONFromOutput extracts JSON data wrapped in markers.
func extractJSONFromOutput(output string) []byte {
	startMarker := "===JSON_OUTPUT_START==="
	endMarker := "===JSON_OUTPUT_END==="

	startIdx := strings.Index(output, startMarker)
	endIdx := strings.Index(output, endMarker)

	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		return nil
	}

	jsonStr := strings.TrimSpace(output[startIdx+len(startMarker) : endIdx])
	return []byte(jsonStr)
}

// parseAndAggregateCloudResults parses and aggregates memtier JSON outputs.
func parseAndAggregateCloudResults(jsonDataList [][]byte) (*domain.Results, error) {
	if len(jsonDataList) == 0 {
		return nil, fmt.Errorf("no JSON data to parse")
	}

	aggregated := &domain.Results{
		Summary:     &domain.SummaryMetrics{},
		ByOperation: make(map[string]*domain.OperationMetrics),
	}

	var totalOps float64
	var totalLatency float64
	var count int

	for _, data := range jsonDataList {
		if len(data) == 0 {
			continue
		}

		// Parse memtier JSON (simplified parser)
		var output struct {
			AllStats struct {
				Sets struct {
					OpsPerSec float64 `json:"Ops/sec"`
					Latency   float64 `json:"Latency"`
				} `json:"Sets"`
				Gets struct {
					OpsPerSec float64 `json:"Ops/sec"`
					Latency   float64 `json:"Latency"`
				} `json:"Gets"`
				Totals struct {
					OpsPerSec   float64 `json:"Ops/sec"`
					Latency     float64 `json:"Latency"`
					Percentiles struct {
						P50  float64 `json:"p50.00"`
						P90  float64 `json:"p90.00"`
						P95  float64 `json:"p95.00"`
						P99  float64 `json:"p99.00"`
						P999 float64 `json:"p99.90"`
					} `json:"Percentile Latencies"`
				} `json:"Totals"`
			} `json:"ALL STATS"`
		}

		if err := json.Unmarshal(data, &output); err != nil {
			continue
		}

		ops := output.AllStats.Totals.OpsPerSec
		if ops == 0 {
			ops = output.AllStats.Sets.OpsPerSec + output.AllStats.Gets.OpsPerSec
		}

		totalOps += ops
		totalLatency += output.AllStats.Totals.Latency
		count++

		// Track percentiles (use max as worst case)
		if output.AllStats.Totals.Percentiles.P50 > aggregated.Summary.P50LatencyMs {
			aggregated.Summary.P50LatencyMs = output.AllStats.Totals.Percentiles.P50
		}
		if output.AllStats.Totals.Percentiles.P90 > aggregated.Summary.P90LatencyMs {
			aggregated.Summary.P90LatencyMs = output.AllStats.Totals.Percentiles.P90
		}
		if output.AllStats.Totals.Percentiles.P95 > aggregated.Summary.P95LatencyMs {
			aggregated.Summary.P95LatencyMs = output.AllStats.Totals.Percentiles.P95
		}
		if output.AllStats.Totals.Percentiles.P99 > aggregated.Summary.P99LatencyMs {
			aggregated.Summary.P99LatencyMs = output.AllStats.Totals.Percentiles.P99
		}
		if output.AllStats.Totals.Percentiles.P999 > aggregated.Summary.P999LatencyMs {
			aggregated.Summary.P999LatencyMs = output.AllStats.Totals.Percentiles.P999
		}

		// Aggregate by operation
		if output.AllStats.Sets.OpsPerSec > 0 {
			if existing, ok := aggregated.ByOperation["SET"]; ok {
				existing.OpsPerSecond += output.AllStats.Sets.OpsPerSec
			} else {
				aggregated.ByOperation["SET"] = &domain.OperationMetrics{
					Operation:    "SET",
					OpsPerSecond: output.AllStats.Sets.OpsPerSec,
					AvgLatencyMs: output.AllStats.Sets.Latency,
				}
			}
		}
		if output.AllStats.Gets.OpsPerSec > 0 {
			if existing, ok := aggregated.ByOperation["GET"]; ok {
				existing.OpsPerSecond += output.AllStats.Gets.OpsPerSec
			} else {
				aggregated.ByOperation["GET"] = &domain.OperationMetrics{
					Operation:    "GET",
					OpsPerSecond: output.AllStats.Gets.OpsPerSec,
					AvgLatencyMs: output.AllStats.Gets.Latency,
				}
			}
		}
	}

	if count == 0 {
		return nil, fmt.Errorf("no valid results parsed")
	}

	aggregated.Summary.OpsPerSecond = totalOps
	aggregated.Summary.AvgLatencyMs = totalLatency / float64(count)

	return aggregated, nil
}

// execCommand is a wrapper for exec.Command.
var execCommand = func(name string, args ...string) *execCommandWrapper {
	return &execCommandWrapper{name: name, args: args}
}

var execCommandContext = func(ctx context.Context, name string, args ...string) *execCommandWrapper {
	return &execCommandWrapper{name: name, args: args, ctx: ctx}
}

type execCommandWrapper struct {
	name string
	args []string
	ctx  context.Context
}

func (c *execCommandWrapper) CombinedOutput() ([]byte, error) {
	var cmd *exec.Cmd
	if c.ctx != nil {
		cmd = exec.CommandContext(c.ctx, c.name, c.args...)
	} else {
		cmd = exec.Command(c.name, c.args...)
	}
	return cmd.CombinedOutput()
}

// infraStateToResponse converts terraform.InfraState to InfraResponse.
func infraStateToResponse(state *terraform.InfraState) InfraResponse {
	resp := InfraResponse{
		ID:        state.ID,
		Name:      state.Name,
		Status:    state.Status,
		Provider:  state.Provider,
		Region:    state.Region,
		CreatedAt: state.CreatedAt,
		UpdatedAt: state.UpdatedAt,
		Error:     state.Error,
	}

	if !state.ExpiresAt.IsZero() {
		resp.ExpiresAt = &state.ExpiresAt
	}

	if state.Outputs != nil {
		resp.Outputs = &InfraOutputs{
			RedisHostname:     state.Outputs.RedisHostname,
			RedisPort:         state.Outputs.RedisPort,
			RedisPrimaryKey:   state.Outputs.RedisPrimaryKey,
			ResourceGroupName: state.Outputs.ResourceGroupName,
			ClusterID:         state.Outputs.ClusterID,
			RunnerIPs:         state.Outputs.RunnerIPs,
			RunnerPrivateIPs:  state.Outputs.RunnerPrivateIPs,
		}
	}

	// Include config summary
	resp.Config = &InfraCreateRequest{
		Name:     state.Config.Name,
		Provider: state.Config.Provider,
		Region:   state.Config.Region,
		Tags:     state.Config.Tags,
	}
	if state.Config.AMR != nil {
		resp.Config.AMR = &InfraAMRConfig{
			SKU:              state.Config.AMR.SKU,
			Modules:          state.Config.AMR.Modules,
			HighAvailability: state.Config.AMR.HighAvailability,
			ClusteringPolicy: state.Config.AMR.ClusteringPolicy,
			EvictionPolicy:   state.Config.AMR.EvictionPolicy,
		}
	}
	if state.Config.Runners != nil {
		resp.Config.Runners = &InfraRunnerConfig{
			Count:         state.Config.Runners.Count,
			InstanceType:  state.Config.Runners.InstanceType,
			SpotInstances: state.Config.Runners.SpotInstances,
		}
	}

	return resp
}

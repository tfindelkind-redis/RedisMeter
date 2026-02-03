package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
	"github.com/tfindelkind-redis/redismeter/internal/plugin"
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
	Config    *BenchmarkConfig `json:"config"`
	Cancel    context.CancelFunc
}

// ServerConfig holds server configuration.
type ServerConfig struct {
	Addr           string
	Storage        plugin.StoragePlugin
	Engine         BenchmarkEngine
	EnableCORS     bool
	AllowedOrigins []string
	APIKey         string // Simple API key auth (optional)
}

// NewServer creates a new API server.
func NewServer(cfg ServerConfig) *Server {
	s := &Server{
		addr:    cfg.Addr,
		storage: cfg.Storage,
		engine:  cfg.Engine,
		mux:     http.NewServeMux(),
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

	// Register routes
	s.mux.HandleFunc("/", s.handleRoot)
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/api/v1/runs", s.handleRuns)
	s.mux.HandleFunc("/api/v1/runs/", s.handleRun)
	s.mux.HandleFunc("/api/v1/baselines", s.handleBaselines)
	s.mux.HandleFunc("/api/v1/baselines/", s.handleBaseline)
	s.mux.HandleFunc("/api/v1/workloads", s.handleWorkloads)
	s.mux.HandleFunc("/api/v1/benchmark", s.handleBenchmark)
	s.mux.HandleFunc("/api/v1/benchmark/", s.handleBenchmarkStatus)
	s.mux.HandleFunc("/api/v1/compare", s.handleCompare)
	s.mux.HandleFunc("/api/v1/analyze", s.handleAnalyze)
	s.mux.HandleFunc("/api/v1/ws", s.handleWebSocket)

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

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	response := map[string]interface{}{
		"name":    "RedisMeter API",
		"version": "1.0.0",
		"endpoints": map[string]string{
			"health":     "/health",
			"runs":       "/api/v1/runs",
			"baselines":  "/api/v1/baselines",
			"workloads":  "/api/v1/workloads",
			"benchmark":  "/api/v1/benchmark",
			"compare":    "/api/v1/compare",
			"analyze":    "/api/v1/analyze",
			"websocket":  "/api/v1/ws",
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
	if err := s.storage.Query(ctx, "benchmark_run", filter, &runs); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to query runs: "+err.Error())
		return
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

	id, err := s.storage.Save(ctx, "benchmark_run", &run)
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
	if err := s.storage.Load(ctx, "benchmark_run", id, &run); err != nil {
		writeError(w, http.StatusNotFound, "Run not found: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, run)
}

func (s *Server) deleteRun(w http.ResponseWriter, r *http.Request, id string) {
	ctx := r.Context()

	if err := s.storage.Delete(ctx, "benchmark_run", id); err != nil {
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

	id, err := s.storage.Save(ctx, "baseline", &baseline)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to save baseline: "+err.Error())
		return
	}
	baseline.ID = id

	writeJSON(w, http.StatusCreated, baseline)
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
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	// Return built-in workloads
	// This would need to be wired to the workload registry
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"workloads": []map[string]string{
			{"name": "cache", "description": "Standard cache workload with GET/SET operations"},
			{"name": "mixed", "description": "Mixed workload with various operation types"},
			{"name": "read-heavy", "description": "Read-heavy workload (90% GET, 10% SET)"},
			{"name": "write-heavy", "description": "Write-heavy workload (10% GET, 90% SET)"},
			{"name": "pipeline", "description": "Pipeline workload for bulk operations"},
			{"name": "large-values", "description": "Large value workload (1KB-10KB)"},
			{"name": "small-values", "description": "Small value workload (8-64 bytes)"},
			{"name": "scan-heavy", "description": "SCAN operation heavy workload"},
		},
	})
}

// BenchmarkConfig represents a benchmark configuration.
type BenchmarkConfig struct {
	Workload string `json:"workload"`
	Target   string `json:"target"`
	Password string `json:"password,omitempty"`
	Duration string `json:"duration,omitempty"`
	Requests int    `json:"requests,omitempty"`
	Clients  int    `json:"clients,omitempty"`
	Threads  int    `json:"threads,omitempty"`
	Pipeline int    `json:"pipeline,omitempty"`
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
		Config:    &cfg,
		Cancel:    cancel,
	}
	s.activeBenchmarks.Store(benchID, active)

	// Start benchmark in background
	go s.runBenchmark(ctx, benchID, &cfg)

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"id":       benchID,
		"status":   "starting",
		"message":  "Benchmark started",
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

	// Run the actual benchmark
	// This would integrate with the engine
	// For now, we'll simulate
	defer func() {
		ab.Status = "completed"
	}()

	// Simulate progress updates
	for i := 0; i <= 100; i += 10 {
		select {
		case <-ctx.Done():
			ab.Status = "cancelled"
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
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"id":         ab.ID,
			"status":     ab.Status,
			"progress":   ab.Progress,
			"start_time": ab.StartTime,
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
	if err := s.storage.Load(ctx, "benchmark_run", req.RunID1, &run1); err != nil {
		writeError(w, http.StatusNotFound, "Run 1 not found: "+err.Error())
		return
	}

	var run2 domain.BenchmarkRun
	if err := s.storage.Load(ctx, "benchmark_run", req.RunID2, &run2); err != nil {
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
	if err := s.storage.Load(ctx, "benchmark_run", req.RunID, &run); err != nil {
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
	throughputChange := 0.0
	if run1.Results != nil && run1.Results.Summary != nil && run1.Results.Summary.OpsPerSecond > 0 {
		throughputChange = (run2.Results.Summary.OpsPerSecond - run1.Results.Summary.OpsPerSecond) / run1.Results.Summary.OpsPerSecond * 100
	}

	latencyChange := 0.0
	if run1.Results != nil && run1.Results.Summary != nil && run1.Results.Summary.AvgLatencyMs > 0 {
		latencyChange = (run2.Results.Summary.AvgLatencyMs - run1.Results.Summary.AvgLatencyMs) / run1.Results.Summary.AvgLatencyMs * 100
	}

	run1Summary := getSummary(run1)
	run2Summary := getSummary(run2)

	return map[string]interface{}{
		"run1_id": run1.ID,
		"run2_id": run2.ID,
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

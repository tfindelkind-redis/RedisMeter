package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/bundle"
	"github.com/tfindelkind-redis/redismeter/internal/infraprofile"
	"github.com/tfindelkind-redis/redismeter/internal/logging"
)

// LogStore interface for log storage
type LogStore interface {
	Query(filter logging.QueryFilter) ([]*logging.Entry, error)
	Count(filter logging.QueryFilter) (int, error)
	GetStats() (*logging.StoreStats, error)
	Export(filter logging.QueryFilter, format string) ([]byte, error)
	Delete(filter logging.QueryFilter) (int, error)
}

// RegisterLogRoutes adds log-related routes to the server
func (s *Server) RegisterLogRoutes(logStore LogStore) {
	s.logStore = logStore
	s.mux.HandleFunc("/api/v1/logs", s.handleLogs)
	s.mux.HandleFunc("/api/v1/logs/stats", s.handleLogStats)
	s.mux.HandleFunc("/api/v1/logs/export", s.handleLogExport)
}

// logStore is stored in the server struct (add this field to Server)
var serverLogStore LogStore

// handleLogs handles GET (list/query) and DELETE for logs
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if s.logStore == nil {
		writeError(w, http.StatusServiceUnavailable, "Log storage not configured")
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.listLogs(w, r)
	case http.MethodDelete:
		s.deleteLogs(w, r)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) listLogs(w http.ResponseWriter, r *http.Request) {
	filter := parseLogFilter(r)

	logs, err := s.logStore.Query(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to query logs: "+err.Error())
		return
	}

	total, _ := s.logStore.Count(filter)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"logs":   logs,
		"count":  len(logs),
		"total":  total,
		"filter": filter,
	})
}

func (s *Server) deleteLogs(w http.ResponseWriter, r *http.Request) {
	filter := parseLogFilter(r)

	// Safety check - require at least one filter
	if filter.Since == nil && filter.Level == "" && filter.Source == "" && filter.BenchmarkID == "" {
		writeError(w, http.StatusBadRequest, "At least one filter is required for deletion")
		return
	}

	deleted, err := s.logStore.Delete(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete logs: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"deleted": deleted,
		"message": "Logs deleted successfully",
	})
}

func (s *Server) handleLogStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	if s.logStore == nil {
		writeError(w, http.StatusServiceUnavailable, "Log storage not configured")
		return
	}

	stats, err := s.logStore.GetStats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get log stats: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleLogExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	if s.logStore == nil {
		writeError(w, http.StatusServiceUnavailable, "Log storage not configured")
		return
	}

	filter := parseLogFilter(r)
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	data, err := s.logStore.Export(filter, format)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to export logs: "+err.Error())
		return
	}

	filename := "redismeter-logs-" + time.Now().Format("2006-01-02") + "." + format
	
	switch format {
	case "json":
		w.Header().Set("Content-Type", "application/json")
	case "jsonl":
		w.Header().Set("Content-Type", "application/x-ndjson")
	case "csv":
		w.Header().Set("Content-Type", "text/csv")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.Write(data)
}

func parseLogFilter(r *http.Request) logging.QueryFilter {
	filter := logging.QueryFilter{}

	if level := r.URL.Query().Get("level"); level != "" {
		filter.Level = logging.Level(level)
	}
	if source := r.URL.Query().Get("source"); source != "" {
		filter.Source = logging.Source(source)
	}
	if benchmarkID := r.URL.Query().Get("benchmark_id"); benchmarkID != "" {
		filter.BenchmarkID = benchmarkID
	}
	if infraID := r.URL.Query().Get("infra_id"); infraID != "" {
		filter.InfraID = infraID
	}
	if operation := r.URL.Query().Get("operation"); operation != "" {
		filter.Operation = operation
	}
	if search := r.URL.Query().Get("search"); search != "" {
		filter.Search = search
	}
	if since := r.URL.Query().Get("since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			filter.Since = &t
		}
	}
	if until := r.URL.Query().Get("until"); until != "" {
		if t, err := time.Parse(time.RFC3339, until); err == nil {
			filter.Until = &t
		}
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

	return filter
}

// --- Infrastructure Profile Routes ---

// InfraProfileStore interface for infrastructure profile storage
type InfraProfileStore interface {
	Create(ctx interface{}, profile *infraprofile.Profile) error
	Get(ctx interface{}, id string) (*infraprofile.Profile, error)
	GetByName(ctx interface{}, name string) (*infraprofile.Profile, error)
	List(ctx interface{}) ([]*infraprofile.Profile, error)
	ListByProvider(ctx interface{}, provider infraprofile.Provider) ([]*infraprofile.Profile, error)
	ListByTag(ctx interface{}, tag string) ([]*infraprofile.Profile, error)
	Update(ctx interface{}, profile *infraprofile.Profile) error
	Delete(ctx interface{}, id string) error
	RecordUsage(ctx interface{}, id string) error
	GetStats(ctx interface{}) (*infraprofile.ProfileStats, error)
	ExportAll(ctx interface{}) ([]byte, error)
	ImportProfiles(ctx interface{}, data []byte, overwrite bool) (int, error)
}

// RegisterInfraProfileRoutes adds infrastructure profile routes to the server
func (s *Server) RegisterInfraProfileRoutes(store InfraProfileStore) {
	s.infraProfileStore = store
	s.mux.HandleFunc("/api/v1/infrastructure-profiles", s.handleInfraProfiles)
	s.mux.HandleFunc("/api/v1/infrastructure-profiles/", s.handleInfraProfile)
	s.mux.HandleFunc("/api/v1/infrastructure-profiles/stats", s.handleInfraProfileStats)
	s.mux.HandleFunc("/api/v1/infrastructure-profiles/export", s.handleInfraProfileExport)
	s.mux.HandleFunc("/api/v1/infrastructure-profiles/import", s.handleInfraProfileImport)
}

func (s *Server) handleInfraProfiles(w http.ResponseWriter, r *http.Request) {
	if s.infraProfileStore == nil {
		writeError(w, http.StatusServiceUnavailable, "Infrastructure profile storage not configured")
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.listInfraProfiles(w, r)
	case http.MethodPost:
		s.createInfraProfile(w, r)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) listInfraProfiles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var profiles []*infraprofile.Profile
	var err error

	// Filter by provider
	if provider := r.URL.Query().Get("provider"); provider != "" {
		profiles, err = s.infraProfileStore.ListByProvider(ctx, infraprofile.Provider(provider))
	} else if tag := r.URL.Query().Get("tag"); tag != "" {
		profiles, err = s.infraProfileStore.ListByTag(ctx, tag)
	} else {
		profiles, err = s.infraProfileStore.List(ctx)
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list profiles: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"profiles": profiles,
		"count":    len(profiles),
	})
}

func (s *Server) createInfraProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var profile infraprofile.Profile
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	if err := profile.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, "Validation failed: "+err.Error())
		return
	}

	if err := s.infraProfileStore.Create(ctx, &profile); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create profile: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, profile)
}

func (s *Server) handleInfraProfile(w http.ResponseWriter, r *http.Request) {
	// Extract profile ID from path: /api/v1/infrastructure-profiles/{id}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/infrastructure-profiles/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Missing profile ID")
		return
	}

	// Handle special endpoints
	if id == "stats" {
		s.handleInfraProfileStats(w, r)
		return
	}
	if id == "export" {
		s.handleInfraProfileExport(w, r)
		return
	}
	if id == "import" {
		s.handleInfraProfileImport(w, r)
		return
	}

	if s.infraProfileStore == nil {
		writeError(w, http.StatusServiceUnavailable, "Infrastructure profile storage not configured")
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getInfraProfile(w, r, id)
	case http.MethodPut:
		s.updateInfraProfile(w, r, id)
	case http.MethodDelete:
		s.deleteInfraProfile(w, r, id)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) getInfraProfile(w http.ResponseWriter, r *http.Request, id string) {
	ctx := r.Context()

	profile, err := s.infraProfileStore.Get(ctx, id)
	if err != nil || profile == nil {
		// Try by name
		profile, err = s.infraProfileStore.GetByName(ctx, id)
		if err != nil || profile == nil {
			writeError(w, http.StatusNotFound, "Profile not found")
			return
		}
	}

	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) updateInfraProfile(w http.ResponseWriter, r *http.Request, id string) {
	ctx := r.Context()

	// Check if trying to update a built-in profile
	existing, err := s.infraProfileStore.Get(ctx, id)
	if err == nil && existing != nil && existing.IsBuiltin {
		writeError(w, http.StatusForbidden, "Cannot modify built-in profile")
		return
	}

	var profile infraprofile.Profile
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	profile.ID = id

	if err := profile.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, "Validation failed: "+err.Error())
		return
	}

	if err := s.infraProfileStore.Update(ctx, &profile); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update profile: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) deleteInfraProfile(w http.ResponseWriter, r *http.Request, id string) {
	ctx := r.Context()

	// Check if trying to delete a built-in profile
	existing, err := s.infraProfileStore.Get(ctx, id)
	if err == nil && existing != nil && existing.IsBuiltin {
		writeError(w, http.StatusForbidden, "Cannot delete built-in profile")
		return
	}

	if err := s.infraProfileStore.Delete(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete profile: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Profile deleted successfully",
	})
}

func (s *Server) handleInfraProfileStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	if s.infraProfileStore == nil {
		writeError(w, http.StatusServiceUnavailable, "Infrastructure profile storage not configured")
		return
	}

	ctx := r.Context()
	stats, err := s.infraProfileStore.GetStats(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get stats: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleInfraProfileExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	if s.infraProfileStore == nil {
		writeError(w, http.StatusServiceUnavailable, "Infrastructure profile storage not configured")
		return
	}

	ctx := r.Context()
	data, err := s.infraProfileStore.ExportAll(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to export profiles: "+err.Error())
		return
	}

	filename := "infrastructure-profiles-" + time.Now().Format("2006-01-02") + ".json"
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.Write(data)
}

func (s *Server) handleInfraProfileImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	if s.infraProfileStore == nil {
		writeError(w, http.StatusServiceUnavailable, "Infrastructure profile storage not configured")
		return
	}

	ctx := r.Context()

	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Failed to read request body: "+err.Error())
		return
	}

	overwrite := r.URL.Query().Get("overwrite") == "true"

	count, err := s.infraProfileStore.ImportProfiles(ctx, data, overwrite)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to import profiles: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"imported": count,
		"message":  "Profiles imported successfully",
	})
}

// --- Export/Import Bundle Routes ---

// BundleDataProvider interface for bundle data access
type BundleDataProvider interface {
	bundle.DataProvider
	bundle.DataImporter
}

// RegisterBundleRoutes adds export/import bundle routes to the server
func (s *Server) RegisterBundleRoutes(provider BundleDataProvider, version string) {
	s.bundleProvider = provider
	s.bundleVersion = version
	s.mux.HandleFunc("/api/v1/export", s.handleExport)
	s.mux.HandleFunc("/api/v1/import", s.handleImport)
	s.mux.HandleFunc("/api/v1/export/options", s.handleExportOptions)
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	if s.bundleProvider == nil {
		writeError(w, http.StatusServiceUnavailable, "Bundle export not configured")
		return
	}

	ctx := r.Context()

	// Parse export options from request body
	var opts bundle.ExportOptions
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		// Use default options if no body provided
		opts = bundle.DefaultExportOptions()
	}

	// Create exporter and export
	exporter := bundle.NewExporter(s.bundleProvider, s.bundleVersion)
	exportBundle, err := exporter.Export(ctx, opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create export: "+err.Error())
		return
	}

	// Convert to ZIP bytes
	data, err := exportBundle.ToBytes()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create ZIP: "+err.Error())
		return
	}

	filename := "redismeter-export-" + time.Now().Format("2006-01-02-150405") + ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Write(data)
}

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	if s.bundleProvider == nil {
		writeError(w, http.StatusServiceUnavailable, "Bundle import not configured")
		return
	}

	ctx := r.Context()

	// Check content type
	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/zip") && 
	   !strings.HasPrefix(contentType, "application/octet-stream") &&
	   !strings.HasPrefix(contentType, "multipart/form-data") {
		writeError(w, http.StatusBadRequest, "Expected application/zip or multipart/form-data content type")
		return
	}

	var data []byte
	var err error

	// Handle multipart form upload
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(100 << 20); err != nil { // 100MB max
			writeError(w, http.StatusBadRequest, "Failed to parse multipart form: "+err.Error())
			return
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "Failed to get file from form: "+err.Error())
			return
		}
		defer file.Close()
		data, err = io.ReadAll(file)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Failed to read file: "+err.Error())
			return
		}
	} else {
		// Read raw body
		data, err = io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Failed to read request body: "+err.Error())
			return
		}
	}

	// Load bundle from ZIP
	importBundle, err := bundle.LoadFromBytes(data)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Failed to parse bundle: "+err.Error())
		return
	}

	// Parse import options
	opts := bundle.DefaultImportOptions()
	if r.URL.Query().Get("overwrite") == "true" {
		opts.OverwriteExisting = true
		opts.SkipExisting = false
	}

	// Import
	importer := bundle.NewImporter(s.bundleProvider)
	result, err := importer.Import(ctx, importBundle, opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to import bundle: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":  result.Success,
		"manifest": importBundle.Manifest,
		"result":   result,
	})
}

func (s *Server) handleExportOptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	// Return the default export options as documentation
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"default_options": bundle.DefaultExportOptions(),
		"description":     "POST to /api/v1/export with these options in the request body",
		"example": map[string]interface{}{
			"include_benchmarks":     true,
			"include_baselines":      true,
			"include_workloads":      true,
			"include_run_profiles":   true,
			"include_infra_profiles": true,
			"include_logs":           true,
			"include_reports":        true,
			"benchmark_ids":          []string{"optional", "specific", "ids"},
			"since":                  "2024-01-01T00:00:00Z",
			"description":            "My export bundle",
			"created_by":             "your-name",
		},
	})
}

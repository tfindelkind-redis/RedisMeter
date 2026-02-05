package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tfindelkind-redis/redismeter/internal/api"
	"github.com/tfindelkind-redis/redismeter/internal/infraprofile"
	"github.com/tfindelkind-redis/redismeter/internal/logging"
)

// serveCmd starts the API server.
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the RedisMeter API server",
	Long: `Start the RedisMeter REST API server.

The API server provides programmatic access to all RedisMeter functionality:
- Run and manage benchmarks
- Query and compare results
- Manage baselines
- Real-time progress via WebSocket

API endpoints:
  GET    /health              - Health check
  GET    /api/v1/runs         - List benchmark runs
  POST   /api/v1/runs         - Create a run
  GET    /api/v1/runs/{id}    - Get a specific run
  DELETE /api/v1/runs/{id}    - Delete a run
  GET    /api/v1/baselines    - List baselines
  POST   /api/v1/baselines    - Create a baseline
  GET    /api/v1/workloads    - List available workloads
  POST   /api/v1/benchmark    - Start a benchmark
  GET    /api/v1/benchmark/{id} - Get benchmark status
  POST   /api/v1/compare      - Compare two runs
  POST   /api/v1/analyze      - Analyze a run
  WS     /api/v1/ws           - WebSocket for real-time updates
`,
	Example: `  # Start server on default port
  redismeter serve

  # Start on custom port
  redismeter serve --port 9090

  # Enable CORS for web UI
  redismeter serve --cors --origins http://localhost:3000

  # With API key authentication
  redismeter serve --api-key mysecretkey`,
	RunE: runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().StringP("addr", "a", ":8080", "Server address (host:port)")
	serveCmd.Flags().IntP("port", "p", 8080, "Server port (shorthand for :port)")
	serveCmd.Flags().Bool("cors", false, "Enable CORS")
	serveCmd.Flags().StringSlice("origins", []string{"*"}, "Allowed CORS origins")
	serveCmd.Flags().String("api-key", "", "API key for authentication")
	serveCmd.Flags().String("web-dir", "", "Directory containing built frontend (enables production mode)")

	viper.BindPFlag("server.addr", serveCmd.Flags().Lookup("addr"))
	viper.BindPFlag("server.port", serveCmd.Flags().Lookup("port"))
	viper.BindPFlag("server.cors", serveCmd.Flags().Lookup("cors"))
	viper.BindPFlag("server.origins", serveCmd.Flags().Lookup("origins"))
	viper.BindPFlag("server.api_key", serveCmd.Flags().Lookup("api-key"))
	viper.BindPFlag("server.web_dir", serveCmd.Flags().Lookup("web-dir"))
}

func runServe(cmd *cobra.Command, args []string) error {
	// Initialize storage using existing helper
	storageType, storagePath := getStorageConfig()
	storage, err := newRunStorage(storageType, storagePath)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	if closer, ok := storage.(interface{ Close() error }); ok {
		defer closer.Close()
	}

	// Initialize extended feature stores
	homeDir, _ := os.UserHomeDir()
	basePath := filepath.Join(homeDir, ".redismeter")

	// Initialize log store (JSON-based)
	logStore, err := logging.NewJSONStore(filepath.Join(basePath, "logs"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Log storage not available: %v\n", err)
	} else {
		defer logStore.Close()
	}

	// Initialize infrastructure profile store
	profileStore, err := infraprofile.NewFileStore(filepath.Join(basePath, "infra-profiles"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Profile storage not available: %v\n", err)
	} else {
		defer profileStore.Close()
	}

	// Get server address
	addr := viper.GetString("server.addr")
	if port := viper.GetInt("server.port"); port != 0 && addr == ":8080" {
		addr = fmt.Sprintf(":%d", port)
	}

	// Create server config
	cfg := api.ServerConfig{
		Addr:           addr,
		Storage:        storage,
		EnableCORS:     viper.GetBool("server.cors"),
		AllowedOrigins: viper.GetStringSlice("server.origins"),
		APIKey:         viper.GetString("server.api_key"),
		WebDir:         viper.GetString("server.web_dir"),
	}

	// Auto-detect web directory if not specified
	if cfg.WebDir == "" {
		// Check common locations
		exePath, _ := os.Executable()
		exeDir := filepath.Dir(exePath)
		possiblePaths := []string{
			filepath.Join(exeDir, "web", "dist"),
			filepath.Join(exeDir, "..", "web", "dist"),
			"web/dist",
		}
		for _, p := range possiblePaths {
			if _, err := os.Stat(filepath.Join(p, "index.html")); err == nil {
				cfg.WebDir = p
				break
			}
		}
	}

	// Create and start server
	server := api.NewServer(cfg)

	// Register extended feature routes
	if logStore != nil {
		logAdapter := &apiLogStoreAdapter{store: logStore}
		server.RegisterLogRoutes(logAdapter)
		fmt.Println("📝 Log API endpoints enabled")
	}
	if profileStore != nil {
		profileAdapter := &apiProfileStoreAdapter{store: profileStore}
		server.RegisterInfraProfileRoutes(profileAdapter)
		fmt.Println("🏗️  Infrastructure Profile API endpoints enabled")
	}

	// Handle graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		fmt.Printf("🚀 RedisMeter API server starting on %s\n", addr)
		fmt.Println()
		fmt.Println("Endpoints:")
		if cfg.WebDir != "" {
			fmt.Printf("  Web UI:     http://localhost%s/\n", addr)
		}
		fmt.Printf("  Health:     http://localhost%s/health\n", addr)
		fmt.Printf("  API:        http://localhost%s/api/v1/\n", addr)
		fmt.Printf("  WebSocket:  ws://localhost%s/api/v1/ws\n", addr)
		fmt.Println()

		if cfg.APIKey != "" {
			fmt.Println("🔐 API key authentication enabled")
		}
		if cfg.EnableCORS {
			fmt.Printf("🌐 CORS enabled for origins: %v\n", cfg.AllowedOrigins)
		}
		fmt.Println()
		fmt.Println("Press Ctrl+C to stop")
		fmt.Println()

		if err := server.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		}
	}()

	<-stop
	fmt.Println("\n📤 Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	fmt.Println("✅ Server stopped gracefully")
	return nil
}

// apiLogStoreAdapter adapts logging.JSONStore to the api.LogStore interface
type apiLogStoreAdapter struct {
	store *logging.JSONStore
}

func (a *apiLogStoreAdapter) Query(filter logging.QueryFilter) ([]*logging.Entry, error) {
	return a.store.Query(context.Background(), &filter)
}

func (a *apiLogStoreAdapter) Count(filter logging.QueryFilter) (int, error) {
	count, err := a.store.Count(context.Background(), &filter)
	return int(count), err
}

func (a *apiLogStoreAdapter) GetStats() (*logging.StoreStats, error) {
	return a.store.GetStats(context.Background())
}

func (a *apiLogStoreAdapter) Export(filter logging.QueryFilter, format string) ([]byte, error) {
	var buf bytes.Buffer
	err := a.store.Export(context.Background(), &filter, &buf)
	return buf.Bytes(), err
}

func (a *apiLogStoreAdapter) Delete(filter logging.QueryFilter) (int, error) {
	if filter.Since != nil {
		count, err := a.store.Delete(context.Background(), *filter.Since)
		return int(count), err
	}
	return 0, nil
}

// apiProfileStoreAdapter adapts infraprofile.FileStore to the api.InfraProfileStore interface
type apiProfileStoreAdapter struct {
	store *infraprofile.FileStore
}

func (a *apiProfileStoreAdapter) List(ctx interface{}) ([]*infraprofile.Profile, error) {
	return a.store.List(context.Background())
}

func (a *apiProfileStoreAdapter) Get(ctx interface{}, id string) (*infraprofile.Profile, error) {
	return a.store.Get(context.Background(), id)
}

func (a *apiProfileStoreAdapter) GetByName(ctx interface{}, name string) (*infraprofile.Profile, error) {
	return a.store.GetByName(context.Background(), name)
}

func (a *apiProfileStoreAdapter) Create(ctx interface{}, profile *infraprofile.Profile) error {
	return a.store.Create(context.Background(), profile)
}

func (a *apiProfileStoreAdapter) Update(ctx interface{}, profile *infraprofile.Profile) error {
	return a.store.Update(context.Background(), profile)
}

func (a *apiProfileStoreAdapter) Delete(ctx interface{}, id string) error {
	return a.store.Delete(context.Background(), id)
}

func (a *apiProfileStoreAdapter) ListByProvider(ctx interface{}, provider infraprofile.Provider) ([]*infraprofile.Profile, error) {
	return a.store.ListByProvider(context.Background(), provider)
}

func (a *apiProfileStoreAdapter) ListByTag(ctx interface{}, tag string) ([]*infraprofile.Profile, error) {
	return a.store.ListByTag(context.Background(), tag)
}

func (a *apiProfileStoreAdapter) RecordUsage(ctx interface{}, id string) error {
	return a.store.RecordUsage(context.Background(), id)
}

func (a *apiProfileStoreAdapter) GetStats(ctx interface{}) (*infraprofile.ProfileStats, error) {
	return a.store.GetStats(context.Background())
}

func (a *apiProfileStoreAdapter) ExportAll(ctx interface{}) ([]byte, error) {
	return a.store.ExportAll(context.Background())
}

func (a *apiProfileStoreAdapter) ImportProfiles(ctx interface{}, data []byte, overwrite bool) (int, error) {
	return a.store.ImportProfiles(context.Background(), data, overwrite)
}

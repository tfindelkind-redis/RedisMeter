package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tfindelkind-redis/redismeter/internal/api"
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

	viper.BindPFlag("server.addr", serveCmd.Flags().Lookup("addr"))
	viper.BindPFlag("server.port", serveCmd.Flags().Lookup("port"))
	viper.BindPFlag("server.cors", serveCmd.Flags().Lookup("cors"))
	viper.BindPFlag("server.origins", serveCmd.Flags().Lookup("origins"))
	viper.BindPFlag("server.api_key", serveCmd.Flags().Lookup("api-key"))
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
	}

	// Create and start server
	server := api.NewServer(cfg)

	// Handle graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		fmt.Printf("🚀 RedisMeter API server starting on %s\n", addr)
		fmt.Println()
		fmt.Println("Endpoints:")
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

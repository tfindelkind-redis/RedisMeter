package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tfindelkind-redis/redismeter/internal/logging"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Manage RedisMeter logs",
	Long: `View, export, and manage RedisMeter operation logs.

Logs are automatically captured for all operations including:
- Benchmark executions
- Infrastructure provisioning
- API requests
- Data imports/exports`,
}

var logsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent logs",
	Long:  `List recent log entries with optional filtering by level, source, or time range.`,
	RunE:  runLogsList,
}

var logsShowCmd = &cobra.Command{
	Use:   "show [log-id]",
	Short: "Show detailed log entry",
	Args:  cobra.ExactArgs(1),
	RunE:  runLogsShow,
}

var logsExportCmd = &cobra.Command{
	Use:   "export [output-file]",
	Short: "Export logs to a file",
	Long:  `Export logs to JSON, JSONL, or CSV format.`,
	RunE:  runLogsExport,
}

var logsClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear old logs",
	Long:  `Clear logs older than specified duration.`,
	RunE:  runLogsClear,
}

var logsStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show log statistics",
	RunE:  runLogsStats,
}

// Flags
var (
	logsLevel       string
	logsSource      string
	logsBenchmarkID string
	logsInfraID     string
	logsSince       string
	logsUntil       string
	logsLimit       int
	logsSearch      string
	logsFormat      string
	logsOlderThan   string
	logsForce       bool
)

func init() {
	rootCmd.AddCommand(logsCmd)
	logsCmd.AddCommand(logsListCmd)
	logsCmd.AddCommand(logsShowCmd)
	logsCmd.AddCommand(logsExportCmd)
	logsCmd.AddCommand(logsClearCmd)
	logsCmd.AddCommand(logsStatsCmd)

	// List flags
	logsListCmd.Flags().StringVarP(&logsLevel, "level", "l", "", "Filter by level (DEBUG, INFO, WARN, ERROR)")
	logsListCmd.Flags().StringVarP(&logsSource, "source", "s", "", "Filter by source (memtier, terraform, api, etc.)")
	logsListCmd.Flags().StringVarP(&logsBenchmarkID, "benchmark", "b", "", "Filter by benchmark ID")
	logsListCmd.Flags().StringVarP(&logsInfraID, "infra", "i", "", "Filter by infrastructure ID")
	logsListCmd.Flags().StringVar(&logsSince, "since", "", "Show logs since (RFC3339 or duration like '1h', '24h')")
	logsListCmd.Flags().StringVar(&logsUntil, "until", "", "Show logs until (RFC3339)")
	logsListCmd.Flags().IntVarP(&logsLimit, "limit", "n", 50, "Maximum number of logs to show")
	logsListCmd.Flags().StringVarP(&logsSearch, "search", "q", "", "Search in log messages")

	// Export flags
	logsExportCmd.Flags().StringVarP(&logsFormat, "format", "f", "json", "Export format (json, jsonl, csv)")
	logsExportCmd.Flags().StringVarP(&logsLevel, "level", "l", "", "Filter by level")
	logsExportCmd.Flags().StringVarP(&logsSource, "source", "s", "", "Filter by source")
	logsExportCmd.Flags().StringVar(&logsSince, "since", "", "Export logs since")
	logsExportCmd.Flags().StringVar(&logsUntil, "until", "", "Export logs until")

	// Clear flags
	logsClearCmd.Flags().StringVar(&logsOlderThan, "older-than", "30d", "Clear logs older than (e.g., '7d', '24h')")
	logsClearCmd.Flags().BoolVarP(&logsForce, "force", "f", false, "Skip confirmation")
}

func getLogStore() (*logging.SQLiteStore, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	dbPath := filepath.Join(homeDir, ".redismeter", "logs.db")

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	return logging.NewSQLiteStore(dbPath)
}

func runLogsList(cmd *cobra.Command, args []string) error {
	store, err := getLogStore()
	if err != nil {
		return err
	}
	defer store.Close()

	filter := buildLogFilter()
	ctx := context.Background()

	entries, err := store.Query(ctx, &filter)
	if err != nil {
		return fmt.Errorf("failed to query logs: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No logs found matching the filter.")
		return nil
	}

	// Print header
	fmt.Printf("%-20s %-6s %-12s %-20s %s\n", "TIMESTAMP", "LEVEL", "SOURCE", "OPERATION", "MESSAGE")
	fmt.Println(strings.Repeat("-", 100))

	for _, entry := range entries {
		timestamp := entry.Timestamp.Format("2006-01-02 15:04:05")
		level := colorLevel(string(entry.Level))
		source := truncateString(string(entry.Source), 12)
		operation := truncateString(entry.Operation, 20)
		message := truncateString(entry.Message, 40)

		fmt.Printf("%-20s %-6s %-12s %-20s %s\n", timestamp, level, source, operation, message)
	}

	total, _ := store.Count(ctx, &filter)
	fmt.Printf("\nShowing %d of %d logs\n", len(entries), total)

	return nil
}

func runLogsShow(cmd *cobra.Command, args []string) error {
	store, err := getLogStore()
	if err != nil {
		return err
	}
	defer store.Close()

	logID := args[0]
	ctx := context.Background()

	// Query for the specific log (search by ID pattern)
	filter := logging.QueryFilter{
		Search: logID,
		Limit:  1,
	}

	entries, err := store.Query(ctx, &filter)
	if err != nil {
		return fmt.Errorf("failed to query log: %w", err)
	}

	if len(entries) == 0 {
		return fmt.Errorf("log entry not found: %s", logID)
	}

	entry := entries[0]

	fmt.Printf("ID:          %s\n", entry.ID)
	fmt.Printf("Timestamp:   %s\n", entry.Timestamp.Format(time.RFC3339))
	fmt.Printf("Level:       %s\n", colorLevel(string(entry.Level)))
	fmt.Printf("Source:      %s\n", entry.Source)
	fmt.Printf("Operation:   %s\n", entry.Operation)
	fmt.Printf("Message:     %s\n", entry.Message)

	if entry.BenchmarkID != "" {
		fmt.Printf("Benchmark:   %s\n", entry.BenchmarkID)
	}
	if entry.InfraID != "" {
		fmt.Printf("Infra:       %s\n", entry.InfraID)
	}
	if entry.Error != "" {
		fmt.Printf("Error:       %s\n", entry.Error)
	}
	if entry.Duration > 0 {
		fmt.Printf("Duration:    %s\n", entry.Duration)
	}
	if len(entry.Context) > 0 {
		fmt.Println("\nContext:")
		for k, v := range entry.Context {
			fmt.Printf("  %s: %v\n", k, v)
		}
	}

	return nil
}

func runLogsExport(cmd *cobra.Command, args []string) error {
	store, err := getLogStore()
	if err != nil {
		return err
	}
	defer store.Close()

	// Determine output file
	var outputFile string
	if len(args) > 0 {
		outputFile = args[0]
	} else {
		outputFile = fmt.Sprintf("redismeter-logs-%s.%s", time.Now().Format("2006-01-02"), logsFormat)
	}

	filter := buildLogFilter()
	filter.Limit = 0 // No limit for export
	ctx := context.Background()

	entries, err := store.Query(ctx, &filter)
	if err != nil {
		return fmt.Errorf("failed to query logs: %w", err)
	}

	var data []byte
	switch logsFormat {
	case "json":
		data, err = exportLogsJSON(entries)
	case "jsonl":
		data, err = exportLogsJSONL(entries)
	case "csv":
		data, err = exportLogsCSV(entries)
	default:
		return fmt.Errorf("unsupported format: %s", logsFormat)
	}

	if err != nil {
		return fmt.Errorf("failed to format logs: %w", err)
	}

	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("Exported %d logs to %s\n", len(entries), outputFile)
	return nil
}

func runLogsClear(cmd *cobra.Command, args []string) error {
	store, err := getLogStore()
	if err != nil {
		return err
	}
	defer store.Close()

	// Parse older-than duration
	cutoff, err := parseOlderThan(logsOlderThan)
	if err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}

	ctx := context.Background()

	// Count logs to be deleted
	filter := logging.QueryFilter{Until: &cutoff}
	count, err := store.Count(ctx, &filter)
	if err != nil {
		return fmt.Errorf("failed to count logs: %w", err)
	}

	if count == 0 {
		fmt.Println("No logs to clear.")
		return nil
	}

	// Confirm unless --force
	if !logsForce {
		fmt.Printf("This will delete %d logs older than %s.\n", count, logsOlderThan)
		fmt.Print("Continue? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	deleted, err := store.Delete(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("failed to delete logs: %w", err)
	}

	fmt.Printf("Deleted %d logs.\n", deleted)
	return nil
}

func runLogsStats(cmd *cobra.Command, args []string) error {
	store, err := getLogStore()
	if err != nil {
		return err
	}
	defer store.Close()

	ctx := context.Background()
	stats, err := store.GetStats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get stats: %w", err)
	}

	fmt.Println("Log Statistics")
	fmt.Println(strings.Repeat("=", 40))
	fmt.Printf("Total Entries:    %d\n", stats.TotalEntries)

	if !stats.OldestEntry.IsZero() {
		fmt.Printf("Oldest Entry:     %s\n", stats.OldestEntry.Format("2006-01-02 15:04:05"))
	}
	if !stats.NewestEntry.IsZero() {
		fmt.Printf("Newest Entry:     %s\n", stats.NewestEntry.Format("2006-01-02 15:04:05"))
	}

	if len(stats.LevelCounts) > 0 {
		fmt.Println("\nBy Level:")
		for level, count := range stats.LevelCounts {
			fmt.Printf("  %-8s %d\n", colorLevel(string(level)), count)
		}
	}

	if len(stats.SourceCounts) > 0 {
		fmt.Println("\nBy Source:")
		for source, count := range stats.SourceCounts {
			fmt.Printf("  %-12s %d\n", source, count)
		}
	}

	return nil
}

// Helper functions

func buildLogFilter() logging.QueryFilter {
	filter := logging.QueryFilter{
		Limit: logsLimit,
	}

	if logsLevel != "" {
		filter.Level = logging.Level(strings.ToUpper(logsLevel))
	}
	if logsSource != "" {
		filter.Source = logging.Source(logsSource)
	}
	if logsBenchmarkID != "" {
		filter.BenchmarkID = logsBenchmarkID
	}
	if logsInfraID != "" {
		filter.InfraID = logsInfraID
	}
	if logsSearch != "" {
		filter.Search = logsSearch
	}

	if logsSince != "" {
		if t := parseSince(logsSince); t != nil {
			filter.Since = t
		}
	}
	if logsUntil != "" {
		if t, err := time.Parse(time.RFC3339, logsUntil); err == nil {
			filter.Until = &t
		}
	}

	return filter
}

func parseSince(s string) *time.Time {
	// Try RFC3339 first
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return &t
	}

	// Try duration
	if d, err := parseDuration(s); err == nil {
		t := time.Now().Add(-d)
		return &t
	}

	return nil
}

func parseDuration(s string) (time.Duration, error) {
	// Handle days
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func parseOlderThan(s string) (time.Time, error) {
	d, err := parseDuration(s)
	if err != nil {
		return time.Time{}, err
	}
	return time.Now().Add(-d), nil
}

func colorLevel(level string) string {
	switch level {
	case "DEBUG":
		return "\033[36mDEBUG\033[0m" // Cyan
	case "INFO":
		return "\033[32mINFO\033[0m" // Green
	case "WARN":
		return "\033[33mWARN\033[0m" // Yellow
	case "ERROR":
		return "\033[31mERROR\033[0m" // Red
	default:
		return level
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func exportLogsJSON(entries []*logging.Entry) ([]byte, error) {
	return json.MarshalIndent(entries, "", "  ")
}

func exportLogsJSONL(entries []*logging.Entry) ([]byte, error) {
	var lines []string
	for _, e := range entries {
		line, err := json.Marshal(e)
		if err != nil {
			return nil, err
		}
		lines = append(lines, string(line))
	}
	return []byte(strings.Join(lines, "\n")), nil
}

func exportLogsCSV(entries []*logging.Entry) ([]byte, error) {
	var lines []string
	lines = append(lines, "timestamp,level,source,operation,message,benchmark_id,infra_id,error")
	for _, e := range entries {
		line := fmt.Sprintf("%s,%s,%s,%s,%q,%s,%s,%q",
			e.Timestamp.Format(time.RFC3339),
			e.Level,
			e.Source,
			e.Operation,
			e.Message,
			e.BenchmarkID,
			e.InfraID,
			e.Error,
		)
		lines = append(lines, line)
	}
	return []byte(strings.Join(lines, "\n")), nil
}

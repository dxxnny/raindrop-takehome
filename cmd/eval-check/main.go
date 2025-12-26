package main

import (
	"log/slog"
	"os"

	"github.com/raindrop/nl2sql/pkg/shared"
)

// This CLI runs evals at build time and fails the build if any eval fails.
// Usage: go run ./cmd/eval-check
func main() {
	slog.Info("Running build-time evals...")

	// Load config from environment
	cfg, err := shared.LoadConfig()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Initialize clients
	tinybird := shared.NewTinybirdClient(cfg)
	openai := shared.NewOpenAIClient(cfg)

	// Fetch schema
	slog.Info("Fetching schema from Tinybird...")
	schema, err := tinybird.FetchSchema()
	if err != nil {
		slog.Error("Failed to fetch schema", "error", err)
		os.Exit(1)
	}
	openai.SetSchema(schema)
	slog.Info("Schema loaded", "tables", len(schema.Datasources))

	// Run evals
	slog.Info("Running evals...")
	results, evalErr := shared.RunEvals(openai, tinybird)
	summary := shared.ComputeSummary(results)

	// Log individual results
	for _, r := range results {
		if r.Passed {
			slog.Info("PASS", "name", r.Name, "sql", r.GeneratedSQL)
		} else {
			slog.Error("FAIL", "name", r.Name, "error", r.Error, "expected", r.ExpectedSQL, "got", r.GeneratedSQL)
		}
	}

	slog.Info("Eval summary",
		"passed", summary.Passed,
		"failed", summary.Failed,
		"total", summary.Total,
		"pass_rate", summary.PassRate,
	)

	if evalErr != nil {
		slog.Error("BUILD FAILED: Evals did not pass", "error", evalErr)
		os.Exit(1)
	}

	slog.Info("BUILD OK: All evals passed")
}


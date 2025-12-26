package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/raindrop/nl2sql/pkg/shared"
)

// Handler is the Vercel serverless function entry point for evals
func Handler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		slog.Warn("Method not allowed", "method", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}

	slog.Info("Running evals")

	// Load config from environment
	cfg, err := shared.LoadConfig()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "server configuration error"})
		return
	}

	// Initialize clients
	tinybird := shared.NewTinybirdClient(cfg)
	openai := shared.NewOpenAIClient(cfg)

	// Fetch schema
	schemaStart := time.Now()
	schema, err := tinybird.FetchSchema()
	if err != nil {
		slog.Error("Failed to fetch schema", "error", err, "duration", time.Since(schemaStart))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to fetch schema"})
		return
	}
	openai.SetSchema(schema)
	slog.Debug("Schema loaded", "tables", len(schema.Datasources), "duration", time.Since(schemaStart))

	// Run evals
	evalStart := time.Now()
	results, evalErr := shared.RunEvals(openai, tinybird)
	summary := shared.ComputeSummary(results)

	// Log individual results
	for _, r := range results {
		if r.Passed {
			slog.Info("PASS", "name", r.Name, "sql", r.GeneratedSQL)
		} else {
			slog.Warn("FAIL", "name", r.Name, "error", r.Error, "expected", r.ExpectedSQL, "got", r.GeneratedSQL)
		}
	}

	slog.Info("Eval summary",
		"passed", summary.Passed,
		"failed", summary.Failed,
		"total", summary.Total,
		"pass_rate", summary.PassRate,
		"eval_duration", time.Since(evalStart),
		"total_duration", time.Since(start),
	)

	response := map[string]interface{}{
		"results": results,
		"summary": summary,
		"passed":  evalErr == nil,
	}
	if evalErr != nil {
		response["error"] = evalErr.Error()
	}

	json.NewEncoder(w).Encode(response)
}


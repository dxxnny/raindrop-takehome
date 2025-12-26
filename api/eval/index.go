package handler

import (
	"encoding/json"
	"net/http"

	"github.com/raindrop/nl2sql/pkg/shared"
)

// Handler is the Vercel serverless function entry point for evals
func Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}

	// Load config from environment
	cfg, err := shared.LoadConfig()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "server configuration error"})
		return
	}

	// Initialize clients
	tinybird := shared.NewTinybirdClient(cfg)
	openai := shared.NewOpenAIClient(cfg)

	// Fetch schema
	schema, err := tinybird.FetchSchema()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to fetch schema"})
		return
	}
	openai.SetSchema(schema)

	// Run evals
	results, evalErr := shared.RunEvals(openai, tinybird)
	summary := shared.ComputeSummary(results)

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


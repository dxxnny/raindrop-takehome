package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/raindrop/nl2sql/pkg/shared"
)

type QueryRequest struct {
	Query string `json:"query"`
}

type QueryResponse struct {
	SQL   string                   `json:"sql"`
	Data  []map[string]interface{} `json:"data"`
	Rows  int                      `json:"rows"`
	Error string                   `json:"error,omitempty"`
}

// Handler is the Vercel serverless function entry point
func Handler(w http.ResponseWriter, r *http.Request) {
	// CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(QueryResponse{Error: "method not allowed"})
		return
	}

	// Load config from environment
	cfg, err := shared.LoadConfig()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(QueryResponse{Error: "server configuration error"})
		return
	}

	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(QueryResponse{Error: "invalid request body"})
		return
	}

	if req.Query == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(QueryResponse{Error: "query is required"})
		return
	}

	// Initialize clients
	tinybird := shared.NewTinybirdClient(cfg)
	openai := shared.NewOpenAIClient(cfg)

	// Fetch schema (this happens on every request in serverless - no caching)
	schema, err := tinybird.FetchSchema()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(QueryResponse{Error: "failed to fetch schema"})
		return
	}
	openai.SetSchema(schema)

	// Generate SQL using GPT-5 with CFG
	sql, err := openai.GenerateSQL(req.Query)
	if err != nil {
		var unsupportedErr shared.ErrUnsupportedQuery
		if errors.As(err, &unsupportedErr) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(QueryResponse{Error: unsupportedErr.Reason})
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(QueryResponse{Error: err.Error()})
		return
	}

	// Execute against Tinybird
	result, err := tinybird.ExecuteQuery(sql)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(QueryResponse{
			SQL:   sql,
			Error: err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(QueryResponse{
		SQL:  sql,
		Data: result.Data,
		Rows: result.Rows,
	})
}


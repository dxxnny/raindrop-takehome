package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

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
	Hint  string                   `json:"hint,omitempty"`
}

// Handler is the Vercel serverless function entry point
func Handler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

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
		slog.Warn("Method not allowed", "method", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(QueryResponse{Error: "method not allowed"})
		return
	}

	// Load config from environment
	cfg, err := shared.LoadConfig()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(QueryResponse{Error: "server configuration error"})
		return
	}

	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("Invalid request body", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(QueryResponse{Error: "invalid request body"})
		return
	}

	if req.Query == "" {
		slog.Warn("Empty query received")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(QueryResponse{Error: "query is required"})
		return
	}

	slog.Info("Query received", "query", req.Query)

	// Initialize clients
	tinybird := shared.NewTinybirdClient(cfg)
	openai := shared.NewOpenAIClient(cfg)

	// Fetch schema (this happens on every request in serverless - no caching)
	schemaStart := time.Now()
	schema, err := tinybird.FetchSchema()
	if err != nil {
		slog.Error("Failed to fetch schema", "error", err, "duration", time.Since(schemaStart))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(QueryResponse{Error: "failed to fetch schema"})
		return
	}
	openai.SetSchema(schema)
	slog.Debug("Schema loaded", "tables", len(schema.Datasources), "duration", time.Since(schemaStart))

	// Generate SQL using GPT-5 with CFG
	sqlStart := time.Now()
	sql, err := openai.GenerateSQL(req.Query)
	sqlDuration := time.Since(sqlStart)

	if err != nil {
		var unsupportedErr shared.ErrUnsupportedQuery
		if errors.As(err, &unsupportedErr) {
			slog.Info("Unsupported query", "reason", unsupportedErr.Reason, "duration", sqlDuration)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(QueryResponse{
				Error: unsupportedErr.Reason,
				Hint:  unsupportedErr.AvailableData,
			})
			return
		}

		slog.Error("OpenAI error", "error", err, "duration", sqlDuration)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(QueryResponse{Error: err.Error()})
		return
	}
	slog.Info("SQL generated", "sql", sql, "duration", sqlDuration)

	// Execute against Tinybird
	dbStart := time.Now()
	result, err := tinybird.ExecuteQuery(sql)
	dbDuration := time.Since(dbStart)

	if err != nil {
		slog.Error("Tinybird error", "error", err, "sql", sql, "duration", dbDuration)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(QueryResponse{
			SQL:   sql,
			Error: err.Error(),
		})
		return
	}

	slog.Info("Query executed",
		"rows", result.Rows,
		"db_duration", dbDuration,
		"total_duration", time.Since(start),
	)

	json.NewEncoder(w).Encode(QueryResponse{
		SQL:  sql,
		Data: result.Data,
		Rows: result.Rows,
	})
}

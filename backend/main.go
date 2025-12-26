package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

// Server holds the application dependencies
type Server struct {
	openai   *OpenAIClient
	tinybird *TinybirdClient
}

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

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting server", "name", "Natural Language → SQL")

	// Load .env file (optional - env vars may come from elsewhere)
	if err := godotenv.Load(); err != nil {
		slog.Debug("No .env file found, using environment variables")
	}

	// Load and validate all config - fails hard if anything is missing
	cfg := LoadConfig()
	slog.Info("Config loaded",
		"openai", "ok",
		"tinybird_host", cfg.TinybirdHost,
		"port", cfg.Port,
	)

	tinybird := NewTinybirdClient(cfg)
	openai := NewOpenAIClient(cfg)

	// Fetch schema from Tinybird - FAIL HARD if this doesn't work
	slog.Info("Fetching schema from Tinybird...")
	schema, err := tinybird.FetchSchema()
	if err != nil {
		slog.Error("FATAL: Failed to fetch schema", "error", err)
		os.Exit(1)
	}

	openai.SetSchema(schema)
	slog.Info("Schema loaded", "tables", len(schema.Datasources))
	for _, ds := range schema.Datasources {
		slog.Debug("Table loaded", "name", ds.Name, "columns", len(ds.Columns))
	}

	// Run startup evals
	slog.Info("Running startup evals...")
	results, err := RunStartupEvals(openai, tinybird)
	LogEvalResults(results)
	if err != nil {
		slog.Error("Startup evals failed", "error", err)
		os.Exit(1)
	}
	slog.Info("Startup evals passed")

	srv := &Server{openai: openai, tinybird: tinybird}

	// Serve static files for frontend
	http.Handle("/", http.FileServer(http.Dir("../frontend")))
	http.HandleFunc("/api/eval", srv.handleEval)
	http.HandleFunc("/api/query", srv.handleQuery)

	slog.Info("Server listening", "port", cfg.Port, "url", "http://localhost:"+cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, nil); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}

// handleEval runs evals on demand
func (s *Server) handleEval(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return
	}

	slog.Info("Running evals on demand")

	results, err := RunStartupEvals(s.openai, s.tinybird)
	LogEvalResults(results)

	summary := ComputeSummary(results)
	response := map[string]interface{}{
		"results": results,
		"summary": summary,
		"passed":  err == nil,
	}
	if err != nil {
		response["error"] = err.Error()
	}
	json.NewEncoder(w).Encode(response)
}

// handleQuery handles natural language to SQL conversion
func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
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
		slog.WarnContext(ctx, "Method not allowed", "method", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(QueryResponse{Error: "method not allowed"})
		return
	}

	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.ErrorContext(ctx, "Invalid request body", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(QueryResponse{Error: "invalid request body"})
		return
	}

	if req.Query == "" {
		slog.WarnContext(ctx, "Empty query received")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(QueryResponse{Error: "query is required"})
		return
	}

	slog.InfoContext(ctx, "Query received", "query", req.Query)

	// Generate SQL using GPT-5 with CFG
	slog.DebugContext(ctx, "Calling GPT-5 with CFG", "input", req.Query)
	sqlStart := time.Now()
	sql, err := s.openai.GenerateSQL(req.Query)
	sqlDuration := time.Since(sqlStart)

	if err != nil {
		// Check if the query is unsupported (can't be answered with available data)
		var unsupportedErr ErrUnsupportedQuery
		if errors.As(err, &unsupportedErr) {
			slog.InfoContext(ctx, "Unsupported query", "reason", unsupportedErr.Reason, "duration", sqlDuration)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(QueryResponse{
				Error: unsupportedErr.Reason,
				Hint:  unsupportedErr.AvailableData,
			})
			return
		}

		slog.ErrorContext(ctx, "OpenAI error", "error", err, "duration", sqlDuration)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(QueryResponse{Error: err.Error()})
		return
	}
	slog.InfoContext(ctx, "SQL generated", "sql", sql, "duration", sqlDuration)

	// Execute against Tinybird
	slog.DebugContext(ctx, "Executing query on Tinybird")
	dbStart := time.Now()
	result, err := s.tinybird.ExecuteQuery(sql)
	dbDuration := time.Since(dbStart)

	if err != nil {
		slog.ErrorContext(ctx, "Tinybird error", "error", err, "sql", sql, "duration", dbDuration)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(QueryResponse{
			SQL:   sql,
			Error: err.Error(),
		})
		return
	}

	slog.InfoContext(ctx, "Query executed",
		"rows", result.Rows,
		"db_duration", dbDuration,
		"total_duration", time.Since(start),
	)

	if len(result.Data) > 0 {
		slog.DebugContext(ctx, "Sample result", "row", result.Data[0])
	}

	json.NewEncoder(w).Encode(QueryResponse{
		SQL:  sql,
		Data: result.Data,
		Rows: result.Rows,
	})
}

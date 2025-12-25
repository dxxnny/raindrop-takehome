package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
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

func main() {
	// Setup slog with pretty console output
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting server", "name", "Natural Language → SQL")

	// Load .env file
	if err := godotenv.Load(); err != nil {
		slog.Warn("No .env file found, using environment variables")
	}

	// Check credentials
	if os.Getenv("OPENAI_API_KEY") == "" {
		slog.Error("OPENAI_API_KEY not set")
	} else {
		slog.Info("Config loaded", "key", "OPENAI_API_KEY", "status", "ok")
	}
	if os.Getenv("TINYBIRD_TOKEN") == "" {
		slog.Error("TINYBIRD_TOKEN not set")
	} else {
		slog.Info("Config loaded", "key", "TINYBIRD_TOKEN", "status", "ok")
	}

	openai := NewOpenAIClient()
	tinybird := NewTinybirdClient()

	// Serve static files for frontend
	fs := http.FileServer(http.Dir("../frontend"))
	http.Handle("/", fs)

	// API endpoint
	http.HandleFunc("/api/query", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		start := time.Now()

		// CORS
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Content-Type", "application/json")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != "POST" {
			slog.WarnContext(ctx, "Method not allowed", "method", r.Method)
			json.NewEncoder(w).Encode(QueryResponse{Error: "method not allowed"})
			return
		}

		var req QueryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			slog.ErrorContext(ctx, "Invalid request body", "error", err)
			json.NewEncoder(w).Encode(QueryResponse{Error: "invalid request body"})
			return
		}

		if req.Query == "" {
			slog.WarnContext(ctx, "Empty query received")
			json.NewEncoder(w).Encode(QueryResponse{Error: "query is required"})
			return
		}

		slog.InfoContext(ctx, "Query received", "query", req.Query)

		// Generate SQL using GPT-5 with CFG
		slog.DebugContext(ctx, "Calling GPT-5 with CFG")
		sqlStart := time.Now()
		sql, err := openai.GenerateSQL(req.Query)
		sqlDuration := time.Since(sqlStart)

		if err != nil {
			slog.ErrorContext(ctx, "OpenAI error", "error", err, "duration", sqlDuration)
			json.NewEncoder(w).Encode(QueryResponse{Error: err.Error()})
			return
		}
		slog.InfoContext(ctx, "SQL generated", "sql", sql, "duration", sqlDuration)

		// Execute against Tinybird
		slog.DebugContext(ctx, "Executing query on Tinybird")
		dbStart := time.Now()
		result, err := tinybird.ExecuteQuery(sql)
		dbDuration := time.Since(dbStart)

		if err != nil {
			slog.ErrorContext(ctx, "Tinybird error", "error", err, "sql", sql, "duration", dbDuration)
			json.NewEncoder(w).Encode(QueryResponse{
				SQL:   sql,
				Error: err.Error(),
			})
			return
		}

		// Log result summary
		slog.InfoContext(ctx, "Query executed",
			"rows", result.Rows,
			"db_duration", dbDuration,
			"total_duration", time.Since(start),
		)

		// Log first row as sample
		if len(result.Data) > 0 {
			slog.DebugContext(ctx, "Sample result", "row", result.Data[0])
		}

		json.NewEncoder(w).Encode(QueryResponse{
			SQL:  sql,
			Data: result.Data,
			Rows: result.Rows,
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	slog.Info("Server listening", "port", port, "url", "http://localhost:"+port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}

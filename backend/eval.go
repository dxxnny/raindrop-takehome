package main

import (
	"fmt"
	"log/slog"
	"strings"
)

// EvalResult holds the result of an evaluation
type EvalResult struct {
	Name    string
	Passed  bool
	SQL     string
	Error   string
	Details string
}

// RunStartupEvals runs basic sanity checks to verify the system works
// Returns error if any critical eval fails
func RunStartupEvals(openai *OpenAIClient, tinybird *TinybirdClient) ([]EvalResult, error) {
	evals := []struct {
		name     string
		query    string
		validate func(sql string, rows int) error
	}{
		// Eval 1: Basic COUNT aggregation
		{
			name:  "count_aggregation",
			query: "Count all items",
			validate: func(sql string, rows int) error {
				if !strings.Contains(strings.ToUpper(sql), "COUNT") {
					return fmt.Errorf("expected COUNT in SQL")
				}
				if rows != 1 {
					return fmt.Errorf("expected 1 row, got %d", rows)
				}
				return nil
			},
		},
		// Eval 2: SUM aggregation
		{
			name:  "sum_aggregation",
			query: "What is the total revenue?",
			validate: func(sql string, rows int) error {
				if !strings.Contains(strings.ToUpper(sql), "SUM") {
					return fmt.Errorf("expected SUM in SQL")
				}
				if !strings.Contains(strings.ToLower(sql), "price") {
					return fmt.Errorf("expected price column in SQL")
				}
				if rows != 1 {
					return fmt.Errorf("expected 1 row, got %d", rows)
				}
				return nil
			},
		},
		// Eval 3: AVG aggregation
		{
			name:  "avg_aggregation",
			query: "What is the average shipping cost?",
			validate: func(sql string, rows int) error {
				if !strings.Contains(strings.ToUpper(sql), "AVG") {
					return fmt.Errorf("expected AVG in SQL")
				}
				if rows != 1 {
					return fmt.Errorf("expected 1 row, got %d", rows)
				}
				return nil
			},
		},
	}

	var results []EvalResult
	var criticalFailure error

	for _, e := range evals {
		result := EvalResult{Name: e.name}

		// Generate SQL
		sql, err := openai.GenerateSQL(e.query)
		if err != nil {
			result.Passed = false
			result.Error = err.Error()
			results = append(results, result)
			criticalFailure = fmt.Errorf("eval %s failed: %w", e.name, err)
			continue
		}
		result.SQL = sql

		// Execute query
		resp, err := tinybird.ExecuteQuery(sql)
		if err != nil {
			result.Passed = false
			result.Error = err.Error()
			results = append(results, result)
			criticalFailure = fmt.Errorf("eval %s failed: %w", e.name, err)
			continue
		}

		// Validate
		if err := e.validate(sql, resp.Rows); err != nil {
			result.Passed = false
			result.Details = err.Error()
			results = append(results, result)
			// Validation failures are warnings, not critical
			continue
		}

		result.Passed = true
		results = append(results, result)
	}

	return results, criticalFailure
}

// LogEvalResults logs the eval results
func LogEvalResults(results []EvalResult) {
	for _, r := range results {
		if r.Passed {
			slog.Info("Eval passed", "name", r.Name, "sql", r.SQL)
		} else {
			slog.Warn("Eval failed", "name", r.Name, "error", r.Error, "details", r.Details)
		}
	}
}

package main

import (
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"sync"
	"time"
)

// EvalCase is a test: natural language query + known-correct SQL
type EvalCase struct {
	Name        string
	Query       string
	ExpectedSQL string
	// ReferenceTime is used for time-based queries. If set, this time is passed
	// to the LLM as "current time" so relative expressions like "last 30 hours"
	// produce predictable SQL. If nil, uses actual current time.
	ReferenceTime *time.Time
	// ExpectUnsupported marks this test as expecting ErrUnsupportedQuery.
	// When true, the test passes if the LLM correctly refuses to generate SQL.
	ExpectUnsupported bool
}

// EvalResult holds pass/fail for a single test
type EvalResult struct {
	Name         string `json:"name"`
	Passed       bool   `json:"passed"`
	Query        string `json:"query"`
	ExpectedSQL  string `json:"expected_sql"`
	GeneratedSQL string `json:"generated_sql"`
	Error        string `json:"error,omitempty"`
}

// EvalSummary is just counts
type EvalSummary struct {
	Total    int     `json:"total"`
	Passed   int     `json:"passed"`
	Failed   int     `json:"failed"`
	PassRate float64 `json:"pass_rate"`
}

// refTime creates a pointer to a time.Time for use in EvalCase.ReferenceTime
func refTime(t time.Time) *time.Time {
	return &t
}

// DefaultEvalCases returns the test cases
func DefaultEvalCases() []EvalCase {
	// Fixed reference time for time-based tests: 2024-06-15 12:00:00 UTC
	// This ensures predictable SQL generation across test runs
	fixedTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	return []EvalCase{
		{
			Name:        "count_all",
			Query:       "Count all items",
			ExpectedSQL: "SELECT COUNT(*) FROM order_items;",
		},
		{
			Name:        "total_revenue",
			Query:       "What is the total revenue?",
			ExpectedSQL: "SELECT SUM(price) FROM order_items;",
		},
		{
			Name:        "avg_shipping",
			Query:       "What is the average shipping cost?",
			ExpectedSQL: "SELECT AVG(freight_value) FROM order_items;",
		},
		{
			Name:        "count_expensive",
			Query:       "How many items cost more than 100?",
			ExpectedSQL: "SELECT COUNT(*) FROM order_items WHERE price > 100;",
		},
		// Time-based test case - uses fixed reference time for predictable results
		{
			Name:          "revenue_last_7_days",
			Query:         "What is the total revenue from items with shipping limit date in the last 7 days?",
			ExpectedSQL:   "SELECT SUM(price) FROM order_items WHERE shipping_limit_date > '2024-06-08 12:00:00';",
			ReferenceTime: refTime(fixedTime),
		},
		// Unsupported query tests - these should be rejected
		{
			Name:              "unsupported_weather",
			Query:             "What's the weather like in Tokyo?",
			ExpectUnsupported: true,
		},
		{
			Name:              "unsupported_nonexistent_table",
			Query:             "How many customers are from California?",
			ExpectUnsupported: true,
		},
	}
}

// RunStartupEvals runs all default eval cases in parallel
func RunStartupEvals(openai *OpenAIClient, tinybird *TinybirdClient) ([]EvalResult, error) {
	cases := DefaultEvalCases()
	results := make([]EvalResult, len(cases))

	var wg sync.WaitGroup
	for i, tc := range cases {
		wg.Add(1)
		go func(idx int, tc EvalCase) {
			defer wg.Done()
			results[idx] = runEval(openai, tinybird, tc)
		}(i, tc)
	}
	wg.Wait()

	// Check for failures
	var firstErr error
	for _, r := range results {
		if !r.Passed {
			firstErr = fmt.Errorf("eval %s failed: %s", r.Name, r.Error)
			break
		}
	}

	return results, firstErr
}

// runEval executes one test case
func runEval(openai *OpenAIClient, tinybird *TinybirdClient, tc EvalCase) EvalResult {
	result := EvalResult{
		Name:        tc.Name,
		Query:       tc.Query,
		ExpectedSQL: tc.ExpectedSQL,
	}

	// Handle unsupported query tests - these expect ErrUnsupportedQuery
	if tc.ExpectUnsupported {
		return runUnsupportedEval(openai, tc)
	}

	// Execute expected SQL
	expected, err := tinybird.ExecuteQuery(tc.ExpectedSQL)
	if err != nil {
		result.Error = fmt.Sprintf("expected SQL failed: %v", err)
		return result
	}

	// Generate SQL from natural language
	// Use reference time if provided (for time-based query tests), otherwise use current time
	var generatedSQL string
	if tc.ReferenceTime != nil {
		generatedSQL, err = openai.GenerateSQLWithTime(tc.Query, *tc.ReferenceTime)
	} else {
		generatedSQL, err = openai.GenerateSQL(tc.Query)
	}
	if err != nil {
		result.Error = fmt.Sprintf("generation failed: %v", err)
		return result
	}
	result.GeneratedSQL = generatedSQL

	// Execute generated SQL
	generated, err := tinybird.ExecuteQuery(generatedSQL)
	if err != nil {
		result.Error = fmt.Sprintf("generated SQL failed: %v", err)
		return result
	}

	// Compare: same row count and same values
	if expected.Rows != generated.Rows {
		result.Error = fmt.Sprintf("row count: expected %d, got %d", expected.Rows, generated.Rows)
		return result
	}

	if !dataEqual(expected.Data, generated.Data) {
		result.Error = "data mismatch"
		return result
	}

	result.Passed = true
	return result
}

// runUnsupportedEval tests that a query correctly returns ErrUnsupportedQuery
func runUnsupportedEval(openai *OpenAIClient, tc EvalCase) EvalResult {
	result := EvalResult{
		Name:        tc.Name,
		Query:       tc.Query,
		ExpectedSQL: "(expected to be unsupported)",
	}

	// Generate SQL - we expect this to fail with ErrUnsupportedQuery
	var err error
	if tc.ReferenceTime != nil {
		_, err = openai.GenerateSQLWithTime(tc.Query, *tc.ReferenceTime)
	} else {
		_, err = openai.GenerateSQL(tc.Query)
	}

	if err == nil {
		result.Error = "expected ErrUnsupportedQuery but got valid SQL"
		return result
	}

	var unsupportedErr ErrUnsupportedQuery
	if !errors.As(err, &unsupportedErr) {
		result.Error = fmt.Sprintf("expected ErrUnsupportedQuery but got: %v", err)
		return result
	}

	result.GeneratedSQL = fmt.Sprintf("(refused: %s)", unsupportedErr.Reason)
	result.Passed = true
	return result
}

// dataEqual compares two result sets
func dataEqual(a, b []map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if !rowEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}

// rowEqual compares two rows by their values (ignores column names for single-value aggregates)
func rowEqual(a, b map[string]interface{}) bool {
	// For single-column results (aggregates), just compare the values
	if len(a) == 1 && len(b) == 1 {
		var va, vb interface{}
		for _, v := range a {
			va = v
		}
		for _, v := range b {
			vb = v
		}
		return valuesEqual(va, vb)
	}

	// For multi-column, need matching keys and values
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok || !valuesEqual(va, vb) {
			return false
		}
	}
	return true
}

// valuesEqual compares values with float tolerance
func valuesEqual(a, b interface{}) bool {
	af, aok := toFloat(a)
	bf, bok := toFloat(b)
	if aok && bok {
		if af == bf {
			return true
		}
		diff := af - bf
		if diff < 0 {
			diff = -diff
		}
		avg := (af + bf) / 2
		if avg < 0 {
			avg = -avg
		}
		if avg == 0 {
			return diff < 0.0001
		}
		return diff/avg < 0.0001
	}
	return reflect.DeepEqual(a, b)
}

func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

// ComputeSummary calculates pass/fail counts
func ComputeSummary(results []EvalResult) EvalSummary {
	s := EvalSummary{Total: len(results)}
	for _, r := range results {
		if r.Passed {
			s.Passed++
		} else {
			s.Failed++
		}
	}
	if s.Total > 0 {
		s.PassRate = float64(s.Passed) / float64(s.Total) * 100
	}
	return s
}

// LogEvalResults logs results
func LogEvalResults(results []EvalResult) {
	for _, r := range results {
		if r.Passed {
			slog.Info("PASS", "name", r.Name, "sql", r.GeneratedSQL)
		} else {
			slog.Warn("FAIL", "name", r.Name, "error", r.Error, "expected", r.ExpectedSQL, "got", r.GeneratedSQL)
		}
	}
	s := ComputeSummary(results)
	slog.Info("Eval summary", "passed", s.Passed, "failed", s.Failed, "total", s.Total)
}

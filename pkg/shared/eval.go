package shared

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"
)

// EvalCase is a test: natural language query + known-correct SQL
type EvalCase struct {
	Name              string
	Query             string
	ExpectedSQL       string
	ReferenceTime     *time.Time
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

func refTime(t time.Time) *time.Time {
	return &t
}

// DefaultEvalCases returns the test cases
func DefaultEvalCases() []EvalCase {
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
		{
			Name:          "revenue_last_7_days",
			Query:         "What is the total revenue from the last 7 days?",
			ExpectedSQL:   "SELECT SUM(price) FROM order_items WHERE shipping_limit_date > '2024-06-08 12:00:00';",
			ReferenceTime: refTime(fixedTime),
		},
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

// RunEvals runs all eval cases
func RunEvals(openai *OpenAIClient, tinybird *TinybirdClient) ([]EvalResult, error) {
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

	var firstErr error
	for _, r := range results {
		if !r.Passed {
			firstErr = fmt.Errorf("eval %s failed: %s", r.Name, r.Error)
			break
		}
	}

	return results, firstErr
}

func runEval(openai *OpenAIClient, tinybird *TinybirdClient, tc EvalCase) EvalResult {
	result := EvalResult{
		Name:        tc.Name,
		Query:       tc.Query,
		ExpectedSQL: tc.ExpectedSQL,
	}

	if tc.ExpectUnsupported {
		return runUnsupportedEval(openai, tc)
	}

	expected, err := tinybird.ExecuteQuery(tc.ExpectedSQL)
	if err != nil {
		result.Error = fmt.Sprintf("expected SQL failed: %v", err)
		return result
	}

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

	generated, err := tinybird.ExecuteQuery(generatedSQL)
	if err != nil {
		result.Error = fmt.Sprintf("generated SQL failed: %v", err)
		return result
	}

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

func runUnsupportedEval(openai *OpenAIClient, tc EvalCase) EvalResult {
	result := EvalResult{
		Name:        tc.Name,
		Query:       tc.Query,
		ExpectedSQL: "(expected to be unsupported)",
	}

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

func rowEqual(a, b map[string]interface{}) bool {
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

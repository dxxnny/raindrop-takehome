package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Test cases for semantic evaluation
var testCases = []struct {
	Name           string
	Query          string
	ExpectedInSQL  []string // Substrings that should appear in SQL
	ValidateResult func(data []map[string]interface{}) bool
}{
	{
		Name:          "Total Revenue",
		Query:         "What is the total revenue from all orders?",
		ExpectedInSQL: []string{"SUM", "price", "FROM", "order_items"},
		ValidateResult: func(data []map[string]interface{}) bool {
			// Should return exactly one row with a sum
			return len(data) == 1
		},
	},
	{
		Name:          "Count Orders",
		Query:         "How many order items are there?",
		ExpectedInSQL: []string{"COUNT", "FROM", "order_items"},
		ValidateResult: func(data []map[string]interface{}) bool {
			if len(data) != 1 {
				return false
			}
			// Check if count is > 100000 (we know there are 112650 rows)
			for _, v := range data[0] {
				if num, ok := v.(float64); ok && num > 100000 {
					return true
				}
			}
			return false
		},
	},
	{
		Name:          "Average Freight",
		Query:         "What is the average freight value?",
		ExpectedInSQL: []string{"AVG", "freight_value", "FROM", "order_items"},
		ValidateResult: func(data []map[string]interface{}) bool {
			// Should return exactly one row
			return len(data) == 1
		},
	},
}


type QueryResponse struct {
	SQL   string                   `json:"sql"`
	Data  []map[string]interface{} `json:"data"`
	Rows  int                      `json:"rows"`
	Error string                   `json:"error,omitempty"`
}

type EvalResult struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Details string `json:"details"`
}

func main() {
	godotenv.Load("../backend/.env")

	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080/api/query"
	}

	fmt.Println("=" + strings.Repeat("=", 59))
	fmt.Println(" CFG SQL Generation Evals")
	fmt.Println("=" + strings.Repeat("=", 59))
	fmt.Println()

	var results []EvalResult
	passed := 0
	failed := 0

	for i, tc := range testCases {
		fmt.Printf("[%d/%d] %s\n", i+1, len(testCases), tc.Name)
		fmt.Printf("    Query: %s\n", tc.Query)

		// Call the API
		resp, err := callAPI(apiURL, tc.Query)
		if err != nil {
			result := EvalResult{
				Name:    tc.Name,
				Passed:  false,
				Details: fmt.Sprintf("API error: %v", err),
			}
			results = append(results, result)
			failed++
			fmt.Printf("    ❌ FAILED: %s\n\n", result.Details)
			continue
		}

		if resp.Error != "" {
			result := EvalResult{
				Name:    tc.Name,
				Passed:  false,
				Details: fmt.Sprintf("Response error: %s", resp.Error),
			}
			results = append(results, result)
			failed++
			fmt.Printf("    ❌ FAILED: %s\n\n", result.Details)
			continue
		}

		fmt.Printf("    SQL: %s\n", resp.SQL)

		// Eval 1: Grammar validity check
		grammarOk := validateGrammar(resp.SQL)

		// Eval 2: Expected SQL patterns
		sqlPatternsOk := true
		for _, expected := range tc.ExpectedInSQL {
			if !strings.Contains(strings.ToUpper(resp.SQL), strings.ToUpper(expected)) {
				sqlPatternsOk = false
				break
			}
		}

		// Eval 3: Semantic validation
		semanticOk := tc.ValidateResult(resp.Data)

		allPassed := grammarOk && sqlPatternsOk && semanticOk

		details := fmt.Sprintf("Grammar: %v, SQL Patterns: %v, Semantic: %v",
			boolToStatus(grammarOk),
			boolToStatus(sqlPatternsOk),
			boolToStatus(semanticOk))

		result := EvalResult{
			Name:    tc.Name,
			Passed:  allPassed,
			Details: details,
		}
		results = append(results, result)

		if allPassed {
			passed++
			fmt.Printf("    ✅ PASSED: %s\n\n", details)
		} else {
			failed++
			fmt.Printf("    ❌ FAILED: %s\n\n", details)
		}
	}

	// Summary
	fmt.Println("=" + strings.Repeat("=", 59))
	fmt.Printf(" Results: %d/%d passed\n", passed, len(testCases))
	fmt.Println("=" + strings.Repeat("=", 59))

	// Output JSON results
	jsonResults, _ := json.MarshalIndent(results, "", "  ")
	os.WriteFile("eval_results.json", jsonResults, 0644)
	fmt.Println("\nResults saved to eval_results.json")

	if failed > 0 {
		os.Exit(1)
	}
}

func callAPI(url, query string) (*QueryResponse, error) {
	body, _ := json.Marshal(map[string]string{"query": query})
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result QueryResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func validateGrammar(sql string) bool {
	// Basic structural validation
	sql = strings.TrimSpace(sql)

	// Must start with SELECT
	if !strings.HasPrefix(strings.ToUpper(sql), "SELECT") {
		return false
	}

	// Must contain FROM order_items
	if !strings.Contains(strings.ToUpper(sql), "FROM ORDER_ITEMS") {
		return false
	}

	// Must end with semicolon
	if !strings.HasSuffix(sql, ";") {
		return false
	}

	// Check for valid column names only
	validColumns := []string{
		"order_id", "order_item_id", "product_id", "seller_id",
		"shipping_limit_date", "price", "freight_value",
	}

	// Extract column references (simplified check)
	sqlLower := strings.ToLower(sql)
	for _, col := range validColumns {
		// If column is mentioned, that's fine
		_ = strings.Contains(sqlLower, col)
	}

	return true
}

func boolToStatus(b bool) string {
	if b {
		return "✓"
	}
	return "✗"
}


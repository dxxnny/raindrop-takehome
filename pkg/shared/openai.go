package shared

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type OpenAIClient struct {
	apiKey          string
	grammar         string
	toolDescription string
}

// ErrUnsupportedQuery is returned when the LLM determines the query
// cannot be answered with the available schema.
type ErrUnsupportedQuery struct {
	Reason string
}

func (e ErrUnsupportedQuery) Error() string {
	return e.Reason
}

func NewOpenAIClient(cfg *Config) *OpenAIClient {
	return &OpenAIClient{
		apiKey: cfg.OpenAIAPIKey,
	}
}

// SetSchema updates the grammar and tool description based on schema.
func (c *OpenAIClient) SetSchema(schema *Schema) {
	c.grammar = schema.GenerateGrammar()
	c.toolDescription = schema.GenerateToolDescription()
}

// Request/Response types for OpenAI Responses API
type ResponsesRequest struct {
	Model             string `json:"model"`
	Input             string `json:"input"`
	Tools             []Tool `json:"tools"`
	ParallelToolCalls bool   `json:"parallel_tool_calls"`
}

type Tool struct {
	Type        string      `json:"type"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Format      *ToolFormat `json:"format,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

type ToolFormat struct {
	Type       string `json:"type"`
	Syntax     string `json:"syntax"`
	Definition string `json:"definition"`
}

type CannotAnswerInput struct {
	Reason string `json:"reason"`
}

type ResponsesResponse struct {
	ID     string       `json:"id"`
	Output []OutputItem `json:"output"`
}

type OutputItem struct {
	Type    string `json:"type"`
	Name    string `json:"name,omitempty"`
	Input   string `json:"input,omitempty"`
	CallID  string `json:"call_id,omitempty"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content,omitempty"`
}

func (c *OpenAIClient) GenerateSQL(naturalLanguage string) (string, error) {
	return c.GenerateSQLWithTime(naturalLanguage, time.Now().UTC())
}

// GenerateSQLWithTime generates SQL with a specific reference time.
func (c *OpenAIClient) GenerateSQLWithTime(naturalLanguage string, currentTime time.Time) (string, error) {
	if c.grammar == "" || c.toolDescription == "" {
		return "", fmt.Errorf("schema not set: call SetSchema before GenerateSQL")
	}

	timeStr := currentTime.Format("2006-01-02 15:04:05")

	reqBody := ResponsesRequest{
		Model: "gpt-5",
		Input: fmt.Sprintf(`Convert this natural language query to a valid ClickHouse SQL query.

If the query CAN be answered with the available schema, call the sql_generator tool.
If the query CANNOT be answered (asks for data not in the schema, or is unrelated to the database), call the cannot_answer tool with a brief explanation.

Current UTC time: %s
Use this timestamp for any relative time calculations (e.g., 'last 30 hours' means since %s minus 30 hours).

Query: %s`,
			timeStr, timeStr, naturalLanguage),
		Tools: []Tool{
			{
				Type:        "custom",
				Name:        "sql_generator",
				Description: c.toolDescription,
				Format: &ToolFormat{
					Type:       "grammar",
					Syntax:     "lark",
					Definition: c.grammar,
				},
			},
			{
				Type:        "function",
				Name:        "cannot_answer",
				Description: "Call this when the query cannot be answered with the available database schema. Use this for questions about data that doesn't exist in the tables, or for completely unrelated questions.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"reason": map[string]interface{}{
							"type":        "string",
							"description": "Brief explanation of why this query cannot be answered",
						},
					},
					"required": []string{"reason"},
				},
			},
		},
		ParallelToolCalls: false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/responses", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai error (%d): %s", resp.StatusCode, string(body))
	}

	var result ResponsesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	for _, item := range result.Output {
		if item.Type == "custom_tool_call" && item.Name == "sql_generator" {
			return item.Input, nil
		}

		if item.Type == "function_call" && item.Name == "cannot_answer" {
			var input CannotAnswerInput
			if err := json.Unmarshal([]byte(item.Input), &input); err != nil {
				return "", ErrUnsupportedQuery{Reason: "Query cannot be answered with available data"}
			}
			return "", ErrUnsupportedQuery{Reason: input.Reason}
		}
	}

	return "", fmt.Errorf("no SQL generated in response")
}


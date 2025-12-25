package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type OpenAIClient struct {
	apiKey          string
	grammar         string
	toolDescription string
}

func NewOpenAIClient() *OpenAIClient {
	return &OpenAIClient{
		apiKey: os.Getenv("OPENAI_API_KEY"),
	}
}

// SetSchema updates the grammar and tool description based on schema
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
}

type ToolFormat struct {
	Type       string `json:"type"`
	Syntax     string `json:"syntax"`
	Definition string `json:"definition"`
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
	// Use dynamic grammar if set, otherwise fall back to static
	grammar := c.grammar
	if grammar == "" {
		grammar = ClickHouseGrammar
	}
	toolDesc := c.toolDescription
	if toolDesc == "" {
		toolDesc = ToolDescription
	}

	reqBody := ResponsesRequest{
		Model: "gpt-5",
		Input: fmt.Sprintf("Convert this natural language query to a valid ClickHouse SQL query. Call the sql_generator tool with the query.\n\nQuery: %s", naturalLanguage),
		Tools: []Tool{
			{
				Type:        "custom",
				Name:        "sql_generator",
				Description: toolDesc,
				Format: &ToolFormat{
					Type:       "grammar",
					Syntax:     "lark",
					Definition: grammar,
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

	// Find the tool call output
	for _, item := range result.Output {
		if item.Type == "custom_tool_call" && item.Name == "sql_generator" {
			return item.Input, nil
		}
	}

	return "", fmt.Errorf("no SQL generated in response")
}

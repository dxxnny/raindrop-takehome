package shared

import (
	"fmt"
	"os"
)

// Config holds all application configuration
type Config struct {
	OpenAIAPIKey  string
	TinybirdHost  string
	TinybirdToken string
}

// LoadConfig loads and validates all required environment variables.
// Returns an error if any required variable is missing.
func LoadConfig() (*Config, error) {
	var missing []string

	openaiKey := os.Getenv("OPENAI_API_KEY")
	if openaiKey == "" {
		missing = append(missing, "OPENAI_API_KEY")
	}

	tinybirdHost := os.Getenv("TINYBIRD_HOST")
	if tinybirdHost == "" {
		missing = append(missing, "TINYBIRD_HOST")
	}

	tinybirdToken := os.Getenv("TINYBIRD_TOKEN")
	if tinybirdToken == "" {
		missing = append(missing, "TINYBIRD_TOKEN")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %v", missing)
	}

	return &Config{
		OpenAIAPIKey:  openaiKey,
		TinybirdHost:  tinybirdHost,
		TinybirdToken: tinybirdToken,
	}, nil
}


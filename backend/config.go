package main

import (
	"fmt"
	"os"
)

// Config holds all application configuration
type Config struct {
	OpenAIAPIKey  string
	TinybirdHost  string
	TinybirdToken string
	Port          string
}

// LoadConfig loads and validates all required environment variables.
// Fails hard if any required variable is missing.
func LoadConfig() *Config {
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
		fmt.Fprintf(os.Stderr, "FATAL: missing required environment variables: %v\n", missing)
		os.Exit(1)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return &Config{
		OpenAIAPIKey:  openaiKey,
		TinybirdHost:  tinybirdHost,
		TinybirdToken: tinybirdToken,
		Port:          port,
	}
}


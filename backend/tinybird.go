package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type TinybirdClient struct {
	host  string
	token string
}

type TinybirdResponse struct {
	Meta       []map[string]string      `json:"meta"`
	Data       []map[string]interface{} `json:"data"`
	Rows       int                      `json:"rows"`
	Statistics map[string]interface{}   `json:"statistics"`
}

func NewTinybirdClient(cfg *Config) *TinybirdClient {
	return &TinybirdClient{
		host:  cfg.TinybirdHost,
		token: cfg.TinybirdToken,
	}
}

func (c *TinybirdClient) ExecuteQuery(sql string) (*TinybirdResponse, error) {
	// Strip trailing semicolon - Tinybird doesn't like it with FORMAT JSON
	sql = strings.TrimSuffix(strings.TrimSpace(sql), ";")
	query := fmt.Sprintf("%s FORMAT JSON", sql)
	reqURL := fmt.Sprintf("%s/v0/sql?q=%s", c.host, url.QueryEscape(query))

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tinybird error (%d): %s", resp.StatusCode, string(body))
	}

	var result TinybirdResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

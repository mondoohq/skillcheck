// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package mondoo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const (
	DefaultBaseURL = "https://mondoo.com/ai-agent-security"
	apiPath        = "/api/v1/lookup"
)

// Finding represents a security finding for a skill.
type Finding struct {
	Name       string   `json:"name"`
	Severity   string   `json:"severity"` // critical, high, medium, low, info
	Categories []string `json:"categories"`
	URL        string   `json:"url"`
}

// Client queries the Mondoo AI Agent Security database.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient creates a new Mondoo API client.
func NewClient() *Client {
	return &Client{
		BaseURL: DefaultBaseURL,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Lookup queries the Mondoo database for a skill by name and hash.
func (c *Client) Lookup(name, hash string) ([]Finding, error) {
	findings, err := c.lookupAPI(name, hash)
	if err == nil {
		return findings, nil
	}

	// API not available — return empty findings with a URL for manual lookup
	return nil, nil
}

// SkillURL returns the Mondoo web URL for a skill.
func (c *Client) SkillURL(name string) string {
	return fmt.Sprintf("%s/skills?q=%s", c.BaseURL, url.QueryEscape(name))
}

func (c *Client) lookupAPI(name, hash string) ([]Finding, error) {
	u := fmt.Sprintf("%s%s?name=%s&hash=%s", c.BaseURL, apiPath,
		url.QueryEscape(name), url.QueryEscape(hash))

	resp, err := c.HTTPClient.Get(u)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var findings []Finding
	if err := json.NewDecoder(resp.Body).Decode(&findings); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %w", err)
	}
	return findings, nil
}

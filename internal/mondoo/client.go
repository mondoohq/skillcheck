// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package mondoo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "https://mondoo.com/ai-agent-security"
	searchPath     = "/api/v1/search/hash"
)

// SearchResponse is the response from the hash search API.
type SearchResponse struct {
	Matches []HashMatch   `json:"matches"`
	Reports []SkillReport `json:"reports"`
}

// HashMatch identifies a skill matching a hash.
type HashMatch struct {
	PURL     string `json:"purl"`
	HashType string `json:"hash_type"`
	Hash     string `json:"hash"`
}

// SkillReport is the security report for a skill.
type SkillReport struct {
	PURL         string  `json:"purl"`
	Status       string  `json:"status"` // affected, clean, queued
	RiskScore    float64 `json:"risk_score"`
	Registry     string  `json:"registry"`
	Owner        string  `json:"owner"`
	Skill        string  `json:"skill"`
	Version      string  `json:"version"`
	Summary      string  `json:"summary"`
	FindingCount int     `json:"finding_count"`
	TopSeverity  string  `json:"top_severity"` // critical, high, medium, low
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

// SearchByHash queries the Mondoo database for a skill by its content hash.
// Returns nil, nil if the hash is not found or the API is unavailable.
func (c *Client) SearchByHash(hash string) (*SearchResponse, error) {
	u := fmt.Sprintf("%s%s?q=%s", c.BaseURL, searchPath, url.QueryEscape(hash))

	resp, err := c.HTTPClient.Get(u)
	if err != nil {
		return nil, nil // API unavailable — fail open
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil // not found or server error — fail open
	}

	var result SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil
	}

	if len(result.Matches) == 0 {
		return nil, nil
	}

	return &result, nil
}

// SkillURL returns the Mondoo web URL for a skill name search.
func (c *Client) SkillURL(name string) string {
	return fmt.Sprintf("%s/skills?q=%s", c.BaseURL, url.QueryEscape(name))
}

// ReportURL returns the Mondoo web URL for a specific skill report.
// Returns empty string if report is nil or any path component is missing.
func (c *Client) ReportURL(report *SkillReport) string {
	if report == nil || report.Registry == "" || report.Owner == "" || report.Skill == "" || report.Version == "" {
		return ""
	}
	// Skill may contain path separators (e.g. "skills/find-skills"),
	// escape each segment independently to preserve the path structure.
	skillPath := escapePathSegments(report.Skill)
	return fmt.Sprintf("%s/skills/%s/%s/%s/%s",
		c.BaseURL,
		url.PathEscape(report.Registry),
		url.PathEscape(report.Owner),
		skillPath,
		url.PathEscape(report.Version))
}

// escapePathSegments splits on "/" and escapes each segment individually,
// preserving path separators.
func escapePathSegments(s string) string {
	parts := strings.Split(s, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

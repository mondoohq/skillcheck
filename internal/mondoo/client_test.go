// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package mondoo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchByHash_Affected(t *testing.T) {
	// Mock the API response for a known malicious skill
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/search/hash" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		q := r.URL.Query().Get("q")
		if q != "1e85f6f9686e145aca4a124e3b704b9bbea9aa87e08515c1e352eee70f6e6e7a" {
			t.Errorf("unexpected hash: %s", q)
		}

		resp := SearchResponse{
			Matches: []HashMatch{
				{
					PURL:     "pkg:github/vercel-labs/skills@004c73806e35?skill=find-skills",
					HashType: "sha256",
					Hash:     q,
				},
			},
			Reports: []SkillReport{
				{
					PURL:         "pkg:github/vercel-labs/skills@004c73806e35?skill=find-skills",
					Status:       "affected",
					RiskScore:    100,
					Registry:     "github",
					Owner:        "vercel-labs",
					Skill:        "skills/find-skills",
					Version:      "004c73806e35",
					Summary:      "The skill enables arbitrary, global installation of third-party code",
					FindingCount: 10,
					TopSeverity:  "critical",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &Client{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	resp, err := client.SearchByHash("1e85f6f9686e145aca4a124e3b704b9bbea9aa87e08515c1e352eee70f6e6e7a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if len(resp.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(resp.Matches))
	}
	if resp.Matches[0].PURL != "pkg:github/vercel-labs/skills@004c73806e35?skill=find-skills" {
		t.Errorf("unexpected PURL: %s", resp.Matches[0].PURL)
	}
	if len(resp.Reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(resp.Reports))
	}
	report := resp.Reports[0]
	if report.Status != "affected" {
		t.Errorf("expected affected, got %s", report.Status)
	}
	if report.RiskScore != 100 {
		t.Errorf("expected risk score 100, got %f", report.RiskScore)
	}
	if report.TopSeverity != "critical" {
		t.Errorf("expected critical, got %s", report.TopSeverity)
	}
}

func TestSearchByHash_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := SearchResponse{Matches: []HashMatch{}, Reports: []SkillReport{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &Client{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	resp, err := client.SearchByHash("0000000000000000000000000000000000000000000000000000000000000000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != nil {
		t.Errorf("expected nil for no matches, got %+v", resp)
	}
}

func TestSearchByHash_APIUnavailable(t *testing.T) {
	client := &Client{
		BaseURL:    "http://localhost:1", // nothing listening
		HTTPClient: http.DefaultClient,
	}

	resp, err := client.SearchByHash("anything")
	if err != nil {
		t.Fatalf("should fail open, got error: %v", err)
	}
	if resp != nil {
		t.Errorf("should return nil when API unavailable")
	}
}

func TestSearchByHash_LiveAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live API test in short mode")
	}

	client := NewClient()
	resp, err := client.SearchByHash("1e85f6f9686e145aca4a124e3b704b9bbea9aa87e08515c1e352eee70f6e6e7a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response from live API, got nil")
	}
	if len(resp.Reports) == 0 {
		t.Fatal("expected at least one report")
	}
	report := resp.Reports[0]
	if report.Status != "affected" {
		t.Errorf("expected affected, got %s", report.Status)
	}
	if report.TopSeverity != "critical" {
		t.Errorf("expected critical severity, got %s", report.TopSeverity)
	}
	if report.RiskScore < 50 {
		t.Errorf("expected high risk score, got %f", report.RiskScore)
	}
}

// Copyright Mondoo, Inc. 2026
// SPDX-License-Identifier: Apache-2.0

package reporter

import "testing"

// HasCriticalOrHigh decides the scan's exit code (1 when true), which is what
// CI gates on.
func TestHasCriticalOrHigh(t *testing.T) {
	scan := func(severities ...string) *ScanResult {
		var skills []SkillResult
		for _, s := range severities {
			skills = append(skills, SkillResult{Name: "s", TopSeverity: s})
		}
		// A second agent with no skills checks that every agent is looked at.
		return &ScanResult{Agents: []AgentResult{{Platform: "empty"}, {Platform: "a", Skills: skills}}}
	}

	tests := []struct {
		name string
		scan *ScanResult
		want bool
	}{
		{"no agents", &ScanResult{}, false},
		{"agents without skills", scan(), false},
		{"critical", scan("critical"), true},
		{"high", scan("high"), true},
		{"medium", scan("medium"), false},
		{"low", scan("low"), false},
		// Fail-open: a skill the database has not analyzed has no severity.
		{"unknown skill", scan(""), false},
		{"one high among clean skills", scan("", "low", "high", "medium"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.scan.HasCriticalOrHigh(); got != tt.want {
				t.Errorf("HasCriticalOrHigh() = %v, want %v", got, tt.want)
			}
		})
	}
}

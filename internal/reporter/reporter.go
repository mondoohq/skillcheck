// Copyright Mondoo, Inc. 2026
// SPDX-License-Identifier: Apache-2.0

package reporter

// ScanResult holds all scan results.
type ScanResult struct {
	Agents []AgentResult `json:"agents"`
}

// AgentResult holds results for a single AI agent.
type AgentResult struct {
	Platform   string            `json:"platform"`
	ConfigPath string            `json:"configPath"`
	Skills     []SkillResult     `json:"skills,omitempty"`
	Plugins    []PluginResult    `json:"plugins,omitempty"`
	MCPServers []MCPServerResult `json:"mcpServers,omitempty"`
	Rules      []RuleResult      `json:"rules,omitempty"`
}

// SkillResult holds a single skill's scan result.
type SkillResult struct {
	Name        string  `json:"name"`
	Hash        string  `json:"hash"`
	Source      string  `json:"source,omitempty"`
	Status      string  `json:"status,omitempty"`      // affected, clean, unknown
	RiskScore   float64 `json:"riskScore,omitempty"`   // 0-100
	TopSeverity string  `json:"topSeverity,omitempty"` // critical, high, medium, low
	Summary     string  `json:"summary,omitempty"`
	PURL        string  `json:"purl,omitempty"`
	URL         string  `json:"url"`
	// Spec lists the Agent Skills specification checks the skill fails. They
	// are reported but never decide the exit code, which stays on the risk
	// database alone.
	Spec []SpecFinding `json:"spec,omitempty"`
}

// SpecFinding is one Agent Skills specification check a skill fails.
type SpecFinding struct {
	UID      string `json:"uid"`
	Title    string `json:"title"`
	Severity string `json:"severity"`
	// Warning marks a low-severity finding, such as a long body.
	Warning bool `json:"warning,omitempty"`
}

// PluginResult holds a single plugin's info.
type PluginResult struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Enabled bool   `json:"enabled"`
}

// MCPServerResult holds a single MCP server's info.
type MCPServerResult struct {
	Name    string `json:"name"`
	Command string `json:"command,omitempty"`
	URL     string `json:"url,omitempty"`
}

// RuleResult holds a single rule's info: an agent rule file, or a project's
// agent instruction file such as AGENTS.md.
type RuleResult struct {
	Name   string `json:"name"`
	Source string `json:"source,omitempty"`
	Hash   string `json:"hash"`
	// Agent names the agent that reads a project instruction file; empty for
	// AGENTS.md, which many agents read.
	Agent string `json:"agent,omitempty"`
}

// HasCriticalOrHigh returns true if any skill has critical or high-severity reports.
func (r *ScanResult) HasCriticalOrHigh() bool {
	for _, agent := range r.Agents {
		for _, skill := range agent.Skills {
			if skill.TopSeverity == "critical" || skill.TopSeverity == "high" {
				return true
			}
		}
	}
	return false
}

// TotalSkills returns the total number of skills across all agents.
func (r *ScanResult) TotalSkills() int {
	n := 0
	for _, agent := range r.Agents {
		n += len(agent.Skills)
	}
	return n
}

// TotalPlugins returns the total number of plugins across all agents.
func (r *ScanResult) TotalPlugins() int {
	n := 0
	for _, agent := range r.Agents {
		n += len(agent.Plugins)
	}
	return n
}

// TotalMCPServers returns the total number of MCP servers across all agents.
func (r *ScanResult) TotalMCPServers() int {
	n := 0
	for _, agent := range r.Agents {
		n += len(agent.MCPServers)
	}
	return n
}

// Reporter writes scan results to output.
type Reporter interface {
	Report(result *ScanResult) error
}

// SkillsWithSpecFindings returns the number of skills that fail at least one
// specification check.
func (r *ScanResult) SkillsWithSpecFindings() int {
	n := 0
	for _, agent := range r.Agents {
		for _, skill := range agent.Skills {
			if len(skill.Spec) > 0 {
				n++
			}
		}
	}
	return n
}

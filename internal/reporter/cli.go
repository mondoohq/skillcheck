// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package reporter

import (
	"fmt"
	"io"
	"strings"

	"go.mondoo.com/skillcheck/internal/mondoo"
)

// CLIReporter writes scan results as colored terminal output.
type CLIReporter struct {
	Writer  io.Writer
	NoColor bool
	Verbose bool
}

func (r *CLIReporter) Report(result *ScanResult) error {
	if len(result.Agents) == 0 {
		fmt.Fprintln(r.Writer, "No AI agents detected.")
		return nil
	}

	fmt.Fprintf(r.Writer, "Detected %d agent(s)\n\n", len(result.Agents))

	for _, agent := range result.Agents {
		r.printAgent(&agent)
	}

	// Summary
	fmt.Fprintln(r.Writer, strings.Repeat("─", 60))
	fmt.Fprintf(r.Writer, "Summary: %d skill(s), %d plugin(s), %d MCP server(s)\n",
		result.TotalSkills(), result.TotalPlugins(), result.TotalMCPServers())

	if result.HasCriticalOrHigh() {
		fmt.Fprintln(r.Writer, r.colorize("FAIL", "red")+" — critical or high-risk findings detected")
	} else {
		fmt.Fprintln(r.Writer, r.colorize("PASS", "green")+" — no critical or high-risk findings")
	}

	return nil
}

func (r *CLIReporter) printAgent(agent *AgentResult) {
	fmt.Fprintf(r.Writer, "%s %s\n", r.colorize("●", "cyan"), r.colorize(agent.Platform, "bold"))
	fmt.Fprintf(r.Writer, "  %s\n", r.colorize(agent.ConfigPath, "dim"))

	if len(agent.Skills) > 0 {
		fmt.Fprintf(r.Writer, "  Skills (%d):\n", len(agent.Skills))
		for _, skill := range agent.Skills {
			status := r.colorize("✓", "green")
			if len(skill.Findings) > 0 {
				maxSev := maxSeverity(skill.Findings)
				status = r.severityIcon(maxSev)
			}
			hashSuffix := ""
			if skill.Hash != "" {
				hashSuffix = " " + r.colorize(skill.Hash[:12], "dim")
			}
			fmt.Fprintf(r.Writer, "    %s %s%s\n", status, skill.Name, hashSuffix)

			for _, f := range skill.Findings {
				fmt.Fprintf(r.Writer, "      %s %s",
					r.severityBadge(f.Severity), f.Name)
				if len(f.Categories) > 0 {
					fmt.Fprintf(r.Writer, " (%s)", strings.Join(f.Categories, ", "))
				}
				fmt.Fprintln(r.Writer)
			}

			if skill.URL != "" && r.Verbose {
				fmt.Fprintf(r.Writer, "      %s\n", skill.URL)
			}
		}
	}

	if len(agent.Plugins) > 0 {
		fmt.Fprintf(r.Writer, "  Plugins (%d):\n", len(agent.Plugins))
		for _, p := range agent.Plugins {
			enabled := r.colorize("✓", "green")
			if !p.Enabled {
				enabled = r.colorize("○", "dim")
			}
			ver := ""
			if p.Version != "" {
				ver = " v" + p.Version
			}
			fmt.Fprintf(r.Writer, "    %s %s%s\n", enabled, p.Name, ver)
		}
	}

	if len(agent.MCPServers) > 0 {
		fmt.Fprintf(r.Writer, "  MCP Servers (%d):\n", len(agent.MCPServers))
		for _, s := range agent.MCPServers {
			detail := s.Command
			if detail == "" {
				detail = s.URL
			}
			if detail != "" {
				fmt.Fprintf(r.Writer, "    → %s (%s)\n", s.Name, detail)
			} else {
				fmt.Fprintf(r.Writer, "    → %s\n", s.Name)
			}
		}
	}

	if len(agent.Rules) > 0 {
		fmt.Fprintf(r.Writer, "  Rules (%d):\n", len(agent.Rules))
		for _, rule := range agent.Rules {
			hashSuffix := ""
			if rule.Hash != "" {
				hashSuffix = " " + r.colorize(rule.Hash[:12], "dim")
			}
			fmt.Fprintf(r.Writer, "    → %s%s\n", rule.Name, hashSuffix)
		}
	}

	fmt.Fprintln(r.Writer)
}

func (r *CLIReporter) colorize(text, style string) string {
	if r.NoColor {
		return text
	}
	switch style {
	case "red":
		return "\033[31m" + text + "\033[0m"
	case "green":
		return "\033[32m" + text + "\033[0m"
	case "yellow":
		return "\033[33m" + text + "\033[0m"
	case "cyan":
		return "\033[36m" + text + "\033[0m"
	case "bold":
		return "\033[1m" + text + "\033[0m"
	case "dim":
		return "\033[2m" + text + "\033[0m"
	default:
		return text
	}
}

func (r *CLIReporter) severityIcon(severity string) string {
	switch severity {
	case "critical":
		return r.colorize("✗", "red")
	case "high":
		return r.colorize("✗", "red")
	case "medium":
		return r.colorize("!", "yellow")
	case "low":
		return r.colorize("~", "yellow")
	default:
		return r.colorize("✓", "green")
	}
}

func (r *CLIReporter) severityBadge(severity string) string {
	upper := strings.ToUpper(severity)
	switch severity {
	case "critical", "high":
		return r.colorize("["+upper+"]", "red")
	case "medium", "low":
		return r.colorize("["+upper+"]", "yellow")
	default:
		return "[" + upper + "]"
	}
}

func maxSeverity(findings []mondoo.Finding) string {
	order := map[string]int{"critical": 4, "high": 3, "medium": 2, "low": 1, "info": 0}
	max := ""
	maxOrder := -1
	for _, f := range findings {
		if o, ok := order[f.Severity]; ok && o > maxOrder {
			maxOrder = o
			max = f.Severity
		}
	}
	return max
}

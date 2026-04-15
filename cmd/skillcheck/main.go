// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"go.mondoo.com/skillcheck/internal/engine"
	"go.mondoo.com/skillcheck/internal/hasher"
	"go.mondoo.com/skillcheck/internal/mondoo"
	"go.mondoo.com/skillcheck/internal/reporter"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var jsonOutput bool
	var noColor bool
	var verbose bool

	rootCmd := &cobra.Command{
		Use:   "skillcheck",
		Short: "AI agent skill security scanner",
		Long:  "Detects locally installed AI agent skills, computes SHA-256 checksums, and queries the Mondoo AI Agent Security database for known threats.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Detect NO_COLOR environment variable
			if _, ok := os.LookupEnv("NO_COLOR"); ok {
				noColor = true
			}

			return runScan(jsonOutput, noColor, verbose)
		},
		SilenceUsage: true,
	}

	rootCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results as JSON")
	rootCmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	rootCmd.Flags().BoolVar(&verbose, "verbose", false, "Show detailed output including hashes and URLs")

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("skillcheck %s (commit: %s, built: %s)\n", version, commit, date)
		},
	})

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// agentDef defines an AI agent and its MQL query templates.
// ConfigDir is relative to home directory.
type agentDef struct {
	Platform   string
	Resource   string // MQL resource name (e.g., "claude.code")
	ConfigDir  string // config dir relative to home (e.g., ".claude")
	Skills     string // field name for skills
	Plugins    string // field name for plugins
	MCPServers string // field name for MCP servers
	Rules      string // field name for rules
}

var agents = []agentDef{
	{
		Platform:   "Claude Code",
		Resource:   "claude.code",
		ConfigDir:  ".claude",
		Skills:     "skills",
		Plugins:    "plugins",
		MCPServers: "mcpServers",
	},
	{
		Platform:   "OpenAI Codex",
		Resource:   "openai.codex",
		ConfigDir:  ".codex",
		Skills:     "skills",
		Plugins:    "plugins",
		MCPServers: "mcpServers",
	},
	// Additional agents will be added as their MQL resources are merged:
	// cursor, github.copilot, goose, gemini, windsurf, zed
}

// buildQuery constructs an MQL query like: claude.code(configPath: "/Users/chris/.claude").skills
func buildQuery(resource, configPath, field string) string {
	return fmt.Sprintf(`%s(configPath: %q).%s`, resource, configPath, field)
}

func runScan(jsonOutput, noColor, verbose bool) error {
	eng, err := engine.New()
	if err != nil {
		return fmt.Errorf("failed to initialize engine: %w", err)
	}
	defer eng.Close()

	client := mondoo.NewClient()
	result := &reporter.ScanResult{}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to determine home directory: %w", err)
	}

	for _, ag := range agents {
		configPath := filepath.Join(home, ag.ConfigDir)
		agentResult := &reporter.AgentResult{Platform: ag.Platform}

		if ag.Skills != "" {
			query := buildQuery(ag.Resource, configPath, ag.Skills)
			if skills := queryResourceList(eng, query); skills != nil {
				for _, s := range skills {
					skill := extractMap(s)
					if skill == nil {
						continue
					}
					name := getString(skill, "name")
					content := getString(skill, "content")
					hash := ""
					if content != "" {
						hash = hasher.Content(content)
					}
					sr := reporter.SkillResult{
						Name:   name,
						Hash:   hash,
						Source: getString(skill, "source"),
						URL:    client.SkillURL(name),
					}
					if hash != "" {
						findings, _ := client.Lookup(name, hash)
						sr.Findings = findings
					}
					agentResult.Skills = append(agentResult.Skills, sr)
				}
	
			}
		}

		if ag.Plugins != "" {
			query := buildQuery(ag.Resource, configPath, ag.Plugins)
			if plugins := queryResourceList(eng, query); plugins != nil {
				for _, p := range plugins {
					plugin := extractMap(p)
					if plugin == nil {
						continue
					}
					agentResult.Plugins = append(agentResult.Plugins, reporter.PluginResult{
						Name:    getString(plugin, "name"),
						Version: getString(plugin, "version"),
						Enabled: getBool(plugin, "enabled"),
					})
				}
	
			}
		}

		if ag.MCPServers != "" {
			query := buildQuery(ag.Resource, configPath, ag.MCPServers)
			if servers := queryResourceList(eng, query); servers != nil {
				for _, s := range servers {
					server := extractMap(s)
					if server == nil {
						continue
					}
					agentResult.MCPServers = append(agentResult.MCPServers, reporter.MCPServerResult{
						Name:    getString(server, "name"),
						Command: getString(server, "command"),
						URL:     getString(server, "url"),
					})
				}
	
			}
		}

		if ag.Rules != "" {
			query := buildQuery(ag.Resource, configPath, ag.Rules)
			if rules := queryResourceList(eng, query); rules != nil {
				for _, r := range rules {
					rule := extractMap(r)
					if rule == nil {
						continue
					}
					content := getString(rule, "content")
					hash := ""
					if content != "" {
						hash = hasher.Content(content)
					}
					agentResult.Rules = append(agentResult.Rules, reporter.RuleResult{
						Name:   getString(rule, "name"),
						Source: getString(rule, "source"),
						Hash:   hash,
					})
				}
	
			}
		}

		// Only include agents that have actual data
		if len(agentResult.Skills) > 0 || len(agentResult.Plugins) > 0 ||
			len(agentResult.MCPServers) > 0 || len(agentResult.Rules) > 0 {
			result.Agents = append(result.Agents, *agentResult)
		}
	}

	var rep reporter.Reporter
	if jsonOutput {
		rep = &reporter.JSONReporter{Writer: os.Stdout}
	} else {
		rep = &reporter.CLIReporter{
			Writer:  os.Stdout,
			NoColor: noColor,
			Verbose: verbose,
		}
	}

	if err := rep.Report(result); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	if result.HasCriticalOrHigh() {
		os.Exit(1)
	}
	return nil
}

// queryResourceList executes an MQL query that returns a list of resources
// (e.g., "claude.code.skills") and returns the raw list.
func queryResourceList(eng *engine.Engine, query string) []interface{} {
	rawData, err := eng.ExecSingle(query)
	if err != nil || rawData == nil || rawData.Value == nil {
		return nil
	}
	list, ok := rawData.Value.([]interface{})
	if !ok {
		return nil
	}
	return list
}

// Helper functions for extracting typed values from maps

func extractMap(v interface{}) map[string]interface{} {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	return m
}

func getString(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func getBool(m map[string]interface{}, key string) bool {
	v, ok := m[key]
	if !ok || v == nil {
		return false
	}
	b, ok := v.(bool)
	if !ok {
		return false
	}
	return b
}

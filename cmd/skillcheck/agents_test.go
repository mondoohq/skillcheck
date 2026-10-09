// Copyright Mondoo, Inc. 2026
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.mondoo.com/skillcheck/internal/engine"
	"go.mondoo.com/skillcheck/internal/hasher"
)

// newEngine starts the embedded MQL engine for a test.
func newEngine(t *testing.T) *engine.Engine {
	t.Helper()
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	t.Cleanup(eng.Close)
	return eng
}

// TestAgentQueriesRun runs every query in `agents` through the embedded engine.
//
// The scan fails open: queryResourceList turns a query error into "nothing
// found", which reports as clean. So a misspelled resource or field, or one
// renamed by an mql bump, would silently stop the scan from seeing that agent.
// This test is where such a break shows up instead.
func TestAgentQueriesRun(t *testing.T) {
	eng := newEngine(t)

	for _, ag := range agents {
		configPath := filepath.Join(t.TempDir(), ag.ConfigDir)
		if err := os.MkdirAll(configPath, 0o755); err != nil {
			t.Fatal(err)
		}
		for kind, field := range map[string]string{
			"skills": ag.Skills, "plugins": ag.Plugins, "mcpServers": ag.MCPServers, "rules": ag.Rules,
		} {
			if field == "" {
				continue
			}
			t.Run(ag.Platform+"/"+kind, func(t *testing.T) {
				query := buildQuery(ag.Resource, configPath, field)
				rd, err := eng.ExecSingle(query)
				if err != nil {
					t.Fatalf("%s: %v", query, err)
				}
				if rd == nil {
					t.Fatalf("%s: no result", query)
				}
				if rd.Error != nil {
					t.Fatalf("%s: %v", query, rd.Error)
				}
				if rd.Value != nil {
					if _, ok := rd.Value.([]any); !ok {
						t.Fatalf("%s: got %T, want a list", query, rd.Value)
					}
				}
			})
		}
	}
}

// skillsDirFor is where an agent reads skills from, given its configPath.
// Most agents read <configPath>/skills; the exceptions are listed here, so a
// change in mql's discovery for an agent fails this test rather than going
// unnoticed.
func skillsDirFor(ag agentDef, configPath string) (dir string, ok bool) {
	switch ag.Resource {
	case "cline", "warp":
		// Shared ~/.agents/skills directory next to the agent's config dir.
		return filepath.Join(filepath.Dir(configPath), ".agents", "skills"), true
	case "github.copilot":
		// Read from every user's home, independent of configPath, so a
		// fixture under a temp configPath cannot reach it.
		return "", false
	}
	return filepath.Join(configPath, "skills"), true
}

// TestAgentSkillsAreFound plants a SKILL.md where each agent reads skills and
// checks the scan's query finds it with the name, source and content the scan
// hashes.
func TestAgentSkillsAreFound(t *testing.T) {
	eng := newEngine(t)

	for _, ag := range agents {
		if ag.Skills == "" {
			continue
		}
		t.Run(ag.Platform, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "home", ag.ConfigDir)
			if err := os.MkdirAll(configPath, 0o755); err != nil {
				t.Fatal(err)
			}
			skillsDir, ok := skillsDirFor(ag, configPath)
			if !ok {
				t.Skipf("%s reads skills independent of configPath", ag.Platform)
			}

			name := "planted-" + strings.NewReplacer(" ", "-", ".", "-").Replace(strings.ToLower(ag.Platform))
			content := "---\nname: " + name + "\ndescription: Planted by the scan test.\n---\n# Planted\n"
			skillFile := filepath.Join(skillsDir, name, "SKILL.md")
			if err := os.MkdirAll(filepath.Dir(skillFile), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(skillFile, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}

			skills := queryResourceList(eng, buildQuery(ag.Resource, configPath, ag.Skills))
			var found map[string]any
			for _, s := range skills {
				if m := extractMap(s); m != nil && getString(m, "name") == name {
					found = m
				}
			}
			if found == nil {
				t.Fatalf("planted skill %q at %s was not found; got %d skills", name, skillFile, len(skills))
			}
			if got := getString(found, "source"); got != skillFile {
				t.Errorf("source = %q, want %q", got, skillFile)
			}
			if got := getString(found, "content"); hasher.Content(got) != hasher.Content(content) {
				t.Errorf("content hash differs from the file's: got content %q", got)
			}
		})
	}
}

// TestSpecFindingsOnInstalledSkills plants a skill the way a generator that
// copies only SKILL.md would install it: an invalid name and an image that
// was left behind. The scan's spec check must report both on the skill the
// agent query finds.
func TestSpecFindingsOnInstalledSkills(t *testing.T) {
	eng := newEngine(t)
	configPath := filepath.Join(t.TempDir(), ".claude")
	skillFile := filepath.Join(configPath, "skills", "acme-design", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillFile), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: Acme-design\ndescription: Design system.\n---\n![home](screens/home.png)\n"
	if err := os.WriteFile(skillFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	skills := queryResourceList(eng, buildQuery("claude.code", configPath, "skills { name description content source }"))
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}
	source := getString(extractMap(skills[0]), "source")

	specs := newSpecChecker(eng)
	got := map[string]string{}
	for _, f := range specs.findings(source) {
		got[f.UID] = f.Severity
	}
	want := map[string]string{
		"skill-name-valid-slug":        "high",
		"skill-name-matches-directory": "medium",
		"skill-references-exist":       "medium",
	}
	if len(got) != len(want) {
		t.Fatalf("findings = %v, want %v", got, want)
	}
	for uid, sev := range want {
		if got[uid] != sev {
			t.Errorf("finding %s = %q, want %q (all: %v)", uid, got[uid], sev, got)
		}
	}

	if f := specs.findings(filepath.Join(configPath, "rules", "x.md")); f != nil {
		t.Errorf("a source that is not a SKILL.md has findings: %v", f)
	}
}

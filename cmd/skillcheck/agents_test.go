// Copyright Mondoo, Inc. 2026
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"archive/zip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"go.mondoo.com/skillcheck/internal/engine"
	"go.mondoo.com/skillcheck/internal/hasher"
	"go.mondoo.com/skillcheck/internal/mondoo"
	"go.mondoo.com/skillcheck/internal/reporter"
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

// offlineClient is a risk-database client whose lookups find nothing, so a
// test sees the fail-open path without the network.
func offlineClient(t *testing.T) *mondoo.Client {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	c := mondoo.NewClient()
	c.BaseURL = srv.URL
	return c
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanProject(t *testing.T) {
	eng := newEngine(t)
	proj := t.TempDir()
	writeTestFile(t, filepath.Join(proj, ".claude/skills/ok-skill/SKILL.md"), "---\nname: ok-skill\ndescription: d\n---\n# ok\n")
	writeTestFile(t, filepath.Join(proj, "AGENTS.md"), "# Agents\n")
	writeTestFile(t, filepath.Join(proj, "services/api/CLAUDE.md"), "# API\n")
	writeTestFile(t, filepath.Join(proj, ".cursor/rules/style.mdc"), "rule\n")
	writeTestFile(t, filepath.Join(proj, "node_modules/dep/AGENTS.md"), "# dependency\n")

	// A packaged skill with a name that breaks the spec.
	f, err := os.Create(filepath.Join(proj, "pkg.skill"))
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("Pkg/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("---\nname: Pkg\ndescription: d\n---\n")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	pr, err := scanProject(eng, offlineClient(t), newSpecChecker(eng), proj)
	if err != nil {
		t.Fatal(err)
	}

	skills := map[string]reporter.SkillResult{}
	for _, s := range pr.Skills {
		skills[s.Name] = s
	}
	if len(skills) != 2 {
		t.Fatalf("skills = %v, want ok-skill and Pkg", pr.Skills)
	}
	if s := skills["ok-skill"]; s.Hash == "" || s.Status != "unknown" || len(s.Spec) != 0 {
		t.Errorf("ok-skill = %+v, want hashed, unknown, no spec findings", s)
	}
	if s := skills["Pkg"]; len(s.Spec) != 1 || s.Spec[0].UID != "skill-name-valid-slug" {
		t.Errorf("Pkg spec findings = %+v, want skill-name-valid-slug", s.Spec)
	}

	var rules []string
	for _, r := range pr.Rules {
		if r.Hash == "" {
			t.Errorf("instruction file %s has no hash", r.Name)
		}
		rules = append(rules, r.Name+"="+r.Agent)
	}
	sort.Strings(rules)
	want := []string{".cursor/rules/style.mdc=cursor", "AGENTS.md=", "services/api/CLAUDE.md=claude.code"}
	if strings.Join(rules, ",") != strings.Join(want, ",") {
		t.Errorf("instruction files = %v, want %v", rules, want)
	}

	if _, err := scanProject(eng, offlineClient(t), newSpecChecker(eng), filepath.Join(proj, "AGENTS.md")); err == nil {
		t.Error("a project that is not a directory: want an error")
	}
}

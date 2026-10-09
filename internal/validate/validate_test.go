// Copyright Mondoo, Inc. 2026
// SPDX-License-Identifier: Apache-2.0

package validate_test

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"go.mondoo.com/skillcheck/internal/engine"
	"go.mondoo.com/skillcheck/internal/validate"
)

// failedUIDs returns the set of check UIDs that did not pass.
func failedUIDs(res validate.Result) map[string]string {
	out := map[string]string{}
	for _, c := range res.Checks {
		if !c.Pass {
			out[c.UID] = c.Error
		}
	}
	return out
}

// assertFailedExactly checks that exactly the given checks failed: every other
// check must pass, so a fixture proves its failures are specific rather than a
// blanket "everything failed".
func assertFailedExactly(t *testing.T, res validate.Result, want ...string) {
	t.Helper()
	if len(res.Checks) == 0 {
		t.Fatal("no checks ran — policy did not load")
	}
	failed := failedUIDs(res)
	var got []string
	for uid := range failed {
		got = append(got, uid)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("failed checks = %v, want %v (errors: %v)", got, want, failed)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("failed checks = %v, want %v (errors: %v)", got, want, failed)
		}
	}
}

// writeFile writes content to repo/rel, creating parent directories.
func writeFile(t *testing.T, repo, rel, content string) {
	t.Helper()
	p := filepath.Join(repo, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeZip writes a zip archive of name -> content to repo/rel.
func writeZip(t *testing.T, repo, rel string, entries map[string]string) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, repo, rel, buf.String())
}

// skillMD renders a SKILL.md with the given frontmatter lines.
func skillMD(frontmatter ...string) string {
	return "---\n" + strings.Join(frontmatter, "\n") + "\n---\n# Skill\nBody.\n"
}

// conformantRepo writes a repository that passes every check, for a test case
// to break one thing in.
func conformantRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	writeFile(t, repo, "AGENTS.md", "# AGENTS.md\n")
	writeFile(t, repo, ".claude-plugin/marketplace.json", `{"plugins": [{"name": "ok-skill", "source": "./skills/ok-skill"}]}`)
	writeFile(t, repo, "skills/ok-skill/SKILL.md", skillMD("name: ok-skill", "description: A conformant skill."))
	return repo
}

// The name, description and slug checks that a skill with an unreadable
// frontmatter also fails, since none of its fields can be read.
var unreadableSkill = []string{
	"skill-frontmatter-valid", "skill-name-valid-slug", "skill-name-length",
	"skill-name-matches-directory", "skill-description-length",
}

// TestEachCheckCanFail breaks a conformant repository one way at a time and
// asserts exactly which checks fail. A check that no case can make fail is
// not checking anything, so the test also requires every check in the policy
// to fail in at least one case.
func TestEachCheckCanFail(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	t.Cleanup(func() { eng.Close() })

	long := func(n int) string { return strings.Repeat("a", n) }

	tests := []struct {
		name   string
		mutate func(t *testing.T, repo string)
		want   []string
	}{
		{"conformant", func(*testing.T, string) {}, nil},
		{"no skills", func(t *testing.T, repo string) {
			if err := os.RemoveAll(filepath.Join(repo, "skills")); err != nil {
				t.Fatal(err)
			}
		}, []string{"skills-present"}},
		{"no frontmatter", func(t *testing.T, repo string) {
			writeFile(t, repo, "skills/ok-skill/SKILL.md", "# Just markdown\n")
		}, unreadableSkill},
		{"unterminated frontmatter", func(t *testing.T, repo string) {
			writeFile(t, repo, "skills/ok-skill/SKILL.md", "---\nname: ok-skill\ndescription: d\n")
		}, unreadableSkill},
		{"invalid YAML", func(t *testing.T, repo string) {
			writeFile(t, repo, "skills/ok-skill/SKILL.md", skillMD("name: ok-skill", "description: [oops"))
		}, unreadableSkill},
		{"field of the wrong type", func(t *testing.T, repo string) {
			writeFile(t, repo, "skills/ok-skill/SKILL.md", skillMD("name: ok-skill", "description: d", "license: 5"))
		}, []string{"skill-frontmatter-valid"}},
		{"uppercase name", func(t *testing.T, repo string) {
			writeFile(t, repo, "skills/My-Skill/SKILL.md", skillMD("name: My-Skill", "description: d"))
		}, []string{"skill-name-valid-slug"}},
		{"double hyphen in name", func(t *testing.T, repo string) {
			writeFile(t, repo, "skills/ok--skill/SKILL.md", skillMD("name: ok--skill", "description: d"))
		}, []string{"skill-name-valid-slug"}},
		{"name over 64 characters", func(t *testing.T, repo string) {
			writeFile(t, repo, "skills/"+long(65)+"/SKILL.md", skillMD("name: "+long(65), "description: d"))
		}, []string{"skill-name-length"}},
		{"name at 64 characters", func(t *testing.T, repo string) {
			writeFile(t, repo, "skills/"+long(64)+"/SKILL.md", skillMD("name: "+long(64), "description: d"))
		}, nil},
		{"missing name", func(t *testing.T, repo string) {
			writeFile(t, repo, "skills/ok-skill/SKILL.md", skillMD("description: d"))
		}, []string{"skill-name-valid-slug", "skill-name-length", "skill-name-matches-directory"}},
		{"name differs from directory", func(t *testing.T, repo string) {
			writeFile(t, repo, "skills/ok-skill/SKILL.md", skillMD("name: other-skill", "description: d"))
		}, []string{"skill-name-matches-directory"}},
		{"missing description", func(t *testing.T, repo string) {
			writeFile(t, repo, "skills/ok-skill/SKILL.md", skillMD("name: ok-skill"))
		}, []string{"skill-description-length"}},
		{"description over 1024 characters", func(t *testing.T, repo string) {
			writeFile(t, repo, "skills/ok-skill/SKILL.md", skillMD("name: ok-skill", "description: "+long(1025)))
		}, []string{"skill-description-length"}},
		{"description at 1024 characters", func(t *testing.T, repo string) {
			writeFile(t, repo, "skills/ok-skill/SKILL.md", skillMD("name: ok-skill", "description: "+long(1024)))
		}, nil},
		{"compatibility over 500 characters", func(t *testing.T, repo string) {
			writeFile(t, repo, "skills/ok-skill/SKILL.md", skillMD("name: ok-skill", "description: d", "compatibility: "+long(501)))
		}, []string{"skill-compatibility-length"}},
		{"compatibility at 500 characters", func(t *testing.T, repo string) {
			writeFile(t, repo, "skills/ok-skill/SKILL.md", skillMD("name: ok-skill", "description: d", "compatibility: "+long(500)))
		}, nil},
		{"link to a file the skill does not bundle", func(t *testing.T, repo string) {
			// An install that copies SKILL.md alone leaves its images behind.
			writeFile(t, repo, "skills/ok-skill/SKILL.md", skillMD("name: ok-skill", "description: d")+"![home](screens/home.png)\n")
		}, []string{"skill-references-exist"}},
		{"path mentioned in prose is not a reference", func(t *testing.T, repo string) {
			writeFile(t, repo, "skills/ok-skill/SKILL.md", skillMD("name: ok-skill", "description: d")+"For example `scripts/rotate.py`.\n")
		}, nil},
		{"reference to a bundled file", func(t *testing.T, repo string) {
			writeFile(t, repo, "skills/ok-skill/SKILL.md", skillMD("name: ok-skill", "description: d")+"See [the guide](references/GUIDE.md).\n")
			writeFile(t, repo, "skills/ok-skill/references/GUIDE.md", "# Guide\n")
		}, nil},
		{"link to a sibling skill", func(t *testing.T, repo string) {
			// Skills in a plugin link to each other; that resolves.
			writeFile(t, repo, "skills/other-skill/SKILL.md", skillMD("name: other-skill", "description: d"))
			writeFile(t, repo, "skills/ok-skill/SKILL.md", skillMD("name: ok-skill", "description: d")+"See [other](../other-skill/SKILL.md).\n")
		}, nil},
		{"link to a sibling skill that does not exist", func(t *testing.T, repo string) {
			writeFile(t, repo, "skills/ok-skill/SKILL.md", skillMD("name: ok-skill", "description: d")+"See [other](../gone/references/api.md).\n")
		}, []string{"skill-references-exist"}},
		{"body over 500 lines", func(t *testing.T, repo string) {
			writeFile(t, repo, "skills/ok-skill/SKILL.md", "---\nname: ok-skill\ndescription: d\n---\n"+strings.Repeat("line\n", 501))
		}, []string{"skill-body-size"}},
		{"body at 500 lines", func(t *testing.T, repo string) {
			writeFile(t, repo, "skills/ok-skill/SKILL.md", "---\nname: ok-skill\ndescription: d\n---\n"+strings.Repeat("line\n", 500))
		}, nil},
		{"skill package with a valid skill", func(t *testing.T, repo string) {
			writeZip(t, repo, "dist/pkg-skill.skill", map[string]string{
				"pkg-skill/SKILL.md":      skillMD("name: pkg-skill", "description: d") + "Run [go](scripts/go.sh).\n",
				"pkg-skill/scripts/go.sh": "#!/bin/sh\n",
			})
		}, nil},
		{"skill package with a bad name is found", func(t *testing.T, repo string) {
			writeZip(t, repo, "dist/pkg.skill", map[string]string{
				"Pkg/SKILL.md": skillMD("name: Pkg", "description: d"),
			})
		}, []string{"skill-name-valid-slug"}},
		{"skill package that is not a zip", func(t *testing.T, repo string) {
			writeFile(t, repo, "dist/broken.skill", "not a zip")
		}, unreadableSkill},
		{"broken skill nested in a plugin is found", func(t *testing.T, repo string) {
			writeFile(t, repo, "plugins/p/skills/nested/SKILL.md", skillMD("name: not-nested", "description: d"))
		}, []string{"skill-name-matches-directory"}},
		{"broken skill under node_modules is ignored", func(t *testing.T, repo string) {
			writeFile(t, repo, "node_modules/dep/skills/Bad/SKILL.md", "no frontmatter\n")
		}, nil},
		{"no AGENTS.md", func(t *testing.T, repo string) {
			if err := os.Remove(filepath.Join(repo, "AGENTS.md")); err != nil {
				t.Fatal(err)
			}
		}, []string{"agents-md-present"}},
		{"no marketplace.json", func(t *testing.T, repo string) {
			if err := os.Remove(filepath.Join(repo, ".claude-plugin", "marketplace.json")); err != nil {
				t.Fatal(err)
			}
		}, []string{"marketplace-json-valid"}},
		{"marketplace.json without plugins", func(t *testing.T, repo string) {
			writeFile(t, repo, ".claude-plugin/marketplace.json", `{"name": "x"}`)
		}, []string{"marketplace-json-valid"}},
		{"marketplace.json that is not JSON", func(t *testing.T, repo string) {
			writeFile(t, repo, ".claude-plugin/marketplace.json", `{"plugins": [`)
		}, []string{"marketplace-json-valid"}},
	}

	everyCheck := map[string]bool{}
	failedSomewhere := map[string]bool{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := conformantRepo(t)
			tt.mutate(t, repo)
			res, err := validate.Run(eng, repo)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			for _, c := range res.Checks {
				everyCheck[c.UID] = true
				if !c.Pass {
					failedSomewhere[c.UID] = true
				}
			}
			assertFailedExactly(t, res, tt.want...)
		})
	}

	for uid := range everyCheck {
		if !failedSomewhere[uid] {
			t.Errorf("no case makes check %q fail; add one", uid)
		}
	}
}

// A low-severity failure is reported as a warning and does not fail the
// result; any other failure does.
func TestWarningsDoNotFail(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	t.Cleanup(func() { eng.Close() })

	repo := conformantRepo(t)
	writeFile(t, repo, "skills/ok-skill/SKILL.md", "---\nname: ok-skill\ndescription: d\n---\n"+strings.Repeat("line\n", 501))
	res, err := validate.Run(eng, repo)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK() || res.Warnings() != 1 || res.Failed() != 0 {
		t.Fatalf("OK=%v warnings=%d failed=%d, want a single warning that does not fail", res.OK(), res.Warnings(), res.Failed())
	}

	// A new repository: the engine caches resources by path.
	repo = conformantRepo(t)
	writeFile(t, repo, "skills/ok-skill/SKILL.md", skillMD("name: ok-skill", "description: d")+"Read [the design](references/DESIGN.md).\n")
	res, err = validate.Run(eng, repo)
	if err != nil {
		t.Fatal(err)
	}
	if res.OK() || res.Warnings() != 0 || res.Failed() != 1 {
		t.Fatalf("OK=%v warnings=%d failed=%d, want a medium failure that fails", res.OK(), res.Warnings(), res.Failed())
	}
}

func TestSeverity(t *testing.T) {
	for impact, want := range map[int]string{100: "critical", 90: "critical", 89: "high", 70: "high", 69: "medium", 40: "medium", 39: "low", 0: "low"} {
		if got := validate.Severity(impact); got != want {
			t.Errorf("Severity(%d) = %q, want %q", impact, got, want)
		}
	}
}

// The scan runs only the spec policy against an installed skill, which has no
// AGENTS.md or marketplace manifest of its own.
func TestRunSelectsPolicies(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	t.Cleanup(func() { eng.Close() })

	skill := filepath.Join(conformantRepo(t), "skills", "ok-skill")
	res, err := validate.Run(eng, skill, validate.SpecPolicy)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Checks) == 0 {
		t.Fatal("no checks ran")
	}
	for _, c := range res.Checks {
		if c.Policy != validate.SpecPolicy {
			t.Errorf("check %s from policy %s ran", c.UID, c.Policy)
		}
	}
	assertFailedExactly(t, res)

	if _, err := validate.Run(eng, skill, "no-such-policy"); err == nil {
		t.Fatal("unknown policy: want an error")
	}
}

func TestRun(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	t.Cleanup(func() { eng.Close() })

	run := func(t *testing.T, repo string) validate.Result {
		t.Helper()
		res, err := validate.Run(eng, repo)
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		return res
	}

	t.Run("conformant repo passes every check", func(t *testing.T) {
		// Includes a skill nested under plugins/, which must be found and pass.
		res := run(t, "testdata/good")
		assertFailedExactly(t, res)
		// Severity comes from the policy: a check without an impact would
		// silently be a warning.
		for _, c := range res.Checks {
			if c.Impact <= 0 {
				t.Errorf("check %s declares no impact", c.UID)
			}
		}
	})

	t.Run("non-conformant repo fails the right checks", func(t *testing.T) {
		// The uppercase name still equals its directory and is within length,
		// so only the slug check fails among the skill checks.
		assertFailedExactly(t, run(t, "testdata/bad"),
			"skill-name-valid-slug", "agents-md-present", "marketplace-json-valid")
	})

	t.Run("frontmatter that breaks the spec beyond name and description", func(t *testing.T) {
		// A 501-character compatibility, and metadata that is a list, not a map.
		assertFailedExactly(t, run(t, "testdata/spec-violations"),
			"skill-frontmatter-valid", "skill-compatibility-length")
	})

	t.Run("a repository without skills fails instead of passing vacuously", func(t *testing.T) {
		repo := t.TempDir()
		if err := os.WriteFile(filepath.Join(repo, "AGENTS.md"), []byte("# AGENTS.md\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(repo, ".claude-plugin"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, ".claude-plugin", "marketplace.json"), []byte(`{"plugins": []}`), 0o644); err != nil {
			t.Fatal(err)
		}
		assertFailedExactly(t, run(t, repo), "skills-present")
	})
}

// Copyright Mondoo, Inc. 2026
// SPDX-License-Identifier: Apache-2.0

package validate_test

import (
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
		assertFailedExactly(t, run(t, "testdata/good"))
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

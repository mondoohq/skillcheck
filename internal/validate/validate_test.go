// Copyright Mondoo, Inc. 2026
// SPDX-License-Identifier: Apache-2.0

package validate_test

import (
	"os"
	"path/filepath"
	"sort"
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

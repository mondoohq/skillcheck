// Copyright Mondoo, Inc. 2026
// SPDX-License-Identifier: Apache-2.0

package validate_test

import (
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

func TestRun(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	t.Cleanup(func() { eng.Close() })

	t.Run("conformant repo passes every check", func(t *testing.T) {
		res, err := validate.Run(eng, "testdata/good")
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if len(res.Checks) == 0 {
			t.Fatal("no checks ran — policy did not load")
		}
		if !res.OK() {
			t.Fatalf("expected all checks to pass, got %d failed: %v", res.Failed(), failedUIDs(res))
		}
	})

	t.Run("non-conformant repo fails the right checks", func(t *testing.T) {
		res, err := validate.Run(eng, "testdata/bad")
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if res.OK() {
			t.Fatal("expected failures, got none")
		}
		failed := failedUIDs(res)
		for _, uid := range []string{"skill-name-valid-slug", "agents-md-present", "marketplace-json-valid"} {
			if _, ok := failed[uid]; !ok {
				t.Errorf("expected check %q to fail, but it passed", uid)
			}
		}
		// The uppercase name still equals its directory and is within length,
		// so those checks must still pass — the failures are specific.
		for _, c := range res.Checks {
			if c.UID == "skill-name-matches-directory" && !c.Pass {
				t.Errorf("skill-name-matches-directory should pass (name==dir), got error: %s", c.Error)
			}
		}
	})
}

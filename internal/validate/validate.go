// Copyright Mondoo, Inc. 2026
// SPDX-License-Identifier: Apache-2.0

// Package validate checks that a repository follows the Agent Skills
// (https://agentskills.io/specification) and AGENTS.md (https://agents.md/)
// specifications. The rules live in an embedded MQL policy bundle, not in Go, so
// they are data: skillcheck runs each check through its compiled-in MQL engine
// against the target repository, passed in as the property `repo`.
package validate

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"

	"go.mondoo.com/mql/v13/llx"
	"go.mondoo.com/mql/v13/mqlc"
	"gopkg.in/yaml.v3"
)

//go:embed policy/repo-contract.mql.yaml
var policyData []byte

// parsedBundle is the embedded policy, parsed once at startup. The data is an
// immutable embedded blob, so a parse failure is a build/developer error, not a
// runtime condition — panic rather than return it from every Run call.
var parsedBundle bundle

func init() {
	if err := yaml.Unmarshal(policyData, &parsedBundle); err != nil {
		panic(fmt.Sprintf("validate: parse embedded policy: %v", err))
	}
}

// Execer is the slice of the MQL engine the runner needs: run one query with
// properties and get the raw result. *engine.Engine satisfies it.
type Execer interface {
	Exec(query string, props mqlc.PropsHandler) (*llx.RawData, error)
}

// bundle is the subset of the cnspec policy-bundle format the runner executes:
// every check's `mql` is run as a boolean assertion. Filters, props defaults,
// scoring, and remediation are ignored here (a full cnspec run honours them).
type bundle struct {
	Policies []struct {
		UID    string `yaml:"uid"`
		Name   string `yaml:"name"`
		Groups []struct {
			Title  string `yaml:"title"`
			Checks []struct {
				UID   string `yaml:"uid"`
				Title string `yaml:"title"`
				MQL   string `yaml:"mql"`
			} `yaml:"checks"`
		} `yaml:"groups"`
	} `yaml:"policies"`
}

// CheckResult is the outcome of one policy check.
type CheckResult struct {
	Group string `json:"group"`
	UID   string `json:"uid"`
	Title string `json:"title"`
	Pass  bool   `json:"pass"`
	Error string `json:"error,omitempty"`
}

// Result is the outcome of validating a repository.
type Result struct {
	Repo   string        `json:"repo"`
	Policy string        `json:"policy"`
	Checks []CheckResult `json:"checks"`
}

// Passed reports the number of checks that passed.
func (r Result) Passed() int {
	n := 0
	for _, c := range r.Checks {
		if c.Pass {
			n++
		}
	}
	return n
}

// Failed reports the number of checks that did not pass.
func (r Result) Failed() int { return len(r.Checks) - r.Passed() }

// OK reports whether every check passed.
func (r Result) OK() bool { return r.Failed() == 0 }

// Run executes the embedded repository-contract policy against repo and returns
// one CheckResult per check. A check passes only when its MQL evaluates to
// boolean true; a compile/runtime error, a nil result, or any non-true value is
// a failure (fail-closed — an unverifiable rule is not a passing rule).
func Run(eng Execer, repo string) (Result, error) {
	abs, err := filepath.Abs(repo)
	if err != nil {
		return Result{}, fmt.Errorf("resolve repo path: %w", err)
	}

	props := mqlc.SimpleProps{"repo": llx.StringPrimitive(abs)}
	res := Result{Repo: abs}

	for _, p := range parsedBundle.Policies {
		res.Policy = p.UID // NOTE: assumes a single-policy bundle (the embedded one is)
		for _, g := range p.Groups {
			for _, c := range g.Checks {
				cr := CheckResult{Group: g.Title, UID: c.UID, Title: c.Title}
				rd, err := eng.Exec(strings.TrimSpace(c.MQL), props)
				switch {
				case err != nil:
					cr.Error = err.Error()
				case rd == nil:
					cr.Error = "no result"
				case rd.Error != nil:
					cr.Error = rd.Error.Error()
				default:
					v, ok := rd.Value.(bool)
					if ok && v {
						cr.Pass = true
					} else if !ok {
						cr.Error = fmt.Sprintf("check did not evaluate to a boolean (got %T)", rd.Value)
					}
				}
				res.Checks = append(res.Checks, cr)
			}
		}
	}

	return res, nil
}

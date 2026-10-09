// Copyright Mondoo, Inc. 2026
// SPDX-License-Identifier: Apache-2.0

// Package validate checks skills against the Agent Skills specification
// (https://agentskills.io/specification), and a repository against what a skills
// repository ships (AGENTS.md, https://agents.md/, and a marketplace manifest).
// The rules live in an embedded MQL policy bundle, not in Go, so they are data:
// skillcheck runs each check through its compiled-in MQL engine against the
// target path, passed in as the property `repo`.
package validate

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"go.mondoo.com/mql/llx"
	"go.mondoo.com/mql/mqlc"
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

// The policies in the embedded bundle.
const (
	// SpecPolicy checks each skill against the Agent Skills specification.
	SpecPolicy = "mondoo-agent-skills-spec"
	// RepoPolicy checks what a skills repository ships besides its skills.
	RepoPolicy = "mondoo-agent-skills-repo-contract"
)

// Execer is the slice of the MQL engine the runner needs: run one query with
// properties and get the raw result. *engine.Engine satisfies it.
type Execer interface {
	Exec(query string, props mqlc.PropsHandler) (*llx.RawData, error)
}

// bundle is the subset of the cnspec policy-bundle format the runner executes:
// every check's `mql` is run as a boolean assertion, and its `impact` sets the
// severity of a failure. Filters, props defaults, and remediation are ignored
// here (a full cnspec run honours them).
type bundle struct {
	Policies []policy `yaml:"policies"`
}

type policy struct {
	UID    string `yaml:"uid"`
	Name   string `yaml:"name"`
	Groups []struct {
		Title  string `yaml:"title"`
		Checks []struct {
			UID    string `yaml:"uid"`
			Title  string `yaml:"title"`
			Impact int    `yaml:"impact"`
			MQL    string `yaml:"mql"`
		} `yaml:"checks"`
	} `yaml:"groups"`
}

// CheckResult is the outcome of one policy check.
type CheckResult struct {
	Policy   string `json:"policy"`
	Group    string `json:"group"`
	UID      string `json:"uid"`
	Title    string `json:"title"`
	Impact   int    `json:"impact"`
	Severity string `json:"severity"`
	Pass     bool   `json:"pass"`
	// Warning marks a failed check of low severity: reported, but it does not
	// fail the result. A check that could not be evaluated is never a warning.
	Warning bool   `json:"warning,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Result is the outcome of validating a path.
type Result struct {
	Repo   string        `json:"repo"`
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

// Warnings reports the number of checks that failed with low severity.
func (r Result) Warnings() int {
	n := 0
	for _, c := range r.Checks {
		if c.Warning {
			n++
		}
	}
	return n
}

// Failed reports the number of checks that failed, warnings excluded.
func (r Result) Failed() int { return len(r.Checks) - r.Passed() - r.Warnings() }

// OK reports whether no check failed; warnings do not count.
func (r Result) OK() bool { return r.Failed() == 0 }

// Severity names the severity of a failed check from its impact, using
// cnspec's buckets.
func Severity(impact int) string {
	switch {
	case impact >= 90:
		return "critical"
	case impact >= 70:
		return "high"
	case impact >= 40:
		return "medium"
	default:
		return "low"
	}
}

// Run executes the given policies of the embedded bundle, or all of them when
// none are named, against path and returns one CheckResult per check. A check
// passes only when its MQL evaluates to boolean true; a compile/runtime error,
// a nil result, or any non-true value is a failure (fail-closed — an
// unverifiable rule is not a passing rule).
func Run(eng Execer, path string, policies ...string) (Result, error) {
	for _, uid := range policies {
		if !slices.ContainsFunc(parsedBundle.Policies, func(p policy) bool { return p.UID == uid }) {
			return Result{}, fmt.Errorf("unknown policy %q", uid)
		}
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return Result{}, fmt.Errorf("resolve path: %w", err)
	}

	props := mqlc.SimpleProps{"repo": llx.StringPrimitive(abs)}
	res := Result{Repo: abs}

	for _, p := range parsedBundle.Policies {
		if len(policies) > 0 && !slices.Contains(policies, p.UID) {
			continue
		}
		for _, g := range p.Groups {
			for _, c := range g.Checks {
				cr := CheckResult{
					Policy: p.UID, Group: g.Title, UID: c.UID, Title: c.Title,
					Impact: c.Impact, Severity: Severity(c.Impact),
				}
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
				cr.Warning = !cr.Pass && cr.Error == "" && cr.Severity == "low"
				res.Checks = append(res.Checks, cr)
			}
		}
	}

	return res, nil
}

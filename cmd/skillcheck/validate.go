// Copyright Mondoo, Inc. 2026
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.mondoo.com/skillcheck/internal/engine"
	"go.mondoo.com/skillcheck/internal/validate"
)

// newValidateCmd builds the `skillcheck validate` subcommand: it runs the
// embedded Agent Skills + AGENTS.md repository-contract policy against a repo and
// exits non-zero if any check fails. Distinct from the default scan (which hunts
// for known-malicious installed skills) — this checks that a repo you author is
// spec-conformant.
func newValidateCmd() *cobra.Command {
	var jsonOutput bool
	var noColor bool

	cmd := &cobra.Command{
		Use:   "validate [path]",
		Short: "Validate a repository against the Agent Skills and AGENTS.md specs",
		Long: "Runs an embedded MQL policy that checks a repository's skills follow the\n" +
			"Agent Skills spec (https://agentskills.io/specification) — name is a valid\n" +
			"slug matching its directory, description within length — and that the repo\n" +
			"ships a root AGENTS.md (https://agents.md/) and a valid marketplace manifest.",
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, ok := os.LookupEnv("NO_COLOR"); ok {
				noColor = true
			}
			repo := "."
			if len(args) == 1 {
				repo = args[0]
			}

			eng, err := engine.New()
			if err != nil {
				return fmt.Errorf("failed to initialize engine: %w", err)
			}
			defer eng.Close()

			res, err := validate.Run(eng, repo)
			if err != nil {
				return err
			}
			if err := validate.Report(os.Stdout, res, jsonOutput, noColor); err != nil {
				return err
			}
			// Return an error rather than os.Exit so the deferred eng.Close()
			// runs; cobra exits non-zero and SilenceUsage keeps it terse.
			if !res.OK() {
				return fmt.Errorf("%d of %d checks failed", res.Failed(), len(res.Checks))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results as JSON")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	return cmd
}

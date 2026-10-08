// Copyright Mondoo, Inc. 2026
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"encoding/json"
	"fmt"
	"io"
)

const (
	colReset = "\033[0m"
	colGreen = "\033[32m"
	colRed   = "\033[31m"
	colDim   = "\033[2m"
	colBold  = "\033[1m"
)

// Report writes the result to w: pretty, grouped text (optionally colored) or,
// when jsonOut is set, the Result as JSON.
func Report(w io.Writer, res Result, jsonOut, noColor bool) error {
	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	c := func(color, s string) string {
		if noColor {
			return s
		}
		return color + s + colReset
	}

	fmt.Fprintf(w, "%s %s\n\n", c(colBold, "Validating"), res.Repo)

	group := ""
	for _, chk := range res.Checks {
		if chk.Group != group {
			group = chk.Group
			fmt.Fprintf(w, "%s\n", c(colDim, group))
		}
		if chk.Pass {
			fmt.Fprintf(w, "  %s %s\n", c(colGreen, "✓"), chk.Title)
		} else {
			fmt.Fprintf(w, "  %s %s\n", c(colRed, "✗"), chk.Title)
			if chk.Error != "" {
				fmt.Fprintf(w, "      %s\n", c(colDim, chk.Error))
			}
		}
	}

	fmt.Fprintln(w)
	summary := fmt.Sprintf("%d passed, %d failed", res.Passed(), res.Failed())
	if res.OK() {
		fmt.Fprintf(w, "%s %s\n", c(colGreen, "PASS"), summary)
	} else {
		fmt.Fprintf(w, "%s %s\n", c(colRed, "FAIL"), summary)
	}
	return nil
}

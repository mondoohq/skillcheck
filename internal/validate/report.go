// Copyright Mondoo, Inc. 2026
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"encoding/json"
	"fmt"
	"io"

	"go.mondoo.com/skillcheck/internal/ui"
)

// Report writes the result to w: grouped text, colored when w is a terminal
// and noColor is unset, or, when jsonOut is set, the Result as JSON.
func Report(w io.Writer, res Result, jsonOut, noColor bool) error {
	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	st := ui.NewStyles(w, noColor)

	fmt.Fprintf(w, "%s %s\n\n", st.Bold.Render("Validating"), res.Repo)

	group := ""
	for _, chk := range res.Checks {
		if chk.Group != group {
			group = chk.Group
			fmt.Fprintf(w, "%s\n", st.Dim.Render(group))
		}
		if chk.Pass {
			fmt.Fprintf(w, "  %s %s\n", st.OK.Render("✓"), chk.Title)
		} else {
			fmt.Fprintf(w, "  %s %s\n", st.Bad.Render("✗"), chk.Title)
			if chk.Error != "" {
				fmt.Fprintf(w, "      %s\n", st.Dim.Render(chk.Error))
			}
		}
	}

	fmt.Fprintln(w)
	summary := fmt.Sprintf("%d passed, %d failed", res.Passed(), res.Failed())
	if res.OK() {
		fmt.Fprintf(w, "%s %s\n", st.OK.Render("PASS"), summary)
	} else {
		fmt.Fprintf(w, "%s %s\n", st.Bad.Render("FAIL"), summary)
	}
	return nil
}

// Copyright Mondoo, Inc. 2026
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/muesli/termenv"
)

// A report is often piped into a file or another tool, where escape codes are
// noise; color must only reach a terminal.
func TestNoColorWhenNotATerminal(t *testing.T) {
	var buf bytes.Buffer
	st := NewStyles(&buf, false)
	for _, s := range []string{st.OK.Render("ok"), st.Bad.Render("bad"), st.Bold.Render("b"), st.Dim.Render("d")} {
		if strings.Contains(s, "\x1b") {
			t.Fatalf("styled text for a non-terminal has escape codes: %q", s)
		}
	}
}

func TestColorWithAProfile(t *testing.T) {
	// The color path itself works; otherwise the test above proves nothing.
	st := newStyles(&bytes.Buffer{}, termenv.ANSI256)
	if got := st.Bad.Render("bad"); !strings.Contains(got, "\x1b[") {
		t.Fatalf("expected escape codes with an ANSI256 profile, got %q", got)
	}
}

func TestNoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if ColorEnabled(&bytes.Buffer{}) {
		t.Fatal("ColorEnabled with NO_COLOR set")
	}
}

func TestSpinnerSilentWhenNotATerminal(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf, true)
	s.Start("Scanning")
	s.Update("Scanning more")
	s.Stop()
	s.Stop() // safe to call twice
	if buf.Len() != 0 {
		t.Fatalf("spinner wrote to a non-terminal: %q", buf.String())
	}
}

// Copyright Mondoo, Inc. 2026
// SPDX-License-Identifier: Apache-2.0

// Package ui holds the terminal styling shared by the scan and validate output.
package ui

import (
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/muesli/termenv"
)

// Styles renders text for one output stream. Color is used only when that
// stream is a terminal, NO_COLOR is unset, and the caller did not turn it off,
// so piped or redirected output never carries escape codes.
type Styles struct {
	OK     lipgloss.Style
	Bad    lipgloss.Style
	Warn   lipgloss.Style
	Accent lipgloss.Style
	Bold   lipgloss.Style
	Dim    lipgloss.Style
}

// NewStyles returns the styles for writing to w.
func NewStyles(w io.Writer, noColor bool) *Styles {
	if noColor || !ColorEnabled(w) {
		return newStyles(w, termenv.Ascii)
	}
	return newStyles(w, lipgloss.NewRenderer(w).ColorProfile())
}

// newStyles builds the styles for a fixed color profile; tests use it to
// render color without a terminal.
func newStyles(w io.Writer, profile termenv.Profile) *Styles {
	r := lipgloss.NewRenderer(w)
	r.SetColorProfile(profile)
	// The 16 basic ANSI colors, which the terminal's own theme maps to readable
	// shades on a light or dark background. Adaptive colors would have lipgloss
	// query the terminal for its background, which writes escape sequences and
	// can leave the reply in the user's input.
	color := func(c string) lipgloss.Style {
		return r.NewStyle().Foreground(lipgloss.Color(c))
	}
	return &Styles{
		OK:     color("2"),
		Bad:    color("1"),
		Warn:   color("3"),
		Accent: color("6"),
		Bold:   r.NewStyle().Bold(true),
		Dim:    r.NewStyle().Faint(true),
	}
}

// ColorEnabled reports whether w is a terminal that should get color: NO_COLOR
// is unset and w is a TTY (including a Cygwin or MSYS terminal on Windows).
func ColorEnabled(w io.Writer) bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	return IsTerminal(w)
}

// IsTerminal reports whether w is an interactive terminal.
func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

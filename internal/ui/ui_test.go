// Copyright Mondoo, Inc. 2026
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"bytes"
	"strings"
	"sync"
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

// The drawing goroutine and Stop must not race; run with -race.
func TestSpinnerStartStop(t *testing.T) {
	var buf safeBuffer
	s := &Spinner{w: &buf, enabled: true, style: NewStyles(&buf, false)}
	s.Start("one")
	s.Start("again") // a second Start only updates the message
	s.Update("two")
	s.Stop()
	s.Stop()
	if !strings.Contains(buf.String(), "\r\033[K") {
		t.Fatalf("spinner did not draw and clear: %q", buf.String())
	}
}

// safeBuffer is a bytes.Buffer safe for the spinner goroutine and the test.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

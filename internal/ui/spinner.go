// Copyright Mondoo, Inc. 2026
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"io"
	"sync"
	"time"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner shows progress on one line of a terminal while work runs. It is
// meant for stderr, so stdout stays clean for the report. A disabled spinner
// writes nothing.
type Spinner struct {
	w       io.Writer
	enabled bool
	style   *Styles

	mu   sync.Mutex
	msg  string
	stop chan struct{}
	done chan struct{}
}

// NewSpinner returns a spinner on w, enabled only when w is a terminal and
// enabled is true.
func NewSpinner(w io.Writer, enabled bool) *Spinner {
	return &Spinner{w: w, enabled: enabled && IsTerminal(w), style: NewStyles(w, false)}
}

// Start begins drawing with msg.
func (s *Spinner) Start(msg string) {
	if !s.enabled {
		return
	}
	s.mu.Lock()
	s.msg = msg
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	s.mu.Unlock()

	go func() {
		defer close(s.done)
		t := time.NewTicker(80 * time.Millisecond)
		defer t.Stop()
		for i := 0; ; i++ {
			s.mu.Lock()
			fmt.Fprintf(s.w, "\r\033[K%s %s", s.style.Accent.Render(spinnerFrames[i%len(spinnerFrames)]), s.msg)
			s.mu.Unlock()
			select {
			case <-s.stop:
				fmt.Fprint(s.w, "\r\033[K")
				return
			case <-t.C:
			}
		}
	}()
}

// Update changes the message shown.
func (s *Spinner) Update(msg string) {
	if !s.enabled {
		return
	}
	s.mu.Lock()
	s.msg = msg
	s.mu.Unlock()
}

// Stop clears the spinner line. It is safe to call without Start.
func (s *Spinner) Stop() {
	if !s.enabled || s.stop == nil {
		return
	}
	close(s.stop)
	<-s.done
	s.stop = nil
}

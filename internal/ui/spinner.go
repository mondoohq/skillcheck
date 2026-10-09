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
	if s.stop != nil {
		s.msg = msg
		s.mu.Unlock()
		return
	}
	s.msg = msg
	stop, done := make(chan struct{}), make(chan struct{})
	s.stop, s.done = stop, done
	s.mu.Unlock()

	go func() {
		defer close(done)
		t := time.NewTicker(80 * time.Millisecond)
		defer t.Stop()
		for i := 0; ; i++ {
			// Write outside the lock, so a slow terminal cannot block
			// Update or Stop.
			s.mu.Lock()
			msg := s.msg
			s.mu.Unlock()
			fmt.Fprintf(s.w, "\r\033[K%s %s", s.style.Accent.Render(spinnerFrames[i%len(spinnerFrames)]), msg)
			select {
			case <-stop:
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

// Stop clears the spinner line and waits for it to stop drawing. It is safe
// to call without Start, and more than once.
func (s *Spinner) Stop() {
	if !s.enabled {
		return
	}
	s.mu.Lock()
	stop, done := s.stop, s.done
	s.stop, s.done = nil, nil
	s.mu.Unlock()
	if stop == nil {
		return
	}
	close(stop)
	<-done
}

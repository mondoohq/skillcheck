// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package hasher

import (
	"testing"
)

func TestContent(t *testing.T) {
	hash := Content("hello world")
	want := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if hash != want {
		t.Errorf("Content(\"hello world\") = %s, want %s", hash, want)
	}
}

func TestContentEmpty(t *testing.T) {
	hash := Content("")
	want := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if hash != want {
		t.Errorf("Content(\"\") = %s, want %s", hash, want)
	}
}

func TestContentDeterministic(t *testing.T) {
	a := Content("test content")
	b := Content("test content")
	if a != b {
		t.Errorf("Content should be deterministic, got %s and %s", a, b)
	}
}

func TestCombined(t *testing.T) {
	// Order should not matter
	a := Combined([]string{"aaa", "bbb", "ccc"})
	b := Combined([]string{"ccc", "aaa", "bbb"})
	if a != b {
		t.Errorf("Combined should be order-independent, got %s and %s", a, b)
	}
}

func TestCombinedEmpty(t *testing.T) {
	hash := Combined([]string{})
	want := Content("")
	if hash != want {
		t.Errorf("Combined([]) = %s, want %s", hash, want)
	}
}

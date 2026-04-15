// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package hasher

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// Content computes the SHA-256 hash of a skill's content.
func Content(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// Combined computes a combined hash from multiple individual hashes.
// Hashes are sorted before concatenation to ensure deterministic output.
func Combined(hashes []string) string {
	sorted := make([]string, len(hashes))
	copy(sorted, hashes)
	sort.Strings(sorted)

	combined := strings.Join(sorted, "")
	h := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(h[:])
}

// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package reporter

import (
	"encoding/json"
	"io"
)

// JSONReporter writes scan results as JSON.
type JSONReporter struct {
	Writer io.Writer
}

func (r *JSONReporter) Report(result *ScanResult) error {
	enc := json.NewEncoder(r.Writer)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

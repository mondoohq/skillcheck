// Copyright Mondoo, Inc. 2026
// SPDX-License-Identifier: Apache-2.0

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

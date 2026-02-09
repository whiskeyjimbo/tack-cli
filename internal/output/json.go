// Package output handles formatting and rendering of plugin results.
package output

import (
	"encoding/json"
	"io"

	abi "github.com/reglet-dev/reglet-abi"
)

// JSONFormatter outputs results as pretty-printed JSON.
type JSONFormatter struct{}

// Format writes the result data as indented JSON.
// If the result has a non-success status and an error, it prints the full result.
// Otherwise, it prints only result.Data for clean piping to jq.
func (f *JSONFormatter) Format(w io.Writer, result abi.Result, _ json.RawMessage) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	if result.IsSuccess() && result.Data != nil {
		return enc.Encode(result.Data)
	}

	// For failures/errors, output the full result including status and error details
	return enc.Encode(result)
}

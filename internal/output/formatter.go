// Package output handles formatting and rendering of plugin results.
package output

import (
	"encoding/json"
	"fmt"
	"io"

	abi "github.com/reglet-dev/reglet-abi"
)

// Formatter renders a plugin result to the given writer.
type Formatter interface {
	// Format writes the result to w in the formatter's format.
	Format(w io.Writer, result abi.Result, outputSchema json.RawMessage) error
}

// NewFormatter returns a Formatter for the given format name.
// Supported formats: "json", "table", "yaml", "quiet".
func NewFormatter(format string) (Formatter, error) {
	switch format {
	case "json":
		return &JSONFormatter{}, nil
	case "table":
		return &TableFormatter{}, nil
	case "yaml":
		return &YAMLFormatter{}, nil
	case "quiet":
		return &QuietFormatter{}, nil
	default:
		return nil, fmt.Errorf("unsupported output format: %q (supported: json, table, yaml, quiet)", format)
	}
}

// QuietFormatter produces no output. The exit code conveys the result.
// Exit 0 for success, exit 1 for failure/error.
type QuietFormatter struct{}

func (f *QuietFormatter) Format(w io.Writer, result abi.Result, _ json.RawMessage) error {
	// No output in quiet mode
	return nil
}

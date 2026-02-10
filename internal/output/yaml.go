// Package output handles formatting and rendering of plugin results.
package output

import (
	"encoding/json"
	"io"

	abi "github.com/reglet-dev/reglet-abi"
	"gopkg.in/yaml.v3"
)

// YAMLFormatter outputs results as YAML.
type YAMLFormatter struct{}

// Format writes the result data as YAML.
func (f *YAMLFormatter) Format(w io.Writer, result abi.Result, _ json.RawMessage) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	defer func() { _ = enc.Close() }()

	if result.IsSuccess() && result.Data != nil {
		return enc.Encode(result.Data)
	}

	return enc.Encode(result)
}

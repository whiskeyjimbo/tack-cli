// Package output handles formatting and rendering of plugin results.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	abi "github.com/reglet-dev/reglet-abi"
)

// TableFormatter outputs results as a human-readable table.
type TableFormatter struct{}

// Format renders result.Data as a table.
func (f *TableFormatter) Format(w io.Writer, result abi.Result, outputSchema json.RawMessage) error {
	if !result.IsSuccess() {
		// For non-success, print status and message
		_, _ = fmt.Fprintf(w, "Status: %s\n", result.Status)
		if result.Message != "" {
			_, _ = fmt.Fprintf(w, "Message: %s\n", result.Message)
		}
		if result.Error != nil {
			_, _ = fmt.Fprintf(w, "Error: [%s] %s\n", result.Error.Type, result.Error.Message)
		}
		return nil
	}

	if len(result.Data) == 0 {
		_, _ = fmt.Fprintln(w, "(no data)")
		return nil
	}

	// Get column order
	columns := columnsFromSchema(outputSchema)
	if len(columns) == 0 {
		columns = sortedKeys(result.Data)
	}

	// Build table
	table := tablewriter.NewTable(w,
		tablewriter.WithHeaderAutoFormat(tw.Off),
		tablewriter.WithRowAutoWrap(tw.WrapNone),
		tablewriter.WithRendition(tw.Rendition{
			Borders: tw.Border{Top: tw.On, Bottom: tw.On, Left: tw.On, Right: tw.On},
		}),
	)

	// Header: convert snake_case to Title Case
	headers := make([]interface{}, len(columns))
	for i, col := range columns {
		headers[i] = snakeToTitle(col)
	}
	table.Header(headers...)

	// Single row (most plugin results are single-record)
	row := make([]interface{}, len(columns))
	for i, col := range columns {
		row[i] = formatValue(result.Data[col])
	}
	table.Append(row...)

	// Render returns error now
	return table.Render()
}

// columnsFromSchema extracts property names from a JSON Schema object.
// Returns nil if the schema is empty or unparseable.
func columnsFromSchema(schema json.RawMessage) []string {
	if len(schema) == 0 {
		return nil
	}

	// Parse as a generic JSON Schema with ordered properties
	var s struct {
		Properties json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(schema, &s); err != nil {
		return nil
	}
	if len(s.Properties) == 0 {
		return nil
	}

	// json.Unmarshal into map doesn't preserve order, but we can use
	// a json.Decoder to extract keys in document order.
	var props map[string]json.RawMessage
	if err := json.Unmarshal(s.Properties, &props); err != nil {
		return nil
	}

	// Sort for consistency since Go maps are unordered
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// sortedKeys returns the sorted keys of a map.
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// snakeToTitle converts "record_type" to "Record Type".
func snakeToTitle(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

// formatValue converts a value to a display string.
func formatValue(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		// JSON numbers are float64; show as int if no fraction
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%.2f", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case []any:
		parts := make([]string, len(val))
		for i, item := range val {
			parts[i] = formatValue(item)
		}
		return strings.Join(parts, ", ")
	case map[string]any:
		b, _ := json.Marshal(val)
		return string(b)
	default:
		return fmt.Sprintf("%v", val)
	}
}

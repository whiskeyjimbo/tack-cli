package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	abi "github.com/reglet-dev/reglet-abi"
)

func testResult() abi.Result {
	return abi.ResultSuccess("ok", map[string]any{
		"hostname":    "example.com",
		"record_type": "A",
		"records":     []any{"93.184.216.34"},
		"ttl":         float64(300),
	})
}

func TestJSONFormatter(t *testing.T) {
	var buf bytes.Buffer
	f := &JSONFormatter{}
	err := f.Format(&buf, testResult(), nil)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "example.com") {
		t.Errorf("expected 'example.com' in output: %s", output)
	}

	// Should parse as valid JSON
	var data map[string]any
	if err := json.Unmarshal(buf.Bytes(), &data); err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}
}

func TestTableFormatter(t *testing.T) {
	var buf bytes.Buffer
	f := &TableFormatter{}
	err := f.Format(&buf, testResult(), nil)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}

	output := buf.String()
	// Should contain table elements
	if !strings.Contains(output, "example.com") {
		t.Errorf("expected 'example.com' in table output: %s", output)
	}
	if !strings.Contains(output, "Hostname") || !strings.Contains(output, "Record Type") {
		t.Errorf("expected title-cased headers in output: %s", output)
	}
}

func TestYAMLFormatter(t *testing.T) {
	var buf bytes.Buffer
	f := &YAMLFormatter{}
	err := f.Format(&buf, testResult(), nil)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "hostname: example.com") {
		t.Errorf("expected YAML key-value in output: %s", output)
	}
}

func TestNewFormatter_Invalid(t *testing.T) {
	_, err := NewFormatter("xml")
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}

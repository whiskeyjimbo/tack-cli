package cli

import (
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestAddFlagsForOperation(t *testing.T) {
	schemaJSON := `{
		"type": "object",
		"properties": {
			"hostname": {"type": "string", "description": "Target hostname"},
			"record_type": {"type": "string", "enum": ["A", "AAAA", "MX"], "default": "A", "description": "Record type"},
			"timeout_seconds": {"type": "integer", "default": 30, "description": "Timeout"},
			"follow_redirects": {"type": "boolean", "default": true, "description": "Follow redirects"},
			"service": {"type": "string"},
			"operation": {"type": "string"}
		},
		"required": ["hostname"]
	}`

	schema, err := parseConfigSchema(json.RawMessage(schemaJSON))
	if err != nil {
		t.Fatalf("parseConfigSchema: %v", err)
	}

	cmd := &cobra.Command{Use: "test"}
	addFlagsForOperation(cmd, schema, []string{"hostname", "record_type", "timeout_seconds", "follow_redirects"}, nil)

	// Should have 4 flags (service/operation skipped)
	flagCount := 0
	cmd.Flags().VisitAll(func(f *pflag.Flag) { flagCount++ })
	if flagCount != 4 {
		t.Errorf("expected 4 flags, got %d", flagCount)
	}

	// Hostname should be required
	f := cmd.Flags().Lookup("hostname")
	if f == nil {
		t.Fatal("expected 'hostname' flag")
	}

	// record-type should exist (kebab-case)
	f = cmd.Flags().Lookup("record-type")
	if f == nil {
		t.Fatal("expected 'record-type' flag")
	}
	if f.DefValue != "A" {
		t.Errorf("expected default 'A' for record-type, got %q", f.DefValue)
	}

	// timeout-seconds should exist (kebab-case from snake_case)
	f = cmd.Flags().Lookup("timeout-seconds")
	if f == nil {
		t.Fatal("expected 'timeout-seconds' flag")
	}

	// service and operation should NOT exist
	if cmd.Flags().Lookup("service") != nil {
		t.Error("service flag should be skipped")
	}
	if cmd.Flags().Lookup("operation") != nil {
		t.Error("operation flag should be skipped")
	}
}

func TestAddFlagsForOperation_InputFieldsFilter(t *testing.T) {
	schemaJSON := `{
		"type": "object",
		"properties": {
			"hostname": {"type": "string"},
			"record_type": {"type": "string"},
			"nameserver": {"type": "string"},
			"region": {"type": "string"}
		}
	}`

	schema, err := parseConfigSchema(json.RawMessage(schemaJSON))
	if err != nil {
		t.Fatalf("parseConfigSchema: %v", err)
	}

	cmd := &cobra.Command{Use: "test"}
	// Only hostname and record_type are in input_fields
	addFlagsForOperation(cmd, schema, []string{"hostname", "record_type"}, nil)

	flagCount := 0
	cmd.Flags().VisitAll(func(f *pflag.Flag) { flagCount++ })
	if flagCount != 2 {
		t.Errorf("expected 2 flags (filtered by input_fields), got %d", flagCount)
	}

	if cmd.Flags().Lookup("hostname") == nil {
		t.Error("expected 'hostname' flag")
	}
	if cmd.Flags().Lookup("record-type") == nil {
		t.Error("expected 'record-type' flag")
	}
	if cmd.Flags().Lookup("nameserver") != nil {
		t.Error("'nameserver' should be filtered out")
	}
	if cmd.Flags().Lookup("region") != nil {
		t.Error("'region' should be filtered out")
	}
}

func TestBuildConfigFromFlags(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("hostname", "", "")
	cmd.Flags().String("record-type", "", "")

	// Simulate setting flags
	_ = cmd.Flags().Set("hostname", "example.com")
	_ = cmd.Flags().Set("record-type", "MX")

	config := buildConfigFromFlags(cmd, "dns", "resolve")

	if config["service"] != "dns" {
		t.Errorf("expected service='dns', got %v", config["service"])
	}
	if config["operation"] != "resolve" {
		t.Errorf("expected operation='resolve', got %v", config["operation"])
	}
	if config["hostname"] != "example.com" {
		t.Errorf("expected hostname='example.com', got %v", config["hostname"])
	}
	// Should be snake_case in config
	if config["record_type"] != "MX" {
		t.Errorf("expected record_type='MX', got %v", config["record_type"])
	}
}

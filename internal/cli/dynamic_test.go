package cli

import (
	"encoding/json"
	"testing"

	abi "github.com/reglet-dev/reglet-abi"
)

func TestGeneratePluginCommand_SingleService(t *testing.T) {
	manifest := abi.Manifest{
		Name:        "dns",
		Description: "DNS resolution",
		Services: map[string]abi.ServiceManifest{
			"dns": {
				Name:        "dns",
				Description: "DNS operations",
				Operations: []abi.OperationManifest{
					{Name: "resolve", Description: "Resolve hostname"},
				},
			},
		},
	}

	outputFormat := "json"
	verbose := false
	trustPlugins := false
	loader := func() ([]byte, error) { return nil, nil }
	cmd := generatePluginCommand(manifest, loader, &outputFormat, &verbose, &trustPlugins, nil)

	if cmd.Use != "dns" {
		t.Errorf("expected Use='dns', got %q", cmd.Use)
	}

	// Single service: operations should be direct subcommands
	if len(cmd.Commands()) != 1 {
		t.Fatalf("expected 1 subcommand, got %d", len(cmd.Commands()))
	}
	if cmd.Commands()[0].Use != "resolve" {
		t.Errorf("expected subcommand 'resolve', got %q", cmd.Commands()[0].Use)
	}
}

func TestGeneratePluginCommand_MultiService(t *testing.T) {
	manifest := abi.Manifest{
		Name:        "aws",
		Description: "AWS operations",
		Services: map[string]abi.ServiceManifest{
			"iam": {
				Name: "iam",
				Operations: []abi.OperationManifest{
					{Name: "get_account_summary", Description: "Check root MFA"},
				},
			},
			"ec2": {
				Name: "ec2",
				Operations: []abi.OperationManifest{
					{Name: "describe_security_groups", Description: "Find open SGs"},
				},
			},
		},
	}

	outputFormat := "json"
	verbose := false
	trustPlugins := false
	loader := func() ([]byte, error) { return nil, nil }
	cmd := generatePluginCommand(manifest, loader, &outputFormat, &verbose, &trustPlugins, nil)

	if len(cmd.Commands()) != 2 {
		t.Fatalf("expected 2 service subcommands, got %d", len(cmd.Commands()))
	}
}

func TestInputJSONToFlags(t *testing.T) {
	input := json.RawMessage(`{"hostname": "example.com", "record_type": "A"}`)
	flags := inputJSONToFlags(input)

	if flags == "" {
		t.Error("expected non-empty flags")
	}
	// Should contain --hostname and --record-type (kebab-case)
	if !containsSubstring(flags, "--hostname") {
		t.Errorf("expected --hostname in flags: %s", flags)
	}
	if !containsSubstring(flags, "--record-type") {
		t.Errorf("expected --record-type in flags: %s", flags)
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

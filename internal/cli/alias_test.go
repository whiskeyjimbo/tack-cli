package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestRegisterAliases(t *testing.T) {
	root := &cobra.Command{Use: "cli"}

	aliases := map[string]string{
		"sg":      "aws ec2 describe_security_groups",
		"buckets": "aws s3 list_buckets",
	}

	registerAliases(root, aliases)

	// Should have 2 alias commands
	if len(root.Commands()) != 2 {
		t.Errorf("expected 2 commands, got %d", len(root.Commands()))
	}

	// Find sg command
	var sgCmd *cobra.Command
	for _, c := range root.Commands() {
		if c.Use == "sg" {
			sgCmd = c
			break
		}
	}

	if sgCmd == nil {
		t.Fatal("expected 'sg' alias command")
	}
	if sgCmd.Short != "Alias for: aws ec2 describe_security_groups" {
		t.Errorf("unexpected short description: %q", sgCmd.Short)
	}
}

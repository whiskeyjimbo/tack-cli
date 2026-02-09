package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/reglet-dev/cli/internal/config"
)

func TestCompletionCommand_Bash(t *testing.T) {
	root := NewRootCommand(config.DefaultConfig(), nil)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"completion", "bash"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "bash") && !strings.Contains(output, "complete") {
		t.Errorf("expected bash completion script, got: %s", output[:min(200, len(output))])
	}
}

func TestCompletionCommand_Zsh(t *testing.T) {
	root := NewRootCommand(config.DefaultConfig(), nil)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"completion", "zsh"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	output := buf.String()
	if len(output) == 0 {
		t.Error("expected non-empty zsh completion output")
	}
}

func TestCompletionCommand_Fish(t *testing.T) {
	root := NewRootCommand(config.DefaultConfig(), nil)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"completion", "fish"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	output := buf.String()
	if len(output) == 0 {
		t.Error("expected non-empty fish completion output")
	}
}

func TestCompletionCommand_InvalidShell(t *testing.T) {
	root := NewRootCommand(config.DefaultConfig(), nil)

	root.SetArgs([]string{"completion", "invalid"})

	err := root.Execute()
	if err == nil {
		t.Error("expected error for invalid shell")
	}
}

func TestCompletionCommand_NoArgs(t *testing.T) {
	root := NewRootCommand(config.DefaultConfig(), nil)

	root.SetArgs([]string{"completion"})

	err := root.Execute()
	if err == nil {
		t.Error("expected error when no shell specified")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

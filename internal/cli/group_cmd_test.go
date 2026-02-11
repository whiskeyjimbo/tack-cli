package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/whiskeyjimb/tack-cli/internal/config"
)

func TestGroupList_Empty(t *testing.T) {
	cfg := config.DefaultConfig()
	cmd := newGroupCommand(cfg, "")

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"list"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "No groups configured") {
		t.Errorf("expected 'No groups configured', got %q", buf.String())
	}
}

func TestGroupList_WithGroups(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Groups = map[string]config.GroupConfig{
		"network": {Description: "Network tools", Plugins: []string{"dns", "http"}},
	}
	cmd := newGroupCommand(cfg, "")

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"list"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "network") {
		t.Errorf("expected 'network' in output, got %q", output)
	}
	if !strings.Contains(output, "dns, http") {
		t.Errorf("expected 'dns, http' in output, got %q", output)
	}
}

func TestGroupCreate(t *testing.T) {
	cfg := config.DefaultConfig()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cmd := newGroupCommand(cfg, path)

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"create", "network", "--description", "Network tools"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, exists := cfg.Groups["network"]; !exists {
		t.Error("expected group 'network' to be created")
	}

	if cfg.Groups["network"].Description != "Network tools" {
		t.Errorf("expected description 'Network tools', got %q", cfg.Groups["network"].Description)
	}

	// Verify file was written
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected config file to be created")
	}
}

func TestGroupCreate_Duplicate(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Groups = map[string]config.GroupConfig{
		"network": {Description: "Network tools", Plugins: []string{"dns"}},
	}

	cmd := newGroupCommand(cfg, "")
	cmd.SetArgs([]string{"create", "network"})

	if err := cmd.Execute(); err == nil {
		t.Error("expected error for duplicate group")
	}
}

func TestGroupCreate_ReservedName(t *testing.T) {
	cfg := config.DefaultConfig()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cmd := newGroupCommand(cfg, path)
	cmd.SetArgs([]string{"create", "plugin"})

	if err := cmd.Execute(); err == nil {
		t.Error("expected error for reserved name")
	}

	if _, exists := cfg.Groups["plugin"]; exists {
		t.Error("reserved group should not have been created")
	}
}

func TestGroupDelete(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Groups = map[string]config.GroupConfig{
		"network": {Description: "Network tools", Plugins: []string{"dns"}},
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cmd := newGroupCommand(cfg, path)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"delete", "network"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, exists := cfg.Groups["network"]; exists {
		t.Error("expected group to be deleted")
	}
}

func TestGroupDelete_NotFound(t *testing.T) {
	cfg := config.DefaultConfig()

	cmd := newGroupCommand(cfg, "")
	cmd.SetArgs([]string{"delete", "nonexistent"})

	if err := cmd.Execute(); err == nil {
		t.Error("expected error for non-existent group")
	}
}

func TestGroupDelete_TopGroup(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Groups = map[string]config.GroupConfig{
		"top": {Description: "Top-level plugins", Plugins: []string{"dns"}},
	}

	cmd := newGroupCommand(cfg, "")
	cmd.SetArgs([]string{"delete", "top"})

	if err := cmd.Execute(); err == nil {
		t.Error("expected error when trying to delete 'top' group")
	} else if !strings.Contains(err.Error(), "cannot delete") {
		t.Errorf("expected 'cannot delete' error message, got: %v", err)
	}
}

func TestGroupAdd(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Groups = map[string]config.GroupConfig{
		"network": {Description: "Network tools", Plugins: []string{"dns"}},
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cmd := newGroupCommand(cfg, path)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"add", "network", "http", "tcp"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	group := cfg.Groups["network"]
	if len(group.Plugins) != 3 {
		t.Fatalf("expected 3 plugins, got %d: %v", len(group.Plugins), group.Plugins)
	}
}

func TestGroupAdd_Duplicate(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Groups = map[string]config.GroupConfig{
		"network": {Description: "Network tools", Plugins: []string{"dns"}},
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cmd := newGroupCommand(cfg, path)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"add", "network", "dns"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	group := cfg.Groups["network"]
	if len(group.Plugins) != 1 {
		t.Errorf("expected 1 plugin (no duplicate), got %d", len(group.Plugins))
	}
}

func TestGroupAdd_GroupNotFound(t *testing.T) {
	cfg := config.DefaultConfig()

	cmd := newGroupCommand(cfg, "")
	cmd.SetArgs([]string{"add", "nonexistent", "dns"})

	if err := cmd.Execute(); err == nil {
		t.Error("expected error for non-existent group")
	}
}

func TestGroupRemove(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Groups = map[string]config.GroupConfig{
		"network": {Description: "Network tools", Plugins: []string{"dns", "http", "tcp"}},
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cmd := newGroupCommand(cfg, path)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"remove", "network", "http"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	group := cfg.Groups["network"]
	if len(group.Plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d: %v", len(group.Plugins), group.Plugins)
	}
	for _, p := range group.Plugins {
		if p == "http" {
			t.Error("http should have been removed")
		}
	}
}

func TestGroupRemove_PluginNotInGroup(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Groups = map[string]config.GroupConfig{
		"network": {Description: "Network tools", Plugins: []string{"dns"}},
	}

	cmd := newGroupCommand(cfg, "")
	cmd.SetArgs([]string{"remove", "network", "http"})

	if err := cmd.Execute(); err == nil {
		t.Error("expected error for plugin not in group")
	}
}

func TestGroupRemove_FromTopWithoutOtherGroup(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Groups = map[string]config.GroupConfig{
		"top": {Description: "Top-level", Plugins: []string{"aws", "dns"}},
	}

	cmd := newGroupCommand(cfg, "")
	cmd.SetArgs([]string{"remove", "top", "aws"})

	if err := cmd.Execute(); err == nil {
		t.Error("expected error when removing plugin from 'top' that's not in any other group")
	} else if !strings.Contains(err.Error(), "not in any other group") {
		t.Errorf("expected 'not in any other group' error, got: %v", err)
	}
}

func TestGroupRemove_FromTopWithOtherGroup(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Groups = map[string]config.GroupConfig{
		"top":     {Description: "Top-level", Plugins: []string{"aws", "dns"}},
		"network": {Description: "Network tools", Plugins: []string{"dns"}},
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cmd := newGroupCommand(cfg, path)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"remove", "top", "dns"})

	// Should succeed because dns is in "network" group
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	group := cfg.Groups["top"]
	if len(group.Plugins) != 1 || group.Plugins[0] != "aws" {
		t.Errorf("expected only 'aws' in top group, got: %v", group.Plugins)
	}
}

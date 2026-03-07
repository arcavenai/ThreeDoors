package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/arcaven/ThreeDoors/internal/core"
)

func setupTestConfig(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	core.SetHomeDir(tmpDir)

	configDir := filepath.Join(tmpDir, ".threedoors")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}

	cfg := &core.ProviderConfig{
		SchemaVersion: core.CurrentSchemaVersion,
		Provider:      "textfile",
		NoteTitle:     "Test Tasks",
		Theme:         "classic",
	}

	configPath := filepath.Join(configDir, "config.yaml")
	if err := core.SaveProviderConfig(configPath, cfg); err != nil {
		t.Fatalf("save test config: %v", err)
	}

	cleanup := func() {
		core.SetHomeDir("")
		if origHome != "" {
			_ = os.Setenv("HOME", origHome)
		}
	}

	return tmpDir, cleanup
}

func TestConfigShow_Human(t *testing.T) {
	_, cleanup := setupTestConfig(t)
	t.Cleanup(cleanup)

	jsonOutput = false
	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"config", "show"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("provider")) {
		t.Error("output should contain 'provider' key")
	}
	if !bytes.Contains([]byte(output), []byte("textfile")) {
		t.Error("output should contain 'textfile' value")
	}
	if !bytes.Contains([]byte(output), []byte("classic")) {
		t.Error("output should contain theme value 'classic'")
	}
}

func TestConfigShow_JSON(t *testing.T) {
	_, cleanup := setupTestConfig(t)
	t.Cleanup(cleanup)

	jsonOutput = true
	t.Cleanup(func() { jsonOutput = false })

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"config", "show", "--json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	var env JSONEnvelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if env.Command != "config.show" {
		t.Errorf("command = %q, want %q", env.Command, "config.show")
	}
	if env.Data == nil {
		t.Fatal("data should not be nil")
	}
}

func TestConfigGet_ValidKey(t *testing.T) {
	_, cleanup := setupTestConfig(t)
	t.Cleanup(cleanup)

	jsonOutput = false

	tests := []struct {
		name string
		key  string
		want string
	}{
		{"provider", "provider", "textfile"},
		{"note_title", "note_title", "Test Tasks"},
		{"theme", "theme", "classic"},
		{"dev_dispatch_enabled", "dev_dispatch_enabled", "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := NewRootCmd()
			var buf bytes.Buffer
			root.SetOut(&buf)
			root.SetArgs([]string{"config", "get", tt.key})

			if err := root.Execute(); err != nil {
				t.Fatalf("execute: %v", err)
			}

			got := bytes.TrimSpace(buf.Bytes())
			if string(got) != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfigGet_UnknownKey(t *testing.T) {
	_, cleanup := setupTestConfig(t)
	t.Cleanup(cleanup)

	jsonOutput = false

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"config", "get", "nonexistent_key"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
}

func TestConfigGet_JSON(t *testing.T) {
	_, cleanup := setupTestConfig(t)
	t.Cleanup(cleanup)

	jsonOutput = true
	t.Cleanup(func() { jsonOutput = false })

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"config", "get", "provider", "--json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	var env JSONEnvelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if env.Command != "config.get" {
		t.Errorf("command = %q, want %q", env.Command, "config.get")
	}
}

func TestConfigSet_UpdatesValue(t *testing.T) {
	_, cleanup := setupTestConfig(t)
	t.Cleanup(cleanup)

	jsonOutput = false

	// Set theme to "modern"
	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"config", "set", "theme", "modern"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute set: %v", err)
	}

	// Verify by reading it back
	root2 := NewRootCmd()
	var buf2 bytes.Buffer
	root2.SetOut(&buf2)
	root2.SetArgs([]string{"config", "get", "theme"})

	if err := root2.Execute(); err != nil {
		t.Fatalf("execute get: %v", err)
	}

	got := bytes.TrimSpace(buf2.Bytes())
	if string(got) != "modern" {
		t.Errorf("got %q, want %q", got, "modern")
	}
}

func TestConfigSet_UnknownKey(t *testing.T) {
	_, cleanup := setupTestConfig(t)
	t.Cleanup(cleanup)

	jsonOutput = false

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"config", "set", "invalid_key", "value"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
}

func TestConfigSet_JSON(t *testing.T) {
	_, cleanup := setupTestConfig(t)
	t.Cleanup(cleanup)

	jsonOutput = true
	t.Cleanup(func() { jsonOutput = false })

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"config", "set", "theme", "scifi", "--json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	var env JSONEnvelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if env.Command != "config.set" {
		t.Errorf("command = %q, want %q", env.Command, "config.set")
	}
}

func TestConfigSet_BoolValue(t *testing.T) {
	_, cleanup := setupTestConfig(t)
	t.Cleanup(cleanup)

	jsonOutput = false

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"config", "set", "dev_dispatch_enabled", "true"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute set: %v", err)
	}

	root2 := NewRootCmd()
	var buf2 bytes.Buffer
	root2.SetOut(&buf2)
	root2.SetArgs([]string{"config", "get", "dev_dispatch_enabled"})

	if err := root2.Execute(); err != nil {
		t.Fatalf("execute get: %v", err)
	}

	got := bytes.TrimSpace(buf2.Bytes())
	if string(got) != "true" {
		t.Errorf("got %q, want %q", got, "true")
	}
}

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestVersionCmd_HumanOutput(t *testing.T) {
	t.Parallel()

	// Set test values
	origVersion, origCommit, origDate := Version, Commit, BuildDate
	Version, Commit, BuildDate = "1.2.3", "abc1234", "2026-01-01T00:00:00Z"
	t.Cleanup(func() {
		Version, Commit, BuildDate = origVersion, origCommit, origDate
	})

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	output := buf.String()
	checks := []string{"1.2.3", "abc1234", "2026-01-01T00:00:00Z", "go"}
	for _, want := range checks {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q:\n%s", want, output)
		}
	}
}

func TestVersionCmd_JSONOutput(t *testing.T) {
	t.Parallel()

	origVersion, origCommit, origDate := Version, Commit, BuildDate
	Version, Commit, BuildDate = "2.0.0", "def5678", "2026-06-15T12:00:00Z"
	t.Cleanup(func() {
		Version, Commit, BuildDate = origVersion, origCommit, origDate
	})

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"--json", "version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	var env JSONEnvelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if env.Command != "version" {
		t.Errorf("command = %q, want %q", env.Command, "version")
	}
	if env.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1", env.SchemaVersion)
	}

	dataMap, ok := env.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data is not a map: %T", env.Data)
	}

	wantFields := map[string]string{
		"version":    "2.0.0",
		"commit":     "def5678",
		"build_date": "2026-06-15T12:00:00Z",
	}
	for key, want := range wantFields {
		got, ok := dataMap[key].(string)
		if !ok {
			t.Errorf("data.%s missing or not string", key)
			continue
		}
		if got != want {
			t.Errorf("data.%s = %q, want %q", key, got, want)
		}
	}

	if _, ok := dataMap["go_version"]; !ok {
		t.Error("data.go_version missing")
	}
}

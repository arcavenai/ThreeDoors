package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/arcaven/ThreeDoors/internal/core"
)

func TestRenderHealthTable_AllPass(t *testing.T) {
	t.Parallel()

	result := core.HealthCheckResult{
		Items: []core.HealthCheckItem{
			{Name: "Task File", Status: core.HealthOK, Message: "exists"},
			{Name: "Database", Status: core.HealthOK, Message: "5 tasks"},
		},
		Overall:  core.HealthOK,
		Duration: 10 * time.Millisecond,
	}

	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, false)
	err := renderHealthTable(f, result)
	if err != nil {
		t.Fatalf("renderHealthTable() error: %v", err)
	}

	output := buf.String()
	for _, want := range []string{"CHECK", "STATUS", "MESSAGE", "Task File", "OK", "Database"} {
		if !bytes.Contains([]byte(output), []byte(want)) {
			t.Errorf("output missing %q:\n%s", want, output)
		}
	}
}

func TestRenderHealthTable_WithFailure(t *testing.T) {
	t.Parallel()

	result := core.HealthCheckResult{
		Items: []core.HealthCheckItem{
			{Name: "Task File", Status: core.HealthOK, Message: "exists"},
			{Name: "Database", Status: core.HealthFail, Message: "corrupt"},
		},
		Overall:  core.HealthFail,
		Duration: 5 * time.Millisecond,
	}

	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, false)
	err := renderHealthTable(f, result)

	var ee *exitError
	if !errors.As(err, &ee) {
		t.Fatalf("expected exitError, got %v", err)
	}
	if ee.code != ExitProviderError {
		t.Errorf("exit code = %d, want %d", ee.code, ExitProviderError)
	}
}

func TestRenderHealthJSON_AllPass(t *testing.T) {
	t.Parallel()

	result := core.HealthCheckResult{
		Items: []core.HealthCheckItem{
			{Name: "Task File", Status: core.HealthOK, Message: "exists"},
			{Name: "Database", Status: core.HealthOK, Message: "3 tasks loaded"},
		},
		Overall:  core.HealthOK,
		Duration: 42 * time.Millisecond,
	}

	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, true)
	err := renderHealthJSON(f, result)
	if err != nil {
		t.Fatalf("renderHealthJSON() error: %v", err)
	}

	var env JSONEnvelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if env.Command != "health" {
		t.Errorf("command = %q, want %q", env.Command, "health")
	}
	if env.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1", env.SchemaVersion)
	}

	dataMap, ok := env.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data is not a map: %T", env.Data)
	}

	if overall, ok := dataMap["overall"].(string); !ok || overall != "OK" {
		t.Errorf("data.overall = %v, want %q", dataMap["overall"], "OK")
	}

	durationMs, ok := dataMap["duration_ms"].(float64)
	if !ok {
		t.Fatalf("data.duration_ms is not a number: %T", dataMap["duration_ms"])
	}
	if durationMs != 42 {
		t.Errorf("data.duration_ms = %v, want 42", durationMs)
	}

	checks, ok := dataMap["checks"].([]interface{})
	if !ok {
		t.Fatalf("data.checks is not an array: %T", dataMap["checks"])
	}
	if len(checks) != 2 {
		t.Errorf("len(data.checks) = %d, want 2", len(checks))
	}
}

func TestRenderHealthJSON_WithFailure(t *testing.T) {
	t.Parallel()

	result := core.HealthCheckResult{
		Items: []core.HealthCheckItem{
			{Name: "Database", Status: core.HealthFail, Message: "error loading"},
		},
		Overall:  core.HealthFail,
		Duration: 1 * time.Millisecond,
	}

	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, true)
	err := renderHealthJSON(f, result)

	var ee *exitError
	if !errors.As(err, &ee) {
		t.Fatalf("expected exitError, got %v", err)
	}
	if ee.code != ExitProviderError {
		t.Errorf("exit code = %d, want %d", ee.code, ExitProviderError)
	}

	// JSON should still have been written before the error
	var env JSONEnvelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	dataMap, ok := env.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data is not a map: %T", env.Data)
	}
	if overall := dataMap["overall"]; overall != "FAIL" {
		t.Errorf("data.overall = %v, want %q", overall, "FAIL")
	}
}

func TestExitError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		code int
	}{
		{"success", ExitSuccess},
		{"not found", ExitNotFound},
		{"validation", ExitValidation},
		{"provider error", ExitProviderError},
		{"ambiguous input", ExitAmbiguousInput},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ee := &exitError{code: tt.code}
			if ee.code != tt.code {
				t.Errorf("code = %d, want %d", ee.code, tt.code)
			}
			// Verify it satisfies the error interface
			var err error = ee
			if err.Error() == "" {
				t.Error("Error() should not be empty")
			}
		})
	}
}

func TestExecuteExitCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		wantCode int
	}{
		{"version returns success", []string{"version"}, ExitSuccess},
		{"unknown command returns error", []string{"nonexistent-cmd-xyz"}, ExitGeneralError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// We can't easily call Execute() directly because it creates
			// its own root cmd, but we can test via NewRootCmd
			root := NewRootCmd()
			root.SetArgs(tt.args)

			err := root.Execute()
			if tt.wantCode == ExitSuccess {
				if err != nil {
					t.Errorf("Execute() error: %v, want nil", err)
				}
			} else {
				if err == nil {
					t.Error("Execute() should have returned error")
				}
			}
		})
	}
}

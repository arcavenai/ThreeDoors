package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arcaven/ThreeDoors/internal/core"
)

func writeTestSessions(t *testing.T, homeDir string, count int) {
	t.Helper()
	writer := core.NewMetricsWriter(filepath.Join(homeDir, ".threedoors"))
	for i := 0; i < count; i++ {
		tracker := core.NewSessionTracker()
		tracker.RecordTaskCompleted()
		tracker.RecordMood("focused", "")
		if err := writer.AppendSession(tracker.Finalize()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestStats_Dashboard_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	setTestHome(t, tmpDir)
	setupConfigDir(t, tmpDir)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })

	if err := runStats(false, false, false); err != nil {
		t.Fatalf("runStats: unexpected error: %v", err)
	}

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	if !strings.Contains(buf.String(), "Dashboard") {
		t.Errorf("expected 'Dashboard' in output, got: %s", buf.String())
	}
}

func TestStats_Dashboard_WithSessions(t *testing.T) {
	tmpDir := t.TempDir()
	setTestHome(t, tmpDir)
	setupConfigDir(t, tmpDir)
	writeTestSessions(t, tmpDir, 3)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })

	if err := runStats(false, false, false); err != nil {
		t.Fatalf("runStats: unexpected error: %v", err)
	}

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()
	if !strings.Contains(output, "Today:") {
		t.Errorf("expected 'Today:' in output, got: %s", output)
	}
	if !strings.Contains(output, "Streak:") {
		t.Errorf("expected 'Streak:' in output, got: %s", output)
	}
}

func TestStats_Dashboard_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	setTestHome(t, tmpDir)
	setupConfigDir(t, tmpDir)
	writeTestSessions(t, tmpDir, 2)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	savedJSON := jsonOutput
	jsonOutput = true
	t.Cleanup(func() {
		jsonOutput = savedJSON
		os.Stdout = old
	})

	if err := runStats(false, false, false); err != nil {
		t.Fatalf("runStats --json: unexpected error: %v", err)
	}

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	var env JSONEnvelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal JSON: %v\nraw: %s", err, buf.String())
	}
	if env.Command != "stats" {
		t.Errorf("command = %q, want %q", env.Command, "stats")
	}
}

func TestStats_Daily(t *testing.T) {
	tmpDir := t.TempDir()
	setTestHome(t, tmpDir)
	setupConfigDir(t, tmpDir)
	writeTestSessions(t, tmpDir, 2)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })

	if err := runStats(true, false, false); err != nil {
		t.Fatalf("runStats --daily: unexpected error: %v", err)
	}

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()
	if !strings.Contains(output, "Daily completions") {
		t.Errorf("expected 'Daily completions' in output, got: %s", output)
	}
	// Should contain today's date
	today := time.Now().UTC().Format("2006-01-02")
	if !strings.Contains(output, today) {
		t.Errorf("expected today's date %s in output", today)
	}
}

func TestStats_Daily_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	setTestHome(t, tmpDir)
	setupConfigDir(t, tmpDir)
	writeTestSessions(t, tmpDir, 1)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	savedJSON := jsonOutput
	jsonOutput = true
	t.Cleanup(func() {
		jsonOutput = savedJSON
		os.Stdout = old
	})

	if err := runStats(true, false, false); err != nil {
		t.Fatalf("runStats --daily --json: unexpected error: %v", err)
	}

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	var env JSONEnvelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal JSON: %v\nraw: %s", err, buf.String())
	}
	if env.Command != "stats.daily" {
		t.Errorf("command = %q, want %q", env.Command, "stats.daily")
	}
}

func TestStats_Weekly(t *testing.T) {
	tmpDir := t.TempDir()
	setTestHome(t, tmpDir)
	setupConfigDir(t, tmpDir)
	writeTestSessions(t, tmpDir, 2)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })

	if err := runStats(false, true, false); err != nil {
		t.Fatalf("runStats --weekly: unexpected error: %v", err)
	}

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	if !strings.Contains(buf.String(), "Week-over-week") {
		t.Errorf("expected 'Week-over-week' in output, got: %s", buf.String())
	}
}

func TestStats_Weekly_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	setTestHome(t, tmpDir)
	setupConfigDir(t, tmpDir)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	savedJSON := jsonOutput
	jsonOutput = true
	t.Cleanup(func() {
		jsonOutput = savedJSON
		os.Stdout = old
	})

	if err := runStats(false, true, false); err != nil {
		t.Fatalf("runStats --weekly --json: unexpected error: %v", err)
	}

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	var env JSONEnvelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal JSON: %v\nraw: %s", err, buf.String())
	}
	if env.Command != "stats.weekly" {
		t.Errorf("command = %q, want %q", env.Command, "stats.weekly")
	}
}

func TestStats_Patterns_InsufficientData(t *testing.T) {
	tmpDir := t.TempDir()
	setTestHome(t, tmpDir)
	setupConfigDir(t, tmpDir)
	writeTestSessions(t, tmpDir, 3)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })

	if err := runStats(false, false, true); err != nil {
		t.Fatalf("runStats --patterns: unexpected error: %v", err)
	}

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	if !strings.Contains(buf.String(), "Insufficient data") {
		t.Errorf("expected 'Insufficient data' message, got: %s", buf.String())
	}
}

func TestStats_Patterns_WithData(t *testing.T) {
	tmpDir := t.TempDir()
	setTestHome(t, tmpDir)
	setupConfigDir(t, tmpDir)
	writeTestSessions(t, tmpDir, 6)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })

	if err := runStats(false, false, true); err != nil {
		t.Fatalf("runStats --patterns: unexpected error: %v", err)
	}

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	if !strings.Contains(buf.String(), "Pattern Analysis") {
		t.Errorf("expected 'Pattern Analysis' in output, got: %s", buf.String())
	}
}

func TestStats_Patterns_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	setTestHome(t, tmpDir)
	setupConfigDir(t, tmpDir)
	writeTestSessions(t, tmpDir, 6)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	savedJSON := jsonOutput
	jsonOutput = true
	t.Cleanup(func() {
		jsonOutput = savedJSON
		os.Stdout = old
	})

	if err := runStats(false, false, true); err != nil {
		t.Fatalf("runStats --patterns --json: unexpected error: %v", err)
	}

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	var env JSONEnvelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal JSON: %v\nraw: %s", err, buf.String())
	}
	if env.Command != "stats.patterns" {
		t.Errorf("command = %q, want %q", env.Command, "stats.patterns")
	}
}

func TestCalculateStreak(t *testing.T) {
	t.Parallel()

	pa := core.NewPatternAnalyzer()
	// No sessions — streak should be 0
	streak := calculateStreak(pa)
	if streak != 0 {
		t.Errorf("streak = %d, want 0", streak)
	}
}

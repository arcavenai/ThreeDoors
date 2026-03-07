package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arcaven/ThreeDoors/internal/core"
)

func setTestHome(t *testing.T, dir string) {
	t.Helper()
	core.SetHomeDir(dir)
	t.Cleanup(func() { core.SetHomeDir("") })
}

func TestMoodSet_ValidMoods(t *testing.T) {
	t.Parallel()

	moods := []string{"focused", "energized", "tired", "stressed", "neutral", "calm", "anxious", "motivated", "frustrated"}
	for _, mood := range moods {
		t.Run(mood, func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			setupConfigDir(t, tmpDir)

			// Can't share the global SetHomeDir across parallel subtests,
			// so we test via the command directly
			cmd := newMoodSetCmd()
			// Override configDir by setting test home
			core.SetHomeDir(tmpDir)
			defer core.SetHomeDir("")

			cmd.SetArgs([]string{mood})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("mood set %s: unexpected error: %v", mood, err)
			}
		})
	}
}

func TestMoodSet_InvalidMood_ExitCode3(t *testing.T) {
	tmpDir := t.TempDir()
	setTestHome(t, tmpDir)
	setupConfigDir(t, tmpDir)

	cmd := newMoodSetCmd()
	cmd.SetArgs([]string{"grumpy"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid mood")
	}

	var ee *exitError
	if !containsExitError(err, &ee) {
		t.Fatalf("expected exitError, got %T: %v", err, err)
	}
	if ee.ExitCode() != ExitValidation {
		t.Errorf("exit code = %d, want %d (ExitValidation)", ee.ExitCode(), ExitValidation)
	}
}

func TestMoodSet_Custom(t *testing.T) {
	tmpDir := t.TempDir()
	setTestHome(t, tmpDir)
	setupConfigDir(t, tmpDir)

	cmd := newMoodSetCmd()
	cmd.SetArgs([]string{"custom", "Feeling creative"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("mood set custom: unexpected error: %v", err)
	}

	sessions := readSessions(t, tmpDir)
	if len(sessions) == 0 {
		t.Fatal("no sessions written")
	}
	last := sessions[len(sessions)-1]
	if len(last.MoodEntries) == 0 {
		t.Fatal("no mood entries in session")
	}
	if last.MoodEntries[0].Mood != "custom" {
		t.Errorf("mood = %q, want %q", last.MoodEntries[0].Mood, "custom")
	}
	if last.MoodEntries[0].CustomText != "Feeling creative" {
		t.Errorf("custom_text = %q, want %q", last.MoodEntries[0].CustomText, "Feeling creative")
	}
}

func TestMoodSet_CustomWithoutText(t *testing.T) {
	tmpDir := t.TempDir()
	setTestHome(t, tmpDir)
	setupConfigDir(t, tmpDir)

	cmd := newMoodSetCmd()
	cmd.SetArgs([]string{"custom"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for custom mood without text")
	}

	var ee *exitError
	if !containsExitError(err, &ee) {
		t.Fatalf("expected exitError, got %T: %v", err, err)
	}
	if ee.ExitCode() != ExitValidation {
		t.Errorf("exit code = %d, want %d", ee.ExitCode(), ExitValidation)
	}
}

func TestMoodHistory_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	setTestHome(t, tmpDir)
	setupConfigDir(t, tmpDir)

	// Redirect stdout to capture output
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })

	cmd := newMoodHistoryCmd()
	if err := cmd.Execute(); err != nil {
		t.Fatalf("mood history: unexpected error: %v", err)
	}

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	if !strings.Contains(buf.String(), "No mood entries") {
		t.Errorf("expected 'No mood entries' message, got: %s", buf.String())
	}
}

func TestMoodHistory_WithEntries(t *testing.T) {
	tmpDir := t.TempDir()
	setTestHome(t, tmpDir)
	setupConfigDir(t, tmpDir)

	tracker := core.NewSessionTracker()
	tracker.RecordMood("focused", "")
	writer := core.NewMetricsWriter(filepath.Join(tmpDir, ".threedoors"))
	if err := writer.AppendSession(tracker.Finalize()); err != nil {
		t.Fatal(err)
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })

	cmd := newMoodHistoryCmd()
	if err := cmd.Execute(); err != nil {
		t.Fatalf("mood history: unexpected error: %v", err)
	}

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	if !strings.Contains(buf.String(), "focused") {
		t.Errorf("expected 'focused' in output, got: %s", buf.String())
	}
}

func TestMoodHistory_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	setTestHome(t, tmpDir)
	setupConfigDir(t, tmpDir)

	tracker := core.NewSessionTracker()
	tracker.RecordMood("tired", "long day")
	writer := core.NewMetricsWriter(filepath.Join(tmpDir, ".threedoors"))
	if err := writer.AppendSession(tracker.Finalize()); err != nil {
		t.Fatal(err)
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	savedJSON := jsonOutput
	jsonOutput = true
	t.Cleanup(func() {
		jsonOutput = savedJSON
		os.Stdout = old
	})

	cmd := newMoodHistoryCmd()
	if err := cmd.Execute(); err != nil {
		t.Fatalf("mood history --json: unexpected error: %v", err)
	}

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	var env JSONEnvelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal JSON: %v\nraw: %s", err, buf.String())
	}
	if env.Command != "mood.history" {
		t.Errorf("command = %q, want %q", env.Command, "mood.history")
	}
	if env.Error != nil {
		t.Errorf("unexpected error in envelope: %+v", env.Error)
	}
}

func TestValidMoodList(t *testing.T) {
	t.Parallel()

	list := validMoodList()
	if list == "" {
		t.Error("validMoodList should not be empty")
	}
	for _, mood := range []string{"calm", "focused", "tired"} {
		if !strings.Contains(list, mood) {
			t.Errorf("validMoodList() should contain %q, got %q", mood, list)
		}
	}
}

// helpers

func setupConfigDir(t *testing.T, homeDir string) {
	t.Helper()
	dir := filepath.Join(homeDir, ".threedoors")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func readSessions(t *testing.T, homeDir string) []core.SessionMetrics {
	t.Helper()
	sessionsPath := filepath.Join(homeDir, ".threedoors", "sessions.jsonl")
	pa := core.NewPatternAnalyzer()
	sessions, err := pa.ReadSessions(sessionsPath)
	if err != nil {
		t.Fatal(err)
	}
	return sessions
}

func containsExitError(err error, target **exitError) bool {
	for err != nil {
		if ee, ok := err.(*exitError); ok {
			*target = ee
			return true
		}
		if u, ok := err.(interface{ Unwrap() error }); ok {
			err = u.Unwrap()
		} else {
			return false
		}
	}
	return false
}

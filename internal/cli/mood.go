package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/arcaven/ThreeDoors/internal/core"
	"github.com/arcaven/ThreeDoors/internal/core/metrics"
	"github.com/spf13/cobra"
)

// validMoods is the set of recognized mood values.
var validMoods = map[string]bool{
	"focused":    true,
	"energized":  true,
	"tired":      true,
	"stressed":   true,
	"neutral":    true,
	"calm":       true,
	"anxious":    true,
	"motivated":  true,
	"frustrated": true,
}

// NewMoodCmd creates the "mood" parent command with "set" and "history" subcommands.
func NewMoodCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mood",
		Short: "Record and view mood entries",
	}

	cmd.AddCommand(newMoodSetCmd())
	cmd.AddCommand(newMoodHistoryCmd())

	return cmd
}

func newMoodSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <mood> [custom-text]",
		Short: "Record a mood entry",
		Long: `Record a mood entry. Valid moods: focused, energized, tired, stressed,
neutral, calm, anxious, motivated, frustrated, custom.

Use "custom" as the mood with a quoted string for free-form mood text:
  threedoors mood set custom "Feeling creative"`,
		Args: cobra.MinimumNArgs(1),
		RunE: runMoodSet,
	}
	return cmd
}

func runMoodSet(cmd *cobra.Command, args []string) error {
	mood := strings.ToLower(args[0])
	customText := ""

	if mood == "custom" {
		if len(args) < 2 {
			return &exitError{
				code: ExitValidation,
				msg:  "custom mood requires a text argument: threedoors mood set custom \"your mood\"",
			}
		}
		customText = strings.Join(args[1:], " ")
	} else if !validMoods[mood] {
		return &exitError{
			code: ExitValidation,
			msg:  fmt.Sprintf("invalid mood %q; valid moods: %s, custom", mood, validMoodList()),
		}
	}

	configDir, err := core.GetConfigDirPath()
	if err != nil {
		return fmt.Errorf("get config dir: %w", err)
	}

	// Record mood via a single-entry session
	tracker := core.NewSessionTracker()
	tracker.RecordMood(mood, customText)
	writer := core.NewMetricsWriter(configDir)
	if err := writer.AppendSession(tracker.Finalize()); err != nil {
		return fmt.Errorf("save mood entry: %w", err)
	}

	formatter := NewOutputFormatter(os.Stdout, jsonOutput)
	if formatter.IsJSON() {
		return formatter.WriteJSON("mood.set", map[string]string{
			"mood":        mood,
			"custom_text": customText,
			"recorded_at": time.Now().UTC().Format(time.RFC3339),
		}, nil)
	}

	if customText != "" {
		return formatter.Writef("Mood recorded: %s (%s)\n", mood, customText)
	}
	return formatter.Writef("Mood recorded: %s\n", mood)
}

func newMoodHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show recent mood entries",
		RunE:  runMoodHistory,
	}
	return cmd
}

func runMoodHistory(cmd *cobra.Command, args []string) error {
	configDir, err := core.GetConfigDirPath()
	if err != nil {
		return fmt.Errorf("get config dir: %w", err)
	}

	reader := metrics.NewReader(filepath.Join(configDir, "sessions.jsonl"))
	sessions, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("read sessions: %w", err)
	}

	// Collect all mood entries from sessions
	type moodRecord struct {
		Timestamp  time.Time `json:"timestamp"`
		Mood       string    `json:"mood"`
		CustomText string    `json:"custom_text,omitempty"`
	}
	var entries []moodRecord
	for _, s := range sessions {
		for _, me := range s.MoodEntries {
			entries = append(entries, moodRecord{
				Timestamp:  me.Timestamp,
				Mood:       me.Mood,
				CustomText: me.CustomText,
			})
		}
	}

	// Sort by timestamp descending (most recent first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})

	// Limit to last 20 entries
	if len(entries) > 20 {
		entries = entries[:20]
	}

	formatter := NewOutputFormatter(os.Stdout, jsonOutput)
	if formatter.IsJSON() {
		return formatter.WriteJSON("mood.history", entries, map[string]int{"count": len(entries)})
	}

	if len(entries) == 0 {
		return formatter.Writef("No mood entries found. Use 'threedoors mood set <mood>' to record one.\n")
	}

	_ = formatter.Writef("Recent mood entries:\n\n")
	tw := formatter.TableWriter()
	_, _ = fmt.Fprintln(tw, "TIME\tMOOD\tNOTE")
	for _, e := range entries {
		note := e.CustomText
		if note == "" {
			note = "-"
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", e.Timestamp.Local().Format("2006-01-02 15:04"), e.Mood, note)
	}
	return tw.Flush()
}

func validMoodList() string {
	moods := make([]string, 0, len(validMoods))
	for m := range validMoods {
		moods = append(moods, m)
	}
	sort.Strings(moods)
	return strings.Join(moods, ", ")
}

// exitError is a CLI error that carries an exit code.
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }

// ExitCode returns the exit code for the error.
func (e *exitError) ExitCode() int { return e.code }

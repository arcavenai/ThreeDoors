package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/arcaven/ThreeDoors/internal/core"
	"github.com/arcaven/ThreeDoors/internal/core/metrics"
	"github.com/spf13/cobra"
)

// NewStatsCmd creates the "stats" command with daily/weekly/patterns flags.
func NewStatsCmd() *cobra.Command {
	var daily, weekly, patterns bool

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Display productivity statistics",
		Long: `Display a summary dashboard of productivity statistics.

Flags:
  --daily     Show daily completions for the last 7 days
  --weekly    Show week-over-week comparison
  --patterns  Show full pattern analysis`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStats(daily, weekly, patterns)
		},
	}

	cmd.Flags().BoolVar(&daily, "daily", false, "show daily completions for the last 7 days")
	cmd.Flags().BoolVar(&weekly, "weekly", false, "show week-over-week comparison")
	cmd.Flags().BoolVar(&patterns, "patterns", false, "show full pattern analysis")

	return cmd
}

func runStats(daily, weekly, patterns bool) error {
	configDir, err := core.GetConfigDirPath()
	if err != nil {
		return fmt.Errorf("get config dir: %w", err)
	}

	sessionsPath := filepath.Join(configDir, "sessions.jsonl")
	reader := metrics.NewReader(sessionsPath)
	sessions, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("read sessions: %w", err)
	}

	analyzer := core.NewPatternAnalyzer()
	if loadErr := analyzer.LoadSessions(sessionsPath); loadErr != nil {
		return fmt.Errorf("load sessions: %w", loadErr)
	}

	formatter := NewOutputFormatter(os.Stdout, jsonOutput)

	// Specific flag modes
	if daily {
		return renderDaily(formatter, analyzer)
	}
	if weekly {
		return renderWeekly(formatter, analyzer)
	}
	if patterns {
		return renderPatterns(formatter, analyzer, sessions)
	}

	// Default: summary dashboard
	return renderDashboard(formatter, analyzer, sessions)
}

func renderDashboard(f *OutputFormatter, pa *core.PatternAnalyzer, sessions []core.SessionMetrics) error {
	dod := pa.GetDayOverDay()
	wow := pa.GetWeekOverWeek()

	// Calculate completion rate (sessions with >=1 task completed / total sessions)
	completedSessions := 0
	for _, s := range sessions {
		if s.TasksCompleted > 0 {
			completedSessions++
		}
	}
	completionRate := 0.0
	if len(sessions) > 0 {
		completionRate = float64(completedSessions) / float64(len(sessions)) * 100
	}

	// Calculate streak: consecutive days with completions ending today or yesterday
	streak := calculateStreak(pa)

	if f.IsJSON() {
		data := map[string]interface{}{
			"today_completed": dod.TodayTasks,
			"streak_days":     streak,
			"completion_rate": completionRate,
			"total_sessions":  len(sessions),
			"this_week_total": wow.ThisWeekTotal,
			"last_week_total": wow.LastWeekTotal,
			"week_change":     wow.PercentChange,
			"week_direction":  wow.Direction,
		}
		return f.WriteJSON("stats", data, nil)
	}

	_ = f.Writef("ThreeDoors Stats Dashboard\n")
	_ = f.Writef("==========================\n\n")
	_ = f.Writef("Today:           %d tasks completed\n", dod.TodayTasks)
	_ = f.Writef("Streak:          %d day(s)\n", streak)
	_ = f.Writef("Completion rate: %.0f%% of sessions productive\n", completionRate)
	_ = f.Writef("This week:       %d tasks", wow.ThisWeekTotal)
	switch wow.Direction {
	case "up":
		_ = f.Writef(" (+%.0f%% vs last week)\n", wow.PercentChange)
	case "down":
		_ = f.Writef(" (%.0f%% vs last week)\n", wow.PercentChange)
	default:
		_ = f.Writef(" (same as last week)\n")
	}
	return nil
}

func renderDaily(f *OutputFormatter, pa *core.PatternAnalyzer) error {
	daily := pa.GetDailyCompletions(7)

	if f.IsJSON() {
		return f.WriteJSON("stats.daily", daily, nil)
	}

	_ = f.Writef("Daily completions (last 7 days):\n\n")
	tw := f.TableWriter()
	_, _ = fmt.Fprintln(tw, "DATE\tCOMPLETED")

	// Sort dates
	dates := make([]string, 0, len(daily))
	for d := range daily {
		dates = append(dates, d)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))

	for _, d := range dates {
		_, _ = fmt.Fprintf(tw, "%s\t%d\n", d, daily[d])
	}
	return tw.Flush()
}

func renderWeekly(f *OutputFormatter, pa *core.PatternAnalyzer) error {
	wow := pa.GetWeekOverWeek()

	if f.IsJSON() {
		return f.WriteJSON("stats.weekly", wow, nil)
	}

	_ = f.Writef("Week-over-week comparison:\n\n")
	_ = f.Writef("This week: %d tasks completed\n", wow.ThisWeekTotal)
	_ = f.Writef("Last week: %d tasks completed\n", wow.LastWeekTotal)
	switch wow.Direction {
	case "up":
		return f.Writef("Change:    +%.0f%% (up)\n", wow.PercentChange)
	case "down":
		return f.Writef("Change:    %.0f%% (down)\n", wow.PercentChange)
	default:
		return f.Writef("Change:    0%% (same)\n")
	}
}

func renderPatterns(f *OutputFormatter, pa *core.PatternAnalyzer, sessions []core.SessionMetrics) error {
	report, err := pa.Analyze(sessions)
	if err != nil {
		return fmt.Errorf("analyze patterns: %w", err)
	}

	if f.IsJSON() {
		if report == nil {
			return f.WriteJSON("stats.patterns", nil, map[string]string{
				"message": "insufficient data for pattern analysis (need 5+ sessions)",
			})
		}
		return f.WriteJSON("stats.patterns", report, nil)
	}

	if report == nil {
		return f.Writef("Insufficient data for pattern analysis (need 5+ sessions).\n")
	}

	_ = f.Writef("Pattern Analysis (%d sessions)\n", report.SessionCount)
	_ = f.Writef("================================\n\n")

	// Door position bias
	dp := report.DoorPositionBias
	_ = f.Writef("Door position bias: %s\n", dp.PreferredPosition)
	_ = f.Writef("  Left: %d  Center: %d  Right: %d\n\n", dp.LeftCount, dp.CenterCount, dp.RightCount)

	// Time of day
	if len(report.TimeOfDayPatterns) > 0 {
		_ = f.Writef("Time of day patterns:\n")
		for _, p := range report.TimeOfDayPatterns {
			_ = f.Writef("  %-10s %d sessions, avg %.1f tasks/session\n", p.Period, p.SessionCount, p.AvgTasksCompleted)
		}
		_ = f.Writef("\n")
	}

	// Mood correlations
	if len(report.MoodCorrelations) > 0 {
		_ = f.Writef("Mood correlations:\n")
		for _, mc := range report.MoodCorrelations {
			_ = f.Writef("  %-12s avg %.1f tasks/session (%d sessions)\n", mc.Mood, mc.AvgTasksCompleted, mc.SessionCount)
		}
		_ = f.Writef("\n")
	}

	// Avoidance list
	if len(report.AvoidanceList) > 0 {
		_ = f.Writef("Frequently bypassed tasks:\n")
		for _, a := range report.AvoidanceList {
			_ = f.Writef("  %q — bypassed %d/%d times", a.TaskText, a.TimesBypassed, a.TimesShown)
			if a.NeverSelected {
				_ = f.Writef(" (never selected)")
			}
			_ = f.Writef("\n")
		}
	}

	return nil
}

// calculateStreak counts consecutive days ending at today with at least one task completed.
func calculateStreak(pa *core.PatternAnalyzer) int {
	daily := pa.GetDailyCompletions(30)

	// Sort dates descending
	dates := make([]string, 0, len(daily))
	for d := range daily {
		dates = append(dates, d)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))

	streak := 0
	for _, d := range dates {
		if daily[d] > 0 {
			streak++
		} else {
			break
		}
	}
	return streak
}

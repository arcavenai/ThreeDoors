package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/arcaven/ThreeDoors/internal/core"
	"github.com/spf13/cobra"
)

// shortID returns the first 8 characters of a task ID for display.
func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

// loadTaskPool initializes a provider from config and returns a loaded TaskPool.
func loadTaskPool() (*core.TaskPool, error) {
	configDir, err := core.GetConfigDirPath()
	if err != nil {
		return nil, fmt.Errorf("config dir: %w", err)
	}

	configPath := filepath.Join(configDir, "config.yaml")
	cfg, err := core.LoadProviderConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	var provider core.TaskProvider
	if len(cfg.Providers) > 1 {
		agg, aggErr := core.ResolveAllProviders(cfg, core.DefaultRegistry())
		if aggErr != nil {
			return nil, fmt.Errorf("init providers: %w", aggErr)
		}
		provider = agg
	} else {
		provider = core.NewProviderFromConfig(cfg)
	}

	tasks, err := provider.LoadTasks()
	if err != nil {
		return nil, fmt.Errorf("load tasks: %w", err)
	}

	pool := core.NewTaskPool()
	for _, t := range tasks {
		pool.AddTask(t)
	}
	return pool, nil
}

// NewTaskCmd creates the "task" parent command with list and show subcommands.
func NewTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Manage tasks",
		Long:  "List, inspect, and manage tasks from the command line.",
	}

	cmd.AddCommand(newTaskListCmd())
	cmd.AddCommand(newTaskShowCmd())

	return cmd
}

func newTaskListCmd() *cobra.Command {
	var statusFilter string
	var typeFilter string
	var effortFilter string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks with optional filters",
		RunE: func(cmd *cobra.Command, args []string) error {
			pool, err := loadTaskPool()
			if err != nil {
				return err
			}

			tasks := pool.GetAllTasks()

			// Apply filters
			var filtered []*core.Task
			for _, t := range tasks {
				if statusFilter != "" && string(t.Status) != statusFilter {
					continue
				}
				if typeFilter != "" && string(t.Type) != typeFilter {
					continue
				}
				if effortFilter != "" && string(t.Effort) != effortFilter {
					continue
				}
				filtered = append(filtered, t)
			}

			// Sort by creation time for stable output
			sort.Slice(filtered, func(i, j int) bool {
				return filtered[i].CreatedAt.Before(filtered[j].CreatedAt)
			})

			total := len(tasks)
			count := len(filtered)
			formatter := NewOutputFormatter(os.Stdout, jsonOutput)

			if jsonOutput {
				type taskItem struct {
					ID     string          `json:"id"`
					Status core.TaskStatus `json:"status"`
					Type   core.TaskType   `json:"type,omitempty"`
					Effort core.TaskEffort `json:"effort,omitempty"`
					Text   string          `json:"text"`
				}
				items := make([]taskItem, 0, count)
				for _, t := range filtered {
					items = append(items, taskItem{
						ID:     t.ID,
						Status: t.Status,
						Type:   t.Type,
						Effort: t.Effort,
						Text:   t.Text,
					})
				}

				meta := map[string]interface{}{
					"total":    total,
					"filtered": count,
				}
				if statusFilter != "" || typeFilter != "" || effortFilter != "" {
					filters := map[string]string{}
					if statusFilter != "" {
						filters["status"] = statusFilter
					}
					if typeFilter != "" {
						filters["type"] = typeFilter
					}
					if effortFilter != "" {
						filters["effort"] = effortFilter
					}
					meta["filters"] = filters
				}
				return formatter.WriteJSON("task.list", items, meta)
			}

			tw := formatter.TableWriter()
			if _, err := fmt.Fprintln(tw, "ID\tSTATUS\tTYPE\tEFFORT\tTEXT"); err != nil {
				return fmt.Errorf("write table header: %w", err)
			}
			for _, t := range filtered {
				if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
					shortID(t.ID),
					t.Status,
					t.Type,
					t.Effort,
					t.Text,
				); err != nil {
					return fmt.Errorf("write table row: %w", err)
				}
			}
			if err := tw.Flush(); err != nil {
				return fmt.Errorf("flush table: %w", err)
			}
			return formatter.Writef("%d tasks found\n", count)
		},
	}

	cmd.Flags().StringVar(&statusFilter, "status", "", "filter by status (todo, in-progress, blocked, etc.)")
	cmd.Flags().StringVar(&typeFilter, "type", "", "filter by type (creative, administrative, technical, physical)")
	cmd.Flags().StringVar(&effortFilter, "effort", "", "filter by effort (quick-win, medium, deep-work)")

	return cmd
}

func newTaskShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show full task details by ID or prefix",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pool, err := loadTaskPool()
			if err != nil {
				return err
			}

			task, err := pool.FindByPrefix(args[0])
			if err != nil {
				formatter := NewOutputFormatter(os.Stderr, jsonOutput)
				if errors.Is(err, core.ErrTaskNotFound) {
					if jsonOutput {
						_ = formatter.WriteJSONError("task.show", ExitNotFound, "task not found", err.Error())
					} else {
						_ = formatter.Writef("Error: %v\n", err)
					}
					os.Exit(ExitNotFound)
				}
				if errors.Is(err, core.ErrAmbiguousPrefix) {
					if jsonOutput {
						_ = formatter.WriteJSONError("task.show", ExitAmbiguousInput, "ambiguous prefix", err.Error())
					} else {
						_ = formatter.Writef("Error: %v\n", err)
					}
					os.Exit(ExitAmbiguousInput)
				}
				return err
			}

			formatter := NewOutputFormatter(os.Stdout, jsonOutput)

			if jsonOutput {
				return formatter.WriteJSON("task.show", task, nil)
			}

			if err := formatter.Writef("ID:        %s\n", task.ID); err != nil {
				return err
			}
			if err := formatter.Writef("Text:      %s\n", task.Text); err != nil {
				return err
			}
			if err := formatter.Writef("Status:    %s\n", task.Status); err != nil {
				return err
			}
			if task.Type != "" {
				if err := formatter.Writef("Type:      %s\n", task.Type); err != nil {
					return err
				}
			}
			if task.Effort != "" {
				if err := formatter.Writef("Effort:    %s\n", task.Effort); err != nil {
					return err
				}
			}
			if task.Context != "" {
				if err := formatter.Writef("Context:   %s\n", task.Context); err != nil {
					return err
				}
			}
			if task.Location != "" {
				if err := formatter.Writef("Location:  %s\n", task.Location); err != nil {
					return err
				}
			}
			if task.Blocker != "" {
				if err := formatter.Writef("Blocker:   %s\n", task.Blocker); err != nil {
					return err
				}
			}
			if err := formatter.Writef("Created:   %s\n", task.CreatedAt.Format("2006-01-02 15:04:05")); err != nil {
				return err
			}
			if err := formatter.Writef("Updated:   %s\n", task.UpdatedAt.Format("2006-01-02 15:04:05")); err != nil {
				return err
			}
			if task.CompletedAt != nil {
				if err := formatter.Writef("Completed: %s\n", task.CompletedAt.Format("2006-01-02 15:04:05")); err != nil {
					return err
				}
			}
			if len(task.Notes) > 0 {
				if err := formatter.Writef("Notes:\n"); err != nil {
					return err
				}
				for _, n := range task.Notes {
					if err := formatter.Writef("  [%s] %s\n", n.Timestamp.Format("2006-01-02 15:04"), n.Text); err != nil {
						return err
					}
				}
			}

			return nil
		},
	}

	return cmd
}

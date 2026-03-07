package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/arcaven/ThreeDoors/internal/core"
	"github.com/spf13/cobra"
)

// healthCheckJSON is the JSON representation of a single health check.
type healthCheckJSON struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// healthDataJSON is the JSON envelope data for the health command.
type healthDataJSON struct {
	Overall    string            `json:"overall"`
	DurationMs int64             `json:"duration_ms"`
	Checks     []healthCheckJSON `json:"checks"`
}

// newHealthCmd creates the health subcommand.
func newHealthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Run system health checks",
		RunE: func(cmd *cobra.Command, _ []string) error {
			provider, err := resolveProvider()
			if err != nil {
				formatter := NewOutputFormatter(os.Stderr, jsonOutput)
				if jsonOutput {
					_ = formatter.WriteJSONError("health", ExitProviderError, "provider initialization failed", err.Error())
				} else {
					_ = formatter.Writef("Error: provider initialization failed: %v\n", err)
				}
				return &exitError{code: ExitProviderError}
			}

			hc := core.NewHealthChecker(provider)
			result := hc.RunAll()

			formatter := NewOutputFormatter(os.Stdout, jsonOutput)
			if jsonOutput {
				return renderHealthJSON(formatter, result)
			}
			return renderHealthTable(formatter, result)
		},
	}
}

// exitError carries an exit code through Cobra's error handling.
type exitError struct {
	code int
}

func (e *exitError) Error() string {
	return fmt.Sprintf("exit code %d", e.code)
}

// resolveProvider creates a TaskProvider from the user's config.
func resolveProvider() (core.TaskProvider, error) {
	configDir, err := core.GetConfigDirPath()
	if err != nil {
		return nil, fmt.Errorf("config dir: %w", err)
	}

	configPath := filepath.Join(configDir, "config.yaml")
	cfg, err := core.LoadProviderConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	provider := core.NewProviderFromConfig(cfg)
	return core.NewWALProvider(provider, configDir), nil
}

// renderHealthJSON writes health results as a JSON envelope.
func renderHealthJSON(f *OutputFormatter, result core.HealthCheckResult) error {
	checks := make([]healthCheckJSON, len(result.Items))
	for i, item := range result.Items {
		checks[i] = healthCheckJSON{
			Name:    item.Name,
			Status:  string(item.Status),
			Message: item.Message,
		}
	}

	data := healthDataJSON{
		Overall:    string(result.Overall),
		DurationMs: result.Duration.Milliseconds(),
		Checks:     checks,
	}

	if err := f.WriteJSON("health", data, nil); err != nil {
		return err
	}

	if result.Overall == core.HealthFail {
		return &exitError{code: ExitProviderError}
	}
	return nil
}

// renderHealthTable writes health results as an aligned table.
func renderHealthTable(f *OutputFormatter, result core.HealthCheckResult) error {
	tw := f.TableWriter()
	if _, err := fmt.Fprintln(tw, "CHECK\tSTATUS\tMESSAGE"); err != nil {
		return err
	}
	for _, item := range result.Items {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\n", item.Name, item.Status, item.Message); err != nil {
			return err
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	if result.Overall == core.HealthFail {
		return &exitError{code: ExitProviderError}
	}
	return nil
}

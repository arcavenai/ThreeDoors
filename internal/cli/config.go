package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/arcaven/ThreeDoors/internal/core"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// validConfigKeys lists all config keys that can be read/written via CLI.
var validConfigKeys = map[string]bool{
	"provider":             true,
	"note_title":           true,
	"theme":                true,
	"dev_dispatch_enabled": true,
	"llm.backend":          true,
}

// NewConfigCmd creates the `config` command group with show/get/set subcommands.
func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and modify ThreeDoors configuration",
	}

	cmd.AddCommand(newConfigShowCmd())
	cmd.AddCommand(newConfigGetCmd())
	cmd.AddCommand(newConfigSetCmd())

	return cmd
}

func configPath() (string, error) {
	configDir, err := core.GetConfigDirPath()
	if err != nil {
		return "", fmt.Errorf("get config dir: %w", err)
	}
	return filepath.Join(configDir, "config.yaml"), nil
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Display the full configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := configPath()
			if err != nil {
				return err
			}

			cfg, err := core.LoadProviderConfig(path)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			formatter := NewOutputFormatter(cmd.OutOrStdout(), jsonOutput)

			if formatter.IsJSON() {
				return formatter.WriteJSON("config.show", configToMap(cfg), nil)
			}

			return renderConfigHuman(formatter, cfg)
		},
	}
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a single configuration value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			if !validConfigKeys[key] {
				formatter := NewOutputFormatter(cmd.ErrOrStderr(), jsonOutput)
				if jsonOutput {
					_ = formatter.WriteJSONError("config.get", ExitValidation, "unknown config key", key)
				} else {
					_ = formatter.Writef("Error: unknown config key %q\n", key)
				}
				cmd.SilenceErrors = true
				return &exitError{code: ExitValidation}
			}

			path, err := configPath()
			if err != nil {
				return err
			}

			cfg, err := core.LoadProviderConfig(path)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			value := getConfigValue(cfg, key)
			formatter := NewOutputFormatter(cmd.OutOrStdout(), jsonOutput)

			if formatter.IsJSON() {
				return formatter.WriteJSON("config.get", map[string]string{"key": key, "value": value}, nil)
			}

			return formatter.Writef("%s\n", value)
		},
	}
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]
			if !validConfigKeys[key] {
				formatter := NewOutputFormatter(cmd.ErrOrStderr(), jsonOutput)
				if jsonOutput {
					_ = formatter.WriteJSONError("config.set", ExitValidation, "unknown config key", key)
				} else {
					_ = formatter.Writef("Error: unknown config key %q\n", key)
				}
				cmd.SilenceErrors = true
				return &exitError{code: ExitValidation}
			}

			path, err := configPath()
			if err != nil {
				return err
			}

			cfg, err := core.LoadProviderConfig(path)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			setConfigValue(cfg, key, value)

			if _, mkdirErr := core.EnsureConfigDir(); mkdirErr != nil {
				return fmt.Errorf("ensure config dir: %w", mkdirErr)
			}

			if err := core.SaveProviderConfig(path, cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			formatter := NewOutputFormatter(cmd.OutOrStdout(), jsonOutput)
			if formatter.IsJSON() {
				return formatter.WriteJSON("config.set", map[string]string{"key": key, "value": value}, nil)
			}

			return formatter.Writef("Set %s = %s\n", key, value)
		},
	}
}

// exitError wraps an exit code for cobra RunE returns.
type exitError struct {
	code int
}

func (e *exitError) Error() string {
	return fmt.Sprintf("exit code %d", e.code)
}

func getConfigValue(cfg *core.ProviderConfig, key string) string {
	switch key {
	case "provider":
		return cfg.Provider
	case "note_title":
		return cfg.NoteTitle
	case "theme":
		return cfg.Theme
	case "dev_dispatch_enabled":
		if cfg.DevDispatchEnabled {
			return "true"
		}
		return "false"
	case "llm.backend":
		return cfg.LLM.Backend
	default:
		return ""
	}
}

func setConfigValue(cfg *core.ProviderConfig, key, value string) {
	switch key {
	case "provider":
		cfg.Provider = value
	case "note_title":
		cfg.NoteTitle = value
	case "theme":
		cfg.Theme = value
	case "dev_dispatch_enabled":
		cfg.DevDispatchEnabled = strings.EqualFold(value, "true")
	case "llm.backend":
		cfg.LLM.Backend = value
	}
}

func configToMap(cfg *core.ProviderConfig) map[string]interface{} {
	// Marshal to YAML then unmarshal to map for a clean representation
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil
	}
	var m map[string]interface{}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil
	}
	return m
}

func renderConfigHuman(f *OutputFormatter, cfg *core.ProviderConfig) error {
	tw := f.TableWriter()
	if _, err := fmt.Fprintln(tw, "KEY\tVALUE"); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	rows := []struct {
		key   string
		value string
	}{
		{"provider", cfg.Provider},
		{"note_title", cfg.NoteTitle},
		{"theme", cfg.Theme},
		{"dev_dispatch_enabled", fmt.Sprintf("%v", cfg.DevDispatchEnabled)},
		{"llm.backend", cfg.LLM.Backend},
	}

	for _, r := range rows {
		if _, err := fmt.Fprintf(tw, "%s\t%s\n", r.key, r.value); err != nil {
			return fmt.Errorf("write row %s: %w", r.key, err)
		}
	}

	for i, p := range cfg.Providers {
		if _, err := fmt.Fprintf(tw, "providers[%d].name\t%s\n", i, p.Name); err != nil {
			return fmt.Errorf("write provider name: %w", err)
		}
		for k, v := range p.Settings {
			if _, err := fmt.Fprintf(tw, "providers[%d].%s\t%s\n", i, k, v); err != nil {
				return fmt.Errorf("write provider setting: %w", err)
			}
		}
	}

	if err := tw.Flush(); err != nil {
		return fmt.Errorf("flush table: %w", err)
	}
	return nil
}

// GetConfigFilePath returns the config file path for use by other commands.
func GetConfigFilePath() (string, error) {
	return configPath()
}

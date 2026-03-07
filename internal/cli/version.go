package cli

import (
	"runtime"

	"github.com/spf13/cobra"
)

// Build-time variables, set via -ldflags.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// versionData holds version info for JSON output.
type versionData struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
}

// newVersionCmd creates the version subcommand.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Display ThreeDoors version information",
		RunE: func(cmd *cobra.Command, _ []string) error {
			w := cmd.OutOrStdout()
			formatter := NewOutputFormatter(w, jsonOutput)

			data := versionData{
				Version:   Version,
				Commit:    Commit,
				BuildDate: BuildDate,
				GoVersion: runtime.Version(),
			}

			if formatter.IsJSON() {
				return formatter.WriteJSON("version", data, nil)
			}

			_ = formatter.Writef("ThreeDoors %s\n", data.Version)
			_ = formatter.Writef("Commit:     %s\n", data.Commit)
			_ = formatter.Writef("Built:      %s\n", data.BuildDate)
			_ = formatter.Writef("Go version: %s\n", data.GoVersion)
			return nil
		},
	}
}

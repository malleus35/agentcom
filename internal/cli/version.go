package cli

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		// version doesn't need DB, skip PersistentPreRunE
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			info := map[string]string{
				"version":   Version,
				"buildDate": BuildDate,
				"goVersion": GoVersion,
				"os":        runtime.GOOS,
				"arch":      runtime.GOARCH,
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(info)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "agentcom %s\n", Version)
			fmt.Fprintf(cmd.OutOrStdout(), "  Build Date: %s\n", BuildDate)
			fmt.Fprintf(cmd.OutOrStdout(), "  Go Version: %s\n", GoVersion)
			fmt.Fprintf(cmd.OutOrStdout(), "  OS/Arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
			return nil
		},
	}
}

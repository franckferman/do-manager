package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// These variables are overridden at build time via -ldflags.
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print do-manager version",
	RunE: func(cmd *cobra.Command, args []string) error {
		if isJSON() {
			return printJSON(map[string]string{
				"version":    Version,
				"commit":     Commit,
				"build_date": BuildDate,
			})
		}
		fmt.Printf("do-manager %s  (commit=%s  built=%s)\n", Version, Commit, BuildDate)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

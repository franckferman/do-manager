package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is the current release of do-manager.
const Version = "0.1.0"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print do-manager version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("do-manager v%s\n", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

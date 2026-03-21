// Package cmd implements the do-manager CLI using Cobra.
package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const banner = `
    ██████╗  ██████╗       ███╗   ███╗ █████╗ ███╗   ██╗ █████╗  ██████╗ ███████╗██████╗
    ██╔══██╗██╔═══██╗      ████╗ ████║██╔══██╗████╗  ██║██╔══██╗██╔════╝ ██╔════╝██╔══██╗
    ██║  ██║██║   ██║█████╗██╔████╔██║███████║██╔██╗ ██║███████║██║  ███╗█████╗  ██████╔╝
    ██║  ██║██║   ██║╚════╝██║╚██╔╝██║██╔══██║██║╚██╗██║██╔══██║██║   ██║██╔══╝  ██╔══██╗
    ██████╔╝╚██████╔╝      ██║ ╚═╝ ██║██║  ██║██║ ╚████║██║  ██║╚██████╔╝███████╗██║  ██║
    ╚═════╝  ╚═════╝       ╚═╝     ╚═╝╚═╝  ╚═╝╚═╝  ╚═══╝╚═╝  ╚═╝ ╚═════╝ ╚══════╝╚═╝  ╚═╝
`

var rootCmd = &cobra.Command{
	Use:   "do-manager",
	Short: "CLI & Go library for managing DigitalOcean infrastructure",
	Long:  color.CyanString(banner) + "\n    Manage Droplets, SSH keys, regions, sizes, and images from the command line.",
}

// Execute is the main entry point called by main().
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, color.RedString("Error: ")+err.Error())
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().String("token", "", "DigitalOcean API token (overrides DO_TOKEN env var)")
	viper.BindPFlag("token", rootCmd.PersistentFlags().Lookup("token")) //nolint:errcheck

	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "", "Output format: table (default) | json")

	// Suppress usage on subcommand errors (cleaner output).
	rootCmd.SilenceUsage = true
}

func initConfig() {
	viper.SetEnvPrefix("DO")
	viper.AutomaticEnv() // maps DO_TOKEN -> token, DO_* -> *

	home, err := os.UserHomeDir()
	if err == nil {
		viper.AddConfigPath(home)
		viper.SetConfigName(".do-manager")
		viper.SetConfigType("yaml")
	}
	_ = viper.ReadInConfig()
}

// Package cmd implements the pluginart CLI command tree.
package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "pluginart",
	Short: "Scaffold and generate code for pluginart plugins",
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(genCmd)
}

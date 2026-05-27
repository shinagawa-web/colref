package main

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:           "colref",
	Short:         "Check whether a DB column is still referenced before you delete it",
	SilenceUsage:  true,
	SilenceErrors: true,
	Version:       version,
}

func init() {
	rootCmd.SetVersionTemplate("{{.Name}} {{.Version}}\n")
}

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var exit = os.Exit

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:           "colref",
	Short:         "Check whether a DB column is still referenced before you delete it",
	SilenceUsage:  true,
	SilenceErrors: true,
}

var (
	flagModel string
	flagField string
	flagOrm   string
)

var checkCmd = &cobra.Command{
	Use:   "check [path]",
	Short: "Scan a codebase for references to a model field",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) == 1 {
			dir = args[0]
		}
		return runCheck(dir, flagModel, flagField, flagOrm)
	},
}

func init() {
	checkCmd.Flags().StringVar(&flagModel, "model", "", "Model name (e.g. User)")
	checkCmd.Flags().StringVar(&flagField, "field", "", "Field name (e.g. email)")
	checkCmd.Flags().StringVar(&flagOrm, "orm", "", "ORM type: django, rails")
	_ = checkCmd.MarkFlagRequired("model")
	_ = checkCmd.MarkFlagRequired("field")
	_ = checkCmd.MarkFlagRequired("orm")

	rootCmd.AddCommand(checkCmd)
}

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
	flagModel      string
	flagField      string
	flagModelsFile string
	flagSchemaFile string
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
		return runCheck(dir, flagModel, flagField, flagModelsFile, flagSchemaFile)
	},
}

func init() {
	checkCmd.Flags().StringVar(&flagModel, "model", "", "Model name (e.g. User)")
	checkCmd.Flags().StringVar(&flagField, "field", "", "Field name (e.g. email)")
	checkCmd.Flags().StringVar(&flagModelsFile, "models-file", "", "Path to models.py (auto-detected if omitted)")
	checkCmd.Flags().StringVar(&flagSchemaFile, "schema-file", "", "Path to db/schema.rb (auto-detected if omitted)")
	_ = checkCmd.MarkFlagRequired("model")
	_ = checkCmd.MarkFlagRequired("field")

	rootCmd.AddCommand(checkCmd)
}

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "colref",
	Short: "Check whether a DB column is still referenced before you delete it",
}

var (
	flagModel      string
	flagField      string
	flagModelsFile string
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
		fmt.Printf("check: model=%s field=%s dir=%s\n", flagModel, flagField, dir)
		return nil
	},
}

func init() {
	checkCmd.Flags().StringVar(&flagModel, "model", "", "Model name (e.g. User)")
	checkCmd.Flags().StringVar(&flagField, "field", "", "Field name (e.g. email)")
	checkCmd.Flags().StringVar(&flagModelsFile, "models-file", "", "Path to models.py (auto-detected if omitted)")
	_ = checkCmd.MarkFlagRequired("model")
	_ = checkCmd.MarkFlagRequired("field")

	rootCmd.AddCommand(checkCmd)
}

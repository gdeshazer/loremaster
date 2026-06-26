package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search the knowledge base",
	Long:  "Runs a hybrid semantic + keyword search scoped to the current project.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("search: not yet implemented (query=%q)\n", args[0])
		return nil
	},
}

func init() {
	searchCmd.Flags().Int("limit", 5, "Maximum number of results to return")
	rootCmd.AddCommand(searchCmd)
}

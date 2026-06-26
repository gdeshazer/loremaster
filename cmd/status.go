package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show index status for the current project",
	Long:  "Displays document count, last indexed time, and DB/Ollama connectivity for the current project.",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("status: not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

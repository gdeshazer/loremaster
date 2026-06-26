package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var indexCmd = &cobra.Command{
	Use:   "index [path]",
	Short: "Index markdown files into the knowledge base",
	Long:  "Walks [path] for .md files and upserts them into the project's search index.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}
		fmt.Printf("index: not yet implemented (path=%s)\n", path)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(indexCmd)
}

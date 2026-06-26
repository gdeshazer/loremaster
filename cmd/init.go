package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new project in the current directory",
	Long: `Creates a project record in the database and writes loremaster.json
and mcp.json into the current directory.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("init: not yet implemented")
		return nil
	},
}

func init() {
	initCmd.Flags().String("name", "", "Human-readable project name")
	initCmd.Flags().String("description", "", "Project description")
	initCmd.Flags().String("slug", "", "Project slug (defaults to directory name)")
	rootCmd.AddCommand(initCmd)
}

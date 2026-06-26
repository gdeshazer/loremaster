package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "Manage loremaster projects",
}

var projectsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all projects with doc counts and descriptions",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("projects list: not yet implemented")
		return nil
	},
}

var projectsDeleteCmd = &cobra.Command{
	Use:   "delete [slug]",
	Short: "Delete a project and all its documents",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("projects delete: not yet implemented (slug=%s)\n", args[0])
		return nil
	},
}

var projectsDescribeCmd = &cobra.Command{
	Use:   "describe [slug]",
	Short: "Show metadata for a project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("projects describe: not yet implemented (slug=%s)\n", args[0])
		return nil
	},
}

func init() {
	projectsCmd.AddCommand(projectsListCmd)
	projectsCmd.AddCommand(projectsDeleteCmd)
	projectsCmd.AddCommand(projectsDescribeCmd)
	rootCmd.AddCommand(projectsCmd)
}

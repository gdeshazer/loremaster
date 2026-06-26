package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP stdio server",
	Long:  "Starts loremaster as an MCP server over stdio, for use with Claude Desktop or Claude Code.",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("serve: not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "loremaster",
	Short: "LLM-queryable knowledge base for story files",
	Long: `Loremaster indexes your story's markdown files into a PostgreSQL
database (pgvector + full-text search) and exposes semantic and keyword
search via an MCP server for Claude, as well as a standalone CLI.

Quick start:
  1. Start the infrastructure:
       podman compose up -d
       podman exec <ollama-container> ollama pull nomic-embed-text

  2. Set required environment variables (or copy .env.example to .env):
       export LOREMASTER_DB_URL="postgres://loremaster:loremaster@localhost:5432/loremaster?sslmode=disable"
       export LOREMASTER_OLLAMA_URL="http://localhost:11434"

  3. Initialize a project in your story directory:
       cd ~/my-story && loremaster init

  4. Index your markdown files:
       loremaster index .

  5. Search from the CLI:
       loremaster search "how does the magic system work"

  6. Add mcp.json to Claude Code MCP settings to search from Claude.

Environment variables:
  LOREMASTER_DB_URL        PostgreSQL connection string (required)
  LOREMASTER_OLLAMA_URL    Ollama base URL (default: http://localhost:11434)
  LOREMASTER_OLLAMA_MODEL  Embedding model name (default: nomic-embed-text)
  LOREMASTER_EMBED_DIMS    Embedding dimensions (default: 768)
  LOREMASTER_PROJECT       Project slug (overridden by --project flag)`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().String("db-url", "", "PostgreSQL connection string (overrides LOREMASTER_DB_URL)")
	rootCmd.PersistentFlags().String("ollama-url", "http://localhost:11434", "Ollama base URL (overrides LOREMASTER_OLLAMA_URL)")
	rootCmd.PersistentFlags().String("project", "", "Project slug (overrides loremaster.json and LOREMASTER_PROJECT)")

	viper.BindPFlag("db_url", rootCmd.PersistentFlags().Lookup("db-url"))
	viper.BindPFlag("ollama_url", rootCmd.PersistentFlags().Lookup("ollama-url"))
	viper.BindPFlag("project", rootCmd.PersistentFlags().Lookup("project"))

	viper.BindEnv("db_url", "LOREMASTER_DB_URL")
	viper.BindEnv("ollama_url", "LOREMASTER_OLLAMA_URL")
	viper.BindEnv("ollama_model", "LOREMASTER_OLLAMA_MODEL")
	viper.BindEnv("embed_dims", "LOREMASTER_EMBED_DIMS")
	viper.BindEnv("project", "LOREMASTER_PROJECT")

	viper.SetDefault("ollama_url", "http://localhost:11434")
	viper.SetDefault("ollama_model", "nomic-embed-text")
	viper.SetDefault("embed_dims", 768)
}

func initConfig() {
	// Project-local config is loaded by internal/config; viper here handles globals.
	viper.SetEnvPrefix("LOREMASTER")
	viper.AutomaticEnv()
}

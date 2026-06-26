package cmd

import (
	"fmt"
	"os"

	"github.com/gdeshazer/loremaster/internal/config"
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

  2. Initialize a project in your story directory (set db_url and ollama_url
     in loremaster.json, or export them as env vars beforehand):
       cd ~/my-story && loremaster init

  3. Index your markdown files:
       loremaster index .

  4. Search from the CLI:
       loremaster search "how does the magic system work"

  5. Add mcp.json to Claude Code MCP settings to search from Claude.

Configuration priority (highest wins):
  --flag  >  loremaster.json  >  LOREMASTER_* env var  >  built-in default

loremaster.json fields (project-local, overrides env vars):
  db_url           PostgreSQL connection string
  ollama_url       Ollama base URL
  ollama_model     Embedding model name (field: embedding_model)
  project          Project slug

Environment variables (global fallback):
  LOREMASTER_DB_URL        PostgreSQL connection string (required if not in loremaster.json)
  LOREMASTER_OLLAMA_URL    Ollama base URL (default: http://localhost:11434)
  LOREMASTER_OLLAMA_MODEL  Embedding model name (default: nomic-embed-text)
  LOREMASTER_EMBED_DIMS    Embedding dimensions (default: 768)
  LOREMASTER_PROJECT       Project slug (overridden by --project flag)`,

	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load("")
		if err != nil {
			return nil // loremaster.json is optional
		}
		flags := cmd.Root().PersistentFlags()
		// Config file values override env vars but not explicit flags.
		if cfg.DBURL != "" && !flags.Changed("db-url") {
			viper.Set("db_url", cfg.DBURL)
		}
		if cfg.OllamaURL != "" && !flags.Changed("ollama-url") {
			viper.Set("ollama_url", cfg.OllamaURL)
		}
		return nil
	},
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

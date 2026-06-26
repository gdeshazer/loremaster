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
	Long: `Loremaster indexes your story's markdown files and exposes
semantic and full-text search via MCP (for Claude) and CLI.`,
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

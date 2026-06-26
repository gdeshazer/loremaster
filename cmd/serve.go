package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/gdeshazer/loremaster/internal/config"
	"github.com/gdeshazer/loremaster/internal/db"
	"github.com/gdeshazer/loremaster/internal/embed"
	lmcp "github.com/gdeshazer/loremaster/internal/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP stdio server",
	Long: `Starts loremaster as an MCP server communicating over stdin/stdout,
following the Model Context Protocol (MCP) specification.

This command is typically launched automatically by the MCP host (Claude
Desktop, Claude Code, Cursor) using the config in mcp.json — you do not
usually run it directly. The binary path and environment variables in mcp.json
are passed through from 'loremaster init'.

Tools exposed to the LLM:
  hybrid_search    Semantic + keyword search merged via RRF (best default)
  semantic_search  Vector similarity search (conceptual/thematic queries)
  keyword_search   Full-text search (exact names, phrases, boolean operators)
  get_document     Retrieve full content of a file by path
  list_documents   List all indexed files in a project
  list_projects    List all projects with doc counts

All search tools require a "project" parameter matching the slug in loremaster.json.`,
	Example: `  # Start the MCP server (usually called by the MCP host, not directly)
  loremaster serve

  # With explicit DB and Ollama settings
  LOREMASTER_DB_URL="postgres://..." loremaster serve`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		dbURL := viper.GetString("db_url")
		if dbURL == "" {
			return fmt.Errorf("LOREMASTER_DB_URL is required (or --db-url flag)")
		}

		pool, err := db.Connect(ctx, dbURL)
		if err != nil {
			return fmt.Errorf("connecting to database: %w", err)
		}
		defer pool.Close()

		if err := db.Migrate(ctx, pool); err != nil {
			return fmt.Errorf("running migrations: %w", err)
		}

		ollamaURL := viper.GetString("ollama_url")
		ollamaModel := viper.GetString("ollama_model")

		// Allow loremaster.json to override the embedding model.
		if cfg, err := config.Load(""); err == nil && cfg.EmbeddingModel != "" {
			ollamaModel = cfg.EmbeddingModel
		}

		embedder, err := embed.NewClient(ollamaURL, ollamaModel)
		if err != nil {
			return fmt.Errorf("creating embed client: %w", err)
		}

		store := db.NewStore(pool)
		h := lmcp.NewHandler(store, embedder)
		s := lmcp.NewServer(h)

		fmt.Fprintln(os.Stderr, "loremaster MCP server starting on stdio")
		return server.ServeStdio(s)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

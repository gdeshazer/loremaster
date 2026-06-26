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
	Long:  "Starts loremaster as an MCP server over stdio, for use with Claude Desktop or Claude Code.",
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

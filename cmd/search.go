package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/gdeshazer/loremaster/internal/config"
	"github.com/gdeshazer/loremaster/internal/db"
	"github.com/gdeshazer/loremaster/internal/embed"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search the knowledge base",
	Long: `Runs a hybrid search (semantic + keyword, merged via Reciprocal Rank Fusion)
scoped to the current project and prints results as JSON to stdout.

The project is resolved from loremaster.json in the current directory tree,
or overridden with --project. The embedding model used for the query vector
is taken from loremaster.json (embedding_model field) or the global default.

Output fields per result:
  file_path    Relative path of the source markdown file
  chunk_index  Which chunk within the file (0-based)
  title        Title from frontmatter or first heading
  content      The matched text excerpt
  score        RRF relevance score (higher = more relevant)
  metadata     Frontmatter fields (characters, location, tags, etc.)`,
	Example: `  # Basic search
  loremaster search "how does the magic system work"

  # Limit results
  loremaster search "Roland's backstory" --limit 10

  # Search a specific project
  loremaster search "the dark tower" --project my-novel`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		query := args[0]

		cfg, err := config.Load("")
		if err != nil {
			return fmt.Errorf("no loremaster.json found — run `loremaster init` first: %w", err)
		}

		projectSlug := viper.GetString("project")
		if projectSlug == "" {
			projectSlug = cfg.Project
		}

		ollamaModel := viper.GetString("ollama_model")
		if cfg.EmbeddingModel != "" {
			ollamaModel = cfg.EmbeddingModel
		}

		dbURL := viper.GetString("db_url")
		if dbURL == "" {
			return fmt.Errorf("LOREMASTER_DB_URL is required")
		}

		limit, _ := cmd.Flags().GetInt("limit")

		pool, err := db.Connect(ctx, dbURL)
		if err != nil {
			return err
		}
		defer pool.Close()

		store := db.NewStore(pool)
		project, err := store.GetProject(ctx, projectSlug)
		if err != nil {
			return fmt.Errorf("project %q not found", projectSlug)
		}

		embedder, err := embed.NewClient(viper.GetString("ollama_url"), ollamaModel)
		if err != nil {
			return err
		}

		vec, err := embedder.Embed(ctx, query)
		if err != nil {
			return fmt.Errorf("embedding query: %w", err)
		}

		results, err := store.HybridSearch(ctx, project.ID, query, vec, limit)
		if err != nil {
			return fmt.Errorf("search: %w", err)
		}

		if len(results) == 0 {
			fmt.Println("No results found.")
			return nil
		}

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	},
}

func init() {
	searchCmd.Flags().Int("limit", 5, "Maximum number of results to return")
	rootCmd.AddCommand(searchCmd)
}

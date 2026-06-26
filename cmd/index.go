package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/gdeshazer/loremaster/internal/config"
	"github.com/gdeshazer/loremaster/internal/db"
	"github.com/gdeshazer/loremaster/internal/embed"
	"github.com/gdeshazer/loremaster/internal/ingest"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var indexCmd = &cobra.Command{
	Use:   "index [path]",
	Short: "Index markdown files into the knowledge base",
	Long: `Walks [path] recursively for .md files and upserts each one into the
project's search index. Defaults to the current directory if no path is given.

Each file is parsed for YAML frontmatter and plaintext content, split into
overlapping chunks (~300 words, 40-word overlap), embedded via Ollama, and
stored in PostgreSQL (pgvector for semantic search, tsvector for keyword search).

Re-running index on already-indexed files is safe — chunks are upserted,
not duplicated. Files deleted from disk are not automatically removed from
the index; use 'loremaster projects delete' and re-index to fully reset.

Reads exclude glob patterns from loremaster.json (e.g. "drafts/**", "*.bak.md").
The project is resolved from loremaster.json in the current directory tree.`,
	Example: `  # Index the current directory
  loremaster index .

  # Index a specific subdirectory
  loremaster index ./chapters

  # Override the project slug
  loremaster index . --project my-novel`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		root := "."
		if len(args) > 0 {
			root = args[0]
		}

		// Load project config.
		cfg, err := config.Load("")
		if err != nil {
			return fmt.Errorf("no loremaster.json found — run `loremaster init` first: %w", err)
		}

		// Flags and env take priority over config file.
		projectSlug := viper.GetString("project")
		if projectSlug == "" {
			projectSlug = cfg.Project
		}
		if projectSlug == "" {
			return fmt.Errorf("project slug is required (set in loremaster.json or --project flag)")
		}

		ollamaModel := viper.GetString("ollama_model")
		if cfg.EmbeddingModel != "" {
			ollamaModel = cfg.EmbeddingModel
		}

		dbURL := viper.GetString("db_url")
		if dbURL == "" {
			return fmt.Errorf("LOREMASTER_DB_URL is required")
		}

		pool, err := db.Connect(ctx, dbURL)
		if err != nil {
			return fmt.Errorf("connecting to database: %w", err)
		}
		defer pool.Close()

		store := db.NewStore(pool)
		project, err := store.GetProject(ctx, projectSlug)
		if err != nil {
			return fmt.Errorf("project %q not found — run `loremaster init` first", projectSlug)
		}

		embedder, err := embed.NewClient(viper.GetString("ollama_url"), ollamaModel)
		if err != nil {
			return err
		}

		// Walk directory for .md files.
		paths, err := ingest.Walk(root, cfg.Exclude)
		if err != nil {
			return fmt.Errorf("walking %s: %w", root, err)
		}
		if len(paths) == 0 {
			fmt.Println("No .md files found.")
			return nil
		}
		fmt.Printf("Found %d file(s) to index\n", len(paths))

		indexed, skipped := 0, 0
		for _, path := range paths {
			src, err := os.ReadFile(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "skip %s: read error: %v\n", path, err)
				skipped++
				continue
			}

			doc, err := ingest.Parse(src, path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "skip %s: parse error: %v\n", path, err)
				skipped++
				continue
			}

			if doc.Plaintext == "" {
				skipped++
				continue
			}

			chunks := ingest.ChunkText(doc.Plaintext, ingest.DefaultChunkSize, ingest.DefaultOverlap)
			for _, chunk := range chunks {
				vec, err := embedder.Embed(ctx, chunk.Content)
				if err != nil {
					fmt.Fprintf(os.Stderr, "skip %s chunk %d: embed error: %v\n", path, chunk.Index, err)
					continue
				}
				if err := store.UpsertChunk(ctx, project.ID, path, chunk.Index, doc.Title, chunk.Content, vec, doc.Metadata); err != nil {
					fmt.Fprintf(os.Stderr, "skip %s chunk %d: upsert error: %v\n", path, chunk.Index, err)
					continue
				}
			}
			indexed++
			fmt.Printf("  indexed %s (%d chunks)\n", path, len(chunks))
		}

		fmt.Printf("\nDone: %d indexed, %d skipped\n", indexed, skipped)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(indexCmd)
}

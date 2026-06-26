package cmd

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gdeshazer/loremaster/internal/config"
	"github.com/gdeshazer/loremaster/internal/db"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show index status for the current project",
	Long:  "Displays document count, last indexed time, and DB/Ollama connectivity for the current project.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		cfg, err := config.Load("")
		if err != nil {
			fmt.Println("No loremaster.json found in this directory tree.")
		}

		projectSlug := viper.GetString("project")
		if cfg != nil && projectSlug == "" {
			projectSlug = cfg.Project
		}

		dbURL := viper.GetString("db_url")
		ollamaURL := viper.GetString("ollama_url")

		// DB connectivity
		dbOK := false
		if dbURL != "" {
			pool, err := db.Connect(ctx, dbURL)
			if err == nil {
				dbOK = true
				if projectSlug != "" {
					store := db.NewStore(pool)
					if p, err := store.GetProject(ctx, projectSlug); err == nil {
						count, _ := store.DocCount(ctx, p.ID)
						fmt.Printf("Project:   %s (%s)\n", p.Name, p.Slug)
						fmt.Printf("Documents: %d chunks indexed\n", count)
					} else {
						fmt.Printf("Project %q not found in database.\n", projectSlug)
					}
				}
				pool.Close()
			}
		}

		// Ollama connectivity (simple HEAD check)
		ollamaOK := false
		if ollamaURL != "" {
			client := &http.Client{Timeout: 3 * time.Second}
			resp, err := client.Get(ollamaURL)
			if err == nil {
				resp.Body.Close()
				ollamaOK = true
			}
		}

		fmt.Println()
		fmt.Printf("DB:     %s  (%s)\n", checkmark(dbOK), dbURL)
		fmt.Printf("Ollama: %s  (%s)\n", checkmark(ollamaOK), ollamaURL)
		return nil
	},
}

func checkmark(ok bool) string {
	if ok {
		return "OK"
	}
	return "UNREACHABLE"
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gdeshazer/loremaster/internal/config"
	"github.com/gdeshazer/loremaster/internal/db"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new project in the current directory",
	Long: `Creates a project record in the database and writes loremaster.json
and mcp.json into the current directory.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		slug, _ := cmd.Flags().GetString("slug")
		name, _ := cmd.Flags().GetString("name")
		description, _ := cmd.Flags().GetString("description")

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		if slug == "" {
			slug = filepath.Base(cwd)
		}
		if name == "" {
			name = slug
		}

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

		store := db.NewStore(pool)
		project, err := store.CreateProject(ctx, slug, name, description)
		if err != nil {
			return fmt.Errorf("creating project: %w", err)
		}
		fmt.Printf("Created project: %s (ID %d)\n", project.Slug, project.ID)

		// Write loremaster.json
		cfg := &config.ProjectConfig{
			Project:        slug,
			EmbeddingModel: viper.GetString("ollama_model"),
		}
		if err := config.Write(cwd, cfg); err != nil {
			return fmt.Errorf("writing loremaster.json: %w", err)
		}
		fmt.Println("Wrote loremaster.json")

		// Write mcp.json (Claude MCP server config block)
		binary, err := os.Executable()
		if err != nil {
			binary = "loremaster"
		}
		mcpConfig := map[string]interface{}{
			"mcpServers": map[string]interface{}{
				"loremaster": map[string]interface{}{
					"command": binary,
					"args":    []string{"serve"},
					"env": map[string]string{
						"LOREMASTER_DB_URL":      dbURL,
						"LOREMASTER_OLLAMA_URL":  viper.GetString("ollama_url"),
						"LOREMASTER_OLLAMA_MODEL": viper.GetString("ollama_model"),
					},
				},
			},
		}
		mcpData, err := json.MarshalIndent(mcpConfig, "", "  ")
		if err != nil {
			return err
		}
		mcpPath := filepath.Join(cwd, "mcp.json")
		if err := os.WriteFile(mcpPath, append(mcpData, '\n'), 0644); err != nil {
			return fmt.Errorf("writing mcp.json: %w", err)
		}
		fmt.Println("Wrote mcp.json")
		fmt.Printf("\nTo use with Claude Code, add the contents of mcp.json to your Claude Code MCP settings.\n")
		fmt.Printf("Then run: loremaster index .\n")
		return nil
	},
}

func init() {
	initCmd.Flags().String("name", "", "Human-readable project name (defaults to slug)")
	initCmd.Flags().String("description", "", "Project description")
	initCmd.Flags().String("slug", "", "Project slug (defaults to directory name)")
	rootCmd.AddCommand(initCmd)
}

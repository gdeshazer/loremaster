package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"strings"

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

		// Write or append to CLAUDE.md with loremaster tool guidance.
		if err := writeCLAUDEMD(cwd, slug, name); err != nil {
			return fmt.Errorf("writing CLAUDE.md: %w", err)
		}
		fmt.Println("Wrote CLAUDE.md (loremaster tool guidance)")

		fmt.Printf("\nNext steps:\n")
		fmt.Printf("  1. Add mcp.json to your Claude Code MCP settings\n")
		fmt.Printf("  2. loremaster index .\n")
		return nil
	},
}

// claudeMDMarker is written into CLAUDE.md so re-running init doesn't duplicate the section.
const claudeMDMarker = "<!-- loremaster -->"

func writeCLAUDEMD(dir, slug, projectName string) error {
	section := fmt.Sprintf(`%s
## Loremaster — Story Knowledge Base

This project (%s) uses **loremaster** for semantic and full-text search over story files.
The MCP server exposes the following tools (all require ` + "`" + `"project": "%s"` + "`" + `):

| Tool | When to use |
|---|---|
| ` + "`" + `hybrid_search` + "`" + ` | **Default.** Combines semantic + keyword results via RRF. Use for most queries. |
| ` + "`" + `semantic_search` + "`" + ` | Conceptual or thematic questions ("scenes about betrayal", "magic system rules"). |
| ` + "`" + `keyword_search` + "`" + ` | Exact names, places, or specific phrases ("Roland", "Dark Tower", "Affiliation"). |
| ` + "`" + `get_document` + "`" + ` | Retrieve the full contents of a specific file by its path. |
| ` + "`" + `list_documents` + "`" + ` | Browse all indexed files (useful when you need to know what exists). |
| ` + "`" + `list_projects` + "`" + ` | List all projects in the knowledge base. |

### Tips

- Always pass ` + "`" + `"project": "%s"` + "`" + ` to scope results to this project.
- The ` + "`" + `filter` + "`" + ` parameter accepts JSONB metadata from frontmatter:
  ` + "`" + `{"characters": "Roland"}` + "`" + ` — match files where the ` + "`" + `characters` + "`" + ` frontmatter field contains "Roland"
  ` + "`" + `{"location": "Forest", "tags": "magic"}` + "`" + ` — AND across multiple keys
- Use ` + "`" + `list_documents` + "`" + ` first if you're unsure what files exist, then ` + "`" + `get_document` + "`" + ` to read one in full.
- Re-index after editing files: ` + "`" + `loremaster index .` + "`" + `
`, claudeMDMarker, projectName, slug, slug)

	claudePath := filepath.Join(dir, "CLAUDE.md")

	existing, err := os.ReadFile(claudePath)
	if err == nil {
		// File exists — only append if our section isn't already there.
		if !containsMarker(existing, claudeMDMarker) {
			f, err := os.OpenFile(claudePath, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = fmt.Fprintf(f, "\n%s", section)
			return err
		}
		// Already present — nothing to do.
		return nil
	}

	// No CLAUDE.md yet — create it.
	return os.WriteFile(claudePath, []byte(section), 0644)
}

func containsMarker(data []byte, marker string) bool {
	return strings.Contains(string(data), marker)
}

func init() {
	initCmd.Flags().String("name", "", "Human-readable project name (defaults to slug)")
	initCmd.Flags().String("description", "", "Project description")
	initCmd.Flags().String("slug", "", "Project slug (defaults to directory name)")
	rootCmd.AddCommand(initCmd)
}

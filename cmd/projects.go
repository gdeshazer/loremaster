package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/gdeshazer/loremaster/internal/db"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "Manage loremaster projects",
	Long: `Commands for listing, inspecting, and deleting loremaster projects.

Each project is an independent namespace in the database. Documents from
different projects never appear in each other's search results. A project
is created by 'loremaster init' and populated by 'loremaster index'.`,
	Example: `  loremaster projects list
  loremaster projects describe my-novel
  loremaster projects delete old-draft`,
}

var projectsListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all projects with doc counts and descriptions",
	Example: "  loremaster projects list",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		store, pool, err := openStore(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		summaries, err := store.ListProjects(ctx)
		if err != nil {
			return err
		}
		if len(summaries) == 0 {
			fmt.Println("No projects found. Run `loremaster init` to create one.")
			return nil
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(summaries)
	},
}

var projectsDeleteCmd = &cobra.Command{
	Use:   "delete [slug]",
	Short: "Delete a project and all its documents",
	Long: `Permanently removes the project record and all its indexed document
chunks (via CASCADE). This cannot be undone. Re-run 'loremaster init' and
'loremaster index' to rebuild the project from scratch.`,
	Example: "  loremaster projects delete old-draft",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		store, pool, err := openStore(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		slug := args[0]
		if err := store.DeleteProject(ctx, slug); err != nil {
			return err
		}
		fmt.Printf("Deleted project %q and all its documents.\n", slug)
		return nil
	},
}

var projectsDescribeCmd = &cobra.Command{
	Use:     "describe [slug]",
	Short:   "Show metadata for a project",
	Example: "  loremaster projects describe my-novel",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		store, pool, err := openStore(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()

		p, err := store.GetProject(ctx, args[0])
		if err != nil {
			return err
		}

		count, _ := store.DocCount(ctx, p.ID)
		fmt.Printf("Slug:        %s\n", p.Slug)
		fmt.Printf("Name:        %s\n", p.Name)
		fmt.Printf("Description: %s\n", p.Description)
		fmt.Printf("Created:     %s\n", p.CreatedAt.Format("2006-01-02 15:04"))
		fmt.Printf("Chunks:      %d\n", count)
		return nil
	},
}

func openStore(ctx context.Context) (*db.Store, interface{ Close() }, error) {
	dbURL := viper.GetString("db_url")
	if dbURL == "" {
		return nil, nil, fmt.Errorf("LOREMASTER_DB_URL is required")
	}
	pool, err := db.Connect(ctx, dbURL)
	if err != nil {
		return nil, nil, err
	}
	return db.NewStore(pool), pool, nil
}

func init() {
	projectsCmd.AddCommand(projectsListCmd)
	projectsCmd.AddCommand(projectsDeleteCmd)
	projectsCmd.AddCommand(projectsDescribeCmd)
	rootCmd.AddCommand(projectsCmd)
}

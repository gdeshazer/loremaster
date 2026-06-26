// Package mcp wires loremaster's search capabilities into an MCP server.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gdeshazer/loremaster/internal/db"
	"github.com/gdeshazer/loremaster/internal/embed"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Searcher is the interface the MCP tool handlers depend on.
// This allows unit testing without a real database.
type Searcher interface {
	GetProject(ctx context.Context, slug string) (*db.Project, error)
	ListProjects(ctx context.Context) ([]db.ProjectSummary, error)
	SemanticSearch(ctx context.Context, projectID int64, queryVec []float32, limit int) ([]db.SearchResult, error)
	KeywordSearch(ctx context.Context, projectID int64, query string, limit int) ([]db.SearchResult, error)
	HybridSearch(ctx context.Context, projectID int64, query string, queryVec []float32, limit int) ([]db.SearchResult, error)
	GetDocument(ctx context.Context, projectID int64, filePath string) ([]db.SearchResult, error)
	ListDocuments(ctx context.Context, projectID int64, limit int) ([]db.DocumentSummary, error)
}

// Embedder is the interface for generating query embeddings.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// Handler holds dependencies for all MCP tool handlers.
type Handler struct {
	store   Searcher
	embedder Embedder
}

// NewHandler creates a Handler.
func NewHandler(store Searcher, embedder Embedder) *Handler {
	return &Handler{store: store, embedder: embedder}
}

// NewServer builds and returns a configured MCP server with all tools registered.
func NewServer(h *Handler) *server.MCPServer {
	s := server.NewMCPServer(
		"loremaster",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	s.AddTool(toolSemanticSearch(), h.handleSemanticSearch)
	s.AddTool(toolKeywordSearch(), h.handleKeywordSearch)
	s.AddTool(toolHybridSearch(), h.handleHybridSearch)
	s.AddTool(toolGetDocument(), h.handleGetDocument)
	s.AddTool(toolListDocuments(), h.handleListDocuments)
	s.AddTool(toolListProjects(), h.handleListProjects)

	return s
}

// --- Tool definitions ---

func toolSemanticSearch() mcpgo.Tool {
	return mcpgo.NewTool("semantic_search",
		mcpgo.WithDescription("Search the story knowledge base using vector similarity (semantic meaning). Returns chunks ranked by cosine distance to the query."),
		mcpgo.WithString("project", mcpgo.Required(), mcpgo.Description("Project slug to search within.")),
		mcpgo.WithString("query", mcpgo.Required(), mcpgo.Description("Natural language search query.")),
		mcpgo.WithNumber("limit", mcpgo.Description("Maximum number of results (default 5, max 20).")),
	)
}

func toolKeywordSearch() mcpgo.Tool {
	return mcpgo.NewTool("keyword_search",
		mcpgo.WithDescription("Search the story knowledge base using full-text keyword matching (PostgreSQL FTS). Best for exact names, places, or specific phrases."),
		mcpgo.WithString("project", mcpgo.Required(), mcpgo.Description("Project slug to search within.")),
		mcpgo.WithString("query", mcpgo.Required(), mcpgo.Description("Keyword search query. Supports AND, OR, NOT, and phrase quotes.")),
		mcpgo.WithNumber("limit", mcpgo.Description("Maximum number of results (default 5, max 20).")),
	)
}

func toolHybridSearch() mcpgo.Tool {
	return mcpgo.NewTool("hybrid_search",
		mcpgo.WithDescription("Search using both semantic similarity and keyword matching, merged with Reciprocal Rank Fusion (RRF). The best general-purpose search — use this when unsure which search type fits."),
		mcpgo.WithString("project", mcpgo.Required(), mcpgo.Description("Project slug to search within.")),
		mcpgo.WithString("query", mcpgo.Required(), mcpgo.Description("Search query (used for both semantic and keyword search).")),
		mcpgo.WithNumber("limit", mcpgo.Description("Maximum number of results (default 5, max 20).")),
	)
}

func toolGetDocument() mcpgo.Tool {
	return mcpgo.NewTool("get_document",
		mcpgo.WithDescription("Retrieve the full content of a specific story file by its path. Returns all chunks concatenated in order."),
		mcpgo.WithString("project", mcpgo.Required(), mcpgo.Description("Project slug.")),
		mcpgo.WithString("file_path", mcpgo.Required(), mcpgo.Description("Relative file path as returned by search results.")),
	)
}

func toolListDocuments() mcpgo.Tool {
	return mcpgo.NewTool("list_documents",
		mcpgo.WithDescription("List all indexed files in a project with their titles and chunk counts."),
		mcpgo.WithString("project", mcpgo.Required(), mcpgo.Description("Project slug.")),
		mcpgo.WithNumber("limit", mcpgo.Description("Maximum number of files to list (default 50).")),
	)
}

func toolListProjects() mcpgo.Tool {
	return mcpgo.NewTool("list_projects",
		mcpgo.WithDescription("List all loremaster projects with their document counts and descriptions."),
	)
}

// --- Tool handlers ---

func (h *Handler) handleSemanticSearch(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	project, query, limit, err := parseSearchArgs(req)
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}

	p, err := h.store.GetProject(ctx, project)
	if err != nil {
		return mcpgo.NewToolResultError(fmt.Sprintf("project %q not found: %v", project, err)), nil
	}

	vec, err := h.embedder.Embed(ctx, query)
	if err != nil {
		return mcpgo.NewToolResultError(fmt.Sprintf("embedding query: %v", err)), nil
	}

	results, err := h.store.SemanticSearch(ctx, p.ID, vec, limit)
	if err != nil {
		return mcpgo.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	return resultsToToolResult(results)
}

func (h *Handler) handleKeywordSearch(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	project, query, limit, err := parseSearchArgs(req)
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}

	p, err := h.store.GetProject(ctx, project)
	if err != nil {
		return mcpgo.NewToolResultError(fmt.Sprintf("project %q not found: %v", project, err)), nil
	}

	results, err := h.store.KeywordSearch(ctx, p.ID, query, limit)
	if err != nil {
		return mcpgo.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	return resultsToToolResult(results)
}

func (h *Handler) handleHybridSearch(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	project, query, limit, err := parseSearchArgs(req)
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}

	p, err := h.store.GetProject(ctx, project)
	if err != nil {
		return mcpgo.NewToolResultError(fmt.Sprintf("project %q not found: %v", project, err)), nil
	}

	vec, err := h.embedder.Embed(ctx, query)
	if err != nil {
		return mcpgo.NewToolResultError(fmt.Sprintf("embedding query: %v", err)), nil
	}

	results, err := h.store.HybridSearch(ctx, p.ID, query, vec, limit)
	if err != nil {
		return mcpgo.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	return resultsToToolResult(results)
}

func (h *Handler) handleGetDocument(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	args := req.GetArguments()
	project, ok := args["project"].(string)
	if !ok || project == "" {
		return mcpgo.NewToolResultError("project is required"), nil
	}
	filePath, ok := args["file_path"].(string)
	if !ok || filePath == "" {
		return mcpgo.NewToolResultError("file_path is required"), nil
	}

	p, err := h.store.GetProject(ctx, project)
	if err != nil {
		return mcpgo.NewToolResultError(fmt.Sprintf("project %q not found: %v", project, err)), nil
	}

	chunks, err := h.store.GetDocument(ctx, p.ID, filePath)
	if err != nil {
		return mcpgo.NewToolResultError(fmt.Sprintf("get document: %v", err)), nil
	}
	if len(chunks) == 0 {
		return mcpgo.NewToolResultError(fmt.Sprintf("file %q not found in project %q", filePath, project)), nil
	}

	return resultsToToolResult(chunks)
}

func (h *Handler) handleListDocuments(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	args := req.GetArguments()
	project, ok := args["project"].(string)
	if !ok || project == "" {
		return mcpgo.NewToolResultError("project is required"), nil
	}
	limit := 50
	if l, ok := args["limit"]; ok {
		limit = clampLimit(toInt(l), 1, 200)
	}

	p, err := h.store.GetProject(ctx, project)
	if err != nil {
		return mcpgo.NewToolResultError(fmt.Sprintf("project %q not found: %v", project, err)), nil
	}

	docs, err := h.store.ListDocuments(ctx, p.ID, limit)
	if err != nil {
		return mcpgo.NewToolResultError(fmt.Sprintf("list documents: %v", err)), nil
	}

	data, _ := json.Marshal(docs)
	return mcpgo.NewToolResultText(string(data)), nil
}

func (h *Handler) handleListProjects(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	projects, err := h.store.ListProjects(ctx)
	if err != nil {
		return mcpgo.NewToolResultError(fmt.Sprintf("list projects: %v", err)), nil
	}
	data, _ := json.Marshal(projects)
	return mcpgo.NewToolResultText(string(data)), nil
}

// --- Helpers ---

func parseSearchArgs(req mcpgo.CallToolRequest) (project, query string, limit int, err error) {
	args := req.GetArguments()
	project, ok := args["project"].(string)
	if !ok || project == "" {
		return "", "", 0, fmt.Errorf("project is required")
	}
	query, ok = args["query"].(string)
	if !ok || query == "" {
		return "", "", 0, fmt.Errorf("query is required")
	}
	limit = 5
	if l, ok := args["limit"]; ok {
		limit = clampLimit(toInt(l), 1, 20)
	}
	return project, query, limit, nil
}

func resultsToToolResult(results []db.SearchResult) (*mcpgo.CallToolResult, error) {
	data, err := json.Marshal(results)
	if err != nil {
		return mcpgo.NewToolResultError(fmt.Sprintf("serializing results: %v", err)), nil
	}
	return mcpgo.NewToolResultText(string(data)), nil
}

func toInt(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case string:
		i, _ := strconv.Atoi(n)
		return i
	}
	return 0
}

func clampLimit(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v

}

// compile-time check that *embed.Client satisfies Embedder
var _ Embedder = (*embed.Client)(nil)

// compile-time check that *db.Store satisfies Searcher
var _ Searcher = (*db.Store)(nil)

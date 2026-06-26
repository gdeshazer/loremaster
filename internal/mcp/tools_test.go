package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/gdeshazer/loremaster/internal/db"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// mockSearcher implements Searcher with configurable return values.
type mockSearcher struct {
	project  *db.Project
	projectErr error
	projects []db.ProjectSummary
	results  []db.SearchResult
	searchErr error
	docs     []db.DocumentSummary
}

func (m *mockSearcher) GetProject(_ context.Context, slug string) (*db.Project, error) {
	return m.project, m.projectErr
}
func (m *mockSearcher) ListProjects(_ context.Context) ([]db.ProjectSummary, error) {
	return m.projects, nil
}
func (m *mockSearcher) SemanticSearch(_ context.Context, _ int64, _ []float32, _ int) ([]db.SearchResult, error) {
	return m.results, m.searchErr
}
func (m *mockSearcher) KeywordSearch(_ context.Context, _ int64, _ string, _ int) ([]db.SearchResult, error) {
	return m.results, m.searchErr
}
func (m *mockSearcher) HybridSearch(_ context.Context, _ int64, _ string, _ []float32, _ int) ([]db.SearchResult, error) {
	return m.results, m.searchErr
}
func (m *mockSearcher) GetDocument(_ context.Context, _ int64, _ string) ([]db.SearchResult, error) {
	return m.results, m.searchErr
}
func (m *mockSearcher) ListDocuments(_ context.Context, _ int64, _ int) ([]db.DocumentSummary, error) {
	return m.docs, nil
}

// mockEmbedder returns a fixed vector.
type mockEmbedder struct {
	vec []float32
	err error
}

func (m *mockEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return m.vec, m.err
}

func makeRequest(args map[string]interface{}) mcpgo.CallToolRequest {
	req := mcpgo.CallToolRequest{}
	if args != nil {
		req.Params.Arguments = map[string]any(args)
	}
	return req
}

func TestHandleSemanticSearch_success(t *testing.T) {
	store := &mockSearcher{
		project: &db.Project{ID: 1, Slug: "my-novel"},
		results: []db.SearchResult{
			{FilePath: "ch1.md", ChunkIndex: 0, Content: "Roland walked east.", Score: 0.95},
		},
	}
	embedder := &mockEmbedder{vec: []float32{0.1, 0.2, 0.3}}
	h := NewHandler(store, embedder)

	req := makeRequest(map[string]interface{}{
		"project": "my-novel",
		"query":   "magic system",
	})
	result, err := h.handleSemanticSearch(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %v", result.Content)
	}

	// Result should be JSON-parseable SearchResult slice.
	var results []db.SearchResult
	text := result.Content[0].(mcpgo.TextContent).Text
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(results) != 1 || results[0].FilePath != "ch1.md" {
		t.Errorf("unexpected results: %v", results)
	}
}

func TestHandleSemanticSearch_missingProject(t *testing.T) {
	h := NewHandler(&mockSearcher{}, &mockEmbedder{vec: []float32{0.1}})
	req := makeRequest(map[string]interface{}{"query": "something"})
	result, _ := h.handleSemanticSearch(context.Background(), req)
	if !result.IsError {
		t.Error("expected tool error for missing project")
	}
}

func TestHandleSemanticSearch_projectNotFound(t *testing.T) {
	store := &mockSearcher{projectErr: errors.New("not found")}
	h := NewHandler(store, &mockEmbedder{vec: []float32{0.1}})
	req := makeRequest(map[string]interface{}{"project": "missing", "query": "q"})
	result, _ := h.handleSemanticSearch(context.Background(), req)
	if !result.IsError {
		t.Error("expected tool error for project not found")
	}
}

func TestHandleSemanticSearch_embedError(t *testing.T) {
	store := &mockSearcher{project: &db.Project{ID: 1}}
	embedder := &mockEmbedder{err: errors.New("ollama down")}
	h := NewHandler(store, embedder)
	req := makeRequest(map[string]interface{}{"project": "p", "query": "q"})
	result, _ := h.handleSemanticSearch(context.Background(), req)
	if !result.IsError {
		t.Error("expected tool error for embed failure")
	}
}

func TestHandleKeywordSearch_success(t *testing.T) {
	store := &mockSearcher{
		project: &db.Project{ID: 1},
		results: []db.SearchResult{{FilePath: "world.md", Score: 0.8}},
	}
	h := NewHandler(store, &mockEmbedder{})
	req := makeRequest(map[string]interface{}{"project": "p", "query": "Roland"})
	result, _ := h.handleKeywordSearch(context.Background(), req)
	if result.IsError {
		t.Errorf("unexpected error: %v", result.Content)
	}
}

func TestHandleHybridSearch_success(t *testing.T) {
	store := &mockSearcher{
		project: &db.Project{ID: 1},
		results: []db.SearchResult{{FilePath: "ch2.md", Score: 0.7}},
	}
	h := NewHandler(store, &mockEmbedder{vec: []float32{0.5}})
	req := makeRequest(map[string]interface{}{"project": "p", "query": "the dark tower"})
	result, _ := h.handleHybridSearch(context.Background(), req)
	if result.IsError {
		t.Errorf("unexpected error: %v", result.Content)
	}
}

func TestHandleListProjects_success(t *testing.T) {
	store := &mockSearcher{
		projects: []db.ProjectSummary{
			{Slug: "novel-a", Name: "Novel A", DocCount: 42},
		},
	}
	h := NewHandler(store, &mockEmbedder{})
	result, _ := h.handleListProjects(context.Background(), makeRequest(nil))
	if result.IsError {
		t.Errorf("unexpected error: %v", result.Content)
	}
	text := result.Content[0].(mcpgo.TextContent).Text
	var summaries []db.ProjectSummary
	if err := json.Unmarshal([]byte(text), &summaries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(summaries) != 1 || summaries[0].Slug != "novel-a" {
		t.Errorf("unexpected summaries: %v", summaries)
	}
}

func TestHandleGetDocument_notFound(t *testing.T) {
	store := &mockSearcher{project: &db.Project{ID: 1}, results: nil}
	h := NewHandler(store, &mockEmbedder{})
	req := makeRequest(map[string]interface{}{"project": "p", "file_path": "missing.md"})
	result, _ := h.handleGetDocument(context.Background(), req)
	if !result.IsError {
		t.Error("expected error for missing document")
	}
}

func TestClampLimit(t *testing.T) {
	cases := []struct{ in, min, max, want int }{
		{0, 1, 20, 1},
		{5, 1, 20, 5},
		{25, 1, 20, 20},
		{-1, 1, 20, 1},
	}
	for _, c := range cases {
		got := clampLimit(c.in, c.min, c.max)
		if got != c.want {
			t.Errorf("clampLimit(%d,%d,%d)=%d, want %d", c.in, c.min, c.max, got, c.want)
		}
	}
}

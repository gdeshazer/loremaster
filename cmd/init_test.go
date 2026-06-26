package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdeshazer/loremaster/internal/config"
)

// TestInitWritesConfigFiles tests that loremaster.json and mcp.json are
// written correctly by the init command's file-writing logic (without
// requiring a real database connection).
func TestInitWritesConfigFiles(t *testing.T) {
	dir := t.TempDir()

	// Simulate what init.go does when writing files.
	cfg := &config.ProjectConfig{
		Project:        "test-story",
		EmbeddingModel: "nomic-embed-text",
	}
	if err := config.Write(dir, cfg); err != nil {
		t.Fatalf("Write loremaster.json: %v", err)
	}

	mcpConfig := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"loremaster": map[string]interface{}{
				"command": "/usr/local/bin/loremaster",
				"args":    []string{"serve"},
			},
		},
	}
	mcpData, err := json.MarshalIndent(mcpConfig, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	mcpPath := filepath.Join(dir, "mcp.json")
	if err := os.WriteFile(mcpPath, append(mcpData, '\n'), 0644); err != nil {
		t.Fatalf("Write mcp.json: %v", err)
	}

	// Verify loremaster.json round-trips correctly.
	loaded, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load loremaster.json: %v", err)
	}
	if loaded.Project != "test-story" {
		t.Errorf("project: got %q, want %q", loaded.Project, "test-story")
	}
	if loaded.EmbeddingModel != "nomic-embed-text" {
		t.Errorf("embedding_model: got %q", loaded.EmbeddingModel)
	}

	// Verify mcp.json is valid JSON with expected structure.
	mcpRaw, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("Read mcp.json: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(mcpRaw, &parsed); err != nil {
		t.Fatalf("mcp.json is not valid JSON: %v", err)
	}
	servers, ok := parsed["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatal("mcp.json missing mcpServers key")
	}
	lm, ok := servers["loremaster"].(map[string]interface{})
	if !ok {
		t.Fatal("mcp.json missing loremaster server config")
	}
	if _, ok := lm["command"]; !ok {
		t.Error("mcp.json missing command field")
	}
}

func TestWriteCLAUDEMD_createsFile(t *testing.T) {
	dir := t.TempDir()
	if err := writeCLAUDEMD(dir, "my-novel", "My Novel"); err != nil {
		t.Fatalf("writeCLAUDEMD: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, claudeMDMarker) {
		t.Error("CLAUDE.md missing loremaster marker")
	}
	if !strings.Contains(content, "my-novel") {
		t.Error("CLAUDE.md missing project slug")
	}
	if !strings.Contains(content, "hybrid_search") {
		t.Error("CLAUDE.md missing hybrid_search tool entry")
	}
	if !strings.Contains(content, "semantic_search") {
		t.Error("CLAUDE.md missing semantic_search tool entry")
	}
	if !strings.Contains(content, "keyword_search") {
		t.Error("CLAUDE.md missing keyword_search tool entry")
	}
}

func TestWriteCLAUDEMD_appendsToExisting(t *testing.T) {
	dir := t.TempDir()
	existing := "# My Story\n\nThis is my project.\n"
	claudePath := filepath.Join(dir, "CLAUDE.md")
	os.WriteFile(claudePath, []byte(existing), 0644)

	if err := writeCLAUDEMD(dir, "my-novel", "My Novel"); err != nil {
		t.Fatalf("writeCLAUDEMD: %v", err)
	}

	data, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	content := string(data)

	if !strings.HasPrefix(content, existing) {
		t.Error("existing CLAUDE.md content was not preserved at the top")
	}
	if !strings.Contains(content, claudeMDMarker) {
		t.Error("loremaster section was not appended")
	}
}

func TestWriteCLAUDEMD_idempotent(t *testing.T) {
	dir := t.TempDir()

	// Run twice — should not duplicate the section.
	if err := writeCLAUDEMD(dir, "my-novel", "My Novel"); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := writeCLAUDEMD(dir, "my-novel", "My Novel"); err != nil {
		t.Fatalf("second write: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	count := strings.Count(string(data), claudeMDMarker)
	if count != 1 {
		t.Errorf("expected marker to appear exactly once, got %d", count)
	}
}

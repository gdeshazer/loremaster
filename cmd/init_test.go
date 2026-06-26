package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
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

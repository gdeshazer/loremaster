package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_findsFileInCurrentDir(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, dir, `{"project":"test-slug","embedding_model":"mxbai-embed-large"}`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Project != "test-slug" {
		t.Errorf("project: got %q, want %q", cfg.Project, "test-slug")
	}
	if cfg.EmbeddingModel != "mxbai-embed-large" {
		t.Errorf("embedding_model: got %q, want %q", cfg.EmbeddingModel, "mxbai-embed-large")
	}
	if cfg.Dir != dir {
		t.Errorf("Dir: got %q, want %q", cfg.Dir, dir)
	}
}

func TestLoad_walksUpToParent(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "sub", "dir")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, parent, `{"project":"parent-slug"}`)

	cfg, err := Load(child)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Project != "parent-slug" {
		t.Errorf("project: got %q, want %q", cfg.Project, "parent-slug")
	}
	if cfg.Dir != parent {
		t.Errorf("Dir: got %q, want %q", cfg.Dir, parent)
	}
}

func TestLoad_stopAtNearestFile(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "inner")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, parent, `{"project":"parent-slug"}`)
	writeJSON(t, child, `{"project":"child-slug"}`)

	cfg, err := Load(child)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Project != "child-slug" {
		t.Errorf("expected nearest file to win; got project %q", cfg.Project)
	}
}

func TestLoad_returnsErrNotFound(t *testing.T) {
	dir := t.TempDir() // empty, no loremaster.json

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	}
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestLoad_invalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "loremaster.json")
	if err := os.WriteFile(path, []byte("{bad json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	var pe *ParseError
	if !asParseError(err, &pe) {
		t.Errorf("expected *ParseError, got %T: %v", err, err)
	}
}

func TestLoad_excludeGlobs(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, dir, `{"project":"p","exclude":["drafts/**","*.bak.md"]}`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Exclude) != 2 {
		t.Errorf("exclude len: got %d, want 2", len(cfg.Exclude))
	}
}

func TestWrite_roundtrip(t *testing.T) {
	dir := t.TempDir()
	original := &ProjectConfig{
		Project:        "my-novel",
		EmbeddingModel: "nomic-embed-text",
		Exclude:        []string{"drafts/**"},
	}
	if err := Write(dir, original); err != nil {
		t.Fatalf("Write error: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if loaded.Project != original.Project {
		t.Errorf("project mismatch: got %q, want %q", loaded.Project, original.Project)
	}
	if loaded.EmbeddingModel != original.EmbeddingModel {
		t.Errorf("model mismatch: got %q", loaded.EmbeddingModel)
	}
	if len(loaded.Exclude) != 1 || loaded.Exclude[0] != "drafts/**" {
		t.Errorf("exclude mismatch: got %v", loaded.Exclude)
	}
}

// helpers

func writeJSON(t *testing.T, dir, content string) {
	t.Helper()
	path := filepath.Join(dir, "loremaster.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func asParseError(err error, target **ParseError) bool {
	pe, ok := err.(*ParseError)
	if ok {
		*target = pe
	}
	return ok
}

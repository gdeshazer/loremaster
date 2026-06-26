package ingest

import (
	"strings"
	"testing"
)

func TestParse_plainMarkdown(t *testing.T) {
	src := []byte("# The Dark Tower\n\nRoland walked east.\n")
	doc, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if doc.Title == "" {
		t.Error("expected a non-empty title")
	}
	if !strings.Contains(doc.Plaintext, "Roland walked east") {
		t.Errorf("plaintext missing content: %q", doc.Plaintext)
	}
}

func TestParse_frontmatterTitle(t *testing.T) {
	src := []byte("---\ntitle: Chapter One\ncharacters: Roland, Jake\n---\n\nSome content here.\n")
	doc, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if doc.Title != "Chapter One" {
		t.Errorf("title: got %q, want %q", doc.Title, "Chapter One")
	}
	if doc.Metadata["characters"] != "Roland, Jake" {
		t.Errorf("characters: got %q", doc.Metadata["characters"])
	}
}

func TestParse_frontmatterTagsSlice(t *testing.T) {
	src := []byte("---\ntags:\n  - magic\n  - conflict\n---\n\nBody.\n")
	doc, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	tags := doc.Metadata["tags"]
	if !strings.Contains(tags, "magic") || !strings.Contains(tags, "conflict") {
		t.Errorf("tags: got %q, want comma-separated list", tags)
	}
}

func TestParse_noFrontmatterUsesH1(t *testing.T) {
	src := []byte("# My Story Chapter\n\nContent here.\n")
	doc, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	// Title should come from H1 text extracted from plaintext.
	if doc.Title == "" {
		t.Error("expected title from H1, got empty")
	}
}

func TestParse_stripsMarkdownFormatting(t *testing.T) {
	src := []byte("**Bold** and _italic_ text.\n")
	doc, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if strings.Contains(doc.Plaintext, "**") || strings.Contains(doc.Plaintext, "_") {
		t.Errorf("plaintext still contains markdown syntax: %q", doc.Plaintext)
	}
	if !strings.Contains(doc.Plaintext, "Bold") || !strings.Contains(doc.Plaintext, "italic") {
		t.Errorf("plaintext missing words: %q", doc.Plaintext)
	}
}

func TestParse_emptyFile(t *testing.T) {
	doc, err := Parse([]byte{})
	if err != nil {
		t.Fatalf("Parse error on empty input: %v", err)
	}
	if doc.Plaintext != "" {
		t.Errorf("expected empty plaintext, got %q", doc.Plaintext)
	}
}

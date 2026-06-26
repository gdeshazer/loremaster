package ingest

import (
	"strings"
	"testing"
)

func TestParse_plainMarkdown(t *testing.T) {
	src := []byte("# The Dark Tower\n\nRoland walked east.\n")
	doc, err := Parse(src, "")
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
	doc, err := Parse(src, "")
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
	doc, err := Parse(src, "")
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
	doc, err := Parse(src, "")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if doc.Title != "My Story Chapter" {
		t.Errorf("title: got %q, want %q", doc.Title, "My Story Chapter")
	}
}

func TestParse_noFrontmatterNoH1FallsBackToFilename(t *testing.T) {
	src := []byte("Just some prose without a heading.\n")
	doc, err := Parse(src, "lore/magic/magic-system.md")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if doc.Title != "magic system" {
		t.Errorf("title: got %q, want %q", doc.Title, "magic system")
	}
}

func TestParse_tagsFromPath(t *testing.T) {
	src := []byte("# Runic Magic\n\nsome example magic system\n")
	doc, err := Parse(src, "lore/magic/magic-system.md")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if doc.Title != "Runic Magic" {
		t.Errorf("title: got %q, want %q", doc.Title, "Runic Magic")
	}
	tags := doc.Metadata["tags"]
	if !strings.Contains(tags, "lore") || !strings.Contains(tags, "magic") {
		t.Errorf("tags: got %q, want lore and magic", tags)
	}
}

func TestParse_frontmatterTagsNotOverriddenByPath(t *testing.T) {
	src := []byte("---\ntags:\n  - worldbuilding\n---\n\n# Runic Magic\n\nContent.\n")
	doc, err := Parse(src, "lore/magic/magic-system.md")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	// Frontmatter tags should win; path-derived tags should not be added.
	if doc.Metadata["tags"] != "worldbuilding" {
		t.Errorf("tags: got %q, want %q", doc.Metadata["tags"], "worldbuilding")
	}
}

func TestParse_stripsMarkdownFormatting(t *testing.T) {
	src := []byte("**Bold** and _italic_ text.\n")
	doc, err := Parse(src, "")
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
	doc, err := Parse([]byte{}, "")
	if err != nil {
		t.Fatalf("Parse error on empty input: %v", err)
	}
	if doc.Plaintext != "" {
		t.Errorf("expected empty plaintext, got %q", doc.Plaintext)
	}
}

func TestTitleFromFilename(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"lore/magic/magic-system.md", "magic system"},
		{"chapters/chapter_one.md", "chapter one"},
		{"README.md", "README"},
		{"file.md", "file"},
	}
	for _, c := range cases {
		got := titleFromFilename(c.path)
		if got != c.want {
			t.Errorf("titleFromFilename(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

func TestTagsFromPath(t *testing.T) {
	cases := []struct {
		path string
		want []string
	}{
		{"lore/magic/magic-system.md", []string{"lore", "magic"}},
		{"file.md", nil},
		{"chapters/one.md", []string{"chapters"}},
	}
	for _, c := range cases {
		got := tagsFromPath(c.path)
		if len(got) != len(c.want) {
			t.Errorf("tagsFromPath(%q) = %v, want %v", c.path, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("tagsFromPath(%q)[%d] = %q, want %q", c.path, i, got[i], c.want[i])
			}
		}
	}
}

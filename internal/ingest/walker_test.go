package ingest

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestWalk_findsMarkdownFiles(t *testing.T) {
	root := t.TempDir()
	createFile(t, root, "a.md")
	createFile(t, root, "subdir/b.md")
	createFile(t, root, "subdir/c.txt") // not markdown — should be excluded
	createFile(t, root, "subdir/d.MD")  // case-insensitive extension match

	paths, err := Walk(root, nil)
	if err != nil {
		t.Fatalf("Walk error: %v", err)
	}
	sort.Strings(paths)

	want := []string{
		filepath.Join(root, "a.md"),
		filepath.Join(root, "subdir/b.md"),
		filepath.Join(root, "subdir/d.MD"),
	}
	sort.Strings(want)

	if len(paths) != len(want) {
		t.Fatalf("got %d paths %v, want %d", len(paths), paths, len(want))
	}
	for i, p := range paths {
		if p != want[i] {
			t.Errorf("paths[%d] = %q, want %q", i, p, want[i])
		}
	}
}

func TestWalk_excludeByBasename(t *testing.T) {
	root := t.TempDir()
	createFile(t, root, "notes.md")
	createFile(t, root, "notes.bak.md")
	createFile(t, root, "subdir/also.bak.md")

	paths, err := Walk(root, []string{"*.bak.md"})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Errorf("expected 1 path, got %d: %v", len(paths), paths)
	}
}

func TestWalk_excludeByDirGlob(t *testing.T) {
	root := t.TempDir()
	createFile(t, root, "keep.md")
	createFile(t, root, "drafts/old/one.md")
	createFile(t, root, "drafts/old/two.md")

	paths, err := Walk(root, []string{"drafts/**"})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Errorf("expected 1 path, got %d: %v", len(paths), paths)
	}
}

func TestWalk_excludeExactRelPath(t *testing.T) {
	root := t.TempDir()
	createFile(t, root, "keep.md")
	createFile(t, root, "skip.md")

	paths, err := Walk(root, []string{"skip.md"})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Errorf("expected 1 path, got %d: %v", len(paths), paths)
	}
}

func TestWalk_emptyDir(t *testing.T) {
	root := t.TempDir()
	paths, err := Walk(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 0 {
		t.Errorf("expected 0 paths, got %v", paths)
	}
}

func createFile(t *testing.T, root string, rel string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("# test\n"), 0644); err != nil {
		t.Fatal(err)
	}
}

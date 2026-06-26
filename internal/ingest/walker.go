package ingest

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// Walk returns all .md file paths under root, excluding any paths that match
// one of the glob patterns in excludes. Patterns are matched against the path
// relative to root using filepath.Match semantics.
func Walk(root string, excludes []string) ([]string, error) {
	var paths []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		if matchesAny(rel, excludes) {
			return nil
		}

		paths = append(paths, path)
		return nil
	})
	return paths, err
}

// matchesAny reports whether rel matches any of the glob patterns.
func matchesAny(rel string, patterns []string) bool {
	for _, pattern := range patterns {
		// Try matching against the full relative path.
		if ok, _ := filepath.Match(pattern, rel); ok {
			return true
		}
		// Also try matching the basename alone (e.g. "*.bak.md" matches "subdir/file.bak.md").
		if ok, _ := filepath.Match(pattern, filepath.Base(rel)); ok {
			return true
		}
		// Support simple dir/** patterns: check if rel is under the prefix dir/.
		if strings.HasSuffix(pattern, "/**") {
			prefix := strings.TrimSuffix(pattern, "/**")
			if strings.HasPrefix(rel, prefix+string(filepath.Separator)) {
				return true
			}
		}
	}
	return false
}

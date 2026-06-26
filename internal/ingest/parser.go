package ingest

import (
	"bytes"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

// ParsedDoc holds the extracted content and metadata from a markdown file.
type ParsedDoc struct {
	// Title is taken from the frontmatter "title" key, or the first H1 heading.
	Title string
	// Plaintext is the rendered text content with all markdown formatting stripped.
	Plaintext string
	// Metadata holds recognized frontmatter keys.
	Metadata map[string]string
}

var md = goldmark.New(
	goldmark.WithExtensions(
		meta.Meta,
	),
	goldmark.WithRendererOptions(
		html.WithHardWraps(),
	),
)

// Parse converts a markdown file's content into a ParsedDoc.
// filePath is optional — pass an empty string when no path is available.
// When provided, it is used as a fallback title source (the filename stem) and
// to derive tags from directory components when the frontmatter has none.
func Parse(src []byte, filePath string) (*ParsedDoc, error) {
	ctx := parser.NewContext()
	var buf bytes.Buffer
	if err := md.Convert(src, &buf, parser.WithContext(ctx)); err != nil {
		return nil, err
	}

	frontmatter := meta.Get(ctx)
	meta_ := extractMetadata(frontmatter)

	plaintext := stripHTML(buf.String())

	title := meta_["title"]
	if title == "" {
		title = extractH1FromSource(src)
	}
	if title == "" && filePath != "" {
		title = titleFromFilename(filePath)
	}

	if meta_["tags"] == "" && filePath != "" {
		if derived := tagsFromPath(filePath); len(derived) > 0 {
			meta_["tags"] = strings.Join(derived, ", ")
		}
	}

	return &ParsedDoc{
		Title:     title,
		Plaintext: plaintext,
		Metadata:  meta_,
	}, nil
}

// recognizedKeys are the frontmatter fields carried into the metadata JSONB column.
var recognizedKeys = []string{"title", "tags", "characters", "location", "date"}

func extractMetadata(fm map[string]interface{}) map[string]string {
	out := make(map[string]string, len(recognizedKeys))
	for _, key := range recognizedKeys {
		v, ok := fm[key]
		if !ok {
			continue
		}
		switch val := v.(type) {
		case string:
			out[key] = val
		case []interface{}:
			// Slices (e.g. tags: [a, b]) are joined as comma-separated.
			parts := make([]string, 0, len(val))
			for _, item := range val {
				if s, ok := item.(string); ok {
					parts = append(parts, s)
				}
			}
			out[key] = strings.Join(parts, ", ")
		default:
			// Fallback: fmt would import fmt — use Sprintf via type assertion.
			if s, ok := v.(interface{ String() string }); ok {
				out[key] = s.String()
			}
		}
	}
	return out
}

// stripHTML removes HTML tags from rendered markdown output.
func stripHTML(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	// Collapse runs of whitespace into single spaces and trim.
	return strings.Join(strings.Fields(b.String()), " ")
}

// extractH1FromSource scans raw markdown bytes for the first ATX H1 heading (`# ...`)
// and returns its text content.
func extractH1FromSource(src []byte) string {
	for _, line := range strings.SplitN(string(src), "\n", 50) {
		trimmed := strings.TrimRight(line, "\r")
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(trimmed[2:])
		}
	}
	return ""
}

// titleFromFilename derives a human-readable title from a file path by taking
// the base name, stripping the extension, and replacing hyphens/underscores with spaces.
func titleFromFilename(filePath string) string {
	base := filepath.Base(filePath)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	return strings.ReplaceAll(strings.ReplaceAll(stem, "-", " "), "_", " ")
}

// tagsFromPath returns the directory components of filePath as tag strings,
// excluding the filename itself. Empty or dot segments are omitted.
func tagsFromPath(filePath string) []string {
	dir := filepath.Dir(filePath)
	if dir == "." || dir == "" {
		return nil
	}
	parts := strings.Split(filepath.ToSlash(dir), "/")
	tags := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" && p != "." {
			tags = append(tags, p)
		}
	}
	return tags
}

// needed to avoid "declared and not used" — goldmark/text is used by the parser
var _ = text.NewReader

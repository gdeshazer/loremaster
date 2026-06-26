package ingest

import (
	"bytes"
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
func Parse(src []byte) (*ParsedDoc, error) {
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
		title = extractFirstHeading(plaintext)
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

// extractFirstHeading returns the text of the first line that looks like a heading.
func extractFirstHeading(plaintext string) string {
	for _, line := range strings.SplitN(plaintext, "\n", 20) {
		line = strings.TrimSpace(line)
		if line != "" {
			// Return the first non-empty line as the implicit title.
			return line
		}
	}
	return ""
}

// needed to avoid "declared and not used" — goldmark/text is used by the parser
var _ = text.NewReader

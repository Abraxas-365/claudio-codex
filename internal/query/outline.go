package query

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Abraxas-365/claudio-codex/internal/index"
)

// OutlineEntry represents a symbol in a file outline.
type OutlineEntry struct {
	Name      string
	Kind      string
	Line      int
	EndLine   int
	Signature string
	Parent    string
	Exported  bool
}

// Outline returns all symbols in a given file.
func Outline(store *index.Store, filePath string) ([]OutlineEntry, error) {
	// Try both absolute and relative path matching
	absPath := filePath
	if !filepath.IsAbs(filePath) {
		absPath = filepath.Join(store.RepoDir(), filePath)
	}

	rows, err := store.DB().Query(`
		SELECT s.name, s.kind, s.line, s.end_line, COALESCE(s.signature, ''), COALESCE(s.parent, ''), s.exported
		FROM symbols s
		JOIN files f ON f.id = s.file_id
		WHERE f.path = ? OR f.path = ?
		ORDER BY s.line
	`, absPath, filePath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []OutlineEntry
	for rows.Next() {
		var e OutlineEntry
		if err := rows.Scan(&e.Name, &e.Kind, &e.Line, &e.EndLine, &e.Signature, &e.Parent, &e.Exported); err != nil {
			return nil, err
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

// FormatOutline formats outline results for display.
func FormatOutline(results []OutlineEntry, filePath string) string {
	if len(results) == 0 {
		return fmt.Sprintf("No symbols found in '%s'.", filePath)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Outline of %s (%d symbols):\n", filePath, len(results))
	for _, e := range results {
		vis := ""
		if !e.Exported {
			vis = " (unexported)"
		}
		parent := ""
		if e.Parent != "" {
			parent = fmt.Sprintf(" [%s]", e.Parent)
		}
		sig := e.Name
		if e.Signature != "" {
			sig = e.Signature
		}
		fmt.Fprintf(&sb, "  %4d  %-10s %s%s%s\n", e.Line, e.Kind, sig, parent, vis)
	}
	return sb.String()
}

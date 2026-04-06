package query

import (
	"fmt"
	"strings"

	"github.com/Abraxas-365/claudio-codex/internal/index"
)

// StructureEntry represents a high-level structural element.
type StructureEntry struct {
	File       string
	Language   string
	Symbols    int
	Functions  int
	Types      int
	Refs       int
}

// Hotspot represents a frequently-called symbol.
type Hotspot struct {
	Name  string
	Kind  string
	File  string
	Line  int
	Calls int
}

// Structure returns a high-level overview of the indexed codebase.
func Structure(store *index.Store) ([]StructureEntry, error) {
	rows, err := store.DB().Query(`
		SELECT f.path, f.language,
			(SELECT COUNT(*) FROM symbols s WHERE s.file_id = f.id) as sym_count,
			(SELECT COUNT(*) FROM symbols s WHERE s.file_id = f.id AND s.kind IN ('function', 'method')) as func_count,
			(SELECT COUNT(*) FROM symbols s WHERE s.file_id = f.id AND s.kind IN ('struct', 'class', 'interface', 'type', 'enum', 'trait')) as type_count,
			(SELECT COUNT(*) FROM refs r WHERE r.file_id = f.id) as ref_count
		FROM files f
		ORDER BY sym_count DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []StructureEntry
	for rows.Next() {
		var e StructureEntry
		if err := rows.Scan(&e.File, &e.Language, &e.Symbols, &e.Functions, &e.Types, &e.Refs); err != nil {
			return nil, err
		}
		e.File = store.RelPath(e.File)
		results = append(results, e)
	}
	return results, rows.Err()
}

// Hotspots returns the most-referenced symbols in the codebase.
func Hotspots(store *index.Store, limit int) ([]Hotspot, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := store.DB().Query(`
		SELECT r.target, COALESCE(s.kind, 'unknown'), COALESCE(f2.path, ''), COALESCE(s.line, 0), COUNT(*) as call_count
		FROM refs r
		LEFT JOIN symbols s ON s.name = r.target
		LEFT JOIN files f2 ON f2.id = s.file_id
		GROUP BY r.target
		ORDER BY call_count DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Hotspot
	for rows.Next() {
		var h Hotspot
		if err := rows.Scan(&h.Name, &h.Kind, &h.File, &h.Line, &h.Calls); err != nil {
			return nil, err
		}
		if h.File != "" {
			h.File = store.RelPath(h.File)
		}
		results = append(results, h)
	}
	return results, rows.Err()
}

// FormatStructure formats structure results for display.
func FormatStructure(results []StructureEntry) string {
	if len(results) == 0 {
		return "No files indexed."
	}

	// Summarize by language
	langStats := make(map[string]struct{ files, symbols, funcs, types int })
	for _, e := range results {
		s := langStats[e.Language]
		s.files++
		s.symbols += e.Symbols
		s.funcs += e.Functions
		s.types += e.Types
		langStats[e.Language] = s
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Codebase structure (%d files):\n\n", len(results))

	fmt.Fprintf(&sb, "%-12s %6s %8s %8s %8s\n", "Language", "Files", "Symbols", "Funcs", "Types")
	fmt.Fprintf(&sb, "%s\n", strings.Repeat("-", 50))
	for lang, s := range langStats {
		fmt.Fprintf(&sb, "%-12s %6d %8d %8d %8d\n", lang, s.files, s.symbols, s.funcs, s.types)
	}

	// Top 10 largest files
	sb.WriteString("\nTop files by symbol count:\n")
	limit := 10
	if len(results) < limit {
		limit = len(results)
	}
	for _, e := range results[:limit] {
		fmt.Fprintf(&sb, "  %4d symbols  %s\n", e.Symbols, e.File)
	}

	return sb.String()
}

// FormatHotspots formats hotspot results for display.
func FormatHotspots(results []Hotspot) string {
	if len(results) == 0 {
		return "No hotspots found."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Most-called symbols (%d):\n", len(results))
	for _, h := range results {
		loc := ""
		if h.File != "" {
			loc = fmt.Sprintf("  %s:%d", h.File, h.Line)
		}
		fmt.Fprintf(&sb, "  %4d calls  %-10s %s%s\n", h.Calls, h.Kind, h.Name, loc)
	}
	return sb.String()
}

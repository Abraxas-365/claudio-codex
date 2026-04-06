package query

import (
	"fmt"
	"strings"

	"github.com/Abraxas-365/claudio-codex/internal/index"
)

// SearchResult represents a symbol found by search.
type SearchResult struct {
	Name      string
	Kind      string
	File      string
	Line      int
	Signature string
	Parent    string
	Exported  bool
}

// Search performs a full-text search on symbol names.
func Search(store *index.Store, query string) ([]SearchResult, error) {
	// Try FTS first, fall back to LIKE
	results, err := searchFTS(store, query)
	if err != nil || len(results) == 0 {
		return searchLike(store, query)
	}
	return results, nil
}

func searchFTS(store *index.Store, query string) ([]SearchResult, error) {
	rows, err := store.DB().Query(`
		SELECT s.name, s.kind, f.path, s.line, COALESCE(s.signature, ''), COALESCE(s.parent, ''), s.exported
		FROM symbols_fts fts
		JOIN symbols s ON s.id = fts.rowid
		JOIN files f ON f.id = s.file_id
		WHERE symbols_fts MATCH ?
		ORDER BY rank
		LIMIT 50
	`, query+"*")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSearchResults(store, rows)
}

func searchLike(store *index.Store, query string) ([]SearchResult, error) {
	pattern := "%" + strings.ToLower(query) + "%"
	rows, err := store.DB().Query(`
		SELECT s.name, s.kind, f.path, s.line, COALESCE(s.signature, ''), COALESCE(s.parent, ''), s.exported
		FROM symbols s
		JOIN files f ON f.id = s.file_id
		WHERE LOWER(s.name) LIKE ?
		ORDER BY s.name
		LIMIT 50
	`, pattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSearchResults(store, rows)
}

func scanSearchResults(store *index.Store, rows interface{ Scan(...any) error; Next() bool; Err() error }) ([]SearchResult, error) {
	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.Name, &r.Kind, &r.File, &r.Line, &r.Signature, &r.Parent, &r.Exported); err != nil {
			return nil, err
		}
		r.File = store.RelPath(r.File)
		results = append(results, r)
	}
	return results, rows.Err()
}

// FormatSearchResults formats search results for display.
func FormatSearchResults(results []SearchResult) string {
	if len(results) == 0 {
		return "No symbols found."
	}
	var sb strings.Builder
	for _, r := range results {
		vis := ""
		if !r.Exported {
			vis = " (unexported)"
		}
		parent := ""
		if r.Parent != "" {
			parent = fmt.Sprintf(" [%s]", r.Parent)
		}
		sig := r.Name
		if r.Signature != "" {
			sig = r.Signature
		}
		fmt.Fprintf(&sb, "%-10s %s%s%s  %s:%d\n", r.Kind, sig, parent, vis, r.File, r.Line)
	}
	return sb.String()
}

package query

import (
	"fmt"
	"strings"

	"github.com/Abraxas-365/claudio-codex/internal/index"
)

// RefResult represents a reference to a symbol.
type RefResult struct {
	Caller string
	File   string
	Line   int
}

// Refs finds all call sites that reference the given symbol.
func Refs(store *index.Store, symbol string) ([]RefResult, error) {
	// Exact match on target
	rows, err := store.DB().Query(`
		SELECT r.caller, f.path, r.line
		FROM refs r
		JOIN files f ON f.id = r.file_id
		WHERE r.target = ? OR r.target LIKE ?
		ORDER BY f.path, r.line
	`, symbol, "%."+symbol)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []RefResult
	for rows.Next() {
		var r RefResult
		if err := rows.Scan(&r.Caller, &r.File, &r.Line); err != nil {
			return nil, err
		}
		r.File = store.RelPath(r.File)
		results = append(results, r)
	}
	return results, rows.Err()
}

// FormatRefs formats reference results for display.
func FormatRefs(results []RefResult, symbol string) string {
	if len(results) == 0 {
		return fmt.Sprintf("No references to '%s' found.", symbol)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "References to '%s' (%d):\n", symbol, len(results))
	for _, r := range results {
		caller := r.Caller
		if caller == "" {
			caller = "<top-level>"
		}
		fmt.Fprintf(&sb, "  %s:%d  called from %s\n", r.File, r.Line, caller)
	}
	return sb.String()
}

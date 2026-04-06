package query

import (
	"fmt"
	"os"
	"strings"

	"github.com/Abraxas-365/claudio-codex/internal/index"
)

// ContextResult bundles symbol definition, callers, callees, and source snippet.
type ContextResult struct {
	Symbol    SearchResult
	Source    string
	Callers   []RefResult
	Callees   []TraceNode
}

// Context returns a bundled view of a symbol: definition + source + callers + callees.
// symbolName can be a plain name like "EventHandler" or a file:line reference like "internal/query/engine.go:29".
func Context(store *index.Store, symbolName string) (*ContextResult, error) {
	var match *SearchResult

	// Check if input is file:line format
	if file, line, ok := parseFileLine(symbolName); ok {
		match = findSymbolAtLine(store, file, line)
	}

	// Fall back to name search
	if match == nil {
		results, err := Search(store, symbolName)
		if err != nil {
			return nil, err
		}

		for i, r := range results {
			if r.Name == symbolName {
				match = &results[i]
				break
			}
		}
		if match == nil && len(results) > 0 {
			match = &results[0]
		}
	}

	if match == nil {
		return nil, fmt.Errorf("symbol '%s' not found", symbolName)
	}

	// Use the matched symbol name for downstream queries
	symbolName = match.Name

	// Get source snippet
	source := extractSource(store, match.File, match.Line)

	// Get callers
	callers, err := Refs(store, symbolName)
	if err != nil {
		callers = nil
	}

	// Get callees (1 level)
	callees, err := Trace(store, symbolName, 1)
	if err != nil {
		callees = nil
	}

	return &ContextResult{
		Symbol:  *match,
		Source:  source,
		Callers: callers,
		Callees: callees,
	}, nil
}

// parseFileLine checks if s is in "file:line" format and returns the parts.
func parseFileLine(s string) (string, int, bool) {
	idx := strings.LastIndex(s, ":")
	if idx <= 0 || idx == len(s)-1 {
		return "", 0, false
	}
	file := s[:idx]
	var line int
	if _, err := fmt.Sscanf(s[idx+1:], "%d", &line); err != nil {
		return "", 0, false
	}
	return file, line, true
}

// findSymbolAtLine finds the most specific symbol at a given file:line.
func findSymbolAtLine(store *index.Store, file string, line int) *SearchResult {
	rows, err := store.DB().Query(`
		SELECT s.name, s.kind, f.path, s.line, COALESCE(s.signature, ''), COALESCE(s.parent, ''), s.exported
		FROM symbols s
		JOIN files f ON f.id = s.file_id
		WHERE f.path LIKE ? AND s.line <= ?
		ORDER BY s.line DESC
		LIMIT 1
	`, "%"+file, line)
	if err != nil {
		return nil
	}
	defer rows.Close()

	results, err := scanSearchResults(store, rows)
	if err != nil || len(results) == 0 {
		return nil
	}
	return &results[0]
}

func extractSource(store *index.Store, relPath string, startLine int) string {
	absPath := relPath
	if store.RepoDir() != "" {
		absPath = store.RepoDir() + "/" + relPath
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")

	// Show function context: from startLine to end of function (max 30 lines)
	start := startLine - 1
	if start < 0 {
		start = 0
	}
	end := start + 30
	if end > len(lines) {
		end = len(lines)
	}

	var sb strings.Builder
	for i := start; i < end; i++ {
		fmt.Fprintf(&sb, "%4d | %s\n", i+1, lines[i])
	}
	return sb.String()
}

// FormatContext formats context results for display.
func FormatContext(result *ContextResult) string {
	var sb strings.Builder

	// Symbol header
	parent := ""
	if result.Symbol.Parent != "" {
		parent = fmt.Sprintf(" [%s]", result.Symbol.Parent)
	}
	fmt.Fprintf(&sb, "=== %s %s%s ===\n", result.Symbol.Kind, result.Symbol.Name, parent)
	fmt.Fprintf(&sb, "File: %s:%d\n", result.Symbol.File, result.Symbol.Line)
	if result.Symbol.Signature != "" {
		fmt.Fprintf(&sb, "Sig:  %s\n", result.Symbol.Signature)
	}

	// Source
	if result.Source != "" {
		sb.WriteString("\nSource:\n")
		sb.WriteString(result.Source)
	}

	// Callers
	if len(result.Callers) > 0 {
		fmt.Fprintf(&sb, "\nCalled by (%d):\n", len(result.Callers))
		for _, c := range result.Callers {
			caller := c.Caller
			if caller == "" {
				caller = "<top-level>"
			}
			fmt.Fprintf(&sb, "  %s:%d  %s\n", c.File, c.Line, caller)
		}
	}

	// Callees
	if len(result.Callees) > 0 {
		fmt.Fprintf(&sb, "\nCalls (%d):\n", len(result.Callees))
		for _, c := range result.Callees {
			fmt.Fprintf(&sb, "  → %s  %s:%d\n", c.Symbol, c.File, c.Line)
		}
	}

	return sb.String()
}

package query

import (
	"fmt"
	"strings"

	"github.com/Abraxas-365/claudio-codex/internal/index"
)

// TraceNode represents a node in the call trace.
type TraceNode struct {
	Symbol string
	File   string
	Line   int
	Depth  int
}

// Trace follows what a function calls (downward), as opposed to Impact (upward).
func Trace(store *index.Store, symbol string, maxDepth int) ([]TraceNode, error) {
	if maxDepth <= 0 {
		maxDepth = 5
	}

	visited := make(map[string]bool)
	var results []TraceNode
	queue := []struct {
		caller string
		depth  int
	}{{symbol, 0}}

	visited[symbol] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.depth > maxDepth {
			continue
		}

		// Find all calls made by this function
		rows, err := store.DB().Query(`
			SELECT DISTINCT r.target, f.path, r.line
			FROM refs r
			JOIN files f ON f.id = r.file_id
			WHERE r.caller = ?
		`, current.caller)
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var target, filePath string
			var line int
			if err := rows.Scan(&target, &filePath, &line); err != nil {
				rows.Close()
				return nil, err
			}

			node := TraceNode{
				Symbol: target,
				File:   store.RelPath(filePath),
				Line:   line,
				Depth:  current.depth + 1,
			}
			results = append(results, node)

			// Only follow further if the target looks like a local function (no dots = not a package call)
			baseName := target
			if idx := strings.LastIndex(target, "."); idx >= 0 {
				baseName = target[idx+1:]
			}

			if !visited[baseName] && current.depth+1 < maxDepth {
				visited[baseName] = true
				queue = append(queue, struct {
					caller string
					depth  int
				}{baseName, current.depth + 1})
			}
		}
		rows.Close()
	}

	return results, nil
}

// FormatTrace formats trace results for display.
func FormatTrace(results []TraceNode, symbol string) string {
	if len(results) == 0 {
		return fmt.Sprintf("No outgoing calls from '%s' found.", symbol)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Call trace from '%s' (%d calls):\n", symbol, len(results))
	for _, n := range results {
		indent := strings.Repeat("  ", n.Depth)
		fmt.Fprintf(&sb, "%s→ %s  %s:%d\n", indent, n.Symbol, n.File, n.Line)
	}
	return sb.String()
}

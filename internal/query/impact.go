package query

import (
	"fmt"
	"strings"

	"github.com/Abraxas-365/claudio-codex/internal/index"
)

// ImpactNode represents a node in the impact graph.
type ImpactNode struct {
	Symbol string
	File   string
	Line   int
	Depth  int
}

// Impact computes the transitive impact of changing the given symbol.
// It follows the call chain upward: who calls this → who calls those → etc.
func Impact(store *index.Store, symbol string, maxDepth int) ([]ImpactNode, error) {
	if maxDepth <= 0 {
		maxDepth = 5
	}

	// Use iterative BFS instead of recursive CTE for more control
	visited := make(map[string]bool)
	var results []ImpactNode
	queue := []struct {
		target string
		depth  int
	}{{symbol, 0}}

	visited[symbol] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.depth > maxDepth {
			continue
		}

		rows, err := store.DB().Query(`
			SELECT DISTINCT r.caller, f.path, r.line
			FROM refs r
			JOIN files f ON f.id = r.file_id
			WHERE r.target = ? OR r.target LIKE ?
		`, current.target, "%."+current.target)
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var caller, filePath string
			var line int
			if err := rows.Scan(&caller, &filePath, &line); err != nil {
				rows.Close()
				return nil, err
			}
			if caller == "" {
				continue
			}

			node := ImpactNode{
				Symbol: caller,
				File:   store.RelPath(filePath),
				Line:   line,
				Depth:  current.depth + 1,
			}
			results = append(results, node)

			if !visited[caller] && current.depth+1 < maxDepth {
				visited[caller] = true
				queue = append(queue, struct {
					target string
					depth  int
				}{caller, current.depth + 1})
			}
		}
		rows.Close()
	}

	return results, nil
}

// FormatImpact formats impact results for display.
func FormatImpact(results []ImpactNode, symbol string) string {
	if len(results) == 0 {
		return fmt.Sprintf("No transitive callers of '%s' found.", symbol)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Impact analysis for '%s' (%d affected):\n", symbol, len(results))
	for _, n := range results {
		indent := strings.Repeat("  ", n.Depth)
		fmt.Fprintf(&sb, "%s%s  %s:%d\n", indent, n.Symbol, n.File, n.Line)
	}
	return sb.String()
}

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Abraxas-365/claudio-codex/internal/index"
	"github.com/Abraxas-365/claudio-codex/internal/query"
)

var Version = "dev"

const instructions = `This plugin provides a pre-built code index for the current project. It answers structural questions about the codebase in ~50 tokens instead of thousands.

ALWAYS prefer plugin_claudio-codex over Grep, Read, or Glob when the task involves:
- Finding where a symbol is defined or used → use "search" or "refs"
- Understanding what calls a function → use "refs" or "context"
- Analyzing impact of changing a symbol → use "impact"
- Getting a codebase overview → use "structure" or "hotspots"
- Listing symbols in a file → use "outline"
- Getting full context for a symbol (source + callers + callees) → use "context"

The "context" command accepts both symbol names ("EventHandler") and file:line references ("internal/query/engine.go:29").

Fall back to Grep/Read only when the index hasn't been built yet (you'll see "Run 'claudio-codex index' first") or when searching for string literals/comments that aren't symbols.`

// hooksSnippet is the recommended Claudio hooks configuration that keeps the
// code index continuously refreshed without manual `claudio-codex index` runs.
const hooksSnippet = `{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "*",
        "hooks": [
          {
            "id": "claudio-codex-session-refresh",
            "type": "command",
            "command": "claudio-codex index >/dev/null 2>&1 || true",
            "async": true,
            "description": "Refresh the claudio-codex index at session start"
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Write|Edit|NotebookEdit",
        "hooks": [
          {
            "id": "claudio-codex-post-edit",
            "type": "command",
            "command": "claudio-codex index >/dev/null 2>&1 || true",
            "async": true,
            "description": "Incrementally re-index after the agent edits a file"
          }
        ]
      }
    ],
    "CwdChanged": [
      {
        "matcher": "*",
        "hooks": [
          {
            "id": "claudio-codex-cwd-refresh",
            "type": "command",
            "command": "claudio-codex index >/dev/null 2>&1 || true",
            "async": true,
            "description": "Refresh the index when switching projects"
          }
        ]
      }
    ]
  }
}
`

const description = `Code index plugin for claudio. Provides fast symbol search, cross-references, call graphs, and impact analysis using tree-sitter parsing.

Commands:
  index [dir]           Build or refresh the code index (default: current dir)
  search <query>        Search for symbols by name
  refs <symbol>         Find all call sites referencing a symbol
  impact <symbol> [depth]  Show transitive callers (impact of changing a symbol)
  trace <symbol> [depth]   Show outgoing calls from a symbol
  outline <file>        List all symbols in a file
  context <symbol>      Bundled view: definition + source + callers + callees
  structure             High-level codebase overview
  hotspots [limit]      Most-referenced symbols`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: claudio-codex <command> [args...]")
		fmt.Fprintln(os.Stderr, description)
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "--describe":
		fmt.Print(description)
		return

	case "--schema":
		printSchema()
		return

	case "--instructions":
		fmt.Print(instructions)
		return

	case "--version":
		fmt.Println(Version)
		return

	case "--hooks", "hooks":
		fmt.Print(hooksSnippet)
		return

	case "install-hooks":
		if err := installHooks(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return

	case "index":
		dir := "."
		if len(os.Args) > 2 {
			dir = os.Args[2]
		}
		runIndex(dir)

	case "search":
		requireArgs(2, "search <query>")
		runQuery(func(store *index.Store) (string, error) {
			results, err := query.Search(store, os.Args[2])
			if err != nil {
				return "", err
			}
			return query.FormatSearchResults(results), nil
		})

	case "refs":
		requireArgs(2, "refs <symbol>")
		runQuery(func(store *index.Store) (string, error) {
			results, err := query.Refs(store, os.Args[2])
			if err != nil {
				return "", err
			}
			return query.FormatRefs(results, os.Args[2]), nil
		})

	case "impact":
		requireArgs(2, "impact <symbol> [depth]")
		depth := 5
		if len(os.Args) > 3 {
			if d, err := strconv.Atoi(os.Args[3]); err == nil {
				depth = d
			}
		}
		runQuery(func(store *index.Store) (string, error) {
			results, err := query.Impact(store, os.Args[2], depth)
			if err != nil {
				return "", err
			}
			return query.FormatImpact(results, os.Args[2]), nil
		})

	case "trace":
		requireArgs(2, "trace <symbol> [depth]")
		depth := 5
		if len(os.Args) > 3 {
			if d, err := strconv.Atoi(os.Args[3]); err == nil {
				depth = d
			}
		}
		runQuery(func(store *index.Store) (string, error) {
			results, err := query.Trace(store, os.Args[2], depth)
			if err != nil {
				return "", err
			}
			return query.FormatTrace(results, os.Args[2]), nil
		})

	case "outline":
		requireArgs(2, "outline <file>")
		runQuery(func(store *index.Store) (string, error) {
			results, err := query.Outline(store, os.Args[2])
			if err != nil {
				return "", err
			}
			return query.FormatOutline(results, os.Args[2]), nil
		})

	case "context":
		requireArgs(2, "context <symbol>")
		runQuery(func(store *index.Store) (string, error) {
			result, err := query.Context(store, os.Args[2])
			if err != nil {
				return "", err
			}
			return query.FormatContext(result), nil
		})

	case "structure":
		runQuery(func(store *index.Store) (string, error) {
			results, err := query.Structure(store)
			if err != nil {
				return "", err
			}
			return query.FormatStructure(results), nil
		})

	case "hotspots":
		limit := 20
		if len(os.Args) > 2 {
			if l, err := strconv.Atoi(os.Args[2]); err == nil {
				limit = l
			}
		}
		runQuery(func(store *index.Store) (string, error) {
			results, err := query.Hotspots(store, limit)
			if err != nil {
				return "", err
			}
			return query.FormatHotspots(results), nil
		})

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		os.Exit(1)
	}
}

func requireArgs(min int, usage string) {
	if len(os.Args) < min+1 {
		fmt.Fprintf(os.Stderr, "Usage: claudio-codex %s\n", usage)
		os.Exit(1)
	}
}

func findRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	// Walk up to find .git
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// No .git found, use cwd
			cwd, _ := os.Getwd()
			return cwd
		}
		dir = parent
	}
}

func openStore() (*index.Store, error) {
	repoDir := findRepoRoot()
	dbPath, err := index.DBPathForRepo(repoDir)
	if err != nil {
		return nil, err
	}
	return index.Open(dbPath, repoDir)
}

func runIndex(dir string) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	dbPath, err := index.DBPathForRepo(absDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	store, err := index.Open(dbPath, absDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening db: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	result, err := index.Index(store)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error indexing: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Indexed %d files (%d skipped, %d deleted) in %s\n",
		result.IndexedFiles, result.SkippedFiles, result.DeletedFiles, result.Duration)
}

func runQuery(fn func(store *index.Store) (string, error)) {
	store, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\nRun 'claudio-codex index' first.\n", err)
		os.Exit(1)
	}
	defer store.Close()

	// Auto-refresh before query
	index.Refresh(store)

	output, err := fn(store)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(output)
}

// installHooks merges the recommended hook entries into ~/.claudio/settings.json,
// creating the file if it doesn't exist. Existing user hooks are preserved;
// any prior claudio-codex hook entries (matched by id prefix) are replaced so
// re-running this command is idempotent.
func installHooks() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	settingsPath := filepath.Join(home, ".claudio", "settings.json")

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return err
	}

	var settings map[string]any
	data, err := os.ReadFile(settingsPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parsing %s: %w", settingsPath, err)
		}
	}
	if settings == nil {
		settings = map[string]any{}
	}

	var snippet map[string]any
	if err := json.Unmarshal([]byte(hooksSnippet), &snippet); err != nil {
		return err
	}
	snippetHooks, _ := snippet["hooks"].(map[string]any)

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}

	for event, snippetMatchers := range snippetHooks {
		existing, _ := hooks[event].([]any)
		// Drop any prior claudio-codex matchers (identified by id prefix).
		filtered := existing[:0:0]
		for _, m := range existing {
			matcher, _ := m.(map[string]any)
			if matcher == nil {
				filtered = append(filtered, m)
				continue
			}
			hs, _ := matcher["hooks"].([]any)
			keep := false
			for _, h := range hs {
				hh, _ := h.(map[string]any)
				if hh == nil {
					continue
				}
				id, _ := hh["id"].(string)
				if id == "" || !startsWith(id, "claudio-codex-") {
					keep = true
					break
				}
			}
			if keep {
				filtered = append(filtered, m)
			}
		}
		newMatchers, _ := snippetMatchers.([]any)
		hooks[event] = append(filtered, newMatchers...)
	}
	settings["hooks"] = hooks

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	if err := os.WriteFile(settingsPath, out, 0o644); err != nil {
		return err
	}
	fmt.Printf("Installed claudio-codex hooks into %s\n", settingsPath)
	return nil
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func printSchema() {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"enum":        []string{"index", "search", "refs", "impact", "trace", "outline", "context", "structure", "hotspots"},
				"description": "The command to execute",
			},
			"args": map[string]any{
				"type":        "string",
				"description": "Arguments for the command (symbol name, file path, etc.)",
			},
		},
		"required": []string{"command"},
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(schema)
}

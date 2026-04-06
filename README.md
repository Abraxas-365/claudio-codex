# claudio-codex

> **A pre-built code index plugin for [Claudio](https://github.com/Abraxas-365/claudio).**
> Answer structural questions about your codebase in ~50 tokens instead of thousands.

`claudio-codex` is a first-party plugin for Claudio that builds and maintains a structural index of your project using [tree-sitter](https://tree-sitter.github.io/). It exposes a small set of commands — `search`, `refs`, `context`, `impact`, `trace`, `outline`, `structure`, `hotspots` — that the AI can call instead of burning context on repeated `grep`, `read`, and `glob` sweeps.

---

## Why does this exist?

Coding agents waste enormous amounts of context on **navigation**: "where is this function defined?", "what calls this?", "what would break if I changed this?". Each of those questions usually translates into multiple `grep` runs, several full file reads, and a lot of irrelevant matches the model has to sift through.

A typical "find the definition of X and see who calls it" round-trip can easily consume **5,000–20,000 tokens**:

- multiple `grep` calls returning hundreds of lines of noise
- full file reads to disambiguate matches
- follow-up reads to gather caller context

`claudio-codex` reduces that to a single tool call returning **~50–200 tokens** of precisely the information the model needs:

- the exact file and line of the definition
- the surrounding source
- the list of callers and callees
- nothing else

The index is built once with `claudio-codex index` and auto-refreshed on subsequent queries, so it stays in sync with your edits without manual intervention.

---

## How it works

```
┌─────────────────────────────────────────────────────────┐
│  claudio (TUI / agent loop)                             │
│                                                         │
│   ┌────────────────────────────────────────────────┐    │
│   │  Deferred tool: plugin_claudio-codex           │    │
│   └─────────────────┬──────────────────────────────┘    │
└─────────────────────┼───────────────────────────────────┘
                      │ exec
                      ▼
       ┌──────────────────────────────┐
       │  ~/.claudio/plugins/         │
       │     claudio-codex            │
       │  (single Go binary)          │
       └──────┬───────────────────────┘
              │
              ▼
       ┌──────────────────────────────┐
       │  tree-sitter parsers         │
       │  (Go, Rust, Python, JS, TS,  │
       │   Java, Ruby, C, ...)        │
       └──────┬───────────────────────┘
              │ symbols, defs, refs
              ▼
       ┌──────────────────────────────┐
       │  SQLite index                │
       │  (per-repo, FTS5 enabled)    │
       │  ~/.claudio/codex/<repo>.db  │
       └──────────────────────────────┘
```

### 1. Indexing

`claudio-codex index` walks the repo, parses each supported source file with tree-sitter, and extracts:

- **Symbols** — functions, methods, types, classes, constants, variables
- **Definitions** — file, line, column, surrounding source range
- **References** — every call site / usage of every symbol
- **Containment** — which symbols belong to which files / packages

These are stored in a **SQLite database** (one per repo) under `~/.claudio/codex/`. SQLite is built with **FTS5** so symbol search uses a real full-text index, not a linear scan.

### 2. Incremental refresh

On every query the binary calls `index.Refresh(store)` which checks file mtimes against the index and re-parses only the files that have changed. You almost never need to re-run `index` manually.

### 3. Querying

The Claudio agent invokes the plugin as a regular subprocess:

```bash
claudio-codex context EventHandler
claudio-codex refs handleRequest
claudio-codex impact ParseConfig 5
```

Each command prints a compact, model-friendly text format on stdout. No JSON noise, no boilerplate — just the structural answer.

---

## Integration with Claudio

Claudio auto-discovers any executable in `~/.claudio/plugins/` and exposes it as a **deferred tool** (the schema is fetched lazily via `--schema` so it doesn't bloat the system prompt). When you install `claudio-codex`, Claudio will:

1. Detect the binary on startup.
2. Call `claudio-codex --describe` to get a human-readable description.
3. Call `claudio-codex --instructions` to get model-facing instructions that tell the agent **when to prefer codex over Grep/Read/Glob**.
4. Register it as `plugin_claudio-codex`, callable from any agent.

The instructions shipped with the plugin explicitly tell the model:

> **ALWAYS prefer plugin_claudio-codex over Grep, Read, or Glob when the task involves:**
> - Finding where a symbol is defined or used → use `search` or `refs`
> - Understanding what calls a function → use `refs` or `context`
> - Analyzing impact of changing a symbol → use `impact`
> - Getting a codebase overview → use `structure` or `hotspots`
> - Listing symbols in a file → use `outline`
> - Getting full context for a symbol (source + callers + callees) → use `context`
>
> Fall back to Grep/Read only when the index hasn't been built yet, or when searching for string literals/comments that aren't symbols.

The result: on a large codebase, a single Claudio session can save **tens of thousands of tokens** that would otherwise be spent re-discovering the same structural facts over and over.

---

## Installation

### One-liner (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/Abraxas-365/claudio-codex/main/install.sh | sh
```

This downloads the latest release binary for your OS/arch into `~/.claudio/plugins/claudio-codex`.

### From source

```bash
git clone https://github.com/Abraxas-365/claudio-codex
cd claudio-codex
make install-plugin
```

This builds the binary with `make build` and copies it to `~/.claudio/plugins/`.

> **Note:** building from source requires CGO (for the SQLite FTS5 extension and tree-sitter bindings). Released binaries are statically linked where possible.

### Verify

```bash
claudio-codex --version
```

Then start Claudio in any project:

```bash
cd your-project
claudio-codex index   # build the index (run once)
claudio                # launch Claudio — the plugin is auto-discovered
```

---

## Commands

| Command | Description |
|---|---|
| `index [dir]` | Build or refresh the code index (default: current dir) |
| `search <query>` | Search for symbols by name |
| `refs <symbol>` | Find all call sites referencing a symbol |
| `context <symbol>` | Bundled view: definition + source + callers + callees |
| `impact <symbol> [depth]` | Show transitive callers (impact of changing a symbol) |
| `trace <symbol> [depth]` | Show outgoing calls from a symbol |
| `outline <file>` | List all symbols in a file |
| `structure` | High-level codebase overview |
| `hotspots [limit]` | Most-referenced symbols |

The `context` command also accepts `file:line` references, e.g. `claudio-codex context internal/query/engine.go:29`.

---

## Supported languages

- Go
- Rust
- Python
- JavaScript / TypeScript
- Java
- Ruby
- C

Each language has its own tree-sitter grammar binding under `internal/parser/`. Adding a new language is a matter of adding a `lang_<name>.go` file that maps tree-sitter node types to symbol kinds and registering it in `languages.go`.

---

## Project layout

```
cmd/claudio-codex/      # CLI entrypoint
internal/walker/        # File system walker (gitignore-aware)
internal/parser/        # Tree-sitter parsers per language
internal/index/         # SQLite store + indexer + incremental refresh
internal/query/         # search, refs, context, impact, trace, outline, structure, hotspots
install.sh              # One-liner installer
Makefile                # build / install / test targets
```

---

## License

MIT

<div align="center">

# claudio-codex

**A structural code index plugin for [Claudio](https://github.com/Abraxas-365/claudio).**

Answer "where is X defined?", "what calls this?", "what breaks if I change this?" in **~50 tokens** instead of thousands of grep sweeps.

[![Go](https://img.shields.io/badge/go-1.23-00ADD8?style=flat-square&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-green?style=flat-square)](LICENSE)
[![Plugin](https://img.shields.io/badge/claudio-plugin-blueviolet?style=flat-square)](https://github.com/Abraxas-365/claudio)

</div>

---

## The problem

Coding agents waste enormous context on **navigation**. A typical "find the definition of X and see who calls it" round-trip looks like:

```
grep -r "handleRequest"     → 40 lines of noise
cat internal/server/http.go → 300 lines, skimmed for the right one
grep -r "http.Handle"       → another 20 lines
cat internal/router/...     → 200 more lines
```

That's **5,000–20,000 tokens** — before the agent writes a single line of code.

`claudio-codex` collapses that to one call:

```
claudio-codex context handleRequest
→ file: internal/server/http.go:42
  callers: [router.Register, main.initServer]
  callees: [auth.Verify, db.Query]
  source: func handleRequest(w http.ResponseWriter, r *http.Request) { ...
```

**~80 tokens. Done.**

---

## How it works

```
claudio agent
    │
    │  plugin_claudio-codex (deferred tool)
    ▼
~/.claudio/plugins/claudio-codex   ← single Go binary
    │
    ├── tree-sitter parsers        ← Go, Rust, Python, JS, Java, Ruby, C
    │
    └── ~/.claudio/codex/<hash>.db ← SQLite + FTS5, one DB per repo
```

**Indexing** — `claudio-codex index` walks the repo, parses every supported file with tree-sitter, and extracts symbols (functions, types, methods, constants), their definitions (file + line), and every call site. Stored in SQLite with FTS5 for fast full-text symbol search.

**Incremental refresh** — every query starts with a fast `mtime + size` check. Only changed files are re-parsed. On a warm cache, the overhead is negligible.

**Per-repo isolation** — each project gets its own `~/.claudio/codex/<sha256-of-path>.db`. Switching repos never invalidates another project's index.

**Auto-discovery** — Claudio picks up the binary from `~/.claudio/plugins/` at startup, calls `--describe` / `--instructions` / `--schema`, and registers it as a deferred tool (`plugin_claudio-codex`). No config needed.

---

## Install

**One-liner:**

```bash
curl -fsSL https://raw.githubusercontent.com/Abraxas-365/claudio-codex/main/install.sh | sh
```

Downloads the prebuilt binary for your OS/arch into `~/.claudio/plugins/`.

**From source** (requires CGO for SQLite FTS5 + tree-sitter):

```bash
git clone https://github.com/Abraxas-365/claudio-codex
cd claudio-codex
make install-plugin
```

**Verify:**

```bash
claudio-codex --version
```

> **Global CLI access:** The binary lives in `~/.claudio/plugins/`. To use `claudio-codex` from any terminal, add that directory to your `PATH`:
>
> ```bash
> # add to ~/.zshrc or ~/.bashrc
> export PATH="$HOME/.claudio/plugins:$PATH"
> ```

---

## Quick start

```bash
cd your-project
claudio-codex index    # build the index (~seconds for most repos)
claudio                # Claudio auto-discovers the plugin
```

That's it. From the first session, Claudio will prefer `plugin_claudio-codex` over `grep`/`read`/`glob` for any structural question.

---

## Auto-indexing

Install the bundled hooks so the index stays warm automatically — no manual `index` runs:

```bash
claudio-codex install-hooks
```

This merges three entries into `~/.claudio/settings.json`. Re-running is idempotent; your existing hooks are never touched.

| Hook | Fires when | Does |
|---|---|---|
| `SessionStart` | Claudio starts | Refreshes the index for the current project |
| `PostToolUse` (`Write\|Edit\|NotebookEdit`) | Agent writes a file | Incrementally re-parses changed files |
| `CwdChanged` | Working directory changes | Refreshes the index for the new project |

All hooks are `async: true` — they never block the agent.

Preview without installing:

```bash
claudio-codex --hooks
```

> **Why hooks and not a file watcher?** Claudio already knows exactly when files change — it's the one writing them. Piggy-backing on the existing event bus is simpler and cheaper than a long-lived `fsnotify` daemon per project.

---

## Commands

| Command | What it does |
|---|---|
| `index [dir]` | Build or refresh the code index |
| `search <query>` | Find symbols by name (FTS, fuzzy-friendly) |
| `refs <symbol>` | All call sites referencing a symbol |
| `context <symbol\|file:line>` | Definition + source + callers + callees in one shot |
| `impact <symbol> [depth]` | Transitive callers — what breaks if this changes? |
| `trace <symbol> [depth]` | Outgoing calls — what does this call? |
| `outline <file>` | All symbols defined in a file |
| `structure` | High-level codebase overview by package/module |
| `hotspots [limit]` | Most-referenced symbols |
| `install-hooks` | Merge auto-index hooks into `~/.claudio/settings.json` |
| `--hooks` | Print the hooks JSON snippet |
| `--version` | Print version |

`context` accepts both symbol names and `file:line` references:

```bash
claudio-codex context handleRequest
claudio-codex context internal/server/http.go:42
```

---

## Supported languages

| Language | Extension |
|---|---|
| Go | `.go` |
| Rust | `.rs` |
| Python | `.py` |
| JavaScript | `.js`, `.jsx`, `.mjs` |
| Java | `.java` |
| Ruby | `.rb` |
| C | `.c`, `.h` |

Adding a language: create `internal/parser/lang_<name>.go`, map tree-sitter node types to symbol kinds, register in `languages.go`.

---

## Project layout

```
cmd/claudio-codex/      CLI entrypoint, all subcommands
internal/walker/        gitignore-aware file system walker
internal/parser/        tree-sitter grammar bindings per language
internal/index/         SQLite store, indexer, incremental refresh
internal/query/         search · refs · context · impact · trace · outline · structure · hotspots
hooks.json              recommended hooks snippet (for manual merging)
install.sh              prebuilt binary installer
Makefile                build / install-plugin / install-hooks / test
```

---

## License

MIT

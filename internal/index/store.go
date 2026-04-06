package index

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"

	"github.com/Abraxas-365/claudio-codex/internal/parser"
)

// Store manages the SQLite index database.
type Store struct {
	db      *sql.DB
	dbPath  string
	repoDir string
}

// DBPathForRepo returns the database path for a given repo root directory.
func DBPathForRepo(repoDir string) (string, error) {
	abs, err := filepath.Abs(repoDir)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(abs))
	name := fmt.Sprintf("%x", hash[:8])

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".claudio", "codex")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".db"), nil
}

// Open opens or creates the index database.
func Open(dbPath, repoDir string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Performance pragmas
	for _, pragma := range []string{
		"PRAGMA cache_size = -64000",
		"PRAGMA mmap_size = 268435456",
		"PRAGMA temp_store = MEMORY",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("pragma %s: %w", pragma, err)
		}
	}

	s := &Store{db: db, dbPath: dbPath, repoDir: repoDir}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying sql.DB for direct query access.
func (s *Store) DB() *sql.DB {
	return s.db
}

// RepoDir returns the repository root directory.
func (s *Store) RepoDir() string {
	return s.repoDir
}

func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS files (
		id       INTEGER PRIMARY KEY,
		path     TEXT UNIQUE NOT NULL,
		hash     TEXT NOT NULL,
		mtime_ns INTEGER NOT NULL,
		size     INTEGER NOT NULL,
		language TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS symbols (
		id        INTEGER PRIMARY KEY,
		file_id   INTEGER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
		name      TEXT NOT NULL,
		kind      TEXT NOT NULL,
		line      INTEGER NOT NULL,
		end_line  INTEGER NOT NULL,
		signature TEXT,
		parent    TEXT,
		exported  BOOLEAN NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS refs (
		id      INTEGER PRIMARY KEY,
		file_id INTEGER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
		caller  TEXT NOT NULL,
		target  TEXT NOT NULL,
		line    INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS imports (
		id      INTEGER PRIMARY KEY,
		file_id INTEGER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
		path    TEXT NOT NULL,
		alias   TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_symbols_name ON symbols(name);
	CREATE INDEX IF NOT EXISTS idx_symbols_file ON symbols(file_id);
	CREATE INDEX IF NOT EXISTS idx_symbols_kind ON symbols(kind);
	CREATE INDEX IF NOT EXISTS idx_refs_target ON refs(target);
	CREATE INDEX IF NOT EXISTS idx_refs_caller ON refs(caller);
	CREATE INDEX IF NOT EXISTS idx_refs_file ON refs(file_id);
	CREATE INDEX IF NOT EXISTS idx_imports_path ON imports(path);
	CREATE INDEX IF NOT EXISTS idx_imports_file ON imports(file_id);
	CREATE INDEX IF NOT EXISTS idx_files_path ON files(path);
	`

	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	// FTS5 table — create only if it doesn't exist
	// We check by querying sqlite_master since CREATE VIRTUAL TABLE IF NOT EXISTS
	// is not supported by all SQLite versions.
	var ftsExists int
	s.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='symbols_fts'").Scan(&ftsExists)
	if ftsExists == 0 {
		fts := `
		CREATE VIRTUAL TABLE symbols_fts USING fts5(name, kind, signature, content=symbols, content_rowid=id);

		CREATE TRIGGER IF NOT EXISTS symbols_ai AFTER INSERT ON symbols BEGIN
			INSERT INTO symbols_fts(rowid, name, kind, signature) VALUES (new.id, new.name, new.kind, new.signature);
		END;

		CREATE TRIGGER IF NOT EXISTS symbols_ad AFTER DELETE ON symbols BEGIN
			INSERT INTO symbols_fts(symbols_fts, rowid, name, kind, signature) VALUES('delete', old.id, old.name, old.kind, old.signature);
		END;
		`
		if _, err := s.db.Exec(fts); err != nil {
			return fmt.Errorf("create fts: %w", err)
		}
	}

	return nil
}

// FileRecord holds stored metadata about an indexed file.
type FileRecord struct {
	ID      int64
	Path    string
	Hash    string
	MtimeNs int64
	Size    int64
	Lang    string
}

// GetFile returns the stored file record, or nil if not found.
func (s *Store) GetFile(path string) (*FileRecord, error) {
	row := s.db.QueryRow("SELECT id, path, hash, mtime_ns, size, language FROM files WHERE path = ?", path)
	var f FileRecord
	err := row.Scan(&f.ID, &f.Path, &f.Hash, &f.MtimeNs, &f.Size, &f.Lang)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// UpsertFile inserts or updates a file record and returns its ID.
func (s *Store) UpsertFile(path, hash string, mtimeNs, size int64, lang string) (int64, error) {
	res, err := s.db.Exec(`
		INSERT INTO files (path, hash, mtime_ns, size, language)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET hash=excluded.hash, mtime_ns=excluded.mtime_ns, size=excluded.size, language=excluded.language
	`, path, hash, mtimeNs, size, lang)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// DeleteFileData removes all symbols, refs, imports for a file.
func (s *Store) DeleteFileData(fileID int64) error {
	for _, table := range []string{"symbols", "refs", "imports"} {
		if _, err := s.db.Exec("DELETE FROM "+table+" WHERE file_id = ?", fileID); err != nil {
			return err
		}
	}
	return nil
}

// DeleteFile removes a file and all its data.
func (s *Store) DeleteFile(fileID int64) error {
	_, err := s.db.Exec("DELETE FROM files WHERE id = ?", fileID)
	return err
}

// InsertParseResult stores symbols, refs, and imports for a file.
func (s *Store) InsertParseResult(fileID int64, result *parser.ParseResult) error {
	for _, sym := range result.Symbols {
		_, err := s.db.Exec(`
			INSERT INTO symbols (file_id, name, kind, line, end_line, signature, parent, exported)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, fileID, sym.Name, sym.Kind, sym.Line, sym.EndLine, sym.Signature, sym.Parent, sym.Exported)
		if err != nil {
			return err
		}
	}

	for _, ref := range result.Refs {
		_, err := s.db.Exec(`
			INSERT INTO refs (file_id, caller, target, line)
			VALUES (?, ?, ?, ?)
		`, fileID, ref.Caller, ref.Target, ref.Line)
		if err != nil {
			return err
		}
	}

	for _, imp := range result.Imports {
		_, err := s.db.Exec(`
			INSERT INTO imports (file_id, path, alias)
			VALUES (?, ?, ?)
		`, fileID, imp.Path, imp.Alias)
		if err != nil {
			return err
		}
	}

	return nil
}

// AllFilePaths returns all indexed file paths.
func (s *Store) AllFilePaths() (map[string]int64, error) {
	rows, err := s.db.Query("SELECT id, path FROM files")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int64)
	for rows.Next() {
		var id int64
		var path string
		if err := rows.Scan(&id, &path); err != nil {
			return nil, err
		}
		result[path] = id
	}
	return result, rows.Err()
}

// RelPath returns path relative to the repo dir for display.
func (s *Store) RelPath(absPath string) string {
	rel, err := filepath.Rel(s.repoDir, absPath)
	if err != nil {
		return absPath
	}
	return rel
}

// LanguageForExt returns the language name for a file extension.
func LanguageForExt(ext string) string {
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".hpp":
		return "cpp"
	case ".rb":
		return "ruby"
	default:
		return strings.TrimPrefix(ext, ".")
	}
}

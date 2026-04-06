package index

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Abraxas-365/claudio-codex/internal/parser"
	"github.com/Abraxas-365/claudio-codex/internal/walker"
)

// IndexResult contains stats from an indexing operation.
type IndexResult struct {
	TotalFiles   int
	IndexedFiles int
	SkippedFiles int
	DeletedFiles int
	Duration     time.Duration
}

// Index performs a full or incremental index of the given directory.
func Index(store *Store) (*IndexResult, error) {
	start := time.Now()
	result := &IndexResult{}

	files, err := walker.Walk(store.RepoDir())
	if err != nil {
		return nil, fmt.Errorf("walk: %w", err)
	}
	result.TotalFiles = len(files)

	// Build set of current files for deletion detection
	currentFiles := make(map[string]bool, len(files))
	for _, f := range files {
		currentFiles[f] = true
	}

	// Remove files that no longer exist
	indexed, err := store.AllFilePaths()
	if err != nil {
		return nil, fmt.Errorf("list indexed: %w", err)
	}
	for path, id := range indexed {
		if !currentFiles[path] {
			if err := store.DeleteFileData(id); err != nil {
				return nil, fmt.Errorf("delete data for %s: %w", path, err)
			}
			if err := store.DeleteFile(id); err != nil {
				return nil, fmt.Errorf("delete file %s: %w", path, err)
			}
			result.DeletedFiles++
		}
	}

	// Begin transaction for batch insert
	tx, err := store.DB().Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	for _, filePath := range files {
		ext := filepath.Ext(filePath)
		if !parser.SupportedExt(ext) {
			result.SkippedFiles++
			continue
		}

		info, err := os.Stat(filePath)
		if err != nil {
			result.SkippedFiles++
			continue
		}

		// Check if file needs reindexing
		existing, err := store.GetFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("get file %s: %w", filePath, err)
		}

		mtimeNs := info.ModTime().UnixNano()

		// Fast path: if mtime and size match, skip
		if existing != nil && existing.MtimeNs == mtimeNs && existing.Size == info.Size() {
			result.SkippedFiles++
			continue
		}

		// Read and hash file
		data, err := os.ReadFile(filePath)
		if err != nil {
			result.SkippedFiles++
			continue
		}
		hash := fmt.Sprintf("%x", sha256.Sum256(data))

		// Content hash check: mtime changed but content didn't
		if existing != nil && existing.Hash == hash {
			// Update mtime only
			store.UpsertFile(filePath, hash, mtimeNs, info.Size(), existing.Lang)
			result.SkippedFiles++
			continue
		}

		// Parse the file
		parseResult, err := parser.Parse(filePath, data)
		if err != nil {
			result.SkippedFiles++
			continue
		}

		lang := LanguageForExt(ext)

		// Delete old data if exists
		if existing != nil {
			if err := store.DeleteFileData(existing.ID); err != nil {
				return nil, fmt.Errorf("delete old data %s: %w", filePath, err)
			}
		}

		// Upsert file record
		fileID, err := store.UpsertFile(filePath, hash, mtimeNs, info.Size(), lang)
		if err != nil {
			return nil, fmt.Errorf("upsert file %s: %w", filePath, err)
		}

		// For upsert, LastInsertId may return 0 if updated — need to fetch ID
		if fileID == 0 && existing != nil {
			fileID = existing.ID
		}

		// Insert new parse results
		if err := store.InsertParseResult(fileID, parseResult); err != nil {
			return nil, fmt.Errorf("insert parse result %s: %w", filePath, err)
		}

		result.IndexedFiles++
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	result.Duration = time.Since(start)
	return result, nil
}

// Refresh checks for changes and re-indexes only modified files.
// It's the same as Index — the incremental logic is built in.
func Refresh(store *Store) (*IndexResult, error) {
	return Index(store)
}

package walker

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// supportedExts is the set of file extensions we index.
var supportedExts = map[string]bool{
	".go":   true,
	".py":   true,
	".js":   true,
	".ts":   true,
	".tsx":  true,
	".jsx":  true,
	".rs":   true,
	".java": true,
	".c":    true,
	".h":    true,
	".cpp":  true,
	".cc":   true,
	".hpp":  true,
	".rb":   true,
}

// Walk returns all indexable files under dir.
// Uses git ls-files if inside a git repo, otherwise falls back to a manual walk.
func Walk(dir string) ([]string, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	files, err := gitLsFiles(absDir)
	if err != nil {
		return nil, err
	}

	var result []string
	for _, f := range files {
		ext := filepath.Ext(f)
		if supportedExts[ext] {
			result = append(result, f)
		}
	}
	return result, nil
}

func gitLsFiles(dir string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return fallbackWalk(dir)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var result []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		result = append(result, filepath.Join(dir, line))
	}
	return result, nil
}

func fallbackWalk(dir string) ([]string, error) {
	var result []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		name := info.Name()
		if info.IsDir() {
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "target" || name == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}
		result = append(result, path)
		return nil
	})
	return result, err
}

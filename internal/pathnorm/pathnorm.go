package pathnorm

import (
	"os"
	"path/filepath"
	"strings"
)

// Normalize converts a file path to a repo-relative, forward-slash normalized form.
// It resolves the path to absolute, resolves symlinks where possible,
// and strips the repo root prefix.
func Normalize(inputPath string, repoRoot string) (string, error) {
	abs, err := filepath.Abs(inputPath)
	if err != nil {
		return "", err
	}

	// Resolve symlinks (best-effort: if the path doesn't exist yet, skip).
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		abs = resolved
	}

	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", err
	}
	resolvedRoot, err := filepath.EvalSymlinks(absRoot)
	if err == nil {
		absRoot = resolvedRoot
	}

	rel, err := filepath.Rel(absRoot, abs)
	if err != nil {
		return "", err
	}

	// Normalize separators to forward slash and clean redundant segments.
	norm := filepath.ToSlash(filepath.Clean(rel))
	return norm, nil
}

// FindRepoRoot walks upward from the current working directory looking
// for a .git directory. Falls back to CWD if none is found.
func FindRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if info, err := os.Stat(filepath.Join(dir, ".git")); err == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return os.Getwd()
		}
		dir = parent
	}
}

// Match checks if a normalized file path matches a normalized claim pattern.
// Supports:
//   - Exact match: "src/auth/login.go"
//   - Directory prefix: "src/auth" matches "src/auth/login.go"
//   - Glob: "src/auth/*.go" matches "src/auth/login.go"
func Match(pattern, filePath string) bool {
	// Exact match.
	if pattern == filePath {
		return true
	}

	// Directory prefix: pattern "src/auth" matches "src/auth/anything".
	if strings.HasPrefix(filePath, pattern+"/") {
		return true
	}

	// Glob match (best-effort, ignore errors on malformed patterns).
	matched, err := filepath.Match(pattern, filePath)
	if err == nil && matched {
		return true
	}

	// Try matching just the directory part for patterns like "src/auth/*".
	dir := filePath
	for dir != "." && dir != "" {
		dir = filepath.Dir(dir)
		matched, err := filepath.Match(pattern, dir)
		if err == nil && matched {
			return true
		}
	}

	return false
}

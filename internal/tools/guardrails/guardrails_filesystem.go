package guardrails

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func NormalizePath(path string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	// Resolve symlinks in the working directory so subsequent comparisons are
	// made against the physical path. This avoids false rejections when wd is
	// itself reached through a symlink.
	if wd, err = filepath.EvalSymlinks(wd); err != nil {
		return "", err
	}

	absPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}

	// Resolve symlinks for the full path to prevent sandbox escape via a
	// symlink component anywhere in the path (e.g. `ln -s /etc/passwd secret`).
	// The target file may not exist yet (write/create operations), in which
	// case we fall back to resolving only the parent directory.
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		dir := filepath.Dir(absPath)
		resolvedDir, derr := filepath.EvalSymlinks(dir)
		if derr != nil {
			return "", err
		}
		resolvedPath = filepath.Join(resolvedDir, filepath.Base(absPath))
	}

	// Check if resolvedPath is within wd (allow exact match or subdirectories).
	// When wd is the filesystem root there is no meaningful prefix to enforce.
	if resolvedPath != wd {
		if wd != string(filepath.Separator) &&
			!strings.HasPrefix(resolvedPath, wd+string(filepath.Separator)) {
			return "", fmt.Errorf("directory %q is outside of the working directory", path)
		}
	}

	return resolvedPath, nil
}

func IsAllowedPath(path string) bool {
	name := filepath.Base(path)
	switch name {
	case ".git", ".env":
		return false
	default:
		return true
	}
}

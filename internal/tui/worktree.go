package tui

import (
	"os"
	"path/filepath"
)

// worktreeRoot is the git checkout that contains cwd.
// The main checkout and a linked worktree both count: the directory that holds .git.
// An empty string means cwd is not inside a checkout.
func worktreeRoot(cwd string) string {
	dir := filepath.Clean(cwd)
	if dir == "" || dir == "." {
		return ""
	}
	for {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

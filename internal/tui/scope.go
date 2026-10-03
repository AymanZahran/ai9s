package tui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// launchScope is the directory ai9s was started in, when the list should
// show only sessions in that directory and its children. The home directory
// returns no scope, so the list shows every session.
func launchScope() (display string, roots []string) {
	wd, err := os.Getwd()
	if err != nil {
		return "", nil
	}
	wd = filepath.Clean(wd)
	if wd == "" || wd == "." {
		return "", nil
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", nil
	}
	home = filepath.Clean(home)
	if sameDir(wd, home) || sameDir(wd, resolved(home)) {
		return "", nil
	}
	real := resolved(wd)
	if sameDir(real, home) || sameDir(real, resolved(home)) {
		return "", nil
	}
	roots = []string{wd}
	if real != "" && !sameDir(real, wd) {
		roots = append(roots, real)
	}
	return wd, roots
}

func resolved(path string) string {
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return ""
	}
	return filepath.Clean(real)
}

func sameDir(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	if (runtime.GOOS == "darwin" || runtime.GOOS == "windows") && strings.EqualFold(a, b) {
		return true
	}
	ia, err1 := os.Stat(a)
	ib, err2 := os.Stat(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return os.SameFile(ia, ib)
}

package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestWorktreeRoot(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	wt := filepath.Join(root, "wt")
	if err := os.MkdirAll(filepath.Join(repo, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=air9s",
			"GIT_AUTHOR_EMAIL=air9s@example.com",
			"GIT_COMMITTER_NAME=air9s",
			"GIT_COMMITTER_EMAIL=air9s@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git(repo, "init")
	git(repo, "commit", "--allow-empty", "-m", "init")
	git(repo, "worktree", "add", "-b", "feature", wt)

	if got := worktreeRoot(filepath.Join(repo, "sub")); got != repo {
		t.Fatalf("main checkout %q", got)
	}
	if got := worktreeRoot(wt); got != wt {
		t.Fatalf("linked worktree %q", got)
	}
	if got := worktreeRoot(root); got != "" {
		t.Fatalf("outside %q", got)
	}
	if got := worktreeRoot(""); got != "" {
		t.Fatalf("empty %q", got)
	}
}

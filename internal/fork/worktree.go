package fork

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/AymanZahran/ai9s/internal/model"
)

// Options selects what Copy changes besides the session id.
type Options struct {
	Worktree bool
	Name     string
}

// dirMove is where a forked session should resume.
// The zero value leaves the copied directory fields alone.
type dirMove struct {
	oldCWD, newCWD       string
	oldBranch, newBranch string
}

func (m dirMove) active() bool { return m.newCWD != "" }

func (m dirMove) swaps() []textSwap {
	if !m.active() {
		return nil
	}
	out := []textSwap{{
		keys: map[string]struct{}{"cwd": {}},
		old:  m.oldCWD,
		new:  m.newCWD,
	}}
	if m.oldBranch != "" && m.newBranch != "" && m.oldBranch != m.newBranch {
		out = append(out, textSwap{
			keys: map[string]struct{}{"gitBranch": {}},
			old:  m.oldBranch,
			new:  m.newBranch,
		})
	}
	return out
}

type textSwap struct {
	keys     map[string]struct{}
	old, new string
}

func prepareWorktree(s model.Session, opt Options) (dirMove, func(), error) {
	noop := func() {}
	if !opt.Worktree {
		return dirMove{}, noop, nil
	}
	switch s.Agent {
	case "junie", "letta":
		return dirMove{}, noop, errors.New("this harness does not record a directory for a worktree")
	}
	root, err := checkoutRoot(s.CWD)
	if err != nil {
		return dirMove{}, noop, err
	}
	if _, err := exec.LookPath("git"); err != nil {
		return dirMove{}, noop, errors.New("git is not on PATH")
	}
	if _, err := git(root, "rev-parse", "--verify", "HEAD"); err != nil {
		return dirMove{}, noop, errors.New("this checkout has no commit to branch from")
	}
	branch := uniqueBranch(root, branchName(opt.Name))
	leaf := strings.ReplaceAll(branch, "/", "-")
	parent := filepath.Join(filepath.Dir(root), filepath.Base(root)+".worktrees")
	path := filepath.Join(parent, leaf)
	if !outsideCheckout(root, path) {
		return dirMove{}, noop, errors.New("refusing to add a worktree inside the checkout")
	}
	if _, err := os.Lstat(path); err == nil {
		return dirMove{}, noop, errors.New("a worktree with this name already exists")
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return dirMove{}, noop, err
	}
	if _, err := git(root, "worktree", "add", "-b", branch, path, "HEAD"); err != nil {
		_ = os.RemoveAll(path)
		return dirMove{}, noop, err
	}
	cleanup := func() {
		_, _ = git(root, "worktree", "remove", "--force", path)
		_ = os.RemoveAll(path)
	}
	rel, err := filepath.Rel(root, filepath.Clean(s.CWD))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		cleanup()
		return dirMove{}, noop, errors.New("this session is not inside a git checkout")
	}
	newCWD := path
	if rel != "." {
		newCWD = filepath.Join(path, rel)
		if err := os.MkdirAll(newCWD, 0o755); err != nil {
			cleanup()
			return dirMove{}, noop, err
		}
	}
	return dirMove{
		oldCWD: filepath.Clean(s.CWD), newCWD: newCWD,
		oldBranch: s.Branch, newBranch: branch,
	}, cleanup, nil
}

func checkoutRoot(cwd string) (string, error) {
	dir := filepath.Clean(strings.TrimSpace(cwd))
	if dir == "" || dir == "." {
		return "", errors.New("this session is not inside a git checkout")
	}
	for {
		info, err := os.Lstat(filepath.Join(dir, ".git"))
		if err == nil && (info.IsDir() || info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("this session is not inside a git checkout")
		}
		dir = parent
	}
}

func outsideCheckout(root, path string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func branchName(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if unicode.Is(unicode.Letter, r) || unicode.Is(unicode.Digit, r) {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		slug = "fork"
	}
	if len(slug) > 40 {
		slug = strings.Trim(slug[:40], "-")
	}
	return "fork/" + slug
}

func uniqueBranch(root, branch string) string {
	if _, err := git(root, "show-ref", "--verify", "--quiet", "refs/heads/"+branch); err != nil {
		return branch
	}
	suffix := newForkID()
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	return branch + "-" + suffix
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", errors.New(gitLine(out))
	}
	return string(out), nil
}

func gitLine(out []byte) string {
	line := ""
	for _, raw := range strings.Split(string(out), "\n") {
		raw = strings.TrimSpace(raw)
		if raw != "" {
			line = raw
		}
	}
	line = strings.TrimPrefix(line, "fatal: ")
	line = strings.TrimPrefix(line, "error: ")
	line = strings.TrimRight(line, ".")
	if line == "" {
		return "could not add a worktree"
	}
	if len(line) > 180 {
		line = line[:180]
	}
	return "could not add a worktree: " + line
}

package plugins_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func scriptsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Dir(file)
}

func runScript(t *testing.T, name string, args []string, env []string, dir string) (string, string, error) {
	t.Helper()
	cmd := exec.Command("sh", append([]string{filepath.Join(scriptsDir(t), name)}, args...)...)
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.Output()
	var stderr string
	if ee, ok := err.(*exec.ExitError); ok {
		stderr = string(ee.Stderr)
	}
	return string(out), stderr, err
}

func TestCopySessionDoesNotEval(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "pbcopy")
	stamp := filepath.Join(dir, "clip")
	body := "#!/bin/sh\ncat > \"$STAMP\"\n"
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "PATH="+dir+":/bin:/usr/bin", "STAMP="+stamp)
	literal := "$(echo pwned); rm -rf /"
	out, stderr, err := runScript(t, "copy-session", []string{"claude", literal, "title"}, env, dir)
	if err != nil {
		t.Fatalf("copy %v\n%s\n%s", err, out, stderr)
	}
	got, err := os.ReadFile(stamp)
	if err != nil {
		t.Fatal(err)
	}
	want := "claude\t" + literal + "\ttitle"
	if string(got) != want {
		t.Fatalf("clipboard %q", got)
	}
}

func TestOpenEditorUsesProgramName(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "proj")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	stub := filepath.Join(dir, "edit-stub")
	stamp := filepath.Join(dir, "opened")
	body := "#!/bin/sh\nprintf '%s\\n' \"$1\" > \"$STAMP\"\n"
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "AIR9S_EDITOR="+stub, "STAMP="+stamp, "PATH=/bin:/usr/bin")
	if _, stderr, err := runScript(t, "open-editor", []string{project}, env, dir); err != nil {
		t.Fatalf("open %v %s", err, stderr)
	}
	got, err := os.ReadFile(stamp)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != project {
		t.Fatalf("opened %q", got)
	}
}

func TestGitStoryLogLimit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	repo, err := os.MkdirTemp("", "air9s-git-story-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		removeAllRetry(t, repo)
	})
	git := func(args ...string) {
		t.Helper()
		// maintenance.auto and gc.auto stop git from writing .git after the
		// command returns. Go 1.27 fails the test when that races cleanup.
		cmd := exec.Command("git", append([]string{
			"-C", repo,
			"-c", "maintenance.auto=false",
			"-c", "gc.auto=0",
			"-c", "core.fsmonitor=",
		}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null",
			"GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-b", "main")
	git("config", "maintenance.auto", "false")
	git("config", "gc.auto", "0")
	for _, msg := range []string{"one", "two", "three"} {
		git("commit", "--allow-empty", "-m", msg)
	}
	env := append(os.Environ(), "AIR9S_GIT_LOG=2", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	cmd := exec.Command("sh", filepath.Join(scriptsDir(t), "git-story"), repo)
	cmd.Env = env
	cmd.Stdin = strings.NewReader("\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git-story %v\n%s", err, out)
	}
	text := string(out)
	if !strings.Contains(text, "== "+repo) || !strings.Contains(text, "Press enter") {
		t.Fatalf("output %s", text)
	}
	commits := 0
	for _, line := range strings.Split(text, "\n") {
		if len(line) >= 7 && isHex(line[:7]) && strings.Contains(line, " ") {
			commits++
		}
	}
	if commits != 2 {
		t.Fatalf("commits %d\n%s", commits, text)
	}

	cmd = exec.Command("sh", filepath.Join(scriptsDir(t), "git-story"), t.TempDir())
	cmd.Env = os.Environ()
	out, err = cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "not a git checkout") {
		t.Fatalf("non-repo %v %s", err, out)
	}
}

func TestNewTerminalStartsInDirectory(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "proj")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	stub := filepath.Join(dir, "term-stub")
	stamp := filepath.Join(dir, "where")
	body := "#!/bin/sh\npwd > \"$STAMP\"\n"
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "AIR9S_TERMINAL="+stub, "STAMP="+stamp, "PATH=/bin:/usr/bin:/usr/bin")
	cmd := exec.Command("sh", filepath.Join(scriptsDir(t), "new-terminal"), project)
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("terminal %v\n%s", err, out)
	}
	deadline := time.Now().Add(2 * time.Second)
	var got []byte
	var err error
	for time.Now().Before(deadline) {
		got, err = os.ReadFile(stamp)
		if err == nil && len(strings.TrimSpace(string(got))) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	where := strings.TrimSpace(string(got))
	resolved, err := filepath.EvalSymlinks(project)
	if err != nil {
		t.Fatal(err)
	}
	gotResolved, err := filepath.EvalSymlinks(where)
	if err != nil {
		t.Fatal(err)
	}
	if gotResolved != resolved {
		t.Fatalf("terminal cwd %q", where)
	}
}

func removeAllRetry(t *testing.T, dir string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var err error
	for {
		err = os.RemoveAll(dir)
		if err == nil || time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Errorf("cleanup %s: %v", dir, err)
	}
}

func isHex(s string) bool {
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		default:
			return false
		}
	}
	return true
}

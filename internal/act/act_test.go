package act

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AymanZahran/air9s/internal/model"
)

func TestPlan(t *testing.T) {
	orig := LookPath
	t.Cleanup(func() { LookPath = orig })
	LookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	dir := t.TempDir()
	cmd, err := Plan(model.Session{Agent: "claude", NativeID: "abc", CWD: dir}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cmd.String(), "--dangerously-skip-permissions") || !strings.Contains(cmd.String(), "--resume abc") {
		t.Fatalf("plan %s", cmd)
	}
	cmd, err = Plan(model.Session{Agent: "codex", NativeID: "u", CWD: dir}, true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(cmd.String(), "danger") || !strings.Contains(cmd.String(), "resume u") {
		t.Fatalf("codex plan %s", cmd)
	}
}

func TestDeleteFileGuards(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", root)
	path := filepath.Join(root, "projects", "demo", "abc.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sess := model.Session{Agent: "claude", NativeID: "abc", CanDelete: true, DeleteMode: "file", SourcePath: path}
	if err := Delete(sess); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file still exists")
	}
	outside := filepath.Join(t.TempDir(), "notes.jsonl")
	if err := os.WriteFile(outside, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sess.SourcePath = outside
	if err := Delete(sess); err == nil {
		t.Fatal("deleted a file outside the claude projects directory")
	}
}

func TestRewriteAgy(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GEMINI_HOME", root)
	path := filepath.Join(root, "antigravity-cli", "history.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "{\"conversationId\":\"keep\",\"display\":\"stay\"}\n{\"conversationId\":\"drop\",\"display\":\"gone\"}\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	err := Delete(model.Session{Agent: "agy", NativeID: "drop", CanDelete: true, DeleteMode: "rewrite", SourcePath: path})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "drop") || !strings.Contains(string(got), "keep") {
		t.Fatalf("history %s", got)
	}
}

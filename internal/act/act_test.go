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

func TestPlanMoreAgents(t *testing.T) {
	orig := LookPath
	t.Cleanup(func() { LookPath = orig })
	LookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	dir := t.TempDir()
	checks := []struct {
		sess model.Session
		yolo bool
		want []string
		ban  []string
	}{
		{model.Session{Agent: "hermes", NativeID: "abc", CWD: dir}, true, []string{"--yolo", "--resume abc"}, []string{"-p "}},
		{model.Session{Agent: "hermes", NativeID: "p:work:abc:def", CWD: dir}, true, []string{"-p work", "--yolo", "--resume abc:def"}, []string{"--resume p:"}},
		{model.Session{Agent: "openclaw", NativeID: "agent:main:abc", CWD: dir}, true, []string{"resume agent:main:abc"}, []string{"yolo", "trust"}},
		{model.Session{Agent: "junie", NativeID: "session-1", CWD: dir}, true, []string{"--brave", "--resume", "--session-id=session-1"}, nil},
		{model.Session{Agent: "jules", NativeID: "12345", CWD: dir}, true, []string{"teleport 12345"}, []string{"yolo", "brave"}},
		{model.Session{Agent: "goose", NativeID: "g1", CWD: dir}, true, []string{"session --resume --session-id g1"}, []string{"yolo"}},
		{model.Session{Agent: "cline", NativeID: "t1", CWD: dir}, true, []string{"task open t1", "--yolo"}, nil},
		{model.Session{Agent: "aider", NativeID: filepath.Join(dir, ".aider.chat.history.md"), SourcePath: filepath.Join(dir, ".aider.chat.history.md"), CWD: dir}, true, []string{"--restore-chat-history"}, []string{"--chat-history-file", "yolo"}},
		{model.Session{Agent: "aider", NativeID: filepath.Join(dir, "custom.md"), SourcePath: filepath.Join(dir, "custom.md"), CWD: dir}, false, []string{"--restore-chat-history", "--chat-history-file"}, nil},
		{model.Session{Agent: "kiro", NativeID: "sid", CWD: dir}, true, []string{"kiro-cli", "--trust-all-tools", "chat", "--resume-id sid"}, nil},
	}
	for _, tc := range checks {
		cmd, err := Plan(tc.sess, tc.yolo)
		if err != nil {
			t.Fatalf("%s: %v", tc.sess.Agent, err)
		}
		got := cmd.String()
		for _, want := range tc.want {
			if !strings.Contains(got, want) {
				t.Fatalf("%s plan %s missing %s", tc.sess.Agent, got, want)
			}
		}
		for _, ban := range tc.ban {
			if strings.Contains(got, ban) {
				t.Fatalf("%s plan %s contains %s", tc.sess.Agent, got, ban)
			}
		}
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

func TestDeleteJunieClineAider(t *testing.T) {
	root := t.TempDir()
	t.Setenv("JUNIE_HOME", root)
	t.Setenv("CLINE_HOME", filepath.Join(root, "cline"))
	session := filepath.Join(root, "sessions", "session-1")
	if err := os.MkdirAll(session, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(session, "transcript.md"), []byte("# Session transcript\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	transcript := filepath.Join(session, "transcript.md")
	if err := Delete(model.Session{Agent: "junie", NativeID: "session-1", CanDelete: true, DeleteMode: "dir", SourcePath: transcript}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(session); !os.IsNotExist(err) {
		t.Fatal("junie session still exists")
	}
	outside := filepath.Join(t.TempDir(), "session-1")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "transcript.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Delete(model.Session{Agent: "junie", NativeID: "session-1", CanDelete: true, DeleteMode: "dir", SourcePath: outside}); err == nil {
		t.Fatal("deleted a junie directory outside JUNIE_HOME")
	}

	home := filepath.Join(root, "cline")
	hist := filepath.Join(home, "data", "state", "taskHistory.json")
	if err := os.MkdirAll(filepath.Dir(hist), 0o755); err != nil {
		t.Fatal(err)
	}
	keepDir := filepath.Join(home, "data", "tasks", "keep")
	dropDir := filepath.Join(home, "data", "tasks", "drop")
	if err := os.MkdirAll(keepDir, 0o755); err != nil || os.MkdirAll(dropDir, 0o755) != nil {
		t.Fatal(err)
	}
	body := "[{\"id\":\"keep\",\"task\":\"stay\"},{\"id\":\"drop\",\"task\":\"gone\"}]\n"
	if err := os.WriteFile(hist, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Delete(model.Session{Agent: "cline", NativeID: "drop", CanDelete: true, DeleteMode: "cline", SourcePath: hist}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(hist)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "drop") || !strings.Contains(string(got), "keep") {
		t.Fatalf("cline history %s", got)
	}
	if _, err := os.Stat(dropDir); !os.IsNotExist(err) {
		t.Fatal("cline task dir still exists")
	}
	if _, err := os.Stat(keepDir); err != nil {
		t.Fatal(err)
	}

	history := filepath.Join(t.TempDir(), ".aider.chat.history.md")
	if err := os.WriteFile(history, []byte("#### user\nhello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Delete(model.Session{Agent: "aider", NativeID: history, CanDelete: true, DeleteMode: "aider", SourcePath: history}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(history); !os.IsNotExist(err) {
		t.Fatal("aider history still exists")
	}
	notes := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(notes, []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Delete(model.Session{Agent: "aider", NativeID: notes, CanDelete: true, DeleteMode: "aider", SourcePath: notes}); err == nil {
		t.Fatal("deleted a file that is not an Aider history")
	}
}

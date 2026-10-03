package act

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/AymanZahran/ai9s/internal/model"
)

func TestRunEcho(t *testing.T) {
	bin, err := exec.LookPath("echo")
	if err != nil {
		t.Skip(err)
	}
	if err := (Command{Name: bin, Args: []string{"ok"}}).Run(); err != nil {
		t.Fatal(err)
	}
}

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
		{model.Session{Agent: "antigravity", NativeID: "c1", CWD: dir}, true, []string{"/usr/bin/agy", "--dangerously-skip-permissions", "--conversation c1"}, []string{"--resume"}},
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
	err := Delete(model.Session{Agent: "antigravity", NativeID: "drop", CanDelete: true, DeleteMode: "rewrite", SourcePath: path})
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

func TestDeleteGrokGeminiJulesKiro(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GROK_HOME", filepath.Join(root, "grok"))
	t.Setenv("GEMINI_HOME", filepath.Join(root, "gemini"))
	t.Setenv("JULES_HOME", filepath.Join(root, "jules"))
	t.Setenv("JULES_API_KEY", "")
	t.Setenv("KIRO_CLI_DB", filepath.Join(root, "kiro.db"))

	drop := filepath.Join(root, "grok", "sessions", "proj", "grok-1")
	keep := filepath.Join(root, "grok", "sessions", "proj", "keep-1")
	if err := os.MkdirAll(drop, 0o755); err != nil || os.MkdirAll(keep, 0o755) != nil {
		t.Fatal(err)
	}
	summary := filepath.Join(drop, "summary.json")
	if err := os.WriteFile(summary, []byte(`{"info":{"id":"grok-1"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(drop, "chat_history.jsonl"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(keep, "summary.json"), []byte(`{"info":{"id":"keep-1"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	shared := filepath.Join(root, "grok", "sessions", "proj", "prompt_history.jsonl")
	if err := os.WriteFile(shared, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "grok", "client-state"), 0o755); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(root, "grok", "active_sessions.json")
	if err := os.WriteFile(active, []byte(`[{"session_id":"grok-1","pid":1},{"session_id":"keep-1","pid":2}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	meta := filepath.Join(root, "grok", "client-state", "session-meta.json")
	if err := os.WriteFile(meta, []byte("{\n  \"grok-1\": {\"provider\": \"xai\"},\n  \"keep-1\": {\"provider\": \"xai\"}\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Delete(model.Session{Agent: "grok", NativeID: "grok-1", CanDelete: true, DeleteMode: "grok", SourcePath: summary}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(drop); !os.IsNotExist(err) {
		t.Fatal("grok session directory still exists")
	}
	if _, err := os.Stat(filepath.Join(keep, "summary.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(shared); err != nil {
		t.Fatal(err)
	}
	activeBody, err := os.ReadFile(active)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(activeBody), "grok-1") || !strings.Contains(string(activeBody), "keep-1") {
		t.Fatalf("active sessions %s", activeBody)
	}
	metaBody, err := os.ReadFile(meta)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(metaBody), "grok-1") || !strings.Contains(string(metaBody), "keep-1") {
		t.Fatalf("session meta %s", metaBody)
	}
	outside := filepath.Join(t.TempDir(), "grok-1")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	outSummary := filepath.Join(outside, "summary.json")
	if err := os.WriteFile(outSummary, []byte(`{"info":{"id":"grok-1"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Delete(model.Session{Agent: "grok", NativeID: "grok-1", CanDelete: true, DeleteMode: "grok", SourcePath: outSummary}); err == nil {
		t.Fatal("deleted a grok session outside GROK_HOME")
	}
	if err := os.Symlink(outside, filepath.Join(root, "grok", "sessions", "proj", "grok-1")); err != nil {
		t.Fatal(err)
	}
	if err := Delete(model.Session{Agent: "grok", NativeID: "grok-1", CanDelete: true, DeleteMode: "grok", SourcePath: filepath.Join(root, "grok", "sessions", "proj", "grok-1", "summary.json")}); err == nil {
		t.Fatal("deleted a grok session through a symlink")
	}
	if _, err := os.Stat(outSummary); err != nil {
		t.Fatal(err)
	}

	gem := filepath.Join(root, "gemini", "tmp", "demo", "chats")
	if err := os.MkdirAll(gem, 0o755); err != nil {
		t.Fatal(err)
	}
	chat := filepath.Join(gem, "session-drop.json")
	other := filepath.Join(gem, "session-keep.json")
	projects := filepath.Join(root, "gemini", "projects.json")
	if err := os.WriteFile(chat, []byte(`{"sessionId":"drop","messages":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, []byte(`{"sessionId":"keep","messages":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projects, []byte(`{"projects":{"/work/demo":"demo"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Delete(model.Session{Agent: "gemini", NativeID: "drop", CanDelete: true, DeleteMode: "gemini", SourcePath: chat}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(chat); !os.IsNotExist(err) {
		t.Fatal("gemini chat still exists")
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(projects); err != nil {
		t.Fatal(err)
	}
	if err := Delete(model.Session{Agent: "gemini", NativeID: "keep", CanDelete: true, DeleteMode: "gemini", SourcePath: other}); err != nil {
		t.Fatal(err)
	}
	mismatch := filepath.Join(gem, "session-nope.json")
	if err := os.WriteFile(mismatch, []byte(`{"sessionId":"other"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Delete(model.Session{Agent: "gemini", NativeID: "nope", CanDelete: true, DeleteMode: "gemini", SourcePath: mismatch}); err == nil {
		t.Fatal("deleted a gemini chat whose session id did not match")
	}
	linked := filepath.Join(gem, "session-link.json")
	if err := os.Symlink(projects, linked); err != nil {
		t.Fatal(err)
	}
	if err := Delete(model.Session{Agent: "gemini", NativeID: "link", CanDelete: true, DeleteMode: "gemini", SourcePath: linked}); err == nil {
		t.Fatal("deleted a symlinked gemini chat")
	}
	if _, err := os.Stat(projects); err != nil {
		t.Fatal(err)
	}

	julesHome := filepath.Join(root, "jules")
	if err := os.MkdirAll(julesHome, 0o755); err != nil {
		t.Fatal(err)
	}
	list := filepath.Join(julesHome, "sessions.json")
	if err := os.WriteFile(list, []byte(`[{"id":"12345","title":"drop"},{"id":"99999","title":"stay"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Delete(model.Session{Agent: "jules", NativeID: "12345", CanDelete: true, DeleteMode: "jules", SourcePath: list}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(list)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "12345") || !strings.Contains(string(got), "99999") {
		t.Fatalf("jules list %s", got)
	}
	text := filepath.Join(julesHome, "sessions.txt")
	if err := os.WriteFile(text, []byte("ID TITLE\n4242 drop\n7777 stay\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Delete(model.Session{Agent: "jules", NativeID: "4242", CanDelete: true, DeleteMode: "jules", SourcePath: text}); err != nil {
		t.Fatal(err)
	}
	got, err = os.ReadFile(text)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "4242") || !strings.Contains(string(got), "7777") || !strings.Contains(string(got), "ID") {
		t.Fatalf("jules text %s", got)
	}
	if err := Delete(model.Session{Agent: "jules", NativeID: "12345", CanDelete: true, DeleteMode: "jules", SourcePath: "jules:remote"}); err == nil {
		t.Fatal("remote jules delete ran without an API key")
	}
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Method != http.MethodDelete || r.URL.Path != "/v1alpha/sessions/12345" || r.Header.Get("X-Goog-Api-Key") != "test-key" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	prev := julesAPIBase
	julesAPIBase = srv.URL + "/v1alpha"
	t.Cleanup(func() { julesAPIBase = prev })
	t.Setenv("JULES_API_KEY", "test-key")
	if err := Delete(model.Session{Agent: "jules", NativeID: "12345", CanDelete: true, DeleteMode: "jules", SourcePath: "jules:remote"}); err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Fatalf("jules api hits %d", hits)
	}

	dbPath := filepath.Join(root, "kiro.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	stmts := []string{
		`CREATE TABLE conversations_v2 (conversation_id TEXT, key TEXT, value TEXT, updated_at INTEGER, created_at INTEGER)`,
		`CREATE TABLE conversations (key TEXT, value TEXT)`,
		`CREATE TABLE history (command TEXT)`,
		`INSERT INTO conversations_v2 VALUES ('conv-1','k1','{"conversation_id":"conv-1"}',1,1)`,
		`INSERT INTO conversations_v2 VALUES ('conv-2','k2','{"note":"conv-1 stays"}',1,1)`,
		`INSERT INTO conversations VALUES ('k1','{"conversation_id":"conv-1"}')`,
		`INSERT INTO conversations VALUES ('k2','{"conversation_id":"conv-2"}')`,
		`INSERT INTO history VALUES ('echo kept')`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := Delete(model.Session{Agent: "kiro", NativeID: "conv-1", CanDelete: true, DeleteMode: "kiro", SourcePath: dbPath}); err != nil {
		t.Fatal(err)
	}
	db, err = sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM conversations_v2 WHERE conversation_id = 'conv-1'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("conv-1 rows %d %v", n, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM conversations_v2 WHERE conversation_id = 'conv-2'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("conv-2 rows %d %v", n, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM conversations WHERE key = 'k1'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("v1 k1 rows %d %v", n, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM conversations WHERE key = 'k2'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("v1 k2 rows %d %v", n, err)
	}
	var cmd string
	if err := db.QueryRow(`SELECT command FROM history`).Scan(&cmd); err != nil || cmd != "echo kept" {
		t.Fatalf("history %q %v", cmd, err)
	}
	if err := Delete(model.Session{Agent: "kiro", NativeID: "missing", CanDelete: true, DeleteMode: "kiro", SourcePath: dbPath}); err == nil {
		t.Fatal("deleted a kiro conversation that was not in the database")
	}
	otherDB := filepath.Join(t.TempDir(), "data.sqlite3")
	if err := os.WriteFile(otherDB, []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Delete(model.Session{Agent: "kiro", NativeID: "conv-2", CanDelete: true, DeleteMode: "kiro", SourcePath: otherDB}); err == nil {
		t.Fatal("deleted from a database that is not the Kiro database")
	}
}

func TestPlanRejectsFlagArgs(t *testing.T) {
	orig := LookPath
	t.Cleanup(func() { LookPath = orig })
	LookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	dir := t.TempDir()
	cases := []model.Session{
		{Agent: "claude", NativeID: "--dangerously-skip-permissions", CWD: dir},
		{Agent: "hermes", NativeID: "p:--yolo:abc", CWD: dir},
		{Agent: "gemini", NativeID: "abc", SourcePath: "--session-file", CWD: dir},
		{Agent: "claude", NativeID: "abc\n--yolo", CWD: dir},
		{Agent: "claude", NativeID: "abc\x1b[31m", CWD: dir},
	}
	for _, sess := range cases {
		if _, err := Plan(sess, false); err == nil {
			t.Fatalf("%s accepted %q", sess.Agent, sess.NativeID)
		}
	}
	if err := userArg("session id", "abc\n"); err == nil || !strings.Contains(err.Error(), "cannot be passed") {
		t.Fatalf("newline %v", err)
	}
	if err := userArg("session id", "ok"); err != nil {
		t.Fatal(err)
	}
}

func TestWriteAtomDoesNotFollowSymlink(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "secret")
	if err := os.WriteFile(secret, []byte("SECRET"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "sessions.json")
	if err := os.WriteFile(target, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, target+".ai9s.tmp"); err != nil {
		t.Fatal(err)
	}
	if err := writeAtom(target, []byte("new")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(secret)
	if err != nil || string(got) != "SECRET" {
		t.Fatalf("secret %q %v", got, err)
	}
	got, err = os.ReadFile(target)
	if err != nil || string(got) != "new" {
		t.Fatalf("target %q %v", got, err)
	}
	st, err := os.Lstat(target)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode()&os.ModeSymlink != 0 || st.Mode().Perm() != before.Mode().Perm() {
		t.Fatalf("mode %v before %v", st.Mode().Perm(), before.Mode().Perm())
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(secret, link); err != nil {
		t.Fatal(err)
	}
	if err := writeAtom(link, []byte("pwn")); err == nil {
		t.Fatal("rewrote a symlink")
	}
	got, err = os.ReadFile(secret)
	if err != nil || string(got) != "SECRET" {
		t.Fatalf("secret after symlink %q %v", got, err)
	}
}

func TestDeleteSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", root)
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.jsonl")
	if err := os.WriteFile(target, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "projects", "demo")); err != nil {
		t.Fatal(err)
	}
	via := filepath.Join(root, "projects", "demo", "secret.jsonl")
	sess := model.Session{Agent: "claude", NativeID: "secret", CanDelete: true, DeleteMode: "file", SourcePath: via}
	if err := Delete(sess); err == nil {
		t.Fatal("deleted a transcript through a symlink directory")
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatal(err)
	}

	proj := filepath.Join(root, "projects", "real")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(proj, "abc.jsonl")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	sess.SourcePath = link
	if err := Delete(sess); err == nil {
		t.Fatal("deleted a symlinked transcript")
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GEMINI_HOME", root)
	dir := filepath.Join(root, "antigravity-cli")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	history := filepath.Join(dir, "history.jsonl")
	other := filepath.Join(outside, "history.jsonl")
	if err := os.WriteFile(other, []byte("{\"conversationId\":\"keep\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, history); err != nil {
		t.Fatal(err)
	}
	err := Delete(model.Session{Agent: "antigravity", NativeID: "keep", CanDelete: true, DeleteMode: "rewrite", SourcePath: history})
	if err == nil {
		t.Fatal("rewrote a symlinked Antigravity history")
	}
	body, err := os.ReadFile(other)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "keep") {
		t.Fatalf("outside history changed: %s", body)
	}
}

func TestDeleteExecDetached(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "opencode")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "go.mod"), []byte("module helper\n\ngo 1.25.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	helper := []byte(`package main
import (
	"fmt"
	"io"
	"os"
)
func main() {
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if len(b) != 0 {
		fmt.Fprintf(os.Stderr, "stdin %q\n", b)
		os.Exit(3)
	}
}
`)
	if err := os.WriteFile(filepath.Join(src, "main.go"), helper, 0o644); err != nil {
		t.Fatal(err)
	}
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = src
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build helper: %v\n%s", err, out)
	}

	unique, err := os.Open(writeTemp(t, dir, "secret-stdin\n"))
	if err != nil {
		t.Fatal(err)
	}
	origStdin := os.Stdin
	os.Stdin = unique
	t.Cleanup(func() {
		os.Stdin = origStdin
		unique.Close()
	})

	orig := LookPath
	t.Cleanup(func() { LookPath = orig })
	LookPath = func(string) (string, error) { return bin, nil }
	if err := Delete(model.Session{Agent: "opencode", NativeID: "ses1", CanDelete: true, DeleteMode: "exec"}); err != nil {
		t.Fatal(err)
	}

	if runtime.GOOS == "windows" {
		return
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip(err)
	}
	fail := filepath.Join(dir, "opencode-fail")
	failBody := "#!/bin/sh\necho refused >&2\necho also-stdout\nexit 3\n"
	if err := os.WriteFile(fail, []byte(failBody), 0o755); err != nil {
		t.Fatal(err)
	}
	LookPath = func(string) (string, error) { return fail, nil }
	err = Delete(model.Session{Agent: "opencode", NativeID: "ses1", CanDelete: true, DeleteMode: "exec"})
	if err == nil || !strings.Contains(err.Error(), "refused") || !strings.Contains(err.Error(), "also-stdout") {
		t.Fatalf("error %v", err)
	}
}

func writeTemp(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "stdin.txt")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTrimOutput(t *testing.T) {
	got := trimOutput([]byte("  " + strings.Repeat("x", 500) + "\n"))
	if len([]rune(got)) != 401 || !strings.HasSuffix(got, "…") {
		t.Fatalf("trimmed %d %q", len([]rune(got)), got[:20])
	}
}

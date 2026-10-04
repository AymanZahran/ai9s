package cmd

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/AymanZahran/ai9s/internal/model"
	"github.com/AymanZahran/ai9s/internal/store"
	_ "modernc.org/sqlite"
)

// bin is the ai9s built from this package. The test drives that binary the
// way a contributor would, against a temporary home, and never the machine's
// real session stores.
var bin string

func TestMain(m *testing.M) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Fprintln(os.Stderr, "caller")
		os.Exit(1)
	}
	dir, err := os.MkdirTemp("", "ai9s-bin")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)
	bin = filepath.Join(dir, "ai9s")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = filepath.Dir(filepath.Dir(file))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestCLIIntegration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("resume and exec-delete checks use a POSIX stub")
	}
	root := t.TempDir()
	stub := filepath.Join(root, "bin")
	logPath := filepath.Join(root, "stub.log")
	if err := os.MkdirAll(stub, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"claude", "hermes", "kiro-cli", "jules", "goose", "kimi", "qwen", "vibe", "mcode"} {
		writeStub(t, filepath.Join(stub, name))
	}
	seedFixtures(t, root)
	env := fixtureEnv(t, root, stub, logPath)

	stats := indexJSON(t, env)
	if stats.Sessions != 13 {
		t.Fatalf("indexed %d sessions, agents %+v, stderr would show warnings above", stats.Sessions, stats.Agents)
	}
	wantAgents := []string{"aider", "cline", "claude", "goose", "grok", "hermes", "jules", "junie", "kimi", "kiro", "minimax", "mistral", "qwen"}
	gotAgents := map[string]int{}
	for _, a := range stats.Agents {
		gotAgents[a.Agent] = a.Sessions
	}
	for _, name := range wantAgents {
		if gotAgents[name] != 1 {
			t.Fatalf("agent %s count %d in %+v", name, gotAgents[name], gotAgents)
		}
	}
	if log := readLog(t, logPath); strings.Contains(log, "jules") {
		t.Fatalf("index invoked jules:\n%s", log)
	}

	again := indexJSON(t, env)
	if again.Sessions != stats.Sessions || again.Messages != stats.Messages {
		t.Fatalf("second index changed %d/%d to %d/%d", stats.Sessions, stats.Messages, again.Sessions, again.Messages)
	}

	rows := searchJSON(t, env, "agent:claude", "ship")
	if len(rows) != 1 || rows[0].ID != "claude:abc" || rows[0].CWD != "/work/demo" || rows[0].Branch != "main" {
		t.Fatalf("claude search %+v", rows)
	}
	if rows[0].Snippets != nil {
		t.Fatalf("search included snippets %+v", rows[0].Snippets)
	}
	dirRows := searchJSON(t, env, "dir:demo")
	if len(dirRows) != 1 || dirRows[0].ID != "claude:abc" {
		t.Fatalf("dir search %+v", dirRows)
	}

	shown := showJSON(t, env, "claude:abc")
	if shown.Title != "ship the feature" || !shown.CanDelete || len(shown.Snippets) == 0 || shown.Snippets[0].Body != "ship the feature" {
		t.Fatalf("show claude %+v", shown)
	}
	kiro := showJSON(t, env, "kiro:conv-1")
	if !kiro.CanDelete || kiro.DeleteMode != "kiro" || kiro.CWD != "/work/kiro" || kiro.Title != "rename the button" {
		t.Fatalf("show kiro %+v", kiro)
	}
	kimiShown := showJSON(t, env, "kimi:k1")
	if kimiShown.Title != "plan the port" || kimiShown.CWD != "/work/kimi" || !kimiShown.CanDelete || kimiShown.DeleteMode != "kimi" {
		t.Fatalf("show kimi %+v", kimiShown)
	}
	qwenShown := showJSON(t, env, "qwen:0123456789abcdef0123456789abcdef")
	if qwenShown.Title != "tune the prompt" || qwenShown.CWD != "/work/qwen" || qwenShown.DeleteMode != "qwen" {
		t.Fatalf("show qwen %+v", qwenShown)
	}
	mistralShown := showJSON(t, env, "mistral:session_20260302_150405_abcd")
	if mistralShown.Title != "review the diff" || mistralShown.CWD != "/work/vibe" || mistralShown.Branch != "main" || mistralShown.DeleteMode != "mistral" {
		t.Fatalf("show mistral %+v", mistralShown)
	}
	minimaxShown := showJSON(t, env, "minimax:mm-1")
	if minimaxShown.Title != "draft the note" || !minimaxShown.CanDelete || minimaxShown.DeleteMode != "minimax" {
		t.Fatalf("show minimax %+v", minimaxShown)
	}

	resume := run(t, env, "resume", "claude:abc", "--yolo", "--print")
	if resume.code != 0 || !strings.Contains(resume.stdout, "--dangerously-skip-permissions") || !strings.Contains(resume.stdout, "--resume abc") {
		t.Fatalf("claude resume code %d\n%s\n%s", resume.code, resume.stdout, resume.stderr)
	}
	kiroResume := run(t, env, "resume", "kiro:conv-1", "--yolo", "--print")
	if kiroResume.code != 0 || !strings.Contains(kiroResume.stdout, "kiro-cli") || !strings.Contains(kiroResume.stdout, "--trust-all-tools") || !strings.Contains(kiroResume.stdout, "--resume-id conv-1") {
		t.Fatalf("kiro resume code %d\n%s\n%s", kiroResume.code, kiroResume.stdout, kiroResume.stderr)
	}
	kimiResume := run(t, env, "resume", "kimi:k1", "--yolo", "--print")
	if kimiResume.code != 0 || !strings.Contains(kimiResume.stdout, "--session k1") || strings.Contains(kimiResume.stdout, "yolo") {
		t.Fatalf("kimi resume code %d\n%s\n%s", kimiResume.code, kimiResume.stdout, kimiResume.stderr)
	}
	qwenResume := run(t, env, "resume", "qwen:0123456789abcdef0123456789abcdef", "--yolo", "--print")
	if qwenResume.code != 0 || !strings.Contains(qwenResume.stdout, "--yolo") || !strings.Contains(qwenResume.stdout, "--resume 0123456789abcdef0123456789abcdef") {
		t.Fatalf("qwen resume code %d\n%s\n%s", qwenResume.code, qwenResume.stdout, qwenResume.stderr)
	}
	if log := readLog(t, logPath); strings.TrimSpace(log) != "" {
		t.Fatalf("resume --print executed a stub:\n%s", log)
	}

	bare := run(t, env)
	if bare.code == 0 || !strings.Contains(bare.stderr, "no terminal") {
		t.Fatalf("bare ai9s code %d stderr %s", bare.code, bare.stderr)
	}
	help := run(t, env, "help")
	if help.code != 0 || !strings.Contains(help.stderr, "ai9s search") {
		t.Fatalf("help code %d stderr %s", help.code, help.stderr)
	}

	claudePath := filepath.Join(root, "claude", "projects", "demo", "abc.jsonl")
	refused := run(t, env, "delete", "claude:abc")
	if refused.code == 0 || !strings.Contains(refused.stderr, "--yes") {
		t.Fatalf("delete without --yes code %d stderr %s", refused.code, refused.stderr)
	}
	if _, err := os.Stat(claudePath); err != nil {
		t.Fatal(err)
	}
	grokPath := filepath.Join(root, "grok", "sessions", "proj", "grok-1", "summary.json")
	grokShared := filepath.Join(root, "grok", "sessions", "proj", "prompt_history.jsonl")
	grokDel := run(t, env, "delete", "grok:grok-1", "--yes")
	if grokDel.code != 0 {
		t.Fatalf("grok delete code %d stderr %s", grokDel.code, grokDel.stderr)
	}
	if _, err := os.Stat(grokPath); !os.IsNotExist(err) {
		t.Fatal("grok session still exists")
	}
	if _, err := os.Stat(grokShared); err != nil {
		t.Fatal(err)
	}

	deleted := run(t, env, "delete", "claude:abc", "--yes")
	if deleted.code != 0 || !strings.Contains(deleted.stdout, "deleted claude:abc") {
		t.Fatalf("delete claude code %d\n%s\n%s", deleted.code, deleted.stdout, deleted.stderr)
	}
	if _, err := os.Stat(claudePath); !os.IsNotExist(err) {
		t.Fatal("claude transcript still exists")
	}
	if left := searchJSON(t, env, "agent:claude"); len(left) != 0 {
		t.Fatalf("claude still indexed %+v", left)
	}

	junieDir := filepath.Join(root, "junie", "sessions", "session-1")
	junieDel := run(t, env, "delete", "junie:session-1", "--yes")
	if junieDel.code != 0 {
		t.Fatalf("delete junie %s", junieDel.stderr)
	}
	if _, err := os.Stat(junieDir); !os.IsNotExist(err) {
		t.Fatal("junie session still exists")
	}

	hermesDel := run(t, env, "delete", "hermes:hs-1", "--yes")
	if hermesDel.code != 0 {
		t.Fatalf("delete hermes %s", hermesDel.stderr)
	}
	log := readLog(t, logPath)
	if !strings.Contains(log, "sessions delete hs-1 --yes") {
		t.Fatalf("hermes stub log:\n%s", log)
	}
	if left := searchJSON(t, env, "agent:hermes"); len(left) != 0 {
		t.Fatalf("hermes still indexed %+v", left)
	}

	kimiPath := filepath.Join(root, "kimi-code", "sessions", "wd_work_aaaaaaaaaaaa", "k1")
	kimiKeep := filepath.Join(root, "kimi-code", "sessions", "wd_work_aaaaaaaaaaaa", "keep.txt")
	kimiDel := run(t, env, "delete", "kimi:k1", "--yes")
	if kimiDel.code != 0 {
		t.Fatalf("delete kimi %s", kimiDel.stderr)
	}
	if _, err := os.Stat(kimiPath); !os.IsNotExist(err) {
		t.Fatal("kimi session still exists")
	}
	if _, err := os.Stat(kimiKeep); err != nil {
		t.Fatal(err)
	}
	qwenPath := filepath.Join(root, "qwen", "projects", "-work-qwen", "chats", "0123456789abcdef0123456789abcdef.jsonl")
	qwenKeep := filepath.Join(root, "qwen", "projects", "-work-qwen", "chats", ".runtime.json")
	qwenDel := run(t, env, "delete", "qwen:0123456789abcdef0123456789abcdef", "--yes")
	if qwenDel.code != 0 {
		t.Fatalf("delete qwen %s", qwenDel.stderr)
	}
	if _, err := os.Stat(qwenPath); !os.IsNotExist(err) {
		t.Fatal("qwen chat still exists")
	}
	if _, err := os.Stat(qwenKeep); err != nil {
		t.Fatal(err)
	}
	vibeDir := filepath.Join(root, "vibe", "logs", "session", "session_20260302_150405_abcd")
	vibeDel := run(t, env, "delete", "mistral:session_20260302_150405_abcd", "--yes")
	if vibeDel.code != 0 {
		t.Fatalf("delete mistral %s", vibeDel.stderr)
	}
	if _, err := os.Stat(vibeDir); !os.IsNotExist(err) {
		t.Fatal("mistral session still exists")
	}

	rebuilt := indexJSON(t, env)
	after := map[string]int{}
	for _, a := range rebuilt.Agents {
		after[a.Agent] = a.Sessions
	}
	if after["claude"] != 0 || after["junie"] != 0 || after["grok"] != 0 || after["hermes"] != 1 || after["jules"] != 1 || after["kiro"] != 1 || after["kimi"] != 0 || after["qwen"] != 0 || after["mistral"] != 0 || after["minimax"] != 1 {
		t.Fatalf("after reindex %+v", after)
	}
	if strings.Contains(readLog(t, logPath), "jules") {
		t.Fatal("jules was executed")
	}
}

func indexJSON(t *testing.T, env []string) store.Stats {
	t.Helper()
	res := run(t, env, "index", "--json")
	if res.code != 0 || strings.Contains(res.stderr, "warning:") {
		t.Fatalf("index code %d\n%s\n%s", res.code, res.stdout, res.stderr)
	}
	var stats store.Stats
	if err := json.Unmarshal([]byte(res.stdout), &stats); err != nil {
		t.Fatalf("index json: %v\n%s", err, res.stdout)
	}
	return stats
}

func searchJSON(t *testing.T, env []string, query ...string) []model.Session {
	t.Helper()
	args := append([]string{"search", "--json"}, query...)
	res := run(t, env, args...)
	if res.code != 0 {
		t.Fatalf("search %v code %d %s", query, res.code, res.stderr)
	}
	var rows []model.Session
	if err := json.Unmarshal([]byte(res.stdout), &rows); err != nil {
		t.Fatalf("search json: %v\n%s", err, res.stdout)
	}
	return rows
}

func showJSON(t *testing.T, env []string, id string) model.Session {
	t.Helper()
	res := run(t, env, "show", "--json", id)
	if res.code != 0 {
		t.Fatalf("show %s code %d %s", id, res.code, res.stderr)
	}
	var sess model.Session
	if err := json.Unmarshal([]byte(res.stdout), &sess); err != nil {
		t.Fatalf("show json: %v\n%s", err, res.stdout)
	}
	return sess
}

type cmdResult struct {
	stdout string
	stderr string
	code   int
}

func run(t *testing.T, env []string, args ...string) cmdResult {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = env
	// A nil stdin is /dev/null, a character device, so the child would pass
	// isTerm and open the TUI. A pipe is not a terminal.
	cmd.Stdin = bytes.NewReader(nil)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatal(err)
		}
		code = exit.ExitCode()
	}
	return cmdResult{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

func fixtureEnv(t *testing.T, root, stub, logPath string) []string {
	t.Helper()
	vals := map[string]string{
		"HOME":               root,
		"USERPROFILE":        root,
		"TMPDIR":             os.TempDir(),
		"PATH":               stub + string(os.PathListSeparator) + os.Getenv("PATH"),
		"TERM":               "xterm",
		"LANG":               "C.UTF-8",
		"AI9S_CACHE_DIR":     filepath.Join(root, "cache"),
		"AI9S_JULES_REMOTE":  "0",
		"AI9S_STUB_LOG":      logPath,
		"XDG_CACHE_HOME":     filepath.Join(root, "xdg", "cache"),
		"XDG_DATA_HOME":      filepath.Join(root, "xdg", "data"),
		"CLAUDE_CONFIG_DIR":  filepath.Join(root, "claude"),
		"CODEX_HOME":         filepath.Join(root, "codex"),
		"GROK_HOME":          filepath.Join(root, "grok"),
		"COPILOT_HOME":       filepath.Join(root, "copilot"),
		"GEMINI_HOME":        filepath.Join(root, "gemini"),
		"CURSOR_HOME":        filepath.Join(root, "cursor"),
		"OPENCODE_DB":        filepath.Join(root, "missing-opencode.db"),
		"HERMES_HOME":        filepath.Join(root, "hermes"),
		"OPENCLAW_HOME":      filepath.Join(root, "openclaw"),
		"OPENCLAW_STATE_DIR": filepath.Join(root, "openclaw"),
		"JUNIE_HOME":         filepath.Join(root, "junie"),
		"JULES_HOME":         filepath.Join(root, "jules"),
		"GOOSE_HOME":         filepath.Join(root, "goose"),
		"CLINE_HOME":         filepath.Join(root, "cline"),
		"KIRO_CLI_DB":        filepath.Join(root, "kiro.db"),
		"KIRO_HOME":          filepath.Join(root, "kiro"),
		"AIDER_HOME":         filepath.Join(root, "aider"),
		"AIDER_CHAT_ROOTS":   filepath.Join(root, "aider-roots"),
		"AIDER_CHAT_HISTORY": filepath.Join(root, "no-aider.md"),
		"AIDER_SCAN_HOME":    "0",
		"KIMI_CODE_HOME":     filepath.Join(root, "kimi-code"),
		"QWEN_RUNTIME_DIR":   filepath.Join(root, "qwen"),
		"QWEN_HOME":          filepath.Join(root, "qwen-home"),
		"VIBE_HOME":          filepath.Join(root, "vibe"),
		"MINIMAX_DATA_DIR":   filepath.Join(root, "minimax"),
		"MAVIS_DATA_DIR":     filepath.Join(root, "minimax-mavis"),
	}
	env := make([]string, 0, len(vals))
	for k, v := range vals {
		env = append(env, k+"="+v)
	}
	return env
}

func writeStub(t *testing.T, path string) {
	t.Helper()
	body := "#!/bin/sh\nprintf '%s' \"$0\" >> \"$AI9S_STUB_LOG\"\nfor a in \"$@\"; do printf ' %s' \"$a\" >> \"$AI9S_STUB_LOG\"; done\nprintf '\\n' >> \"$AI9S_STUB_LOG\"\nexit 0\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func readLog(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatal(err)
	}
	return string(body)
}

func seedFixtures(t *testing.T, root string) {
	t.Helper()
	writeFile(t, filepath.Join(root, "claude", "projects", "demo", "abc.jsonl"),
		`{"type":"user","timestamp":"2026-03-02T15:04:05Z","cwd":"/work/demo","gitBranch":"main","message":{"content":"ship the feature"}}`+"\n")
	writeFile(t, filepath.Join(root, "grok", "sessions", "proj", "grok-1", "summary.json"),
		`{"info":{"id":"grok-1","cwd":"/work/grok"},"session_summary":"plan the release","current_model_id":"grok-4","head_branch":"main","updated_at":"2026-03-02T15:04:05Z","reasoning_effort":"high"}`)
	writeFile(t, filepath.Join(root, "grok", "sessions", "proj", "prompt_history.jsonl"), "{}\n")
	writeFile(t, filepath.Join(root, "junie", "sessions", "session-1", "transcript.md"),
		"# Session transcript\nuser: tidy the docs\n")
	writeFile(t, filepath.Join(root, "jules", "sessions.json"),
		`[{"id":"4242","title":"cloud task","repo":"owner/repo"}]`)
	writeFile(t, filepath.Join(root, "cline", "data", "state", "taskHistory.json"),
		`[{"id":"task-1","task":"wire the button","cwdOnTaskInitialization":"/work/cline","tokensIn":8,"tokensOut":2}]`)
	writeFile(t, filepath.Join(root, "cline", "data", "tasks", "task-1", "api_conversation_history.json"),
		`[{"role":"user","content":"wire the button"}]`)
	writeFile(t, filepath.Join(root, "aider", "proj", ".aider.chat.history.md"),
		"#### user\nexplain the makefile\n#### assistant\nit builds\n")

	execDB(t, filepath.Join(root, "hermes", "state.db"),
		`CREATE TABLE sessions (
			id TEXT, source TEXT, model TEXT, title TEXT, cwd TEXT, git_branch TEXT,
			message_count INTEGER, input_tokens INTEGER, output_tokens INTEGER,
			cache_read_tokens INTEGER, cache_write_tokens INTEGER, reasoning_tokens INTEGER,
			actual_cost_usd REAL, estimated_cost_usd REAL,
			started_at REAL, last_activity_at REAL, ended_at REAL,
			hidden INTEGER, archived INTEGER)`,
		`CREATE TABLE messages (id INTEGER PRIMARY KEY, session_id TEXT, role TEXT, content TEXT, active INTEGER)`,
		`INSERT INTO sessions VALUES ('hs-1','cli','opus','profile the cache','/work/hermes','main',1,4,1,0,0,0,0,0,0,1700000000,0,0,0)`,
		`INSERT INTO messages VALUES (1,'hs-1','user','profile the cache',1)`,
	)
	execDB(t, filepath.Join(root, "goose", "sessions", "sessions.db"),
		`CREATE TABLE sessions (
			id TEXT, name TEXT, description TEXT, working_dir TEXT,
			updated_at TEXT, created_at TEXT,
			input_tokens INTEGER, output_tokens INTEGER,
			cache_read_tokens INTEGER, cache_write_tokens INTEGER,
			accumulated_input_tokens INTEGER, accumulated_output_tokens INTEGER,
			accumulated_cache_read_tokens INTEGER, accumulated_cache_write_tokens INTEGER,
			accumulated_cost REAL, model_config_json TEXT, archived_at TEXT)`,
		`CREATE TABLE messages (id INTEGER PRIMARY KEY, session_id TEXT, role TEXT, content_json TEXT)`,
		`INSERT INTO sessions VALUES ('g1','Goose task','','/work/goose','2026-01-02 03:04:05','2026-01-02 03:04:05',0,0,0,0,12,3,0,0,0.5,'{"model_name":"gpt"}',NULL)`,
	)
	value := `{"history":[[{"content":{"Prompt":"rename the button"},"env_context":{"env_state":{"current_working_directory":"/work/kiro"}}}]]}`
	execDB(t, filepath.Join(root, "kiro.db"),
		`CREATE TABLE conversations_v2 (conversation_id TEXT, key TEXT, value TEXT, updated_at INTEGER, created_at INTEGER)`,
		`CREATE TABLE history (command TEXT)`,
		`INSERT INTO history VALUES ('echo secret')`,
		`INSERT INTO conversations_v2 VALUES ('conv-1','k1','`+value+`',1767225600000,1767225600000)`,
	)

	kimiDir := filepath.Join(root, "kimi-code", "sessions", "wd_work_aaaaaaaaaaaa", "k1")
	writeFile(t, filepath.Join(kimiDir, "state.json"), `{"id":"k1","title":"plan the port","cwd":"/work/kimi","createdAt":1700000000000,"updatedAt":1700000000000}`)
	writeFile(t, filepath.Join(kimiDir, "agents", "main", "wire.jsonl"), `{"type":"context.append_message","message":{"role":"user","content":"plan the port"}}`+"\n")
	writeFile(t, filepath.Join(root, "kimi-code", "sessions", "wd_work_aaaaaaaaaaaa", "keep.txt"), "stay")
	kimiLine, err := json.Marshal(map[string]string{"sessionId": "k1", "sessionDir": kimiDir, "workDir": "/work/kimi"})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "kimi-code", "session_index.jsonl"), string(kimiLine)+"\n")

	qwenID := "0123456789abcdef0123456789abcdef"
	writeFile(t, filepath.Join(root, "qwen", "projects", "-work-qwen", "chats", qwenID+".jsonl"),
		`{"type":"user","timestamp":"2026-03-02T15:04:05Z","cwd":"/work/qwen","message":{"role":"user","parts":[{"text":"tune the prompt"}]}}`+"\n"+
			`{"type":"system","subtype":"custom_title","systemPayload":{"customTitle":"tune the prompt"}}`+"\n")
	writeFile(t, filepath.Join(root, "qwen", "projects", "-work-qwen", "chats", ".runtime.json"), "{}")

	vibeDir := filepath.Join(root, "vibe", "logs", "session", "session_20260302_150405_abcd")
	writeFile(t, filepath.Join(vibeDir, "meta.json"), `{"session_id":"session_20260302_150405_abcd","title":"review the diff","git_branch":"main","environment":{"working_directory":"/work/vibe"}}`)
	writeFile(t, filepath.Join(vibeDir, "messages.jsonl"), `{"role":"user","content":"review the diff"}`+"\n")

	mmDir := filepath.Join(root, "minimax", "v2", "sessions", "aa", "bb", "cc", "mm1")
	writeFile(t, filepath.Join(mmDir, "manifest.json"), `{"sessionId":"mm-1","updatedAtMs":1700000000000}`)
	writeFile(t, filepath.Join(mmDir, "messages.jsonl"), `{"message":{"role":"user","content":"draft the note"}}`+"\n")
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func execDB(t *testing.T, path string, stmts ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
}

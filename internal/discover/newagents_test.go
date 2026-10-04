package discover

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestScanKimi(t *testing.T) {
	root := isolate(t)
	home := filepath.Join(root, "kimi-code")
	bucket := filepath.Join(home, "sessions", "wd_work_aaaaaaaaaaaa")
	dir := filepath.Join(bucket, "k1")
	writeBytes(t, filepath.Join(dir, "state.json"), []byte(`{"id":"k1","title":"plan the port","cwd":"/work/kimi","createdAt":1700000000000,"updatedAt":1700000000000,"archived":false}`))
	writeBytes(t, filepath.Join(dir, "agents", "main", "wire.jsonl"), []byte(
		`{"type":"metadata"}`+"\n"+
			`{"type":"context.append_message","message":{"role":"user","content":"plan the port"}}`+"\n"+
			`{"type":"token_counting.measured","tokens":1234}`+"\n"+
			`{"type":"tools.update_store"}`+"\n"))
	writeBytes(t, filepath.Join(bucket, "k2", "state.json"), []byte(`{"id":"k2","lastPrompt":"walked session","cwd":"/work/kimi-walk","updatedAt":1700000001000}`))
	writeBytes(t, filepath.Join(bucket, "keep.txt"), []byte("stay"))
	outside := filepath.Join(root, "outside", "evil")
	writeBytes(t, filepath.Join(outside, "state.json"), []byte(`{"id":"evil","title":"escaped","cwd":"/etc","updatedAt":1700000000000}`))
	index := []map[string]string{
		{"sessionId": "k1", "sessionDir": dir, "workDir": "/work/kimi"},
		{"sessionId": "evil", "sessionDir": outside, "workDir": "/etc"},
		{"sessionId": "rel", "sessionDir": filepath.Join("..", "..", "outside", "evil"), "workDir": "/etc"},
	}
	var lines []byte
	for _, row := range index {
		body, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, body...)
		lines = append(lines, '\n')
	}
	writeBytes(t, filepath.Join(home, "session_index.jsonl"), lines)

	b := scanKimi(nil)
	if b.Err != nil {
		t.Fatal(b.Err)
	}
	got := map[string]modelSession{}
	for _, s := range b.Sessions {
		got[s.NativeID] = modelSession{title: s.Title, cwd: s.CWD, mode: s.DeleteMode, context: s.Usage.Context, total: s.Usage.Total, msgs: s.Messages, del: s.CanDelete}
	}
	k1, ok := got["k1"]
	if !ok || k1.title != "plan the port" || k1.cwd != "/work/kimi" || k1.mode != "kimi" || !k1.del || k1.context != 1234 || k1.total != 0 || k1.msgs != 1 {
		t.Fatalf("k1 %+v", got["k1"])
	}
	k2 := got["k2"]
	if k2.title != "walked session" || k2.cwd != "/work/kimi-walk" {
		t.Fatalf("k2 %+v", got)
	}
	if _, bad := got["evil"]; bad || len(got) != 2 {
		t.Fatalf("sessions %+v", got)
	}
	if _, err := os.Stat(filepath.Join(bucket, "keep.txt")); err != nil {
		t.Fatal(err)
	}
}

type modelSession struct {
	title, cwd, mode string
	context, total   int
	msgs             int
	del              bool
}

func TestScanQwen(t *testing.T) {
	root := isolate(t)
	id := "0123456789abcdef0123456789abcdef"
	parent := "abcdefabcdefabcdefabcdefabcdefab"
	chats := filepath.Join(root, "qwen", "projects", "-work-qwen", "chats")
	body := `{"type":"user","timestamp":"2026-03-02T15:04:05Z","cwd":"/work/qwen","gitBranch":"main","message":{"role":"user","parts":[{"text":"tune the prompt"}]}}` + "\n" +
		`{"type":"assistant","timestamp":"2026-03-02T15:05:05Z","model":"qwen3","contextWindowSize":32768,"usageMetadata":{"promptTokenCount":20,"candidatesTokenCount":5,"totalTokenCount":99,"cachedContentTokenCount":3,"thoughtsTokenCount":1},"message":{"role":"assistant","parts":[{"text":"tuned"}]}}` + "\n" +
		`{"type":"system","subtype":"custom_title","systemPayload":{"customTitle":"tune the prompt"}}` + "\n"
	writeBytes(t, filepath.Join(chats, id+".jsonl"), []byte(body))
	writeBytes(t, filepath.Join(chats, "archive", id+".jsonl"), []byte(`{"type":"system","subtype":"custom_title","customTitle":"from archive"}`+"\n"))
	writeBytes(t, filepath.Join(chats, parent+".jsonl"), []byte(`{"type":"system","subtype":"parent_session"}`+"\n"+`{"type":"user","message":{"content":"child prompt"}}`+"\n"))
	writeBytes(t, filepath.Join(chats, ".runtime.json"), []byte(`{}`))
	writeBytes(t, filepath.Join(root, "qwen-home", "projects", "-work-qwen", "chats", id+".jsonl"), []byte(`{"type":"user","message":{"content":"wrong home"}}`+"\n"))

	b := scanQwen(nil)
	if b.Err != nil || len(b.Sessions) != 1 {
		t.Fatalf("qwen %+v %v", b.Sessions, b.Err)
	}
	s := b.Sessions[0]
	if s.NativeID != id || s.Title != "tune the prompt" || s.CWD != "/work/qwen" || s.Branch != "main" || s.Model != "qwen3" || s.DeleteMode != "qwen" || !s.CanDelete {
		t.Fatalf("session %+v", s)
	}
	if s.Usage.Input != 0 || s.Usage.Output != 5 || s.Usage.Total != 0 || s.Usage.Context != 20 || s.Usage.Window != 32768 || s.Usage.CacheRead != 3 || s.Usage.Reasoning != 1 || s.Messages != 2 {
		t.Fatalf("usage %+v messages %d", s.Usage, s.Messages)
	}
}

func TestScanMistral(t *testing.T) {
	root := isolate(t)
	dir := filepath.Join(root, "vibe", "logs", "session", "session_20260302_150405_abcd")
	writeBytes(t, filepath.Join(dir, "meta.json"), []byte(`{"session_id":"session_20260302_150405_abcd","title":"review the diff","git_branch":"main","environment":{"working_directory":"/work/vibe"},"config":{"active_model":"mistral-large"},"start_time":"2026-03-02T15:04:05Z","bumped_at":"2026-03-02T15:05:05Z"}`))
	writeBytes(t, filepath.Join(dir, "messages.jsonl"), []byte(`{"role":"user","content":"review the diff"}`+"\n"+`{"role":"assistant","content":"looks good"}`+"\n"))
	child := filepath.Join(root, "vibe", "logs", "session", "child")
	writeBytes(t, filepath.Join(child, "meta.json"), []byte(`{"session_id":"child-1","parent_session_id":"session_20260302_150405_abcd","title":"child session"}`))
	writeBytes(t, filepath.Join(child, "messages.jsonl"), []byte(`{"role":"user","content":"child session"}`+"\n"))

	b := scanMistral(nil)
	if b.Err != nil || len(b.Sessions) != 1 {
		t.Fatalf("mistral %+v %v", b.Sessions, b.Err)
	}
	s := b.Sessions[0]
	if s.NativeID != "session_20260302_150405_abcd" || s.Title != "review the diff" || s.CWD != "/work/vibe" || s.Branch != "main" || s.Model != "mistral-large" || s.DeleteMode != "mistral" || !s.CanDelete || s.Messages != 2 || !s.Usage.Empty() {
		t.Fatalf("session %+v", s)
	}
	if filepath.Base(s.SourcePath) != "messages.jsonl" {
		t.Fatalf("source %s", s.SourcePath)
	}
}

func TestScanMistralSaveDir(t *testing.T) {
	root := isolate(t)
	home := filepath.Join(root, "vibe")
	writeBytes(t, filepath.Join(home, "config.toml"), []byte("[session_logging]\nsave_dir = \"custom/sessions\"\n"))
	custom := filepath.Join(home, "custom", "sessions", "kept")
	writeBytes(t, filepath.Join(custom, "meta.json"), []byte(`{"session_id":"kept","title":"custom dir"}`))
	writeBytes(t, filepath.Join(custom, "messages.jsonl"), []byte(`{"role":"user","content":"custom dir"}`+"\n"))
	decoy := filepath.Join(home, "logs", "session", "decoy")
	writeBytes(t, filepath.Join(decoy, "meta.json"), []byte(`{"session_id":"decoy","title":"default dir"}`))
	writeBytes(t, filepath.Join(decoy, "messages.jsonl"), []byte(`{"role":"user","content":"default dir"}`+"\n"))

	b := scanMistral(nil)
	if b.Err != nil || len(b.Sessions) != 1 || b.Sessions[0].NativeID != "kept" {
		t.Fatalf("save_dir %+v %v", b.Sessions, b.Err)
	}

	unsafe := filepath.VolumeName(home) + string(os.PathSeparator)
	writeBytes(t, filepath.Join(home, "config.toml"), []byte("[session_logging]\nsave_dir = '"+unsafe+"'\n"))
	b = scanMistral(nil)
	if b.Err != nil || len(b.Sessions) != 1 || b.Sessions[0].NativeID != "decoy" {
		t.Fatalf("unsafe save_dir %+v %v", b.Sessions, b.Err)
	}
}

func TestScanMinimaxDB(t *testing.T) {
	root := isolate(t)
	record := `{"effectiveModel":"fallback-model","effectiveModelContextWindow":128000,"title":"from record","workspaceDir":"/work/from-record","runLocation":{"resolvedBranch":"feat","resolvedDir":"/work/resolved"}}`
	dbPath := filepath.Join(root, "minimax", "v2", "sqlite", "runtime-state.sqlite")
	execDB(t, dbPath,
		`CREATE TABLE local_runtime_sessions (
			session_id TEXT PRIMARY KEY, record_json TEXT, updated_at_ms INTEGER, created_at_ms INTEGER,
			visibility TEXT, session_kind TEXT, parent_session_id TEXT, workspace_dir TEXT, title TEXT,
			history_relative_dir TEXT)`,
		`CREATE TABLE local_runtime_token_usage (
			id INTEGER PRIMARY KEY, session_id TEXT, model TEXT, ts INTEGER,
			input_tokens INTEGER, output_tokens INTEGER, reasoning_tokens INTEGER,
			cache_read_tokens INTEGER, cache_write_tokens INTEGER, cost_usd REAL)`,
		`INSERT INTO local_runtime_sessions VALUES ('mm-1','`+record+`',1700000000000,1700000000000,'visible','conversation','','/work/minimax','ship the minimax','aa/bb/cc/mm1')`,
		`INSERT INTO local_runtime_sessions VALUES ('hid','{}',1700000000000,1700000000000,'hidden','conversation','','/work/minimax','hidden one','aa/bb/cc/hid1')`,
		`INSERT INTO local_runtime_sessions VALUES ('cron-1','{}',1700000000000,1700000000000,'visible','cron','','/work/minimax','cron one','aa/bb/cc/cron')`,
		`INSERT INTO local_runtime_sessions VALUES ('child','{}',1700000000000,1700000000000,'visible','conversation','mm-1','/work/minimax','child one','aa/bb/cc/chld')`,
		`INSERT INTO local_runtime_token_usage VALUES (1,'mm-1','old',1,11,3,1,0,0,0.25)`,
		`INSERT INTO local_runtime_token_usage VALUES (2,'mm-1','MiniMax-M2',2,7,1,1,1,1,0.25)`,
	)
	msgs := filepath.Join(root, "minimax", "v2", "sessions", "aa", "bb", "cc", "mm1", "messages.jsonl")
	writeBytes(t, msgs, []byte(
		`{"message":{"role":"user","content":"plan the port"}}`+"\n"+
			`{"message":{"role":"compactionSummary","content":"secret"}}`+"\n"+
			`{"message":{"role":"tool","content":"ran"}}`+"\n"+
			`{"message":{"role":"assistant","content":"done","model":"file-model","usage":{"input":9,"output":1,"cost":{"total":9}}}}`+"\n"))
	other := filepath.Join(root, "minimax", "v2", "sessions", "aa", "bb", "cc", "othr")
	writeBytes(t, filepath.Join(other, "manifest.json"), []byte(`{"sessionId":"other-1"}`))
	writeBytes(t, filepath.Join(other, "messages.jsonl"), []byte(`{"message":{"role":"user","content":"manifest only"}}`+"\n"))
	writeBytes(t, filepath.Join(root, "minimax-mavis", "v2", "sessions", "aa", "bb", "cc", "mav1", "manifest.json"), []byte(`{"sessionId":"mavis-1"}`))
	writeBytes(t, filepath.Join(root, "minimax-mavis", "v2", "sessions", "aa", "bb", "cc", "mav1", "messages.jsonl"), []byte(`{"message":{"role":"user","content":"mavis"}}`+"\n"))

	b := scanMinimax(nil)
	if b.Err != nil || len(b.Sessions) != 1 {
		t.Fatalf("minimax %+v %v", b.Sessions, b.Err)
	}
	s := b.Sessions[0]
	if s.NativeID != "mm-1" || s.Title != "ship the minimax" || s.CWD != "/work/minimax" || s.Branch != "feat" || s.Model != "MiniMax-M2" || s.DeleteMode != "minimax" || !s.CanDelete || s.SourcePath != dbPath {
		t.Fatalf("session %+v", s)
	}
	if s.Usage.Input != 18 || s.Usage.Output != 4 || s.Usage.CostUSD != 0.5 || s.Usage.Context != 7 || s.Usage.Window != 128000 || s.Usage.Reasoning != 2 || s.Usage.CacheRead != 1 || s.Usage.CacheWrite != 1 || s.Usage.Effort != "" || s.Messages != 2 {
		t.Fatalf("usage %+v messages %d", s.Usage, s.Messages)
	}
}

func TestScanMinimaxManifest(t *testing.T) {
	root := isolate(t)
	dir := filepath.Join(root, "minimax", "v2", "sessions", "aa", "bb", "cc", "mm1")
	writeBytes(t, filepath.Join(dir, "manifest.json"), []byte(`{"sessionId":"mm-1","updatedAtMs":1700000000000}`))
	msgs := filepath.Join(dir, "messages.jsonl")
	writeBytes(t, msgs, []byte(
		`{"message":{"role":"user","content":"draft the note"}}`+"\n"+
			`{"message":{"role":"assistant","model":"MiniMax-Text","content":"drafted","usage":{"input":4,"output":2,"cost":{"total":0.25}}}}`+"\n"))
	b := scanMinimax(nil)
	if b.Err != nil || len(b.Sessions) != 1 {
		t.Fatalf("manifest %+v %v", b.Sessions, b.Err)
	}
	s := b.Sessions[0]
	if s.NativeID != "mm-1" || s.Title != "draft the note" || s.Model != "MiniMax-Text" || s.SourcePath != msgs || s.DeleteMode != "minimax" {
		t.Fatalf("session %+v", s)
	}
	if s.Usage.Input != 4 || s.Usage.Output != 2 || s.Usage.CostUSD != 0.25 || s.Usage.Context != 4 {
		t.Fatalf("usage %+v", s.Usage)
	}
}

func TestScanMinimaxCorruptDB(t *testing.T) {
	root := isolate(t)
	writeBytes(t, filepath.Join(root, "minimax", "v2", "sqlite", "runtime-state.sqlite"), []byte("not a database"))
	dir := filepath.Join(root, "minimax", "v2", "sessions", "aa", "bb", "cc", "mm1")
	writeBytes(t, filepath.Join(dir, "manifest.json"), []byte(`{"sessionId":"mm-1"}`))
	writeBytes(t, filepath.Join(dir, "messages.jsonl"), []byte(`{"message":{"role":"user","content":"draft the note"}}`+"\n"))
	b := scanMinimax(nil)
	if b.Err == nil || len(b.Sessions) != 0 {
		t.Fatalf("corrupt %+v %v", b.Sessions, b.Err)
	}
}

func writeBytes(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

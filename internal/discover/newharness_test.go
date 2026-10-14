package discover

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/AymanZahran/ai9s/internal/model"
)

func TestScanPiAndOmp(t *testing.T) {
	root := isolate(t)
	piDir := filepath.Join(root, "pi-real")
	t.Setenv("PI_CODING_AGENT_SESSION_DIR", piDir)
	writeBytes(t, filepath.Join(piDir, "proj", "2026.jsonl"), []byte(
		`{"type":"session","id":"pi-1","timestamp":"2026-01-02T03:04:05Z","cwd":"/work/pi"}`+"\n"+
			`{"type":"model_change","provider":"openai","modelId":"gpt-test"}`+"\n"+
			`{"type":"message","message":{"role":"user","content":"plan the port"}}`+"\n"+
			`{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":"done"}],"usage":{"input":6,"output":2,"cacheRead":1,"cacheWrite":1,"reasoning":1,"cost":{"total":0.25}}}}`+"\n"))
	writeBytes(t, filepath.Join(piDir, "proj", ".hidden.jsonl"), []byte(`{"type":"session","id":"hidden"}`+"\n"))
	writeBytes(t, filepath.Join(piDir, "proj", "lock.jsonl.lock"), []byte("{}\n"))
	writeBytes(t, filepath.Join(piDir, "proj", "nested", "deep.jsonl"), []byte(`{"type":"session","id":"deep"}`+"\n"))

	fresh := scanPi(func(string, int64) bool { return true })
	if fresh.Err != nil || len(fresh.Sessions) != 0 || len(fresh.Files) != 1 || !fresh.Files[0].Fresh {
		t.Fatalf("fresh %+v %v", fresh, fresh.Err)
	}
	b := scanPi(nil)
	if b.Err != nil || len(b.Sessions) != 1 {
		t.Fatalf("pi %+v %v", b.Sessions, b.Err)
	}
	s := b.Sessions[0]
	if s.NativeID != "pi-1" || s.Title != "plan the port" || s.CWD != "/work/pi" || s.Model != "openai/gpt-test" || s.DeleteMode != "file" || !s.CanDelete || s.Messages != 2 {
		t.Fatalf("session %+v", s)
	}
	if s.Usage.Context != 8 || s.Usage.Output != 2 || s.Usage.CacheRead != 1 || s.Usage.CacheWrite != 1 || s.Usage.Reasoning != 1 || s.Usage.CostUSD != 0.25 {
		t.Fatalf("usage %+v", s.Usage)
	}
	if strings.Contains(joined(s), "hidden") {
		t.Fatal("read a dotfile")
	}

	ompDir := filepath.Join(root, "omp-real")
	t.Setenv("PI_CODING_AGENT_SESSION_DIR", ompDir)
	writeBytes(t, filepath.Join(ompDir, "proj", "one.jsonl"), []byte(
		`{"type":"session","id":"omp-1","cwd":"/work/omp","timestamp":1700000000000}`+"\n"+
			`{"type":"title","title":"ship the omp"}`+"\n"+
			`{"type":"model_change","model":"omp-model"}`+"\n"+
			`{"type":"custom","text":"custom stays out"}`+"\n"+
			`{"type":"message","message":{"role":"user","content":[{"type":"text","text":"plan the port"}],"usage":{"input":3,"output":1,"cost":1.5}}}`+"\n"))
	b = scanOmp(nil)
	if b.Err != nil || len(b.Sessions) != 1 {
		t.Fatalf("omp %+v %v", b.Sessions, b.Err)
	}
	s = b.Sessions[0]
	if s.NativeID != "omp-1" || s.Title != "ship the omp" || s.CWD != "/work/omp" || s.Model != "omp-model" || s.DeleteMode != "file" || s.Messages != 1 || s.Usage.CostUSD != 1.5 || s.Usage.Context != 3 {
		t.Fatalf("omp session %+v usage %+v", s, s.Usage)
	}
	if strings.Contains(joined(s), "custom stays out") {
		t.Fatal("indexed a custom record")
	}
}

func TestPiAndOmpHomes(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("PI_CODING_AGENT_SESSION_DIR", "")
	t.Setenv("PI_CODING_AGENT_DIR", "")
	writeBytes(t, filepath.Join(home, ".pi", "agent", "sessions", "proj", "a.jsonl"), []byte(`{"type":"session","id":"from-pi","cwd":"/work/pi"}`+"\n"+`{"type":"message","message":{"role":"user","content":"pi home"}}`+"\n"))
	writeBytes(t, filepath.Join(home, ".omp", "agent", "sessions", "proj", "b.jsonl"), []byte(`{"type":"session","id":"from-omp","cwd":"/work/omp"}`+"\n"+`{"type":"message","message":{"role":"user","content":"omp home"}}`+"\n"))
	pi := scanPi(nil)
	omp := scanOmp(nil)
	if pi.Err != nil || len(pi.Sessions) != 1 || pi.Sessions[0].NativeID != "from-pi" {
		t.Fatalf("pi default %+v %v", pi.Sessions, pi.Err)
	}
	if omp.Err != nil || len(omp.Sessions) != 1 || omp.Sessions[0].NativeID != "from-omp" {
		t.Fatalf("omp default %+v %v", omp.Sessions, omp.Err)
	}

	shared := filepath.Join(home, "shared")
	t.Setenv("PI_CODING_AGENT_SESSION_DIR", shared)
	writeBytes(t, filepath.Join(shared, "proj", "c.jsonl"), []byte(`{"type":"session","id":"shared-1","cwd":"/work/shared"}`+"\n"+`{"type":"message","message":{"role":"user","content":"shared"}}`+"\n"))
	pi = scanPi(nil)
	omp = scanOmp(nil)
	if len(pi.Sessions) != 1 || pi.Sessions[0].NativeID != "shared-1" || len(omp.Sessions) != 1 || omp.Sessions[0].NativeID != "shared-1" {
		t.Fatalf("shared pi %+v omp %+v", pi.Sessions, omp.Sessions)
	}
}

func TestScanDroid(t *testing.T) {
	root := isolate(t)
	dir := filepath.Join(root, "factory", "sessions", "proj")
	path := filepath.Join(dir, "droid-1.jsonl")
	writeBytes(t, path, []byte(
		`{"type":"session_start","id":"droid-1","title":"ship the droid","cwd":"/work/droid","owner":"owner stays out","organizationId":"org stays out"}`+"\n"+
			`{"type":"message","role":"user","content":[{"type":"text","text":"plan the port"}],"timestamp":1700000000000}`+"\n"+
			`{"type":"agent_turn_outcome","text":"outcome stays out"}`+"\n"))
	writeBytes(t, filepath.Join(dir, "droid-1.settings.json"), []byte(`{"model":"droid-model","reasoningEffort":"high","tokenUsage":{"inputTokens":20,"outputTokens":5,"cacheReadTokens":2,"cacheCreationTokens":1,"thinkingTokens":3,"factoryCredits":99}}`))
	b := scanDroid(nil)
	if b.Err != nil || len(b.Sessions) != 1 {
		t.Fatalf("droid %+v %v", b.Sessions, b.Err)
	}
	s := b.Sessions[0]
	if s.NativeID != "droid-1" || s.Title != "ship the droid" || s.CWD != "/work/droid" || s.Model != "droid-model" || s.DeleteMode != "file" || s.SourcePath != path || s.Messages != 1 {
		t.Fatalf("session %+v", s)
	}
	if s.Usage.Input != 20 || s.Usage.Output != 5 || s.Usage.CacheRead != 2 || s.Usage.CacheWrite != 1 || s.Usage.Reasoning != 3 || s.Usage.Context != 20 || s.Usage.CostUSD != 0 || s.Usage.Effort != "high" {
		t.Fatalf("usage %+v", s.Usage)
	}
	if strings.Contains(joined(s), "owner stays out") || strings.Contains(joined(s), "outcome stays out") {
		t.Fatal("indexed owner or outcome text")
	}
}

func TestScanDevin(t *testing.T) {
	root := isolate(t)
	path := filepath.Join(root, "devin.db")
	t.Setenv("DEVIN_SESSIONS_DB", path)
	execDB(t, path,
		`CREATE TABLE sessions (id TEXT, working_directory TEXT, model TEXT, title TEXT, created_at INTEGER, last_activity_at INTEGER, hidden INTEGER)`,
		`CREATE TABLE message_nodes (row_id INTEGER, session_id TEXT, chat_message TEXT, created_at INTEGER)`,
		`CREATE TABLE prompt_history (id INTEGER, content TEXT)`,
		`INSERT INTO prompt_history VALUES (1, 'prompt history stays out')`,
		`INSERT INTO sessions VALUES ('hidden-1','/secret','m','hidden title',1700000000,1700000000,1)`,
		`INSERT INTO sessions VALUES ('devin-1','/work/devin','devin-model','ship the devin',1700000000,1700000005,0)`,
		`INSERT INTO message_nodes VALUES (1,'devin-1','{"role":"user","content":"plan the port"}',1700000000)`,
		`INSERT INTO message_nodes VALUES (2,'devin-1','{"role":"assistant","content":[{"type":"text","text":"done"}],"usage":{"input_tokens":4,"output_tokens":2,"cache_read_input_tokens":1}}',1700000005)`,
	)
	b := scanDevin(nil)
	if b.Err != nil || len(b.Sessions) != 1 {
		t.Fatalf("devin %+v %v", b.Sessions, b.Err)
	}
	s := b.Sessions[0]
	if s.NativeID != "devin-1" || s.Title != "ship the devin" || s.CWD != "/work/devin" || s.Model != "devin-model" || s.DeleteMode != "exec" || !s.CanDelete || s.Messages != 2 || s.SourcePath != path {
		t.Fatalf("session %+v", s)
	}
	if s.Usage.Context != 5 || s.Usage.Output != 2 || s.Usage.CacheRead != 1 || s.Usage.CostUSD != 0 {
		t.Fatalf("usage %+v", s.Usage)
	}
	if strings.Contains(joined(s), "prompt history stays out") || strings.Contains(s.Title, "hidden") {
		t.Fatal("indexed hidden or prompt history")
	}
	t.Setenv("DEVIN_SESSIONS_DB", filepath.Join(root, "missing-devin.db"))
	missing := scanDevin(nil)
	if missing.Err != nil || len(missing.Sessions) != 0 {
		t.Fatalf("missing %+v %v", missing.Sessions, missing.Err)
	}
	bad := filepath.Join(root, "bad-devin.db")
	writeBytes(t, bad, []byte("not a database"))
	t.Setenv("DEVIN_SESSIONS_DB", bad)
	corrupt := scanDevin(nil)
	if corrupt.Err == nil || len(corrupt.Sessions) != 0 {
		t.Fatalf("corrupt %+v %v", corrupt.Sessions, corrupt.Err)
	}
}

func TestDevinAndKiloPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("DEVIN_SESSIONS_DB", "/override/devin.db")
	if devinDB() != "/override/devin.db" {
		t.Fatal(devinDB())
	}
	t.Setenv("DEVIN_SESSIONS_DB", "")
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "xdg"))
	if devinDB() != filepath.Join(home, "xdg", "devin", "cli", "sessions.db") {
		t.Fatal(devinDB())
	}
	t.Setenv("XDG_DATA_HOME", "")
	if devinDB() != filepath.Join(home, ".local", "share", "devin", "cli", "sessions.db") {
		t.Fatal(devinDB())
	}
	t.Setenv("KILO_DB", "/override/kilo.db")
	if kiloDB() != "/override/kilo.db" {
		t.Fatal(kiloDB())
	}
	t.Setenv("KILO_DB", "")
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "xdg"))
	if kiloDB() != filepath.Join(home, "xdg", "kilo", "kilo.db") {
		t.Fatal(kiloDB())
	}
}

func TestScanKilo(t *testing.T) {
	root := isolate(t)
	path := filepath.Join(root, "kilo.db")
	t.Setenv("KILO_DB", path)
	execDB(t, path,
		`CREATE TABLE session (id TEXT, directory TEXT, title TEXT, model TEXT, time_updated INTEGER, time_created INTEGER, parent_id TEXT, cost REAL, tokens_input INTEGER, tokens_output INTEGER, tokens_reasoning INTEGER, tokens_cache_read INTEGER, tokens_cache_write INTEGER)`,
		`CREATE TABLE message (id TEXT, session_id TEXT, time_created INTEGER, time_updated INTEGER, data TEXT)`,
		`CREATE TABLE part (id TEXT, message_id TEXT, session_id TEXT, time_created INTEGER, data TEXT)`,
		`INSERT INTO session VALUES ('kilo-1','/work/kilo','ship the kilo','{"providerID":"prov","id":"kilo-model"}',1700000000000,1700000000000,'',0.5,11,4,1,2,3)`,
		`INSERT INTO session VALUES ('kilo-child','/work/kilo','child','{}',1700000000000,1700000000000,'kilo-1',0,0,0,0,0,0)`,
		`INSERT INTO message VALUES ('m1','kilo-1',1700000000000,1700000000000,'{"role":"user"}')`,
		`INSERT INTO part VALUES ('p1','m1','kilo-1',1700000000000,'{"type":"text","text":"plan the port"}')`,
		`INSERT INTO message VALUES ('m2','kilo-1',1700000001000,1700000001000,'{"role":"assistant","tokens":{"input":9},"model":{"providerID":"prov","id":"kilo-model"}}')`,
		`INSERT INTO part VALUES ('p2','m2','kilo-1',1700000001000,'{"type":"text","text":"done"}')`,
	)
	b := scanKilo(nil)
	if b.Err != nil || len(b.Sessions) != 1 {
		t.Fatalf("kilo %+v %v", b.Sessions, b.Err)
	}
	s := b.Sessions[0]
	if s.NativeID != "kilo-1" || s.Title != "ship the kilo" || s.CWD != "/work/kilo" || s.Model != "prov/kilo-model" || s.DeleteMode != "exec" || s.Messages != 2 {
		t.Fatalf("session %+v", s)
	}
	if s.Usage.Input != 11 || s.Usage.Output != 4 || s.Usage.Reasoning != 1 || s.Usage.CacheRead != 2 || s.Usage.CacheWrite != 3 || s.Usage.CostUSD != 0.5 || s.Usage.Context != 9 {
		t.Fatalf("usage %+v", s.Usage)
	}
	bad := filepath.Join(root, "bad-kilo.db")
	writeBytes(t, bad, []byte("not a database"))
	t.Setenv("KILO_DB", bad)
	corrupt := scanKilo(nil)
	if corrupt.Err == nil || len(corrupt.Sessions) != 0 {
		t.Fatalf("corrupt %+v %v", corrupt.Sessions, corrupt.Err)
	}
}

func TestScanQoder(t *testing.T) {
	root := isolate(t)
	dir := filepath.Join(root, "qoder", "projects", "proj")
	path := filepath.Join(dir, "q1.jsonl")
	writeBytes(t, path, []byte(
		`{"type":"workspace-directories","directories":["/work/from-list"]}`+"\n"+
			`{"type":"runtime-config","model":"qoder-model","timestamp":1700000000000}`+"\n"+
			`{"type":"system","cwd":"/work/qoder","gitBranch":"main"}`+"\n"+
			`{"type":"user","isSidechain":true,"message":{"role":"user","content":"side stays out"},"sessionId":"q1"}`+"\n"+
			`{"type":"user","message":{"role":"user","content":"plan the port"},"cwd":"/work/qoder","gitBranch":"main","sessionId":"q1","timestamp":1700000001000}`+"\n"+
			`{"type":"assistant","message":{"role":"assistant","model":"qoder-model","content":[{"type":"thinking","thinking":"secret"},{"type":"text","text":"done"}],"usage":{"input_tokens":8,"output_tokens":2,"cache_read_input_tokens":1,"cache_creation_input_tokens":4,"credits":3.5}},"timestamp":1700000002000}`+"\n"+
			`{"type":"ai-title","aiTitle":"ship the qoder"}`+"\n"))
	writeBytes(t, filepath.Join(dir, "q1", "extra.jsonl"), []byte(`{"type":"user","sessionId":"nope","message":{"role":"user","content":"nested stays out"}}`+"\n"))
	b := scanQoder(nil)
	if b.Err != nil || len(b.Sessions) != 1 {
		t.Fatalf("qoder %+v %v", b.Sessions, b.Err)
	}
	s := b.Sessions[0]
	if s.NativeID != "q1" || s.Title != "ship the qoder" || s.CWD != "/work/qoder" || s.Branch != "main" || s.Model != "qoder-model" || s.DeleteMode != "file" || s.SourcePath != path || s.Messages != 2 {
		t.Fatalf("session %+v", s)
	}
	if s.Usage.Context != 13 || s.Usage.Output != 2 || s.Usage.CacheRead != 1 || s.Usage.CacheWrite != 4 || s.Usage.CostUSD != 0 {
		t.Fatalf("usage %+v", s.Usage)
	}
	if strings.Contains(joined(s), "side stays out") || strings.Contains(joined(s), "nested stays out") || strings.Contains(joined(s), "secret") {
		t.Fatal("indexed a sidechain, nested file, or thinking text")
	}
}

func TestScanLetta(t *testing.T) {
	root := isolate(t)
	base := filepath.Join(root, "letta", "transcripts", "agent-1")
	writeBytes(t, filepath.Join(base, "default", "transcript.jsonl"), []byte(
		`{"kind":"user","text":"plan the port","captured_at":"2026-01-02T03:04:05Z"}`+"\n"+
			`{"kind":"reasoning","text":"reasoning stays out"}`+"\n"+
			`{"kind":"assistant","text":"done","captured_at":"2026-01-02T03:04:06Z"}`+"\n"))
	writeBytes(t, filepath.Join(base, "room", "transcript.jsonl"), []byte(`{"kind":"user","text":"second chat"}`+"\n"))
	writeBytes(t, filepath.Join(base, "memory.md"), []byte("memory stays out"))
	writeBytes(t, filepath.Join(root, "letta", "logs", "debug.log"), []byte("log stays out"))
	b := scanLetta(nil)
	if b.Err != nil || len(b.Sessions) != 2 {
		t.Fatalf("letta %+v %v", b.Sessions, b.Err)
	}
	got := map[string]model.Session{}
	for _, s := range b.Sessions {
		got[s.NativeID] = s
	}
	if s := got["agent-1"]; s.Title != "plan the port" || s.Messages != 2 || s.DeleteMode != "file" || strings.Contains(joined(s), "reasoning stays out") {
		t.Fatalf("default %+v", s)
	}
	if s := got["agent-1:room"]; s.Title != "second chat" || s.Messages != 1 {
		t.Fatalf("room %+v", s)
	}
}

func TestScanMastra(t *testing.T) {
	root := isolate(t)
	path := filepath.Join(root, "mastra.db")
	t.Setenv("MASTRA_DB_PATH", path)
	execDB(t, path,
		`CREATE TABLE mastra_threads (id TEXT, title TEXT, metadata TEXT, createdAt TEXT, updatedAt TEXT)`,
		`CREATE TABLE mastra_messages (id TEXT, thread_id TEXT, content TEXT, role TEXT, type TEXT, createdAt TEXT)`,
		`INSERT INTO mastra_threads VALUES ('thread-1','ship the mastra','{"cwd":"/work/mastra"}','2026-01-02T03:04:05Z','2026-01-02T03:05:05Z')`,
		`INSERT INTO mastra_threads VALUES ('thread-child','child','{"parentThreadId":"thread-1"}','2026-01-02T03:04:05Z','2026-01-02T03:05:05Z')`,
		`INSERT INTO mastra_messages VALUES ('a','thread-1','{"parts":[{"type":"text","text":"plan the port"}]}','user','text','2026-01-02T03:04:05Z')`,
		`INSERT INTO mastra_messages VALUES ('b','thread-child','child text stays out','user','text','2026-01-02T03:04:05Z')`,
	)
	b := scanMastra(nil)
	if b.Err != nil || len(b.Sessions) != 1 {
		t.Fatalf("mastra %+v %v", b.Sessions, b.Err)
	}
	s := b.Sessions[0]
	if s.NativeID != "thread-1" || s.Title != "ship the mastra" || s.CWD != "/work/mastra" || s.CanDelete || s.DeleteReason != mastraDeleteReason || s.Messages != 1 {
		t.Fatalf("session %+v", s)
	}
	if strings.Contains(joined(s), "child text") {
		t.Fatal("indexed a child thread")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "xdg"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData"))
	t.Setenv("MASTRA_DB_PATH", "")
	missing := scanMastra(nil)
	if missing.Err != nil || len(missing.Sessions) != 0 {
		t.Fatalf("missing %+v %v", missing.Sessions, missing.Err)
	}
	var dir string
	switch runtime.GOOS {
	case "darwin":
		dir = filepath.Join(home, "Library", "Application Support", "mastracode")
	case "windows":
		dir = filepath.Join(home, "AppData", "mastracode")
	default:
		dir = filepath.Join(home, "xdg", "mastracode")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("created %s", dir)
	}
}

func TestScanAmp(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeBytes(t, filepath.Join(home, ".amp", "threads.jsonl"), []byte("{\"id\":\"nope\"}\n"))
	b := scanAmp(nil)
	if b.Err != nil || b.Agent != "amp" || len(b.Sessions) != 0 || len(b.Files) != 0 {
		t.Fatalf("%+v %v", b, b.Err)
	}
}

func TestScanMaki(t *testing.T) {
	root := isolate(t)
	dir := filepath.Join(root, "maki", "sessions")
	path := filepath.Join(dir, "maki-1.jsonl")
	writeBytes(t, path, []byte(
		`{"t":"header","v":2,"id":"maki-1","model":"maki-model","cwd":"/work/maki","created_at":1700000000000}`+"\n"+
			`{"t":"msg","d":{"role":"user","content":[{"type":"text","text":"plan the port"}]}}`+"\n"+
			`{"t":"sub_msg","sub":"child","d":{"role":"assistant","content":[{"type":"text","text":"subagent stays out"}]}}`+"\n"+
			`{"t":"out","id":"t1","d":{"text":"tool stays out"}}`+"\n"+
			`{"t":"meta","title":"ship the maki","token_usage":{"input":10,"output":4,"cache_creation":2,"cache_read":3,"cost":0.5,"subsidised_list_cost":9},"updated_at":1700000001000,"context_size":8000}`+"\n"))
	writeBytes(t, filepath.Join(dir, "archive", "maki-1", "1.jsonl"), []byte(`{"t":"header","id":"archived","cwd":"/old"}`+"\n"+`{"t":"msg","d":{"role":"user","content":[{"type":"text","text":"archive stays out"}]}}`+"\n"))
	b := scanMaki(nil)
	if b.Err != nil || len(b.Sessions) != 1 {
		t.Fatalf("maki %+v %v", b.Sessions, b.Err)
	}
	s := b.Sessions[0]
	if s.NativeID != "maki-1" || s.Title != "ship the maki" || s.CWD != "/work/maki" || s.Model != "maki-model" || s.DeleteMode != "exec" || s.SourcePath != path || s.Messages != 1 {
		t.Fatalf("session %+v", s)
	}
	if s.Usage.Input != 10 || s.Usage.Output != 4 || s.Usage.CacheWrite != 2 || s.Usage.CacheRead != 3 || s.Usage.CostUSD != 0.5 || s.Usage.Context != 15 || s.Usage.Window != 8000 {
		t.Fatalf("usage %+v", s.Usage)
	}
	if strings.Contains(joined(s), "subagent stays out") || strings.Contains(joined(s), "tool stays out") || strings.Contains(joined(s), "archive stays out") {
		t.Fatal("indexed a skipped record")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("MAKI_STATE_DIR", "")
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	writeBytes(t, filepath.Join(home, ".maki", "sessions", "legacy.jsonl"), []byte(`{"t":"header","id":"legacy","cwd":"/legacy","created_at":1700000000000}`+"\n"+`{"t":"msg","d":{"role":"user","content":"legacy home"}}`+"\n"))
	writeBytes(t, filepath.Join(home, "state", "maki", "sessions", "xdg.jsonl"), []byte(`{"t":"header","id":"xdg","cwd":"/xdg","created_at":1700000000000}`+"\n"+`{"t":"msg","d":{"role":"user","content":"xdg home"}}`+"\n"))
	legacy := scanMaki(nil)
	if legacy.Err != nil || len(legacy.Sessions) != 1 || legacy.Sessions[0].NativeID != "legacy" {
		t.Fatalf("legacy %+v %v", legacy.Sessions, legacy.Err)
	}
	if err := os.RemoveAll(filepath.Join(home, ".maki")); err != nil {
		t.Fatal(err)
	}
	xdg := scanMaki(nil)
	if xdg.Err != nil || len(xdg.Sessions) != 1 || xdg.Sessions[0].NativeID != "xdg" {
		t.Fatalf("xdg %+v %v", xdg.Sessions, xdg.Err)
	}
}

func joined(s model.Session) string {
	var b strings.Builder
	b.WriteString(s.Title)
	b.WriteByte('\n')
	for _, sn := range s.Snippets {
		b.WriteString(sn.Body)
		b.WriteByte('\n')
	}
	return b.String()
}

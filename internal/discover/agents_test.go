package discover

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

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

func TestNewAgentStores(t *testing.T) {
	root := isolate(t)

	execDB(t, filepath.Join(root, "hermes", "state.db"),
		`CREATE TABLE sessions (
			id TEXT, source TEXT, model TEXT, title TEXT, cwd TEXT, git_branch TEXT,
			message_count INTEGER, input_tokens INTEGER, output_tokens INTEGER,
			cache_read_tokens INTEGER, cache_write_tokens INTEGER, reasoning_tokens INTEGER,
			actual_cost_usd REAL, estimated_cost_usd REAL,
			started_at REAL, last_activity_at REAL, ended_at REAL,
			hidden INTEGER, archived INTEGER)`,
		`CREATE TABLE messages (id INTEGER PRIMARY KEY, session_id TEXT, role TEXT, content TEXT, active INTEGER)`,
		`INSERT INTO sessions VALUES ('home-1','cli','opus','desk','/work/h','main',2,10,4,0,0,0,1.5,0.2,0,1700000000,0,0,0)`,
		`INSERT INTO sessions VALUES ('hidden-1','cli','opus','nope','/work/h','',1,0,0,0,0,0,0,0,0,1700000000,0,1,0)`,
		`INSERT INTO messages VALUES (1,'home-1','user','hello from hermes',1)`,
	)
	execDB(t, filepath.Join(root, "hermes", "profiles", "work", "state.db"),
		`CREATE TABLE sessions (
			id TEXT, source TEXT, model TEXT, title TEXT, cwd TEXT, git_branch TEXT,
			message_count INTEGER, input_tokens INTEGER, output_tokens INTEGER,
			cache_read_tokens INTEGER, cache_write_tokens INTEGER, reasoning_tokens INTEGER,
			actual_cost_usd REAL, estimated_cost_usd REAL,
			started_at REAL, last_activity_at REAL, ended_at REAL,
			hidden INTEGER, archived INTEGER)`,
		`CREATE TABLE messages (id INTEGER PRIMARY KEY, session_id TEXT, role TEXT, content TEXT, active INTEGER)`,
		`INSERT INTO sessions VALUES ('abc:def','cli','opus','profile','/work/p','',1,0,0,0,0,0,0,0,0,1700000001,0,0,0)`,
	)
	hermes := scanHermes(nil)
	if hermes.Err != nil {
		t.Fatal(hermes.Err)
	}
	got := map[string]string{}
	for _, s := range hermes.Sessions {
		got[s.NativeID] = s.CWD
		if !s.CanDelete || s.DeleteMode != "exec" {
			t.Fatalf("hermes delete %+v", s)
		}
	}
	if got["home-1"] != "/work/h" || got["p:work:abc:def"] != "/work/p" || len(got) != 2 {
		t.Fatalf("hermes ids %+v", got)
	}

	entry := `{"sessionId":"uuid-1","displayName":"Ship","model":"gpt","modelProvider":"openai","systemPromptReport":{"workspaceDir":"/work/app"},"inputTokens":3,"outputTokens":1,"contextTokens":200000,"contextBudgetStatus":{"estimatedPromptTokens":500}}`
	execDB(t, filepath.Join(root, "openclaw", "agents", "main", "agent", "openclaw-agent.sqlite"),
		`CREATE TABLE session_nodes (
			session_key TEXT, display_name TEXT, label TEXT,
			updated_at INTEGER, last_activity_at INTEGER, archived_at INTEGER,
			entry_json TEXT)`,
		`INSERT INTO session_nodes VALUES ('agent:main:live','Ship','',1767225600000,1767225600000,0,'`+entry+`')`,
		`INSERT INTO session_nodes VALUES ('agent:main:old','Old','',1767225600000,1767225600000,1767225600000,'{}')`,
	)
	claw := scanOpenClaw(nil)
	if claw.Err != nil || len(claw.Sessions) != 1 {
		t.Fatalf("openclaw %+v %v", claw.Sessions, claw.Err)
	}
	if claw.Sessions[0].NativeID != "agent:main:live" || claw.Sessions[0].CWD != "/work/app" ||
		claw.Sessions[0].Usage.Context != 500 || claw.Sessions[0].Usage.Window != 200000 || !claw.Sessions[0].CanDelete {
		t.Fatalf("openclaw session %+v", claw.Sessions[0])
	}

	junieDir := filepath.Join(root, "junie", "sessions", "session-9")
	if err := os.MkdirAll(junieDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(junieDir, "transcript.md"), []byte("# Session transcript\nuser: fix the prompt\nassistant: done\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	junie := scanJunie(nil)
	if junie.Err != nil || len(junie.Sessions) != 1 || junie.Sessions[0].NativeID != "session-9" ||
		junie.Sessions[0].SourcePath != filepath.Join(junieDir, "transcript.md") || junie.Sessions[0].DeleteMode != "dir" || junie.Sessions[0].Title != "fix the prompt" {
		t.Fatalf("junie %+v %v", junie.Sessions, junie.Err)
	}

	if err := os.MkdirAll(filepath.Join(root, "jules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "jules", "sessions.json"), []byte(`[{"id":"12345","title":"ship the widget","repo":"owner/repo"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	jules := scanJules(nil)
	if jules.Err != nil || len(jules.Sessions) != 1 || jules.Sessions[0].NativeID != "12345" ||
		jules.Sessions[0].CWD != "owner/repo" || !jules.Sessions[0].CanDelete || jules.Sessions[0].DeleteMode != "jules" {
		t.Fatalf("jules %+v %v", jules.Sessions, jules.Err)
	}

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
		`INSERT INTO sessions VALUES ('g2','Archived','','/work/goose','2026-01-02 03:04:05','2026-01-02 03:04:05',0,0,0,0,0,0,0,0,0,'{}','2026-01-03')`,
	)
	goose := scanGoose(nil)
	if goose.Err != nil || len(goose.Sessions) != 1 {
		t.Fatalf("goose %+v %v", goose.Sessions, goose.Err)
	}
	if goose.Sessions[0].NativeID != "g1" || goose.Sessions[0].Model != "gpt" || goose.Sessions[0].Usage.Input != 12 || goose.Sessions[0].DeleteMode != "exec" {
		t.Fatalf("goose session %+v", goose.Sessions[0])
	}

	clineHome := filepath.Join(root, "cline")
	hist := filepath.Join(clineHome, "data", "state", "taskHistory.json")
	if err := os.MkdirAll(filepath.Dir(hist), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(clineHome, "data", "tasks", "task-1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hist, []byte(`[{"id":"task-1","task":"wire the button","cwdOnTaskInitialization":"/work/cline","tokensIn":8,"tokensOut":2,"totalCost":0.25}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clineHome, "data", "tasks", "task-1", "api_conversation_history.json"), []byte(`[{"role":"user","content":"wire the button"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	cline := scanCline(nil)
	if cline.Err != nil || len(cline.Sessions) != 1 || cline.Sessions[0].NativeID != "task-1" ||
		cline.Sessions[0].CWD != "/work/cline" || cline.Sessions[0].Usage.Input != 8 || cline.Sessions[0].DeleteMode != "cline" {
		t.Fatalf("cline %+v %v", cline.Sessions, cline.Err)
	}

	aiderDir := filepath.Join(root, "aider", "proj")
	if err := os.MkdirAll(aiderDir, 0o755); err != nil {
		t.Fatal(err)
	}
	aiderPath := filepath.Join(aiderDir, ".aider.chat.history.md")
	if err := os.WriteFile(aiderPath, []byte("#### user\nexplain the makefile\n#### assistant\nit builds\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	aider := scanAider(nil)
	if aider.Err != nil || len(aider.Sessions) != 1 || aider.Sessions[0].NativeID != aiderPath ||
		aider.Sessions[0].CWD != aiderDir || aider.Sessions[0].Title != "explain the makefile" || aider.Sessions[0].DeleteMode != "aider" {
		t.Fatalf("aider %+v %v", aider.Sessions, aider.Err)
	}

	kiroPath := filepath.Join(root, "kiro.db")
	t.Setenv("KIRO_CLI_DB", kiroPath)
	value := `{"history":[[{"content":{"Prompt":"rename the button"},"env_context":{"env_state":{"current_working_directory":"/work/kiro"}}}]]}`
	execDB(t, kiroPath,
		`CREATE TABLE conversations_v2 (conversation_id TEXT, key TEXT, value TEXT, updated_at INTEGER, created_at INTEGER)`,
		`CREATE TABLE history (command TEXT)`,
		`INSERT INTO history VALUES ('echo secret')`,
		`INSERT INTO conversations_v2 VALUES ('conv-1','k1','`+value+`',1767225600000,1767225600000)`,
	)
	kiro := scanKiro(nil)
	if kiro.Err != nil || len(kiro.Sessions) != 1 {
		t.Fatalf("kiro %+v %v", kiro.Sessions, kiro.Err)
	}
	if kiro.Sessions[0].NativeID != "conv-1" || kiro.Sessions[0].CWD != "/work/kiro" || !kiro.Sessions[0].CanDelete || kiro.Sessions[0].DeleteMode != "kiro" ||
		kiro.Sessions[0].Title != "rename the button" {
		t.Fatalf("kiro session %+v", kiro.Sessions[0])
	}
}

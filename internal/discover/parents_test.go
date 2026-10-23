package discover

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AymanZahran/ai9s/internal/model"
)

func TestClaudeAndCursorSubagents(t *testing.T) {
	root := isolate(t)
	proj := filepath.Join(root, "claude", "projects", "-work-claude")
	writeBytes(t, filepath.Join(proj, "sess-1.jsonl"), []byte(
		`{"type":"user","timestamp":"2026-03-02T15:04:05Z","cwd":"/work/claude","sessionId":"sess-1","message":{"role":"user","content":"parent prompt"}}`+"\n"))
	writeBytes(t, filepath.Join(proj, "sess-1", "subagents", "agent-abc.jsonl"), []byte(
		`{"type":"user","timestamp":"2026-03-02T15:04:06Z","cwd":"/work/claude","sessionId":"sess-1","message":{"role":"user","content":"child prompt"}}`+"\n"))
	writeBytes(t, filepath.Join(proj, "sess-1", "subagents", "agent-def.jsonl"), []byte(
		`{"type":"user","timestamp":"2026-03-02T15:04:07Z","cwd":"/work/claude","message":{"role":"user","content":"folder parent"}}`+"\n"))

	claude := scanClaude(nil)
	if claude.Err != nil || len(claude.Sessions) != 3 {
		t.Fatalf("claude %+v %v", claude.Sessions, claude.Err)
	}
	got := map[string]model.Session{}
	for _, s := range claude.Sessions {
		got[s.NativeID] = s
	}
	if s := got["sess-1"]; s.ParentID != "" || s.CWD != "/work/claude" {
		t.Fatalf("parent %+v", s)
	}
	if s := got["agent-abc"]; s.ParentID != "claude:sess-1" || s.CWD != "/work/claude" || s.NativeID == "subagents" {
		t.Fatalf("sessionId parent %+v", s)
	}
	if s := got["agent-def"]; s.ParentID != "claude:sess-1" || s.CWD != "/work/claude" {
		t.Fatalf("folder parent %+v", s)
	}

	id := "conv-1"
	dir := filepath.Join(root, "cursor", "projects", "demo", "agent-transcripts", id)
	writeBytes(t, filepath.Join(dir, id+".jsonl"), []byte(`{"role":"user","message":{"content":"parent cursor"}}`+"\n"))
	writeBytes(t, filepath.Join(dir, "subagents", "child-file.jsonl"), []byte(`{"role":"user","message":{"content":"child cursor"}}`+"\n"))
	cursor := scanCursor(nil)
	if cursor.Err != nil || len(cursor.Sessions) != 2 {
		t.Fatalf("cursor %+v %v", cursor.Sessions, cursor.Err)
	}
	got = map[string]model.Session{}
	for _, s := range cursor.Sessions {
		got[s.NativeID] = s
	}
	if s := got[id]; s.ParentID != "" || s.NativeID == "subagents" {
		t.Fatalf("cursor parent %+v", s)
	}
	if s := got["child-file"]; s.ParentID != "cursor:conv-1" || s.NativeID == "subagents" {
		t.Fatalf("cursor child %+v", s)
	}
}

func TestGrokSpawnParents(t *testing.T) {
	root := isolate(t)
	base := filepath.Join(root, "grok", "sessions", "proj")
	writeBytes(t, filepath.Join(base, "parent-1", "summary.json"), []byte(`{"info":{"id":"parent-1","cwd":"/work/grok"},"session_summary":"parent session"}`))
	writeBytes(t, filepath.Join(base, "parent-1", "updates.jsonl"), []byte(
		`{"params":{"update":{"sessionUpdate":"subagent_spawned","child_session_id":"child-spawn","parent_session_id":"parent-1"}}}`+"\n"+
			`{"params":{"update":{"sessionUpdate":"subagent_spawned","child_session_id":"child-sum","parent_session_id":"wrong"}}}`+"\n"))
	writeBytes(t, filepath.Join(base, "child-spawn", "summary.json"), []byte(`{"info":{"id":"child-spawn","cwd":"/work/grok"},"session_summary":"spawned child"}`))
	writeBytes(t, filepath.Join(base, "child-sum", "summary.json"), []byte(`{"info":{"id":"child-sum","cwd":"/work/grok"},"parent_session_id":"parent-1","session_summary":"summary child"}`))

	fresh := scanGrok(func(string, int64) bool { return true })
	if fresh.Err != nil || len(fresh.Sessions) != 0 {
		t.Fatalf("fresh %+v %v", fresh.Sessions, fresh.Err)
	}
	if fresh.Parents["grok:child-spawn"] != "grok:parent-1" || fresh.Parents["grok:child-sum"] != "grok:parent-1" {
		t.Fatalf("fresh parents %+v", fresh.Parents)
	}

	full := scanGrok(nil)
	if full.Err != nil || len(full.Sessions) != 3 {
		t.Fatalf("grok %+v %v", full.Sessions, full.Err)
	}
	got := map[string]model.Session{}
	for _, s := range full.Sessions {
		got[s.NativeID] = s
	}
	if got["child-spawn"].ParentID != "" || got["child-sum"].ParentID != "grok:parent-1" {
		t.Fatalf("summary parents spawn %q sum %q", got["child-spawn"].ParentID, got["child-sum"].ParentID)
	}
	if full.Parents["grok:child-spawn"] != "grok:parent-1" || full.Parents["grok:child-sum"] != "grok:parent-1" {
		t.Fatalf("parents %+v", full.Parents)
	}
}

func TestCodexSpawnEdges(t *testing.T) {
	root := isolate(t)
	dir := filepath.Join(root, "codex", "sessions", "2026", "03", "02")
	body := func(id, text string) string {
		return `{"type":"session_meta","timestamp":"2026-03-02T15:04:05Z","payload":{"session_id":"` + id + `","cwd":"/work/codex"}}` + "\n" +
			`{"type":"response_item","timestamp":"2026-03-02T15:04:06Z","payload":{"type":"message","role":"user","content":"` + text + `"}}` + "\n"
	}
	writeBytes(t, filepath.Join(dir, "rollout-parent.jsonl"), []byte(body("parent-1", "parent")))
	writeBytes(t, filepath.Join(dir, "rollout-child.jsonl"), []byte(body("child-1", "child")))
	execDB(t, filepath.Join(root, "codex", "state_1.sqlite"),
		`CREATE TABLE thread_spawn_edges (parent_thread_id TEXT, child_thread_id TEXT, status TEXT)`,
		`INSERT INTO thread_spawn_edges VALUES ('decoy','child-1','ok')`,
	)
	state := filepath.Join(root, "codex", "state_5.sqlite")
	execDB(t, state,
		`CREATE TABLE thread_spawn_edges (parent_thread_id TEXT, child_thread_id TEXT, status TEXT)`,
		`INSERT INTO thread_spawn_edges VALUES ('parent-1','child-1','ok')`,
		`INSERT INTO thread_spawn_edges VALUES ('child-1','child-1','ok')`,
	)
	b := scanCodex(nil)
	if b.Err != nil || len(b.Sessions) != 2 {
		t.Fatalf("codex %+v %v", b.Sessions, b.Err)
	}
	if b.Parents["codex:child-1"] != "codex:parent-1" || len(b.Parents) != 1 {
		t.Fatalf("parents %+v", b.Parents)
	}
	if err := os.Remove(state); err != nil {
		t.Fatal(err)
	}
	execDB(t, state, `CREATE TABLE other (id TEXT)`)
	missing := scanCodex(nil)
	if missing.Err != nil || missing.Parents == nil || len(missing.Parents) != 0 {
		t.Fatalf("missing table %+v %v", missing.Parents, missing.Err)
	}
}

func TestOpenCodeHermesOpenClawParents(t *testing.T) {
	root := isolate(t)
	path := filepath.Join(root, "opencode.db")
	t.Setenv("OPENCODE_DB", path)
	execDB(t, path,
		`CREATE TABLE session (
			id TEXT, directory TEXT, title TEXT, model TEXT, time_updated INTEGER, time_created INTEGER,
			parent_id TEXT, cost REAL, tokens_input INTEGER, tokens_output INTEGER, tokens_reasoning INTEGER,
			tokens_cache_read INTEGER, tokens_cache_write INTEGER)`,
		`INSERT INTO session VALUES ('oc-1','/work','parent','{}',1000,1000,'',0,0,0,0,0,0)`,
		`INSERT INTO session VALUES ('oc-child','/work','child','{}',1001,1000,'oc-1',0,0,0,0,0,0)`,
		`INSERT INTO session VALUES ('oc-self','/work','self','{}',1002,1000,'oc-self',0,0,0,0,0,0)`,
	)
	oc := scanOpenCode(nil)
	if oc.Err != nil || len(oc.Sessions) != 3 {
		t.Fatalf("opencode %+v %v", oc.Sessions, oc.Err)
	}
	got := map[string]model.Session{}
	for _, s := range oc.Sessions {
		got[s.NativeID] = s
	}
	if got["oc-child"].ParentID != "opencode:oc-1" || got["oc-self"].ParentID != "" || got["oc-1"].ParentID != "" {
		t.Fatalf("opencode parents child %q self %q", got["oc-child"].ParentID, got["oc-self"].ParentID)
	}

	execDB(t, filepath.Join(root, "hermes", "state.db"),
		`CREATE TABLE sessions (
			id TEXT, source TEXT, model TEXT, title TEXT, cwd TEXT, git_branch TEXT,
			message_count INTEGER, input_tokens INTEGER, output_tokens INTEGER,
			cache_read_tokens INTEGER, cache_write_tokens INTEGER, reasoning_tokens INTEGER,
			actual_cost_usd REAL, estimated_cost_usd REAL,
			started_at REAL, last_activity_at REAL, ended_at REAL,
			hidden INTEGER, archived INTEGER, parent_session_id TEXT)`,
		`CREATE TABLE messages (id INTEGER PRIMARY KEY, session_id TEXT, role TEXT, content TEXT, active INTEGER)`,
		`INSERT INTO sessions VALUES ('home-1','cli','opus','desk','/work/h','main',1,0,0,0,0,0,0,0,0,1700000000,0,0,0,'')`,
		`INSERT INTO sessions VALUES ('home-child','cli','opus','child','/work/h','main',1,0,0,0,0,0,0,0,0,1700000001,0,0,0,'home-1')`,
	)
	execDB(t, filepath.Join(root, "hermes", "profiles", "work", "state.db"),
		`CREATE TABLE sessions (
			id TEXT, source TEXT, model TEXT, title TEXT, cwd TEXT, git_branch TEXT,
			message_count INTEGER, input_tokens INTEGER, output_tokens INTEGER,
			cache_read_tokens INTEGER, cache_write_tokens INTEGER, reasoning_tokens INTEGER,
			actual_cost_usd REAL, estimated_cost_usd REAL,
			started_at REAL, last_activity_at REAL, ended_at REAL,
			hidden INTEGER, archived INTEGER, parent_session_id TEXT)`,
		`CREATE TABLE messages (id INTEGER PRIMARY KEY, session_id TEXT, role TEXT, content TEXT, active INTEGER)`,
		`INSERT INTO sessions VALUES ('abc','cli','opus','profile','/work/p','',1,0,0,0,0,0,0,0,0,1700000001,0,0,0,'')`,
		`INSERT INTO sessions VALUES ('kid','cli','opus','profile child','/work/p','',1,0,0,0,0,0,0,0,0,1700000002,0,0,0,'abc')`,
	)
	hermes := scanHermes(nil)
	if hermes.Err != nil {
		t.Fatal(hermes.Err)
	}
	got = map[string]model.Session{}
	for _, s := range hermes.Sessions {
		got[s.NativeID] = s
	}
	if got["home-child"].ParentID != "hermes:home-1" || got["p:work:kid"].ParentID != "hermes:p:work:abc" {
		t.Fatalf("hermes %+v", got)
	}

	execDB(t, filepath.Join(root, "openclaw", "agents", "main", "agent", "openclaw-agent.sqlite"),
		`CREATE TABLE session_nodes (
			session_key TEXT, display_name TEXT, label TEXT,
			updated_at INTEGER, last_activity_at INTEGER, archived_at INTEGER,
			entry_json TEXT, parent_session_key TEXT)`,
		`INSERT INTO session_nodes VALUES ('agent:main:live','Ship','',1767225600000,1767225600000,0,'{}','')`,
		`INSERT INTO session_nodes VALUES ('agent:main:child','Child','',1767225600001,1767225600001,0,'{}','agent:main:live')`,
		`INSERT INTO session_nodes VALUES ('agent:main:old','Old','',1767225600000,1767225600000,1767225600000,'{}','agent:main:live')`,
		`INSERT INTO session_nodes VALUES ('agent:main:self','Self','',1767225600002,1767225600002,0,'{}','agent:main:self')`,
	)
	claw := scanOpenClaw(nil)
	if claw.Err != nil || len(claw.Sessions) != 3 {
		t.Fatalf("openclaw %+v %v", claw.Sessions, claw.Err)
	}
	got = map[string]model.Session{}
	for _, s := range claw.Sessions {
		got[s.NativeID] = s
	}
	if _, ok := got["agent:main:old"]; ok {
		t.Fatal("archived row was indexed")
	}
	if got["agent:main:child"].ParentID != "openclaw:agent:main:live" || got["agent:main:self"].ParentID != "" || got["agent:main:live"].ParentID != "" {
		t.Fatalf("openclaw parents %+v", got)
	}
}

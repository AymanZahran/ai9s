package discover

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestClaudeCodexCopilotUsage(t *testing.T) {
	dir := t.TempDir()
	claude := filepath.Join(dir, "abc.jsonl")
	body := `{"type":"user","timestamp":"2026-03-02T15:04:05Z","cwd":"/work","message":{"content":"hello there"}}
{"type":"assistant","timestamp":"2026-03-02T15:04:06Z","message":{"model":"claude-sonnet","usage":{"input_tokens":2,"cache_creation_input_tokens":1000,"cache_read_input_tokens":50,"output_tokens":10,"output_tokens_details":{"thinking_tokens":4}}}}
{"type":"assistant","timestamp":"2026-03-02T15:05:05Z","message":{"usage":{"input_tokens":3,"cache_read_input_tokens":20,"output_tokens":5}}}
{"type":"cost-state","totalCostUSD":0.12}
`
	if err := os.WriteFile(claude, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, ok := readClaude(claude)
	if !ok {
		t.Fatal("claude")
	}
	if got.Usage.Context != 23 || got.Usage.Output != 15 || got.Usage.CacheRead != 70 || got.Usage.CacheWrite != 1000 || got.Usage.Reasoning != 4 || got.Usage.CostUSD != 0.12 {
		t.Fatalf("claude usage %+v", got.Usage)
	}

	codex := filepath.Join(dir, "rollout-1.jsonl")
	codexBody := `{"type":"session_meta","timestamp":"2026-03-02T15:04:05Z","payload":{"session_id":"s1","cwd":"/work","model":"gpt"}}
{"type":"response_item","timestamp":"2026-03-02T15:04:06Z","payload":{"type":"message","role":"user","content":"hello"}}
{"type":"event_msg","timestamp":"2026-03-02T15:04:07Z","payload":{"type":"token_count","info":{"model_context_window":200000,"total_token_usage":{"input_tokens":100,"cached_input_tokens":40,"output_tokens":20,"reasoning_output_tokens":5,"total_tokens":165},"last_token_usage":{"input_tokens":10,"cached_input_tokens":40,"output_tokens":8,"total_tokens":58}}}}
`
	if err := os.WriteFile(codex, []byte(codexBody), 0o600); err != nil {
		t.Fatal(err)
	}
	cs, ok := readCodex(codex)
	if !ok {
		t.Fatal("codex")
	}
	if cs.Usage.Window != 200000 || cs.Usage.Context != 50 || cs.Usage.Input != 100 || cs.Usage.Output != 20 || cs.Usage.Reasoning != 5 {
		t.Fatalf("codex usage %+v", cs.Usage)
	}

	cp := filepath.Join(dir, "copilot-session")
	if err := os.MkdirAll(cp, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cp, "workspace.yaml"), []byte("id: sess\nname: demo\ncwd: /work\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	events := `{"type":"session.start","timestamp":"2026-03-02T15:04:05Z","data":{"selectedModel":"gpt","reasoningEffort":"medium"}}
{"type":"user.message","timestamp":"2026-03-02T15:04:06Z","data":{"content":"hello"}}
{"type":"session.usage_checkpoint","timestamp":"2026-03-02T15:04:07Z","data":{"totalPremiumRequests":2,"promptCacheBreakState":[{"models":{"gpt":{"prompt_tokens":22000,"cache_read":100,"cache_write":400}}}]}}
`
	if err := os.WriteFile(filepath.Join(cp, "events.jsonl"), []byte(events), 0o600); err != nil {
		t.Fatal(err)
	}
	ps, ok := readCopilotDir(cp, "copilot")
	if !ok {
		t.Fatal("copilot")
	}
	if ps.Usage.Context != 22000 || ps.Usage.CacheRead != 100 || ps.Usage.CacheWrite != 400 || ps.Usage.Requests != 2 || ps.Usage.Effort != "medium" {
		t.Fatalf("copilot usage %+v", ps.Usage)
	}
}

func TestOpenCodeUsage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	schema := `
CREATE TABLE session (
  id TEXT, directory TEXT, title TEXT, model TEXT, time_updated INTEGER, time_created INTEGER,
  cost REAL, tokens_input INTEGER, tokens_output INTEGER, tokens_reasoning INTEGER,
  tokens_cache_read INTEGER, tokens_cache_write INTEGER
);
CREATE TABLE message (id TEXT, session_id TEXT, time_created INTEGER, time_updated INTEGER, data TEXT);
CREATE TABLE part (id TEXT, message_id TEXT, session_id TEXT, time_created INTEGER, time_updated INTEGER, data TEXT);
`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO session VALUES ('abc','/work','title','{"modelID":"m"}',1000,1000,0.5,100,20,3,7,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO message VALUES ('m1','abc',1000,1000,'{"role":"assistant","tokens":{"input":80,"output":20}}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO part VALUES ('p1','m1','abc',1000,1000,'{"type":"text","text":"hello from the model"}')`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	sessions, err := readOpenCode(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("sessions %d", len(sessions))
	}
	u := sessions[0].Usage
	if u.Input != 100 || u.Output != 20 || u.Reasoning != 3 || u.CacheRead != 7 || u.CacheWrite != 1 || u.CostUSD != 0.5 || u.Context != 80 {
		t.Fatalf("opencode usage %+v", u)
	}
}

package discover

import (
	"os"
	"path/filepath"
	"testing"
)

func isolate(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(root, "claude"))
	t.Setenv("CODEX_HOME", filepath.Join(root, "codex"))
	t.Setenv("GROK_HOME", filepath.Join(root, "grok"))
	t.Setenv("COPILOT_HOME", filepath.Join(root, "copilot"))
	t.Setenv("GEMINI_HOME", filepath.Join(root, "gemini"))
	t.Setenv("CURSOR_HOME", filepath.Join(root, "cursor"))
	t.Setenv("OPENCODE_DB", filepath.Join(root, "missing.db"))
	t.Setenv("HERMES_HOME", filepath.Join(root, "hermes"))
	t.Setenv("OPENCLAW_HOME", filepath.Join(root, "openclaw"))
	t.Setenv("OPENCLAW_STATE_DIR", filepath.Join(root, "openclaw"))
	t.Setenv("JUNIE_HOME", filepath.Join(root, "junie"))
	t.Setenv("JULES_HOME", filepath.Join(root, "jules"))
	t.Setenv("AIR9S_JULES_REMOTE", "0")
	t.Setenv("GOOSE_HOME", filepath.Join(root, "goose"))
	t.Setenv("CLINE_HOME", filepath.Join(root, "cline"))
	t.Setenv("KIRO_CLI_DB", filepath.Join(root, "missing-kiro.db"))
	t.Setenv("KIRO_HOME", filepath.Join(root, "kiro"))
	t.Setenv("AIDER_CHAT_ROOTS", filepath.Join(root, "aider-roots"))
	t.Setenv("AIDER_HOME", filepath.Join(root, "aider"))
	t.Setenv("AIDER_CHAT_HISTORY", filepath.Join(root, "no-aider.md"))
	t.Setenv("AIDER_SCAN_HOME", "0")
	return root
}

func TestScanAgyAndGemini(t *testing.T) {
	root := isolate(t)
	agy := filepath.Join(root, "gemini", "antigravity-cli", "history.jsonl")
	if err := os.MkdirAll(filepath.Dir(agy), 0o755); err != nil {
		t.Fatal(err)
	}
	hist := "{\"display\":\"ignore me\",\"timestamp\":1,\"workspace\":\"/work\"}\n" +
		"{\"conversationId\":\"c1\",\"display\":\"fix the tests\",\"timestamp\":1700000000000,\"type\":\"user\",\"workspace\":\"/work/app\"}\n"
	if err := os.WriteFile(agy, []byte(hist), 0o600); err != nil {
		t.Fatal(err)
	}
	chat := filepath.Join(root, "gemini", "tmp", "demo", "chats", "session-abc.json")
	if err := os.MkdirAll(filepath.Dir(chat), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "gemini", "projects.json"), []byte(`{"projects":{"/work/demo":"demo"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	body := `{"sessionId":"abc","lastUpdated":"2026-01-02T03:04:05Z","messages":[{"type":"info","content":"hello"},{"type":"user","content":"ship it"},{"type":"gemini","content":"done"}]}`
	if err := os.WriteFile(chat, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	var agyS, gemS int
	for _, b := range Collect(nil) {
		switch b.Agent {
		case "agy":
			if b.Err != nil {
				t.Fatal(b.Err)
			}
			agyS = len(b.Sessions)
			if agyS != 1 || b.Sessions[0].NativeID != "c1" || b.Sessions[0].CWD != "/work/app" || !b.Sessions[0].CanDelete {
				t.Fatalf("agy %+v", b.Sessions)
			}
		case "gemini":
			if b.Err != nil {
				t.Fatal(b.Err)
			}
			gemS = len(b.Sessions)
			if gemS != 1 || b.Sessions[0].CWD != "/work/demo" || b.Sessions[0].CanDelete || b.Sessions[0].Title != "ship it" {
				t.Fatalf("gemini %+v", b.Sessions)
			}
		}
	}
	if agyS != 1 || gemS != 1 {
		t.Fatalf("counts agy %d gemini %d", agyS, gemS)
	}
}

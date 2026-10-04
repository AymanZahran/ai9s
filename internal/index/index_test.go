package index

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AymanZahran/ai9s/internal/query"
	"github.com/AymanZahran/ai9s/internal/store"
)

func TestRebuildClaude(t *testing.T) {
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
	t.Setenv("AI9S_JULES_REMOTE", "0")
	t.Setenv("GOOSE_HOME", filepath.Join(root, "goose"))
	t.Setenv("CLINE_HOME", filepath.Join(root, "cline"))
	t.Setenv("KIRO_CLI_DB", filepath.Join(root, "missing-kiro.db"))
	t.Setenv("KIRO_HOME", filepath.Join(root, "kiro"))
	t.Setenv("AIDER_CHAT_ROOTS", filepath.Join(root, "aider-roots"))
	t.Setenv("AIDER_HOME", filepath.Join(root, "aider"))
	t.Setenv("AIDER_CHAT_HISTORY", filepath.Join(root, "no-aider.md"))
	t.Setenv("AIDER_SCAN_HOME", "0")
	t.Setenv("KIMI_CODE_HOME", filepath.Join(root, "kimi-code"))
	t.Setenv("QWEN_RUNTIME_DIR", filepath.Join(root, "qwen"))
	t.Setenv("QWEN_HOME", filepath.Join(root, "qwen-home"))
	t.Setenv("VIBE_HOME", filepath.Join(root, "vibe"))
	t.Setenv("MINIMAX_DATA_DIR", filepath.Join(root, "minimax"))
	t.Setenv("MAVIS_DATA_DIR", filepath.Join(root, "minimax-mavis"))

	proj := filepath.Join(root, "claude", "projects", "demo")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	line := `{"type":"user","cwd":"/work/demo","gitBranch":"main","timestamp":"2026-01-02T03:04:05Z","message":{"content":"ship the feature"}}` + "\n"
	if err := os.WriteFile(filepath.Join(proj, "abc.jsonl"), []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(root, "cache", "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	stats, warnings, err := Rebuild(st)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatal(warnings)
	}
	if stats.Sessions != 1 {
		t.Fatalf("stats %+v", stats)
	}
	got, err := st.Search(query.Parse("agent:claude ship"), 10)
	if err != nil || len(got) != 1 || got[0].Branch != "main" || got[0].CWD != "/work/demo" {
		t.Fatalf("found %+v %v", got, err)
	}
}

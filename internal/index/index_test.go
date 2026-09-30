package index

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AymanZahran/air9s/internal/query"
	"github.com/AymanZahran/air9s/internal/store"
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

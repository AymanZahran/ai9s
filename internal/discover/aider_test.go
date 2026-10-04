package discover

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanAiderHomeHistoryFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)
	t.Setenv("AIDER_HOME", "")
	t.Setenv("AIDER_CHAT_ROOTS", "")
	t.Setenv("AIDER_CHAT_HISTORY", "")
	t.Setenv("AIDER_SCAN_HOME", "")
	path := filepath.Join(root, ".aider.chat.history.md")
	if err := os.WriteFile(path, []byte("#### user\nhello from home\n#### assistant\nhi\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := scanAider(nil)
	if got.Err != nil || len(got.Sessions) != 1 || got.Sessions[0].SourcePath != path || got.Sessions[0].CWD != root {
		t.Fatalf("aider home %+v %v", got.Sessions, got.Err)
	}

	t.Setenv("AIDER_HOME", filepath.Join(root, "empty-aider"))
	if err := os.MkdirAll(os.Getenv("AIDER_HOME"), 0o755); err != nil {
		t.Fatal(err)
	}
	got = scanAider(nil)
	if got.Err != nil || len(got.Sessions) != 0 {
		t.Fatalf("aider home leaked %+v %v", got.Sessions, got.Err)
	}
}

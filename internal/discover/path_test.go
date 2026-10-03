package discover

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecodeDashedPath(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, "work", "echo-news")
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatal(err)
	}
	dashed := strings.ReplaceAll(want, string(os.PathSeparator), "-")
	if got := decodeDashedPath(dashed); got != want {
		t.Fatalf("leading dash: got %q want %q", got, want)
	}
	trimmed := strings.TrimPrefix(dashed, "-")
	if got := decodeDashedPath(trimmed); got != want {
		t.Fatalf("no leading dash: got %q want %q", got, want)
	}
	if got := decodeDashedPath("no-such-directory-ai9s"); got != "" {
		t.Fatalf("missing path returned %q", got)
	}
}

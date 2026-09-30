package query

import "testing"

func TestParse(t *testing.T) {
	f := Parse(`agent:claude dir:echo date:<7d sort:messages "auth bug"`)
	if f.Agent != "claude" || f.Dir != "echo" || f.Sort != "messages" {
		t.Fatalf("fields: %+v", f)
	}
	if f.Since.IsZero() {
		t.Fatal("expected since")
	}
	if f.Text != "auth bug" {
		t.Fatalf("text %q", f.Text)
	}
	if got := FTS(`hello "world!"`); got != `"hello" AND "world"` {
		t.Fatalf("fts %q", got)
	}
}

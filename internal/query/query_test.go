package query

import (
	"strings"
	"testing"
)

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

func TestParsePartialAndHome(t *testing.T) {
	t.Setenv("HOME", "/Users/ada")
	empty := Parse("agent:")
	if empty.Agent != "" || empty.Text != "" {
		t.Fatalf("empty token %+v", empty)
	}
	got := Parse(`agent:clau dir:~/work ~/notes`)
	if got.Agent != "clau" || got.Dir != "/Users/ada/work" {
		t.Fatalf("fields %+v", got)
	}
	if !strings.Contains(got.Text, "/Users/ada/notes") {
		t.Fatalf("text %q", got.Text)
	}
	bogus := Parse("sort:bogus date:nope")
	if bogus.Sort != "recent" || bogus.Text != "" {
		t.Fatalf("swallowed %+v", bogus)
	}
}

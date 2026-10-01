package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AymanZahran/air9s/internal/model"
	"github.com/AymanZahran/air9s/internal/query"
)

func TestIndexFileMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.db")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	st.Close()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("index mode %o", fi.Mode().Perm())
	}
}

func TestSearchAndPrune(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	when := time.Date(2026, 3, 2, 15, 4, 5, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "abc.jsonl")
	sess := model.Session{
		ID: "claude:abc", NativeID: "abc", Agent: "claude", Title: "ship the feature",
		CWD: "/work/demo", Branch: "main", Model: "sonnet", Updated: when, Messages: 2,
		SourcePath: path, SourceMtime: 10, CanDelete: true, DeleteMode: "file",
		Usage:    model.Usage{Context: 1200, Output: 30, Effort: "low"},
		Snippets: []model.Snippet{{Role: "user", Body: "please fix the auth bug"}},
	}
	if err := st.Apply("claude", []model.Session{sess}, []Source{{Path: path, Mtime: 10, Fresh: false}}); err != nil {
		t.Fatal(err)
	}
	got, err := st.Search(query.Parse("auth agent:claude dir:demo sort:messages"), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "claude:abc" || got[0].Usage.Context != 1200 {
		t.Fatalf("search %+v", got)
	}
	full, err := st.Get("claude:abc")
	if err != nil || len(full.Snippets) != 1 || full.Usage.Output != 30 || full.Usage.Effort != "low" {
		t.Fatalf("get %+v %v", full, err)
	}
	if !st.Fresh(path, 10) {
		t.Fatal("expected fresh stamp")
	}
	if err := st.Apply("claude", nil, nil); err != nil {
		t.Fatal(err)
	}
	got, err = st.Search(query.Parse(""), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("prune left %+v", got)
	}
}

func TestSearchSubstrings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	st, err := Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	path := filepath.Join(t.TempDir(), "abc.jsonl")
	cwd := filepath.Join(home, "work", "demo")
	sess := model.Session{
		ID: "claude:abc", NativeID: "abc", Agent: "claude", Title: "ship the feature",
		CWD: cwd, Branch: "main", Model: "sonnet",
		Updated: time.Date(2026, 3, 2, 15, 4, 5, 0, time.UTC), Messages: 2,
		SourcePath: path, CanDelete: true, DeleteMode: "file",
		Snippets: []model.Snippet{{Role: "user", Body: "please fix the auth bug"}},
	}
	if err := st.Apply("claude", []model.Session{sess}, []Source{{Path: path, Mtime: 10}}); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"fea", "agent:clau", "dir:~/work", "main", "sonn", "agent:"} {
		got, err := st.Search(query.Parse(q), 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].ID != "claude:abc" {
			t.Fatalf("%s -> %+v", q, got)
		}
	}
	got, err := st.Search(query.Parse("agent:grok"), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("agent prefix matched %+v", got)
	}
}

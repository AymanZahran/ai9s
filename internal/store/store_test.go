package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/AymanZahran/ai9s/internal/model"
	"github.com/AymanZahran/ai9s/internal/query"
)

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
		Snippets: []model.Snippet{{Role: "user", Body: "please fix the auth bug"}},
	}
	if err := st.Apply("claude", []model.Session{sess}, []Source{{Path: path, Mtime: 10, Fresh: false}}); err != nil {
		t.Fatal(err)
	}
	got, err := st.Search(query.Parse("auth agent:claude dir:demo sort:messages"), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "claude:abc" {
		t.Fatalf("search %+v", got)
	}
	full, err := st.Get("claude:abc")
	if err != nil || len(full.Snippets) != 1 {
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

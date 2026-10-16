package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/AymanZahran/ai9s/internal/model"
)

func TestForkSurvivesReindex(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	parentPath := filepath.Join(t.TempDir(), "parent.jsonl")
	childPath := filepath.Join(t.TempDir(), "child.jsonl")
	otherPath := filepath.Join(t.TempDir(), "other.jsonl")
	parent := model.Session{
		ID: "claude:parent", NativeID: "parent", Agent: "claude", Title: "recorded title",
		Updated: time.Date(2026, 3, 2, 15, 4, 5, 0, time.UTC), Messages: 2,
		SourcePath: parentPath, SourceMtime: 1, CanDelete: true, DeleteMode: "file",
	}
	child := parent
	child.ID, child.NativeID, child.SourcePath = "claude:child", "child", childPath
	other := parent
	other.ID, other.NativeID, other.Title, other.SourcePath = "claude:other", "other", "second", otherPath
	files := []Source{{Path: parentPath, Mtime: 1}, {Path: childPath, Mtime: 1}, {Path: otherPath, Mtime: 1}}
	if err := st.Apply("claude", []model.Session{parent, child, other}, files); err != nil {
		t.Fatal(err)
	}
	if err := st.SetFork("claude:child", "claude:parent"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetFork("claude:other", "claude:parent"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetFork("missing", "claude:parent"); err == nil {
		t.Fatal("missing child")
	}
	if err := st.SetFork("claude:parent", "claude:parent"); err == nil {
		t.Fatal("self fork")
	}
	parent.Title = "rewritten"
	if err := st.Apply("claude", []model.Session{parent, child, other}, files); err != nil {
		t.Fatal(err)
	}
	got, err := st.Get("claude:parent")
	if err != nil || got.Forks != 2 || got.Title != "rewritten" {
		t.Fatalf("parent %+v %v", got, err)
	}
	got, err = st.Get("claude:child")
	if err != nil || got.Parent != "rewritten" || got.Forks != 0 {
		t.Fatalf("child %+v %v", got, err)
	}
	if err := st.SetName("claude:parent", "Alpha"); err != nil {
		t.Fatal(err)
	}
	got, err = st.Get("claude:child")
	if err != nil || got.Parent != "Alpha" {
		t.Fatalf("named parent %+v %v", got, err)
	}
	if err := st.Forget("claude:other"); err != nil {
		t.Fatal(err)
	}
	got, err = st.Get("claude:parent")
	if err != nil || got.Forks != 1 {
		t.Fatalf("after forget %+v %v", got, err)
	}
	if err := st.Apply("claude", []model.Session{child}, []Source{{Path: childPath, Mtime: 2}}); err != nil {
		t.Fatal(err)
	}
	got, err = st.Get("claude:child")
	if err != nil || got.Parent != "claude:parent" {
		t.Fatalf("parent gone %+v %v", got, err)
	}
	if _, err := st.Get("claude:parent"); err == nil {
		t.Fatal("parent still indexed")
	}
}

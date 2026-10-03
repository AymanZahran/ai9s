package store

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AymanZahran/ai9s/internal/model"
	"github.com/AymanZahran/ai9s/internal/query"
)

func TestNameSurvivesReindex(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	path := filepath.Join(t.TempDir(), "abc.jsonl")
	sess := model.Session{
		ID: "claude:abc", NativeID: "abc", Agent: "claude", Title: "recorded title",
		CWD: "/work/demo", Updated: time.Date(2026, 3, 2, 15, 4, 5, 0, time.UTC), Messages: 4,
		SourcePath: path, SourceMtime: 10, CanDelete: true, DeleteMode: "file",
		Usage: model.Usage{Context: 1500, Window: 8000, Output: 20},
	}
	if err := st.Apply("claude", []model.Session{sess}, []Source{{Path: path, Mtime: 10}}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetName("claude:abc", "  Alpha  "); err != nil {
		t.Fatal(err)
	}
	got, err := st.Get("claude:abc")
	if err != nil || got.Name != "Alpha" || got.Title != "recorded title" {
		t.Fatalf("named %+v %v", got, err)
	}
	sess.Title = "rewritten by the agent"
	sess.Messages = 5
	if err := st.Apply("claude", []model.Session{sess}, []Source{{Path: path, Mtime: 11}}); err != nil {
		t.Fatal(err)
	}
	got, err = st.Get("claude:abc")
	if err != nil || got.Name != "Alpha" || got.Title != "rewritten by the agent" || got.Messages != 5 {
		t.Fatalf("after reindex %+v %v", got, err)
	}
	found, err := st.Search(query.Parse("Alpha"), 10)
	if err != nil || len(found) != 1 || found[0].Name != "Alpha" {
		t.Fatalf("search name %+v %v", found, err)
	}
	if err := st.SetName("claude:abc", "   "); err != nil {
		t.Fatal(err)
	}
	got, err = st.Get("claude:abc")
	if err != nil || got.Name != "" || got.Title != "rewritten by the agent" {
		t.Fatalf("cleared %+v %v", got, err)
	}
	if err := st.SetName("missing", "nope"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing %v", err)
	}
	long := strings.Repeat("n", MaxSessionName+1)
	if err := st.SetName("claude:abc", long); err == nil {
		t.Fatal("accepted a long name")
	}
	if err := st.SetName("claude:abc", "safe\u202eevil\nname"); err != nil {
		t.Fatal(err)
	}
	got, err = st.Get("claude:abc")
	if err != nil || got.Name != "safeevilname" {
		t.Fatalf("controls %+v %v", got, err)
	}
}

func TestNameDroppedWhenSessionGoes(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	path := filepath.Join(t.TempDir(), "abc.jsonl")
	sess := model.Session{
		ID: "claude:abc", NativeID: "abc", Agent: "claude", Title: "recorded",
		Updated: time.Date(2026, 3, 2, 15, 4, 5, 0, time.UTC), Messages: 1,
		SourcePath: path, CanDelete: true, DeleteMode: "file",
	}
	if err := st.Apply("claude", []model.Session{sess}, []Source{{Path: path, Mtime: 10}}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetName(sess.ID, "kept"); err != nil {
		t.Fatal(err)
	}
	if err := st.Forget(sess.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.Apply("claude", []model.Session{sess}, []Source{{Path: path, Mtime: 11}}); err != nil {
		t.Fatal(err)
	}
	got, err := st.Get(sess.ID)
	if err != nil || got.Name != "" {
		t.Fatalf("forget left %+v %v", got, err)
	}
	if err := st.SetName(sess.ID, "gone"); err != nil {
		t.Fatal(err)
	}
	if err := st.Apply("claude", nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := st.Apply("claude", []model.Session{sess}, []Source{{Path: path, Mtime: 12}}); err != nil {
		t.Fatal(err)
	}
	got, err = st.Get(sess.ID)
	if err != nil || got.Name != "" {
		t.Fatalf("prune left %+v %v", got, err)
	}
}

func TestTitleSortUsesCustomName(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	when := time.Date(2026, 3, 2, 15, 4, 5, 0, time.UTC)
	mk := func(id, title, path string) model.Session {
		return model.Session{
			ID: id, NativeID: id, Agent: "claude", Title: title,
			Updated: when, Messages: 1, SourcePath: path,
			CanDelete: true, DeleteMode: "file",
		}
	}
	dir := t.TempDir()
	a := mk("claude:a", "aaa", filepath.Join(dir, "a.jsonl"))
	b := mk("claude:b", "mmm", filepath.Join(dir, "b.jsonl"))
	if err := st.Apply("claude", []model.Session{a, b}, []Source{{Path: a.SourcePath, Mtime: 1}, {Path: b.SourcePath, Mtime: 1}}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetName(a.ID, "zzz"); err != nil {
		t.Fatal(err)
	}
	got, err := st.Search(query.Parse("sort:title"), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != b.ID || got[1].ID != a.ID {
		t.Fatalf("order %+v", got)
	}
}

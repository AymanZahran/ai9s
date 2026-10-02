package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AymanZahran/air9s/internal/model"
	"github.com/AymanZahran/air9s/internal/query"
	_ "modernc.org/sqlite"
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

func TestFreshRereadsDisabledDelete(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	path := filepath.Join(t.TempDir(), "summary.json")
	sess := model.Session{
		ID: "grok:old", NativeID: "old", Agent: "grok", Title: "kept",
		Updated: time.Date(2026, 3, 2, 15, 4, 5, 0, time.UTC), Messages: 1,
		SourcePath: path, SourceMtime: 10, CanDelete: false, DeleteReason: "disabled",
	}
	if err := st.Apply("grok", []model.Session{sess}, storeSource(path)); err != nil {
		t.Fatal(err)
	}
	if st.Fresh(path, 10) {
		t.Fatal("a session indexed with delete disabled should be read again")
	}
	sess.CanDelete = true
	sess.DeleteMode = "grok"
	sess.DeleteReason = ""
	if err := st.Apply("grok", []model.Session{sess}, storeSource(path)); err != nil {
		t.Fatal(err)
	}
	if !st.Fresh(path, 10) {
		t.Fatal("a session with delete enabled should stay fresh")
	}
	got, err := st.Get("grok:old")
	if err != nil || !got.CanDelete || got.DeleteMode != "grok" || got.DeleteReason != "" {
		t.Fatalf("refreshed %+v %v", got, err)
	}
}

func TestFreshRereadsAgyName(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	path := filepath.Join(t.TempDir(), "history.jsonl")
	old := model.Session{
		ID: "agy:c1", NativeID: "c1", Agent: "agy", Title: "old name",
		Updated: time.Date(2026, 3, 2, 15, 4, 5, 0, time.UTC), Messages: 1,
		SourcePath: path, SourceMtime: 10, CanDelete: true, DeleteMode: "rewrite",
	}
	if err := st.Apply("agy", []model.Session{old}, storeSource(path)); err != nil {
		t.Fatal(err)
	}
	if st.Fresh(path, 10) {
		t.Fatal("a session stored as agy should be read again")
	}
	next := old
	next.ID = "antigravity:c1"
	next.Agent = "antigravity"
	if err := st.Apply("antigravity", []model.Session{next}, storeSource(path)); err != nil {
		t.Fatal(err)
	}
	if !st.Fresh(path, 10) {
		t.Fatal("antigravity should stay fresh")
	}
	if _, err := st.Get("agy:c1"); err == nil {
		t.Fatal("old agy id should be gone")
	}
	got, err := st.Get("antigravity:c1")
	if err != nil || got.Agent != "antigravity" {
		t.Fatalf("renamed %+v %v", got, err)
	}
}

func storeSource(path string) []Source {
	return []Source{{Path: path, Mtime: 10}}
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

func TestRevisionRereadsSources(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE files (path TEXT PRIMARY KEY, mtime INTEGER NOT NULL, agent TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO files VALUES ('src', 9, 'grok')`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	var mt int64
	if err := st.db.QueryRow(`SELECT mtime FROM files WHERE path = 'src'`).Scan(&mt); err != nil || mt != 0 {
		t.Fatalf("mtime %d %v", mt, err)
	}
	if _, err := st.db.Exec(`UPDATE files SET mtime = 4 WHERE path = 'src'`); err != nil {
		t.Fatal(err)
	}
	st.Close()
	st, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.db.QueryRow(`SELECT mtime FROM files WHERE path = 'src'`).Scan(&mt); err != nil || mt != 4 {
		t.Fatalf("kept mtime %d %v", mt, err)
	}
}

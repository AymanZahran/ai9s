package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/AymanZahran/ai9s/internal/model"
	"github.com/AymanZahran/ai9s/internal/query"
)

func TestSearchRoots(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	when := time.Date(2026, 3, 2, 15, 4, 5, 0, time.UTC)
	dir := t.TempDir()
	rows := []model.Session{
		{ID: "claude:root", NativeID: "root", CWD: "/work/app"},
		{ID: "claude:child", NativeID: "child", CWD: "/work/app/pkg"},
		{ID: "claude:slash", NativeID: "slash", CWD: "/work/app/"},
		{ID: "claude:sibling", NativeID: "sibling", CWD: "/work/application"},
		{ID: "claude:other", NativeID: "other", CWD: "/tmp/other"},
		{ID: "claude:blank", NativeID: "blank", CWD: ""},
		{ID: "claude:score", NativeID: "score", CWD: "/work/app_name"},
		{ID: "claude:wild", NativeID: "wild", CWD: "/work/appXname"},
		{ID: "claude:scored", NativeID: "scored", CWD: "/work/app_name/lib"},
	}
	sources := make([]Source, len(rows))
	for i := range rows {
		rows[i].Agent = "claude"
		rows[i].Title = rows[i].NativeID
		rows[i].Updated = when
		rows[i].Messages = 1
		rows[i].SourcePath = filepath.Join(dir, rows[i].NativeID)
		rows[i].CanDelete = true
		rows[i].DeleteMode = "file"
		sources[i] = Source{Path: rows[i].SourcePath, Mtime: 1}
	}
	if err := st.Apply("claude", rows, sources); err != nil {
		t.Fatal(err)
	}
	got := searchIDs(t, st, query.Filter{Sort: "recent", Roots: []string{"/work/app"}})
	for _, id := range []string{"claude:root", "claude:child", "claude:slash"} {
		if !got[id] {
			t.Fatalf("missing %s in %v", id, got)
		}
	}
	for _, id := range []string{"claude:sibling", "claude:other", "claude:blank", "claude:score", "claude:wild"} {
		if got[id] {
			t.Fatalf("included %s in %v", id, got)
		}
	}
	scored := searchIDs(t, st, query.Filter{Sort: "recent", Roots: []string{"/work/app_name"}})
	if !scored["claude:score"] || !scored["claude:scored"] || scored["claude:wild"] || scored["claude:root"] {
		t.Fatalf("underscore %v", scored)
	}
	narrow := query.Parse("dir:pkg")
	narrow.Roots = []string{"/work/app"}
	got = searchIDs(t, st, narrow)
	if len(got) != 1 || !got["claude:child"] {
		t.Fatalf("narrow %v", got)
	}
	all := searchIDs(t, st, query.Filter{Sort: "recent"})
	if len(all) != len(rows) {
		t.Fatalf("unscoped %d", len(all))
	}
}

func searchIDs(t *testing.T, st *Store, f query.Filter) map[string]bool {
	t.Helper()
	rows, err := st.Search(f, 20)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, row := range rows {
		got[row.ID] = true
	}
	return got
}

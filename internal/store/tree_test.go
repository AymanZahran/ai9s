package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/AymanZahran/ai9s/internal/model"
	"github.com/AymanZahran/ai9s/internal/query"
)

func TestParentRoundTripAndTree(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	base := time.Date(2026, 3, 2, 15, 0, 0, 0, time.UTC)
	rows := []model.Session{
		treeSess("claude:parent", "parent session", base.Add(2*time.Hour), "", 2),
		treeSess("claude:child", "widget match", base.Add(4*time.Hour), "claude:parent", 3),
		treeSess("claude:sibling", "other task", base.Add(3*time.Hour), "claude:parent", 1),
		treeSess("claude:older", "older root", base.Add(time.Hour), "", 1),
		treeSess("claude:alpha", "alpha root", base.Add(5*time.Hour), "", 1),
		treeSess("claude:bravo", "bravo root", base.Add(6*time.Hour), "", 1),
		treeSess("claude:orphan", "orphan root", base, "claude:missing", 1),
		treeSess("claude:cycle-a", "cycle a", base.Add(30*time.Minute), "claude:cycle-b", 1),
		treeSess("claude:cycle-b", "cycle b", base.Add(20*time.Minute), "claude:cycle-a", 1),
		treeSess("claude:fork-src", "fork source", base.Add(10*time.Minute), "", 1),
	}
	if err := st.Apply("claude", rows, treeSources(rows)); err != nil {
		t.Fatal(err)
	}
	got, err := st.Get("claude:child")
	if err != nil || got.ParentID != "claude:parent" || got.ParentName != "parent session" {
		t.Fatalf("get %+v %v", got, err)
	}
	if err := st.SetName("claude:parent", "Given Name"); err != nil {
		t.Fatal(err)
	}
	got, err = st.Get("claude:child")
	if err != nil || got.ParentName != "Given Name" {
		t.Fatalf("name %+v %v", got, err)
	}
	if err := st.SetFork("claude:child", "claude:fork-src"); err != nil {
		t.Fatal(err)
	}
	got, err = st.Get("claude:child")
	if err != nil || got.Parent != "fork source" || got.ParentID != "claude:parent" {
		t.Fatalf("fork and parent %+v %v", got, err)
	}
	flat, err := st.Search(query.Parse(""), 100)
	if err != nil {
		t.Fatal(err)
	}
	if !hasID(flat, "claude:child") {
		t.Fatal("search omitted the subagent")
	}

	collapsed, sessions, messages, err := st.SessionTree(query.Filter{}, 100, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sessions != len(rows) || messages != 13 {
		t.Fatalf("counts sessions %d messages %d", sessions, messages)
	}
	if ids := treeIDs(collapsed); containsID(ids, "claude:child") || containsID(ids, "claude:sibling") {
		t.Fatalf("collapsed %+v", ids)
	}
	parentRow := findTree(collapsed, "claude:parent")
	if parentRow.Children != 2 || parentRow.Open || parentRow.Depth != 0 {
		t.Fatalf("parent row %+v", parentRow)
	}
	if findTree(collapsed, "claude:orphan").Depth != 0 || findTree(collapsed, "claude:cycle-a").Children != 0 || findTree(collapsed, "claude:cycle-b").Depth != 0 {
		t.Fatalf("roots %+v", treeIDs(collapsed))
	}

	open := map[string]bool{"claude:parent": true}
	expanded, _, _, err := st.SessionTree(query.Filter{}, 100, open)
	if err != nil {
		t.Fatal(err)
	}
	if child := findTree(expanded, "claude:child"); child.Depth != 1 || !findTree(expanded, "claude:parent").Open {
		t.Fatalf("expanded %+v", treeIDs(expanded))
	}
	if treeIDs(expanded)[indexID(treeIDs(expanded), "claude:parent")+1] != "claude:sibling" && treeIDs(expanded)[indexID(treeIDs(expanded), "claude:parent")+1] != "claude:child" {
		t.Fatalf("child not under parent %+v", treeIDs(expanded))
	}

	limited, _, _, err := st.SessionTree(query.Filter{}, 2, map[string]bool{"claude:parent": true})
	if err != nil {
		t.Fatal(err)
	}
	limIDs := treeIDs(limited)
	if containsID(limIDs, "claude:child") || limIDs[0] != "claude:bravo" || limIDs[1] != "claude:alpha" {
		t.Fatalf("limit %+v", limIDs)
	}

	textRows, textSessions, textMessages, err := st.SessionTree(query.Parse("widget"), 100, nil)
	if err != nil {
		t.Fatal(err)
	}
	if textSessions != 1 || textMessages != 3 {
		t.Fatalf("text counts %d %d", textSessions, textMessages)
	}
	textIDs := treeIDs(textRows)
	if len(textIDs) != 2 || textIDs[0] != "claude:parent" || textIDs[1] != "claude:child" || !findTree(textRows, "claude:parent").Open || findTree(textRows, "claude:child").Depth != 1 {
		t.Fatalf("text %+v", textIDs)
	}
	manual, _, _, err := st.SessionTree(query.Parse("widget"), 100, open)
	if err != nil {
		t.Fatal(err)
	}
	if !containsID(treeIDs(manual), "claude:sibling") {
		t.Fatalf("manual expand %+v", treeIDs(manual))
	}

	grokRows := []model.Session{
		treeSess("grok:parent", "grok parent", base, "", 1),
		treeSess("grok:child", "grok child", base, "", 1),
	}
	grokRows[0].Agent = "grok"
	grokRows[1].Agent = "grok"
	if err := st.Apply("grok", grokRows, treeSources(grokRows)); err != nil {
		t.Fatal(err)
	}
	if err := st.ApplyParents("grok", map[string]string{"grok:child": "grok:parent"}); err != nil {
		t.Fatal(err)
	}
	got, err = st.Get("grok:child")
	if err != nil || got.ParentID != "grok:parent" || got.ParentName != "grok parent" {
		t.Fatalf("apply parents %+v %v", got, err)
	}
	if err := st.ApplyParents("grok", map[string]string{}); err != nil {
		t.Fatal(err)
	}
	got, err = st.Get("grok:child")
	if err != nil || got.ParentID != "" {
		t.Fatalf("empty map %+v %v", got, err)
	}
	if err := st.ApplyParents("grok", map[string]string{"grok:child": "grok:parent"}); err != nil {
		t.Fatal(err)
	}
	if err := st.ApplyParents("grok", nil); err != nil {
		t.Fatal(err)
	}
	got, err = st.Get("grok:child")
	if err != nil || got.ParentID != "grok:parent" {
		t.Fatalf("nil map %+v %v", got, err)
	}
	if err := st.ApplyParents("codex", map[string]string{}); err != nil {
		t.Fatal(err)
	}
	got, err = st.Get("grok:child")
	if err != nil || got.ParentID != "grok:parent" {
		t.Fatalf("other agent %+v %v", got, err)
	}
	missing, err := st.Get("claude:orphan")
	if err != nil || missing.ParentName != "claude:missing" {
		t.Fatalf("missing parent name %+v %v", missing, err)
	}
}

func treeSess(id, title string, updated time.Time, parent string, messages int) model.Session {
	native := id
	if i := len("claude:"); len(id) > i && id[:i] == "claude:" {
		native = id[i:]
	}
	if i := len("grok:"); len(id) > i && id[:i] == "grok:" {
		native = id[i:]
	}
	agent := "claude"
	if len(id) > 5 && id[:5] == "grok:" {
		agent = "grok"
	}
	return model.Session{
		ID: id, NativeID: native, Agent: agent, Title: title, Updated: updated,
		Messages: messages, SourcePath: id, ParentID: parent,
		CanDelete: true, DeleteMode: "file",
	}
}

func treeSources(rows []model.Session) []Source {
	out := make([]Source, len(rows))
	for i, row := range rows {
		out[i] = Source{Path: row.SourcePath, Mtime: 1}
	}
	return out
}

func treeIDs(rows []TreeRow) []string {
	out := make([]string, len(rows))
	for i, row := range rows {
		out[i] = row.Session.ID
	}
	return out
}

func findTree(rows []TreeRow, id string) TreeRow {
	for _, row := range rows {
		if row.Session.ID == id {
			return row
		}
	}
	return TreeRow{}
}

func containsID(ids []string, id string) bool {
	for _, got := range ids {
		if got == id {
			return true
		}
	}
	return false
}

func indexID(ids []string, id string) int {
	for i, got := range ids {
		if got == id {
			return i
		}
	}
	return -1
}

func hasID(rows []model.Session, id string) bool {
	for _, row := range rows {
		if row.ID == id {
			return true
		}
	}
	return false
}

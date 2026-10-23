package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AymanZahran/ai9s/internal/config"
	"github.com/AymanZahran/ai9s/internal/model"
	"github.com/AymanZahran/ai9s/internal/store"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestTreeLabel(t *testing.T) {
	if got := treeLabel("ship", 0, 2, false); got != "▶ 2 ship" {
		t.Fatalf("collapsed %q", got)
	}
	if got := treeLabel("ship", 0, 2, true); got != "▼ ship" {
		t.Fatalf("open %q", got)
	}
	if got := treeLabel("child", 1, 0, false); got != "  └ child" {
		t.Fatalf("child %q", got)
	}
	if got := treeLabel("deep", 2, 1, false); got != "    └ ▶ 1 deep" {
		t.Fatalf("deep %q", got)
	}
}

func TestSessionTreeKeys(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	base := time.Date(2026, 3, 2, 15, 0, 0, 0, time.UTC)
	parent := model.Session{
		ID: "claude:parent-1", NativeID: "parent-1", Agent: "claude", Title: "parent session",
		Updated: base.Add(time.Hour), Messages: 2, SourcePath: "parent-1",
		CanDelete: true, DeleteMode: "file",
	}
	child := model.Session{
		ID: "claude:agent-abc", NativeID: "agent-abc", Agent: "claude", Title: "child task",
		Updated: base.Add(2 * time.Hour), Messages: 1, SourcePath: "agent-abc",
		ParentID: "claude:parent-1", CanDelete: true, DeleteMode: "file",
	}
	older := model.Session{
		ID: "claude:older", NativeID: "older", Agent: "claude", Title: "older root",
		Updated: base, Messages: 1, SourcePath: "older",
		CanDelete: true, DeleteMode: "file",
	}
	rows := []model.Session{parent, child, older}
	sources := make([]store.Source, len(rows))
	for i, row := range rows {
		sources[i] = store.Source{Path: row.SourcePath, Mtime: 1}
	}
	if err := st.Apply("claude", rows, sources); err != nil {
		t.Fatal(err)
	}
	if err := st.SetBookmark(child.ID, true); err != nil {
		t.Fatal(err)
	}
	app := tview.NewApplication()
	ui := newUI(app, st, config.Defaults())
	ui.reload()
	menu, _ := ui.menuText()
	if !strings.Contains(menu, "space") || !strings.Contains(menu, "expand") {
		t.Fatalf("menu %s", menu)
	}
	if len(ui.rows) != 2 || ui.rows[0].ID != parent.ID || ui.rows[1].ID != older.ID || ui.treeKids[0] != 1 || ui.treeOpen[0] {
		t.Fatalf("collapsed %+v kids %v", sessionIDs(ui.rows), ui.treeKids)
	}
	if text := strings.Join(ui.lines, "\n"); !strings.Contains(text, "▶ 1") || !strings.Contains(text, "parent session") || strings.Contains(text, "child task") {
		t.Fatalf("lines %s", text)
	}

	screen, _ := startApp(t, ui)
	defer ui.app.Stop()
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	waitUI(t, ui.app, func() bool {
		return len(ui.rows) == 3 && ui.rows[0].ID == parent.ID && ui.rows[1].ID == child.ID && ui.rows[2].ID == older.ID && ui.treeOpen[0]
	})
	if text := strings.Join(ui.lines, "\n"); !strings.Contains(text, "▼") || !strings.Contains(text, "└") || !strings.Contains(text, "child task") {
		t.Fatalf("expanded %s", text)
	}

	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModNone))
	waitUI(t, ui.app, func() bool {
		row, _ := ui.table.GetSelection()
		return row == 2
	})
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModNone))
	waitUI(t, ui.app, func() bool {
		return ui.describing() && strings.Contains(screenText(screen), "subagent of parent session") && strings.Contains(screenText(screen), "enter resumes the parent")
	})

	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyRune, '6', tcell.ModNone))
	waitUI(t, ui.app, func() bool {
		menu, _ := ui.menuText()
		text := strings.Join(ui.lines, "\n")
		return ui.view == viewBookmarks && len(ui.rows) == 1 && ui.rows[0].ID == child.ID && !strings.Contains(text, "▶") && !strings.Contains(menu, "expand")
	})
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModNone))
	waitUI(t, ui.app, func() bool {
		return ui.describing() && strings.Contains(screenText(screen), "subagent of parent session")
	})
}

func sessionIDs(rows []model.Session) []string {
	out := make([]string, len(rows))
	for i, row := range rows {
		out[i] = row.ID
	}
	return out
}

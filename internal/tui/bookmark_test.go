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

func TestBookmarkAndCost(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	path := filepath.Join(t.TempDir(), "abc.jsonl")
	sess := model.Session{
		ID: "claude:abc", NativeID: "abc", Agent: "claude", Title: "recorded title",
		Updated: time.Date(2026, 3, 2, 15, 4, 5, 0, time.UTC), Messages: 4,
		SourcePath: path, CanDelete: true, DeleteMode: "file",
		Usage: model.Usage{CostUSD: 1.25, Total: 100},
	}
	if err := st.Apply("claude", []model.Session{sess}, []store.Source{{Path: path, Mtime: 1}}); err != nil {
		t.Fatal(err)
	}
	app := tview.NewApplication()
	ui := newUI(app, st, config.Defaults())
	ui.reload()
	screen, _ := startApp(t, ui)
	menu, _ := ui.menuText()
	if !strings.Contains(menu, "<f>") || !strings.Contains(menu, "bookmark") || strings.Contains(menu, "reindex") {
		t.Fatalf("menu %s", menu)
	}
	waitUI(t, ui.app, func() bool {
		painted := screenText(screen)
		return strings.Contains(painted, "COST") && strings.Contains(painted, "$1.25")
	})

	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModNone))
	waitUI(t, ui.app, func() bool {
		return len(ui.rows) == 1 && ui.rows[0].Bookmarked
	})
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModNone))
	waitUI(t, ui.app, func() bool {
		return ui.describing() && strings.Contains(screenText(screen), "bookmark   yes")
	})

	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyRune, '2', tcell.ModNone))
	waitUI(t, ui.app, func() bool { return ui.view == viewProviders })
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModNone))
	waitUI(t, ui.app, func() bool {
		return strings.Contains(screenText(screen), "Switch to sessions before bookmarking.")
	})
}

func TestBookmarksView(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	keptPath := filepath.Join(t.TempDir(), "kept.jsonl")
	otherPath := filepath.Join(t.TempDir(), "other.jsonl")
	kept := model.Session{
		ID: "claude:kept", NativeID: "kept", Agent: "claude", Title: "kept session",
		Updated: time.Date(2026, 3, 2, 15, 4, 5, 0, time.UTC), Messages: 2,
		SourcePath: keptPath, CanDelete: true, DeleteMode: "file",
	}
	other := model.Session{
		ID: "claude:other", NativeID: "other", Agent: "claude", Title: "other session",
		Updated: time.Date(2026, 3, 1, 15, 4, 5, 0, time.UTC), Messages: 1,
		SourcePath: otherPath, CanDelete: true, DeleteMode: "file",
	}
	if err := st.Apply("claude", []model.Session{kept, other}, []store.Source{{Path: keptPath, Mtime: 1}, {Path: otherPath, Mtime: 1}}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetBookmark(kept.ID, true); err != nil {
		t.Fatal(err)
	}
	app := tview.NewApplication()
	ui := newUI(app, st, config.Defaults())
	ui.reload()
	app.SetRoot(ui.layout, false)
	menu, _ := ui.menuText()
	if !strings.Contains(menu, "<6>") || !strings.Contains(menu, "bookmarks") {
		t.Fatalf("menu %s", menu)
	}
	if len(ui.rows) != 2 {
		t.Fatalf("rows %d", len(ui.rows))
	}

	send(ui.table, tcell.NewEventKey(tcell.KeyRune, '6', tcell.ModNone))
	if ui.view != viewBookmarks || ui.table.GetTitle() != " bookmarks " || len(ui.rows) != 1 || ui.rows[0].ID != kept.ID {
		t.Fatalf("view %s title %q rows %+v", ui.view, ui.table.GetTitle(), ui.rows)
	}
	if ui.filter.GetText() != "" {
		t.Fatalf("filter %q", ui.filter.GetText())
	}
	if !strings.Contains(ui.crumbs.GetText(true), "Bookmarks") {
		t.Fatalf("crumbs %q", ui.crumbs.GetText(true))
	}
	enter := ""
	for _, hint := range ui.actionHints("x", "y") {
		if hint.key == "enter" {
			enter = hint.label
		}
	}
	if enter != "resume" {
		t.Fatalf("enter %q", enter)
	}

	send(ui.table, tcell.NewEventKey(tcell.KeyRune, '1', tcell.ModNone))
	if ui.view != viewSessions || len(ui.rows) != 2 {
		t.Fatalf("back view %s rows %d", ui.view, len(ui.rows))
	}

	ui.filter.SetText("agent:claude mark:no")
	ui.applyCommand(":bookmarks")
	if ui.view != viewBookmarks || ui.filter.GetText() != "agent:claude mark:no" || len(ui.rows) != 1 || ui.rows[0].ID != kept.ID {
		t.Fatalf("colon view %s filter %q rows %+v", ui.view, ui.filter.GetText(), ui.rows)
	}
	ui.applyCommand("bookmarks")
	if ui.view != viewBookmarks || len(ui.rows) != 1 {
		t.Fatalf("name view %s rows %d", ui.view, len(ui.rows))
	}

	send(ui.table, tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModNone))
	if ui.view != viewBookmarks || len(ui.rows) != 0 {
		t.Fatalf("cleared view %s rows %d", ui.view, len(ui.rows))
	}
	send(ui.table, tcell.NewEventKey(tcell.KeyRune, '1', tcell.ModNone))
	if ui.view != viewSessions || len(ui.rows) != 2 || ui.filter.GetText() != "agent:claude mark:no" {
		t.Fatalf("mark:no view %s rows %d filter %q", ui.view, len(ui.rows), ui.filter.GetText())
	}
}

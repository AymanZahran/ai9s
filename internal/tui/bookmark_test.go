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

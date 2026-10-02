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

func TestListPanShowsTailOnScreen(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	title := strings.Repeat("word ", 40) + "TAILMARK"
	path := filepath.Join(t.TempDir(), "wide.jsonl")
	sess := model.Session{
		ID: "grok:wide", NativeID: "wide", Agent: "grok", Title: title,
		CWD: "/work/app", Branch: "main", Updated: time.Now(),
		Messages: 2, SourcePath: path, CanDelete: true, DeleteMode: "file",
	}
	if err := st.Apply("grok", []model.Session{sess}, []store.Source{{Path: path, Mtime: 1}}); err != nil {
		t.Fatal(err)
	}
	app := tview.NewApplication()
	ui := newUI(app, st, config.Defaults())
	ui.reload()
	screen, _ := startApp(t, ui)

	snap := func() string {
		var painted string
		ui.app.QueueUpdate(func() { painted = screenText(screen) })
		return painted
	}
	waitUI(t, ui.app, func() bool { return strings.Contains(screenText(screen), "grok") })
	before := snap()
	if !strings.Contains(before, "grok") {
		t.Fatalf("list did not paint\nwide %d view %d\n%s", ui.listWide, ui.listViewW, before)
	}
	if strings.Contains(before, "TAILMARK") {
		t.Fatalf("tail visible before pan\nwide %d view %d x %d\n%s", ui.listWide, ui.listViewW, ui.listX, before)
	}
	if ui.listXBar.h < 2 {
		t.Fatalf("no horizontal bar %+v\n%s", ui.listXBar, before)
	}
	for i := 0; i < 80; i++ {
		ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	}
	waitUI(t, ui.app, func() bool {
		cell := ui.table.GetCell(1, 0)
		return cell != nil && strings.Contains(cell.Text, "TAILMARK")
	})
	after := snap()
	if !strings.Contains(after, "TAILMARK") {
		t.Fatalf("tail missing on screen wide %d view %d x %d\n%s", ui.listWide, ui.listViewW, ui.listX, after)
	}
	if strings.Contains(after, "…") || strings.Contains(after, "...") {
		t.Fatalf("ellipsis still covers the tail\n%s", after)
	}
}

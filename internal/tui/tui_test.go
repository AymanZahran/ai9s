package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AymanZahran/air9s/internal/config"
	"github.com/AymanZahran/air9s/internal/model"
	"github.com/AymanZahran/air9s/internal/store"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestTabScrollsPreview(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	snips := make([]model.Snippet, 40)
	for i := range snips {
		snips[i] = model.Snippet{Role: "user", Body: "line of the session excerpt"}
	}
	path := filepath.Join(t.TempDir(), "scroll.jsonl")
	sess := model.Session{
		ID: "claude:scroll", NativeID: "scroll", Agent: "claude", Title: "scroll me",
		Updated: time.Date(2026, 3, 2, 15, 4, 5, 0, time.UTC), Messages: 80,
		SourcePath: path, CanDelete: true, DeleteMode: "file",
		Snippets: snips,
	}
	if err := st.Apply("claude", []model.Session{sess}, []store.Source{{Path: path, Mtime: 1}}); err != nil {
		t.Fatal(err)
	}

	app := tview.NewApplication()
	ui := newUI(app, st, config.Defaults())
	ui.reload()
	app.SetRoot(ui.layout, false)

	if app.GetFocus() != ui.table {
		t.Fatalf("focus started on %T", app.GetFocus())
	}
	if !strings.Contains(ui.footer.GetText(true), "tab") {
		t.Fatalf("session footer %q", ui.footer.GetText(true))
	}

	send(ui.table, tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if app.GetFocus() != ui.preview {
		t.Fatal("tab did not focus the preview")
	}
	if ui.preview.GetTitle() != " preview · scroll " {
		t.Fatalf("title %q", ui.preview.GetTitle())
	}
	if !strings.Contains(ui.footer.GetText(true), "scroll") {
		t.Fatalf("preview footer %q", ui.footer.GetText(true))
	}
	row, _ := ui.preview.GetScrollOffset()
	if row != 0 {
		t.Fatalf("preview opened at row %d", row)
	}

	send(ui.preview, tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModNone))
	send(ui.preview, tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	row, _ = ui.preview.GetScrollOffset()
	if row < 2 {
		t.Fatalf("preview scrolled to row %d", row)
	}

	send(ui.preview, tcell.NewEventKey(tcell.KeyRune, 'g', tcell.ModNone))
	row, _ = ui.preview.GetScrollOffset()
	if row != 0 {
		t.Fatalf("g left the preview at row %d", row)
	}

	send(ui.preview, tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if app.GetFocus() != ui.table {
		t.Fatal("esc did not return to the session list")
	}
}

func TestViewsCommandAndManual(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	path := filepath.Join(t.TempDir(), "one.jsonl")
	sess := model.Session{
		ID: "claude:one", NativeID: "one", Agent: "claude", Title: "one",
		CWD: "/work/app", Branch: "main", Model: "sonnet", Messages: 2,
		Updated:    time.Date(2026, 3, 2, 15, 4, 5, 0, time.UTC),
		SourcePath: path, CanDelete: true, DeleteMode: "file",
	}
	if err := st.Apply("claude", []model.Session{sess}, []store.Source{{Path: path, Mtime: 1}}); err != nil {
		t.Fatal(err)
	}
	app := tview.NewApplication()
	ui := newUI(app, st, config.Defaults())
	ui.reload()
	app.SetRoot(ui.layout, false)
	if !strings.Contains(ui.header.GetText(true), "<1>") || !strings.Contains(ui.header.GetText(true), "manual") {
		t.Fatalf("header %q", ui.header.GetText(true))
	}

	send(ui.table, tcell.NewEventKey(tcell.KeyRune, '2', tcell.ModNone))
	if ui.table.GetTitle() != " providers " {
		t.Fatalf("title %q", ui.table.GetTitle())
	}
	send(ui.table, tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if ui.view != viewSessions || !strings.Contains(ui.filter.GetText(), "agent:claude") {
		t.Fatalf("view %s filter %q", ui.view, ui.filter.GetText())
	}

	send(ui.table, tcell.NewEventKey(tcell.KeyRune, ':', tcell.ModNone))
	if app.GetFocus() != ui.command {
		t.Fatal("colon did not open command mode")
	}
	send(ui.command, tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if ui.view != viewProviders || ui.table.GetTitle() != " providers " {
		t.Fatalf("cycled view %s title %q", ui.view, ui.table.GetTitle())
	}

	send(ui.table, tcell.NewEventKey(tcell.KeyRune, '?', tcell.ModNone))
	manual, ok := app.GetFocus().(*tview.TextView)
	if !ok || manual.GetTitle() != " manual " {
		t.Fatalf("manual focus %T", app.GetFocus())
	}
	if !strings.Contains(manual.GetText(true), "kiro-cli") || !strings.Contains(manual.GetText(true), "AIR9S_JULES_REMOTE") {
		t.Fatal("manual is missing agent help")
	}
	send(manual, tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if app.GetFocus() != ui.table {
		t.Fatal("escape did not leave the manual")
	}
}

func TestEscapeClearsFilter(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	path := filepath.Join(t.TempDir(), "one.jsonl")
	sess := model.Session{
		ID: "claude:one", NativeID: "one", Agent: "claude", Title: "ship the feature",
		CWD: "/work/app", Updated: time.Date(2026, 3, 2, 15, 4, 5, 0, time.UTC),
		SourcePath: path, CanDelete: true, DeleteMode: "file",
	}
	if err := st.Apply("claude", []model.Session{sess}, []store.Source{{Path: path, Mtime: 1}}); err != nil {
		t.Fatal(err)
	}
	app := tview.NewApplication()
	ui := newUI(app, st, config.Defaults())
	ui.reload()
	app.SetRoot(ui.layout, false)

	ui.filter.SetText("agent:claude")
	send(ui.table, tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if ui.filter.GetText() != "" {
		t.Fatalf("table esc left %q", ui.filter.GetText())
	}
	if len(ui.rows) != 1 {
		t.Fatalf("rows %d", len(ui.rows))
	}

	ui.filter.SetText("missing-title")
	app.SetFocus(ui.filter)
	send(ui.filter, tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if ui.filter.GetText() != "" || app.GetFocus() != ui.table {
		t.Fatalf("filter esc text %q focus %T", ui.filter.GetText(), app.GetFocus())
	}

	ui.filter.SetText("agent:claude")
	send(ui.table, tcell.NewEventKey(tcell.KeyRune, ':', tcell.ModNone))
	send(ui.command, tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if ui.filter.GetText() != "agent:claude" || app.GetFocus() != ui.table {
		t.Fatalf("command esc filter %q focus %T", ui.filter.GetText(), app.GetFocus())
	}

	send(ui.table, tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	send(ui.preview, tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if ui.filter.GetText() != "agent:claude" || app.GetFocus() != ui.table {
		t.Fatalf("preview esc filter %q focus %T", ui.filter.GetText(), app.GetFocus())
	}
}

func send(p tview.Primitive, ev *tcell.EventKey) {
	if h := p.InputHandler(); h != nil {
		h(ev, func(tview.Primitive) {})
	}
}

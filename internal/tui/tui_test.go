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

func TestArrowsSelectWhilePromptIsOpen(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	dir := t.TempDir()
	sessions := []model.Session{
		{
			ID: "claude:newest", NativeID: "newest", Agent: "claude", Title: "newest",
			Updated: time.Date(2026, 3, 3, 15, 4, 5, 0, time.UTC), Messages: 3,
			SourcePath: filepath.Join(dir, "newest.jsonl"), CanDelete: true, DeleteMode: "file",
		},
		{
			ID: "claude:middle", NativeID: "middle", Agent: "claude", Title: "middle",
			Updated: time.Date(2026, 3, 2, 15, 4, 5, 0, time.UTC), Messages: 2,
			SourcePath: filepath.Join(dir, "middle.jsonl"), CanDelete: true, DeleteMode: "file",
		},
		{
			ID: "claude:oldest", NativeID: "oldest", Agent: "claude", Title: "oldest",
			Updated: time.Date(2026, 3, 1, 15, 4, 5, 0, time.UTC), Messages: 1,
			SourcePath: filepath.Join(dir, "oldest.jsonl"), CanDelete: true, DeleteMode: "file",
		},
	}
	sources := make([]store.Source, len(sessions))
	for i, s := range sessions {
		sources[i] = store.Source{Path: s.SourcePath, Mtime: 1}
	}
	if err := st.Apply("claude", sessions, sources); err != nil {
		t.Fatal(err)
	}

	app := tview.NewApplication()
	ui := newUI(app, st, config.Defaults())
	ui.reload()
	app.SetRoot(ui.layout, false)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(100, 30)
	ui.table.SetRect(0, 0, 80, 24)
	ui.table.Draw(screen)

	row, _ := ui.table.GetSelection()
	if row != 1 || !strings.Contains(ui.preview.GetText(true), "newest") {
		t.Fatalf("start row %d preview %q", row, ui.preview.GetText(true))
	}

	send(ui.table, tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	if app.GetFocus() != ui.filter {
		t.Fatal("slash did not focus the filter")
	}
	send(ui.filter, tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	row, _ = ui.table.GetSelection()
	if row != 2 || !strings.Contains(ui.preview.GetText(true), "middle") {
		t.Fatalf("down row %d preview %q", row, ui.preview.GetText(true))
	}
	if app.GetFocus() != ui.filter || ui.filter.GetText() != "" {
		t.Fatalf("down left filter %q focus %T", ui.filter.GetText(), app.GetFocus())
	}
	send(ui.filter, tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	send(ui.filter, tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	row, _ = ui.table.GetSelection()
	if row != 2 {
		t.Fatalf("up returned to row %d", row)
	}
	send(ui.filter, tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	row, _ = ui.table.GetSelection()
	if row != 2 || ui.filter.GetText() != "" || app.GetFocus() != ui.filter {
		t.Fatalf("left row %d text %q focus %T", row, ui.filter.GetText(), app.GetFocus())
	}
	send(ui.filter, tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone))
	row, _ = ui.table.GetSelection()
	if row != 3 || !strings.Contains(ui.preview.GetText(true), "oldest") || ui.filter.GetText() != "" || app.GetFocus() != ui.filter {
		t.Fatalf("page row %d text %q focus %T preview %q", row, ui.filter.GetText(), app.GetFocus(), ui.preview.GetText(true))
	}
	send(ui.filter, tcell.NewEventKey(tcell.KeyRune, 'k', tcell.ModNone))
	if ui.filter.GetText() != "k" || app.GetFocus() != ui.filter {
		t.Fatalf("k typed %q focus %T", ui.filter.GetText(), app.GetFocus())
	}
	send(ui.filter, tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if ui.filter.GetText() != "" || app.GetFocus() != ui.table {
		t.Fatalf("filter esc text %q focus %T", ui.filter.GetText(), app.GetFocus())
	}

	send(ui.table, tcell.NewEventKey(tcell.KeyRune, ':', tcell.ModNone))
	send(ui.command, tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if ui.view != viewProviders {
		t.Fatalf("empty command cycled to %s", ui.view)
	}

	send(ui.table, tcell.NewEventKey(tcell.KeyRune, ':', tcell.ModNone))
	if app.GetFocus() != ui.command {
		t.Fatal("colon did not focus the command field")
	}
	send(ui.command, tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	send(ui.command, tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	row, _ = ui.table.GetSelection()
	if row != 3 || ui.command.GetText() != "" || app.GetFocus() != ui.command {
		t.Fatalf("command down row %d text %q focus %T", row, ui.command.GetText(), app.GetFocus())
	}
	if !strings.Contains(ui.preview.GetText(true), "directories") {
		t.Fatalf("command preview %q", ui.preview.GetText(true))
	}
	send(ui.command, tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if ui.view != viewDirectories || app.GetFocus() != ui.table {
		t.Fatalf("arrowed command view %s focus %T", ui.view, app.GetFocus())
	}
}

func send(p tview.Primitive, ev *tcell.EventKey) {
	if h := p.InputHandler(); h != nil {
		h(ev, func(tview.Primitive) {})
	}
}

package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AymanZahran/air9s/internal/act"
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

func TestEnterPlansResumeWithoutAScreen(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	path := filepath.Join(t.TempDir(), "one.jsonl")
	sess := model.Session{
		ID: "claude:one", NativeID: "one", Agent: "claude", Title: "one",
		CWD: t.TempDir(), Updated: time.Date(2026, 3, 2, 15, 4, 5, 0, time.UTC),
		SourcePath: path, CanDelete: true, DeleteMode: "file",
	}
	if err := st.Apply("claude", []model.Session{sess}, []store.Source{{Path: path, Mtime: 1}}); err != nil {
		t.Fatal(err)
	}
	orig := act.LookPath
	act.LookPath = func(string) (string, error) { return "/bin/echo", nil }
	t.Cleanup(func() { act.LookPath = orig })

	app := tview.NewApplication()
	ui := newUI(app, st, config.Defaults())
	ui.reload()
	send(ui.table, tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if ui.pending == nil || len(ui.pending.Args) < 2 || ui.pending.Args[0] != "--resume" || ui.pending.Args[1] != "one" {
		t.Fatalf("pending %#v", ui.pending)
	}
}

func TestPageAndScrollbar(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	const n = 40
	sessions := make([]model.Session, n)
	sources := make([]store.Source, n)
	dir := t.TempDir()
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("s%02d", i)
		path := filepath.Join(dir, id+".jsonl")
		sessions[i] = model.Session{
			ID: "claude:" + id, NativeID: id, Agent: "claude", Title: "session",
			Updated: time.Date(2026, 3, 1, 0, 0, n-i, 0, time.UTC), Messages: 1,
			SourcePath: path, CanDelete: true, DeleteMode: "file",
		}
		sources[i] = store.Source{Path: path, Mtime: 1}
	}
	if err := st.Apply("claude", sessions, sources); err != nil {
		t.Fatal(err)
	}

	app := tview.NewApplication()
	ui := newUI(app, st, config.Defaults())
	ui.reload()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(100, 40)
	ui.table.SetRect(0, 0, 60, 20)
	ui.table.Draw(screen)

	_, _, width, _ := ui.table.GetInnerRect()
	if width != 57 {
		t.Fatalf("list inner width %d", width)
	}
	if ui.listBar.h < 2 {
		t.Fatalf("list bar %+v", ui.listBar)
	}
	r, _, _, _ := screen.GetContent(ui.listBar.x, ui.listBar.y)
	if r != '┃' && r != '│' {
		t.Fatalf("list bar rune %q", r)
	}

	start, _ := ui.table.GetSelection()
	if start != 1 {
		t.Fatalf("start row %d", start)
	}
	send(ui.table, tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone))
	row, _ := ui.table.GetSelection()
	if row != start+ui.listPage() {
		t.Fatalf("page down row %d page %d", row, ui.listPage())
	}
	if ui.listPage() < 2 {
		t.Fatalf("page size %d", ui.listPage())
	}
	send(ui.table, tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone))
	row, _ = ui.table.GetSelection()
	if row != start {
		t.Fatalf("page up row %d", row)
	}
	send(ui.table, tcell.NewEventKey(tcell.KeyCtrlF, 0, tcell.ModNone))
	row, _ = ui.table.GetSelection()
	if row != start+ui.listPage() {
		t.Fatalf("ctrl-f row %d", row)
	}
	send(ui.table, tcell.NewEventKey(tcell.KeyCtrlB, 0, tcell.ModNone))

	ui.onMouse(tcell.NewEventMouse(4, 4, tcell.WheelDown, tcell.ModNone), tview.MouseScrollDown)
	row, _ = ui.table.GetSelection()
	if row != start+wheelRows {
		t.Fatalf("wheel row %d", row)
	}

	ui.table.Draw(screen)
	y := ui.listBar.y + ui.listBar.h - 1
	ui.onMouse(tcell.NewEventMouse(ui.listBar.x, y, tcell.Button1, tcell.ModNone), tview.MouseLeftDown)
	row, _ = ui.table.GetSelection()
	if row < n/2 {
		t.Fatalf("scrollbar click row %d", row)
	}

	var lines []string
	for i := 0; i < 40; i++ {
		lines = append(lines, "preview line stays on one row")
	}
	ui.preview.SetText(strings.Join(lines, "\n"))
	ui.preview.SetRect(70, 0, 30, 16)
	ui.preview.Draw(screen)
	if ui.previewBar.h < 2 {
		t.Fatalf("preview bar %+v", ui.previewBar)
	}
	_, _, pw, ph := ui.preview.GetInnerRect()
	if pw != 27 || ph != 14 {
		t.Fatalf("preview inner %d x %d", pw, ph)
	}
	send(ui.preview, tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone))
	prow, _ := ui.preview.GetScrollOffset()
	if prow != ph {
		t.Fatalf("preview page row %d height %d", prow, ph)
	}
	send(ui.preview, tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	prow, _ = ui.preview.GetScrollOffset()
	if prow != ph+1 {
		t.Fatalf("preview arrow row %d", prow)
	}
	ui.onMouse(tcell.NewEventMouse(72, 2, tcell.WheelDown, tcell.ModNone), tview.MouseScrollDown)
	prow, _ = ui.preview.GetScrollOffset()
	if prow != ph+1+wheelRows {
		t.Fatalf("preview wheel row %d", prow)
	}
	send(ui.preview, tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone))
	prow, _ = ui.preview.GetScrollOffset()
	if prow != ph+1+wheelRows-ph {
		t.Fatalf("preview page up row %d", prow)
	}
}

func send(p tview.Primitive, ev *tcell.EventKey) {
	if h := p.InputHandler(); h != nil {
		h(ev, func(tview.Primitive) {})
	}
}

package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AymanZahran/ai9s/internal/act"
	"github.com/AymanZahran/ai9s/internal/config"
	"github.com/AymanZahran/ai9s/internal/model"
	"github.com/AymanZahran/ai9s/internal/store"
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
	if header := ui.header.GetText(true); !strings.Contains(header, "describe") || !strings.Contains(header, "g/G") {
		t.Fatalf("session menu %q", header)
	}
	if name, _ := ui.body.GetFrontPage(); name != "list" {
		t.Fatalf("front page %s", name)
	}

	send(ui.table, tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModNone))
	if app.GetFocus() != ui.preview {
		t.Fatal("d did not open describe")
	}
	if name, _ := ui.body.GetFrontPage(); name != "describe" {
		t.Fatalf("describe page %s", name)
	}
	if ui.preview.GetTitle() != " describe · scroll " {
		t.Fatalf("title %q", ui.preview.GetTitle())
	}
	if !strings.Contains(ui.header.GetText(true), "scroll") {
		t.Fatalf("preview menu %q", ui.header.GetText(true))
	}

	send(ui.preview, tcell.NewEventKey(tcell.KeyCtrlD, 0, tcell.ModNone))
	if !confirmVisible(app) {
		t.Fatalf("ctrl-d from describe focus %T", app.GetFocus())
	}
	if !ui.describing() {
		t.Fatal("ctrl-d closed describe before confirmation")
	}
	send(app.GetFocus(), tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if app.GetFocus() != ui.preview {
		t.Fatalf("cancel delete focus %T", app.GetFocus())
	}
	if !ui.describing() {
		t.Fatal("cancel left describe")
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
	if name, _ := ui.body.GetFrontPage(); name != "list" {
		t.Fatalf("after esc page %s", name)
	}

	send(ui.table, tcell.NewEventKey(tcell.KeyCtrlD, 0, tcell.ModNone))
	if !confirmVisible(app) {
		t.Fatalf("ctrl-d focus %T", app.GetFocus())
	}
	if ui.describing() {
		t.Fatal("ctrl-d from the list opened describe")
	}
}

func confirmVisible(app *tview.Application) bool {
	switch app.GetFocus().(type) {
	case *tview.Modal, *tview.Button:
		return true
	default:
		return false
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
	headerText := ui.header.GetText(true)
	for _, want := range []string{"<1>", "manual", "j/k ↑/↓", "h/l ←/→", "⌘↑/⌘↓", "⌘←/⌘→"} {
		if !strings.Contains(headerText, want) {
			t.Fatalf("header missing %q in %q", want, headerText)
		}
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
	if !strings.Contains(manual.GetText(true), "kiro-cli") || !strings.Contains(manual.GetText(true), "AI9S_JULES_REMOTE") {
		t.Fatal("manual is missing agent help")
	}
	if manual.GetBackgroundColor() != tcell.GetColor("#000000") {
		t.Fatalf("manual background %v", manual.GetBackgroundColor())
	}
	send(manual, tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModNone))
	row, _ := manual.GetScrollOffset()
	if row < 1 {
		t.Fatalf("manual j row %d", row)
	}
	send(manual, tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone))
	paged, _ := manual.GetScrollOffset()
	if paged <= row {
		t.Fatalf("manual page %d from %d", paged, row)
	}
	ui.onMouse(tcell.NewEventMouse(2, 2, tcell.WheelDown, tcell.ModNone), tview.MouseScrollDown)
	wheeled, _ := manual.GetScrollOffset()
	if wheeled < paged+wheelRows {
		t.Fatalf("manual wheel %d from %d", wheeled, paged)
	}
	send(manual, tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModMeta))
	cmd, _ := manual.GetScrollOffset()
	if cmd <= wheeled {
		t.Fatalf("manual cmd-down %d from %d", cmd, wheeled)
	}
	send(manual, tcell.NewEventKey(tcell.KeyRune, 'g', tcell.ModNone))
	top, _ := manual.GetScrollOffset()
	if top != 0 {
		t.Fatalf("manual g row %d", top)
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

func TestEscapeReturnsToDrilledView(t *testing.T) {
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

	send(ui.table, tcell.NewEventKey(tcell.KeyRune, '2', tcell.ModNone))
	if ui.view != viewProviders {
		t.Fatalf("providers %s", ui.view)
	}
	send(ui.table, tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if ui.view != viewSessions || ui.drilled != viewProviders || !strings.Contains(ui.filter.GetText(), "agent:claude") {
		t.Fatalf("drill view %s from %s filter %q", ui.view, ui.drilled, ui.filter.GetText())
	}
	send(ui.table, tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModNone))
	send(ui.preview, tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if ui.view != viewSessions || ui.drilled != viewProviders || !strings.Contains(ui.filter.GetText(), "agent:claude") {
		t.Fatalf("describe esc view %s from %s filter %q", ui.view, ui.drilled, ui.filter.GetText())
	}
	send(ui.table, tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if ui.view != viewProviders || ui.drilled != "" || ui.filter.GetText() != "" {
		t.Fatalf("back view %s from %q filter %q", ui.view, ui.drilled, ui.filter.GetText())
	}
	ui.filter.SetText("zzz")
	send(ui.table, tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if ui.view != viewProviders || ui.filter.GetText() != "" {
		t.Fatalf("group root view %s filter %q", ui.view, ui.filter.GetText())
	}
	send(ui.table, tcell.NewEventKey(tcell.KeyRune, '1', tcell.ModNone))
	ui.filter.SetText("agent:claude")
	send(ui.table, tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if ui.view != viewSessions || ui.filter.GetText() != "" {
		t.Fatalf("sessions esc view %s filter %q", ui.view, ui.filter.GetText())
	}
	header := ui.header.GetText(true)
	info := ui.info.GetText(true)
	if strings.Contains(info, "claude") || strings.Contains(header, "yolo") || strings.Contains(header, "pgup") {
		t.Fatalf("chrome info %q header %q", info, header)
	}
	if !strings.Contains(ui.lines[0], "AGE") || !strings.Contains(ui.lines[0], "DATE") || !strings.Contains(ui.lines[0], "TOKENS") {
		t.Fatalf("header %q", ui.lines[0])
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
	text, _, _ := screen.Get(ui.listBar.x, ui.listBar.y)
	r := primaryRune(text)
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

	ui.table.Select(start, 0)
	send(ui.table, tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModMeta))
	row, _ = ui.table.GetSelection()
	if row != start+ui.listPage() {
		t.Fatalf("cmd-down row %d page %d", row, ui.listPage())
	}
	send(ui.table, tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModCtrl))
	row, _ = ui.table.GetSelection()
	if row != start {
		t.Fatalf("ctrl-up row %d", row)
	}
	send(ui.table, tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModAlt))
	row, _ = ui.table.GetSelection()
	if row != start+ui.listPage() {
		t.Fatalf("alt-down row %d", row)
	}
	send(ui.table, tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	row, _ = ui.table.GetSelection()
	if row != start+ui.listPage()-1 {
		t.Fatalf("plain up row %d", row)
	}

	var lines []string
	for i := 0; i < 40; i++ {
		lines = append(lines, "preview line stays on one row")
	}
	ui.preview.SetText(strings.Join(lines, "\n"))
	ui.body.SwitchToPage("describe")
	ui.preview.SetRect(70, 0, 30, 16)
	ui.preview.Draw(screen)
	if ui.previewBar.h < 2 {
		t.Fatalf("preview bar %+v", ui.previewBar)
	}
	_, _, pw, ph := ui.preview.GetInnerRect()
	// The fixture line is wider than this pane, so the bottom bar takes one row.
	if pw != 27 || ph != 13 || ui.previewXBar.h < 2 {
		t.Fatalf("preview inner %d x %d bar %+v", pw, ph, ui.previewXBar)
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
	held := prow
	send(ui.preview, tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModMeta))
	prow, _ = ui.preview.GetScrollOffset()
	if prow != held+ph {
		t.Fatalf("preview cmd-down row %d held %d page %d", prow, held, ph)
	}
}

func TestHorizontalScroll(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	title := strings.Repeat("title-", 30) + "TAIL"
	path := filepath.Join(t.TempDir(), "wide.jsonl")
	sess := model.Session{
		ID: "claude:wide", NativeID: "wide", Agent: "claude", Title: title,
		CWD: "/work/app", Branch: "main", Updated: time.Date(2026, 3, 2, 15, 4, 5, 0, time.UTC),
		Messages: 2, SourcePath: path, CanDelete: true, DeleteMode: "file",
	}
	if err := st.Apply("claude", []model.Session{sess}, []store.Source{{Path: path, Mtime: 1}}); err != nil {
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
	screen.SetSize(80, 30)
	ui.table.SetRect(0, 0, 36, 12)
	ui.table.Draw(screen)

	cell := ui.table.GetCell(1, 0)
	if cell == nil || strings.Contains(cell.Text, "TAIL") {
		t.Fatalf("unscrolled cell %#v", cell)
	}
	if ui.listWide <= ui.listViewW || ui.listX != 0 || ui.listXBar.h < 2 {
		t.Fatalf("wide %d view %d x %d bar %+v", ui.listWide, ui.listViewW, ui.listX, ui.listXBar)
	}
	text, _, _ := screen.Get(ui.listXBar.x, ui.listXBar.y)
	r := primaryRune(text)
	if r != '─' && r != '━' {
		t.Fatalf("bottom bar rune %q", string(r))
	}
	_, _, width, _ := ui.table.GetInnerRect()
	if width != 33 {
		t.Fatalf("list inner width %d", width)
	}

	prev := -1
	for i := 0; i < 400 && ui.listX != prev; i++ {
		prev = ui.listX
		send(ui.table, tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	}
	ui.table.Draw(screen)
	cell = ui.table.GetCell(1, 0)
	if cell == nil || !strings.Contains(cell.Text, "TAIL") {
		t.Fatalf("scrolled cell %#v x %d wide %d", cell, ui.listX, ui.listWide)
	}
	if ui.listWide < 200 {
		t.Fatalf("row was cut to %d", ui.listWide)
	}

	send(ui.table, tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if ui.listX != prev-hScrollStep && ui.listX >= prev {
		t.Fatalf("left x %d prev %d", ui.listX, prev)
	}
	ui.setListX(0)
	send(ui.table, tcell.NewEventKey(tcell.KeyRune, 'l', tcell.ModNone))
	if ui.listX != hScrollStep {
		t.Fatalf("l x %d", ui.listX)
	}
	send(ui.table, tcell.NewEventKey(tcell.KeyRune, 'h', tcell.ModNone))
	if ui.listX != 0 {
		t.Fatalf("h x %d", ui.listX)
	}

	ui.onMouse(tcell.NewEventMouse(4, 4, tcell.WheelRight, tcell.ModNone), tview.MouseScrollRight)
	if ui.listX != hScrollStep {
		t.Fatalf("wheel x %d", ui.listX)
	}
	ui.table.Draw(screen)
	ui.onMouse(tcell.NewEventMouse(ui.listXBar.x+ui.listXBar.h-1, ui.listXBar.y, tcell.Button1, tcell.ModNone), tview.MouseLeftDown)
	if ui.listX < ui.listWide/2 {
		t.Fatalf("bar click x %d wide %d", ui.listX, ui.listWide)
	}

	ui.app.SetFocus(ui.filter)
	held := ui.listX
	send(ui.filter, tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if ui.listX != held || app.GetFocus() != ui.filter {
		t.Fatalf("filter left x %d focus %T", ui.listX, app.GetFocus())
	}

	ui.preview.SetText(strings.Repeat("m", 180))
	ui.body.SwitchToPage("describe")
	ui.preview.SetRect(0, 14, 40, 12)
	ui.preview.Draw(screen)
	if ui.previewXBar.h < 2 {
		t.Fatalf("preview x bar %+v", ui.previewXBar)
	}
	send(ui.preview, tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	_, col := ui.preview.GetScrollOffset()
	if col != hScrollStep {
		t.Fatalf("preview col %d", col)
	}
	send(ui.preview, tcell.NewEventKey(tcell.KeyRune, 'h', tcell.ModNone))
	_, col = ui.preview.GetScrollOffset()
	if col != 0 {
		t.Fatalf("preview h col %d", col)
	}
	ui.onMouse(tcell.NewEventMouse(2, 16, tcell.WheelRight, tcell.ModNone), tview.MouseScrollRight)
	_, col = ui.preview.GetScrollOffset()
	if col != hScrollStep {
		t.Fatalf("preview wheel col %d", col)
	}
	page := ui.pageStepX()
	if page <= hScrollStep {
		t.Fatalf("preview page %d", page)
	}
	if got := ui.previewKeys(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModMeta)); got != nil {
		t.Fatal("preview page key was not consumed")
	}
	_, col = ui.preview.GetScrollOffset()
	if col != hScrollStep+page {
		t.Fatalf("preview cmd-right col %d page %d", col, page)
	}
	send(ui.preview, tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModCtrl))
	_, col = ui.preview.GetScrollOffset()
	if col != hScrollStep {
		t.Fatalf("preview ctrl-left col %d", col)
	}

	ui.body.SwitchToPage("list")
	ui.setListX(0)
	page = ui.pageStepX()
	if page != ui.listViewW || page <= hScrollStep {
		t.Fatalf("list page %d view %d", page, ui.listViewW)
	}
	send(ui.table, tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModMeta))
	if ui.listX != page {
		t.Fatalf("cmd-right x %d page %d", ui.listX, page)
	}
	send(ui.table, tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModAlt))
	if ui.listX != 0 {
		t.Fatalf("alt-left x %d", ui.listX)
	}
	ui.app.SetFocus(ui.filter)
	send(ui.filter, tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if ui.listX != 0 || app.GetFocus() != ui.filter {
		t.Fatalf("filter left x %d focus %T", ui.listX, app.GetFocus())
	}
	send(ui.filter, tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModMeta))
	if ui.listX != page {
		t.Fatalf("filter cmd-right x %d page %d", ui.listX, page)
	}
	if !strings.Contains(ui.lines[0], "AGE") || !strings.Contains(ui.lines[0], "DATE") {
		t.Fatalf("columns %q", ui.lines[0])
	}
	if !strings.Contains(ui.lines[1], absDate(sess.Updated)) {
		t.Fatalf("row %q date %q", ui.lines[1], absDate(sess.Updated))
	}
	sel, _ := ui.table.GetSelection()
	ui.setListX(0)
	ui.onMouse(tcell.NewEventMouse(4, 4, tcell.WheelDown, tcell.ModShift), tview.MouseScrollDown)
	got, _ := ui.table.GetSelection()
	if ui.listX != hScrollStep || got != sel {
		t.Fatalf("shift-wheel x %d row %d want %d", ui.listX, got, sel)
	}
}

func TestBranchesShowWorktrees(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	wt := filepath.Join(root, "wt")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=ai9s",
			"GIT_AUTHOR_EMAIL=ai9s@example.com",
			"GIT_COMMITTER_NAME=ai9s",
			"GIT_COMMITTER_EMAIL=ai9s@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git(repo, "init")
	git(repo, "commit", "--allow-empty", "-m", "init")
	git(repo, "worktree", "add", "-b", "feature", wt)
	loose := filepath.Join(root, "loose")
	if err := os.MkdirAll(loose, 0o755); err != nil {
		t.Fatal(err)
	}

	st, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	when := time.Date(2026, 3, 2, 15, 4, 5, 0, time.UTC)
	sessions := []model.Session{
		{
			ID: "claude:mainrepo", NativeID: "mainrepo", Agent: "claude", Title: "in the main checkout",
			CWD: repo, Branch: "main", Updated: when, Messages: 2,
			SourcePath: filepath.Join(root, "main.jsonl"), CanDelete: true, DeleteMode: "file",
		},
		{
			ID: "claude:linked", NativeID: "linked", Agent: "claude", Title: "in the linked worktree",
			CWD: wt, Branch: "main", Updated: when.Add(time.Hour), Messages: 1,
			SourcePath: filepath.Join(root, "linked.jsonl"), CanDelete: true, DeleteMode: "file",
		},
		{
			ID: "claude:loose", NativeID: "loose", Agent: "claude", Title: "not a checkout",
			CWD: loose, Branch: "other", Updated: when, Messages: 1,
			SourcePath: filepath.Join(root, "loose.jsonl"), CanDelete: true, DeleteMode: "file",
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
	send(ui.table, tcell.NewEventKey(tcell.KeyRune, '4', tcell.ModNone))
	if ui.view != viewBranches || ui.table.GetRowCount() != 4 {
		t.Fatalf("view %s rows %d", ui.view, ui.table.GetRowCount())
	}
	var lines []string
	for row := 0; row < ui.table.GetRowCount(); row++ {
		cell := ui.table.GetCell(row, 0)
		if cell == nil {
			t.Fatalf("missing row %d", row)
		}
		lines = append(lines, cell.Text)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(lines[0], "BRANCH") || !strings.Contains(lines[0], "WORKTREE") || !strings.Contains(lines[0], "AGE") || !strings.Contains(lines[0], "DATE") {
		t.Fatalf("header %q", lines[0])
	}
	if !strings.Contains(joined, "/repo") || !strings.Contains(joined, "/wt") || !strings.Contains(joined, "-") {
		t.Fatalf("rows\n%s", joined)
	}
	if !strings.Contains(ui.preview.GetText(true), "worktree") {
		t.Fatalf("preview %q", ui.preview.GetText(true))
	}

	first := repo
	if wt < repo {
		first = wt
	}
	send(ui.table, tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if ui.view != viewSessions || !strings.Contains(ui.filter.GetText(), "branch:main") || !strings.Contains(ui.filter.GetText(), first) {
		t.Fatalf("filter %q view %s", ui.filter.GetText(), ui.view)
	}
}

func send(p tview.Primitive, ev *tcell.EventKey) {
	if h := p.InputHandler(); h != nil {
		h(ev, func(tview.Primitive) {})
	}
}

func startApp(t *testing.T, ui *ui) (tcell.SimulationScreen, <-chan struct{}) {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	ui.app.SetScreen(screen)
	screen.SetSize(120, 40)
	ui.app.SetRoot(ui.layout, true)
	finished := make(chan struct{})
	go func() {
		_ = ui.app.Run()
		close(finished)
	}()
	ready := make(chan struct{})
	go func() {
		ui.app.QueueUpdate(func() { close(ready) })
	}()
	select {
	case <-ready:
	case <-finished:
		t.Fatal("application stopped before it was ready")
	case <-time.After(5 * time.Second):
		t.Fatal("event loop did not start")
	}
	t.Cleanup(func() {
		ui.app.Stop()
		select {
		case <-finished:
		case <-time.After(3 * time.Second):
			t.Errorf("application did not stop")
		}
	})
	return screen, finished
}

func waitUI(t *testing.T, app *tview.Application, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var ok bool
		app.QueueUpdate(func() { ok = cond() })
		if ok {
			return
		}
		time.Sleep(15 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the interface")
}

func primaryRune(text string) rune {
	for _, r := range text {
		if r != 0 {
			return r
		}
	}
	return ' '
}

func screenText(screen tcell.SimulationScreen) string {
	w, h := screen.Size()
	var b strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			text, _, _ := screen.Get(x, y)
			r := primaryRune(text)
			b.WriteRune(r)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func TestDeleteStaysOnScreen(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", root)
	path := filepath.Join(root, "projects", "demo", "abc.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	sess := model.Session{
		ID: "claude:abc", NativeID: "abc", Agent: "claude", Title: "keep me posted",
		Updated: time.Date(2026, 3, 2, 15, 4, 5, 0, time.UTC), Messages: 1,
		SourcePath: path, CanDelete: true, DeleteMode: "file",
	}
	if err := st.Apply("claude", []model.Session{sess}, []store.Source{{Path: path, Mtime: 1}}); err != nil {
		t.Fatal(err)
	}

	app := tview.NewApplication()
	ui := newUI(app, st, config.Defaults())
	ui.reload()
	screen, _ := startApp(t, ui)

	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModNone))
	waitUI(t, ui.app, func() bool { return ui.describing() })
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyCtrlD, 0, tcell.ModNone))
	waitUI(t, ui.app, func() bool { return confirmVisible(ui.app) && ui.describing() })
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	waitUI(t, ui.app, func() bool {
		return !ui.busy && !ui.describing() && len(ui.rows) == 0 && ui.app.GetFocus() == ui.table
	})
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("transcript still exists")
	}
	var painted string
	ui.app.QueueUpdate(func() { painted = screenText(screen) })
	if strings.Contains(painted, "deleting") {
		t.Fatal("delete status stayed on screen")
	}
}

func TestDeleteErrorAndPanicStayOnScreen(t *testing.T) {
	orig := deleteSession
	t.Cleanup(func() { deleteSession = orig })
	for _, tc := range []struct {
		name string
		fn   func(model.Session) error
		want string
	}{
		{name: "error", fn: func(model.Session) error { return fmt.Errorf("disk full") }, want: "disk full"},
		{name: "panic", fn: func(model.Session) error { panic("delete exploded") }, want: "exploded"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			path := filepath.Join(t.TempDir(), "one.jsonl")
			sess := model.Session{
				ID: "claude:one", NativeID: "one", Agent: "claude", Title: "one",
				Updated: time.Date(2026, 3, 2, 15, 4, 5, 0, time.UTC), Messages: 1,
				SourcePath: path, CanDelete: true, DeleteMode: "file",
			}
			if err := st.Apply("claude", []model.Session{sess}, []store.Source{{Path: path, Mtime: 1}}); err != nil {
				t.Fatal(err)
			}
			deleteSession = tc.fn
			app := tview.NewApplication()
			ui := newUI(app, st, config.Defaults())
			ui.reload()
			screen, _ := startApp(t, ui)
			ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyCtrlD, 0, tcell.ModNone))
			waitUI(t, ui.app, func() bool { return confirmVisible(ui.app) })
			ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
			waitUI(t, ui.app, func() bool {
				return !ui.busy && len(ui.rows) == 1 && strings.Contains(screenText(screen), tc.want)
			})
		})
	}
}

func TestDeleteCtrlCLeavesTheSession(t *testing.T) {
	orig := deleteSession
	release := make(chan struct{})
	started := make(chan struct{})
	deleteSession = func(model.Session) error {
		close(started)
		<-release
		return nil
	}
	t.Cleanup(func() {
		deleteSession = orig
		select {
		case <-release:
		default:
			close(release)
		}
	})

	st, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	path := filepath.Join(t.TempDir(), "one.jsonl")
	sess := model.Session{
		ID: "claude:one", NativeID: "one", Agent: "claude", Title: "one",
		Updated: time.Date(2026, 3, 2, 15, 4, 5, 0, time.UTC), Messages: 1,
		SourcePath: path, CanDelete: true, DeleteMode: "file",
	}
	if err := st.Apply("claude", []model.Session{sess}, []store.Source{{Path: path, Mtime: 1}}); err != nil {
		t.Fatal(err)
	}
	app := tview.NewApplication()
	ui := newUI(app, st, config.Defaults())
	ui.reload()
	_, finished := startApp(t, ui)
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyCtrlD, 0, tcell.ModNone))
	waitUI(t, ui.app, func() bool { return confirmVisible(ui.app) })
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("delete did not start")
	}
	var note string
	ui.app.QueueUpdate(func() { note = ui.crumbs.GetText(true) })
	if !strings.Contains(note, "deleting") {
		t.Fatalf("crumbs %q", note)
	}
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModNone))
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("ctrl-c did not leave ai9s while delete was running")
	}
}

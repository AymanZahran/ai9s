package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	ui := newUI(app, st)
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

func send(p tview.Primitive, ev *tcell.EventKey) {
	if h := p.InputHandler(); h != nil {
		h(ev, func(tview.Primitive) {})
	}
}

package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/AymanZahran/ai9s/internal/config"
	"github.com/AymanZahran/ai9s/internal/fork"
	"github.com/AymanZahran/ai9s/internal/model"
	"github.com/AymanZahran/ai9s/internal/store"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestForkAsksForANameAndListsBothSessions(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	path := filepath.Join(t.TempDir(), "parent.jsonl")
	childPath := filepath.Join(t.TempDir(), "child.jsonl")
	sess := model.Session{
		ID: "claude:abc", NativeID: "abc", Agent: "claude", Title: "recorded title",
		Updated: time.Date(2026, 3, 2, 15, 4, 5, 0, time.UTC), Messages: 2,
		SourcePath: path, CanDelete: true, DeleteMode: "file",
	}
	if err := st.Apply("claude", []model.Session{sess}, []store.Source{{Path: path, Mtime: 1}}); err != nil {
		t.Fatal(err)
	}
	origCopy, origRebuild := copySession, rebuildIndex
	t.Cleanup(func() {
		copySession = origCopy
		rebuildIndex = origRebuild
	})
	copySession = func(s model.Session, opt fork.Options) (string, error) {
		if s.ID != "claude:abc" || opt.Worktree || opt.Name != "recorded title fork" {
			t.Errorf("copied %s worktree %v name %q", s.ID, opt.Worktree, opt.Name)
		}
		return "child", nil
	}
	rebuildIndex = func(got *store.Store) (store.Stats, []string, error) {
		child := sess
		child.ID = "claude:child"
		child.NativeID = "child"
		child.SourcePath = childPath
		err := got.Apply("claude", []model.Session{sess, child}, []store.Source{{Path: path, Mtime: 1}, {Path: childPath, Mtime: 1}})
		return store.Stats{}, nil, err
	}

	app := tview.NewApplication()
	ui := newUI(app, st, config.Defaults())
	ui.reload()
	screen, _ := startApp(t, ui)
	menu, _ := ui.menuText()
	if !strings.Contains(menu, "<F>") || !strings.Contains(menu, "fork") {
		t.Fatalf("menu %s", menu)
	}
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyRune, 'F', tcell.ModNone))
	var prefill string
	waitUI(t, ui.app, func() bool {
		field, ok := ui.app.GetFocus().(*tview.InputField)
		if ok && field.GetLabel() == "Name" {
			prefill = field.GetText()
			return true
		}
		return false
	})
	if prefill != "recorded title fork" {
		t.Fatalf("prefill %q", prefill)
	}
	if !strings.Contains(strings.Join(strings.Fields(screenText(screen)), " "), "Worktree") {
		t.Fatal("missing worktree option")
	}
	saveForkDialog(ui)
	waitUI(t, ui.app, func() bool {
		// The modal is narrower than the sentence, so the border sits between the two lines.
		flat := strings.Join(strings.Fields(screenText(screen)), " ")
		return len(ui.rows) == 2 && strings.Contains(flat, "Forked.") && strings.Contains(flat, "recorded title fork.")
	})
	var names []string
	for _, row := range ui.rows {
		names = append(names, row.Name)
	}
	if !contains(names, "recorded title fork") || !contains(names, "") {
		t.Fatalf("names %v", names)
	}
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	waitUI(t, ui.app, func() bool {
		row, _ := ui.table.GetSelection()
		return ui.app.GetFocus() == ui.table && row > 0 && ui.rows[row-1].ID == "claude:child"
	})
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModNone))
	waitUI(t, ui.app, func() bool {
		text := ui.preview.GetText(false)
		return strings.Contains(text, "fork of    recorded title")
	})
	parent, err := st.Get("claude:abc")
	if err != nil || parent.Forks != 1 {
		t.Fatalf("parent %+v %v", parent, err)
	}
	child, err := st.Get("claude:child")
	if err != nil || child.Name != "recorded title fork" || child.Parent != "recorded title" {
		t.Fatalf("child %+v %v", child, err)
	}
}

func TestForkOnGroupAsksForSessions(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	path := filepath.Join(t.TempDir(), "abc.jsonl")
	sess := model.Session{
		ID: "claude:abc", NativeID: "abc", Agent: "claude", Title: "recorded",
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
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyRune, '2', tcell.ModNone))
	waitUI(t, ui.app, func() bool { return ui.view == viewAgents })
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyRune, 'F', tcell.ModNone))
	waitUI(t, ui.app, func() bool {
		return strings.Contains(screenText(screen), "Switch to sessions before forking.")
	})
}

func TestForkEmptyNameStaysAName(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	path := filepath.Join(t.TempDir(), "abc.jsonl")
	sess := model.Session{
		ID: "claude:abc", NativeID: "abc", Agent: "claude", Title: "recorded",
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
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyRune, 'F', tcell.ModNone))
	var n int
	waitUI(t, ui.app, func() bool {
		field, ok := ui.app.GetFocus().(*tview.InputField)
		if ok && field.GetLabel() == "Name" {
			n = utf8.RuneCountInString(field.GetText())
			return n > 0
		}
		return false
	})
	for i := 0; i < n; i++ {
		ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone))
	}
	saveForkDialog(ui)
	waitUI(t, ui.app, func() bool {
		return strings.Contains(screenText(screen), "Type a name for the fork.")
	})
	if len(ui.rows) != 1 {
		t.Fatalf("rows %d", len(ui.rows))
	}
}

func TestForkWorktreeCheckboxIsOptional(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	path := filepath.Join(t.TempDir(), "parent.jsonl")
	sess := model.Session{
		ID: "claude:abc", NativeID: "abc", Agent: "claude", Title: "recorded",
		Updated: time.Date(2026, 3, 2, 15, 4, 5, 0, time.UTC), Messages: 1,
		SourcePath: path, CanDelete: true, DeleteMode: "file",
	}
	if err := st.Apply("claude", []model.Session{sess}, []store.Source{{Path: path, Mtime: 1}}); err != nil {
		t.Fatal(err)
	}
	origCopy, origRebuild := copySession, rebuildIndex
	t.Cleanup(func() {
		copySession = origCopy
		rebuildIndex = origRebuild
	})
	var got fork.Options
	copySession = func(s model.Session, opt fork.Options) (string, error) {
		got = opt
		return "child", nil
	}
	rebuildIndex = func(storeGot *store.Store) (store.Stats, []string, error) {
		child := sess
		child.ID = "claude:child"
		child.NativeID = "child"
		child.SourcePath = path + ".child"
		err := storeGot.Apply("claude", []model.Session{sess, child}, []store.Source{{Path: path, Mtime: 1}, {Path: child.SourcePath, Mtime: 1}})
		return store.Stats{}, nil, err
	}
	app := tview.NewApplication()
	ui := newUI(app, st, config.Defaults())
	ui.reload()
	screen, _ := startApp(t, ui)
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyRune, 'F', tcell.ModNone))
	waitUI(t, ui.app, func() bool {
		field, ok := ui.app.GetFocus().(*tview.InputField)
		return ok && field.GetLabel() == "Name"
	})
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	waitUI(t, ui.app, func() bool {
		flat := strings.Join(strings.Fields(screenText(screen)), " ")
		return strings.Contains(flat, "new worktree")
	})
	if !got.Worktree || got.Name != "recorded fork" {
		t.Fatalf("option %+v", got)
	}
}

func saveForkDialog(ui *ui) {
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
}

func TestNameDialogFitsANarrowPane(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	app := tview.NewApplication()
	ui := newUI(app, st, config.Defaults())
	ui.layout.SetRect(0, 0, 40, 24)
	box, field := nameDialogSize(ui)
	if box != 40 || field != 28 {
		t.Fatalf("narrow box %d field %d", box, field)
	}
	ui.layout.SetRect(0, 0, 120, 40)
	box, field = nameDialogSize(ui)
	if box != 62 || field != 36 {
		t.Fatalf("wide box %d field %d", box, field)
	}
}

func TestForkSuggestFitsTheNameLimit(t *testing.T) {
	if forkSuggest("recorded title") != "recorded title fork" || forkSuggest("") != "fork" || forkSuggest("-") != "fork" {
		t.Fatal(forkSuggest("recorded title"))
	}
	long := strings.Repeat("n", store.MaxSessionName)
	got := forkSuggest(long)
	if utf8.RuneCountInString(got) != store.MaxSessionName || !strings.HasSuffix(got, " fork") {
		t.Fatalf("long %d %q", utf8.RuneCountInString(got), got)
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

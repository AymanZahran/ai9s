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

func TestRenameAndSessionStats(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	path := filepath.Join(t.TempDir(), "abc.jsonl")
	sess := model.Session{
		ID: "claude:abc", NativeID: "abc", Agent: "claude", Title: "recorded title",
		Model: "sonnet", Updated: time.Date(2026, 3, 2, 15, 4, 5, 0, time.UTC), Messages: 4,
		SourcePath: path, CanDelete: true, DeleteMode: "file",
		Usage: model.Usage{Context: 1500, Window: 8000, Output: 20, Effort: "low"},
	}
	if err := st.Apply("claude", []model.Session{sess}, []store.Source{{Path: path, Mtime: 1}}); err != nil {
		t.Fatal(err)
	}
	app := tview.NewApplication()
	ui := newUI(app, st, config.Defaults())
	ui.reload()
	screen, _ := startApp(t, ui)
	menu, _ := ui.menuText()
	if !strings.Contains(menu, "<n>") || !strings.Contains(menu, "rename") || !strings.Contains(menu, "<u>") || !strings.Contains(menu, "usage") || !strings.Contains(menu, "<s>") {
		t.Fatalf("menu %s", menu)
	}

	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone))
	waitUI(t, ui.app, func() bool {
		field, ok := ui.app.GetFocus().(*tview.InputField)
		return ok && field.GetLabel() == "Name"
	})
	for _, r := range "Alpha" {
		ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	waitUI(t, ui.app, func() bool {
		return len(ui.rows) == 1 && ui.rows[0].Name == "Alpha" && ui.app.GetFocus() == ui.table
	})
	waitUI(t, ui.app, func() bool {
		painted := screenText(screen)
		return strings.Contains(painted, "Alpha") && strings.Contains(painted, "NAME") && !strings.Contains(painted, "TITLE")
	})

	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyRune, 'u', tcell.ModNone))
	var usage string
	waitUI(t, ui.app, func() bool {
		usage = screenText(screen)
		return strings.Contains(usage, "messages   4") && strings.Contains(usage, "context    1.5k") && strings.Contains(usage, "title      recorded")
	})
	if strings.Contains(usage, "sessions,") {
		t.Fatalf("usage showed every session\n%s", usage)
	}
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	waitUI(t, ui.app, func() bool { return ui.app.GetFocus() == ui.table })

	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModNone))
	waitUI(t, ui.app, func() bool { return strings.Contains(screenText(screen), "sessions,") })

	sess.Title = "rewritten title"
	if err := st.Apply("claude", []model.Session{sess}, []store.Source{{Path: path, Mtime: 2}}); err != nil {
		t.Fatal(err)
	}
	ui.app.QueueUpdateDraw(func() { ui.reload() })
	waitUI(t, ui.app, func() bool {
		return len(ui.rows) == 1 && ui.rows[0].Name == "Alpha" && ui.rows[0].Title == "rewritten title"
	})
}

func TestRenameOnGroupAsksForSessions(t *testing.T) {
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
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone))
	waitUI(t, ui.app, func() bool { return strings.Contains(screenText(screen), "Switch to sessions before renaming.") })
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	waitUI(t, ui.app, func() bool { return ui.app.GetFocus() == ui.table })
	ui.app.QueueEvent(tcell.NewEventKey(tcell.KeyRune, 'u', tcell.ModNone))
	waitUI(t, ui.app, func() bool {
		return strings.Contains(screenText(screen), "Switch to sessions for this usage.")
	})
}

func TestPluginNameUsesCustomName(t *testing.T) {
	env := map[string]string{"NAME": "plugin"}
	fillPluginEnv(env, model.Session{ID: "claude:abc", NativeID: "abc", Title: "recorded", Name: "Alpha"})
	if env["NAME"] != "Alpha" || env["TITLE"] != "recorded" {
		t.Fatalf("%v", env)
	}
	fillPluginEnv(env, model.Session{Title: "bad\x1b]0;x\a", CWD: "/work/app", Name: "A\u202eB"})
	if strings.Contains(env["TITLE"], "\x1b") || strings.Contains(env["NAME"], "\u202e") || env["CWD"] != "/work/app" {
		t.Fatalf("%v", env)
	}
}

func TestAcceptSessionNameRejectsControls(t *testing.T) {
	if acceptSessionName("a", '\n') || acceptSessionName("a", 0x1b) || acceptSessionName("a", '\u202e') {
		t.Fatal("accepted a control or bidi character")
	}
	if !acceptSessionName("Alpha", 'a') {
		t.Fatal("rejected a letter")
	}
}

func TestSessionStatsText(t *testing.T) {
	text := sessionStatsText(model.Session{
		ID: "claude:abc", NativeID: "abc", Agent: "claude", Title: "recorded", Name: "Alpha",
		Model: "sonnet", Messages: 4, Usage: model.Usage{Context: 1500, Output: 20},
	})
	for _, want := range []string{"Alpha", "Ca claude", "model      sonnet", "messages   4", "title      recorded", "context    1.5k", "out 20"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
}

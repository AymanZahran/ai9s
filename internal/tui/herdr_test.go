package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AymanZahran/ai9s/internal/act"
	"github.com/AymanZahran/ai9s/internal/config"
	"github.com/AymanZahran/ai9s/internal/model"
	"github.com/AymanZahran/ai9s/internal/store"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestHerdrOpenLeavesTheList(t *testing.T) {
	dir := t.TempDir()
	stamp := filepath.Join(dir, "stamp")
	script := filepath.Join(dir, "open")
	body := "#!/bin/sh\nprintf '%s\\n%s\\n%s\\n' \"$AI9S_ID\" \"$AI9S_YOLO\" \"$AI9S_BIN\" > \"$AI9S_HERDR_STAMP\"\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AI9S_HERDR_OPEN", script)
	t.Setenv("AI9S_HERDR_STAMP", stamp)
	orig := act.LookPath
	act.LookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	t.Cleanup(func() { act.LookPath = orig })

	st, err := store.Open(filepath.Join(dir, "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	path := filepath.Join(dir, "one.jsonl")
	sess := model.Session{
		ID: "claude:one", NativeID: "one", Agent: "claude", Title: "ship the feature",
		Name: "port", CWD: "/work/app", Updated: time.Date(2026, 3, 2, 15, 4, 5, 0, time.UTC),
		SourcePath: path, CanDelete: true, DeleteMode: "file",
	}
	if err := st.Apply("claude", []model.Session{sess}, []store.Source{{Path: path, Mtime: 1}}); err != nil {
		t.Fatal(err)
	}
	app := tview.NewApplication()
	ui := newUI(app, st, config.Defaults())
	ui.reload()
	send(ui.table, tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	waitForFile(t, stamp)
	got, err := os.ReadFile(stamp)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "claude:one\n0\n/usr/bin/claude\n" {
		t.Fatalf("hook env %q", got)
	}
	if ui.view != viewSessions || ui.pending != nil {
		t.Fatalf("view %s pending %v", ui.view, ui.pending != nil)
	}
}

func TestHerdrOpenHarness(t *testing.T) {
	dir := t.TempDir()
	stamp := filepath.Join(dir, "stamp")
	script := filepath.Join(dir, "open")
	body := "#!/bin/sh\nprintf '%s\\n%s\\n' \"$AI9S_ID\" \"$AI9S_BIN\" > \"$AI9S_HERDR_STAMP\"\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AI9S_HERDR_OPEN", script)
	t.Setenv("AI9S_HERDR_STAMP", stamp)
	orig := act.LookPath
	act.LookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	t.Cleanup(func() { act.LookPath = orig })

	st, err := store.Open(filepath.Join(dir, "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	path := filepath.Join(dir, "one.jsonl")
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
	send(ui.table, tcell.NewEventKey(tcell.KeyRune, '2', tcell.ModNone))
	send(ui.table, tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	waitForFile(t, stamp)
	got, err := os.ReadFile(stamp)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "\nclaude\n" {
		t.Fatalf("hook env %q", got)
	}
	if ui.view != viewAgents || ui.within.agent != "" {
		t.Fatalf("view %s within %q", ui.view, ui.within.agent)
	}
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}

func TestMergeEnvReplacesKeys(t *testing.T) {
	got := mergeEnv([]string{"AI9S_ID=old", "KEEP=1"}, map[string]string{"AI9S_ID": "new"})
	if len(got) != 2 || got[0] != "KEEP=1" || got[1] != "AI9S_ID=new" {
		t.Fatalf("env %q", got)
	}
}

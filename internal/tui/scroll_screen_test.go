package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AymanZahran/ai9s/internal/config"
	"github.com/AymanZahran/ai9s/internal/model"
	"github.com/AymanZahran/ai9s/internal/store"
	"github.com/rivo/tview"
)

func TestLongNameStaysOnTheRow(t *testing.T) {
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
	if len(ui.lines) < 2 {
		t.Fatal("no session row")
	}
	header := ui.lines[0]
	agentAt := strings.Index(header, "AGENT")
	nameAt := strings.Index(header, "NAME")
	if agentAt < 0 || nameAt < agentAt || strings.TrimSpace(header[nameAt+len("NAME"):]) != "" {
		t.Fatalf("header %q", header)
	}
	if !strings.Contains(ui.lines[1], "TAILMARK") || strings.Contains(ui.lines[1], "Gk") {
		t.Fatalf("row %q", ui.lines[1])
	}
	if ui.listWide < 200 {
		t.Fatalf("row was cut to %d", ui.listWide)
	}
	before := snap()
	if !strings.Contains(before, "grok") || strings.Contains(before, "TAILMARK") {
		t.Fatalf("visible list wide %d\n%s", ui.listWide, before)
	}
}

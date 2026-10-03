package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AymanZahran/ai9s/internal/config"
	"github.com/AymanZahran/ai9s/internal/model"
	"github.com/AymanZahran/ai9s/internal/store"
	"github.com/rivo/tview"
)

func TestLaunchScope(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	proj := filepath.Join(home, "proj")
	if err := os.Mkdir(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(home)
	display, roots := launchScope()
	if display != "" || len(roots) != 0 {
		t.Fatalf("home display %q roots %v", display, roots)
	}
	t.Chdir(proj)
	display, roots = launchScope()
	if !sameDir(display, proj) {
		t.Fatalf("display %q", display)
	}
	if len(roots) == 0 || !sameDir(roots[0], proj) {
		t.Fatalf("roots %v", roots)
	}
}

func TestScopeLimitsTheList(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	dir := t.TempDir()
	when := time.Date(2026, 3, 2, 15, 4, 5, 0, time.UTC)
	rows := []model.Session{
		{ID: "claude:in", NativeID: "in", Title: "inside", CWD: "/work/app"},
		{ID: "claude:sub", NativeID: "sub", Title: "nested", CWD: "/work/app/pkg"},
		{ID: "claude:out", NativeID: "out", Title: "outside", CWD: "/work/other"},
	}
	sources := make([]store.Source, len(rows))
	for i := range rows {
		rows[i].Agent = "claude"
		rows[i].Updated = when
		rows[i].Messages = 1
		rows[i].SourcePath = filepath.Join(dir, rows[i].NativeID)
		rows[i].CanDelete = true
		rows[i].DeleteMode = "file"
		sources[i] = store.Source{Path: rows[i].SourcePath, Mtime: 1}
	}
	if err := st.Apply("claude", rows, sources); err != nil {
		t.Fatal(err)
	}
	app := tview.NewApplication()
	ui := newUI(app, st, config.Defaults())
	ui.scope = "/work/app"
	ui.roots = []string{"/work/app"}
	ui.reload()
	if len(ui.rows) != 2 {
		t.Fatalf("rows %d", len(ui.rows))
	}
	text := strings.Join(ui.lines, "\n")
	if !strings.Contains(text, "inside") || !strings.Contains(text, "nested") || strings.Contains(text, "outside") {
		t.Fatalf("list %s", text)
	}
	if !strings.Contains(ui.crumbs.GetText(true), "/work/app") || strings.Contains(ui.crumbs.GetText(true), "› all") {
		t.Fatalf("crumbs %s", ui.crumbs.GetText(true))
	}
	ui.roots = nil
	ui.scope = ""
	ui.reload()
	if len(ui.rows) != 3 {
		t.Fatalf("cleared scope rows %d", len(ui.rows))
	}
}

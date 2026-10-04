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

func TestIdleProvidersWarnToInstallOrLogIn(t *testing.T) {
	orig := agentInstalled
	t.Cleanup(func() { agentInstalled = orig })
	agentInstalled = func(name string) (string, bool) {
		switch name {
		case "minimax":
			return "mcode", false
		case "mistral":
			return "vibe", false
		default:
			return name, false
		}
	}

	st, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	path := filepath.Join(t.TempDir(), "one.jsonl")
	sess := model.Session{
		ID: "claude:one", NativeID: "one", Agent: "claude", Title: "one",
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
	ui.setView(viewProviders)

	if ui.groups[0].key != "claude" || ui.groups[0].sessions != 1 {
		t.Fatalf("first provider %+v", ui.groups[0])
	}
	want := []string{"kimi", "minimax", "qwen", "mistral"}
	found := map[string]groupRow{}
	for _, g := range ui.groups {
		found[g.key] = g
	}
	for _, agent := range want {
		g, ok := found[agent]
		if !ok || g.sessions != 0 {
			t.Fatalf("%s group %+v present %v", agent, g, ok)
		}
	}
	if strings.Count(providerNames(ui.groups), "claude") != 1 {
		t.Fatalf("providers %s", providerNames(ui.groups))
	}

	row := groupIndex(t, ui.groups, "minimax")
	ui.table.Select(row+1, 0)
	ui.showRow(row + 1)
	preview := ui.preview.GetText(true)
	if !strings.Contains(preview, "minimax (mcode) is not on PATH") || !strings.Contains(preview, "log in") {
		t.Fatalf("preview %q", preview)
	}
	send(ui.table, tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if ui.view != viewProviders || ui.filter.GetText() != "" {
		t.Fatalf("enter view %s filter %q", ui.view, ui.filter.GetText())
	}
	if !confirmVisible(app) {
		t.Fatalf("enter focus %T", app.GetFocus())
	}

	agentInstalled = func(name string) (string, bool) {
		if name == "mistral" {
			return "vibe", true
		}
		return name, true
	}
	ui.showRow(groupIndex(t, ui.groups, "mistral") + 1)
	preview = ui.preview.GetText(true)
	if !strings.Contains(preview, "mistral (vibe) is installed") || !strings.Contains(preview, "Log in") {
		t.Fatalf("installed preview %q", preview)
	}

	ui.filter.SetText("dir:/work/missing")
	ui.reload()
	for _, g := range ui.groups {
		if g.key == "kimi" {
			t.Fatal("dir filter listed an agent with no sessions")
		}
	}
	ui.filter.SetText("agent:kimi")
	ui.reload()
	if len(ui.groups) != 1 || ui.groups[0].key != "kimi" || ui.groups[0].sessions != 0 {
		t.Fatalf("agent filter %+v", ui.groups)
	}
}

func providerNames(groups []groupRow) string {
	names := make([]string, len(groups))
	for i, g := range groups {
		names[i] = g.key
	}
	return strings.Join(names, ",")
}

func groupIndex(t *testing.T, groups []groupRow, key string) int {
	t.Helper()
	for i, g := range groups {
		if g.key == key {
			return i
		}
	}
	t.Fatalf("missing %s", key)
	return 0
}

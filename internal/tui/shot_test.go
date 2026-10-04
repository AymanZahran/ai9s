//go:build shots

package tui

import (
	"fmt"
	"html"
	"os"
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

// TestWriteScreenshots draws the real interface from example sessions and
// writes one HTML file per view. Regenerate with:
//
//	AI9S_SHOT_DIR=/tmp/ai9s-shots go test -tags shots -count=1 -run TestWriteScreenshots ./internal/tui
//
// The rows are fixtures. This test does not read the developer's index.
func TestWriteScreenshots(t *testing.T) {
	dir := os.Getenv("AI9S_SHOT_DIR")
	if dir == "" {
		t.Fatal("set AI9S_SHOT_DIR")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	st := shotStore(t)
	app := tview.NewApplication()
	ui := newUI(app, st, config.Defaults())
	ui.reload()
	const cols = 168
	screen := shotStart(t, ui, cols, 22)

	shoot := func(name string, height int, setup func()) {
		t.Helper()
		ui.app.QueueUpdateDraw(func() {
			screen.SetSize(cols, height)
			setup()
		})
		var page string
		ui.app.QueueUpdate(func() {
			page = screenDocument(screen)
		})
		path := filepath.Join(dir, name+".html")
		if err := os.WriteFile(path, []byte(page), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	reset := func() {
		ui.manual = nil
		ui.app.SetRoot(ui.layout, true)
		ui.closeDescribe()
		ui.closeCommand()
		ui.setView(viewSessions)
		ui.focusSessions()
	}

	shoot("sessions", 22, func() { reset() })
	shoot("describe", 30, func() {
		reset()
		ui.openDescribe()
	})
	shoot("directories", 22, func() {
		reset()
		ui.setView(viewDirectories)
		ui.focusSessions()
	})
	shoot("command", 34, func() {
		reset()
		ui.openCommand()
	})
	shoot("manual", 32, func() {
		reset()
		ui.showManual()
	})
	shoot("usage", 22, func() {
		reset()
		ui.showSessionStats()
	})
	shoot("plugins", 23, func() {
		reset()
		ui.cfg.Plugins = []config.Plugin{
			{Name: "open-editor", ShortCut: "e", Description: "edit", Scopes: []string{"sessions", "directories"}, Command: "open-editor"},
			{Name: "copy-session", ShortCut: "c", Description: "copy", Scopes: []string{"sessions", "directories"}, Command: "copy-session"},
			{Name: "git-story", ShortCut: "b", Description: "git", Scopes: []string{"sessions", "directories"}, Command: "git-story"},
			{Name: "new-terminal", ShortCut: "t", Description: "shell", Scopes: []string{"sessions", "directories"}, Command: "new-terminal"},
		}
		ui.focusSessions()
	})
}

func shotStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	now := time.Now()
	sessions := []model.Session{
		{
			ID: "claude:readme", NativeID: "readme", Agent: "claude",
			Title: "sketch the readme", Name: "readme draft",
			Summary: "Install notes for the public Homebrew tap.",
			CWD:     "/work/docs", Branch: "main", Model: "claude-sonnet",
			Updated: now.Add(-2 * time.Hour), Messages: 14,
			SourcePath: "/fixtures/claude/readme.jsonl", SourceMtime: 1,
			CanDelete: true, DeleteMode: "file",
			Usage: model.Usage{
				Context: 12000, Window: 200000, Input: 18000, Output: 4200,
				CacheRead: 9000, Total: 48000, CostUSD: 0.42, Effort: "medium",
			},
			Snippets: []model.Snippet{
				{Role: "user", Body: "Draft the install section for the public tap."},
				{Role: "assistant", Body: "The tap command needs the repository URL."},
			},
		},
		{
			ID: "codex:rename", NativeID: "rename", Agent: "codex",
			Title: "rename the command", CWD: "/work/cli", Branch: "main", Model: "gpt-5",
			Updated: now.Add(-5 * time.Hour), Messages: 22,
			SourcePath: "/fixtures/codex/rename.jsonl", SourceMtime: 1,
			CanDelete: true, DeleteMode: "file",
			Usage:    model.Usage{Context: 8000, Window: 128000, Input: 8000, Output: 2100, Total: 31000},
			Snippets: []model.Snippet{{Role: "user", Body: "Keep NAME as the last column."}},
		},
		{
			ID: "hermes:deploy", NativeID: "deploy", Agent: "hermes",
			Title: "check the deploy", CWD: "/work/ops", Branch: "main", Model: "hermes-3",
			Updated: now.Add(-26 * time.Hour), Messages: 6,
			SourcePath: "/fixtures/hermes/deploy.jsonl", SourceMtime: 1,
			CanDelete: true, DeleteMode: "file",
			Snippets: []model.Snippet{{Role: "user", Body: "Confirm the pages workflow finished."}},
		},
		{
			ID: "grok:quiet", NativeID: "quiet", Agent: "grok",
			Title: "quiet the startup line", CWD: "/work/cli", Branch: "main", Model: "grok-4",
			Updated: now.Add(-3 * 24 * time.Hour), Messages: 9,
			SourcePath: "/fixtures/grok/quiet.jsonl", SourceMtime: 1,
			CanDelete: true, DeleteMode: "file",
			Usage:    model.Usage{Context: 4000, Effort: "low"},
			Snippets: []model.Snippet{{Role: "user", Body: "Do not print a line before the screen opens."}},
		},
		{
			ID: "kiro:button", NativeID: "button", Agent: "kiro",
			Title: "wire the button", CWD: "/work/app", Branch: "main", Model: "kiro",
			Updated: now.Add(-6 * 24 * time.Hour), Messages: 4,
			SourcePath: "/fixtures/kiro/button.jsonl", SourceMtime: 1,
			CanDelete: true, DeleteMode: "file",
			Snippets: []model.Snippet{{Role: "user", Body: "The example button only needs a label."}},
		},
		{
			ID: "copilot:path", NativeID: "path", Agent: "copilot",
			Title: "keep the ending of a long path",
			CWD:   "/work/projects/documentation/website/assets", Branch: "main", Model: "gpt-4.1",
			Updated: now.Add(-8 * 24 * time.Hour), Messages: 11,
			SourcePath: "/fixtures/copilot/path.jsonl", SourceMtime: 1,
			CanDelete: true, DeleteMode: "file",
			Snippets: []model.Snippet{{Role: "user", Body: "A long directory should keep its ending."}},
		},
		{
			ID: "antigravity:views", NativeID: "views", Agent: "antigravity",
			Title: "group by directory", CWD: "/work/docs/site", Branch: "feature/views", Model: "gemini-2.5",
			Updated: now.Add(-10 * 24 * time.Hour), Messages: 7,
			SourcePath: "/fixtures/antigravity/views.jsonl", SourceMtime: 1,
			CanDelete: true, DeleteMode: "file",
			Snippets: []model.Snippet{{Role: "user", Body: "Directories are a view, not a second window."}},
		},
		{
			ID: "cursor:sign", NativeID: "sign", Agent: "cursor",
			Title: "sign the checksums", CWD: "/work/release", Branch: "main", Model: "cursor",
			Updated: now.Add(-12 * 24 * time.Hour), Messages: 5,
			SourcePath: "/fixtures/cursor/sign.jsonl", SourceMtime: 1,
			CanDelete: true, DeleteMode: "file",
			Snippets: []model.Snippet{{Role: "user", Body: "Sign the checksum file on the next release."}},
		},
	}
	grouped := map[string][]model.Session{}
	for _, sess := range sessions {
		grouped[sess.Agent] = append(grouped[sess.Agent], sess)
	}
	for agent, list := range grouped {
		files := make([]store.Source, 0, len(list))
		for _, sess := range list {
			files = append(files, store.Source{Path: sess.SourcePath, Mtime: sess.SourceMtime})
		}
		if err := st.Apply(agent, list, files); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.SetName("claude:readme", "readme draft"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetBookmark("claude:readme", true); err != nil {
		t.Fatal(err)
	}
	return st
}

func shotStart(t *testing.T, ui *ui, cols, rows int) tcell.SimulationScreen {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	ui.app.SetScreen(screen)
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	screen.SetSize(cols, rows)
	ui.app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		w, _ := screen.Size()
		logo := 10
		if ui.cfg.Body.UI.Logoless {
			logo = 0
		}
		avail := w - logo - 1
		if avail < 32 {
			avail = 32
		}
		if avail == ui.menuMeasured {
			return false
		}
		ui.menuMeasured = avail
		ui.paintHeader()
		return false
	})
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
	return screen
}

func screenDocument(screen tcell.SimulationScreen) string {
	w, h := screen.Size()
	var b strings.Builder
	b.WriteString("<!DOCTYPE html><html><head><meta charset=\"utf-8\"><style>")
	b.WriteString("html,body{margin:0;background:#000}")
	b.WriteString("pre{margin:0;background:#000;display:inline-block;font-family:Menlo,ui-monospace,monospace;font-size:13px;line-height:16px}")
	b.WriteString("</style></head><body><pre id=\"screen\">")
	for y := 0; y < h; y++ {
		var run strings.Builder
		var runCSS string
		flush := func() {
			if run.Len() == 0 {
				return
			}
			fmt.Fprintf(&b, "<span style=\"%s\">%s</span>", runCSS, html.EscapeString(run.String()))
			run.Reset()
		}
		for x := 0; x < w; {
			text, style, width := screen.Get(x, y)
			if width == 0 {
				x++
				continue
			}
			if text == "" {
				text = " "
			}
			css := styleCSS(style)
			if run.Len() > 0 && css != runCSS {
				flush()
			}
			runCSS = css
			run.WriteString(text)
			if width < 1 {
				width = 1
			}
			x += width
		}
		flush()
		if y+1 < h {
			b.WriteByte('\n')
		}
	}
	b.WriteString("</pre></body></html>\n")
	return b.String()
}

func styleCSS(style tcell.Style) string {
	fg, bg, attr := style.Decompose()
	if attr&tcell.AttrReverse != 0 {
		fg, bg = bg, fg
	}
	parts := []string{
		"color:" + colorHex(fg, "#ffffff"),
		"background:" + colorHex(bg, "#000000"),
	}
	if attr&tcell.AttrBold != 0 {
		parts = append(parts, "font-weight:700")
	}
	if attr&tcell.AttrUnderline != 0 {
		parts = append(parts, "text-decoration:underline")
	}
	if attr&tcell.AttrDim != 0 {
		parts = append(parts, "opacity:.65")
	}
	return strings.Join(parts, ";")
}

func colorHex(c tcell.Color, fallback string) string {
	if !c.Valid() {
		return fallback
	}
	v := c.Hex()
	if v < 0 {
		return fallback
	}
	return fmt.Sprintf("#%06x", v)
}

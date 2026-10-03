package tui

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

// menuHint is one key shown in the top menu.
type menuHint struct {
	key       string
	label     string
	keyColor  string
	textColor string
	hi        string
	active    bool
}

func (h menuHint) visibleWidth() int {
	return runewidth.StringWidth("<" + h.key + "> " + h.label)
}

func (h menuHint) render() string {
	return menuItem(h.keyColor, h.textColor, h.hi, h.key, h.label, h.active)
}

// menuCols is how many shortcut columns fit. Five matches the view keys.
func menuCols(width int) int {
	const minCol = 16
	if width < minCol {
		return 1
	}
	n := width / minCol
	if n > 5 {
		return 5
	}
	return n
}

// renderGrid paints hints in fixed columns so each row lines up with the one above.
// A group always starts on a new row.
func renderGrid(items []menuHint, width, cols int) string {
	if len(items) == 0 || cols < 1 {
		return ""
	}
	if width < cols {
		width = cols
	}
	base := width / cols
	extra := width % cols
	var b strings.Builder
	for i, h := range items {
		if i%cols == 0 {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteByte(' ')
		}
		col := base
		if i%cols < extra {
			col++
		}
		cell := h.render()
		visible := h.visibleWidth()
		if visible < col {
			cell += strings.Repeat(" ", col-visible)
		}
		b.WriteString(cell)
	}
	return b.String()
}

func (ui *ui) menuWidth() int {
	if ui.menuMeasured >= 32 {
		return ui.menuMeasured
	}
	if ui.header != nil {
		_, _, w, _ := ui.header.GetInnerRect()
		if w >= 32 {
			return w
		}
	}
	return 100
}

func (ui *ui) menuText() (string, int) {
	width := ui.menuWidth() - 1
	if width < 16 {
		width = 16
	}
	cols := menuCols(width)
	num := ui.cfg.Skin.Frame.Menu.NumKey
	key := ui.cfg.Skin.Frame.Menu.Key
	fg := ui.cfg.Skin.Frame.Menu.Fg
	hi := ui.cfg.Skin.Frame.Title.Highlight

	var parts []string
	if row := renderGrid(ui.viewHints(num, fg, hi), width, cols); row != "" {
		parts = append(parts, row)
	}
	if row := renderGrid(ui.actionHints(key, fg), width, cols); row != "" {
		parts = append(parts, row)
	}
	if row := renderGrid(ui.pluginMenu(key, fg), width, cols); row != "" {
		parts = append(parts, row)
	}
	text := strings.Join(parts, "\n")
	if text == "" {
		return "", 0
	}
	return text, strings.Count(text, "\n") + 1
}

func (ui *ui) viewHints(num, fg, hi string) []menuHint {
	active := ui.view
	if active == "" {
		active = viewSessions
	}
	hints := make([]menuHint, len(viewSpecs))
	for i, spec := range viewSpecs {
		hints[i] = menuHint{
			key: spec.key, label: spec.name,
			keyColor: num, textColor: fg, hi: hi,
			active: spec.name == active,
		}
	}
	return hints
}

func (ui *ui) actionHints(keyColor, fg string) []menuHint {
	item := func(k, label string) menuHint {
		return menuHint{key: k, label: label, keyColor: keyColor, textColor: fg}
	}
	switch ui.focused {
	case "preview":
		return []menuHint{
			item("j/k ↑/↓", "line"),
			item("h/l ←/→", "pan"),
			item("⌘↑/⌘↓", "page"),
			item("⌘←/⌘→", "page"),
			item("g/G", "top/end"),
			item("wheel", "scroll"),
			item("esc", "list"),
			item("ctrl-d", "delete"),
			item("enter", "resume"),
			item("n", "rename"),
			item("u", "usage"),
			item("s", "stats"),
			item("?", "manual"),
			item("q", "quit"),
		}
	case "filter":
		return []menuHint{
			item("↑/↓", "select"),
			item("⌘↑/⌘↓", "page"),
			item("⌘←/⌘→", "page"),
			item("enter", "list"),
			item("esc", "back"),
			item("tab", "describe"),
		}
	case "command":
		return []menuHint{
			item("↑/↓", "select"),
			item("⌘↑/⌘↓", "page"),
			item("⌘←/⌘→", "page"),
			item("enter", "apply"),
			item(":", "next"),
			item("esc", "cancel"),
			item("?", "manual"),
		}
	default:
		enter := "resume"
		if ui.view != "" && ui.view != viewSessions {
			enter = "filter"
		}
		return []menuHint{
			item("/", "filter"),
			item(":", "command"),
			item("d", "describe"),
			item("ctrl-d", "delete"),
			item("j/k ↑/↓", "line"),
			item("g/G", "top/end"),
			item("h/l ←/→", "pan"),
			item("enter", enter),
			item("⌘↑/⌘↓", "page"),
			item("⌘←/⌘→", "page"),
			item("wheel", "scroll"),
			item("esc", "back"),
			item("a", "agent"),
			item("p", "directory"),
			item("o", "sort"),
			item("r", "reindex"),
			item("n", "rename"),
			item("u", "usage"),
			item("s", "stats"),
			item("?", "manual"),
			item("q", "quit"),
		}
	}
}

// pluginMenu is every plugin the current view accepts. Command mode hides
// them because that field consumes the key.
func (ui *ui) pluginMenu(keyColor, fg string) []menuHint {
	if ui.focused == "command" {
		return nil
	}
	view := ui.view
	if view == "" {
		view = viewSessions
	}
	var hints []menuHint
	for _, p := range ui.cfg.Plugins {
		if !p.Allows(view) {
			continue
		}
		label := strings.TrimSpace(p.Description)
		if label == "" {
			label = p.Name
		}
		if runes := []rune(label); len(runes) > 24 {
			label = string(runes[:24])
		}
		hints = append(hints, menuHint{
			key: p.ShortCut, label: label,
			keyColor: keyColor, textColor: fg,
		})
	}
	return hints
}

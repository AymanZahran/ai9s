package tui

import (
	"fmt"
	"strings"

	"github.com/AymanZahran/air9s/internal/act"
	"github.com/AymanZahran/air9s/internal/index"
	"github.com/AymanZahran/air9s/internal/model"
	"github.com/AymanZahran/air9s/internal/query"
	"github.com/AymanZahran/air9s/internal/store"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Run draws the session list. It returns a resume command when the user
// picks a session, or nil when they quit.
func Run(st *store.Store) (*act.Command, error) {
	app := tview.NewApplication()
	ui := newUI(app, st)
	ui.reload()
	if err := app.SetRoot(ui.layout, true).EnableMouse(true).Run(); err != nil {
		return nil, err
	}
	return ui.pending, nil
}

type ui struct {
	app             *tview.Application
	store           *store.Store
	layout          *tview.Flex
	header          *tview.TextView
	filter          *tview.InputField
	table           *tview.Table
	preview         *tview.TextView
	footer          *tview.TextView
	rows            []model.Session
	agents          []string
	warnings        []string
	yolo            bool
	pending         *act.Command
	busy            bool
	focused         string
	view            string
	pages           *tview.Pages
	command         *tview.InputField
	commandOpen     bool
	suppressCommand bool
	groups          []groupRow
	suggestions     []commandHint
}

func newUI(app *tview.Application, st *store.Store) *ui {
	ui := &ui{app: app, store: st}
	ui.header = tview.NewTextView().SetDynamicColors(true)
	ui.header.SetBackgroundColor(tcell.ColorDarkBlue)
	ui.footer = tview.NewTextView().SetDynamicColors(true)
	ui.footer.SetBackgroundColor(tcell.ColorDarkSlateGray)

	ui.filter = tview.NewInputField().SetLabel(" filter ").SetFieldWidth(0)
	ui.filter.SetLabelColor(tcell.ColorYellow)
	ui.filter.SetChangedFunc(func(string) { ui.reload() })
	ui.filter.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			ui.focusPreview()
		case tcell.KeyEnter, tcell.KeyEscape, tcell.KeyBacktab:
			ui.focusSessions()
		}
	})
	ui.filter.SetFocusFunc(func() {
		ui.focused = "filter"
		ui.paintChrome()
	})

	ui.command = tview.NewInputField().SetLabel(" : ").SetFieldWidth(0)
	ui.command.SetLabelColor(tcell.ColorYellow)
	ui.command.SetChangedFunc(func(text string) {
		if ui.suppressCommand || !ui.commandOpen {
			return
		}
		ui.paintSuggestions(text)
	})
	ui.command.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			ui.applyCommand(ui.command.GetText())
		case tcell.KeyEscape:
			ui.closeCommand()
		}
	})
	ui.command.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyRune && ev.Rune() == ':' {
			ui.cycleCommandView()
			return nil
		}
		return ev
	})
	ui.command.SetFocusFunc(func() {
		ui.focused = "command"
		ui.paintChrome()
	})
	ui.pages = tview.NewPages().
		AddPage("filter", ui.filter, true, true).
		AddPage("command", ui.command, true, false)

	ui.table = tview.NewTable().SetSelectable(true, false).SetFixed(1, 0)
	ui.table.SetBorder(true).SetTitle(" sessions ")
	ui.table.SetSelectedStyle(tcell.StyleDefault.Reverse(true))
	ui.table.SetSelectionChangedFunc(func(row, _ int) { ui.showRow(row) })
	ui.table.SetInputCapture(ui.tableKeys)
	ui.table.SetFocusFunc(func() {
		ui.focused = "table"
		ui.paintChrome()
	})

	ui.preview = tview.NewTextView().SetDynamicColors(true).SetWrap(true).SetScrollable(true)
	ui.preview.SetBorder(true).SetTitle(" preview ")
	ui.preview.SetInputCapture(ui.previewKeys)
	ui.preview.SetDoneFunc(ui.previewDone)
	ui.preview.SetFocusFunc(func() {
		ui.focused = "preview"
		ui.paintChrome()
	})

	body := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(ui.table, 0, 3, true).
		AddItem(ui.preview, 0, 2, false)
	ui.layout = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.header, 5, 0, false).
		AddItem(ui.pages, 1, 0, false).
		AddItem(body, 0, 1, true).
		AddItem(ui.footer, 1, 0, false)
	ui.focused = "table"
	ui.view = viewSessions
	ui.paintChrome()
	return ui
}

const (
	footerSessions = `[yellow]tab[-] preview   [yellow]enter[-] resume   [yellow]d[-] delete   [yellow]/[-] filter   [yellow]a[-] agent   [yellow]p[-] directory   [yellow]o[-] sort   [yellow]y[-] yolo   [yellow]r[-] reindex   [yellow]s[-] stats   [yellow]?[-] help   [yellow]q[-] quit`
	footerPreview  = `[yellow]j/k[-] ↑↓ scroll   [yellow]ctrl-b/f[-] page   [yellow]g/G[-] top/end   [yellow]tab[-] [yellow]esc[-] sessions   [yellow]enter[-] resume   [yellow]q[-] quit`
)

func (ui *ui) paintChrome() {
	// Do not call HasFocus here. TextView.Focus holds its lock while this runs.
	ui.table.SetBorderColor(tcell.ColorWhite)
	ui.preview.SetBorderColor(tcell.ColorWhite)
	if ui.commandOpen {
		ui.table.SetTitle(" commands ")
	} else {
		ui.table.SetTitle(viewTitle(ui.view))
	}
	ui.preview.SetTitle(" preview ")
	ui.footer.SetText(footerSessions)
	switch ui.focused {
	case "preview":
		ui.preview.SetBorderColor(tcell.ColorYellow)
		ui.preview.SetTitle(" preview · scroll ")
		ui.footer.SetText(footerPreview)
	case "table", "command":
		ui.table.SetBorderColor(tcell.ColorYellow)
	}
	ui.paintHeader()
}

func (ui *ui) focusSessions() {
	ui.app.SetFocus(ui.table)
}

func (ui *ui) focusPreview() {
	ui.app.SetFocus(ui.preview)
}

func (ui *ui) previewDone(key tcell.Key) {
	if key == tcell.KeyEnter {
		ui.resumeSelected()
		return
	}
	ui.focusSessions()
}

func (ui *ui) tableKeys(ev *tcell.EventKey) *tcell.EventKey {
	if ui.busy {
		if ev.Key() == tcell.KeyRune && ev.Rune() == 'q' {
			ui.app.Stop()
		}
		return nil
	}
	switch ev.Key() {
	case tcell.KeyEnter:
		ui.resumeSelected()
		return nil
	case tcell.KeyCtrlD:
		ui.confirmDelete()
		return nil
	case tcell.KeyTab, tcell.KeyBacktab:
		ui.focusPreview()
		return nil
	}
	if ev.Key() != tcell.KeyRune {
		return ev
	}
	switch ev.Rune() {
	case 'q':
		ui.app.Stop()
	case '/':
		ui.app.SetFocus(ui.filter)
	case ':':
		ui.openCommand()
	case 'd':
		ui.confirmDelete()
	case 'y':
		ui.yolo = !ui.yolo
		ui.paintHeader()
	case 'r':
		ui.reindex()
	case 'a':
		ui.cycleAgent()
	case 'o':
		ui.cycleSort()
	case 'p':
		ui.promptDir()
	case 's':
		ui.showStats()
	case '?':
		ui.showManual()
	case 'j':
		return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
	case 'k':
		return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
	case '1', '2', '3', '4', '5':
		if spec, ok := viewByKey(string(ev.Rune())); ok {
			ui.setView(spec.name)
		}
	default:
		return ev
	}
	return nil
}

func (ui *ui) previewKeys(ev *tcell.EventKey) *tcell.EventKey {
	if ui.busy {
		if ev.Key() == tcell.KeyRune && ev.Rune() == 'q' {
			ui.app.Stop()
		}
		return nil
	}
	if ev.Key() == tcell.KeyCtrlD {
		ui.confirmDelete()
		return nil
	}
	if ev.Key() == tcell.KeyRune {
		switch ev.Rune() {
		case 'q':
			ui.app.Stop()
			return nil
		case 'd':
			ui.confirmDelete()
			return nil
		case '/':
			ui.app.SetFocus(ui.filter)
			return nil
		case ':':
			ui.openCommand()
			return nil
		case '?':
			ui.showManual()
			return nil
		case '1', '2', '3', '4', '5':
			if spec, ok := viewByKey(string(ev.Rune())); ok {
				ui.setView(spec.name)
			}
			return nil
		}
	}
	return ev
}

func (ui *ui) reload() {
	if ui.commandOpen {
		ui.paintSuggestions(ui.command.GetText())
		ui.paintHeader()
		return
	}
	f := query.Parse(ui.filter.GetText())
	rows, err := ui.store.Search(f, 400)
	if err != nil {
		ui.header.SetText("[red]search failed: " + err.Error() + "[-]")
		return
	}
	ui.rows = rows
	stats, _ := ui.store.Stats()
	ui.agents = ui.agents[:0]
	for _, a := range stats.Agents {
		ui.agents = append(ui.agents, a.Agent)
	}
	ui.paintHeader()
	if ui.view != "" && ui.view != viewSessions {
		ui.paintGroups()
		return
	}
	ui.paintSessions()
}

func colorOf(agent string) tcell.Color {
	switch agent {
	case "claude":
		return tcell.ColorOrange
	case "codex":
		return tcell.ColorGreen
	case "copilot":
		return tcell.ColorDodgerBlue
	case "grok":
		return tcell.ColorAqua
	case "agy":
		return tcell.ColorPurple
	case "gemini":
		return tcell.ColorYellow
	case "cursor":
		return tcell.ColorSilver
	case "opencode":
		return tcell.ColorFuchsia
	case "hermes":
		return tcell.NewHexColor(0xFFD700)
	case "openclaw":
		return tcell.NewHexColor(0x2DD4BF)
	case "junie":
		return tcell.NewHexColor(0x7DD3FC)
	case "jules":
		return tcell.NewHexColor(0x5A009D)
	case "goose":
		return tcell.NewHexColor(0xF59E0B)
	case "cline":
		return tcell.NewHexColor(0x22C55E)
	case "aider":
		return tcell.NewHexColor(0xFB7185)
	case "kiro":
		return tcell.NewHexColor(0xA78BFA)
	default:
		return tcell.ColorWhite
	}
}

func (ui *ui) paintHeader() {
	stats, _ := ui.store.Stats()
	var b strings.Builder
	fmt.Fprintf(&b, " %s\n %s\n", hotkeyViews(ui.view), hotkeyActions(ui.focused, ui.view))
	fmt.Fprintf(&b, "[::b] air9s [-]  %d sessions   %d messages", stats.Sessions, stats.Messages)
	if ui.yolo {
		b.WriteString("   [yellow]yolo[-]")
	}
	if ui.busy {
		b.WriteString("   indexing…")
	}
	b.WriteByte('\n')
	if len(stats.Agents) == 0 {
		b.WriteString(" ")
	}
	for i, a := range stats.Agents {
		if i > 0 {
			b.WriteString("   ")
		}
		fmt.Fprintf(&b, "%s [%s]%s %d[-]", Icon(a.Agent), agentColor(a.Agent), a.Agent, a.Sessions)
	}
	if len(ui.warnings) > 0 {
		fmt.Fprintf(&b, "\n[red]%s[-]", ui.warnings[0])
	} else {
		b.WriteString("\n[gray]agent: dir: branch: model: date:<7d sort:recent[-]")
	}
	ui.header.SetText(b.String())
}

func (ui *ui) showRow(row int) {
	if ui.commandOpen {
		if row <= 0 || row-1 >= len(ui.suggestions) {
			return
		}
		hint := ui.suggestions[row-1]
		ui.preview.SetText(fmt.Sprintf("[::b]%s[-]\n%s\n", hint.insert, hint.hint))
		ui.preview.ScrollToBeginning()
		return
	}
	if ui.view != "" && ui.view != viewSessions {
		ui.showGroup(row)
		return
	}
	if row <= 0 || row-1 >= len(ui.rows) {
		return
	}
	id := ui.rows[row-1].ID
	full, err := ui.store.Get(id)
	if err != nil {
		ui.preview.SetText(err.Error())
		return
	}
	ui.preview.SetText(preview(full))
	ui.preview.ScrollToBeginning()
}

func (ui *ui) selected() (model.Session, bool) {
	row, _ := ui.table.GetSelection()
	if row <= 0 || row-1 >= len(ui.rows) {
		return model.Session{}, false
	}
	full, err := ui.store.Get(ui.rows[row-1].ID)
	if err != nil {
		ui.alert(err.Error())
		return model.Session{}, false
	}
	return full, true
}

func (ui *ui) resumeSelected() {
	if ui.view != "" && ui.view != viewSessions {
		ui.activateGroup()
		return
	}
	s, ok := ui.selected()
	if !ok {
		return
	}
	cmd, err := act.Plan(s, ui.yolo)
	if err != nil {
		ui.alert(err.Error())
		return
	}
	ui.pending = &cmd
	ui.app.Stop()
}

func (ui *ui) confirmDelete() {
	if ui.view != "" && ui.view != viewSessions {
		ui.alert("Switch to sessions before deleting.")
		return
	}
	s, ok := ui.selected()
	if !ok {
		return
	}
	if !s.CanDelete {
		reason := s.DeleteReason
		if reason == "" {
			reason = "Deletion is disabled for " + Label(s.Agent) + "."
		}
		ui.alert(reason)
		return
	}
	text := fmt.Sprintf("Delete this %s session?\n\n%s\n%s", Label(s.Agent), s.Title, shortPath(s.SourcePath))
	modal := tview.NewModal().SetText(text).AddButtons([]string{"Delete", "Cancel"}).SetDoneFunc(func(_ int, label string) {
		ui.app.SetRoot(ui.layout, true)
		ui.focusSessions()
		if label != "Delete" {
			return
		}
		var err error
		ui.app.Suspend(func() {
			err = act.Delete(s)
		})
		if err != nil {
			ui.alert(err.Error())
			return
		}
		if err := ui.store.Forget(s.ID); err != nil {
			ui.alert(err.Error())
			return
		}
		ui.reload()
	})
	ui.app.SetRoot(modal, true)
}

func (ui *ui) alert(msg string) {
	modal := tview.NewModal().SetText(msg).AddButtons([]string{"OK"}).SetDoneFunc(func(int, string) {
		ui.app.SetRoot(ui.layout, true)
		ui.focusSessions()
	})
	ui.app.SetRoot(modal, false).SetFocus(modal)
}

func (ui *ui) showStats() {
	stats, err := ui.store.Stats()
	if err != nil {
		ui.alert(err.Error())
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d sessions, %d messages\n\n", stats.Sessions, stats.Messages)
	for _, a := range stats.Agents {
		fmt.Fprintf(&b, "%s %-10s %5d sessions   %6d messages\n", Icon(a.Agent), a.Agent, a.Sessions, a.Messages)
	}
	if len(ui.warnings) > 0 {
		b.WriteString("\n")
		for _, w := range ui.warnings {
			b.WriteString(w)
			b.WriteByte('\n')
		}
	}
	ui.alert(b.String())
}

func (ui *ui) cycleAgent() {
	cur := query.Parse(ui.filter.GetText()).Agent
	next := ""
	if cur == "" {
		if len(ui.agents) > 0 {
			next = ui.agents[0]
		}
	} else {
		for i, a := range ui.agents {
			if a == cur {
				if i+1 < len(ui.agents) {
					next = ui.agents[i+1]
				}
				break
			}
		}
	}
	ui.filter.SetText(setToken(ui.filter.GetText(), "agent", next))
}

func (ui *ui) cycleSort() {
	order := []string{"", "oldest", "messages", "title"}
	cur := query.Parse(ui.filter.GetText()).Sort
	if cur == "recent" {
		cur = ""
	}
	next := "oldest"
	for i, s := range order {
		if s == cur {
			next = order[(i+1)%len(order)]
			break
		}
	}
	ui.filter.SetText(setToken(ui.filter.GetText(), "sort", next))
}

func (ui *ui) promptDir() {
	text := ui.filter.GetText()
	if !strings.Contains(text, "dir:") && !strings.Contains(text, "cwd:") {
		if text != "" && !strings.HasSuffix(text, " ") {
			text += " "
		}
		text += "dir:"
		ui.filter.SetText(text)
	}
	ui.app.SetFocus(ui.filter)
}

func (ui *ui) reindex() {
	if ui.busy {
		return
	}
	ui.busy = true
	ui.paintHeader()
	go func() {
		_, warnings, err := index.Rebuild(ui.store)
		ui.app.QueueUpdateDraw(func() {
			ui.busy = false
			ui.warnings = warnings
			if err != nil {
				ui.warnings = append(ui.warnings, err.Error())
			}
			ui.reload()
		})
	}()
}

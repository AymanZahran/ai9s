package tui

import (
	"fmt"
	"strings"

	"github.com/AymanZahran/ai9s/internal/act"
	"github.com/AymanZahran/ai9s/internal/index"
	"github.com/AymanZahran/ai9s/internal/model"
	"github.com/AymanZahran/ai9s/internal/query"
	"github.com/AymanZahran/ai9s/internal/store"
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
	app      *tview.Application
	store    *store.Store
	layout   *tview.Flex
	header   *tview.TextView
	filter   *tview.InputField
	table    *tview.Table
	preview  *tview.TextView
	footer   *tview.TextView
	rows     []model.Session
	agents   []string
	warnings []string
	yolo     bool
	pending  *act.Command
	busy     bool
}

func newUI(app *tview.Application, st *store.Store) *ui {
	ui := &ui{app: app, store: st}
	ui.header = tview.NewTextView().SetDynamicColors(true)
	ui.header.SetBackgroundColor(tcell.ColorDarkBlue)
	ui.footer = tview.NewTextView().SetDynamicColors(true)
	ui.footer.SetBackgroundColor(tcell.ColorDarkSlateGray)
	ui.footer.SetText(`[yellow]enter[-] resume   [yellow]d[-] delete   [yellow]/[-] filter   [yellow]a[-] agent   [yellow]p[-] directory   [yellow]o[-] sort   [yellow]y[-] yolo   [yellow]r[-] reindex   [yellow]s[-] stats   [yellow]?[-] help   [yellow]q[-] quit`)

	ui.filter = tview.NewInputField().SetLabel(" filter ").SetFieldWidth(0)
	ui.filter.SetLabelColor(tcell.ColorYellow)
	ui.filter.SetChangedFunc(func(string) { ui.reload() })
	ui.filter.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter || key == tcell.KeyEscape {
			ui.app.SetFocus(ui.table)
		}
	})

	ui.table = tview.NewTable().SetSelectable(true, false).SetFixed(1, 0)
	ui.table.SetBorder(true).SetTitle(" sessions ")
	ui.table.SetSelectedStyle(tcell.StyleDefault.Reverse(true))
	ui.table.SetSelectionChangedFunc(func(row, _ int) { ui.showRow(row) })
	ui.table.SetInputCapture(ui.tableKeys)

	ui.preview = tview.NewTextView().SetDynamicColors(true).SetWrap(true).SetScrollable(true)
	ui.preview.SetBorder(true).SetTitle(" preview ")

	body := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(ui.table, 0, 3, true).
		AddItem(ui.preview, 0, 2, false)
	ui.layout = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.header, 2, 0, false).
		AddItem(ui.filter, 1, 0, false).
		AddItem(body, 0, 1, true).
		AddItem(ui.footer, 1, 0, false)
	return ui
}

func (ui *ui) tableKeys(ev *tcell.EventKey) *tcell.EventKey {
	if ui.busy {
		if ev.Key() == tcell.KeyRune && ev.Rune() == 'q' {
			ui.app.Stop()
		}
		return nil
	}
	if ev.Key() == tcell.KeyEnter {
		ui.resumeSelected()
		return nil
	}
	if ev.Key() == tcell.KeyCtrlD {
		ui.confirmDelete()
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
		ui.showHelp()
	case 'j':
		return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
	case 'k':
		return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
	default:
		return ev
	}
	return nil
}

func (ui *ui) reload() {
	f := query.Parse(ui.filter.GetText())
	rows, err := ui.store.Search(f, 400)
	if err != nil {
		ui.header.SetText("[red]search failed: " + err.Error() + "[-]")
		return
	}
	prev := ""
	if row, _ := ui.table.GetSelection(); row > 0 && row-1 < len(ui.rows) {
		prev = ui.rows[row-1].ID
	}
	ui.rows = rows
	stats, _ := ui.store.Stats()
	ui.agents = ui.agents[:0]
	for _, a := range stats.Agents {
		ui.agents = append(ui.agents, a.Agent)
	}
	ui.paintHeader()
	ui.table.Clear()
	headers := []string{"AGE", "AGENT", "DIR", "BRANCH", "MSGS", "TITLE"}
	exp := []int{0, 0, 1, 0, 0, 3}
	for i, h := range headers {
		cell := tview.NewTableCell(h).SetSelectable(false).SetTextColor(tcell.ColorYellow).SetExpansion(exp[i])
		ui.table.SetCell(0, i, cell)
	}
	sel := 1
	for i, s := range rows {
		if s.ID == prev {
			sel = i + 1
		}
		dir := shortPath(s.CWD)
		branch := s.Branch
		if branch == "" {
			branch = "-"
		}
		ui.table.SetCell(i+1, 0, tview.NewTableCell(relAge(s.Updated)))
		ui.table.SetCell(i+1, 1, tview.NewTableCell(s.Agent).SetTextColor(colorOf(s.Agent)))
		ui.table.SetCell(i+1, 2, tview.NewTableCell(dir).SetMaxWidth(36).SetExpansion(1))
		ui.table.SetCell(i+1, 3, tview.NewTableCell(branch).SetMaxWidth(18))
		ui.table.SetCell(i+1, 4, tview.NewTableCell(fmt.Sprintf("%d", s.Messages)).SetAlign(tview.AlignRight))
		ui.table.SetCell(i+1, 5, tview.NewTableCell(s.Title).SetExpansion(3))
	}
	if len(rows) == 0 {
		ui.preview.SetText("\n[gray]No sessions match this filter.[-]")
		return
	}
	if sel > len(rows) {
		sel = 1
	}
	ui.table.Select(sel, 0)
	ui.showRow(sel)
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
	default:
		return tcell.ColorWhite
	}
}

func (ui *ui) paintHeader() {
	stats, _ := ui.store.Stats()
	var b strings.Builder
	fmt.Fprintf(&b, "[::b] ai9s [-]  %d sessions   %d messages", stats.Sessions, stats.Messages)
	for _, a := range stats.Agents {
		fmt.Fprintf(&b, "   [%s]%s %d[-]", agentColor(a.Agent), a.Agent, a.Sessions)
	}
	if ui.yolo {
		b.WriteString("   [yellow]yolo[-]")
	}
	if ui.busy {
		b.WriteString("   indexing…")
	}
	if len(ui.warnings) > 0 {
		fmt.Fprintf(&b, "\n[red]%s[-]", ui.warnings[0])
	} else {
		b.WriteString("\n[gray]agent: dir: branch: model: date:<7d sort:recent[-]")
	}
	ui.header.SetText(b.String())
}

func (ui *ui) showRow(row int) {
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
	s, ok := ui.selected()
	if !ok {
		return
	}
	if !s.CanDelete {
		reason := s.DeleteReason
		if reason == "" {
			reason = "Deletion is disabled for " + s.Agent + "."
		}
		ui.alert(reason)
		return
	}
	text := fmt.Sprintf("Delete this %s session?\n\n%s\n%s", s.Agent, s.Title, shortPath(s.SourcePath))
	modal := tview.NewModal().SetText(text).AddButtons([]string{"Delete", "Cancel"}).SetDoneFunc(func(_ int, label string) {
		ui.app.SetRoot(ui.layout, true)
		ui.app.SetFocus(ui.table)
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
		ui.app.SetFocus(ui.table)
	})
	ui.app.SetRoot(modal, false).SetFocus(modal)
}

func (ui *ui) showHelp() {
	text := `enter     resume in the session directory
d, ctrl-d delete, after confirmation
/         edit the filter
a         cycle the agent filter
p         add a dir: filter
o         cycle sort (recent, oldest, messages, title)
y         toggle extra approval flags where the agent has one
r         reindex
s         stats
q         quit

Filter words are matched in the title and transcript.
agent:  dir:  branch:  model:  date:<7d  date:>30d  date:YYYY-MM-DD  sort:recent`
	ui.alert(text)
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
		fmt.Fprintf(&b, "%-10s %5d sessions   %6d messages\n", a.Agent, a.Sessions, a.Messages)
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

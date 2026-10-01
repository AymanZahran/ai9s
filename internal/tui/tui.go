package tui

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/AymanZahran/air9s/internal/act"
	"github.com/AymanZahran/air9s/internal/config"
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
	cfg, err := config.Load()
	if err != nil {
		cfg.Warnings = append(cfg.Warnings, err.Error())
	}
	app := tview.NewApplication()
	ui := newUI(app, st, cfg)
	ui.reload()
	if cfg.Body.NoExitOnCtrlC {
		app.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
			if ev.Key() == tcell.KeyCtrlC {
				return nil
			}
			return ev
		})
	}
	app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
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
	stopRefresh := func() {}
	if cfg.Body.RefreshRate > 0 {
		stopRefresh = ui.startRefresh(time.Duration(cfg.Body.RefreshRate) * time.Second)
	}
	runErr := app.SetRoot(ui.layout, true).EnableMouse(cfg.Mouse()).Run()
	stopRefresh()
	if runErr != nil {
		return nil, runErr
	}
	return ui.pending, nil
}

type ui struct {
	app             *tview.Application
	store           *store.Store
	layout          *tview.Flex
	top             *tview.Flex
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
	menuMeasured    int
	view            string
	pages           *tview.Pages
	body            *tview.Pages
	command         *tview.InputField
	commandOpen     bool
	commandMoved    bool
	suppressCommand bool
	groups          []groupRow
	suggestions     []commandHint
	cfg             config.Loaded
	crumbs          *tview.TextView
	info            *tview.TextView
	logo            *tview.TextView
	listBar         scrollBar
	previewBar      scrollBar
	listXBar        scrollBar
	previewXBar     scrollBar
	lines           []string
	listWide        int
	listX           int
	listViewW       int
	scrollDrag      string
	scrollGrab      int
}

func newUI(app *tview.Application, st *store.Store, cfg config.Loaded) *ui {
	ui := &ui{app: app, store: st, cfg: cfg, warnings: append([]string(nil), cfg.Warnings...)}
	ui.header = tview.NewTextView().SetDynamicColors(true).SetWrap(false)
	ui.logo = tview.NewTextView().SetDynamicColors(true).SetWrap(false)
	ui.crumbs = tview.NewTextView().SetDynamicColors(true).SetWrap(false)
	ui.info = tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	ui.footer = tview.NewTextView().SetDynamicColors(true).SetWrap(false)

	ui.filter = tview.NewInputField().SetLabel(" / ").SetFieldWidth(0)
	ui.filter.SetChangedFunc(func(string) { ui.reload() })
	ui.filter.SetInputCapture(ui.forwardListMotion)
	ui.filter.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			ui.focusPreview()
		case tcell.KeyEscape:
			ui.clearFilter()
			ui.focusSessions()
		case tcell.KeyEnter, tcell.KeyBacktab:
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
		ui.commandMoved = false
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
		return ui.forwardListMotion(ev)
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

	ui.preview = tview.NewTextView().SetDynamicColors(true).SetWrap(false).SetScrollable(true)
	ui.preview.SetBorder(true).SetTitle(" describe ")
	ui.preview.SetInputCapture(ui.previewKeys)
	ui.preview.SetDoneFunc(ui.previewDone)
	ui.preview.SetFocusFunc(func() {
		ui.focused = "preview"
		ui.paintChrome()
	})

	// The list is the only window. Describe replaces it until Esc.
	ui.body = tview.NewPages().
		AddPage("list", ui.table, true, true).
		AddPage("describe", ui.preview, true, false)
	menuH, crumbsH, infoH, logoW := 2, 1, 2, 10
	if cfg.Body.UI.Headless {
		menuH, crumbsH, infoH, logoW = 0, 0, 0, 0
	}
	if cfg.Body.UI.Logoless {
		logoW = 0
	}
	ui.top = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(ui.header, 0, 1, false).
		AddItem(ui.logo, logoW, 0, false)
	ui.layout = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.top, menuH, 0, false).
		AddItem(ui.crumbs, crumbsH, 0, false).
		AddItem(ui.info, infoH, 0, false).
		AddItem(ui.pages, 1, 0, false).
		AddItem(ui.body, 0, 1, true).
		AddItem(ui.footer, 1, 0, false)
	ui.focused = "table"
	ui.view = viewSessions
	if spec, ok := viewByName(cfg.Body.DefaultView); ok {
		ui.view = spec.name
	}
	ui.installScroll()
	ui.paintChrome()
	return ui
}

func (ui *ui) startRefresh(d time.Duration) func() {
	stop := make(chan struct{})
	go func() {
		t := time.NewTicker(d)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				ui.app.QueueUpdateDraw(func() { ui.reindex() })
			}
		}
	}()
	return func() { close(stop) }
}

func (ui *ui) clearFilter() {
	if ui.filter.GetText() == "" {
		return
	}
	ui.filter.SetText("")
}

const (
	footerSessions = `[yellow]d[-] describe   [yellow]ctrl-d[-] delete   [yellow]h/l[-] pan   [yellow]pgup/pgdn[-] page   [yellow]enter[-] resume   [yellow]/[-] filter   [yellow]a[-] agent   [yellow]p[-] directory   [yellow]o[-] sort   [yellow]y[-] yolo   [yellow]r[-] reindex   [yellow]s[-] stats   [yellow]?[-] help   [yellow]q[-] quit`
	footerPreview  = `[yellow]j/k[-] line   [yellow]h/l[-] pan   [yellow]pgup/pgdn[-] page   [yellow]g/G[-] top/end   [yellow]wheel[-] scroll   [yellow]esc[-] list   [yellow]ctrl-d[-] delete   [yellow]enter[-] resume   [yellow]q[-] quit`
)

func (ui *ui) paintChrome() {
	// Do not call HasFocus here. TextView.Focus holds its lock while this runs.
	ui.applySkin()
	if ui.commandOpen {
		ui.table.SetTitle(" commands ")
	} else {
		ui.table.SetTitle(viewTitle(ui.view))
	}
	ui.preview.SetTitle(" describe ")
	footer := footerSessions
	switch ui.focused {
	case "preview":
		ui.preview.SetBorderColor(paintColor(ui.cfg.Skin.Frame.Border.Focus, "white"))
		ui.preview.SetTitleColor(paintColor(ui.cfg.Skin.Frame.Title.Highlight, "white"))
		ui.preview.SetTitle(" describe · scroll ")
		footer = footerPreview
	case "table", "filter", "command":
		ui.table.SetBorderColor(paintColor(ui.cfg.Skin.Frame.Border.Focus, "white"))
		ui.table.SetTitleColor(paintColor(ui.cfg.Skin.Frame.Title.Highlight, "white"))
	}
	ui.footer.SetText(ui.paintFooter(footer))
	ui.paintHeader()
}

// forwardListMotion lets the list move while / or : still has the cursor.
// Letters, including j and k, and left/right/home/end stay in the field.
func (ui *ui) forwardListMotion(ev *tcell.EventKey) *tcell.EventKey {
	switch ev.Key() {
	case tcell.KeyUp, tcell.KeyDown:
	case tcell.KeyPgUp, tcell.KeyCtrlB:
		if ui.commandOpen {
			ui.commandMoved = true
		}
		ui.moveSelection(-ui.listPage())
		return nil
	case tcell.KeyPgDn, tcell.KeyCtrlF:
		if ui.commandOpen {
			ui.commandMoved = true
		}
		ui.moveSelection(ui.listPage())
		return nil
	default:
		return ev
	}
	if ui.commandOpen {
		ui.commandMoved = true
	}
	if h := ui.table.InputHandler(); h != nil {
		h(ev, func(tview.Primitive) {})
	}
	ui.paintCrumbs()
	return nil
}

func (ui *ui) focusSessions() {
	ui.app.SetFocus(ui.table)
}

func (ui *ui) describing() bool {
	if ui.body == nil {
		return false
	}
	name, _ := ui.body.GetFrontPage()
	return name == "describe"
}

func (ui *ui) focusPreview() {
	ui.openDescribe()
}

// openDescribe replaces the list with the preview for the selected row.
func (ui *ui) openDescribe() {
	if ui.table.GetRowCount() <= 1 {
		return
	}
	row, _ := ui.table.GetSelection()
	if row <= 0 {
		return
	}
	ui.showRow(row)
	ui.preview.ScrollToBeginning()
	ui.body.SwitchToPage("describe")
	ui.app.SetFocus(ui.preview)
}

func (ui *ui) restoreBodyFocus() {
	if ui.describing() {
		ui.app.SetFocus(ui.preview)
		return
	}
	ui.focusSessions()
}

func (ui *ui) closeDescribe() {
	if ui.body != nil {
		ui.body.SwitchToPage("list")
	}
	ui.focusSessions()
}

func (ui *ui) previewDone(key tcell.Key) {
	if key == tcell.KeyEnter {
		ui.closeDescribe()
		ui.resumeSelected()
		return
	}
	ui.closeDescribe()
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
		ui.openDescribe()
		return nil
	case tcell.KeyEscape:
		ui.clearFilter()
		return nil
	case tcell.KeyPgUp:
		ui.moveSelection(-ui.listPage())
		return nil
	case tcell.KeyPgDn:
		ui.moveSelection(ui.listPage())
		return nil
	case tcell.KeyLeft:
		ui.scrollListX(-hScrollStep)
		return nil
	case tcell.KeyRight:
		ui.scrollListX(hScrollStep)
		return nil
	}
	if ev.Key() != tcell.KeyRune {
		if ui.tryPlugin(ev) {
			return nil
		}
		if ev.Key() == tcell.KeyCtrlB {
			ui.moveSelection(-ui.listPage())
			return nil
		}
		if ev.Key() == tcell.KeyCtrlF {
			ui.moveSelection(ui.listPage())
			return nil
		}
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
		ui.openDescribe()
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
	case 'h':
		ui.scrollListX(-hScrollStep)
	case 'l':
		ui.scrollListX(hScrollStep)
	case '1', '2', '3', '4', '5':
		if spec, ok := viewByKey(string(ev.Rune())); ok {
			ui.setView(spec.name)
		}
	default:
		if ui.tryPlugin(ev) {
			return nil
		}
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
	switch ev.Key() {
	case tcell.KeyPgUp:
		ui.scrollPreview(-ui.previewPage())
		return nil
	case tcell.KeyPgDn:
		ui.scrollPreview(ui.previewPage())
		return nil
	case tcell.KeyLeft:
		ui.scrollPreviewX(-hScrollStep)
		return nil
	case tcell.KeyRight:
		ui.scrollPreviewX(hScrollStep)
		return nil
	case tcell.KeyCtrlB, tcell.KeyCtrlF:
		if ui.tryPlugin(ev) {
			return nil
		}
		delta := ui.previewPage()
		if ev.Key() == tcell.KeyCtrlB {
			delta = -delta
		}
		ui.scrollPreview(delta)
		return nil
	}
	if ev.Key() == tcell.KeyRune {
		switch ev.Rune() {
		case 'q':
			ui.app.Stop()
			return nil
		case '/':
			ui.closeDescribe()
			ui.app.SetFocus(ui.filter)
			return nil
		case ':':
			ui.closeDescribe()
			ui.openCommand()
			return nil
		case 'd':
			return nil
		case 'h':
			ui.scrollPreviewX(-hScrollStep)
			return nil
		case 'l':
			ui.scrollPreviewX(hScrollStep)
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
	if ui.tryPlugin(ev) {
		return nil
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
	rows, err := ui.store.Search(f, ui.cfg.Limit())
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
	if ui.view != "" && ui.view != viewSessions {
		ui.paintGroups()
	} else {
		ui.paintSessions()
	}
	ui.paintHeader()
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
	case "antigravity", "agy":
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
	if ui.cfg.Body.UI.Headless {
		return
	}
	text, lines := ui.menuText()
	ui.header.SetText(text)
	if ui.layout != nil && ui.top != nil && lines > 0 {
		ui.layout.ResizeItem(ui.top, lines, 0)
	}
	if ui.cfg.Body.UI.Logoless {
		ui.logo.SetText("")
	} else {
		logo := ui.cfg.Skin.Body.Logo
		ui.logo.SetText(fmt.Sprintf("[%s]╭──╮╭──╮[-]\n[%s]╰──┴┴──╯[-]", logo, logo))
	}
	ui.paintCrumbs()
	ui.paintInfo()
}

func (ui *ui) paintCrumbs() {
	view := ui.view
	if view == "" {
		view = viewSessions
	}
	if ui.commandOpen {
		view = "command"
	}
	label := strings.ToUpper(view[:1]) + view[1:]
	filter := strings.TrimSpace(ui.filter.GetText())
	if filter == "" {
		filter = "all"
	}
	if len(filter) > 42 {
		filter = filter[:41] + "…"
	}
	stats, _ := ui.store.Stats()
	active := ui.cfg.Skin.Frame.Crumbs.Active
	var extra string
	if ui.yolo {
		extra += "  yolo"
	}
	if ui.busy {
		extra += "  indexing…"
	}
	ui.crumbs.SetText(fmt.Sprintf(" air9s › [%s::b]%s[-] › %s    %d sessions · %d messages%s    %s",
		active, label, filter, stats.Sessions, stats.Messages, extra, ui.counter()))
	footer := footerSessions
	if ui.focused == "preview" {
		footer = footerPreview
	}
	ui.footer.SetText(ui.paintFooter(footer))
}

func (ui *ui) paintFooter(base string) string {
	key := strings.TrimSpace(ui.cfg.Skin.Frame.Menu.Key)
	if key == "" {
		key = "white"
	}
	return strings.ReplaceAll(base, "[yellow]", "["+key+"]") + "   " + ui.counter()
}

func (ui *ui) paintInfo() {
	stats, _ := ui.store.Stats()
	var b strings.Builder
	if len(ui.warnings) > 0 {
		fmt.Fprintf(&b, "[red]%s[-]\n", ui.warnings[0])
	}
	if len(stats.Agents) == 0 {
		b.WriteString(" ")
	}
	for i, a := range stats.Agents {
		if i > 0 {
			b.WriteString("   ")
		}
		fmt.Fprintf(&b, "%s [%s]%s %d[-]", ui.mark(a.Agent), ui.agentTag(a.Agent), a.Agent, a.Sessions)
	}
	if len(ui.warnings) == 0 {
		b.WriteString("\n[gray]agent: dir: branch: model: date:<7d sort:recent[-]")
	}
	ui.info.SetText(b.String())
}

func (ui *ui) counter() string {
	total := len(ui.rows)
	if ui.commandOpen {
		total = len(ui.suggestions)
	} else if ui.view != "" && ui.view != viewSessions {
		total = len(ui.groups)
	}
	row, _ := ui.table.GetSelection()
	if row < 1 || total == 0 {
		return fmt.Sprintf("0/%d", total)
	}
	if row > total {
		row = total
	}
	color := ui.cfg.Skin.Frame.Title.Counter
	if color == "" {
		color = "papayawhip"
	}
	return fmt.Sprintf("[%s]%d/%d[-]", color, row, total)
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
	ui.preview.SetText(ui.previewBody(full))
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
	var runErr error
	if ui.app.Suspend(func() {
		runErr = cmd.Run()
	}) {
		if runErr != nil {
			var exitErr *exec.ExitError
			if !errors.As(runErr, &exitErr) {
				ui.alert(runErr.Error())
			}
		}
		ui.reindex()
		return
	}
	// The screen is not up, so the caller runs the command after the TUI stops.
	ui.pending = &cmd
	ui.app.Stop()
}

func (ui *ui) confirmDelete() {
	if ui.view != "" && ui.view != viewSessions {
		ui.alert("Switch to sessions before deleting.")
		return
	}
	if ui.cfg.Body.ReadOnly {
		ui.alert("Delete is off. readOnly is set in the config.")
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
	modal := ui.modal(text, []string{"Delete", "Cancel"}, func(_ int, label string) {
		ui.app.SetRoot(ui.layout, true)
		if label != "Delete" {
			ui.restoreBodyFocus()
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
		if ui.describing() {
			ui.closeDescribe()
		}
		ui.reload()
	})
	ui.app.SetRoot(modal, true)
}

func (ui *ui) alert(msg string) {
	modal := ui.modal(msg, []string{"OK"}, func(int, string) {
		ui.app.SetRoot(ui.layout, true)
		ui.restoreBodyFocus()
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
		fmt.Fprintf(&b, "%s %-10s %5d sessions   %6d messages\n", ui.mark(a.Agent), a.Agent, a.Sessions, a.Messages)
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

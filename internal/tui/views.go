package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/AymanZahran/air9s/internal/query"
	"github.com/rivo/tview"
)

const (
	viewSessions    = "sessions"
	viewProviders   = "providers"
	viewDirectories = "directories"
	viewBranches    = "branches"
	viewModels      = "models"
)

type viewSpec struct {
	key   string
	name  string
	title string
	token string
}

var viewSpecs = []viewSpec{
	{"1", viewSessions, " sessions ", ""},
	{"2", viewProviders, " providers ", "agent"},
	{"3", viewDirectories, " directories ", "dir"},
	{"4", viewBranches, " branches ", "branch"},
	{"5", viewModels, " models ", "model"},
}

type groupRow struct {
	key      string
	sessions int
	messages int
	updated  time.Time
	sampleID string
}

type commandHint struct {
	insert string
	hint   string
}

func viewByKey(key string) (viewSpec, bool) {
	for _, spec := range viewSpecs {
		if spec.key == key {
			return spec, true
		}
	}
	return viewSpec{}, false
}

func viewByName(name string) (viewSpec, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, spec := range viewSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return viewSpec{}, false
}

func viewTitle(name string) string {
	if spec, ok := viewByName(name); ok {
		return spec.title
	}
	return " sessions "
}

func (ui *ui) setView(name string) {
	if _, ok := viewByName(name); !ok {
		name = viewSessions
	}
	ui.view = name
	ui.reload()
}

func (ui *ui) cycleView() {
	next := viewProviders
	for i, spec := range viewSpecs {
		if spec.name == ui.view {
			next = viewSpecs[(i+1)%len(viewSpecs)].name
			break
		}
	}
	ui.view = next
}

func (ui *ui) openCommand() {
	ui.commandOpen = true
	ui.commandMoved = false
	ui.suppressCommand = true
	ui.command.SetText("")
	ui.suppressCommand = false
	ui.pages.SwitchToPage("command")
	ui.app.SetFocus(ui.command)
	ui.paintSuggestions("")
}

func (ui *ui) dismissCommand() {
	ui.commandOpen = false
	ui.suppressCommand = true
	ui.command.SetText("")
	ui.suppressCommand = false
	ui.pages.SwitchToPage("filter")
}

func (ui *ui) closeCommand() {
	ui.dismissCommand()
	ui.focusSessions()
	ui.reload()
}

func (ui *ui) cycleCommandView() {
	names := make([]string, len(viewSpecs))
	for i, spec := range viewSpecs {
		names[i] = spec.name
	}
	cur := strings.TrimSpace(ui.command.GetText())
	next := names[0]
	for i, name := range names {
		if cur == name {
			next = names[(i+1)%len(names)]
			break
		}
	}
	ui.command.SetText(next)
}

func (ui *ui) applyCommand(text string) {
	text = strings.TrimSpace(text)
	if ui.commandMoved {
		ui.commandMoved = false
		if row, _ := ui.table.GetSelection(); row > 0 && row-1 < len(ui.suggestions) {
			hint := ui.suggestions[row-1].insert
			if hint != text {
				ui.applyCommand(hint)
				return
			}
		}
	}
	if text == "" {
		ui.cycleView()
		ui.closeCommand()
		return
	}
	if spec, ok := viewByName(text); ok {
		ui.view = spec.name
		ui.closeCommand()
		return
	}
	if key, val, ok := filterToken(text); ok {
		ui.view = viewSessions
		ui.dismissCommand()
		if val == "" {
			ui.promptFilterToken(key)
			return
		}
		ui.filter.SetText(setToken(ui.filter.GetText(), key, quoteTok(val)))
		ui.focusSessions()
		ui.reload()
		return
	}
	matches := filterHints(text, ui.agents)
	if len(matches) == 1 && matches[0].insert != text {
		ui.applyCommand(matches[0].insert)
		return
	}
	if row, _ := ui.table.GetSelection(); row > 0 && row-1 < len(ui.suggestions) {
		hint := ui.suggestions[row-1].insert
		if hint != text && strings.HasPrefix(strings.ToLower(hint), strings.ToLower(text)) {
			ui.applyCommand(hint)
			return
		}
	}
	ui.dismissCommand()
	ui.focusSessions()
	ui.reload()
	ui.alert("Unknown command: " + text)
}

func filterToken(text string) (key, val string, ok bool) {
	key, val, found := strings.Cut(text, ":")
	if !found {
		return "", "", false
	}
	switch strings.ToLower(key) {
	case "agent", "a":
		return "agent", val, true
	case "dir", "directory", "cwd", "d":
		return "dir", val, true
	case "branch", "b":
		return "branch", val, true
	case "model", "m":
		return "model", val, true
	case "date":
		return "date", val, true
	case "sort":
		return "sort", val, true
	default:
		return "", "", false
	}
}

func (ui *ui) promptFilterToken(key string) {
	text := ui.filter.GetText()
	if !strings.Contains(strings.ToLower(text), strings.ToLower(key)+":") {
		if text != "" && !strings.HasSuffix(text, " ") {
			text += " "
		}
		text += key + ":"
		ui.filter.SetText(text)
	}
	ui.app.SetFocus(ui.filter)
}

func allHints(agents []string) []commandHint {
	hints := []commandHint{
		{"sessions", "show the session list"},
		{"providers", "group the current filter by agent"},
		{"directories", "group the current filter by directory"},
		{"branches", "group the current filter by git branch"},
		{"models", "group the current filter by model"},
		{"agent:", "filter by agent, then return to sessions"},
		{"dir:", "filter by directory"},
		{"branch:", "filter by branch"},
		{"model:", "filter by model"},
		{"date:", "filter by date, for example date:<7d"},
		{"sort:", "recent, oldest, messages, or title"},
	}
	for _, agent := range agents {
		hints = append(hints, commandHint{insert: "agent:" + agent, hint: "show " + agent + " sessions"})
	}
	return hints
}

func filterHints(text string, agents []string) []commandHint {
	all := allHints(agents)
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return all
	}
	var out []commandHint
	for _, hint := range all {
		if strings.Contains(strings.ToLower(hint.insert), text) || strings.Contains(strings.ToLower(hint.hint), text) {
			out = append(out, hint)
		}
	}
	return out
}

func (ui *ui) paintSuggestions(text string) {
	ui.suggestions = filterHints(text, ui.agents)
	ui.table.Clear()
	ui.table.SetTitle(" commands ")
	ui.table.SetCell(0, 0, ui.headerCell("COMMAND", 0))
	ui.table.SetCell(0, 1, ui.headerCell("DETAIL", 1))
	for i, hint := range ui.suggestions {
		ui.table.SetCell(i+1, 0, ui.cell(hint.insert))
		ui.table.SetCell(i+1, 1, ui.cell(hint.hint).SetExpansion(1))
	}
	if len(ui.suggestions) == 0 {
		ui.preview.SetText("\n[gray]No command matches.[-]")
		return
	}
	ui.table.Select(1, 0)
}

func (ui *ui) paintSessions() {
	prev := ""
	if row, _ := ui.table.GetSelection(); row > 0 && row-1 < len(ui.rows) && ui.view == viewSessions {
		prev = ui.rows[row-1].ID
	}
	ui.groups = nil
	ui.table.Clear()
	ui.table.SetTitle(viewTitle(viewSessions))
	sortName := querySort(ui.filter.GetText())
	headers := []string{sorted("AGE", sortName, "recent", "oldest"), "", "AGENT", "DIR", "BRANCH", "CTX", sorted("MSGS", sortName, "messages", ""), sorted("TITLE", sortName, "", "title")}
	exp := []int{0, 0, 0, 1, 0, 0, 0, 3}
	for i, h := range headers {
		ui.table.SetCell(0, i, ui.headerCell(h, exp[i]))
	}
	sel := 1
	for i, s := range ui.rows {
		if s.ID == prev {
			sel = i + 1
		}
		branch := s.Branch
		if branch == "" {
			branch = "-"
		}
		ui.table.SetCell(i+1, 0, ui.cell(relAge(s.Updated)))
		ui.table.SetCell(i+1, 1, ui.cell(ui.mark(s.Agent)).SetAlign(tview.AlignCenter))
		ui.table.SetCell(i+1, 2, ui.cell(s.Agent).SetTextColor(ui.colorOfAgent(s.Agent)))
		ui.table.SetCell(i+1, 3, ui.cell(shortPath(s.CWD)).SetMaxWidth(36).SetExpansion(1))
		ui.table.SetCell(i+1, 4, ui.cell(branch).SetMaxWidth(18))
		ui.table.SetCell(i+1, 5, ui.cell(contextLabel(s.Usage)).SetAlign(tview.AlignRight))
		ui.table.SetCell(i+1, 6, ui.cell(fmt.Sprintf("%d", s.Messages)).SetAlign(tview.AlignRight))
		ui.table.SetCell(i+1, 7, ui.cell(s.Title).SetExpansion(3))
	}
	if len(ui.rows) == 0 {
		ui.preview.SetText("\n[gray]No sessions match this filter.[-]")
		return
	}
	if sel > len(ui.rows) {
		sel = 1
	}
	ui.table.Select(sel, 0)
	ui.showRow(sel)
}

func groupSessions(rows []sessionSnap, kind string) []groupRow {
	order := []string{}
	by := map[string]*groupRow{}
	for _, s := range rows {
		key := groupKey(s, kind)
		g, ok := by[key]
		if !ok {
			g = &groupRow{key: key}
			by[key] = g
			order = append(order, key)
		}
		g.sessions++
		g.messages += s.messages
		if s.updated.After(g.updated) {
			g.updated = s.updated
			g.sampleID = s.id
		}
	}
	out := make([]groupRow, 0, len(order))
	for _, key := range order {
		out = append(out, *by[key])
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].sessions != out[j].sessions {
			return out[i].sessions > out[j].sessions
		}
		return out[i].key < out[j].key
	})
	return out
}

type sessionSnap struct {
	id       string
	agent    string
	cwd      string
	branch   string
	model    string
	messages int
	updated  time.Time
}

func groupKey(s sessionSnap, kind string) string {
	var key string
	switch kind {
	case viewProviders:
		key = s.agent
	case viewDirectories:
		key = s.cwd
	case viewBranches:
		key = s.branch
	case viewModels:
		key = s.model
	}
	if strings.TrimSpace(key) == "" {
		return "(none)"
	}
	return key
}

func (ui *ui) paintGroups() {
	prev := ""
	if row, _ := ui.table.GetSelection(); row > 0 && row-1 < len(ui.groups) {
		prev = ui.groups[row-1].key
	}
	snaps := make([]sessionSnap, len(ui.rows))
	for i, s := range ui.rows {
		snaps[i] = sessionSnap{id: s.ID, agent: s.Agent, cwd: s.CWD, branch: s.Branch, model: s.Model, messages: s.Messages, updated: s.Updated}
	}
	ui.groups = groupSessions(snaps, ui.view)
	ui.table.Clear()
	ui.table.SetTitle(viewTitle(ui.view))
	headers := []string{"", "NAME", "SESSIONS", "MSGS", "LATEST"}
	for i, h := range headers {
		exp := 0
		if i == 1 {
			exp = 1
		}
		ui.table.SetCell(0, i, ui.headerCell(h, exp))
	}
	sel := 1
	for i, g := range ui.groups {
		if g.key == prev {
			sel = i + 1
		}
		mark := "  "
		name := g.key
		color := paintColor(ui.cfg.Skin.Views.Table.Fg, "blue")
		if ui.view == viewProviders {
			mark = ui.mark(g.key)
			color = ui.colorOfAgent(g.key)
		}
		if ui.view == viewDirectories && g.key != "(none)" {
			name = shortPath(g.key)
		}
		ui.table.SetCell(i+1, 0, ui.cell(mark).SetAlign(tview.AlignCenter))
		ui.table.SetCell(i+1, 1, ui.cell(name).SetTextColor(color).SetExpansion(1))
		ui.table.SetCell(i+1, 2, ui.cell(fmt.Sprintf("%d", g.sessions)).SetAlign(tview.AlignRight))
		ui.table.SetCell(i+1, 3, ui.cell(fmt.Sprintf("%d", g.messages)).SetAlign(tview.AlignRight))
		ui.table.SetCell(i+1, 4, ui.cell(relAge(g.updated)))
	}
	if len(ui.groups) == 0 {
		ui.preview.SetText("\n[gray]No sessions match this filter.[-]")
		return
	}
	if sel > len(ui.groups) {
		sel = 1
	}
	ui.table.Select(sel, 0)
	ui.showRow(sel)
}

func (ui *ui) showGroup(row int) {
	if row <= 0 || row-1 >= len(ui.groups) {
		return
	}
	g := ui.groups[row-1]
	var b strings.Builder
	fmt.Fprintf(&b, "[::b]%s[-]\n%d sessions   %d messages   %s\n", g.key, g.sessions, g.messages, relAge(g.updated))
	if g.key == "(none)" {
		b.WriteString("\n[gray]This group has an empty value, so enter will not add a filter.[-]\n")
	} else if spec, ok := viewByName(ui.view); ok && spec.token != "" {
		fmt.Fprintf(&b, "\nenter applies [yellow]%s:%s[-] and returns to sessions\n", spec.token, g.key)
	}
	if g.sampleID != "" {
		if full, err := ui.store.Get(g.sampleID); err == nil {
			b.WriteString("\n")
			b.WriteString(ui.previewBody(full))
		}
	}
	ui.preview.SetText(b.String())
	ui.preview.ScrollToBeginning()
}

func (ui *ui) activateGroup() {
	row, _ := ui.table.GetSelection()
	if row <= 0 || row-1 >= len(ui.groups) {
		return
	}
	g := ui.groups[row-1]
	spec, ok := viewByName(ui.view)
	if !ok || spec.token == "" {
		return
	}
	if g.key == "(none)" {
		ui.alert("That group has an empty value, so there is no filter to apply.")
		return
	}
	ui.view = viewSessions
	ui.filter.SetText(setToken(ui.filter.GetText(), spec.token, quoteTok(g.key)))
	ui.focusSessions()
	ui.reload()
}

func querySort(raw string) string {
	return query.Parse(raw).Sort
}

func sorted(label, current, down, up string) string {
	switch {
	case current == down || (current == "" && down == "recent" && label == "AGE"):
		return label + "↓"
	case up != "" && current == up:
		return label + "↑"
	default:
		return label
	}
}

func menuItem(keyColor, textColor, hi, key, label string, active bool) string {
	if keyColor == "" {
		keyColor = "dodgerblue"
	}
	if textColor == "" {
		textColor = "white"
	}
	if active && hi != "" {
		textColor = hi
	}
	spec := textColor
	if active {
		spec += "::b"
	} else {
		spec += "::d"
	}
	return fmt.Sprintf("[%s]<%s>[-] [%s]%s[-]", keyColor, key, spec, label)
}

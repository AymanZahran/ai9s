package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/AymanZahran/ai9s/internal/query"
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
	worktree string
	sessions int
	messages int
	updated  time.Time
	sampleID string
}

func (g groupRow) selectKey() string {
	return g.key + "\x00" + g.worktree
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
	if ui.describing() {
		ui.closeDescribe()
	}
	ui.drilled = ""
	ui.listX = 0
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
	ui.drilled = ""
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
		ui.drilled = ""
		ui.view = spec.name
		ui.closeCommand()
		return
	}
	if key, val, ok := filterToken(text); ok {
		ui.drilled = ""
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
		{"branches", "group the current filter by git branch and worktree"},
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
	ui.table.SetTitle(" commands ")
	if len(ui.suggestions) == 0 {
		ui.setLines(nil, 0)
		ui.preview.SetText("\n[gray]No command matches.[-]")
		return
	}
	rows := make([][]cellText, 0, len(ui.suggestions)+1)
	rows = append(rows, []cellText{{text: "COMMAND", max: colCommand}, {text: "DETAIL", max: colDetail}})
	for _, hint := range ui.suggestions {
		rows = append(rows, []cellText{{text: hint.insert, max: colCommand}, {text: hint.hint, max: colDetail}})
	}
	ui.useRows(rows)
	ui.table.Select(1, 0)
}

func (ui *ui) emptyList() string {
	if strings.TrimSpace(ui.filter.GetText()) == "" && ui.scope != "" {
		return "\n[gray]No sessions in this directory.[-]"
	}
	return "\n[gray]No sessions match this filter.[-]"
}

func (ui *ui) paintSessions() {
	prev := ""
	if row, _ := ui.table.GetSelection(); row > 0 && row-1 < len(ui.rows) && ui.view == viewSessions {
		prev = ui.rows[row-1].ID
	}
	ui.groups = nil
	ui.table.SetTitle(viewTitle(viewSessions))
	if len(ui.rows) == 0 {
		ui.setLines(nil, 0)
		ui.preview.SetText(ui.emptyList())
		return
	}
	sortName := querySort(ui.filter.GetText())
	rows := make([][]cellText, 0, len(ui.rows)+1)
	rows = append(rows, []cellText{
		{text: sorted("AGE", sortName, "recent", "oldest"), max: colAge},
		{text: "DATE", max: colDate},
		{text: "AGENT", max: colAgent},
		{text: "DIR", max: colDir},
		{text: "BRANCH", max: colBranch},
		{text: "CTX", right: true, max: colCtx},
		{text: "TOKENS", right: true, max: colTokens},
		{text: sorted("MSGS", sortName, "messages", ""), right: true, max: colMsgs},
		{text: sorted("NAME", sortName, "", "title")},
	})
	sel := 1
	for i, s := range ui.rows {
		if s.ID == prev {
			sel = i + 1
		}
		branch := s.Branch
		if branch == "" {
			branch = "-"
		}
		rows = append(rows, []cellText{
			{text: relAge(s.Updated), max: colAge},
			{text: absDate(s.Updated), max: colDate},
			{text: s.Agent, color: ui.agentTag(s.Agent), max: colAgent},
			{text: shortPath(s.CWD), max: colDir, tail: true},
			{text: branch, max: colBranch},
			{text: contextLabel(s.Usage), right: true, max: colCtx},
			{text: tokenLabel(s.Usage), right: true, max: colTokens},
			{text: fmt.Sprintf("%d", s.Messages), right: true, max: colMsgs},
			{text: sessionName(s)},
		})
	}
	ui.useRows(rows)
	if sel > len(ui.rows) {
		sel = 1
	}
	ui.table.Select(sel, 0)
	ui.showRow(sel)
}

func groupSessions(rows []sessionSnap, kind string) []groupRow {
	order := []string{}
	by := map[string]*groupRow{}
	seen := map[string]string{}
	for _, s := range rows {
		key := groupKey(s, kind)
		worktree := ""
		id := key
		if kind == viewBranches {
			worktree = cachedWorktree(seen, s.cwd)
			id = key + "\x00" + worktree
		}
		g, ok := by[id]
		if !ok {
			g = &groupRow{key: key, worktree: worktree}
			by[id] = g
			order = append(order, id)
		}
		g.sessions++
		g.messages += s.messages
		if s.updated.After(g.updated) {
			g.updated = s.updated
			g.sampleID = s.id
		}
	}
	out := make([]groupRow, 0, len(order))
	for _, id := range order {
		out = append(out, *by[id])
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].sessions != out[j].sessions {
			return out[i].sessions > out[j].sessions
		}
		if out[i].key != out[j].key {
			return out[i].key < out[j].key
		}
		return out[i].worktree < out[j].worktree
	})
	return out
}

func cachedWorktree(seen map[string]string, cwd string) string {
	if wt, ok := seen[cwd]; ok {
		return wt
	}
	wt := worktreeRoot(cwd)
	seen[cwd] = wt
	return wt
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
		prev = ui.groups[row-1].selectKey()
	}
	snaps := make([]sessionSnap, len(ui.rows))
	for i, s := range ui.rows {
		snaps[i] = sessionSnap{id: s.ID, agent: s.Agent, cwd: s.CWD, branch: s.Branch, model: s.Model, messages: s.Messages, updated: s.Updated}
	}
	ui.groups = groupSessions(snaps, ui.view)
	ui.table.SetTitle(viewTitle(ui.view))
	if len(ui.groups) == 0 {
		ui.setLines(nil, 0)
		ui.preview.SetText(ui.emptyList())
		return
	}
	rows := make([][]cellText, 0, len(ui.groups)+1)
	if ui.view == viewBranches {
		rows = append(rows, []cellText{
			{text: "BRANCH", max: colBranch},
			{text: "WORKTREE", max: colDir},
			{text: "SESSIONS", right: true, max: colSessions},
			{text: "MSGS", right: true, max: colMsgs},
			{text: "AGE", max: colAge},
			{text: "DATE", max: colDate},
		})
	} else {
		rows = append(rows, []cellText{
			{text: "NAME", max: colGroup},
			{text: "SESSIONS", right: true, max: colSessions},
			{text: "MSGS", right: true, max: colMsgs},
			{text: "AGE", max: colAge},
			{text: "DATE", max: colDate},
		})
	}
	sel := 1
	for i, g := range ui.groups {
		if g.selectKey() == prev {
			sel = i + 1
		}
		if ui.view == viewBranches {
			wt := "-"
			if g.worktree != "" {
				wt = shortPath(g.worktree)
			}
			rows = append(rows, []cellText{
				{text: g.key, max: colBranch},
				{text: wt, max: colDir, tail: wt != "-"},
				{text: fmt.Sprintf("%d", g.sessions), right: true, max: colSessions},
				{text: fmt.Sprintf("%d", g.messages), right: true, max: colMsgs},
				{text: relAge(g.updated), max: colAge},
				{text: absDate(g.updated), max: colDate},
			})
			continue
		}
		name := g.key
		color := ""
		if ui.view == viewProviders {
			color = ui.agentTag(g.key)
		}
		if ui.view == viewDirectories && g.key != "(none)" {
			name = shortPath(g.key)
		}
		rows = append(rows, []cellText{
			{text: name, color: color, max: colGroup, tail: ui.view == viewDirectories && g.key != "(none)"},
			{text: fmt.Sprintf("%d", g.sessions), right: true, max: colSessions},
			{text: fmt.Sprintf("%d", g.messages), right: true, max: colMsgs},
			{text: relAge(g.updated), max: colAge},
			{text: absDate(g.updated), max: colDate},
		})
	}
	ui.useRows(rows)
	if sel > len(ui.groups) {
		sel = 1
	}
	ui.table.Select(sel, 0)
	ui.showRow(sel)
}

func (ui *ui) useRows(rows [][]cellText) {
	widths := columnWidths(rows)
	lines := make([]string, len(rows))
	for i, row := range rows {
		lines[i] = renderCells(row, widths)
	}
	ui.setLines(lines, rowWidth(widths))
}

func (ui *ui) showGroup(row int) {
	if row <= 0 || row-1 >= len(ui.groups) {
		return
	}
	g := ui.groups[row-1]
	var b strings.Builder
	fmt.Fprintf(&b, "[::b]%s[-]\n", markupLine(g.key))
	if ui.view == viewBranches {
		wt := "-"
		if g.worktree != "" {
			wt = markupLine(shortPath(g.worktree))
		}
		fmt.Fprintf(&b, "worktree  %s\n", wt)
	}
	fmt.Fprintf(&b, "%d sessions   %d messages   %s   %s\n", g.sessions, g.messages, relAge(g.updated), absDate(g.updated))
	if g.key == "(none)" && g.worktree == "" {
		b.WriteString("\n[gray]This group has an empty value, so enter will not add a filter.[-]\n")
	} else if spec, ok := viewByName(ui.view); ok && spec.token != "" {
		fmt.Fprintf(&b, "\nenter applies %s and returns to sessions\n", groupFilterText(spec, g))
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
	if g.key == "(none)" && g.worktree == "" {
		ui.alert("That group has an empty value, so there is no filter to apply.")
		return
	}
	text := ui.filter.GetText()
	if g.key != "(none)" {
		text = setToken(text, spec.token, quoteTok(g.key))
	}
	if spec.name == viewBranches && g.worktree != "" {
		text = setToken(text, "dir", quoteTok(g.worktree))
	}
	ui.drilled = ui.view
	ui.view = viewSessions
	ui.filter.SetText(text)
	ui.focusSessions()
	ui.reload()
}

func groupFilterText(spec viewSpec, g groupRow) string {
	var parts []string
	if g.key != "(none)" && spec.token != "" {
		parts = append(parts, "[yellow]"+spec.token+":"+markupLine(g.key)+"[-]")
	}
	if spec.name == viewBranches && g.worktree != "" {
		parts = append(parts, "[yellow]dir:"+markupLine(shortPath(g.worktree))+"[-]")
	}
	return strings.Join(parts, " and ")
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
	useHi := strings.TrimSpace(hi) != ""
	keyColor = colorTag(keyColor)
	textColor = colorTag(textColor)
	hi = colorTag(hi)
	if active && useHi {
		textColor = hi
	}
	spec := textColor
	if active {
		spec += "::b"
	} else {
		spec += "::d"
	}
	return fmt.Sprintf("[%s]<%s>[-] [%s]%s[-]", keyColor, markupLine(key), spec, markupLine(label))
}

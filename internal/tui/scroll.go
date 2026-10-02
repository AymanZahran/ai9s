package tui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// wheelRows is how many rows or preview lines one mouse-wheel notch moves.
// hScrollStep is how many columns left/right, h/l, and the horizontal wheel move.
const (
	wheelRows   = 3
	hScrollStep = 4
)

// scrollBar is a gutter on a pane. Vertical bars use x as the column and h as
// the track height. Horizontal bars set horizontal, use y as the row, and h as
// the track width. pos is the current position and maxPos is the furthest it
// can go. A maxPos of 0 means the whole pane fits, so the thumb fills the track.
type scrollBar struct {
	x, y, h    int
	pos        int
	maxPos     int
	top        int
	length     int
	horizontal bool
}

func (b scrollBar) hit(x, y int) bool {
	if b.h <= 0 {
		return false
	}
	if b.horizontal {
		return y == b.y && x >= b.x && x < b.x+b.h
	}
	return x == b.x && y >= b.y && y < b.y+b.h
}

// posForThumbTop maps a thumb top, relative to the track, back to a position.
func (b scrollBar) posForThumbTop(top int) int {
	maxTop := b.h - b.length
	if b.maxPos <= 0 || maxTop <= 0 {
		return 0
	}
	if top < 0 {
		top = 0
	}
	if top > maxTop {
		top = maxTop
	}
	return top * b.maxPos / maxTop
}

func layoutBar(x, y, h, pos, maxPos, span, total int) scrollBar {
	b := scrollBar{x: x, y: y, h: h, pos: pos, maxPos: maxPos}
	if h <= 0 {
		return b
	}
	if total < 1 {
		total = 1
	}
	if span < 1 {
		span = 1
	}
	if span > total {
		span = total
	}
	b.length = span * h / total
	if b.length < 1 {
		b.length = 1
	}
	if b.length > h {
		b.length = h
	}
	if maxPos <= 0 {
		b.pos = 0
		b.top = 0
		b.length = h
		return b
	}
	if pos < 0 {
		pos = 0
	}
	if pos > maxPos {
		pos = maxPos
	}
	b.pos = pos
	b.top = pos * (h - b.length) / maxPos
	return b
}

func (ui *ui) installScroll() {
	ui.table.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		return ui.drawScroll(screen, x, y, width, height, "list")
	})
	ui.preview.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		return ui.drawScroll(screen, x, y, width, height, "preview")
	})
	ui.app.SetMouseCapture(ui.onMouse)
}

// drawScroll paints the right gutter, and a bottom gutter when a line is wider
// than the window. The returned rect stays clear of both.
func (ui *ui) drawScroll(screen tcell.Screen, x, y, width, height int, kind string) (int, int, int, int) {
	ix, iy, iw, ih := insetBorder(x, y, width, height)
	if iw < 2 || ih < 1 {
		ui.storeBar(kind, scrollBar{})
		ui.storeBar(kind+"-x", scrollBar{})
		return ix, iy, iw, ih
	}
	contentW := iw - 1
	contentH := ih
	wide := ui.listWide
	paintW := contentW
	if kind == "preview" {
		wide = lineWidth(ui.preview.GetText(true))
	} else if contentW > 0 {
		ui.applyHScroll(contentW)
		wide = ui.listWide
		if ui.listViewW > 0 {
			paintW = ui.listViewW
		}
	}
	showX := wide > paintW && ih >= 2
	if showX {
		contentH = ih - 1
	}
	var bar scrollBar
	switch kind {
	case "preview":
		page := contentH
		if page < 1 {
			page = 1
		}
		total := previewLines(ui.preview.GetText(true))
		maxPos := total - page
		if maxPos < 0 {
			maxPos = 0
		}
		offset, _ := ui.preview.GetScrollOffset()
		bar = layoutBar(ix+iw-1, iy, contentH, offset, maxPos, page, total)
	default:
		total := ui.listCount()
		page := contentH - 1
		if page < 1 {
			page = 1
		}
		maxPos := 0
		if total > page {
			maxPos = total - 1
		}
		bar = layoutBar(ix+iw-1, iy, contentH, ui.listPos(), maxPos, page, total)
	}
	ui.storeBar(kind, bar)
	ui.paintBar(screen, bar, kind)

	var across scrollBar
	if showX {
		max := wide - paintW
		if max < 0 {
			max = 0
		}
		pos := ui.listX
		if kind == "preview" {
			_, pos = ui.preview.GetScrollOffset()
		}
		if pos > max {
			pos = max
		}
		across = layoutBar(ix, iy+ih-1, contentW, pos, max, paintW, wide)
		across.horizontal = true
		ui.paintBar(screen, across, kind+"-x")
		if screen != nil {
			screen.SetContent(ix+iw-1, iy+ih-1, '┘', nil, ui.barStyle(false, kind))
		}
	}
	ui.storeBar(kind+"-x", across)
	return ix, iy, contentW, contentH
}

func (ui *ui) storeBar(kind string, bar scrollBar) {
	switch kind {
	case "preview":
		ui.previewBar = bar
	case "preview-x":
		ui.previewXBar = bar
	case "list-x":
		ui.listXBar = bar
	default:
		ui.listBar = bar
	}
}

func (ui *ui) barFocused(kind string) bool {
	if strings.HasPrefix(kind, "preview") {
		return ui.focused == "preview"
	}
	return ui.focused == "table" || ui.focused == "filter" || ui.focused == "command"
}

func (ui *ui) barStyle(thumb bool, kind string) tcell.Style {
	bg := paintColor(ui.cfg.Skin.Views.Table.Bg, "black")
	fg := paintColor(ui.cfg.Skin.Frame.Border.Fg, "white")
	if thumb && ui.barFocused(kind) {
		fg = paintColor(ui.cfg.Skin.Frame.Border.Focus, "white")
	}
	return tcell.StyleDefault.Foreground(fg).Background(bg)
}

func (ui *ui) paintBar(screen tcell.Screen, bar scrollBar, kind string) {
	if bar.h <= 0 || screen == nil {
		return
	}
	track := ui.barStyle(false, kind)
	thumb := ui.barStyle(true, kind)
	trackRune, thumbRune := '│', '┃'
	if bar.horizontal {
		trackRune, thumbRune = '─', '━'
	}
	for i := 0; i < bar.h; i++ {
		r := trackRune
		style := track
		if i >= bar.top && i < bar.top+bar.length {
			r = thumbRune
			style = thumb
		}
		px, py := bar.x, bar.y+i
		if bar.horizontal {
			px, py = bar.x+i, bar.y
		}
		screen.SetContent(px, py, r, nil, style)
	}
}

func insetBorder(x, y, width, height int) (int, int, int, int) {
	x++
	y++
	width -= 2
	height -= 2
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	return x, y, width, height
}

func (ui *ui) listCount() int {
	n := ui.table.GetRowCount() - 1
	if n < 0 {
		return 0
	}
	return n
}

func (ui *ui) listPos() int {
	row, _ := ui.table.GetSelection()
	if row < 1 {
		return 0
	}
	return row - 1
}

func (ui *ui) listPage() int {
	_, _, _, h := ui.table.GetInnerRect()
	n := h - 1
	if n < 1 {
		return 1
	}
	return n
}

func (ui *ui) previewPage() int {
	_, _, _, h := ui.preview.GetInnerRect()
	if h < 1 {
		return 1
	}
	return h
}

func (ui *ui) moveSelection(delta int) {
	if delta == 0 {
		return
	}
	ui.setListPos(ui.listPos() + delta)
	ui.paintCrumbs()
}

func (ui *ui) setListPos(pos int) {
	n := ui.listCount()
	if n == 0 {
		return
	}
	if pos < 0 {
		pos = 0
	}
	if pos >= n {
		pos = n - 1
	}
	row, col := ui.table.GetSelection()
	if row == pos+1 {
		return
	}
	ui.table.Select(pos+1, col)
}

func (ui *ui) scrollPreview(delta int) {
	if delta == 0 {
		return
	}
	row, _ := ui.preview.GetScrollOffset()
	ui.setPreviewPos(row + delta)
}

func (ui *ui) setPreviewPos(pos int) {
	_, col := ui.preview.GetScrollOffset()
	if pos < 0 {
		pos = 0
	}
	if ui.previewBar.maxPos > 0 && pos > ui.previewBar.maxPos {
		pos = ui.previewBar.maxPos
	}
	ui.preview.ScrollTo(pos, col)
}

func (ui *ui) setLines(lines []string, wide int) {
	ui.lines = lines
	ui.listWide = wide
	w := ui.listViewW
	if w < 1 {
		w = wide
		if w < 1 {
			w = 1
		}
	}
	ui.applyHScroll(w)
}

func (ui *ui) applyHScroll(viewW int) {
	if viewW < 1 {
		viewW = 1
	}
	ui.listViewW = viewW
	max := ui.listWide - viewW
	if max < 0 {
		max = 0
	}
	if ui.listX > max {
		ui.listX = max
	}
	if ui.listX < 0 {
		ui.listX = 0
	}
	if len(ui.lines) == 0 {
		ui.table.Clear()
		return
	}
	for i, line := range ui.lines {
		shown := fitTagged(line, ui.listX, viewW)
		w := textWidth(shown)
		if w < 1 {
			w = 1
		}
		cell := ui.cell(shown).SetExpansion(0).SetMaxWidth(w)
		if i == 0 {
			cell = ui.headerCell(shown, 0).SetMaxWidth(w)
		}
		ui.table.SetCell(i, 0, cell)
	}
	for ui.table.GetRowCount() > len(ui.lines) {
		ui.table.RemoveRow(ui.table.GetRowCount() - 1)
	}
}

func (ui *ui) scrollListX(delta int) {
	ui.setListX(ui.listX + delta)
}

func (ui *ui) setListX(pos int) {
	ui.listX = pos
	w := ui.listViewW
	if w < 1 {
		w = 1
	}
	ui.applyHScroll(w)
}

func (ui *ui) scrollPreviewX(delta int) {
	_, col := ui.preview.GetScrollOffset()
	ui.setPreviewCol(col + delta)
}

func (ui *ui) setPreviewCol(col int) {
	row, _ := ui.preview.GetScrollOffset()
	if col < 0 {
		col = 0
	}
	if max := ui.previewMaxCol(); col > max {
		col = max
	}
	ui.preview.ScrollTo(row, col)
}

// previewMaxCol is how far describe can pan. The bar is zero until the first
// draw, so the limit comes from the text width when that bar is not ready.
func (ui *ui) previewMaxCol() int {
	wide := lineWidth(ui.preview.GetText(true))
	view := ui.previewXBar.h
	if view < 1 {
		_, _, view, _ = ui.preview.GetInnerRect()
	}
	if view < 1 {
		return wide
	}
	if wide <= view {
		return 0
	}
	return wide - view
}

func (ui *ui) onMouse(ev *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
	if ev == nil {
		return nil, action
	}
	if ui.manual != nil {
		switch action {
		case tview.MouseScrollUp, tview.MouseScrollDown:
			step := wheelRows
			if action == tview.MouseScrollUp {
				step = -step
			}
			ui.scrollManual(step)
			return nil, action
		}
		return ev, action
	}
	x, y := ev.Position()
	// Only the visible window owns the pointer. The hidden one keeps its old rect.
	describe := ui.describing()
	switch action {
	case tview.MouseLeftUp, tview.MouseLeftClick:
		onBar := ui.listBar.hit(x, y) || ui.listXBar.hit(x, y)
		if describe {
			onBar = ui.previewBar.hit(x, y) || ui.previewXBar.hit(x, y)
		}
		if ui.scrollDrag != "" || onBar {
			ui.scrollDrag = ""
			return nil, action
		}
	case tview.MouseMove:
		if ui.scrollDrag != "" {
			if strings.HasSuffix(ui.scrollDrag, "-x") {
				ui.dragToX(ui.scrollDrag, x)
			} else {
				ui.dragTo(ui.scrollDrag, y)
			}
			return nil, action
		}
	case tview.MouseLeftDown:
		if !describe && ui.listBar.hit(x, y) {
			ui.beginDrag("list", ui.listBar, y)
			ui.focusSessions()
			return nil, action
		}
		if !describe && ui.listXBar.hit(x, y) {
			ui.beginDrag("list-x", ui.listXBar, x)
			ui.focusSessions()
			return nil, action
		}
		if describe && ui.previewBar.hit(x, y) {
			ui.beginDrag("preview", ui.previewBar, y)
			ui.app.SetFocus(ui.preview)
			return nil, action
		}
		if describe && ui.previewXBar.hit(x, y) {
			ui.beginDrag("preview-x", ui.previewXBar, x)
			ui.app.SetFocus(ui.preview)
			return nil, action
		}
	case tview.MouseScrollUp, tview.MouseScrollDown, tview.MouseScrollLeft, tview.MouseScrollRight:
		if !ui.wheelHits(x, y, describe) {
			return ev, action
		}
		// A horizontal wheel pans. Shift with the vertical wheel is the same
		// gesture on terminals that do not send a separate horizontal wheel.
		horizontal := action == tview.MouseScrollLeft || action == tview.MouseScrollRight
		if !horizontal && ev.Modifiers()&tcell.ModShift != 0 {
			horizontal = true
			if action == tview.MouseScrollUp {
				action = tview.MouseScrollLeft
			} else {
				action = tview.MouseScrollRight
			}
		}
		if horizontal {
			step := hScrollStep
			if action == tview.MouseScrollLeft {
				step = -step
			}
			if describe {
				ui.scrollPreviewX(step)
			} else {
				ui.scrollListX(step)
			}
			return nil, action
		}
		step := wheelRows
		if action == tview.MouseScrollUp {
			step = -step
		}
		if describe {
			ui.scrollPreview(step)
		} else {
			ui.moveSelection(step)
		}
		return nil, action
	}
	return ev, action
}

// wheelHits is true when the pointer is over the pane that should scroll.
// The layout counts as well: a wheel on the menu or the crumbs still
// moves the list, which is what a full-screen pager does.
func (ui *ui) wheelHits(x, y int, describe bool) bool {
	if describe && ui.preview != nil && ui.preview.InRect(x, y) {
		return true
	}
	if !describe && ui.table != nil && ui.table.InRect(x, y) {
		return true
	}
	if ui.body != nil && ui.body.InRect(x, y) {
		return true
	}
	return ui.layout != nil && ui.layout.InRect(x, y)
}

func (ui *ui) beginDrag(which string, bar scrollBar, at int) {
	ui.scrollDrag = which
	origin := bar.y
	if bar.horizontal {
		origin = bar.x
	}
	grab := at - (origin + bar.top)
	if grab < 0 || grab >= bar.length {
		grab = bar.length / 2
	}
	ui.scrollGrab = grab
	if bar.horizontal {
		ui.dragToX(which, at)
		return
	}
	ui.dragTo(which, at)
}

func (ui *ui) dragTo(which string, y int) {
	bar := ui.listBar
	if which == "preview" {
		bar = ui.previewBar
	}
	if bar.maxPos <= 0 || bar.h <= 0 {
		return
	}
	pos := bar.posForThumbTop((y - bar.y) - ui.scrollGrab)
	if which == "preview" {
		ui.setPreviewPos(pos)
		return
	}
	ui.setListPos(pos)
	ui.paintCrumbs()
}

func (ui *ui) dragToX(which string, x int) {
	bar := ui.listXBar
	if which == "preview-x" {
		bar = ui.previewXBar
	}
	if bar.maxPos <= 0 || bar.h <= 0 {
		return
	}
	pos := bar.posForThumbTop((x - bar.x) - ui.scrollGrab)
	if which == "preview-x" {
		ui.setPreviewCol(pos)
		return
	}
	ui.setListX(pos)
}

func lineWidth(text string) int {
	max := 0
	for _, line := range strings.Split(text, "\n") {
		if w := textWidth(line); w > max {
			max = w
		}
	}
	return max
}

func previewLines(text string) int {
	if text == "" {
		return 0
	}
	return strings.Count(text, "\n") + 1
}

// wrappedRows estimates how many screen rows text would occupy if it wrapped.
func wrappedRows(text string, width int) int {
	if text == "" {
		return 0
	}
	if width < 1 {
		width = 1
	}
	rows := 0
	for _, line := range strings.Split(text, "\n") {
		rows += oneLineRows(line, width)
	}
	return rows
}

func oneLineRows(line string, width int) int {
	if line == "" || width < 1 {
		return 1
	}
	rows := 1
	used := 0
	word := ""
	flush := func(s string) {
		w := textWidth(s)
		if w == 0 {
			return
		}
		if used > 0 && used+w > width {
			rows++
			used = 0
		}
		for w > width {
			rows++
			w -= width
			used = 0
		}
		used += w
	}
	for _, r := range line {
		if r == ' ' || r == '\t' {
			if word != "" {
				flush(word)
				word = ""
			}
			if used > 0 {
				used++
				if used >= width {
					rows++
					used = 0
				}
			}
			continue
		}
		word += string(r)
	}
	flush(word)
	return rows
}

package tui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/rivo/tview"
)

// wheelRows is how many rows or preview lines one mouse-wheel notch moves.
const wheelRows = 3

// scrollBar is the one-column gutter drawn on the right of a pane.
// pos is the current position and maxPos is the furthest it can go.
// A maxPos of 0 means the whole pane fits, so the thumb fills the track.
type scrollBar struct {
	x, y, h int
	pos     int
	maxPos  int
	top     int
	length  int
}

func (b scrollBar) hit(x, y int) bool {
	return b.h > 0 && x == b.x && y >= b.y && y < b.y+b.h
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

// drawScroll paints the gutter and returns an inner rect one column narrower
// so the table or preview text stays clear of it.
func (ui *ui) drawScroll(screen tcell.Screen, x, y, width, height int, kind string) (int, int, int, int) {
	ix, iy, iw, ih := insetBorder(x, y, width, height)
	if iw < 2 || ih < 1 {
		ui.storeBar(kind, scrollBar{})
		return ix, iy, iw, ih
	}
	var bar scrollBar
	switch kind {
	case "preview":
		page := ih
		total := wrappedRows(ui.preview.GetText(true), iw-1)
		maxPos := total - page
		if maxPos < 0 {
			maxPos = 0
		}
		offset, _ := ui.preview.GetScrollOffset()
		bar = layoutBar(ix+iw-1, iy, ih, offset, maxPos, page, total)
	default:
		total := ui.listCount()
		page := ih - 1
		if page < 1 {
			page = 1
		}
		maxPos := 0
		if total > page {
			maxPos = total - 1
		}
		bar = layoutBar(ix+iw-1, iy, ih, ui.listPos(), maxPos, page, total)
	}
	ui.storeBar(kind, bar)
	ui.paintBar(screen, bar, kind)
	return ix, iy, iw - 1, ih
}

func (ui *ui) storeBar(kind string, bar scrollBar) {
	if kind == "preview" {
		ui.previewBar = bar
		return
	}
	ui.listBar = bar
}

func (ui *ui) paintBar(screen tcell.Screen, bar scrollBar, kind string) {
	if bar.h <= 0 {
		return
	}
	bg := paintColor(ui.cfg.Skin.Views.Table.Bg, "black")
	track := tcell.StyleDefault.Foreground(paintColor(ui.cfg.Skin.Frame.Border.Fg, "dodgerblue")).Background(bg)
	thumbColor := paintColor(ui.cfg.Skin.Frame.Border.Fg, "dodgerblue")
	focused := (kind == "preview" && ui.focused == "preview") ||
		(kind != "preview" && (ui.focused == "table" || ui.focused == "filter" || ui.focused == "command"))
	if focused {
		thumbColor = paintColor(ui.cfg.Skin.Frame.Border.Focus, "aqua")
	}
	thumb := tcell.StyleDefault.Foreground(thumbColor).Background(bg)
	for i := 0; i < bar.h; i++ {
		r := '│'
		style := track
		if i >= bar.top && i < bar.top+bar.length {
			r = '┃'
			style = thumb
		}
		screen.SetContent(bar.x, bar.y+i, r, nil, style)
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
	if pos <= 0 {
		ui.preview.ScrollToBeginning()
		return
	}
	_, col := ui.preview.GetScrollOffset()
	if ui.previewBar.maxPos > 0 && pos >= ui.previewBar.maxPos {
		ui.preview.ScrollTo(ui.previewBar.maxPos, col)
		return
	}
	ui.preview.ScrollTo(pos, col)
}

func (ui *ui) onMouse(ev *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
	if ev == nil {
		return nil, action
	}
	x, y := ev.Position()
	// Only the visible window owns the pointer. The hidden one keeps its old rect.
	describe := ui.describing()
	switch action {
	case tview.MouseLeftUp, tview.MouseLeftClick:
		onBar := ui.listBar.hit(x, y)
		if describe {
			onBar = ui.previewBar.hit(x, y)
		}
		if ui.scrollDrag != "" || onBar {
			ui.scrollDrag = ""
			return nil, action
		}
	case tview.MouseMove:
		if ui.scrollDrag != "" {
			ui.dragTo(ui.scrollDrag, y)
			return nil, action
		}
	case tview.MouseLeftDown:
		if !describe && ui.listBar.hit(x, y) {
			ui.beginDrag("list", ui.listBar, y)
			ui.focusSessions()
			return nil, action
		}
		if describe && ui.previewBar.hit(x, y) {
			ui.beginDrag("preview", ui.previewBar, y)
			ui.app.SetFocus(ui.preview)
			return nil, action
		}
	case tview.MouseScrollUp, tview.MouseScrollDown:
		step := wheelRows
		if action == tview.MouseScrollUp {
			step = -wheelRows
		}
		if !describe && ui.table.InRect(x, y) {
			ui.moveSelection(step)
			return nil, action
		}
		if describe && ui.preview.InRect(x, y) {
			ui.scrollPreview(step)
			return nil, action
		}
	}
	return ev, action
}

func (ui *ui) beginDrag(which string, bar scrollBar, y int) {
	ui.scrollDrag = which
	grab := y - (bar.y + bar.top)
	if grab < 0 || grab >= bar.length {
		grab = bar.length / 2
	}
	ui.scrollGrab = grab
	ui.dragTo(which, y)
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

// wrappedRows estimates how many screen rows text occupies at width.
// The scrollbar uses it so the thumb tracks wrapped preview lines.
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
		w := runewidth.StringWidth(s)
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

package tui

import (
	"strings"
	"unicode/utf8"

	"github.com/rivo/tview"
)

// textWidth is the column count tview will paint. The list scroller uses the
// same count, so a line tview would ellipsize is a line the scroller can pan.
func textWidth(s string) int {
	return tview.TaggedStringWidth(s)
}

func runeWidth(r rune) int {
	w := textWidth(string(r))
	if w < 1 {
		return 1
	}
	return w
}

// Column caps. The session NAME column has no cap: it stays last and the row
// pans. Group names, paths, and command text do have a cap.
const (
	colAge      = 6
	colDate     = 16
	colAgent    = 12
	colDir      = 32
	colBranch   = 16
	colCtx      = 13
	colTokens   = 13
	colMsgs     = 6
	colSessions = 8
	colGroup    = 28
	colCommand  = 24
	colDetail   = 40
)

// cellText is one column in a row that can be wider than the window.
type cellText struct {
	text  string
	color string
	right bool
	max   int
	tail  bool
}

func padWidth(text string, width int, right bool) string {
	w := textWidth(text)
	if w >= width {
		return text
	}
	gap := strings.Repeat(" ", width-w)
	if right {
		return gap + text
	}
	return text + gap
}

func columnWidths(rows [][]cellText) []int {
	n := 0
	for _, row := range rows {
		if len(row) > n {
			n = len(row)
		}
	}
	widths := make([]int, n)
	for _, row := range rows {
		for i, cell := range row {
			if w := textWidth(cellShown(cell)); w > widths[i] {
				widths[i] = w
			}
		}
	}
	return widths
}

func cellShown(cell cellText) string {
	text := VisibleLine(cell.text)
	if cell.max <= 0 {
		return text
	}
	return clipCell(text, cell.max, cell.tail)
}

// clipCell shortens text to max columns. tail keeps the end of the text,
// which is the useful part of a path.
func clipCell(text string, max int, tail bool) string {
	if max <= 0 || textWidth(text) <= max {
		return text
	}
	const ell = "…"
	ellW := textWidth(ell)
	if ellW >= max {
		return fitTagged(text, 0, max)
	}
	keep := max - ellW
	if tail {
		skip := textWidth(text) - keep
		if skip < 0 {
			skip = 0
		}
		return ell + fitTagged(text, skip, keep)
	}
	return fitTagged(text, 0, keep) + ell
}

func rowWidth(widths []int) int {
	if len(widths) == 0 {
		return 0
	}
	w := len(widths) - 1
	for _, n := range widths {
		w += n
	}
	return w
}

func renderCells(cells []cellText, widths []int) string {
	var b strings.Builder
	for i, cell := range cells {
		if i > 0 {
			b.WriteByte(' ')
		}
		width := 0
		if i < len(widths) {
			width = widths[i]
		}
		text := breakTags(padWidth(cellShown(cell), width, cell.right))
		if cell.color != "" {
			b.WriteByte('[')
			b.WriteString(colorTag(cell.color))
			b.WriteByte(']')
			b.WriteString(text)
			b.WriteString("[-]")
			continue
		}
		b.WriteString(text)
	}
	return b.String()
}

// clipTagged returns the visible slice of s, skipping skip columns and
// keeping at most take columns. Color tags stay in effect across the cut.
func clipTagged(s string, skip, take int) string {
	if take < 1 || s == "" {
		return ""
	}
	var b strings.Builder
	style := ""
	started := false
	visible := 0
	col := 0
	for i := 0; i < len(s); {
		if s[i] == '[' {
			if i+1 < len(s) && s[i+1] == '[' {
				if !writeRune(&b, &style, &started, &visible, &col, skip, take, '[', 1) {
					break
				}
				i += 2
				continue
			}
			end := strings.IndexByte(s[i:], ']')
			if end > 0 {
				tag := s[i : i+end+1]
				style = tag
				if started {
					b.WriteString(tag)
				}
				i += end + 1
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		w := runeWidth(r)
		if !writeRune(&b, &style, &started, &visible, &col, skip, take, r, w) {
			break
		}
		i += size
	}
	return b.String()
}

// fitTagged clips s to take columns and then drops whole runes until tview's
// own width agrees. A leftover column is what makes the table draw an ellipsis
// that never moves.
func fitTagged(s string, skip, take int) string {
	if take < 1 {
		return ""
	}
	shown := clipTagged(s, skip, take)
	for take > 1 && textWidth(shown) > take {
		take--
		shown = clipTagged(s, skip, take)
	}
	return shown
}

func writeRune(b *strings.Builder, style *string, started *bool, visible, col *int, skip, take int, r rune, w int) bool {
	if *col+w <= skip {
		*col += w
		return true
	}
	if !*started {
		*started = true
		if *style != "" {
			b.WriteString(*style)
		}
		if *col < skip {
			*col += w
			return true
		}
	}
	if *visible+w > take {
		return false
	}
	if r == '[' {
		b.WriteString("[[")
	} else {
		b.WriteRune(r)
	}
	*visible += w
	*col += w
	return true
}

package tui

import (
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
	"github.com/rivo/tview"
)

// cellText is one column in a row that can be wider than the window.
type cellText struct {
	text  string
	color string
	right bool
}

func padWidth(text string, width int, right bool) string {
	w := runewidth.StringWidth(text)
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
			if w := runewidth.StringWidth(cell.text); w > widths[i] {
				widths[i] = w
			}
		}
	}
	return widths
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
		text := tview.Escape(padWidth(cell.text, width, cell.right))
		if cell.color != "" {
			b.WriteByte('[')
			b.WriteString(cell.color)
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
		w := runewidth.RuneWidth(r)
		if w < 1 {
			w = 1
		}
		if !writeRune(&b, &style, &started, &visible, &col, skip, take, r, w) {
			break
		}
		i += size
	}
	return b.String()
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

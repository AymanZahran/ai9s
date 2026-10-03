package tui

import (
	"strings"
	"unicode"
)

// Visible drops characters a terminal would treat as commands: C0 and C1
// controls, and Unicode format characters such as bidi overrides. Newlines
// and tabs stay so an excerpt keeps its breaks.
func Visible(s string) string {
	return strings.Map(func(r rune) rune { return dropUnsafe(r, true) }, s)
}

// VisibleLine is Visible with newlines and tabs removed, for one-line fields.
func VisibleLine(s string) string {
	return strings.Map(func(r rune) rune { return dropUnsafe(r, false) }, s)
}

func dropUnsafe(r rune, keepBreak bool) rune {
	if keepBreak && (r == '\n' || r == '\t') {
		return r
	}
	if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
		return -1
	}
	return r
}

// markup is user text safe to place inside a tview dynamic-color string.
// Newlines stay.
func markup(s string) string {
	return breakTags(Visible(s))
}

// markupLine is markup for a single-line field.
func markupLine(s string) string {
	return breakTags(VisibleLine(s))
}

// breakTags keeps tview from reading a style or hyperlink tag in text.
// tview.Escape only rewrites tags whose contents match a small character
// set, so [:::https://evil.example/a] is left intact and becomes a link.
// A zero-width space after '[' is not a tag character, and the bracket
// is drawn as itself.
func breakTags(s string) string {
	if !strings.Contains(s, "[") {
		return s
	}
	return strings.ReplaceAll(s, "[", "[\u200b")
}

// colorTag is a skin or agent color placed inside [color]. Anything that
// could close the tag or start another one becomes white.
func colorTag(c string) string {
	c = strings.TrimSpace(c)
	if c == "" {
		return "white"
	}
	for _, r := range c {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '#' || r == '_' || r == '-':
		default:
			return "white"
		}
	}
	return c
}

package tui

import (
	"strings"
	"testing"
)

func TestClipTagged(t *testing.T) {
	if got := clipTagged("abcdef", 2, 3); got != "cde" {
		t.Fatalf("plain %q", got)
	}
	if got := clipTagged("[red]abcdef[-]", 2, 3); got != "[red]cde" {
		t.Fatalf("color %q", got)
	}
	if got := clipTagged("ab[[cde", 2, 3); got != "[[cd" {
		t.Fatalf("bracket %q", got)
	}
	if got := clipTagged("ok", 10, 4); got != "" {
		t.Fatalf("past end %q", got)
	}
}

func TestColumnCap(t *testing.T) {
	name := clipCell(strings.Repeat("n", 40)+"END", 32, false)
	if textWidth(name) > 32 || !strings.HasSuffix(name, "…") || strings.Contains(name, "END") {
		t.Fatalf("name %q width %d", name, textWidth(name))
	}
	dir := clipCell("/Users/a/very/long/path/to/the/project/ai9s", 32, true)
	if textWidth(dir) > 32 || !strings.HasPrefix(dir, "…") || !strings.HasSuffix(dir, "ai9s") {
		t.Fatalf("dir %q width %d", dir, textWidth(dir))
	}
	if clipCell("short", 10, false) != "short" || clipCell("~/ai9s", 10, true) != "~/ai9s" {
		t.Fatal("short values were cut")
	}
	rows := [][]cellText{
		{{text: "NAME", max: 8}, {text: "DIR", max: 8}},
		{{text: "a very long session name", max: 8}, {text: "/Users/a/proj/ai9s", max: 8, tail: true}},
	}
	widths := columnWidths(rows)
	if widths[0] > 8 || widths[1] > 8 {
		t.Fatalf("widths %v", widths)
	}
	line := renderCells(rows[1], widths)
	if textWidth(line) > 17 || !strings.Contains(line, "ai9s") || strings.Contains(line, "session") {
		t.Fatalf("line %q width %d", line, textWidth(line))
	}
}

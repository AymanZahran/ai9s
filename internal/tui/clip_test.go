package tui

import "testing"

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

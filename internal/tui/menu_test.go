package tui

import (
	"strings"
	"testing"
)

func TestRenderGridAlignsRows(t *testing.T) {
	views := []menuHint{
		{key: "1", label: "sessions", keyColor: "fuchsia", textColor: "white", active: true, hi: "fuchsia"},
		{key: "2", label: "providers", keyColor: "fuchsia", textColor: "white"},
		{key: "3", label: "directories", keyColor: "fuchsia", textColor: "white"},
		{key: "4", label: "branches", keyColor: "fuchsia", textColor: "white"},
		{key: "5", label: "models", keyColor: "fuchsia", textColor: "white"},
	}
	actions := []menuHint{
		{key: "/", label: "filter", keyColor: "dodgerblue", textColor: "white"},
		{key: "enter", label: "resume", keyColor: "dodgerblue", textColor: "white"},
	}
	plugins := []menuHint{
		{key: "e", label: "edit", keyColor: "dodgerblue", textColor: "white"},
		{key: "c", label: "copy", keyColor: "dodgerblue", textColor: "white"},
	}
	width, cols := 80, 5
	text := strings.Join([]string{
		renderGrid(views, width, cols),
		renderGrid(actions, width, cols),
		renderGrid(plugins, width, cols),
	}, "\n")
	lines := strings.Split(text, "\n")
	if len(lines) != 3 {
		t.Fatalf("lines %d\n%s", len(lines), text)
	}
	if !strings.Contains(lines[0], "<1>") || !strings.Contains(lines[0], "<5>") || strings.Contains(lines[0], "<e>") {
		t.Fatalf("views %s", lines[0])
	}
	if strings.Contains(lines[1], "<e>") || !strings.Contains(lines[1], "</>") {
		t.Fatalf("actions %s", lines[1])
	}
	if !strings.Contains(lines[2], "<e>") || !strings.Contains(lines[2], "edit") || strings.Contains(lines[2], "<1>") {
		t.Fatalf("plugins %s", lines[2])
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, " [") {
			t.Fatalf("indent %q", line)
		}
	}
}

func TestRenderGridWrapsExtraPlugins(t *testing.T) {
	var hints []menuHint
	for _, key := range []string{"e", "c", "l", "t", "n", "b"} {
		hints = append(hints, menuHint{key: key, label: "plug", keyColor: "dodgerblue", textColor: "white"})
	}
	text := renderGrid(hints, 80, 5)
	if strings.Count(text, "\n") != 1 {
		t.Fatalf("wrap\n%s", text)
	}
	if !strings.Contains(text, "<b>") {
		t.Fatalf("missing last plugin\n%s", text)
	}
}

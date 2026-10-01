package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/AymanZahran/air9s/internal/model"
	"github.com/mattn/go-runewidth"
	"github.com/rivo/tview"
)

func TestSetTokenAndAge(t *testing.T) {
	got := setToken(`agent:claude "auth bug"`, "agent", "grok")
	if !strings.Contains(got, "agent:grok") || !strings.Contains(got, "auth bug") {
		t.Fatalf("token %q", got)
	}
	if setToken("sort:oldest ship", "sort", "") != "ship" {
		t.Fatalf("clear %+v", setToken("sort:oldest ship", "sort", ""))
	}
	if relAge(time.Time{}) != "-" || absDate(time.Time{}) != "-" {
		t.Fatal(relAge(time.Time{}), absDate(time.Time{}))
	}
	old := time.Now().Add(-40 * 24 * time.Hour)
	if relAge(old) != "5w" {
		t.Fatalf("weeks %q", relAge(old))
	}
	year := time.Now().Add(-400 * 24 * time.Hour)
	if relAge(year) != "1y" {
		t.Fatalf("years %q", relAge(year))
	}
	ahead := time.Now().Add(3*24*time.Hour + 6*time.Hour)
	if relAge(ahead) != "in 3d" {
		t.Fatalf("future %q", relAge(ahead))
	}
	if ageUnit(13*24*time.Hour) != "13d" || ageUnit(14*24*time.Hour) != "2w" || ageUnit(60*24*time.Hour) != "2mo" || ageUnit(365*24*time.Hour) != "1y" {
		t.Fatalf("units %s %s %s %s", ageUnit(13*24*time.Hour), ageUnit(14*24*time.Hour), ageUnit(60*24*time.Hour), ageUnit(365*24*time.Hour))
	}
	stamp := time.Date(2026, 3, 2, 15, 4, 0, 0, time.Local)
	if absDate(stamp) != "2026-03-02 15:04" {
		t.Fatalf("date %q", absDate(stamp))
	}
	text := preview(model.Session{Title: "hi", Agent: "claude", ID: "claude:1", CanDelete: true})
	if !strings.Contains(text, "hi") || strings.Contains(text, "delete:") || !strings.Contains(text, "context    -") || !strings.Contains(text, "tokens     -") || !strings.Contains(text, Icon("claude")) {
		t.Fatalf("preview %s", text)
	}
	var snips []model.Snippet
	for i := 0; i < 12; i++ {
		snips = append(snips, model.Snippet{Role: "user", Body: fmt.Sprintf("turn-%d", i)})
	}
	long := preview(model.Session{Title: "long", Agent: "codex", Messages: 40, Snippets: snips})
	if !strings.Contains(long, "turn-0") || !strings.Contains(long, "turn-11") || !strings.Contains(long, "excerpt") {
		t.Fatalf("preview dropped turns: %s", long)
	}
}

func TestUsageLines(t *testing.T) {
	lines := UsageLines(model.Usage{Context: 39275, Window: 200000, Output: 14, CacheWrite: 1000, CostUSD: 0.12, Effort: "high"})
	text := strings.Join(lines, "\n")
	for _, want := range []string{"context    39.3k / 200k  (19%)", "out 14", "cache write 1k", "cost       $0.12", "effort     high"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	if contextLabel(model.Usage{Context: 39275, Window: 200000}) != "39.3k/200k" {
		t.Fatal(contextLabel(model.Usage{Context: 39275, Window: 200000}))
	}
	if contextLabel(model.Usage{}) != "-" || tokenLabel(model.Usage{}) != "-" {
		t.Fatal("empty labels")
	}
	if tokenLabel(model.Usage{Total: 1500, Context: 20}) != "1.5k" {
		t.Fatalf("total %s", tokenLabel(model.Usage{Total: 1500, Context: 20}))
	}
	if tokenLabel(model.Usage{Input: 1000, Output: 20}) != "1k+20" {
		t.Fatalf("parts %s", tokenLabel(model.Usage{Input: 1000, Output: 20}))
	}
	empty := strings.Join(UsageLines(model.Usage{}), "\n")
	if !strings.Contains(empty, "context    -") || !strings.Contains(empty, "tokens     -") || strings.Contains(empty, "cost") {
		t.Fatalf("empty usage %s", empty)
	}
}

func TestIcons(t *testing.T) {
	want := map[string]string{
		"claude":      "✳ ",
		"codex":       "Cx",
		"copilot":     "Cp",
		"grok":        "Gk",
		"antigravity": "Ag",
		"gemini":      "✦ ",
		"cursor":      "Cu",
		"opencode":    "Oc",
		"hermes":      "☤ ",
		"openclaw":    "🦞",
		"junie":       "Jn",
		"jules":       "Ju",
		"goose":       "🪿",
		"cline":       "Cl",
		"aider":       "Ai",
		"kiro":        "Ki",
	}
	seen := map[string]bool{}
	for _, agent := range []string{"claude", "codex", "copilot", "grok", "antigravity", "gemini", "cursor", "opencode", "hermes", "openclaw", "junie", "jules", "goose", "cline", "aider", "kiro"} {
		icon := Icon(agent)
		if icon != want[agent] || icon == Icon("unknown") || seen[icon] {
			t.Fatalf("%s icon %q", agent, icon)
		}
		if runewidth.StringWidth(icon) != 2 || tview.TaggedStringWidth(icon) != 2 {
			t.Fatalf("%s width rune %d tag %d %q", agent, runewidth.StringWidth(icon), tview.TaggedStringWidth(icon), icon)
		}
		seen[icon] = true
		if !strings.HasPrefix(Label(agent), icon+" ") || !strings.HasSuffix(Label(agent), agent) {
			t.Fatalf("label %q", Label(agent))
		}
	}
	unknown := Icon("unknown")
	if unknown != "??" || runewidth.StringWidth(unknown) != 2 || tview.TaggedStringWidth(unknown) != 2 {
		t.Fatalf("unknown %q", unknown)
	}
}

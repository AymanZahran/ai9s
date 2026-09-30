package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/AymanZahran/air9s/internal/model"
)

func TestSetTokenAndAge(t *testing.T) {
	got := setToken(`agent:claude "auth bug"`, "agent", "grok")
	if !strings.Contains(got, "agent:grok") || !strings.Contains(got, "auth bug") {
		t.Fatalf("token %q", got)
	}
	if setToken("sort:oldest ship", "sort", "") != "ship" {
		t.Fatalf("clear %+v", setToken("sort:oldest ship", "sort", ""))
	}
	if relAge(time.Time{}) != "-" {
		t.Fatal(relAge(time.Time{}))
	}
	text := preview(model.Session{Title: "hi", Agent: "claude", ID: "claude:1", CanDelete: true})
	if !strings.Contains(text, "hi") || !strings.Contains(text, "delete: yes") || !strings.Contains(text, Icon("claude")) {
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
	if UsageLines(model.Usage{}) != nil {
		t.Fatal("empty usage")
	}
}

func TestIcons(t *testing.T) {
	seen := map[string]bool{}
	for _, agent := range []string{"claude", "codex", "copilot", "grok", "agy", "gemini", "cursor", "opencode"} {
		icon := Icon(agent)
		if icon == "" || icon == Icon("unknown") || seen[icon] {
			t.Fatalf("%s icon %q", agent, icon)
		}
		seen[icon] = true
		if !strings.HasPrefix(Label(agent), icon+" ") || !strings.HasSuffix(Label(agent), agent) {
			t.Fatalf("label %q", Label(agent))
		}
	}
}

package tui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/AymanZahran/ai9s/internal/model"
	"github.com/AymanZahran/ai9s/internal/query"
)

// Icon is the two-column mark beside an agent. The marks are letters.
// An unknown agent is "??".
func Icon(agent string) string {
	icon := map[string]string{
		"claude":      "Ca",
		"codex":       "Cx",
		"copilot":     "Cp",
		"grok":        "Gk",
		"antigravity": "Ag",
		"gemini":      "Ge",
		"cursor":      "Cu",
		"opencode":    "Oc",
		"hermes":      "He",
		"openclaw":    "Oa",
		"junie":       "Jn",
		"jules":       "Ju",
		"goose":       "Go",
		"cline":       "Cl",
		"aider":       "Ai",
		"kiro":        "Ki",
		"kimi":        "Km",
		"minimax":     "Mm",
		"qwen":        "Qw",
		"mistral":     "Mi",
	}[agent]
	if icon == "" {
		return "??"
	}
	return icon
}

// Label is the icon plus the harness name, for lists and prompts.
func Label(agent string) string {
	if agent == "" {
		return ""
	}
	return Icon(agent) + " " + model.HarnessName(agent)
}

// sessionName is the name set with n. Otherwise it is the recorded title.
// When that title is missing, or it only repeats the native id, the cell is the session id.
func sessionName(s model.Session) string {
	if name := strings.TrimSpace(s.Name); name != "" {
		return name
	}
	name := strings.TrimSpace(s.Title)
	native := strings.TrimSpace(s.NativeID)
	id := strings.TrimSpace(s.ID)
	if name != "" && name != native && name != id {
		return name
	}
	if id != "" {
		return id
	}
	if native != "" {
		return native
	}
	return "-"
}

func shortPath(p string) string {
	if p == "" {
		return "-"
	}
	home, err := os.UserHomeDir()
	if err == nil && home != "" && (p == home || strings.HasPrefix(p, home+string(os.PathSeparator))) {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}

func relAge(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t)
	future := d < 0
	if future {
		d = -d
	}
	label := ageUnit(d)
	if future && label != "now" {
		return "in " + label
	}
	return label
}

func ageUnit(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 14*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 60*24*time.Hour:
		return fmt.Sprintf("%dw", int(d.Hours()/(24*7)))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo", int(d.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%dy", int(d.Hours()/(24*365)))
	}
}

// absDate is the local clock time, kept in its own column from the relative age.
func absDate(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04")
}

func agentColor(name string) string {
	switch name {
	case "claude":
		return "orange"
	case "codex":
		return "green"
	case "copilot":
		return "blue"
	case "grok":
		return "aqua"
	case "antigravity", "agy":
		return "purple"
	case "gemini":
		return "yellow"
	case "cursor":
		return "silver"
	case "opencode":
		return "fuchsia"
	case "hermes":
		return "#ffd700"
	case "openclaw":
		return "#2dd4bf"
	case "junie":
		return "#7dd3fc"
	case "jules":
		return "#5a009d"
	case "goose":
		return "#f59e0b"
	case "cline":
		return "#22c55e"
	case "aider":
		return "#fb7185"
	case "kiro":
		return "#a78bfa"
	case "kimi":
		return "#60a5fa"
	case "minimax":
		return "#f97316"
	case "qwen":
		return "#6366f1"
	case "mistral":
		return "#f43f5e"
	default:
		return "white"
	}
}

func quoteTok(s string) string {
	if s == "" || strings.ContainsAny(s, " \t\"'") {
		return strconv.Quote(s)
	}
	return s
}

func tokenFamily(key string) string {
	switch strings.ToLower(key) {
	case "harness", "agent", "a", "h":
		return "harness"
	case "dir", "directory", "cwd", "d":
		return "dir"
	case "branch", "b":
		return "branch"
	case "model", "m":
		return "model"
	default:
		return strings.ToLower(key)
	}
}

func setToken(raw, key, val string) string {
	out := make([]string, 0, 8)
	replaced := false
	family := tokenFamily(key)
	for _, tok := range query.Tokens(raw) {
		k, _, ok := strings.Cut(tok, ":")
		if ok && tokenFamily(k) == family {
			if !replaced && val != "" {
				out = append(out, key+":"+val)
				replaced = true
			}
			continue
		}
		out = append(out, quoteTok(tok))
	}
	if !replaced && val != "" {
		out = append(out, key+":"+val)
	}
	return strings.Join(out, " ")
}

func preview(s model.Session) string {
	return previewText(s, agentColor(s.Agent), true)
}

// previewText draws one session. tag is the agent color. icons is false when
// the two-letter mark is left off the describe line. User fields are escaped;
// the color tags are ours.
func previewText(s model.Session, tag string, icons bool) string {
	tag = colorTag(tag)
	var b strings.Builder
	if name := strings.TrimSpace(s.Name); name != "" && name != strings.TrimSpace(s.Title) {
		fmt.Fprintf(&b, "[::b]%s[-]\n", markupLine(name))
	}
	mark := Icon(s.Agent)
	if !icons {
		mark = "  "
	}
	fmt.Fprintf(&b, "[::b]%s[-]  %s [%s]%s[-]\n", markupLine(s.Title), mark, tag, markupLine(model.HarnessName(s.Agent)))
	fmt.Fprintf(&b, "%s", markupLine(shortPath(s.CWD)))
	if s.Branch != "" {
		fmt.Fprintf(&b, "  [green]%s[-]", markupLine(s.Branch))
	}
	if s.Model != "" {
		fmt.Fprintf(&b, "  [gray]%s[-]", markupLine(s.Model))
	}
	fmt.Fprintf(&b, "\n%s   %s   %d messages   %s\n", relAge(s.Updated), absDate(s.Updated), s.Messages, markupLine(s.ID))
	if s.Bookmarked {
		b.WriteString("bookmark   yes\n")
	}
	for _, line := range UsageLines(s.Usage) {
		fmt.Fprintf(&b, "%s\n", markupLine(line))
	}
	if s.Summary != "" && s.Summary != s.Title {
		fmt.Fprintf(&b, "\n[::b]summary[-]\n%s\n", markup(s.Summary))
	}
	if s.Messages > len(s.Snippets) && len(s.Snippets) > 0 {
		b.WriteString("\n[gray]excerpt: the start and the latest turns[-]\n")
	}
	for _, sn := range s.Snippets {
		color := "yellow"
		if sn.Role == "assistant" {
			color = "blue"
		}
		fmt.Fprintf(&b, "\n[%s]%s[-]\n%s\n", color, markupLine(sn.Role), markup(sn.Body))
	}
	return b.String()
}

// sessionStatsText is the usage panel for one session.
func sessionStatsText(s model.Session) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n%s\n", sessionName(s), Label(s.Agent))
	if modelName := strings.TrimSpace(s.Model); modelName != "" {
		fmt.Fprintf(&b, "model      %s\n", modelName)
	}
	fmt.Fprintf(&b, "messages   %d\n", s.Messages)
	fmt.Fprintf(&b, "updated    %s   %s\n", relAge(s.Updated), absDate(s.Updated))
	if title := strings.TrimSpace(s.Title); title != "" && title != sessionName(s) {
		fmt.Fprintf(&b, "title      %s\n", title)
	}
	for _, line := range UsageLines(s.Usage) {
		fmt.Fprintf(&b, "%s\n", line)
	}
	return strings.TrimRight(b.String(), "\n")
}

// UsageLines describes context and token accounting for the preview and show command.
// Context and tokens are always present. A dash means the session file did not record them.
func UsageLines(u model.Usage) []string {
	var lines []string
	switch {
	case u.Context > 0 && u.Window > 0:
		pct := u.Context * 100 / u.Window
		lines = append(lines, fmt.Sprintf("context    %s / %s  (%d%%)", compactCount(u.Context), compactCount(u.Window), pct))
	case u.Context > 0:
		lines = append(lines, "context    "+compactCount(u.Context))
	case u.Window > 0:
		lines = append(lines, "context    window "+compactCount(u.Window))
	default:
		lines = append(lines, "context    -")
	}
	var parts []string
	if u.Input > 0 && u.Input != u.Context {
		parts = append(parts, "in "+compactCount(u.Input))
	}
	if u.Output > 0 {
		parts = append(parts, "out "+compactCount(u.Output))
	}
	if u.CacheRead > 0 {
		parts = append(parts, "cache read "+compactCount(u.CacheRead))
	}
	if u.CacheWrite > 0 {
		parts = append(parts, "cache write "+compactCount(u.CacheWrite))
	}
	if u.Reasoning > 0 {
		parts = append(parts, "reasoning "+compactCount(u.Reasoning))
	}
	if u.Total > 0 && u.Total != u.Context && u.Input == 0 && u.Output == 0 {
		parts = append(parts, "total "+compactCount(u.Total))
	}
	if len(parts) > 0 {
		lines = append(lines, "tokens     "+strings.Join(parts, "   "))
	} else {
		lines = append(lines, "tokens     -")
	}
	if u.CostUSD > 0 {
		lines = append(lines, "cost       $"+trimCost(u.CostUSD))
	}
	if u.Requests > 0 {
		lines = append(lines, fmt.Sprintf("requests   %d premium", u.Requests))
	}
	if u.Effort != "" {
		lines = append(lines, "effort     "+u.Effort)
	}
	return lines
}

func contextLabel(u model.Usage) string {
	switch {
	case u.Context > 0 && u.Window > 0:
		return compactCount(u.Context) + "/" + compactCount(u.Window)
	case u.Context > 0:
		return compactCount(u.Context)
	case u.Window > 0:
		return "/" + compactCount(u.Window)
	default:
		return "-"
	}
}

// tokenLabel is the TOKENS column. It is the session total when the file
// recorded one, otherwise input and output. It is not the CTX prompt size.
func tokenLabel(u model.Usage) string {
	switch {
	case u.Total > 0:
		return compactCount(u.Total)
	case u.Input > 0 && u.Output > 0:
		return compactCount(u.Input) + "+" + compactCount(u.Output)
	case u.Input > 0:
		return compactCount(u.Input)
	case u.Output > 0:
		return compactCount(u.Output)
	default:
		return "-"
	}
}

func costLabel(u model.Usage) string {
	if u.CostUSD <= 0 {
		return "-"
	}
	return "$" + trimCost(u.CostUSD)
}

func compactCount(n int) string {
	if n < 0 {
		n = 0
	}
	switch {
	case n >= 1000000:
		return trimFloat(float64(n)/1000000) + "m"
	case n >= 1000:
		return trimFloat(float64(n)/1000) + "k"
	default:
		return strconv.Itoa(n)
	}
}

func trimFloat(v float64) string {
	s := strconv.FormatFloat(v, 'f', 1, 64)
	return strings.TrimSuffix(s, ".0")
}

func trimCost(v float64) string {
	s := strconv.FormatFloat(v, 'f', 4, 64)
	s = strings.TrimRight(s, "0")
	return strings.TrimRight(s, ".")
}

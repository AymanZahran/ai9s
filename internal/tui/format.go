package tui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/AymanZahran/air9s/internal/model"
	"github.com/AymanZahran/air9s/internal/query"
	"github.com/mattn/go-runewidth"
)

// Icon is the mark drawn beside an agent. Every icon occupies two columns.
// Glyphs are taken from what that product prints. Cursor's logo is a cube
// image, so its cell is blank. Jules, Goose, Cline, and Aider print a name
// and no glyph, so those cells are the first two letters of the name.
func Icon(agent string) string {
	icon := map[string]string{
		"claude":   "✻",  // Claude Code status glyph, U+273B
		"codex":    ">_", // Codex banner: ">_ OpenAI Codex"
		"copilot":  "╭╮", // eyes of the Copilot CLI mascot
		"grok":     "⣠⣾", // opening cells of Grok's braille logo
		"agy":      "▄▀", // opening cells of the Antigravity CLI logo
		"gemini":   "✦",  // Gemini CLI prompt glyph, U+2726
		"cursor":   "  ",
		"opencode": "█▀", // opening cells of the OpenCode wordmark
		"hermes":   "██", // opening cells of the Hermes banner
		"openclaw": "🦞",  // OpenClaw's own README mark
		"junie":    "//", // Junie help banner
		"jules":    "Ju",
		"goose":    "Go",
		"cline":    "Cl",
		"aider":    "Ai",
		"kiro":     "╭─", // Kiro CLI menu frame
	}[agent]
	if icon == "" {
		icon = "⚪"
	}
	if runewidth.StringWidth(icon) < 2 {
		icon += " "
	}
	return icon
}

// Label is the icon plus the agent id, for lists and prompts.
func Label(agent string) string {
	if agent == "" {
		return ""
	}
	return Icon(agent) + " " + agent
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
	if d < 0 {
		return t.Local().Format("2006-01-02")
	}
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 14*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	default:
		return t.Local().Format("2006-01-02")
	}
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
	case "agy":
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

func setToken(raw, key, val string) string {
	out := make([]string, 0, 8)
	replaced := false
	for _, tok := range query.Tokens(raw) {
		k, _, ok := strings.Cut(tok, ":")
		if ok && strings.EqualFold(k, key) {
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
	var b strings.Builder
	fmt.Fprintf(&b, "[::b]%s[-]  %s [%s]%s[-]\n", s.Title, Icon(s.Agent), agentColor(s.Agent), s.Agent)
	fmt.Fprintf(&b, "%s", shortPath(s.CWD))
	if s.Branch != "" {
		fmt.Fprintf(&b, "  [green]%s[-]", s.Branch)
	}
	if s.Model != "" {
		fmt.Fprintf(&b, "  [gray]%s[-]", s.Model)
	}
	fmt.Fprintf(&b, "\n%s   %d messages   %s\n", relAge(s.Updated), s.Messages, s.ID)
	if s.CanDelete {
		b.WriteString("[green]delete: yes[-]\n")
	} else {
		reason := s.DeleteReason
		if reason == "" {
			reason = "disabled for this agent"
		}
		fmt.Fprintf(&b, "[red]delete: no[-]  %s\n", reason)
	}
	for _, line := range UsageLines(s.Usage) {
		fmt.Fprintf(&b, "%s\n", line)
	}
	if s.Summary != "" && s.Summary != s.Title {
		fmt.Fprintf(&b, "\n[::b]summary[-]\n%s\n", s.Summary)
	}
	if s.Messages > len(s.Snippets) && len(s.Snippets) > 0 {
		b.WriteString("\n[gray]excerpt: the start and the latest turns[-]\n")
	}
	for _, sn := range s.Snippets {
		color := "yellow"
		if sn.Role == "assistant" {
			color = "blue"
		}
		fmt.Fprintf(&b, "\n[%s]%s[-]\n%s\n", color, sn.Role, sn.Body)
	}
	return b.String()
}

// UsageLines describes context and token accounting for the preview and show command.
func UsageLines(u model.Usage) []string {
	if u.Empty() {
		return nil
	}
	var lines []string
	if u.Context > 0 || u.Window > 0 {
		switch {
		case u.Context > 0 && u.Window > 0:
			pct := u.Context * 100 / u.Window
			lines = append(lines, fmt.Sprintf("context    %s / %s  (%d%%)", compactCount(u.Context), compactCount(u.Window), pct))
		case u.Context > 0:
			lines = append(lines, "context    "+compactCount(u.Context))
		default:
			lines = append(lines, "context    window "+compactCount(u.Window))
		}
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
	case u.Total > 0:
		return compactCount(u.Total)
	case u.Window > 0:
		return "/" + compactCount(u.Window)
	default:
		return ""
	}
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

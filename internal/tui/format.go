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
	fmt.Fprintf(&b, "[::b]%s[-]  [%s]%s[-]\n", s.Title, agentColor(s.Agent), s.Agent)
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
	if s.Summary != "" && s.Summary != s.Title {
		fmt.Fprintf(&b, "\n[::b]summary[-]\n%s\n", s.Summary)
	}
	snips := s.Snippets
	if len(snips) > 8 {
		head := snips[:2]
		tail := snips[len(snips)-6:]
		snips = append(append([]model.Snippet{}, head...), tail...)
		b.WriteString("\n[gray]… middle of the transcript omitted[-]\n")
	}
	for _, sn := range snips {
		color := "yellow"
		if sn.Role == "assistant" {
			color = "blue"
		}
		fmt.Fprintf(&b, "\n[%s]%s[-]\n%s\n", color, sn.Role, sn.Body)
	}
	return b.String()
}

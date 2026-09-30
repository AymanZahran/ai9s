// Package query parses the filter line used by the UI and the search command.
package query

import (
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Filter is a parsed search.
type Filter struct {
	Text   string
	Agent  string
	Dir    string
	Branch string
	Model  string
	Since  time.Time
	Until  time.Time
	Sort   string // recent, oldest, messages, title
}

// Parse splits agent:, dir:, branch:, model:, date:, and sort: tokens from free text.
func Parse(raw string) Filter {
	f := Filter{Sort: "recent"}
	var words []string
	for _, tok := range Tokens(raw) {
		key, val, ok := strings.Cut(tok, ":")
		if !ok || val == "" {
			words = append(words, tok)
			continue
		}
		switch strings.ToLower(key) {
		case "agent", "a":
			f.Agent = strings.ToLower(val)
		case "dir", "directory", "cwd", "d":
			f.Dir = val
		case "branch", "b":
			f.Branch = val
		case "model", "m":
			f.Model = val
		case "sort":
			switch strings.ToLower(val) {
			case "recent", "oldest", "messages", "title":
				f.Sort = strings.ToLower(val)
			}
		case "date":
			applyDate(&f, val)
		default:
			words = append(words, tok)
		}
	}
	f.Text = strings.Join(words, " ")
	return f
}

func applyDate(f *Filter, val string) {
	now := time.Now()
	switch {
	case strings.HasPrefix(val, "<"):
		if d, ok := duration(val[1:]); ok {
			f.Since = now.Add(-d)
		}
	case strings.HasPrefix(val, ">"):
		if d, ok := duration(val[1:]); ok {
			f.Until = now.Add(-d)
		}
	default:
		if t, err := time.Parse("2006-01-02", val); err == nil {
			f.Since = t
			f.Until = t.Add(24 * time.Hour)
		} else if len(val) == 7 {
			if t, err := time.Parse("2006-01", val); err == nil {
				f.Since = t
				f.Until = t.AddDate(0, 1, 0)
			}
		}
	}
}

func duration(s string) (time.Duration, bool) {
	if s == "" {
		return 0, false
	}
	unit := s[len(s)-1]
	n, err := strconv.Atoi(s[:len(s)-1])
	if err != nil || n < 0 {
		return 0, false
	}
	switch unit {
	case 'm':
		return time.Duration(n) * time.Minute, true
	case 'h':
		return time.Duration(n) * time.Hour, true
	case 'd':
		return time.Duration(n) * 24 * time.Hour, true
	case 'w':
		return time.Duration(n) * 7 * 24 * time.Hour, true
	default:
		return 0, false
	}
}

// Tokens splits a filter line on whitespace, keeping quoted phrases together.
func Tokens(s string) []string {
	var out []string
	var b strings.Builder
	q := rune(0)
	for _, r := range s {
		switch {
		case q != 0:
			if r == q {
				q = 0
			} else {
				b.WriteRune(r)
			}
		case r == '"' || r == '\'':
			q = r
		case unicode.IsSpace(r):
			if b.Len() > 0 {
				out = append(out, b.String())
				b.Reset()
			}
		default:
			b.WriteRune(r)
		}
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}

// FTS turns free text into a safe FTS5 MATCH expression.
func FTS(text string) string {
	var parts []string
	for _, w := range Tokens(text) {
		w = strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
				return r
			}
			return -1
		}, w)
		if w != "" {
			parts = append(parts, `"`+w+`"`)
		}
	}
	return strings.Join(parts, " AND ")
}

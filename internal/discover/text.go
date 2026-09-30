package discover

import (
	"encoding/json"
	"os"
	"strings"
	"unicode/utf8"
)

func readJSON(path string, dest any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dest)
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}

func contentText(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		var b strings.Builder
		for _, part := range t {
			m, ok := part.(map[string]any)
			if !ok {
				continue
			}
			if s, ok := m["text"].(string); ok && s != "" {
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(s)
			}
		}
		return b.String()
	case map[string]any:
		if s, ok := t["text"].(string); ok {
			return s
		}
		if c, ok := t["content"]; ok {
			return contentText(c)
		}
	}
	return ""
}

func messageText(raw map[string]any) string {
	if m, ok := raw["message"].(map[string]any); ok {
		if s := contentText(m["content"]); s != "" {
			return s
		}
	}
	if s := contentText(raw["content"]); s != "" {
		return s
	}
	if s, ok := raw["text"].(string); ok {
		return s
	}
	return ""
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func homeJoin(elem ...string) string {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return ""
	}
	return join(h, elem...)
}

func join(root string, elem ...string) string {
	if root == "" {
		return ""
	}
	all := append([]string{root}, elem...)
	return strings.Join(all, string(os.PathSeparator))
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

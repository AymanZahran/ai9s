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

func nestedText(obj map[string]any) (role, text string) {
	if msg, ok := obj["message"].(map[string]any); ok {
		role = strings.TrimSpace(asString(msg["role"]))
		text = contentText(msg["content"])
	}
	if role == "" {
		role = strings.TrimSpace(asString(obj["role"]))
	}
	if role == "" {
		role = strings.TrimSpace(asString(obj["kind"]))
	}
	if text == "" {
		text = contentText(obj["content"])
	}
	if text == "" {
		text = strings.TrimSpace(asString(obj["text"]))
	}
	return role, text
}

func nestedUsage(obj map[string]any) map[string]any {
	if msg, ok := obj["message"].(map[string]any); ok {
		if u, ok := msg["usage"].(map[string]any); ok {
			return u
		}
	}
	if u, ok := obj["usage"].(map[string]any); ok {
		return u
	}
	return nil
}

func sidechain(obj map[string]any) bool {
	v, ok := obj["isSidechain"].(bool)
	return ok && v
}

func asInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

func asFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
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

package discover

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AymanZahran/air9s/internal/model"
)

func kiroDB() string {
	if v := strings.TrimSpace(os.Getenv("KIRO_CLI_DB")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("KIRO_HOME")); v != "" {
		return filepath.Join(v, "data.sqlite3")
	}
	var candidates []string
	if data := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); data != "" {
		candidates = append(candidates, filepath.Join(data, "kiro-cli", "data.sqlite3"))
	}
	if runtime.GOOS == "darwin" {
		candidates = append(candidates, homeJoin("Library", "Application Support", "kiro-cli", "data.sqlite3"))
	}
	candidates = append(candidates, homeJoin(".local", "share", "kiro-cli", "data.sqlite3"))
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return ""
}

func scanKiro(fresh func(string, int64) bool) Batch {
	b := Batch{Agent: "kiro"}
	path := kiroDB()
	if path == "" {
		return b
	}
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return b
	}
	mt, skip := stamp(path, fresh)
	b.Files = []File{{Path: path, Mtime: mt, Fresh: skip}}
	if skip {
		return b
	}
	sessions, err := readKiro(path, mt)
	if err != nil {
		b.Err = err
		return b
	}
	b.Sessions = sessions
	return b
}

// readKiro lists conversations_v2. That JSON has the transcript and a
// context-file budget. It does not record a prompt size or token counts,
// so CTX and TOKENS stay a dash.
func readKiro(path string, mt int64) ([]model.Session, error) {
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`
		SELECT coalesce(conversation_id,''), coalesce(key,''), coalesce(value,''), updated_at, created_at
		FROM conversations_v2`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Session
	seen := map[string]bool{}
	for rows.Next() {
		var convID, key, raw string
		var updated, created sql.NullInt64
		if err := rows.Scan(&convID, &key, &raw, &updated, &created); err != nil {
			return nil, err
		}
		id := strings.TrimSpace(convID)
		if id == "" {
			id = strings.TrimSpace(key)
		}
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		s := kiroSession(id, raw, path, mt)
		s.Updated = unixish(float64(updated.Int64))
		if s.Updated.IsZero() {
			s.Updated = unixish(float64(created.Int64))
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func kiroSession(id, raw, path string, mt int64) model.Session {
	s := model.Session{
		ID: model.ID("kiro", id), NativeID: id, Agent: "kiro",
		Title: id, SourcePath: path, SourceMtime: mt,
		CanDelete: true, DeleteMode: "kiro",
	}
	var obj map[string]any
	if json.Unmarshal([]byte(raw), &obj) != nil {
		return s
	}
	if cid := firstString(obj, "conversation_id"); cid != "" {
		s.NativeID = cid
		s.ID = model.ID("kiro", cid)
	}
	history, _ := obj["history"].([]any)
	var buf snippetBuf
	for _, turn := range history {
		parts, _ := turn.([]any)
		if parts == nil {
			if m, ok := turn.(map[string]any); ok {
				parts = []any{m}
			}
		}
		for _, part := range parts {
			m, ok := part.(map[string]any)
			if !ok {
				continue
			}
			if s.CWD == "" {
				s.CWD = kiroCWD(m)
			}
			text := kiroText(m["content"])
			if text == "" {
				continue
			}
			role := "assistant"
			if content, ok := m["content"].(map[string]any); ok {
				if _, isPrompt := content["Prompt"]; isPrompt {
					role = "user"
				}
			}
			if strings.EqualFold(firstString(m, "role", "type"), "user") {
				role = "user"
			}
			buf.add(role, text, "")
		}
	}
	s.Snippets = buf.snippets()
	s.Messages = buf.n
	if title := snippetTitle(s.Snippets); title != "" {
		s.Title = title
	}
	if summary := firstString(obj, "latest_summary"); summary != "" && summary != s.Title {
		s.Summary = clip(summary, 280)
	}
	return s
}

func kiroCWD(m map[string]any) string {
	env, _ := m["env_context"].(map[string]any)
	state, _ := env["env_state"].(map[string]any)
	return firstString(state, "current_working_directory", "cwd")
}

func kiroText(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		if p, ok := t["Prompt"]; ok {
			if s := kiroText(p); s != "" {
				return s
			}
		}
		if s := firstString(t, "prompt", "text", "content"); s != "" {
			return s
		}
		return contentText(t)
	default:
		return contentText(v)
	}
}

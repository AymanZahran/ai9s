package discover

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"

	"github.com/AymanZahran/air9s/internal/model"
)

func openclawHome() string {
	if v := strings.TrimSpace(os.Getenv("OPENCLAW_STATE_DIR")); v != "" {
		return v
	}
	return envOr("OPENCLAW_HOME", homeJoin(".openclaw"))
}

func scanOpenClaw(fresh func(string, int64) bool) Batch {
	b := Batch{Agent: "openclaw"}
	home := openclawHome()
	if home == "" {
		return b
	}
	entries, err := os.ReadDir(filepath.Join(home, "agents"))
	if err != nil {
		return b
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(home, "agents", e.Name(), "agent", "openclaw-agent.sqlite")
		st, err := os.Stat(path)
		if err != nil || st.IsDir() {
			continue
		}
		mt, skip := stamp(path, fresh)
		if skip {
			b.Files = append(b.Files, File{Path: path, Mtime: mt, Fresh: true})
			continue
		}
		sessions, err := readOpenClaw(path, mt)
		if err != nil {
			// Leave the previous rows in place and try this file again next time.
			b.Files = append(b.Files, File{Path: path, Mtime: mt, Fresh: true})
			continue
		}
		b.Files = append(b.Files, File{Path: path, Mtime: mt})
		b.Sessions = append(b.Sessions, sessions...)
	}
	return b
}

func readOpenClaw(path string, mt int64) ([]model.Session, error) {
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`
		SELECT session_key, coalesce(display_name,''), coalesce(label,''),
			updated_at, last_activity_at, archived_at,
			json_extract(entry_json, '$.sessionId'),
			json_extract(entry_json, '$.displayName'),
			json_extract(entry_json, '$.model'),
			json_extract(entry_json, '$.modelProvider'),
			json_extract(entry_json, '$.systemPromptReport.workspaceDir'),
			json_extract(entry_json, '$.inputTokens'),
			json_extract(entry_json, '$.outputTokens'),
			json_extract(entry_json, '$.cacheRead'),
			json_extract(entry_json, '$.cacheWrite'),
			json_extract(entry_json, '$.estimatedCostUsd'),
			json_extract(entry_json, '$.contextTokens'),
			json_extract(entry_json, '$.contextBudgetStatus.estimatedPromptTokens')
		FROM session_nodes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Session
	seen := map[string]int{}
	ids := map[string]string{}
	for rows.Next() {
		var key, display, label string
		var updated, activity, archived sql.NullInt64
		var sessionID, entryName, modelName, provider, cwd sql.NullString
		var inTok, outTok, cacheR, cacheW sql.NullFloat64
		var cost, window, context sql.NullFloat64
		if err := rows.Scan(&key, &display, &label, &updated, &activity, &archived,
			&sessionID, &entryName, &modelName, &provider, &cwd,
			&inTok, &outTok, &cacheR, &cacheW, &cost, &window, &context); err != nil {
			return nil, err
		}
		key = strings.TrimSpace(key)
		if key == "" || (archived.Valid && archived.Int64 != 0) {
			continue
		}
		title := strings.TrimSpace(display)
		if title == "" {
			title = strings.TrimSpace(entryName.String)
		}
		if title == "" {
			title = strings.TrimSpace(label)
		}
		if title == "" {
			title = key
		}
		modelLine := strings.TrimSpace(modelName.String)
		if provider.String != "" && modelLine != "" {
			modelLine = provider.String + "/" + modelLine
		}
		when := unixish(float64(updated.Int64))
		if when.IsZero() {
			when = unixish(float64(activity.Int64))
		}
		s := model.Session{
			ID: model.ID("openclaw", key), NativeID: key, Agent: "openclaw",
			Title: title, CWD: cwd.String, Model: modelLine, Updated: when,
			SourcePath: path, SourceMtime: mt,
			CanDelete: true, DeleteMode: "exec",
			Usage: model.Usage{
				Input: int(inTok.Float64), Output: int(outTok.Float64),
				CacheRead: int(cacheR.Float64), CacheWrite: int(cacheW.Float64),
				CostUSD: cost.Float64, Window: int(window.Float64), Context: int(context.Float64),
			},
		}
		ids[key] = strings.TrimSpace(sessionID.String)
		if i, ok := seen[key]; ok {
			if s.Updated.After(out[i].Updated) {
				out[i] = s
			}
			continue
		}
		seen[key] = len(out)
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	_ = fillOpenClawSnippets(db, out, ids)
	return out, nil
}

func fillOpenClawSnippets(db *sql.DB, sessions []model.Session, ids map[string]string) error {
	rows, err := db.Query(`SELECT session_id, role, substr(coalesce(text,''), 1, 2000) FROM session_transcript_fts`)
	if err != nil {
		return err
	}
	defer rows.Close()
	bySession := map[string]*snippetBuf{}
	for rows.Next() {
		var id, role, body string
		if err := rows.Scan(&id, &role, &body); err != nil {
			return err
		}
		buf := bySession[id]
		if buf == nil {
			buf = &snippetBuf{}
			bySession[id] = buf
		}
		buf.add(role, body, "")
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range sessions {
		buf := bySession[ids[sessions[i].NativeID]]
		if buf == nil {
			buf = bySession[sessions[i].NativeID]
		}
		if buf == nil {
			continue
		}
		sessions[i].Snippets = buf.snippets()
		if buf.n > 0 {
			sessions[i].Messages = buf.n
		}
		if sessions[i].Title == sessions[i].NativeID {
			if title := snippetTitle(sessions[i].Snippets); title != "" {
				sessions[i].Title = title
			}
		}
	}
	return nil
}

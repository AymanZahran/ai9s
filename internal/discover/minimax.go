package discover

import (
	"database/sql"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/AymanZahran/ai9s/internal/model"
)

func minimaxData() string {
	if v := strings.TrimSpace(os.Getenv("MINIMAX_DATA_DIR")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("MAVIS_DATA_DIR")); v != "" {
		return v
	}
	return homeJoin(".minimax")
}

func minimaxDB() string {
	data := minimaxData()
	if data == "" {
		return ""
	}
	return filepath.Join(data, "v2", "sqlite", "runtime-state.sqlite")
}

func minimaxSessions() string {
	data := minimaxData()
	if data == "" {
		return ""
	}
	return filepath.Join(data, "v2", "sessions")
}

func scanMinimax(fresh func(string, int64) bool) Batch {
	b := Batch{Agent: "minimax"}
	path := minimaxDB()
	if path != "" && regularFile(path) {
		has, err := minimaxHasSessions(path)
		if err != nil {
			b.Err = err
			return b
		}
		if has {
			mt, skip := stamp(path, fresh)
			b.Files = append(b.Files, File{Path: path, Mtime: mt, Fresh: skip})
			if skip {
				return b
			}
			sessions, err := readMinimaxDB(path, mt)
			if err != nil {
				b.Err = err
				return b
			}
			b.Sessions = sessions
			return b
		}
	}
	walkMinimaxDirs(&b, minimaxSessions(), fresh)
	return b
}

func minimaxHasSessions(path string) (bool, error) {
	db, err := openDB(path)
	if err != nil {
		return false, err
	}
	defer db.Close()
	var n int
	err = db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'local_runtime_sessions'`).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func readMinimaxDB(path string, mt int64) ([]model.Session, error) {
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	usage := minimaxUsage(db)
	rows, err := db.Query(`
		SELECT session_id,
		       coalesce(title, ''),
		       coalesce(workspace_dir, ''),
		       coalesce(updated_at_ms, 0),
		       coalesce(created_at_ms, 0),
		       coalesce(history_relative_dir, ''),
		       coalesce(visibility, 'visible'),
		       coalesce(session_kind, ''),
		       coalesce(parent_session_id, ''),
		       coalesce(record_json, '')
		FROM local_runtime_sessions`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Session
	for rows.Next() {
		var id, title, cwd, history, visibility, kind, parent, record string
		var updated, created int64
		if err := rows.Scan(&id, &title, &cwd, &updated, &created, &history, &visibility, &kind, &parent, &record); err != nil {
			return nil, err
		}
		id = strings.TrimSpace(id)
		if id == "" || strings.TrimSpace(parent) != "" {
			continue
		}
		if visibility == "hidden" || kind == "peek" || kind == "channel" || kind == "cron" {
			continue
		}
		extraModel, branch, window, recTitle, recCWD := minimaxRecord(record)
		if strings.TrimSpace(title) == "" {
			title = recTitle
		}
		if strings.TrimSpace(cwd) == "" {
			cwd = recCWD
		}
		u := usage[id]
		if window > 0 {
			u.Window = window
		}
		modelName := u.Effort
		u.Effort = ""
		if modelName == "" {
			modelName = extraModel
		}
		var buf snippetBuf
		if dir := minimaxHistoryDir(history); dir != "" {
			buf = readMinimaxMessages(filepath.Join(dir, "messages.jsonl"), &u, &modelName)
		}
		if strings.TrimSpace(title) == "" {
			title = snippetTitle(buf.snippets())
		}
		if title == "" {
			title = id
		}
		when := unixish(float64(updated))
		if when.IsZero() {
			when = unixish(float64(created))
		}
		if when.IsZero() {
			when = unixish(float64(mt))
		}
		out = append(out, model.Session{
			ID: model.ID("minimax", id), NativeID: id, Agent: "minimax",
			Title: title, CWD: cwd, Branch: branch, Model: modelName,
			Updated: when, Messages: buf.n, Snippets: buf.snippets(), Usage: u,
			SourcePath: path, SourceMtime: mt,
			CanDelete: true, DeleteMode: "minimax",
		})
	}
	return out, rows.Err()
}

func minimaxUsage(db *sql.DB) map[string]model.Usage {
	out := map[string]model.Usage{}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'local_runtime_token_usage'`).Scan(&n); err != nil || n == 0 {
		return out
	}
	rows, err := db.Query(`
		SELECT session_id,
		       coalesce(sum(input_tokens), 0),
		       coalesce(sum(output_tokens), 0),
		       coalesce(sum(reasoning_tokens), 0),
		       coalesce(sum(cache_read_tokens), 0),
		       coalesce(sum(cache_write_tokens), 0),
		       coalesce(sum(cost_usd), 0)
		FROM local_runtime_token_usage
		GROUP BY session_id`)
	if err != nil {
		return out
	}
	for rows.Next() {
		var id string
		var u model.Usage
		if err := rows.Scan(&id, &u.Input, &u.Output, &u.Reasoning, &u.CacheRead, &u.CacheWrite, &u.CostUSD); err != nil {
			rows.Close()
			return out
		}
		out[id] = u
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return out
	}
	rows.Close()
	latest, err := db.Query(`
		SELECT session_id, coalesce(input_tokens, 0), coalesce(model, '')
		FROM local_runtime_token_usage
		WHERE id IN (SELECT max(id) FROM local_runtime_token_usage GROUP BY session_id)`)
	if err != nil {
		return out
	}
	defer latest.Close()
	for latest.Next() {
		var id, name string
		var context int
		if err := latest.Scan(&id, &context, &name); err != nil {
			return out
		}
		u := out[id]
		if context > 0 {
			u.Context = context
		}
		if name != "" {
			u.Effort = name
		}
		out[id] = u
	}
	return out
}

func minimaxRecord(raw string) (modelName, branch string, window int, title, cwd string) {
	var rec map[string]any
	if raw == "" || json.Unmarshal([]byte(raw), &rec) != nil {
		return "", "", 0, "", ""
	}
	modelName = strings.TrimSpace(asString(rec["effectiveModel"]))
	window = num(rec["effectiveModelContextWindow"])
	title = strings.TrimSpace(asString(rec["title"]))
	cwd = strings.TrimSpace(asString(rec["workspaceDir"]))
	if cwd == "" {
		cwd = strings.TrimSpace(asString(rec["workspace_dir"]))
	}
	if loc, ok := rec["runLocation"].(map[string]any); ok {
		branch = strings.TrimSpace(asString(loc["resolvedBranch"]))
		if cwd == "" {
			cwd = strings.TrimSpace(asString(loc["resolvedDir"]))
		}
	}
	return modelName, branch, window, title, cwd
}

func walkMinimaxDirs(b *Batch, root string, fresh func(string, int64) bool) {
	if root == "" {
		return
	}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() || d.Name() != "manifest.json" {
			return nil
		}
		if !regularFile(path) {
			return nil
		}
		dir := filepath.Dir(path)
		if !minimaxSessionDir(root, dir) {
			return nil
		}
		msgs := filepath.Join(dir, "messages.jsonl")
		if !regularFile(msgs) {
			return nil
		}
		mt, skip := stamp(msgs, fresh, path)
		b.Files = append(b.Files, File{Path: msgs, Mtime: mt, Fresh: skip})
		if skip {
			return nil
		}
		sess, ok := readMinimaxManifest(path, msgs, mt)
		if ok {
			b.Sessions = append(b.Sessions, sess)
		}
		return nil
	})
}

func readMinimaxManifest(manifest, msgs string, mt int64) (model.Session, bool) {
	var raw map[string]any
	if err := readJSON(manifest, &raw); err != nil {
		return model.Session{}, false
	}
	id := strings.TrimSpace(asString(raw["sessionId"]))
	if id == "" || strings.ContainsAny(id, `/\`) || strings.HasPrefix(id, "-") {
		return model.Session{}, false
	}
	var usage model.Usage
	modelName := ""
	buf := readMinimaxMessages(msgs, &usage, &modelName)
	title := snippetTitle(buf.snippets())
	if title == "" {
		title = id
	}
	updated := unixish(asFloat(raw["updatedAtMs"]))
	if updated.IsZero() {
		updated = unixish(asFloat(raw["createdAtMs"]))
	}
	if updated.IsZero() {
		updated = unixish(float64(mt))
	}
	return model.Session{
		ID: model.ID("minimax", id), NativeID: id, Agent: "minimax",
		Title: title, Model: modelName, Updated: updated,
		Messages: buf.n, Snippets: buf.snippets(), Usage: usage,
		SourcePath: msgs, SourceMtime: mt,
		CanDelete: true, DeleteMode: "minimax",
	}, true
}

func readMinimaxMessages(path string, usage *model.Usage, modelName *string) snippetBuf {
	var buf snippetBuf
	if !regularFile(path) {
		return buf
	}
	emptyUsage := usage.Empty()
	_ = walkJSONL(path, func(obj map[string]any) error {
		msg, _ := obj["message"].(map[string]any)
		if msg == nil {
			msg = obj
		}
		role := asString(msg["role"])
		if role == "compactionSummary" || role == "tool" {
			return nil
		}
		text := contentText(msg["content"])
		if text == "" {
			text = contentText(msg["text"])
		}
		buf.add(role, text, "")
		if name := strings.TrimSpace(asString(msg["model"])); name != "" && *modelName == "" {
			*modelName = name
		}
		if role == "assistant" && emptyUsage {
			addMinimaxUsage(msg["usage"], usage)
		}
		return nil
	})
	return buf
}

func addMinimaxUsage(v any, usage *model.Usage) {
	raw, _ := v.(map[string]any)
	if raw == nil {
		return
	}
	usage.Input += num(raw["input"])
	usage.Output += num(raw["output"])
	usage.CacheRead += num(raw["cacheRead"])
	usage.CacheWrite += num(raw["cacheWrite"])
	usage.Reasoning += num(raw["reasoning"])
	if n := num(raw["input"]); n > 0 {
		usage.Context = n
	}
	if cost, ok := raw["cost"].(map[string]any); ok {
		usage.CostUSD += asFloat(cost["total"])
	}
}

func minimaxHistoryDir(rel string) string {
	rel = strings.TrimSpace(rel)
	if rel == "" || strings.Contains(rel, `\`) || strings.Contains(rel, "..") {
		return ""
	}
	parts := strings.Split(rel, "/")
	if len(parts) != 4 {
		return ""
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return ""
		}
	}
	root := minimaxSessions()
	dir := filepath.Join(root, parts[0], parts[1], parts[2], parts[3])
	if !minimaxSessionDir(root, dir) {
		return ""
	}
	return dir
}

func minimaxSessionDir(root, dir string) bool {
	if !withinDir(root, dir) {
		return false
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return false
	}
	parts := strings.Split(rel, string(os.PathSeparator))
	return len(parts) == 4
}

package discover

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/AymanZahran/ai9s/internal/model"
)

func gooseHome() string {
	if v := strings.TrimSpace(os.Getenv("GOOSE_HOME")); v != "" {
		return v
	}
	data := strings.TrimSpace(os.Getenv("XDG_DATA_HOME"))
	if data == "" {
		data = homeJoin(".local", "share")
	}
	if data == "" {
		return ""
	}
	return filepath.Join(data, "goose")
}

func gooseDB() string {
	home := gooseHome()
	if home == "" {
		return ""
	}
	return filepath.Join(home, "sessions", "sessions.db")
}

func scanGoose(fresh func(string, int64) bool) Batch {
	b := Batch{Agent: "goose"}
	path := gooseDB()
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
	sessions, err := readGoose(path, mt)
	if err != nil {
		b.Err = err
		return b
	}
	b.Sessions = sessions
	return b
}

func readGoose(path string, mt int64) ([]model.Session, error) {
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`
		SELECT id, coalesce(name,''), coalesce(description,''), coalesce(working_dir,''),
			coalesce(updated_at, created_at, ''),
			coalesce(input_tokens,0), coalesce(output_tokens,0),
			coalesce(cache_read_tokens,0), coalesce(cache_write_tokens,0),
			coalesce(accumulated_input_tokens,0), coalesce(accumulated_output_tokens,0),
			coalesce(accumulated_cache_read_tokens,0), coalesce(accumulated_cache_write_tokens,0),
			coalesce(accumulated_cost,0), coalesce(model_config_json,''), archived_at
		FROM sessions`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Session
	byID := map[string]int{}
	for rows.Next() {
		var id, name, desc, cwd, updated, modelJSON string
		var inTok, outTok, cacheR, cacheW int
		var accIn, accOut, accCR, accCW int
		var cost float64
		var archived sql.NullString
		if err := rows.Scan(&id, &name, &desc, &cwd, &updated, &inTok, &outTok, &cacheR, &cacheW,
			&accIn, &accOut, &accCR, &accCW, &cost, &modelJSON, &archived); err != nil {
			return nil, err
		}
		id = strings.TrimSpace(id)
		if id == "" || strings.TrimSpace(archived.String) != "" {
			continue
		}
		if inTok == 0 {
			inTok = accIn
		}
		if outTok == 0 {
			outTok = accOut
		}
		if cacheR == 0 {
			cacheR = accCR
		}
		if cacheW == 0 {
			cacheW = accCW
		}
		title := strings.TrimSpace(name)
		if title == "" {
			title = strings.TrimSpace(desc)
		}
		if title == "" {
			title = id
		}
		s := model.Session{
			ID: model.ID("goose", id), NativeID: id, Agent: "goose",
			Title: title, Summary: strings.TrimSpace(desc), CWD: cwd,
			Model: gooseModel(modelJSON), Updated: parseTime(updated),
			SourcePath: path, SourceMtime: mt,
			CanDelete: true, DeleteMode: "exec",
			Usage: model.Usage{Input: inTok, Output: outTok, CacheRead: cacheR, CacheWrite: cacheW, CostUSD: cost},
		}
		byID[id] = len(out)
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	_ = fillGooseSnippets(db, out, byID)
	return out, nil
}

func gooseModel(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var m struct {
		ModelName string `json:"model_name"`
	}
	if json.Unmarshal([]byte(raw), &m) == nil {
		return m.ModelName
	}
	return ""
}

func fillGooseSnippets(db *sql.DB, sessions []model.Session, byID map[string]int) error {
	rows, err := db.Query(`
		SELECT session_id, role, substr(coalesce(content_json,''), 1, 4000)
		FROM messages
		ORDER BY session_id, id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	bufs := map[string]*snippetBuf{}
	for rows.Next() {
		var id, role, raw string
		if err := rows.Scan(&id, &role, &raw); err != nil {
			return err
		}
		buf := bufs[id]
		if buf == nil {
			buf = &snippetBuf{}
			bufs[id] = buf
		}
		buf.add(role, gooseText(raw), "")
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for id, buf := range bufs {
		i, ok := byID[id]
		if !ok {
			continue
		}
		sessions[i].Snippets = buf.snippets()
		sessions[i].Messages = buf.n
		if sessions[i].Title == id {
			if title := snippetTitle(buf.snippets()); title != "" {
				sessions[i].Title = title
			}
		}
	}
	return nil
}

func gooseText(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var v any
	if json.Unmarshal([]byte(raw), &v) != nil {
		return raw
	}
	return contentText(v)
}

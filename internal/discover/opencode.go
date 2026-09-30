package discover

import (
	"database/sql"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AymanZahran/air9s/internal/model"
	_ "modernc.org/sqlite"
)

func opencodeDB() string {
	if v := strings.TrimSpace(os.Getenv("OPENCODE_DB")); v != "" {
		return v
	}
	data := strings.TrimSpace(os.Getenv("XDG_DATA_HOME"))
	if data == "" {
		data = homeJoin(".local", "share")
	}
	if data == "" {
		return ""
	}
	return filepath.Join(data, "opencode", "opencode.db")
}

func scanOpenCode(fresh func(string, int64) bool) Batch {
	path := opencodeDB()
	b := Batch{Agent: "opencode"}
	if path == "" {
		return b
	}
	st, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return b
		}
		b.Err = err
		return b
	}
	mt := st.ModTime().Unix()
	if fresh != nil && fresh(path, mt) {
		b.Files = []File{{Path: path, Mtime: mt, Fresh: true}}
		return b
	}
	sessions, err := readOpenCode(path, mt)
	if err != nil {
		b.Err = err
		return b
	}
	b.Files = []File{{Path: path, Mtime: mt, Fresh: false}}
	b.Sessions = sessions
	return b
}

func readOpenCode(path string, mt int64) ([]model.Session, error) {
	u := url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro&_pragma=busy_timeout(3000)"}
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT id, directory, title, model, time_updated, time_created,
		cost, tokens_input, tokens_output, tokens_reasoning, tokens_cache_read, tokens_cache_write FROM session`)
	withUsage := true
	if err != nil {
		withUsage = false
		rows, err = db.Query(`SELECT id, directory, title, model, time_updated, time_created FROM session`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Session
	for rows.Next() {
		var id, directory, title, modelRaw sql.NullString
		var updated, created sql.NullInt64
		var cost sql.NullFloat64
		var inTok, outTok, reasonTok, cacheRead, cacheWrite sql.NullInt64
		if withUsage {
			err = rows.Scan(&id, &directory, &title, &modelRaw, &updated, &created,
				&cost, &inTok, &outTok, &reasonTok, &cacheRead, &cacheWrite)
		} else {
			err = rows.Scan(&id, &directory, &title, &modelRaw, &updated, &created)
		}
		if err != nil {
			return nil, err
		}
		if id.String == "" {
			continue
		}
		s := model.Session{
			ID: model.ID("opencode", id.String), NativeID: id.String, Agent: "opencode",
			Title: strings.TrimSpace(title.String), CWD: directory.String,
			Model:      parseOpenCodeModel(modelRaw.String),
			SourcePath: path, SourceMtime: mt,
			CanDelete: true, DeleteMode: "exec",
		}
		when := updated.Int64
		if when == 0 {
			when = created.Int64
		}
		if when > 0 {
			s.Updated = time.UnixMilli(when)
		} else {
			s.Updated = time.Unix(mt, 0)
		}
		n, snippets, modelName := openCodeSnippets(db, id.String)
		s.Messages = n
		s.Snippets = snippets
		if s.Model == "" {
			s.Model = modelName
		}
		if s.Title == "" {
			s.Title = snippetTitle(snippets)
		}
		if s.Title == "" {
			s.Title = id.String
		}
		s.Usage = model.Usage{
			Input:      int(inTok.Int64),
			Output:     int(outTok.Int64),
			Reasoning:  int(reasonTok.Int64),
			CacheRead:  int(cacheRead.Int64),
			CacheWrite: int(cacheWrite.Int64),
			CostUSD:    cost.Float64,
			Context:    openCodeContext(db, id.String),
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func parseOpenCodeModel(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return ""
	}
	if strings.HasPrefix(raw, "{") {
		var m struct {
			ProviderID string `json:"providerID"`
			ModelID    string `json:"modelID"`
		}
		if json.Unmarshal([]byte(raw), &m) == nil && m.ModelID != "" {
			if m.ProviderID != "" {
				return m.ProviderID + "/" + m.ModelID
			}
			return m.ModelID
		}
	}
	return raw
}

func openCodeSnippets(db *sql.DB, sessionID string) (int, []model.Snippet, string) {
	rows, err := db.Query(`
		SELECT m.id, m.data, p.data
		FROM message m
		LEFT JOIN part p ON p.message_id = m.id
		WHERE m.session_id = ?
		ORDER BY m.time_created, m.id, p.time_created, p.id`, sessionID)
	if err != nil {
		return 0, nil, ""
	}
	defer rows.Close()
	var buf snippetBuf
	var modelName string
	var cur string
	var curRole string
	var curText strings.Builder
	flush := func() {
		if cur == "" {
			return
		}
		role := curRole
		if role != "user" && role != "assistant" {
			role = "assistant"
		}
		buf.add(role, curText.String(), "")
		cur = ""
		curText.Reset()
	}
	for rows.Next() {
		var mid, mdata, pdata sql.NullString
		if err := rows.Scan(&mid, &mdata, &pdata); err != nil {
			break
		}
		var meta struct {
			Role  string `json:"role"`
			Model struct {
				ProviderID string `json:"providerID"`
				ModelID    string `json:"modelID"`
			} `json:"model"`
		}
		_ = json.Unmarshal([]byte(mdata.String), &meta)
		if cur != "" && mid.String != cur {
			flush()
		}
		if cur == "" {
			cur = mid.String
			curRole = meta.Role
			if modelName == "" {
				modelName = joinModel(meta.Model.ProviderID, meta.Model.ModelID)
			}
		}
		if pdata.Valid {
			var part struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			if json.Unmarshal([]byte(pdata.String), &part) == nil && part.Type == "text" && part.Text != "" {
				if curText.Len() > 0 {
					curText.WriteByte('\n')
				}
				curText.WriteString(part.Text)
			}
		}
	}
	flush()
	return buf.n, buf.snippets(), modelName
}

func openCodeContext(db *sql.DB, sessionID string) int {
	rows, err := db.Query(`SELECT data FROM message WHERE session_id = ? ORDER BY time_created DESC LIMIT 8`, sessionID)
	if err != nil {
		return 0
	}
	defer rows.Close()
	for rows.Next() {
		var data sql.NullString
		if err := rows.Scan(&data); err != nil || data.String == "" {
			continue
		}
		var meta struct {
			Tokens struct {
				Input int `json:"input"`
			} `json:"tokens"`
		}
		if json.Unmarshal([]byte(data.String), &meta) == nil && meta.Tokens.Input > 0 {
			return meta.Tokens.Input
		}
	}
	return 0
}

func joinModel(provider, id string) string {
	if id == "" {
		return ""
	}
	if provider == "" {
		return id
	}
	return provider + "/" + id
}

func snippetTitle(snips []model.Snippet) string {
	for _, sn := range snips {
		if sn.Role == "user" && sn.Body != "" {
			return clip(sn.Body, 140)
		}
	}
	return ""
}

package discover

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"

	"github.com/AymanZahran/air9s/internal/model"
)

func hermesHome() string {
	return envOr("HERMES_HOME", homeJoin(".hermes"))
}

func scanHermes(fresh func(string, int64) bool) Batch {
	b := Batch{Agent: "hermes"}
	home := hermesHome()
	if home == "" {
		return b
	}
	if st, err := os.Stat(home); err != nil || !st.IsDir() {
		return b
	}
	dbs := []struct{ path, profile string }{{filepath.Join(home, "state.db"), ""}}
	entries, _ := os.ReadDir(filepath.Join(home, "profiles"))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dbs = append(dbs, struct{ path, profile string }{
			filepath.Join(home, "profiles", e.Name(), "state.db"),
			e.Name(),
		})
	}
	var failed []string
	for _, db := range dbs {
		st, err := os.Stat(db.path)
		if err != nil || st.IsDir() {
			continue
		}
		mt, skip := stamp(db.path, fresh)
		if skip {
			b.Files = append(b.Files, File{Path: db.path, Mtime: mt, Fresh: true})
			continue
		}
		sessions, err := readHermes(db.path, db.profile, mt)
		if err != nil {
			failed = append(failed, filepath.Base(filepath.Dir(db.path))+": "+err.Error())
			b.Files = append(b.Files, File{Path: db.path, Mtime: mt, Fresh: true})
			continue
		}
		b.Files = append(b.Files, File{Path: db.path, Mtime: mt})
		b.Sessions = append(b.Sessions, sessions...)
	}
	if len(b.Sessions) == 0 && len(failed) > 0 && len(b.Files) == len(failed) {
		b.Err = errJoin(failed)
		b.Files = nil
	}
	return b
}

func readHermes(path, profile string, mt int64) ([]model.Session, error) {
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`
		SELECT id, coalesce(source,''), coalesce(model,''), coalesce(title,''),
			coalesce(cwd,''), coalesce(git_branch,''), coalesce(message_count,0),
			coalesce(input_tokens,0), coalesce(output_tokens,0),
			coalesce(cache_read_tokens,0), coalesce(cache_write_tokens,0),
			coalesce(reasoning_tokens,0), actual_cost_usd, estimated_cost_usd,
			started_at, last_activity_at, ended_at
		FROM sessions
		WHERE coalesce(hidden,0) = 0 AND coalesce(archived,0) = 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Session
	byID := map[string]*model.Session{}
	for rows.Next() {
		var id, source, modelName, title, cwd, branch string
		var messages, inTok, outTok, cacheR, cacheW, reason int
		var actual, estimated sql.NullFloat64
		var started, last, ended sql.NullFloat64
		if err := rows.Scan(&id, &source, &modelName, &title, &cwd, &branch, &messages,
			&inTok, &outTok, &cacheR, &cacheW, &reason, &actual, &estimated, &started, &last, &ended); err != nil {
			return nil, err
		}
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		native := id
		if profile != "" && profile != "default" {
			native = "p:" + profile + ":" + id
		}
		when := unixish(last.Float64)
		if when.IsZero() {
			when = unixish(ended.Float64)
		}
		if when.IsZero() {
			when = unixish(started.Float64)
		}
		cost := estimated.Float64
		if actual.Valid && actual.Float64 > 0 {
			cost = actual.Float64
		}
		s := model.Session{
			ID: model.ID("hermes", native), NativeID: native, Agent: "hermes",
			Title: strings.TrimSpace(title), Summary: source, CWD: cwd, Branch: branch,
			Model: modelName, Updated: when, Messages: messages,
			SourcePath: path, SourceMtime: mt,
			CanDelete: true, DeleteMode: "exec",
			Usage: model.Usage{
				Input: inTok, Output: outTok, CacheRead: cacheR, CacheWrite: cacheW,
				Reasoning: reason, CostUSD: cost,
			},
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		bare := out[i].NativeID
		if profile != "" && profile != "default" {
			bare = strings.TrimPrefix(out[i].NativeID, "p:"+profile+":")
		}
		byID[bare] = &out[i]
	}
	_ = fillHermesSnippets(db, byID)
	for i := range out {
		if out[i].Title == "" {
			out[i].Title = snippetTitle(out[i].Snippets)
		}
		if out[i].Title == "" {
			out[i].Title = out[i].NativeID
		}
	}
	return out, nil
}

func fillHermesSnippets(db *sql.DB, byID map[string]*model.Session) error {
	rows, err := db.Query(`
		SELECT session_id, role, substr(coalesce(content,''), 1, 2000)
		FROM messages
		WHERE role IN ('user', 'assistant') AND coalesce(active, 1) = 1
		ORDER BY session_id, id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	bufs := map[string]*snippetBuf{}
	for rows.Next() {
		var id, role, body string
		if err := rows.Scan(&id, &role, &body); err != nil {
			return err
		}
		buf := bufs[id]
		if buf == nil {
			buf = &snippetBuf{}
			bufs[id] = buf
		}
		buf.add(role, body, "")
	}
	for id, buf := range bufs {
		s := byID[id]
		if s == nil {
			continue
		}
		s.Snippets = buf.snippets()
		if buf.n > s.Messages {
			s.Messages = buf.n
		}
	}
	return rows.Err()
}

func errJoin(parts []string) error {
	return &scanError{msg: strings.Join(parts, "; ")}
}

type scanError struct{ msg string }

func (e *scanError) Error() string { return e.msg }

package discover

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AymanZahran/ai9s/internal/model"
)

func kiloDB() string {
	if v := strings.TrimSpace(os.Getenv("KILO_DB")); v != "" {
		return v
	}
	data := strings.TrimSpace(os.Getenv("XDG_DATA_HOME"))
	if data == "" {
		data = homeJoin(".local", "share")
	}
	if data == "" {
		return ""
	}
	return filepath.Join(data, "kilo", "kilo.db")
}

func scanKilo(fresh func(string, int64) bool) Batch {
	return scanSQLite("kilo", kiloDB(), fresh, readKilo)
}

func readKilo(path string, mt int64) ([]model.Session, error) {
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, withParent, withUsage, err := queryKilo(db)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type kiloHead struct {
		id, directory, title, modelRaw, parent string
		updated, created                       int64
		cost                                   float64
		inTok, outTok, reasonTok               int64
		cacheRead, cacheWrite                  int64
	}
	var heads []kiloHead
	for rows.Next() {
		var id, directory, title, modelRaw sql.NullString
		var parent sql.NullString
		var updated, created sql.NullInt64
		var cost sql.NullFloat64
		var inTok, outTok, reasonTok, cacheRead, cacheWrite sql.NullInt64
		var err error
		switch {
		case withParent && withUsage:
			err = rows.Scan(&id, &directory, &title, &modelRaw, &updated, &created, &parent,
				&cost, &inTok, &outTok, &reasonTok, &cacheRead, &cacheWrite)
		case withUsage:
			err = rows.Scan(&id, &directory, &title, &modelRaw, &updated, &created,
				&cost, &inTok, &outTok, &reasonTok, &cacheRead, &cacheWrite)
		case withParent:
			err = rows.Scan(&id, &directory, &title, &modelRaw, &updated, &created, &parent)
		default:
			err = rows.Scan(&id, &directory, &title, &modelRaw, &updated, &created)
		}
		if err != nil {
			return nil, err
		}
		native := strings.TrimSpace(id.String)
		if native == "" {
			continue
		}
		heads = append(heads, kiloHead{
			id: native, directory: directory.String, title: strings.TrimSpace(title.String),
			parent:   strings.TrimSpace(parent.String),
			modelRaw: modelRaw.String, updated: updated.Int64, created: created.Int64,
			cost: cost.Float64, inTok: inTok.Int64, outTok: outTok.Int64, reasonTok: reasonTok.Int64,
			cacheRead: cacheRead.Int64, cacheWrite: cacheWrite.Int64,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// openDB allows one connection. Snippet queries wait until these rows close.
	rows.Close()
	var out []model.Session
	for _, head := range heads {
		when := head.updated
		if when == 0 {
			when = head.created
		}
		updatedAt := time.Unix(mt, 0)
		if when > 0 {
			updatedAt = time.UnixMilli(when)
		}
		n, snippets, modelName := openCodeSnippets(db, head.id)
		name := head.title
		if name == "" {
			name = snippetTitle(snippets)
		}
		if name == "" {
			name = head.id
		}
		modelLabel := parseOpenCodeModel(head.modelRaw)
		if modelLabel == "" {
			modelLabel = modelName
		}
		parent := ""
		if head.parent != "" && head.parent != head.id {
			parent = model.ID("kilo", head.parent)
		}
		out = append(out, model.Session{
			ID: model.ID("kilo", head.id), NativeID: head.id, Agent: "kilo",
			Title: name, CWD: head.directory, Model: modelLabel,
			Updated: updatedAt, Messages: n, Snippets: snippets,
			ParentID: parent,
			Usage: model.Usage{
				Input:      int(head.inTok),
				Output:     int(head.outTok),
				Reasoning:  int(head.reasonTok),
				CacheRead:  int(head.cacheRead),
				CacheWrite: int(head.cacheWrite),
				CostUSD:    head.cost,
				Context:    openCodeContext(db, head.id),
			},
			SourcePath: path, SourceMtime: mt,
			CanDelete: true, DeleteMode: "exec",
		})
	}
	return out, nil
}

func queryKilo(db *sql.DB) (*sql.Rows, bool, bool, error) {
	rows, err := db.Query(`SELECT id, directory, title, model, time_updated, time_created, coalesce(parent_id, ''),
		cost, tokens_input, tokens_output, tokens_reasoning, tokens_cache_read, tokens_cache_write FROM session`)
	if err == nil {
		return rows, true, true, nil
	}
	rows, err = db.Query(`SELECT id, directory, title, model, time_updated, time_created,
		cost, tokens_input, tokens_output, tokens_reasoning, tokens_cache_read, tokens_cache_write FROM session`)
	if err == nil {
		return rows, false, true, nil
	}
	rows, err = db.Query(`SELECT id, directory, title, model, time_updated, time_created, coalesce(parent_id, '') FROM session`)
	if err == nil {
		return rows, true, false, nil
	}
	rows, err = db.Query(`SELECT id, directory, title, model, time_updated, time_created FROM session`)
	if err != nil {
		return nil, false, false, err
	}
	return rows, false, false, nil
}

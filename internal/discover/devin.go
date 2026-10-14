package discover

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AymanZahran/ai9s/internal/model"
)

// DEVIN_SESSIONS_DB is an ai9s override so a test does not open the live
// database. The CLI stores sessions under the XDG data directory.

func devinDB() string {
	if v := strings.TrimSpace(os.Getenv("DEVIN_SESSIONS_DB")); v != "" {
		return v
	}
	data := strings.TrimSpace(os.Getenv("XDG_DATA_HOME"))
	if data == "" {
		data = homeJoin(".local", "share")
	}
	if data == "" {
		return ""
	}
	return filepath.Join(data, "devin", "cli", "sessions.db")
}

func scanDevin(fresh func(string, int64) bool) Batch {
	return scanSQLite("devin", devinDB(), fresh, readDevin)
}

func readDevin(path string, mt int64) ([]model.Session, error) {
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT id, working_directory, coalesce(model, ''), coalesce(title, ''),
		created_at, last_activity_at FROM sessions WHERE coalesce(hidden, 0) = 0`)
	if err != nil {
		rows, err = db.Query(`SELECT id, working_directory, coalesce(model, ''), coalesce(title, ''),
			created_at, last_activity_at FROM sessions`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type devinHead struct {
		id, directory, modelName, title string
		created, updated                int64
	}
	var heads []devinHead
	for rows.Next() {
		var id, directory, modelName, title sql.NullString
		var created, updated sql.NullInt64
		if err := rows.Scan(&id, &directory, &modelName, &title, &created, &updated); err != nil {
			return nil, err
		}
		if strings.TrimSpace(id.String) == "" {
			continue
		}
		heads = append(heads, devinHead{
			id: strings.TrimSpace(id.String), directory: strings.TrimSpace(directory.String),
			modelName: strings.TrimSpace(modelName.String), title: strings.TrimSpace(title.String),
			created: created.Int64, updated: updated.Int64,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// openDB allows one connection. Message queries wait until these rows close.
	rows.Close()
	var out []model.Session
	for _, head := range heads {
		n, snippets, usage, err := devinMessages(db, head.id)
		if err != nil {
			return nil, err
		}
		when := unixish(float64(head.updated))
		if when.IsZero() {
			when = unixish(float64(head.created))
		}
		if when.IsZero() {
			when = time.Unix(mt, 0)
		}
		name := head.title
		if name == "" {
			name = snippetTitle(snippets)
		}
		if name == "" {
			name = head.id
		}
		out = append(out, model.Session{
			ID: model.ID("devin", head.id), NativeID: head.id, Agent: "devin",
			Title: name, CWD: head.directory, Model: head.modelName,
			Updated: when, Messages: n, Snippets: snippets, Usage: usage,
			SourcePath: path, SourceMtime: mt,
			CanDelete: true, DeleteMode: "exec",
		})
	}
	return out, nil
}

func devinMessages(db *sql.DB, sessionID string) (int, []model.Snippet, model.Usage, error) {
	rows, err := db.Query(`SELECT chat_message FROM message_nodes WHERE session_id = ? ORDER BY created_at, row_id`, sessionID)
	if err != nil {
		rows, err = db.Query(`SELECT chat_message FROM message_nodes WHERE session_id = ? ORDER BY created_at`, sessionID)
	}
	if err != nil {
		return 0, nil, model.Usage{}, err
	}
	defer rows.Close()
	var buf snippetBuf
	var usage model.Usage
	for rows.Next() {
		var raw sql.NullString
		if err := rows.Scan(&raw); err != nil {
			return 0, nil, model.Usage{}, err
		}
		if strings.TrimSpace(raw.String) == "" {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(raw.String), &obj) != nil {
			continue
		}
		role, text := nestedText(obj)
		buf.add(role, text, "")
		noteTurnUsage(&usage, nestedUsage(obj), true)
	}
	if err := rows.Err(); err != nil {
		return 0, nil, model.Usage{}, err
	}
	return buf.n, buf.snippets(), usage, nil
}

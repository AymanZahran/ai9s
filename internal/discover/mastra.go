package discover

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/AymanZahran/ai9s/internal/model"
)

const mastraDeleteReason = "MastraCode does not document a command that deletes one thread"

func mastraDB() string {
	if v := strings.TrimSpace(os.Getenv("MASTRA_DB_PATH")); v != "" {
		return v
	}
	dir := mastraAppDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "mastra.db")
}

func mastraAppDir() string {
	switch runtime.GOOS {
	case "darwin":
		return homeJoin("Library", "Application Support", "mastracode")
	case "windows":
		if v := strings.TrimSpace(os.Getenv("APPDATA")); v != "" {
			return filepath.Join(v, "mastracode")
		}
		return ""
	default:
		data := strings.TrimSpace(os.Getenv("XDG_DATA_HOME"))
		if data == "" {
			data = homeJoin(".local", "share")
		}
		if data == "" {
			return ""
		}
		return filepath.Join(data, "mastracode")
	}
}

func scanMastra(fresh func(string, int64) bool) Batch {
	return scanSQLite("mastracode", mastraDB(), fresh, readMastra)
}

func readMastra(path string, mt int64) ([]model.Session, error) {
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT id, title, metadata, createdAt, updatedAt FROM mastra_threads`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type mastraHead struct {
		id, title, meta  string
		created, updated any
	}
	var heads []mastraHead
	for rows.Next() {
		var id, title, meta sql.NullString
		var created, updated any
		if err := rows.Scan(&id, &title, &meta, &created, &updated); err != nil {
			return nil, err
		}
		native := strings.TrimSpace(id.String)
		if native == "" {
			continue
		}
		heads = append(heads, mastraHead{
			id: native, title: strings.TrimSpace(title.String), meta: meta.String,
			created: created, updated: updated,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// openDB allows one connection. Message queries wait until these rows close.
	rows.Close()
	var out []model.Session
	for _, head := range heads {
		n, snippets, err := mastraMessages(db, head.id)
		if err != nil {
			return nil, err
		}
		when := whenOf(head.updated)
		if when.IsZero() {
			when = whenOf(head.created)
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
		parent := mastraParent(head.meta)
		if parent == head.id {
			parent = ""
		}
		out = append(out, model.Session{
			ID: model.ID("mastracode", head.id), NativeID: head.id, Agent: "mastracode",
			Title: name, CWD: mastraCWD(head.meta),
			Updated: when, Messages: n, Snippets: snippets,
			SourcePath: path, SourceMtime: mt,
			CanDelete: false, DeleteReason: mastraDeleteReason,
			ParentID: parentOf("mastracode", parent),
		})
	}
	return out, nil
}

func mastraMessages(db *sql.DB, threadID string) (int, []model.Snippet, error) {
	rows, err := db.Query(`SELECT role, content FROM mastra_messages WHERE thread_id = ? ORDER BY createdAt`, threadID)
	if err != nil {
		rows, err = db.Query(`SELECT role, content FROM mastra_messages WHERE thread_id = ?`, threadID)
	}
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	var buf snippetBuf
	for rows.Next() {
		var role, content sql.NullString
		if err := rows.Scan(&role, &content); err != nil {
			return 0, nil, err
		}
		buf.add(strings.TrimSpace(role.String), mastraText(content.String), "")
	}
	if err := rows.Err(); err != nil {
		return 0, nil, err
	}
	return buf.n, buf.snippets(), nil
}

func mastraParent(meta string) string {
	m := mastraMeta(meta)
	if m == nil {
		return ""
	}
	for _, k := range []string{"parentThreadId", "parentId", "forkOf"} {
		if s := strings.TrimSpace(asString(m[k])); s != "" {
			return s
		}
	}
	return ""
}

func mastraCWD(meta string) string {
	m := mastraMeta(meta)
	if m == nil {
		return ""
	}
	for _, k := range []string{"cwd", "directory", "workingDirectory", "projectPath", "path"} {
		if s := strings.TrimSpace(asString(m[k])); s != "" {
			return s
		}
	}
	return ""
}

func mastraMeta(meta string) map[string]any {
	meta = strings.TrimSpace(meta)
	if meta == "" || meta == "null" {
		return nil
	}
	var m map[string]any
	if json.Unmarshal([]byte(meta), &m) != nil {
		return nil
	}
	return m
}

func mastraText(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.HasPrefix(raw, "{") && !strings.HasPrefix(raw, "[") {
		return raw
	}
	var v any
	if json.Unmarshal([]byte(raw), &v) != nil {
		return raw
	}
	if s := contentText(v); s != "" {
		return s
	}
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	if parts, ok := m["parts"]; ok {
		if s := contentText(parts); s != "" {
			return s
		}
	}
	return ""
}

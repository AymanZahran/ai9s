package discover

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AymanZahran/air9s/internal/model"
)

func scanCursor(fresh func(string, int64) bool) Batch {
	root := CursorProjects()
	b := Batch{Agent: "cursor"}
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return b
		}
		b.Err = err
		return b
	}
	dbPath := cursorStateDB()
	var db *sql.DB
	defer func() {
		if db != nil {
			db.Close()
		}
	}()
	b.Err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == "subagents" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".jsonl") || !strings.Contains(path, string(filepath.Separator)+"agent-transcripts"+string(filepath.Separator)) {
			return nil
		}
		if strings.Contains(path, string(filepath.Separator)+"subagents"+string(filepath.Separator)) {
			return nil
		}
		mt, isFresh := stamp(path, fresh, dbPath)
		b.Files = append(b.Files, File{Path: path, Mtime: mt, Fresh: isFresh})
		if isFresh {
			return nil
		}
		if s, ok := readCursor(path); ok {
			if db == nil && dbPath != "" {
				if st, err := os.Stat(dbPath); err == nil && !st.IsDir() {
					db, _ = openDB(dbPath)
				}
			}
			if db != nil {
				s.Usage = cursorUsage(db, s.NativeID)
			}
			s.SourceMtime = mt
			b.Sessions = append(b.Sessions, s)
		}
		return nil
	})
	return b
}

func readCursor(path string) (model.Session, bool) {
	native := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	parent := filepath.Base(filepath.Dir(path))
	if parent != "" && parent != "." && parent != "agent-transcripts" {
		native = parent
	}
	var buf snippetBuf
	err := walkJSONL(path, func(obj map[string]any) error {
		role := asString(obj["role"])
		if role != "user" && role != "assistant" {
			return nil
		}
		buf.add(role, messageText(obj), "")
		return nil
	})
	if err != nil || buf.n == 0 {
		return model.Session{}, false
	}
	title := buf.title
	if title == "" {
		title = native
	}
	return model.Session{
		ID: model.ID("cursor", native), NativeID: native, Agent: "cursor",
		Title: title, CWD: decodeDashedPath(cursorProjectName(path)),
		Updated: time.Unix(fileMtime(path), 0), Messages: buf.n,
		SourcePath: path, SourceMtime: fileMtime(path),
		CanDelete: true, DeleteMode: "file", Snippets: buf.snippets(),
	}, true
}

func cursorProjectName(path string) string {
	dir := filepath.Dir(path)
	for i := 0; i < 6; i++ {
		if filepath.Base(dir) == "agent-transcripts" {
			return filepath.Base(filepath.Dir(dir))
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

package discover

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AymanZahran/ai9s/internal/model"
)

func scanClaude(fresh func(string, int64) bool) Batch {
	dir := ClaudeProjects()
	b := Batch{Agent: "claude"}
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return b
		}
		b.Err = err
		return b
	}
	b.Err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		mt, isFresh := stamp(path, fresh)
		b.Files = append(b.Files, File{Path: path, Mtime: mt, Fresh: isFresh})
		if isFresh {
			return nil
		}
		if s, ok := readClaude(path); ok {
			b.Sessions = append(b.Sessions, s)
		}
		return nil
	})
	return b
}

func readClaude(path string) (model.Session, bool) {
	var buf snippetBuf
	var cwd, branch, modelName, sessionID string
	var updated time.Time
	var usage model.Usage
	err := walkJSONL(path, func(obj map[string]any) error {
		if ts := parseTime(asString(obj["timestamp"])); ts.After(updated) {
			updated = ts
		}
		if sessionID == "" {
			sessionID = asString(obj["sessionId"])
		}
		if cwd == "" {
			cwd = asString(obj["cwd"])
		}
		if branch == "" {
			branch = asString(obj["gitBranch"])
		}
		role := asString(obj["type"])
		if role == "cost-state" {
			noteClaudeCost(&usage, obj)
			return nil
		}
		if role != "user" && role != "assistant" {
			return nil
		}
		if m, ok := obj["message"].(map[string]any); ok {
			if modelName == "" {
				modelName = asString(m["model"])
			}
			noteClaudeUsage(&usage, m)
		}
		buf.add(role, messageText(obj), asString(obj["timestamp"]))
		return nil
	})
	if err != nil || buf.n == 0 && buf.title == "" {
		return model.Session{}, false
	}
	native, parent := claudeIDs(path, sessionID)
	if cwd == "" {
		cwd = decodeDashedPath(claudeProject(path))
	}
	if updated.IsZero() {
		updated = time.Unix(fileMtime(path), 0)
	}
	title := buf.title
	if title == "" {
		title = native
	}
	return model.Session{
		ID: model.ID("claude", native), NativeID: native, Agent: "claude",
		Title: title, CWD: cwd, Branch: branch, Model: modelName,
		Updated: updated, Messages: buf.n, SourcePath: path, SourceMtime: fileMtime(path),
		CanDelete: true, DeleteMode: "file", Usage: usage, Snippets: buf.snippets(),
		ParentID: parentOf("claude", parent),
	}, true
}

// claudeIDs returns the session id and, for a file under subagents/, the parent session id.
// The parent is the sessionId on the transcript, or the directory above subagents/.
func claudeIDs(path, sessionID string) (native, parent string) {
	native = strings.TrimSuffix(filepath.Base(path), ".jsonl")
	if filepath.Base(filepath.Dir(path)) != "subagents" {
		return native, ""
	}
	parent = strings.TrimSpace(sessionID)
	if parent == "" {
		parent = filepath.Base(filepath.Dir(filepath.Dir(path)))
	}
	if parent == native {
		parent = ""
	}
	return native, parent
}

func claudeProject(path string) string {
	dir := filepath.Dir(path)
	if filepath.Base(dir) == "subagents" {
		dir = filepath.Dir(filepath.Dir(dir))
	}
	return filepath.Base(dir)
}

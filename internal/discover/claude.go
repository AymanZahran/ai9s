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
			if d.Name() == "subagents" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".jsonl") || strings.Contains(path, string(filepath.Separator)+"subagents"+string(filepath.Separator)) {
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
	native := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	var buf snippetBuf
	var cwd, branch, modelName string
	var updated time.Time
	err := walkJSONL(path, func(obj map[string]any) error {
		if ts := parseTime(asString(obj["timestamp"])); ts.After(updated) {
			updated = ts
		}
		if cwd == "" {
			cwd = asString(obj["cwd"])
		}
		if branch == "" {
			branch = asString(obj["gitBranch"])
		}
		role := asString(obj["type"])
		if role != "user" && role != "assistant" {
			return nil
		}
		if m, ok := obj["message"].(map[string]any); ok {
			if modelName == "" {
				modelName = asString(m["model"])
			}
		}
		buf.add(role, messageText(obj), asString(obj["timestamp"]))
		return nil
	})
	if err != nil || buf.n == 0 && buf.title == "" {
		return model.Session{}, false
	}
	if cwd == "" {
		cwd = decodeDashedPath(filepath.Base(filepath.Dir(path)))
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
		CanDelete: true, DeleteMode: "file", Snippets: buf.snippets(),
	}, true
}

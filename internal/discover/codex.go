package discover

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AymanZahran/ai9s/internal/model"
)

func scanCodex(fresh func(string, int64) bool) Batch {
	root := CodexSessions()
	b := Batch{Agent: "codex"}
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return b
		}
		b.Err = err
		return b
	}
	b.Err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasPrefix(d.Name(), "rollout-") || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		mt, isFresh := stamp(path, fresh)
		b.Files = append(b.Files, File{Path: path, Mtime: mt, Fresh: isFresh})
		if isFresh {
			return nil
		}
		if s, ok := readCodex(path); ok {
			b.Sessions = append(b.Sessions, s)
		}
		return nil
	})
	return b
}

func readCodex(path string) (model.Session, bool) {
	var buf snippetBuf
	var native, cwd, modelName string
	var updated time.Time
	err := walkJSONL(path, func(obj map[string]any) error {
		if ts := parseTime(asString(obj["timestamp"])); ts.After(updated) {
			updated = ts
		}
		pl, _ := obj["payload"].(map[string]any)
		if pl == nil {
			return nil
		}
		switch asString(obj["type"]) {
		case "session_meta":
			native = firstNonEmpty(asString(pl["session_id"]), asString(pl["id"]))
			cwd = asString(pl["cwd"])
			if m := asString(pl["model"]); m != "" {
				modelName = m
			}
		case "turn_context":
			if m := asString(pl["model"]); m != "" {
				modelName = m
			}
		case "response_item":
			if asString(pl["type"]) != "message" {
				return nil
			}
			role := asString(pl["role"])
			if role == "developer" || role == "system" {
				return nil
			}
			if role != "user" && role != "assistant" {
				role = "assistant"
			}
			buf.add(role, contentText(pl["content"]), asString(obj["timestamp"]))
		}
		return nil
	})
	if err != nil || native == "" {
		return model.Session{}, false
	}
	if buf.title == "" && buf.n == 0 {
		return model.Session{}, false
	}
	title := buf.title
	if title == "" {
		title = native
	}
	if updated.IsZero() {
		updated = time.Unix(fileMtime(path), 0)
	}
	return model.Session{
		ID: model.ID("codex", native), NativeID: native, Agent: "codex",
		Title: title, CWD: cwd, Model: modelName, Updated: updated, Messages: buf.n,
		SourcePath: path, SourceMtime: fileMtime(path), CanDelete: true, DeleteMode: "file", Snippets: buf.snippets(),
	}, true
}

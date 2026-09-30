package discover

import (
	"os"
	"path/filepath"
	"time"

	"github.com/AymanZahran/air9s/internal/model"
)

func scanGrok(fresh func(string, int64) bool) Batch {
	root := GrokSessions()
	b := Batch{Agent: "grok"}
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return b
		}
		b.Err = err
		return b
	}
	b.Err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "summary.json" {
			return nil
		}
		hist := filepath.Join(filepath.Dir(path), "chat_history.jsonl")
		mt, isFresh := stamp(path, fresh, hist)
		b.Files = append(b.Files, File{Path: path, Mtime: mt, Fresh: isFresh})
		if isFresh {
			return nil
		}
		if s, ok := readGrok(path); ok {
			s.SourceMtime = mt
			b.Sessions = append(b.Sessions, s)
		}
		return nil
	})
	return b
}

func readGrok(summaryPath string) (model.Session, bool) {
	var doc map[string]any
	if err := readJSON(summaryPath, &doc); err != nil {
		return model.Session{}, false
	}
	info, _ := doc["info"].(map[string]any)
	native := asString(info["id"])
	if native == "" {
		native = asString(doc["agent_id"])
	}
	if native == "" {
		return model.Session{}, false
	}
	cwd := asString(info["cwd"])
	title := asString(doc["session_summary"])
	modelName := asString(doc["current_model_id"])
	branch := asString(doc["head_branch"])
	updated := parseTime(asString(doc["updated_at"]))
	if updated.IsZero() {
		updated = parseTime(asString(doc["created_at"]))
	}
	var buf snippetBuf
	hist := filepath.Join(filepath.Dir(summaryPath), "chat_history.jsonl")
	_ = walkJSONL(hist, func(obj map[string]any) error {
		role := asString(obj["type"])
		if role == "system" {
			return nil
		}
		if role != "user" && role != "assistant" {
			role = "assistant"
		}
		body := asString(obj["content"])
		if body == "" {
			body = messageText(obj)
		}
		buf.add(role, body, "")
		return nil
	})
	if title == "" {
		title = buf.title
	}
	if title == "" {
		title = native
	}
	msgs := buf.n
	if n, ok := doc["num_chat_messages"].(float64); ok && int(n) > msgs {
		msgs = int(n)
	}
	if updated.IsZero() {
		updated = time.Unix(fileMtime(summaryPath), 0)
	}
	return model.Session{
		ID: model.ID("grok", native), NativeID: native, Agent: "grok",
		Title: title, Summary: asString(doc["session_summary"]), CWD: cwd, Branch: branch,
		Model: modelName, Updated: updated, Messages: msgs,
		Usage:      model.Usage{Effort: asString(doc["reasoning_effort"])},
		SourcePath: summaryPath, SourceMtime: fileMtime(summaryPath),
		CanDelete:    false,
		DeleteReason: "Grok keeps a search index and active-session state beside the transcript. Delete it from Grok's own session list.",
		Snippets:     buf.snippets(),
	}, true
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

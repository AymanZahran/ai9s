package discover

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AymanZahran/air9s/internal/model"
)

func geminiRoot() string {
	return envOr("GEMINI_HOME", homeJoin(".gemini"))
}

func scanGemini(fresh func(string, int64) bool) Batch {
	root := filepath.Join(geminiRoot(), "tmp")
	b := Batch{Agent: "gemini"}
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return b
		}
		b.Err = err
		return b
	}
	projects := loadGeminiProjects(geminiRoot())
	b.Err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasPrefix(d.Name(), "session-") || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}
		if filepath.Base(filepath.Dir(path)) != "chats" {
			return nil
		}
		mt, isFresh := stamp(path, fresh)
		b.Files = append(b.Files, File{Path: path, Mtime: mt, Fresh: isFresh})
		if isFresh {
			return nil
		}
		cwd := projects[filepath.Base(filepath.Dir(filepath.Dir(path)))]
		if s, ok := readGemini(path, cwd); ok {
			b.Sessions = append(b.Sessions, s)
		}
		return nil
	})
	return b
}

func loadGeminiProjects(root string) map[string]string {
	out := map[string]string{}
	b, err := os.ReadFile(filepath.Join(root, "projects.json"))
	if err != nil {
		return out
	}
	var doc struct {
		Projects map[string]string `json:"projects"`
	}
	if json.Unmarshal(b, &doc) != nil {
		return out
	}
	for path, name := range doc.Projects {
		if path != "" && name != "" {
			out[name] = path
		}
	}
	return out
}

func readGemini(path, cwd string) (model.Session, bool) {
	f, err := os.Open(path)
	if err != nil {
		return model.Session{}, false
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return model.Session{}, false
	}
	var native, start, updated string
	var buf snippetBuf
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return model.Session{}, false
		}
		name, _ := key.(string)
		switch name {
		case "sessionId":
			if err := dec.Decode(&native); err != nil {
				return model.Session{}, false
			}
		case "startTime":
			if err := dec.Decode(&start); err != nil {
				return model.Session{}, false
			}
		case "lastUpdated":
			if err := dec.Decode(&updated); err != nil {
				return model.Session{}, false
			}
		case "messages":
			open, err := dec.Token()
			if err != nil {
				return model.Session{}, false
			}
			if open == nil {
				continue
			}
			if open != json.Delim('[') {
				return model.Session{}, false
			}
			for dec.More() {
				var m struct {
					Type      string          `json:"type"`
					Content   json.RawMessage `json:"content"`
					Timestamp string          `json:"timestamp"`
				}
				if err := dec.Decode(&m); err != nil {
					return model.Session{}, false
				}
				role := ""
				switch m.Type {
				case "user":
					role = "user"
				case "gemini", "model", "assistant", "error":
					role = "assistant"
				default:
					continue
				}
				buf.add(role, geminiContent(m.Content), m.Timestamp)
			}
			if _, err := dec.Token(); err != nil {
				return model.Session{}, false
			}
		default:
			var skip json.RawMessage
			if err := dec.Decode(&skip); err != nil {
				return model.Session{}, false
			}
		}
	}
	if native == "" {
		native = strings.TrimSuffix(strings.TrimPrefix(filepath.Base(path), "session-"), ".json")
	}
	if buf.n == 0 {
		return model.Session{}, false
	}
	title := buf.title
	if title == "" {
		title = native
	}
	when := parseTime(updated)
	if when.IsZero() {
		when = parseTime(start)
	}
	if when.IsZero() {
		when = time.Unix(fileMtime(path), 0)
	}
	return model.Session{
		ID: model.ID("gemini", native), NativeID: native, Agent: "gemini",
		Title: title, CWD: cwd, Updated: when, Messages: buf.n,
		SourcePath: path, SourceMtime: fileMtime(path),
		CanDelete:    false,
		DeleteReason: "Gemini keeps project caches and side files beside the chat. Remove it from Gemini's own session list.",
		Snippets:     buf.snippets(),
	}, true
}

func geminiContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var v any
	if json.Unmarshal(raw, &v) == nil {
		return contentText(v)
	}
	return ""
}

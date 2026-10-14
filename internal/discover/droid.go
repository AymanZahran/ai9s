package discover

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/AymanZahran/ai9s/internal/model"
)

// FACTORY_HOME is an ai9s override. The droid CLI does not document a
// session-root variable. The default is ~/.factory.

func factoryRoot() string {
	return envOr("FACTORY_HOME", homeJoin(".factory"))
}

func droidSessions() string {
	root := factoryRoot()
	if root == "" {
		return ""
	}
	return filepath.Join(root, "sessions")
}

func scanDroid(fresh func(string, int64) bool) Batch {
	b := Batch{Agent: "droid"}
	root := droidSessions()
	if root == "" {
		return b
	}
	seen := map[string]bool{}
	walkShallowJSONL(root, func(path string) {
		settings := strings.TrimSuffix(path, ".jsonl") + ".settings.json"
		mt, skip := stamp(path, fresh, settings)
		if mt == 0 {
			return
		}
		if skip {
			b.Files = append(b.Files, File{Path: path, Mtime: mt, Fresh: true})
			return
		}
		sess, err := readDroidFile(path, mt)
		if err != nil || sess.NativeID == "" || seen[sess.NativeID] {
			return
		}
		seen[sess.NativeID] = true
		b.Files = append(b.Files, File{Path: path, Mtime: mt, Fresh: false})
		b.Sessions = append(b.Sessions, sess)
	})
	return b
}

func readDroidFile(path string, mt int64) (model.Session, error) {
	var buf snippetBuf
	var native, title, cwd string
	var updated time.Time
	err := walkJSONL(path, func(obj map[string]any) error {
		switch asString(obj["type"]) {
		case "session_start":
			if id := strings.TrimSpace(asString(obj["id"])); id != "" {
				native = id
			}
			if s := strings.TrimSpace(asString(obj["title"])); s != "" {
				title = s
			}
			if s := strings.TrimSpace(asString(obj["cwd"])); s != "" {
				cwd = s
			}
			updated = later(updated, whenOf(obj["timestamp"]))
		case "message":
			role, text := nestedText(obj)
			buf.add(role, text, "")
			updated = later(updated, whenOf(obj["timestamp"]))
		}
		return nil
	})
	if err != nil {
		return model.Session{}, err
	}
	if native == "" {
		native = strings.TrimSuffix(filepath.Base(path), ".jsonl")
	}
	if title == "" {
		title = snippetTitle(buf.snippets())
	}
	if title == "" {
		title = native
	}
	if updated.IsZero() {
		updated = time.Unix(fileMtime(path), 0)
	}
	modelName, usage := readDroidSettings(strings.TrimSuffix(path, ".jsonl") + ".settings.json")
	return model.Session{
		ID: model.ID("droid", native), NativeID: native, Agent: "droid",
		Title: title, CWD: cwd, Model: modelName,
		Updated: updated, Messages: buf.n, Snippets: buf.snippets(), Usage: usage,
		SourcePath: path, SourceMtime: mt,
		CanDelete: true, DeleteMode: "file",
	}, nil
}

func readDroidSettings(path string) (string, model.Usage) {
	var raw map[string]any
	if readJSON(path, &raw) != nil {
		return "", model.Usage{}
	}
	modelName := strings.TrimSpace(asString(raw["model"]))
	tok, _ := raw["tokenUsage"].(map[string]any)
	if tok == nil {
		return modelName, model.Usage{}
	}
	usage := model.Usage{
		Input:      num(tok["inputTokens"]),
		Output:     num(tok["outputTokens"]),
		CacheRead:  num(tok["cacheReadTokens"]),
		CacheWrite: num(tok["cacheCreationTokens"]),
		Reasoning:  num(tok["thinkingTokens"]),
	}
	if usage.Input > 0 {
		usage.Context = usage.Input
	}
	if effort := strings.TrimSpace(asString(raw["reasoningEffort"])); effort != "" {
		usage.Effort = effort
	}
	return modelName, usage
}

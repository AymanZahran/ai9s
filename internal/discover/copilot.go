package discover

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AymanZahran/ai9s/internal/model"
)

func scanCopilotCLI(fresh func(string, int64) bool) Batch {
	root := CopilotState()
	b := Batch{Agent: "copilot"}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return b
		}
		b.Err = err
		return b
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		mt, isFresh := stamp(dir, fresh, filepath.Join(dir, "workspace.yaml"), filepath.Join(dir, "events.jsonl"))
		b.Files = append(b.Files, File{Path: dir, Mtime: mt, Fresh: isFresh})
		if isFresh {
			continue
		}
		if s, ok := readCopilotDir(dir, "copilot"); ok {
			s.SourceMtime = mt
			b.Sessions = append(b.Sessions, s)
		}
	}
	return b
}

func readCopilotDir(dir, agent string) (model.Session, bool) {
	meta := map[string]string{}
	if b, err := os.ReadFile(filepath.Join(dir, "workspace.yaml")); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			k, v, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			meta[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	native := meta["id"]
	if native == "" {
		native = filepath.Base(dir)
	}
	var buf snippetBuf
	var updated time.Time
	var modelName string
	_ = walkJSONL(filepath.Join(dir, "events.jsonl"), func(obj map[string]any) error {
		if ts := parseTime(asString(obj["timestamp"])); ts.After(updated) {
			updated = ts
		}
		data, _ := obj["data"].(map[string]any)
		switch asString(obj["type"]) {
		case "session.start":
			if m := asString(data["selectedModel"]); m != "" {
				modelName = m
			}
			if ctx, ok := data["context"].(map[string]any); ok && meta["cwd"] == "" {
				meta["cwd"] = asString(ctx["cwd"])
			}
		case "user.message":
			buf.add("user", copilotBody(data), asString(obj["timestamp"]))
		case "assistant.message":
			buf.add("assistant", copilotBody(data), asString(obj["timestamp"]))
		}
		return nil
	})
	title := meta["name"]
	if title == "" || strings.EqualFold(title, "ready to kick one?") {
		title = buf.title
	}
	if title == "" {
		title = native
	}
	if u := parseTime(meta["updated_at"]); u.After(updated) {
		updated = u
	}
	if updated.IsZero() {
		updated = time.Unix(fileMtime(dir), 0)
	}
	if buf.n == 0 && meta["name"] == "" {
		return model.Session{}, false
	}
	return model.Session{
		ID: model.ID(agent, native), NativeID: native, Agent: agent,
		Title: title, CWD: meta["cwd"], Model: modelName, Updated: updated, Messages: buf.n,
		SourcePath: dir, SourceMtime: fileMtime(dir), CanDelete: true, DeleteMode: "dir", Snippets: buf.snippets(),
	}, true
}

func copilotBody(data map[string]any) string {
	if data == nil {
		return ""
	}
	if s := contentText(data["content"]); s != "" {
		return s
	}
	return contentText(data["text"])
}

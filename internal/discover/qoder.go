package discover

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/AymanZahran/ai9s/internal/model"
)

func qoderRoot() string {
	return envOr("QODER_CONFIG_DIR", homeJoin(".qoder"))
}

func qoderProjects() string {
	root := qoderRoot()
	if root == "" {
		return ""
	}
	return filepath.Join(root, "projects")
}

func scanQoder(fresh func(string, int64) bool) Batch {
	b := Batch{Agent: "qodercli"}
	root := qoderProjects()
	if root == "" {
		return b
	}
	seen := map[string]bool{}
	walkShallowJSONL(root, func(path string) {
		mt, skip := stamp(path, fresh)
		if mt == 0 {
			return
		}
		if skip {
			b.Files = append(b.Files, File{Path: path, Mtime: mt, Fresh: true})
			return
		}
		sess, err := readQoderFile(path, mt)
		if err != nil || sess.NativeID == "" || seen[sess.NativeID] {
			return
		}
		seen[sess.NativeID] = true
		b.Files = append(b.Files, File{Path: path, Mtime: mt, Fresh: false})
		b.Sessions = append(b.Sessions, sess)
	})
	return b
}

func readQoderFile(path string, mt int64) (model.Session, error) {
	var buf snippetBuf
	var usage model.Usage
	var native, title, cwd, branch, modelName string
	var updated time.Time
	err := walkJSONL(path, func(obj map[string]any) error {
		if s := strings.TrimSpace(asString(obj["sessionId"])); s != "" {
			native = s
		}
		if s := strings.TrimSpace(asString(obj["cwd"])); s != "" {
			cwd = s
		}
		if s := strings.TrimSpace(asString(obj["gitBranch"])); s != "" {
			branch = s
		}
		if s := strings.TrimSpace(asString(obj["aiTitle"])); s != "" {
			title = s
		}
		if cwd == "" {
			cwd = firstDir(obj["directories"])
		}
		if cwd == "" {
			cwd = firstDir(obj["workspaceDirectories"])
		}
		typ := asString(obj["type"])
		if typ == "runtime-config" {
			if m := strings.TrimSpace(asString(obj["model"])); m != "" {
				modelName = m
			}
		}
		updated = later(updated, whenOf(obj["timestamp"]))
		if sidechain(obj) || (typ != "user" && typ != "assistant") {
			return nil
		}
		role, text := nestedText(obj)
		if role == "" {
			role = typ
		}
		buf.add(role, text, "")
		noteTurnUsage(&usage, nestedUsage(obj), false)
		if modelName == "" {
			if msg, ok := obj["message"].(map[string]any); ok {
				modelName = strings.TrimSpace(asString(msg["model"]))
			}
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
		updated = time.Unix(mt, 0)
	}
	return model.Session{
		ID: model.ID("qodercli", native), NativeID: native, Agent: "qodercli",
		Title: title, CWD: cwd, Branch: branch, Model: modelName,
		Updated: updated, Messages: buf.n, Snippets: buf.snippets(), Usage: usage,
		SourcePath: path, SourceMtime: mt,
		CanDelete: true, DeleteMode: "file",
	}, nil
}

func firstDir(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case []any:
		for _, item := range t {
			if s := firstDir(item); s != "" {
				return s
			}
		}
	case map[string]any:
		for _, k := range []string{"path", "directory", "cwd", "dir"} {
			if s := strings.TrimSpace(asString(t[k])); s != "" {
				return s
			}
		}
	}
	return ""
}

package discover

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AymanZahran/ai9s/internal/model"
)

// Pi and OMP are the same session layout. Both follow
// PI_CODING_AGENT_SESSION_DIR, then PI_CODING_AGENT_DIR. The defaults
// differ when those variables are unset.

func piAgentDir() string {
	return envOr("PI_CODING_AGENT_DIR", homeJoin(".pi", "agent"))
}

func piSessions() string {
	if v := strings.TrimSpace(os.Getenv("PI_CODING_AGENT_SESSION_DIR")); v != "" {
		return v
	}
	dir := piAgentDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "sessions")
}

func ompAgentDir() string {
	if v := strings.TrimSpace(os.Getenv("PI_CODING_AGENT_DIR")); v != "" {
		return v
	}
	return homeJoin(".omp", "agent")
}

func ompSessions() string {
	if v := strings.TrimSpace(os.Getenv("PI_CODING_AGENT_SESSION_DIR")); v != "" {
		return v
	}
	dir := ompAgentDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "sessions")
}

func scanPi(fresh func(string, int64) bool) Batch {
	return scanPiFamily("pi", piSessions(), fresh)
}

func scanOmp(fresh func(string, int64) bool) Batch {
	return scanPiFamily("omp", ompSessions(), fresh)
}

func scanPiFamily(agent, root string, fresh func(string, int64) bool) Batch {
	b := Batch{Agent: agent}
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
		sess, err := readPiFile(agent, path, mt)
		if err != nil || sess.NativeID == "" || seen[sess.NativeID] {
			return
		}
		seen[sess.NativeID] = true
		b.Files = append(b.Files, File{Path: path, Mtime: mt, Fresh: false})
		b.Sessions = append(b.Sessions, sess)
	})
	return b
}

func readPiFile(agent, path string, mt int64) (model.Session, error) {
	var buf snippetBuf
	var usage model.Usage
	var native, title, cwd, modelName string
	var updated time.Time
	err := walkJSONL(path, func(obj map[string]any) error {
		switch asString(obj["type"]) {
		case "session":
			if id := strings.TrimSpace(asString(obj["id"])); id != "" {
				native = id
			}
			if s := strings.TrimSpace(asString(obj["cwd"])); s != "" && cwd == "" {
				cwd = s
			}
			updated = later(updated, whenOf(obj["timestamp"]))
		case "title":
			if s := strings.TrimSpace(asString(obj["title"])); s != "" {
				title = s
			}
			updated = later(updated, whenOf(obj["updatedAt"]))
		case "model_change":
			if m := piModelName(obj); m != "" {
				modelName = m
			}
		case "message":
			role, text := nestedText(obj)
			buf.add(role, text, "")
			noteTurnUsage(&usage, nestedUsage(obj), true)
			if modelName == "" {
				if msg, ok := obj["message"].(map[string]any); ok {
					modelName = piModelName(msg)
				}
			}
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
		updated = time.Unix(mt, 0)
	}
	return model.Session{
		ID: model.ID(agent, native), NativeID: native, Agent: agent,
		Title: title, CWD: cwd, Model: modelName,
		Updated: updated, Messages: buf.n, Snippets: buf.snippets(), Usage: usage,
		SourcePath: path, SourceMtime: mt,
		CanDelete: true, DeleteMode: "file",
	}, nil
}

func piModelName(obj map[string]any) string {
	if obj == nil {
		return ""
	}
	if m := strings.TrimSpace(asString(obj["model"])); m != "" && !strings.HasPrefix(m, "{") {
		return m
	}
	provider := strings.TrimSpace(asString(obj["provider"]))
	id := strings.TrimSpace(asString(obj["modelId"]))
	if id == "" {
		id = strings.TrimSpace(asString(obj["modelID"]))
	}
	return joinModel(provider, id)
}

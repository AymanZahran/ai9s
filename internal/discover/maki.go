package discover

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AymanZahran/ai9s/internal/model"
)

// MAKI_STATE_DIR is an ai9s override. Otherwise a legacy ~/.maki directory
// is used when it exists, then $XDG_STATE_HOME/maki or ~/.local/state/maki.

func makiState() string {
	if v := strings.TrimSpace(os.Getenv("MAKI_STATE_DIR")); v != "" {
		return v
	}
	if legacy := homeJoin(".maki"); legacy != "" {
		if st, err := os.Stat(legacy); err == nil && st.IsDir() {
			return legacy
		}
	}
	state := strings.TrimSpace(os.Getenv("XDG_STATE_HOME"))
	if state == "" {
		state = homeJoin(".local", "state")
	}
	if state == "" {
		return ""
	}
	return filepath.Join(state, "maki")
}

func makiSessions() string {
	root := makiState()
	if root == "" {
		return ""
	}
	return filepath.Join(root, "sessions")
}

func scanMaki(fresh func(string, int64) bool) Batch {
	b := Batch{Agent: "maki"}
	root := makiSessions()
	if root == "" {
		return b
	}
	seen := map[string]bool{}
	for _, path := range listTopJSONL(root) {
		mt, skip := stamp(path, fresh)
		if mt == 0 {
			continue
		}
		if skip {
			b.Files = append(b.Files, File{Path: path, Mtime: mt, Fresh: true})
			continue
		}
		sess, err := readMakiFile(path, mt)
		if err != nil || sess.NativeID == "" || seen[sess.NativeID] {
			continue
		}
		seen[sess.NativeID] = true
		b.Files = append(b.Files, File{Path: path, Mtime: mt, Fresh: false})
		b.Sessions = append(b.Sessions, sess)
	}
	return b
}

func readMakiFile(path string, mt int64) (model.Session, error) {
	var buf snippetBuf
	var usage model.Usage
	var native, title, cwd, modelName string
	var updated time.Time
	var window int
	err := walkJSONL(path, func(obj map[string]any) error {
		switch asString(obj["t"]) {
		case "header":
			if id := strings.TrimSpace(asString(obj["id"])); id != "" {
				native = id
			}
			if s := strings.TrimSpace(asString(obj["model"])); s != "" {
				modelName = s
			}
			if s := strings.TrimSpace(asString(obj["cwd"])); s != "" {
				cwd = s
			}
			updated = later(updated, whenOf(obj["created_at"]))
		case "msg":
			msg, _ := obj["d"].(map[string]any)
			if msg == nil {
				return nil
			}
			role := strings.TrimSpace(asString(msg["role"]))
			buf.add(role, contentText(msg["content"]), "")
		case "meta":
			if s := strings.TrimSpace(asString(obj["title"])); s != "" {
				title = s
			}
			if n := num(obj["context_size"]); n > 0 {
				window = n
			}
			if tu, ok := obj["token_usage"].(map[string]any); ok {
				usage = makiUsage(tu)
			}
			updated = later(updated, whenOf(obj["updated_at"]))
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
	if window > 0 {
		usage.Window = window
	}
	return model.Session{
		ID: model.ID("maki", native), NativeID: native, Agent: "maki",
		Title: title, CWD: cwd, Model: modelName,
		Updated: updated, Messages: buf.n, Snippets: buf.snippets(), Usage: usage,
		SourcePath: path, SourceMtime: mt,
		CanDelete: true, DeleteMode: "exec",
	}, nil
}

func makiUsage(tu map[string]any) model.Usage {
	usage := model.Usage{
		Input:      num(tu["input"]),
		Output:     num(tu["output"]),
		CacheRead:  num(tu["cache_read"]),
		CacheWrite: num(tu["cache_creation"]),
		CostUSD:    costOf(tu["cost"]),
	}
	if ctx := usage.Input + usage.CacheRead + usage.CacheWrite; ctx > 0 {
		usage.Context = ctx
	}
	return usage
}

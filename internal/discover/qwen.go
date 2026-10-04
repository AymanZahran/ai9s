package discover

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/AymanZahran/ai9s/internal/model"
)

// Qwen names a chat file with the session id. 32 to 36 hex characters,
// hyphens allowed, matching the CLI's session file pattern.
var qwenSessionFile = regexp.MustCompile(`^[0-9a-fA-F-]{32,36}\.jsonl$`)

func qwenRoot() string {
	if v := strings.TrimSpace(os.Getenv("QWEN_RUNTIME_DIR")); v != "" {
		return v
	}
	return envOr("QWEN_HOME", homeJoin(".qwen"))
}

func scanQwen(fresh func(string, int64) bool) Batch {
	b := Batch{Agent: "qwen"}
	root := qwenRoot()
	if root == "" {
		return b
	}
	projects := filepath.Join(root, "projects")
	entries, err := os.ReadDir(projects)
	if err != nil {
		return b
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		chats := filepath.Join(projects, entry.Name(), "chats")
		readQwenChats(&b, chats, fresh, seen)
		readQwenChats(&b, filepath.Join(chats, "archive"), fresh, seen)
	}
	return b
}

func readQwenChats(b *Batch, dir string, fresh func(string, int64) bool, seen map[string]bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !qwenSessionFile.MatchString(entry.Name()) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if !regularFile(path) {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".jsonl")
		if seen[id] {
			continue
		}
		mt, skip := stamp(path, fresh)
		if skip {
			b.Files = append(b.Files, File{Path: path, Mtime: mt, Fresh: true})
			seen[id] = true
			continue
		}
		sess, child, err := readQwenFile(path, id, mt)
		if child {
			seen[id] = true
			continue
		}
		if err != nil {
			continue
		}
		seen[id] = true
		b.Files = append(b.Files, File{Path: path, Mtime: mt, Fresh: false})
		b.Sessions = append(b.Sessions, sess)
	}
}

func readQwenFile(path, id string, mt int64) (model.Session, bool, error) {
	var buf snippetBuf
	var usage model.Usage
	var cwd, branch, modelName, custom, lastTS string
	var window int
	child := false
	err := walkJSONL(path, func(obj map[string]any) error {
		typ := asString(obj["type"])
		sub := asString(obj["subtype"])
		if typ == "system" && sub == "parent_session" {
			child = true
			return nil
		}
		if cwd == "" {
			cwd = strings.TrimSpace(asString(obj["cwd"]))
		}
		if branch == "" {
			branch = strings.TrimSpace(asString(obj["gitBranch"]))
		}
		if name := strings.TrimSpace(asString(obj["model"])); name != "" {
			modelName = name
		}
		if w := num(obj["contextWindowSize"]); w > 0 {
			window = w
		}
		if sub == "custom_title" {
			if title := qwenCustomTitle(obj); title != "" {
				custom = title
			}
		}
		qwenAddUsage(obj, &usage)
		if (typ == "user" || typ == "assistant") && sub == "" {
			buf.add(typ, qwenText(obj), asString(obj["timestamp"]))
			if ts := asString(obj["timestamp"]); ts != "" {
				lastTS = ts
			}
		}
		return nil
	})
	if child {
		return model.Session{}, true, nil
	}
	if err != nil {
		return model.Session{}, false, err
	}
	title := custom
	if title == "" {
		title = snippetTitle(buf.snippets())
	}
	if title == "" {
		title = id
	}
	updated := parseTime(lastTS)
	if updated.IsZero() {
		updated = unixish(float64(mt))
	}
	if window > 0 {
		usage.Window = window
	}
	return model.Session{
		ID: model.ID("qwen", id), NativeID: id, Agent: "qwen",
		Title: title, CWD: cwd, Branch: branch, Model: modelName,
		Updated: updated, Messages: buf.n, Snippets: buf.snippets(), Usage: usage,
		SourcePath: path, SourceMtime: mt,
		CanDelete: true, DeleteMode: "qwen",
	}, false, nil
}

func qwenCustomTitle(obj map[string]any) string {
	if title := strings.TrimSpace(asString(obj["customTitle"])); title != "" {
		return title
	}
	payload, _ := obj["systemPayload"].(map[string]any)
	if payload == nil {
		return ""
	}
	return strings.TrimSpace(asString(payload["customTitle"]))
}

func qwenText(obj map[string]any) string {
	if s := messageText(obj); s != "" {
		return s
	}
	msg, _ := obj["message"].(map[string]any)
	if msg == nil {
		return ""
	}
	return contentText(msg["parts"])
}

func qwenAddUsage(obj map[string]any, usage *model.Usage) {
	meta, _ := obj["usageMetadata"].(map[string]any)
	if meta == nil {
		return
	}
	if n := num(meta["promptTokenCount"]); n > 0 {
		usage.Context = n
	}
	if asString(obj["type"]) == "assistant" {
		usage.Output += num(meta["candidatesTokenCount"])
		usage.Reasoning += num(meta["thoughtsTokenCount"])
	}
	if n := num(meta["cachedContentTokenCount"]); n > 0 {
		usage.CacheRead = n
	}
}

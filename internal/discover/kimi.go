package discover

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/AymanZahran/ai9s/internal/model"
)

func kimiHome() string {
	return envOr("KIMI_CODE_HOME", homeJoin(".kimi-code"))
}

func kimiSessions() string {
	home := kimiHome()
	if home == "" {
		return ""
	}
	return filepath.Join(home, "sessions")
}

func scanKimi(fresh func(string, int64) bool) Batch {
	b := Batch{Agent: "kimi"}
	home := kimiHome()
	sessions := kimiSessions()
	if home == "" || sessions == "" {
		return b
	}
	seen := map[string]bool{}
	readKimiIndex(&b, home, sessions, fresh, seen)
	walkKimiSessions(&b, sessions, fresh, seen)
	return b
}

func readKimiIndex(b *Batch, home, sessions string, fresh func(string, int64) bool, seen map[string]bool) {
	path := filepath.Join(home, "session_index.jsonl")
	if !regularFile(path) {
		return
	}
	_ = walkJSONL(path, func(obj map[string]any) error {
		id := strings.TrimSpace(asString(obj["sessionId"]))
		dir := kimiIndexedDir(home, sessions, asString(obj["sessionDir"]))
		if id == "" || dir == "" || seen[id] {
			return nil
		}
		work := strings.TrimSpace(asString(obj["workDir"]))
		if s, ok := loadKimi(dir, id, work, fresh); ok {
			seen[id] = true
			seen[s.id] = true
			seen[filepath.Base(dir)] = true
			b.Files = append(b.Files, s.files...)
			if s.session != nil {
				b.Sessions = append(b.Sessions, *s.session)
			}
		}
		return nil
	})
}

func walkKimiSessions(b *Batch, sessions string, fresh func(string, int64) bool, seen map[string]bool) {
	buckets, err := os.ReadDir(sessions)
	if err != nil {
		return
	}
	for _, bucket := range buckets {
		if !bucket.IsDir() {
			continue
		}
		bucketPath := filepath.Join(sessions, bucket.Name())
		ids, err := os.ReadDir(bucketPath)
		if err != nil {
			continue
		}
		for _, id := range ids {
			if !id.IsDir() {
				continue
			}
			dir := filepath.Join(bucketPath, id.Name())
			if !withinDir(sessions, dir) {
				continue
			}
			native := id.Name()
			if seen[native] {
				continue
			}
			if s, ok := loadKimi(dir, native, "", fresh); ok {
				seen[s.id] = true
				seen[native] = true
				b.Files = append(b.Files, s.files...)
				if s.session != nil {
					b.Sessions = append(b.Sessions, *s.session)
				}
			}
		}
	}
}

type kimiLoaded struct {
	id      string
	files   []File
	session *model.Session
}

func loadKimi(dir, fallbackID, workDir string, fresh func(string, int64) bool) (kimiLoaded, bool) {
	statePath := filepath.Join(dir, "state.json")
	if !regularFile(statePath) {
		return kimiLoaded{}, false
	}
	wire := filepath.Join(dir, "agents", "main", "wire.jsonl")
	extra := []string{}
	if regularFile(wire) {
		extra = append(extra, wire)
	}
	mt, skip := stamp(statePath, fresh, extra...)
	files := []File{{Path: statePath, Mtime: mt, Fresh: skip}}
	if len(extra) > 0 {
		files = append(files, File{Path: wire, Mtime: fileMtime(wire), Fresh: skip})
	}
	var raw map[string]any
	if err := readJSON(statePath, &raw); err != nil {
		return kimiLoaded{}, false
	}
	id := strings.TrimSpace(asString(raw["id"]))
	if id == "" {
		id = fallbackID
	}
	if id == "" {
		return kimiLoaded{}, false
	}
	out := kimiLoaded{id: id, files: files}
	if skip {
		return out, true
	}
	var buf snippetBuf
	var context int
	if regularFile(wire) {
		_ = walkJSONL(wire, func(obj map[string]any) error {
			switch asString(obj["type"]) {
			case "context.append_message":
				msg, _ := obj["message"].(map[string]any)
				if msg == nil {
					return nil
				}
				role := asString(msg["role"])
				text := messageText(map[string]any{"message": msg})
				if text == "" {
					text = contentText(msg["parts"])
				}
				buf.add(role, text, "")
			case "token_counting.measured":
				if n := num(obj["tokens"]); n > 0 {
					context = n
				}
			}
			return nil
		})
	}
	title := strings.TrimSpace(asString(raw["title"]))
	if title == "" {
		title = strings.TrimSpace(asString(raw["lastPrompt"]))
	}
	if title == "" {
		title = snippetTitle(buf.snippets())
	}
	if title == "" {
		title = id
	}
	cwd := strings.TrimSpace(asString(raw["cwd"]))
	if cwd == "" {
		cwd = workDir
	}
	updated := unixish(asFloat(raw["updatedAt"]))
	if updated.IsZero() {
		updated = unixish(asFloat(raw["createdAt"]))
	}
	if updated.IsZero() {
		updated = unixish(float64(mt))
	}
	usage := model.Usage{}
	if context > 0 {
		usage.Context = context
	}
	sess := model.Session{
		ID: model.ID("kimi", id), NativeID: id, Agent: "kimi",
		Title: title, CWD: cwd, Updated: updated,
		Messages: buf.n, Snippets: buf.snippets(), Usage: usage,
		SourcePath: statePath, SourceMtime: mt,
		CanDelete: true, DeleteMode: "kimi",
	}
	out.session = &sess
	return out, true
}

func kimiIndexedDir(home, sessions, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !filepath.IsAbs(raw) {
		raw = filepath.Join(home, raw)
	}
	raw = filepath.Clean(raw)
	if !withinDir(sessions, raw) {
		return ""
	}
	return raw
}

func regularFile(path string) bool {
	st, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return st.Mode().IsRegular()
}

func kimiIndexPath() string {
	home := kimiHome()
	if home == "" {
		return ""
	}
	return filepath.Join(home, "session_index.jsonl")
}

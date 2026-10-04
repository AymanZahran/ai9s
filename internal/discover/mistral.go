package discover

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/AymanZahran/ai9s/internal/model"
)

func vibeHome() string {
	return envOr("VIBE_HOME", homeJoin(".vibe"))
}

func vibeSessions() string {
	home := vibeHome()
	if home == "" {
		return ""
	}
	return vibeSaveDir(home)
}

func scanMistral(fresh func(string, int64) bool) Batch {
	b := Batch{Agent: "mistral"}
	root := vibeSessions()
	if root == "" || !safeDataDir(root) {
		return b
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return b
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		if !withinDir(root, dir) || filepath.Dir(dir) != filepath.Clean(root) {
			continue
		}
		meta := filepath.Join(dir, "meta.json")
		msgs := filepath.Join(dir, "messages.jsonl")
		if !regularFile(meta) || !regularFile(msgs) {
			continue
		}
		mt, skip := stamp(msgs, fresh, meta)
		b.Files = append(b.Files, File{Path: msgs, Mtime: mt, Fresh: skip})
		b.Files = append(b.Files, File{Path: meta, Mtime: fileMtime(meta), Fresh: skip})
		if skip {
			continue
		}
		sess, ok := readMistral(meta, msgs, mt)
		if !ok {
			continue
		}
		b.Sessions = append(b.Sessions, sess)
	}
	return b
}

func readMistral(metaPath, msgsPath string, mt int64) (model.Session, bool) {
	var meta map[string]any
	if err := readJSON(metaPath, &meta); err != nil {
		return model.Session{}, false
	}
	if strings.TrimSpace(asString(meta["parent_session_id"])) != "" {
		return model.Session{}, false
	}
	id := strings.TrimSpace(asString(meta["session_id"]))
	if id == "" {
		return model.Session{}, false
	}
	var buf snippetBuf
	var lastTS string
	_ = walkJSONL(msgsPath, func(obj map[string]any) error {
		role := asString(obj["role"])
		text := messageText(obj)
		if text == "" {
			text = strings.TrimSpace(asString(obj["user_display_content"]))
		}
		ts := asString(obj["timestamp"])
		buf.add(role, text, ts)
		if ts != "" {
			lastTS = ts
		}
		return nil
	})
	title := strings.TrimSpace(asString(meta["title"]))
	if title == "" {
		title = snippetTitle(buf.snippets())
	}
	if title == "" {
		title = id
	}
	cwd := mistralCWD(meta)
	branch := strings.TrimSpace(asString(meta["git_branch"]))
	modelName := mistralModel(meta)
	updated := parseTime(asString(meta["bumped_at"]))
	if updated.IsZero() {
		updated = parseTime(asString(meta["end_time"]))
	}
	if updated.IsZero() {
		updated = parseTime(asString(meta["start_time"]))
	}
	if updated.IsZero() {
		updated = parseTime(lastTS)
	}
	if updated.IsZero() {
		updated = unixish(float64(mt))
	}
	return model.Session{
		ID: model.ID("mistral", id), NativeID: id, Agent: "mistral",
		Title: title, CWD: cwd, Branch: branch, Model: modelName,
		Updated: updated, Messages: buf.n, Snippets: buf.snippets(),
		SourcePath: msgsPath, SourceMtime: mt,
		CanDelete: true, DeleteMode: "mistral",
	}, true
}

func mistralCWD(meta map[string]any) string {
	if env, ok := meta["environment"].(map[string]any); ok {
		if cwd := strings.TrimSpace(asString(env["working_directory"])); cwd != "" {
			return cwd
		}
	}
	return strings.TrimSpace(asString(meta["origin_directory"]))
}

func mistralModel(meta map[string]any) string {
	cfg, _ := meta["config"].(map[string]any)
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(asString(cfg["active_model"]))
}

func vibeSaveDir(home string) string {
	def := filepath.Join(home, "logs", "session")
	body, err := os.ReadFile(filepath.Join(home, "config.toml"))
	if err != nil {
		return def
	}
	section := ""
	save := ""
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.Trim(line, "[]"))
			continue
		}
		if section != "session_logging" {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) != "save_dir" {
			continue
		}
		save = tomlString(val)
	}
	if save == "" {
		return def
	}
	if !filepath.IsAbs(save) {
		save = filepath.Join(home, save)
	}
	save = filepath.Clean(save)
	if !safeDataDir(save) {
		return def
	}
	return save
}

func tomlString(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') {
		q := v[0]
		if end := strings.IndexByte(v[1:], q); end >= 0 {
			return v[1 : 1+end]
		}
	}
	if i := strings.IndexByte(v, '#'); i >= 0 {
		v = strings.TrimSpace(v[:i])
	}
	return strings.Trim(v, `"'`)
}

func safeDataDir(dir string) bool {
	if dir == "" || dir == "." || dir == string(os.PathSeparator) {
		return false
	}
	return filepath.Dir(dir) != dir
}

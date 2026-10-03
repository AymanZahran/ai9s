package discover

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/AymanZahran/ai9s/internal/model"
)

func scanAider(fresh func(string, int64) bool) Batch {
	b := Batch{Agent: "aider"}
	for _, path := range aiderFiles() {
		st, err := os.Stat(path)
		if err != nil || st.IsDir() {
			continue
		}
		mt, skip := stamp(path, fresh)
		b.Files = append(b.Files, File{Path: path, Mtime: mt, Fresh: skip})
		if skip {
			continue
		}
		b.Sessions = append(b.Sessions, readAider(path, mt))
	}
	return b
}

func aiderFiles() []string {
	var roots []string
	if v := strings.TrimSpace(os.Getenv("AIDER_CHAT_ROOTS")); v != "" {
		roots = append(roots, strings.Split(v, string(os.PathListSeparator))...)
	}
	if v := strings.TrimSpace(os.Getenv("AIDER_HOME")); v != "" {
		roots = append(roots, v)
	} else if home := homeJoin(".aider"); home != "" {
		roots = append(roots, home)
	}
	if os.Getenv("AIDER_SCAN_HOME") == "1" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			roots = append(roots, home)
		}
	}
	seen := map[string]bool{}
	var out []string
	add := func(path string) {
		path = filepath.Clean(path)
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		out = append(out, path)
	}
	if v := strings.TrimSpace(os.Getenv("AIDER_CHAT_HISTORY")); v != "" {
		add(v)
	}
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		st, err := os.Stat(root)
		if err != nil {
			continue
		}
		if !st.IsDir() {
			if filepath.Base(root) == ".aider.chat.history.md" {
				add(root)
			}
			continue
		}
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				base := d.Name()
				if path != root && skipDir(base) {
					return filepath.SkipDir
				}
				if depth(root, path) > 6 {
					return filepath.SkipDir
				}
				return nil
			}
			if d.Name() == ".aider.chat.history.md" {
				add(path)
			}
			return nil
		})
	}
	return out
}

func skipDir(name string) bool {
	switch name {
	case "node_modules", ".git", "Library", "vendor", "dist", ".cache", "caches", "target", ".venv", "venv", ".Trash":
		return true
	default:
		return false
	}
}

func depth(root, path string) int {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return 0
	}
	return strings.Count(rel, string(os.PathSeparator))
}

func readAider(path string, mt int64) model.Session {
	id := path
	s := model.Session{
		ID: model.ID("aider", id), NativeID: id, Agent: "aider",
		Title: filepath.Base(filepath.Dir(path)), CWD: filepath.Dir(path),
		Updated: unixish(float64(mt)), SourcePath: path, SourceMtime: mt,
		CanDelete: true, DeleteMode: "aider",
	}
	f, err := os.Open(path)
	if err != nil {
		return s
	}
	defer f.Close()
	if info, err := f.Stat(); err == nil {
		s.Updated = info.ModTime()
	}
	var buf snippetBuf
	role := ""
	var body strings.Builder
	flush := func() {
		buf.add(role, body.String(), "")
		body.Reset()
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		trim := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trim, "#### "):
			flush()
			role = aiderRole(strings.TrimPrefix(trim, "#### "))
		case strings.HasPrefix(trim, "> ") && role == "":
			role = "user"
			body.WriteString(strings.TrimPrefix(trim, "> "))
			body.WriteByte('\n')
		default:
			if role == "" {
				continue
			}
			body.WriteString(line)
			body.WriteByte('\n')
		}
	}
	flush()
	s.Snippets = buf.snippets()
	s.Messages = buf.n
	if title := snippetTitle(s.Snippets); title != "" {
		s.Title = title
	}
	return s
}

func aiderRole(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if strings.HasPrefix(s, "user") || strings.HasPrefix(s, "human") {
		return "user"
	}
	return "assistant"
}

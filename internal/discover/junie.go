package discover

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/AymanZahran/ai9s/internal/model"
)

func junieHome() string {
	return envOr("JUNIE_HOME", homeJoin(".junie"))
}

func scanJunie(fresh func(string, int64) bool) Batch {
	b := Batch{Agent: "junie"}
	root := filepath.Join(junieHome(), "sessions")
	entries, err := os.ReadDir(root)
	if err != nil {
		return b
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "session-") {
			continue
		}
		path := filepath.Join(root, e.Name(), "transcript.md")
		st, err := os.Stat(path)
		if err != nil || st.IsDir() {
			continue
		}
		mt, skip := stamp(path, fresh)
		b.Files = append(b.Files, File{Path: path, Mtime: mt, Fresh: skip})
		if skip {
			continue
		}
		s := readJunie(path, e.Name(), mt)
		b.Sessions = append(b.Sessions, s)
	}
	return b
}

func readJunie(path, id string, mt int64) model.Session {
	s := model.Session{
		ID: model.ID("junie", id), NativeID: id, Agent: "junie",
		Title: id, Updated: unixish(float64(mt)),
		SourcePath: path, SourceMtime: mt,
		CanDelete: true, DeleteMode: "dir",
	}
	f, err := os.Open(path)
	if err != nil {
		return s
	}
	defer f.Close()
	var buf snippetBuf
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line == "# Session transcript" {
			continue
		}
		role := "assistant"
		body := line
		switch {
		case strings.HasPrefix(strings.ToLower(line), "user:"):
			role, body = "user", strings.TrimSpace(line[len("user:"):])
		case strings.HasPrefix(strings.ToLower(line), "assistant:"):
			role, body = "assistant", strings.TrimSpace(line[len("assistant:"):])
		case strings.HasPrefix(line, "#"):
			role, body = "user", strings.TrimSpace(strings.TrimLeft(line, "#"))
		}
		buf.add(role, body, "")
	}
	s.Snippets = buf.snippets()
	s.Messages = buf.n
	if title := snippetTitle(s.Snippets); title != "" && title != "Session transcript" {
		s.Title = title
	}
	if info, err := os.Stat(path); err == nil {
		s.Updated = info.ModTime()
	}
	return s
}

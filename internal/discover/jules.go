package discover

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/AymanZahran/ai9s/internal/model"
)

func julesHome() string {
	return envOr("JULES_HOME", homeJoin(".jules"))
}

func scanJules(fresh func(string, int64) bool) Batch {
	b := Batch{Agent: "jules"}
	if sessions, files := readJulesLocal(fresh); len(files) > 0 || len(sessions) > 0 {
		b.Sessions = append(b.Sessions, sessions...)
		b.Files = append(b.Files, files...)
	}
	// Remote listing is opt-in. `jules remote list` talks to Google and can sit
	// silent until it is killed, which would stall every index.
	if os.Getenv("AI9S_JULES_REMOTE") != "1" {
		return b
	}
	remote, err := scanJulesRemote(fresh)
	if err != nil {
		if len(b.Files) == 0 {
			b.Err = err
		}
		return b
	}
	if len(remote.Sessions) == 0 && len(remote.Files) == 0 {
		return b
	}
	return remote
}

func readJulesLocal(fresh func(string, int64) bool) ([]model.Session, []File) {
	home := julesHome()
	if home == "" {
		return nil, nil
	}
	var sessions []model.Session
	var files []File
	for _, name := range []string{"sessions.json", "sessions.txt"} {
		path := filepath.Join(home, name)
		st, err := os.Stat(path)
		if err != nil || st.IsDir() {
			continue
		}
		mt, skip := stamp(path, fresh)
		files = append(files, File{Path: path, Mtime: mt, Fresh: skip})
		if skip {
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		sessions = append(sessions, parseJulesList(string(body), path, mt)...)
	}
	return sessions, files
}

func scanJulesRemote(fresh func(string, int64) bool) (Batch, error) {
	b := Batch{Agent: "jules"}
	bin, err := exec.LookPath("jules")
	if err != nil {
		return b, nil
	}
	const source = "jules:remote"
	bucket := time.Now().Unix() / 900 * 900
	if fresh != nil && fresh(source, bucket) {
		b.Files = []File{{Path: source, Mtime: bucket, Fresh: true}}
		return b, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "remote", "list", "--session")
	cmd.Env = append(os.Environ(), "CI=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return b, &scanError{msg: "jules remote list: " + clip(msg, 180)}
	}
	b.Sessions = parseJulesList(stdout.String(), source, bucket)
	b.Files = []File{{Path: source, Mtime: bucket}}
	return b, nil
}

var julesID = regexp.MustCompile(`^[0-9]{3,}$`)

func parseJulesList(raw, source string, mt int64) []model.Session {
	raw = stripANSI(raw)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if strings.HasPrefix(raw, "[") || strings.HasPrefix(raw, "{") {
		if sessions := parseJulesJSON(raw, source, mt); len(sessions) > 0 {
			return sessions
		}
	}
	var out []model.Session
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "ID") || strings.HasPrefix(strings.ToLower(line), "session") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 || !julesID.MatchString(fields[0]) {
			continue
		}
		id := fields[0]
		title := id
		cwd := ""
		if len(fields) > 1 && strings.Contains(fields[1], "/") {
			cwd = fields[1]
		}
		if len(fields) > 1 {
			rest := strings.TrimSpace(strings.TrimPrefix(line, id))
			if rest != "" {
				title = clip(rest, 140)
			}
		}
		out = append(out, model.Session{
			ID: model.ID("jules", id), NativeID: id, Agent: "jules",
			Title: title, CWD: cwd, Updated: time.Unix(mt, 0),
			SourcePath: source, SourceMtime: mt,
			CanDelete: true, DeleteMode: "jules",
		})
	}
	return out
}

func parseJulesJSON(raw, source string, mt int64) []model.Session {
	var rows []map[string]any
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		var wrap struct {
			Sessions []map[string]any `json:"sessions"`
		}
		if err := json.Unmarshal([]byte(raw), &wrap); err != nil {
			return nil
		}
		rows = wrap.Sessions
	}
	var out []model.Session
	for _, row := range rows {
		id := firstString(row, "id", "session", "sessionId", "name")
		id = strings.TrimPrefix(id, "sessions/")
		if id == "" {
			continue
		}
		title := firstString(row, "title", "prompt", "task", "summary")
		if title == "" {
			title = id
		}
		when := parseTime(firstString(row, "updateTime", "updated", "createTime", "created"))
		if when.IsZero() {
			when = time.Unix(mt, 0)
		}
		out = append(out, model.Session{
			ID: model.ID("jules", id), NativeID: id, Agent: "jules",
			Title: clip(title, 140), CWD: firstString(row, "repo", "repository", "cwd"),
			Updated: when, SourcePath: source, SourceMtime: mt,
			CanDelete: true, DeleteMode: "jules",
		})
	}
	return out
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != 0x1b {
			b.WriteByte(s[i])
			continue
		}
		j := i + 1
		for j < len(s) && ((s[j] >= '0' && s[j] <= '9') || s[j] == '[' || s[j] == ';' || s[j] == '?') {
			j++
		}
		if j < len(s) {
			j++
		}
		i = j - 1
	}
	return b.String()
}

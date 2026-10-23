package discover

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/AymanZahran/ai9s/internal/model"
)

func scanCodex(fresh func(string, int64) bool) Batch {
	root := CodexSessions()
	b := Batch{Agent: "codex"}
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return b
		}
		b.Err = err
		return b
	}
	b.Err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasPrefix(d.Name(), "rollout-") || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		mt, isFresh := stamp(path, fresh)
		b.Files = append(b.Files, File{Path: path, Mtime: mt, Fresh: isFresh})
		if isFresh {
			return nil
		}
		if s, ok := readCodex(path); ok {
			b.Sessions = append(b.Sessions, s)
		}
		return nil
	})
	if b.Err != nil {
		return b
	}
	// thread_spawn_edges lives beside the rollouts. Read it on every scan
	// so a fresh rollout still keeps its parent. A missing database means
	// there are no edges. An unreadable one leaves the stored links alone.
	if parents, ok := codexParents(); ok {
		b.Parents = parents
	}
	return b
}

func codexStatePath() string {
	home := envOr("CODEX_HOME", homeJoin(".codex"))
	if home == "" {
		return ""
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		return ""
	}
	best := ""
	bestN := -1
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "state_") || !strings.HasSuffix(name, ".sqlite") {
			continue
		}
		mid := strings.TrimSuffix(strings.TrimPrefix(name, "state_"), ".sqlite")
		n, err := strconv.Atoi(mid)
		if err != nil || n < bestN {
			continue
		}
		bestN = n
		best = filepath.Join(home, name)
	}
	return best
}

func codexParents() (map[string]string, bool) {
	path := codexStatePath()
	if path == "" {
		return map[string]string{}, true
	}
	db, err := openDB(path)
	if err != nil {
		return nil, false
	}
	defer db.Close()
	rows, err := db.Query(`SELECT parent_thread_id, child_thread_id FROM thread_spawn_edges`)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return map[string]string{}, true
		}
		return nil, false
	}
	defer rows.Close()
	parents := map[string]string{}
	for rows.Next() {
		var parent, child string
		if err := rows.Scan(&parent, &child); err != nil {
			return nil, false
		}
		noteParent(parents, "codex", child, parent)
	}
	if err := rows.Err(); err != nil {
		return nil, false
	}
	return parents, true
}

func readCodex(path string) (model.Session, bool) {
	var buf snippetBuf
	var native, cwd, modelName string
	var updated time.Time
	var usage model.Usage
	err := walkJSONL(path, func(obj map[string]any) error {
		if ts := parseTime(asString(obj["timestamp"])); ts.After(updated) {
			updated = ts
		}
		pl, _ := obj["payload"].(map[string]any)
		if pl == nil {
			return nil
		}
		switch asString(obj["type"]) {
		case "session_meta":
			native = firstNonEmpty(asString(pl["session_id"]), asString(pl["id"]))
			cwd = asString(pl["cwd"])
			if m := asString(pl["model"]); m != "" {
				modelName = m
			}
		case "turn_context":
			if m := asString(pl["model"]); m != "" {
				modelName = m
			}
		case "event_msg":
			if asString(pl["type"]) == "token_count" {
				if info, ok := pl["info"].(map[string]any); ok {
					noteCodexTokens(&usage, info)
				}
			}
		case "response_item":
			if asString(pl["type"]) != "message" {
				return nil
			}
			role := asString(pl["role"])
			if role == "developer" || role == "system" {
				return nil
			}
			if role != "user" && role != "assistant" {
				role = "assistant"
			}
			buf.add(role, contentText(pl["content"]), asString(obj["timestamp"]))
		}
		return nil
	})
	if err != nil || native == "" {
		return model.Session{}, false
	}
	if buf.title == "" && buf.n == 0 {
		return model.Session{}, false
	}
	title := buf.title
	if title == "" {
		title = native
	}
	if updated.IsZero() {
		updated = time.Unix(fileMtime(path), 0)
	}
	return model.Session{
		ID: model.ID("codex", native), NativeID: native, Agent: "codex",
		Title: title, CWD: cwd, Model: modelName, Updated: updated, Messages: buf.n,
		SourcePath: path, SourceMtime: fileMtime(path), CanDelete: true, DeleteMode: "file",
		Usage: usage, Snippets: buf.snippets(),
	}, true
}

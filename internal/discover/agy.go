package discover

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/AymanZahran/ai9s/internal/model"
)

func agyHistoryPath() string {
	return filepath.Join(geminiRoot(), "antigravity-cli", "history.jsonl")
}

func scanAgy(fresh func(string, int64) bool) Batch {
	path := agyHistoryPath()
	b := Batch{Agent: "antigravity"}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return b
		}
		b.Err = err
		return b
	}
	mt, isFresh := stamp(path, fresh, agyConversationDBs(path)...)
	b.Files = []File{{Path: path, Mtime: mt, Fresh: isFresh}}
	if isFresh {
		return b
	}
	sessions, err := readAgy(path)
	if err != nil {
		b.Err = err
		return b
	}
	for i := range sessions {
		sessions[i].SourceMtime = mt
	}
	b.Sessions = sessions
	return b
}

type agyLine struct {
	Display        string `json:"display"`
	Timestamp      int64  `json:"timestamp"`
	Workspace      string `json:"workspace"`
	ConversationID string `json:"conversationId"`
	Type           string `json:"type"`
}

func readAgy(path string) ([]model.Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	type acc struct {
		buf     snippetBuf
		cwd     string
		updated time.Time
	}
	order := []string{}
	groups := map[string]*acc{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var row agyLine
		if err := json.Unmarshal(line, &row); err != nil || row.ConversationID == "" {
			continue
		}
		g := groups[row.ConversationID]
		if g == nil {
			g = &acc{}
			groups[row.ConversationID] = g
			order = append(order, row.ConversationID)
		}
		if row.Workspace != "" {
			g.cwd = row.Workspace
		}
		if row.Timestamp > 0 {
			ts := time.UnixMilli(row.Timestamp)
			if ts.After(g.updated) {
				g.updated = ts
			}
		}
		role := "assistant"
		switch row.Type {
		case "", "user", "slash_command":
			role = "user"
		}
		g.buf.add(role, row.Display, "")
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	out := make([]model.Session, 0, len(order))
	for _, id := range order {
		g := groups[id]
		if g.buf.n == 0 {
			continue
		}
		title := g.buf.title
		if title == "" {
			title = id
		}
		updated := g.updated
		if updated.IsZero() {
			updated = time.Unix(fileMtime(path), 0)
		}
		out = append(out, model.Session{
			ID: model.ID("antigravity", id), NativeID: id, Agent: "antigravity",
			Title: title, CWD: g.cwd, Updated: updated, Messages: g.buf.n,
			Usage:      agyUsage(path, id),
			SourcePath: path, CanDelete: true, DeleteMode: "rewrite",
			Snippets: g.buf.snippets(),
		})
	}
	return out, nil
}

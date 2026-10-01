package discover

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/AymanZahran/air9s/internal/model"
)

func clineHome() string {
	return envOr("CLINE_HOME", homeJoin(".cline"))
}

func scanCline(fresh func(string, int64) bool) Batch {
	b := Batch{Agent: "cline"}
	home := clineHome()
	if home == "" {
		return b
	}
	hist := filepath.Join(home, "data", "state", "taskHistory.json")
	st, err := os.Stat(hist)
	if err != nil || st.IsDir() {
		return b
	}
	extra := clineConversationPaths(home)
	mt, skip := stamp(hist, fresh, extra...)
	b.Files = []File{{Path: hist, Mtime: mt, Fresh: skip}}
	if skip {
		return b
	}
	sessions, err := readCline(home, hist, mt)
	if err != nil {
		b.Err = err
		return b
	}
	b.Sessions = sessions
	return b
}

func clineConversationPaths(home string) []string {
	entries, err := os.ReadDir(filepath.Join(home, "data", "tasks"))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(home, "data", "tasks", e.Name(), "api_conversation_history.json")
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			out = append(out, p)
		}
	}
	return out
}

func readCline(home, hist string, mt int64) ([]model.Session, error) {
	body, err := os.ReadFile(hist)
	if err != nil {
		return nil, err
	}
	var rows []map[string]any
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, err
	}
	var out []model.Session
	for _, row := range rows {
		id := firstString(row, "id", "ulid")
		if id == "" {
			continue
		}
		title := clip(firstString(row, "task"), 140)
		if title == "" {
			title = id
		}
		when := unixish(float64(num(row["ts"])))
		conv := filepath.Join(home, "data", "tasks", id, "api_conversation_history.json")
		snips, n := clineSnippets(conv)
		if n == 0 {
			n = num(row["size"])
		}
		if when.IsZero() {
			if st, err := os.Stat(conv); err == nil {
				when = st.ModTime()
			}
		}
		out = append(out, model.Session{
			ID: model.ID("cline", id), NativeID: id, Agent: "cline",
			Title: title, CWD: firstString(row, "cwdOnTaskInitialization", "cwd"),
			Model: firstString(row, "modelId", "model"), Updated: when, Messages: n,
			SourcePath: hist, SourceMtime: mt, Snippets: snips,
			CanDelete: true, DeleteMode: "cline",
			Usage: model.Usage{
				Input: num(row["tokensIn"]), Output: num(row["tokensOut"]),
				CacheRead: num(row["cacheReads"]), CacheWrite: num(row["cacheWrites"]),
				CostUSD: floatNum(row["totalCost"]),
			},
		})
	}
	return out, nil
}

func clineSnippets(path string) ([]model.Snippet, int) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, 0
	}
	var rows []map[string]any
	if json.Unmarshal(body, &rows) != nil {
		return nil, 0
	}
	var buf snippetBuf
	for _, row := range rows {
		role := firstString(row, "role")
		buf.add(role, contentText(row["content"]), "")
	}
	return buf.snippets(), buf.n
}

func floatNum(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case json.Number:
		f, _ := n.Float64()
		return f
	default:
		return 0
	}
}

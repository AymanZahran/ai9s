package discover

import (
	"os"
	"path/filepath"
	"time"

	"github.com/AymanZahran/air9s/internal/model"
)

func scanGrok(fresh func(string, int64) bool) Batch {
	root := GrokSessions()
	b := Batch{Agent: "grok"}
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return b
		}
		b.Err = err
		return b
	}
	b.Err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "summary.json" {
			return nil
		}
		dir := filepath.Dir(path)
		hist := filepath.Join(dir, "chat_history.jsonl")
		mt, isFresh := stamp(path, fresh, hist, filepath.Join(dir, "usage.json"), filepath.Join(dir, "signals.json"))
		b.Files = append(b.Files, File{Path: path, Mtime: mt, Fresh: isFresh})
		if isFresh {
			return nil
		}
		if s, ok := readGrok(path); ok {
			s.SourceMtime = mt
			b.Sessions = append(b.Sessions, s)
		}
		return nil
	})
	return b
}

func readGrok(summaryPath string) (model.Session, bool) {
	var doc map[string]any
	if err := readJSON(summaryPath, &doc); err != nil {
		return model.Session{}, false
	}
	info, _ := doc["info"].(map[string]any)
	native := asString(info["id"])
	if native == "" {
		native = asString(doc["agent_id"])
	}
	if native == "" {
		return model.Session{}, false
	}
	cwd := asString(info["cwd"])
	title := asString(doc["session_summary"])
	modelName := asString(doc["current_model_id"])
	branch := asString(doc["head_branch"])
	updated := parseTime(asString(doc["updated_at"]))
	if updated.IsZero() {
		updated = parseTime(asString(doc["created_at"]))
	}
	var buf snippetBuf
	hist := filepath.Join(filepath.Dir(summaryPath), "chat_history.jsonl")
	_ = walkJSONL(hist, func(obj map[string]any) error {
		role := asString(obj["type"])
		if role == "system" {
			return nil
		}
		if role != "user" && role != "assistant" {
			role = "assistant"
		}
		body := asString(obj["content"])
		if body == "" {
			body = messageText(obj)
		}
		buf.add(role, body, "")
		return nil
	})
	if title == "" {
		title = buf.title
	}
	if title == "" {
		title = native
	}
	msgs := buf.n
	if n, ok := doc["num_chat_messages"].(float64); ok && int(n) > msgs {
		msgs = int(n)
	}
	if updated.IsZero() {
		updated = time.Unix(fileMtime(summaryPath), 0)
	}
	return model.Session{
		ID: model.ID("grok", native), NativeID: native, Agent: "grok",
		Title: title, Summary: asString(doc["session_summary"]), CWD: cwd, Branch: branch,
		Model: modelName, Updated: updated, Messages: msgs,
		Usage:      grokUsage(filepath.Dir(summaryPath), asString(doc["reasoning_effort"])),
		SourcePath: summaryPath, SourceMtime: fileMtime(summaryPath),
		CanDelete: true, DeleteMode: "grok",
		Snippets: buf.snippets(),
	}, true
}

// grokUsage reads the siblings next to summary.json.
// signals.json holds the latest prompt size and the window.
// contextWindowUsage is a percent, so it is not a token count.
// usage.json session totals are the token columns. costUsdTicks is 1e10 ticks per USD.
func grokUsage(dir, effort string) model.Usage {
	u := model.Usage{Effort: effort}
	var usage map[string]any
	if readJSON(filepath.Join(dir, "usage.json"), &usage) == nil {
		sess, _ := usage["session"].(map[string]any)
		u.Input = asInt(sess["inputTokens"])
		u.Output = asInt(sess["outputTokens"])
		u.CacheRead = asInt(sess["cachedReadTokens"])
		u.CacheWrite = asInt(sess["cacheCreationTokens"])
		u.Reasoning = asInt(sess["reasoningTokens"])
		u.Total = asInt(sess["totalTokens"])
		if ticks := asFloat(sess["costUsdTicks"]); ticks > 0 {
			u.CostUSD = ticks / 1e10
		}
	}
	var signals map[string]any
	if readJSON(filepath.Join(dir, "signals.json"), &signals) == nil {
		u.Context = asInt(signals["contextTokensUsed"])
		u.Window = asInt(signals["contextWindowTokens"])
	}
	return u
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

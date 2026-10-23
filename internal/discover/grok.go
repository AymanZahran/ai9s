package discover

import (
	"os"
	"path/filepath"
	"time"

	"github.com/AymanZahran/ai9s/internal/model"
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
	// Spawn edges live on the parent's updates.jsonl, so collect them even
	// when summary.json itself has not changed. The summary parent is
	// applied after every spawn, so it wins when both name a parent.
	b.Parents = map[string]string{}
	spawned := map[string]string{}
	fromSummary := map[string]string{}
	b.Err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "summary.json" {
			return nil
		}
		dir := filepath.Dir(path)
		grokSpawnParents(filepath.Join(dir, "updates.jsonl"), spawned)
		childNative, parentNative := grokSummaryParent(path)
		noteParent(fromSummary, "grok", childNative, parentNative)
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
	for child, parent := range spawned {
		b.Parents[child] = parent
	}
	for child, parent := range fromSummary {
		b.Parents[child] = parent
	}
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
	parent := asString(doc["parent_session_id"])
	if parent == native {
		parent = ""
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
		ParentID: parentOf("grok", parent),
	}, true
}

func grokSummaryParent(path string) (child, parent string) {
	var doc map[string]any
	if readJSON(path, &doc) != nil {
		return "", ""
	}
	info, _ := doc["info"].(map[string]any)
	child = asString(info["id"])
	if child == "" {
		child = asString(doc["agent_id"])
	}
	parent = asString(doc["parent_session_id"])
	return child, parent
}

func grokSpawnParents(path string, parents map[string]string) {
	if !regularFile(path) {
		return
	}
	_ = walkJSONL(path, func(obj map[string]any) error {
		params, _ := obj["params"].(map[string]any)
		update, _ := params["update"].(map[string]any)
		if asString(update["sessionUpdate"]) != "subagent_spawned" {
			return nil
		}
		noteParent(parents, "grok", asString(update["child_session_id"]), asString(update["parent_session_id"]))
		return nil
	})
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

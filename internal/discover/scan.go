package discover

import (
	"fmt"

	"github.com/AymanZahran/ai9s/internal/model"
)

// File is one source the indexer uses for incremental updates.
type File struct {
	Path  string
	Mtime int64
	Fresh bool
}

// Batch is everything one agent scanner found.
type Batch struct {
	Agent    string
	Sessions []model.Session
	Files    []File
	Err      error
}

// Scanner walks one agent's local session store.
type Scanner struct {
	Agent string
	Scan  func(fresh func(path string, mtime int64) bool) Batch
}

// Scanners returns the built-in agents in display order.
func Scanners() []Scanner {
	return []Scanner{
		{"claude", scanClaude},
		{"codex", scanCodex},
		{"copilot", scanCopilotCLI},
		{"grok", scanGrok},
		{"antigravity", scanAgy},
		{"gemini", scanGemini},
		{"cursor", scanCursor},
		{"opencode", scanOpenCode},
		{"hermes", scanHermes},
		{"openclaw", scanOpenClaw},
		{"junie", scanJunie},
		{"jules", scanJules},
		{"goose", scanGoose},
		{"cline", scanCline},
		{"aider", scanAider},
		{"kiro", scanKiro},
	}
}

// Collect runs every scanner. A failure in one agent does not stop the others.
func Collect(fresh func(path string, mtime int64) bool) []Batch {
	scanners := Scanners()
	out := make([]Batch, 0, len(scanners))
	for _, sc := range scanners {
		out = append(out, safeScan(sc, fresh))
	}
	return out
}

func safeScan(sc Scanner, fresh func(string, int64) bool) (b Batch) {
	defer func() {
		if r := recover(); r != nil {
			b = Batch{Agent: sc.Agent, Err: fmt.Errorf("scanner failed: %v", r)}
		}
	}()
	b = sc.Scan(fresh)
	if b.Agent == "" {
		b.Agent = sc.Agent
	}
	return b
}

func stamp(path string, fresh func(string, int64) bool, extra ...string) (int64, bool) {
	mt := fileMtime(path)
	for _, p := range extra {
		if t := fileMtime(p); t > mt {
			mt = t
		}
	}
	if mt > 0 && fresh != nil && fresh(path, mt) {
		return mt, true
	}
	return mt, false
}

// Package model is the shared session record used by scanners, the index, and the UI.
package model

import "time"

// Session is one resumable agent conversation.
type Session struct {
	ID           string    `json:"id"`
	NativeID     string    `json:"native_id"`
	Agent        string    `json:"agent"`
	Title        string    `json:"title"`
	Name         string    `json:"name,omitempty"` // display name set in ai9s; Title stays the agent's own title
	Summary      string    `json:"summary,omitempty"`
	CWD          string    `json:"cwd,omitempty"`
	Branch       string    `json:"branch,omitempty"`
	Model        string    `json:"model,omitempty"`
	Updated      time.Time `json:"updated"`
	Messages     int       `json:"messages"`
	SourcePath   string    `json:"source_path,omitempty"`
	SourceMtime  int64     `json:"-"`
	CanDelete    bool      `json:"can_delete"`
	DeleteMode   string    `json:"delete_mode,omitempty"`
	DeleteReason string    `json:"delete_reason,omitempty"`
	Usage        Usage     `json:"usage,omitempty"`
	Snippets     []Snippet `json:"snippets,omitempty"`
}

// Usage is the accounting an agent wrote next to the transcript.
// Zero fields are left out. Context is the latest prompt size, not a sum of every turn.
type Usage struct {
	Input      int     `json:"input,omitempty"`
	Output     int     `json:"output,omitempty"`
	CacheRead  int     `json:"cache_read,omitempty"`
	CacheWrite int     `json:"cache_write,omitempty"`
	Reasoning  int     `json:"reasoning,omitempty"`
	Total      int     `json:"total,omitempty"`
	Context    int     `json:"context,omitempty"`
	Window     int     `json:"window,omitempty"`
	Requests   int     `json:"requests,omitempty"`
	CostUSD    float64 `json:"cost_usd,omitempty"`
	Effort     string  `json:"effort,omitempty"`
}

// Empty reports that no accounting was recorded.
func (u Usage) Empty() bool {
	return u.Input == 0 && u.Output == 0 && u.CacheRead == 0 && u.CacheWrite == 0 &&
		u.Reasoning == 0 && u.Total == 0 && u.Context == 0 && u.Window == 0 &&
		u.Requests == 0 && u.CostUSD == 0 && u.Effort == ""
}

// Snippet is a short piece of the transcript kept for preview and search.
type Snippet struct {
	Role string `json:"role"`
	Body string `json:"body"`
}

// ID builds the stable key stored in the index.
func ID(agent, native string) string {
	return agent + ":" + native
}

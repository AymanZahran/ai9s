// Package model is the shared session record used by scanners, the index, and the UI.
package model

import "time"

// Session is one resumable agent conversation.
type Session struct {
	ID           string    `json:"id"`
	NativeID     string    `json:"native_id"`
	Agent        string    `json:"agent"`
	Title        string    `json:"title"`
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
	Snippets     []Snippet `json:"snippets,omitempty"`
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

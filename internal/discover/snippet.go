package discover

import "github.com/AymanZahran/ai9s/internal/model"

// snippetBuf keeps the first and last user/assistant lines so search still
// sees the start of a long session and the latest turn.
type snippetBuf struct {
	first []model.Snippet
	last  []model.Snippet
	title string
	n     int
	lastT string
}

const (
	snippetKeep  = 40
	snippetRunes = 1600
)

func (b *snippetBuf) add(role, body, ts string) {
	body = clip(body, snippetRunes)
	if body == "" {
		return
	}
	if role != "user" && role != "assistant" {
		return
	}
	b.n++
	if ts != "" {
		b.lastT = ts
	}
	sn := model.Snippet{Role: role, Body: body}
	if role == "user" && b.title == "" {
		b.title = clip(body, 140)
	}
	if len(b.first) < snippetKeep {
		b.first = append(b.first, sn)
		return
	}
	b.last = append(b.last, sn)
	if len(b.last) > snippetKeep {
		b.last = b.last[1:]
	}
}

func (b *snippetBuf) snippets() []model.Snippet {
	if len(b.last) == 0 {
		return b.first
	}
	out := make([]model.Snippet, 0, len(b.first)+len(b.last))
	out = append(out, b.first...)
	// Drop overlap when the session is shorter than 2*snippetKeep.
	start := 0
	if len(b.first)+len(b.last) > b.n && b.n > 0 {
		start = len(b.first) + len(b.last) - b.n
		if start < 0 {
			start = 0
		}
	}
	if start < len(b.last) {
		out = append(out, b.last[start:]...)
	}
	return out
}

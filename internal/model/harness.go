package model

// harnessNames is the program name for each indexed harness id.
// The id stays the value stored in the index.
var harnessNames = [][2]string{
	{"claude", "Claude Code"},
	{"codex", "Codex"},
	{"copilot", "Copilot CLI"},
	{"grok", "Grok Build"},
	{"antigravity", "Antigravity"},
	{"gemini", "Gemini CLI"},
	{"cursor", "Cursor"},
	{"opencode", "OpenCode"},
	{"hermes", "Hermes"},
	{"openclaw", "OpenClaw"},
	{"junie", "Junie"},
	{"jules", "Jules"},
	{"goose", "Goose"},
	{"cline", "Cline"},
	{"aider", "Aider"},
	{"kiro", "Kiro"},
	{"kimi", "Kimi"},
	{"minimax", "MiniMax"},
	{"qwen", "Qwen"},
	{"mistral", "Mistral Vibe"},
}

// HarnessNames returns harness id and the name shown in the list, in roster order.
func HarnessNames() [][2]string {
	out := make([][2]string, len(harnessNames))
	copy(out, harnessNames)
	return out
}

// HarnessName is the name shown for a harness id. An unknown id is returned as given.
func HarnessName(id string) string {
	for _, pair := range harnessNames {
		if pair[0] == id {
			return pair[1]
		}
	}
	return id
}

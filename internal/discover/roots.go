package discover

import "path/filepath"

// Directory helpers honor the same environment variables the scanners use.
// AIR9S_CACHE_DIR is handled by the index, not here.

func ClaudeProjects() string {
	return filepath.Join(envOr("CLAUDE_CONFIG_DIR", homeJoin(".claude")), "projects")
}

func CodexSessions() string {
	return filepath.Join(envOr("CODEX_HOME", homeJoin(".codex")), "sessions")
}

func CopilotState() string {
	return filepath.Join(envOr("COPILOT_HOME", homeJoin(".copilot")), "session-state")
}

func GrokHome() string {
	return envOr("GROK_HOME", homeJoin(".grok"))
}

func GrokSessions() string {
	return filepath.Join(GrokHome(), "sessions")
}

func GeminiRoot() string { return geminiRoot() }

func JulesHome() string { return julesHome() }

func KiroDB() string { return kiroDB() }

func CursorProjects() string {
	return filepath.Join(envOr("CURSOR_HOME", homeJoin(".cursor")), "projects")
}

func AgyFile() string { return agyHistoryPath() }

func OpenCodeFile() string { return opencodeDB() }

func JunieSessions() string {
	return filepath.Join(junieHome(), "sessions")
}

func ClineHome() string { return clineHome() }

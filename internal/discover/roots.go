package discover

import "path/filepath"

// Directory helpers honor the same environment variables the scanners use.
// AI9S_CACHE_DIR is handled by the index, not here.

func ClaudeProjects() string {
	return filepath.Join(envOr("CLAUDE_CONFIG_DIR", homeJoin(".claude")), "projects")
}

func CodexSessions() string {
	return filepath.Join(envOr("CODEX_HOME", homeJoin(".codex")), "sessions")
}

func CopilotState() string {
	return filepath.Join(envOr("COPILOT_HOME", homeJoin(".copilot")), "session-state")
}

func GrokSessions() string {
	return filepath.Join(envOr("GROK_HOME", homeJoin(".grok")), "sessions")
}

func CursorProjects() string {
	return filepath.Join(envOr("CURSOR_HOME", homeJoin(".cursor")), "projects")
}

func AgyFile() string { return agyHistoryPath() }

func OpenCodeFile() string { return opencodeDB() }

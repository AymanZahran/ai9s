package act

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/AymanZahran/air9s/internal/discover"
	"github.com/AymanZahran/air9s/internal/model"
	_ "modernc.org/sqlite"
)

// LookPath resolves a resume or delete binary. Tests replace it.
var LookPath = exec.LookPath

// Command is a child process to run in a session's directory.
type Command struct {
	Name string
	Args []string
	Dir  string
}

func (c Command) String() string {
	parts := []string{quote(c.Name)}
	for _, a := range c.Args {
		parts = append(parts, quote(a))
	}
	line := strings.Join(parts, " ")
	if c.Dir != "" {
		return "cd " + quote(c.Dir) + " && " + line
	}
	return line
}

func quote(s string) string {
	if s == "" || strings.ContainsAny(s, " \t\n\"'\\$") {
		return strconv.Quote(s)
	}
	return s
}

// Run executes the command attached to the current terminal.
// On a terminal, the command gets the foreground so Ctrl-C reaches the
// agent and air9s is still there when the agent exits.
func (c Command) Run() error {
	cmd := exec.Command(c.Name, c.Args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if c.Dir != "" {
		cmd.Dir = c.Dir
	}
	return runAttached(cmd)
}

// Plan builds the agent's own resume command.
func Plan(s model.Session, yolo bool) (Command, error) {
	if err := userArg("session id", s.NativeID); err != nil {
		return Command{}, err
	}
	var name string
	var args []string
	switch s.Agent {
	case "claude":
		name = "claude"
		if yolo {
			args = append(args, "--dangerously-skip-permissions")
		}
		args = append(args, "--resume", s.NativeID)
	case "codex":
		name = "codex"
		args = []string{"resume", s.NativeID}
	case "grok":
		name = "grok"
		if yolo {
			args = append(args, "--always-approve")
		}
		args = append(args, "--resume", s.NativeID)
	case "copilot":
		name = "copilot"
		if yolo {
			args = append(args, "--allow-all-tools")
		}
		args = append(args, "--resume", s.NativeID)
	case "agy":
		name = "agy"
		if yolo {
			args = append(args, "--dangerously-skip-permissions")
		}
		args = append(args, "--conversation", s.NativeID)
	case "gemini":
		if s.SourcePath == "" {
			return Command{}, errors.New("gemini session file is missing")
		}
		if err := userArg("session file", s.SourcePath); err != nil {
			return Command{}, err
		}
		name = "gemini"
		args = []string{"--session-file", s.SourcePath}
	case "cursor":
		name = "cursor-agent"
		if _, err := LookPath(name); err != nil {
			name = "agent"
		}
		if yolo {
			args = append(args, "--force")
		}
		args = append(args, "--resume", s.NativeID)
	case "opencode":
		name = "opencode"
		args = []string{"--session", s.NativeID}
	case "hermes":
		name = "hermes"
		id := s.NativeID
		if profile, bare, ok := hermesProfile(s.NativeID); ok {
			if err := userArg("hermes profile", profile); err != nil {
				return Command{}, err
			}
			if err := userArg("session id", bare); err != nil {
				return Command{}, err
			}
			args = append(args, "-p", profile)
			id = bare
		}
		if yolo {
			args = append(args, "--yolo")
		}
		args = append(args, "--resume", id)
	case "openclaw":
		name = "openclaw"
		args = []string{"resume", s.NativeID}
	case "junie":
		name = "junie"
		if yolo {
			args = append(args, "--brave")
		}
		args = append(args, "--resume", "--session-id="+s.NativeID)
	case "jules":
		name = "jules"
		args = []string{"teleport", s.NativeID}
	case "goose":
		name = "goose"
		args = []string{"session", "--resume", "--session-id", s.NativeID}
	case "cline":
		name = "cline"
		args = []string{"task", "open", s.NativeID}
		if yolo {
			args = append(args, "--yolo")
		}
	case "aider":
		name = "aider"
		args = []string{"--restore-chat-history"}
		if s.SourcePath != "" && filepath.Base(s.SourcePath) != ".aider.chat.history.md" {
			if err := userArg("chat history file", s.SourcePath); err != nil {
				return Command{}, err
			}
			args = append(args, "--chat-history-file", s.SourcePath)
		}
	case "kiro":
		name = "kiro-cli"
		if yolo {
			args = append(args, "--trust-all-tools")
		}
		args = append(args, "chat", "--resume-id", s.NativeID)
	default:
		return Command{}, fmt.Errorf("resume is not implemented for %s", s.Agent)
	}
	bin, err := LookPath(name)
	if err != nil {
		return Command{}, fmt.Errorf("%s is not on PATH", name)
	}
	c := Command{Name: bin, Args: args}
	if s.CWD != "" {
		st, err := os.Stat(s.CWD)
		if err != nil || !st.IsDir() {
			if s.Agent == "grok" || s.Agent == "agy" {
				return Command{}, fmt.Errorf("working directory %s is not available", s.CWD)
			}
		} else {
			c.Dir = s.CWD
		}
	}
	return c, nil
}

// Delete removes a session using the mode recorded by its scanner.
func Delete(s model.Session) error {
	if !s.CanDelete {
		if s.DeleteReason != "" {
			return errors.New(s.DeleteReason)
		}
		return fmt.Errorf("deletion is disabled for %s", s.Agent)
	}
	switch s.DeleteMode {
	case "file":
		return removeTranscript(s)
	case "dir":
		return removeSessionDir(s)
	case "rewrite":
		return rewriteAgy(s)
	case "exec":
		return deleteExec(s)
	case "cline":
		return deleteCline(s)
	case "aider":
		return deleteAider(s)
	case "grok":
		return deleteGrok(s)
	case "gemini":
		return deleteGemini(s)
	case "jules":
		return deleteJules(s)
	case "kiro":
		return deleteKiro(s)
	default:
		return fmt.Errorf("deletion is disabled for %s", s.Agent)
	}
}

// julesAPIBase is the Jules sessions collection. Tests point it at a local server.
var julesAPIBase = "https://jules.googleapis.com/v1alpha"

func hermesProfile(native string) (profile, id string, ok bool) {
	rest, found := strings.CutPrefix(native, "p:")
	if !found {
		return "", native, false
	}
	profile, id, found = strings.Cut(rest, ":")
	if !found || profile == "" || profile == "default" || id == "" {
		return "", native, false
	}
	return profile, id, true
}

func removeTranscript(s model.Session) error {
	root, err := fileRoot(s.Agent)
	if err != nil {
		return err
	}
	path, err := cleanWithin(s.SourcePath, root)
	if err != nil {
		return err
	}
	if !strings.HasSuffix(path, ".jsonl") {
		return errors.New("refusing to delete a file that is not a transcript")
	}
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSymlink != 0 || !fi.Mode().IsRegular() {
		return errors.New("refusing to delete a non-regular file")
	}
	return os.Remove(path)
}

func fileRoot(agent string) (string, error) {
	switch agent {
	case "claude":
		return discover.ClaudeProjects(), nil
	case "codex":
		return discover.CodexSessions(), nil
	case "cursor":
		return discover.CursorProjects(), nil
	default:
		return "", fmt.Errorf("file delete is not enabled for %s", agent)
	}
}

func removeSessionDir(s model.Session) error {
	switch s.Agent {
	case "copilot":
		return removeCopilotDir(s)
	case "junie":
		return removeJunieDir(s)
	default:
		return fmt.Errorf("directory delete is not enabled for %s", s.Agent)
	}
}

func removeCopilotDir(s model.Session) error {
	path, err := cleanWithin(s.SourcePath, discover.CopilotState())
	if err != nil {
		return err
	}
	if filepath.Base(filepath.Dir(path)) != "session-state" {
		return errors.New("refusing to delete outside a Copilot session-state directory")
	}
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
		return errors.New("refusing to delete a non-directory")
	}
	if _, err := os.Stat(filepath.Join(path, "events.jsonl")); err != nil {
		if _, err2 := os.Stat(filepath.Join(path, "workspace.yaml")); err2 != nil {
			return errors.New("refusing to delete a directory that is not a Copilot session")
		}
	}
	return os.RemoveAll(path)
}

func removeJunieDir(s model.Session) error {
	if !strings.HasPrefix(s.NativeID, "session-") || strings.ContainsAny(s.NativeID, `/\`) {
		return errors.New("refusing to delete a directory that is not a Junie session")
	}
	path, err := cleanWithin(s.SourcePath, discover.JunieSessions())
	if err != nil {
		return err
	}
	if filepath.Base(path) == "transcript.md" {
		path = filepath.Dir(path)
	}
	if filepath.Base(filepath.Dir(path)) != "sessions" || filepath.Base(path) != s.NativeID {
		return errors.New("refusing to delete outside a Junie sessions directory")
	}
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
		return errors.New("refusing to delete a non-directory")
	}
	transcript := filepath.Join(path, "transcript.md")
	st, err := os.Lstat(transcript)
	if err != nil || st.Mode()&os.ModeSymlink != 0 || !st.Mode().IsRegular() {
		return errors.New("refusing to delete a directory that is not a Junie session")
	}
	return os.RemoveAll(path)
}

func rewriteAgy(s model.Session) error {
	if s.NativeID == "" {
		return errors.New("missing conversation id")
	}
	want := filepath.Clean(discover.AgyFile())
	path := filepath.Clean(s.SourcePath)
	if path != want || filepath.Base(path) != "history.jsonl" {
		return errors.New("refusing to rewrite a file that is not the Antigravity history")
	}
	path, err := sameRegularFile(path, want)
	if err != nil {
		return err
	}
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()
	mode := os.FileMode(0o600)
	if st, err := in.Stat(); err == nil {
		mode = st.Mode().Perm()
	}
	tmp := path + ".air9s.tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	removed := 0
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 64*1024), 8*1024*1024)
	bw := bufio.NewWriter(out)
	for sc.Scan() {
		line := sc.Bytes()
		var row struct {
			ConversationID string `json:"conversationId"`
		}
		if json.Unmarshal(line, &row) == nil && row.ConversationID == s.NativeID {
			removed++
			continue
		}
		if _, err := bw.Write(line); err != nil {
			out.Close()
			os.Remove(tmp)
			return err
		}
		if err := bw.WriteByte('\n'); err != nil {
			out.Close()
			os.Remove(tmp)
			return err
		}
	}
	if err := sc.Err(); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if removed == 0 {
		out.Close()
		os.Remove(tmp)
		return errors.New("conversation id was not in the Antigravity history")
	}
	if err := bw.Flush(); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func deleteExec(s model.Session) error {
	if err := userArg("session id", s.NativeID); err != nil {
		return err
	}
	var name string
	var args []string
	switch s.Agent {
	case "opencode":
		name = "opencode"
		args = []string{"session", "delete", s.NativeID}
	case "hermes":
		name = "hermes"
		id := s.NativeID
		if profile, bare, ok := hermesProfile(s.NativeID); ok {
			if err := userArg("hermes profile", profile); err != nil {
				return err
			}
			if err := userArg("session id", bare); err != nil {
				return err
			}
			args = append(args, "-p", profile)
			id = bare
		}
		args = append(args, "sessions", "delete", id, "--yes")
	case "openclaw":
		name = "openclaw"
		args = []string{"sessions", "delete", s.NativeID, "--yes"}
	case "goose":
		name = "goose"
		args = []string{"session", "remove", "--session-id", s.NativeID}
	default:
		return fmt.Errorf("exec delete is not enabled for %s", s.Agent)
	}
	bin, err := LookPath(name)
	if err != nil {
		return fmt.Errorf("%s is not on PATH", name)
	}
	cmd := exec.Command(bin, args...)
	if st, err := os.Stat(s.CWD); err == nil && st.IsDir() {
		cmd.Dir = s.CWD
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s delete: %w", name, err)
	}
	return nil
}

func deleteCline(s model.Session) error {
	if s.Agent != "cline" || !safeSegment(s.NativeID) {
		return errors.New("refusing to delete a Cline task without a plain task id")
	}
	home := discover.ClineHome()
	hist := filepath.Join(home, "data", "state", "taskHistory.json")
	if filepath.Clean(s.SourcePath) != filepath.Clean(hist) {
		return errors.New("refusing to rewrite a file that is not the Cline task history")
	}
	if _, err := sameRegularFile(s.SourcePath, hist); err != nil {
		return err
	}
	taskDir := filepath.Join(home, "data", "tasks", s.NativeID)
	removeDir := false
	if st, err := os.Lstat(taskDir); err == nil {
		if st.Mode()&os.ModeSymlink != 0 || !st.IsDir() {
			return errors.New("refusing to delete a Cline task that is not a directory")
		}
		if _, err := cleanWithin(taskDir, filepath.Join(home, "data", "tasks")); err != nil {
			return err
		}
		removeDir = true
	}
	if err := rewriteClineHistory(hist, s.NativeID); err != nil {
		return err
	}
	if removeDir {
		return os.RemoveAll(taskDir)
	}
	return nil
}

func rewriteClineHistory(path, id string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(body, &rows); err != nil {
		return err
	}
	kept := make([]json.RawMessage, 0, len(rows))
	removed := 0
	for _, raw := range rows {
		var row struct {
			ID   string `json:"id"`
			ULID string `json:"ulid"`
		}
		if json.Unmarshal(raw, &row) == nil && (row.ID == id || (row.ID == "" && row.ULID == id)) {
			removed++
			continue
		}
		kept = append(kept, raw)
	}
	if removed == 0 {
		return errors.New("task id was not in the Cline history")
	}
	out, err := json.MarshalIndent(kept, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return writeAtom(path, out)
}

func deleteGrok(s model.Session) error {
	dir, err := grokSessionDir(s)
	if err != nil {
		return err
	}
	if err := scrubGrokIndexes(s.NativeID); err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

func grokSessionDir(s model.Session) (string, error) {
	if s.Agent != "grok" {
		return "", errors.New("grok delete is only used for Grok")
	}
	if err := plainID("session id", s.NativeID); err != nil {
		return "", err
	}
	summary := filepath.Clean(s.SourcePath)
	if filepath.Base(summary) != "summary.json" {
		return "", errors.New("refusing to delete a file that is not a Grok session summary")
	}
	dir := filepath.Dir(summary)
	if filepath.Base(dir) != s.NativeID {
		return "", errors.New("refusing to delete a directory that is not that Grok session")
	}
	dir, err := cleanWithin(dir, discover.GrokSessions())
	if err != nil {
		return "", err
	}
	st, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	if st.Mode()&os.ModeSymlink != 0 || !st.IsDir() {
		return "", errors.New("refusing to delete a non-directory")
	}
	sum := filepath.Join(dir, "summary.json")
	info, err := os.Lstat(sum)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", errors.New("refusing to delete a directory that is not a Grok session")
	}
	return dir, nil
}

func scrubGrokIndexes(id string) error {
	home := discover.GrokHome()
	active := filepath.Join(home, "active_sessions.json")
	if err := rewriteIfPresent(active, home, func(body []byte) ([]byte, bool, error) {
		var rows []json.RawMessage
		if err := json.Unmarshal(body, &rows); err != nil {
			return nil, false, errors.New("active session list is not a JSON array")
		}
		kept := make([]json.RawMessage, 0, len(rows))
		removed := 0
		for _, raw := range rows {
			var row struct {
				SessionID string `json:"session_id"`
			}
			if json.Unmarshal(raw, &row) == nil && row.SessionID == id {
				removed++
				continue
			}
			kept = append(kept, raw)
		}
		if removed == 0 {
			return nil, false, nil
		}
		out, err := json.MarshalIndent(kept, "", "  ")
		if err != nil {
			return nil, false, err
		}
		return append(out, '\n'), true, nil
	}); err != nil {
		return err
	}
	meta := filepath.Join(home, "client-state", "session-meta.json")
	return rewriteIfPresent(meta, home, func(body []byte) ([]byte, bool, error) {
		var doc map[string]json.RawMessage
		if err := json.Unmarshal(body, &doc); err != nil {
			return nil, false, errors.New("session metadata is not a JSON object")
		}
		if _, ok := doc[id]; !ok {
			return nil, false, nil
		}
		delete(doc, id)
		out, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return nil, false, err
		}
		return append(out, '\n'), true, nil
	})
}

func rewriteIfPresent(path, root string, edit func([]byte) ([]byte, bool, error)) error {
	if _, err := os.Lstat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if _, err := cleanWithin(path, root); err != nil {
		return err
	}
	if _, err := sameRegularFile(path, path); err != nil {
		return err
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out, changed, err := edit(body)
	if err != nil || !changed {
		return err
	}
	return writeAtom(path, out)
}

func deleteGemini(s model.Session) error {
	if s.Agent != "gemini" {
		return errors.New("gemini delete is only used for Gemini")
	}
	if err := plainID("session id", s.NativeID); err != nil {
		return err
	}
	root := filepath.Join(discover.GeminiRoot(), "tmp")
	path, err := cleanWithin(s.SourcePath, root)
	if err != nil {
		return err
	}
	base := filepath.Base(path)
	if filepath.Base(filepath.Dir(path)) != "chats" || !strings.HasPrefix(base, "session-") || !strings.HasSuffix(base, ".json") {
		return errors.New("refusing to delete a file that is not a Gemini chat")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("refusing to delete a non-regular file")
	}
	got, err := geminiFileID(path)
	if err != nil {
		return err
	}
	stem := strings.TrimSuffix(strings.TrimPrefix(base, "session-"), ".json")
	if got != "" && got != s.NativeID {
		return errors.New("refusing to delete a Gemini chat for a different session")
	}
	if got == "" && stem != s.NativeID {
		return errors.New("refusing to delete a Gemini chat for a different session")
	}
	return os.Remove(path)
}

func geminiFileID(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return "", errors.New("gemini chat is not a JSON object")
	}
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return "", err
		}
		name, _ := key.(string)
		if name == "sessionId" {
			var id string
			if err := dec.Decode(&id); err != nil {
				return "", err
			}
			return strings.TrimSpace(id), nil
		}
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return "", err
		}
	}
	return "", nil
}

func deleteJules(s model.Session) error {
	if s.Agent != "jules" {
		return errors.New("jules delete is only used for Jules")
	}
	if err := plainID("session id", s.NativeID); err != nil {
		return err
	}
	if s.SourcePath == "jules:remote" {
		return deleteJulesRemote(s.NativeID, strings.TrimSpace(os.Getenv("JULES_API_KEY")))
	}
	return rewriteJulesLocal(s)
}

func deleteJulesRemote(id, apiKey string) error {
	if apiKey == "" {
		return errors.New("jules has no delete command; set JULES_API_KEY to delete the cloud session")
	}
	endpoint := strings.TrimRight(julesAPIBase, "/") + "/sessions/" + url.PathEscape(id)
	req, err := http.NewRequest(http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Goog-Api-Key", apiKey)
	client := &http.Client{
		Timeout: 20 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("jules delete: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("jules delete returned %s", resp.Status)
	}
	return nil
}

func rewriteJulesLocal(s model.Session) error {
	home := discover.JulesHome()
	path := filepath.Clean(s.SourcePath)
	base := filepath.Base(path)
	if (base != "sessions.json" && base != "sessions.txt") || filepath.Clean(filepath.Dir(path)) != filepath.Clean(home) {
		return errors.New("refusing to rewrite a file that is not the Jules session list")
	}
	resolved, err := cleanWithin(path, home)
	if err != nil {
		return err
	}
	info, err := os.Lstat(resolved)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("refusing to rewrite a file that is not a regular file")
	}
	body, err := os.ReadFile(resolved)
	if err != nil {
		return err
	}
	trimmed := strings.TrimSpace(string(body))
	if strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "{") {
		out, err := dropJulesJSON(body, s.NativeID)
		if err != nil {
			return err
		}
		return writeAtom(resolved, out)
	}
	out, err := dropJulesText(body, s.NativeID)
	if err != nil {
		return err
	}
	return writeAtom(resolved, out)
}

func dropJulesJSON(body []byte, id string) ([]byte, error) {
	var rows []json.RawMessage
	if err := json.Unmarshal(body, &rows); err == nil {
		kept, removed, err := filterJulesRows(rows, id)
		if err != nil {
			return nil, err
		}
		if removed == 0 {
			return nil, errors.New("session id was not in the Jules session list")
		}
		out, err := json.MarshalIndent(kept, "", "  ")
		if err != nil {
			return nil, err
		}
		return append(out, '\n'), nil
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, errors.New("jules session list is not JSON")
	}
	raw, ok := doc["sessions"]
	if !ok {
		return nil, errors.New("jules session list has no sessions")
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	kept, removed, err := filterJulesRows(rows, id)
	if err != nil {
		return nil, err
	}
	if removed == 0 {
		return nil, errors.New("session id was not in the Jules session list")
	}
	encoded, err := json.Marshal(kept)
	if err != nil {
		return nil, err
	}
	doc["sessions"] = encoded
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

func filterJulesRows(rows []json.RawMessage, id string) ([]json.RawMessage, int, error) {
	kept := make([]json.RawMessage, 0, len(rows))
	removed := 0
	for _, raw := range rows {
		var row map[string]any
		if json.Unmarshal(raw, &row) == nil && julesRowID(row) == id {
			removed++
			continue
		}
		kept = append(kept, raw)
	}
	return kept, removed, nil
}

func julesRowID(row map[string]any) string {
	for _, key := range []string{"id", "session", "sessionId", "name"} {
		s, ok := row[key].(string)
		if !ok {
			continue
		}
		s = strings.TrimSpace(s)
		s = strings.TrimPrefix(s, "sessions/")
		if s != "" {
			return s
		}
	}
	return ""
}

func dropJulesText(body []byte, id string) ([]byte, error) {
	text := string(body)
	nl := strings.HasSuffix(text, "\n")
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	kept := make([]string, 0, len(lines))
	removed := 0
	for _, line := range lines {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) > 0 && fields[0] == id {
			removed++
			continue
		}
		kept = append(kept, line)
	}
	if removed == 0 {
		return nil, errors.New("session id was not in the Jules session list")
	}
	out := strings.Join(kept, "\n")
	if nl || out != "" {
		out += "\n"
	}
	return []byte(out), nil
}

func deleteKiro(s model.Session) error {
	if s.Agent != "kiro" {
		return errors.New("kiro delete is only used for Kiro")
	}
	if err := plainID("session id", s.NativeID); err != nil {
		return err
	}
	want := discover.KiroDB()
	path, err := sameRegularFile(s.SourcePath, want)
	if err != nil {
		return err
	}
	db, err := openWriteDB(path)
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	removed, err := deleteKiroRows(tx, s.NativeID)
	if err != nil {
		return err
	}
	if removed == 0 {
		return errors.New("conversation id was not in the Kiro database")
	}
	return tx.Commit()
}

func deleteKiroRows(tx *sql.Tx, id string) (int, error) {
	removed := 0
	hasV2, err := tableExists(tx, "conversations_v2")
	if err != nil {
		return 0, err
	}
	keys := map[string]struct{}{}
	matchedV2 := false
	if hasV2 {
		rows, err := tx.Query(`
			SELECT coalesce(key,''), coalesce(conversation_id,'')
			FROM conversations_v2
			WHERE conversation_id = ?
			   OR (coalesce(conversation_id,'') = '' AND key = ?)
			   OR json_extract(value, '$.conversation_id') = ?`, id, id, id)
		if err != nil {
			return 0, err
		}
		type pair struct{ key, conv string }
		var pairs []pair
		for rows.Next() {
			var key, conv string
			if err := rows.Scan(&key, &conv); err != nil {
				rows.Close()
				return 0, err
			}
			pairs = append(pairs, pair{key, conv})
			if key != "" {
				keys[key] = struct{}{}
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return 0, err
		}
		rows.Close()
		matchedV2 = len(pairs) > 0
		for _, p := range pairs {
			res, err := tx.Exec(`DELETE FROM conversations_v2 WHERE coalesce(key,'') = ? AND coalesce(conversation_id,'') = ?`, p.key, p.conv)
			if err != nil {
				return 0, err
			}
			n, err := res.RowsAffected()
			if err != nil {
				return 0, err
			}
			removed += int(n)
		}
	}
	hasV1, err := tableExists(tx, "conversations")
	if err != nil {
		return 0, err
	}
	if hasV1 {
		ids := make([]string, 0, len(keys)+1)
		if matchedV2 {
			for key := range keys {
				ids = append(ids, key)
			}
		} else {
			ids = append(ids, id)
		}
		for _, key := range ids {
			res, err := tx.Exec(`DELETE FROM conversations WHERE key = ?`, key)
			if err != nil {
				return 0, err
			}
			n, err := res.RowsAffected()
			if err != nil {
				return 0, err
			}
			removed += int(n)
		}
	}
	return removed, nil
}

func tableExists(tx *sql.Tx, name string) (bool, error) {
	var got string
	err := tx.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&got)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func openWriteDB(path string) (*sql.DB, error) {
	u := url.URL{Scheme: "file", Path: path, RawQuery: "_pragma=busy_timeout(3000)"}
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

func plainID(label, value string) error {
	if err := userArg(label, value); err != nil {
		return err
	}
	if strings.ContainsAny(value, `/\`) || strings.Contains(value, "..") {
		return fmt.Errorf("%s cannot be passed to the agent CLI", label)
	}
	return nil
}

func deleteAider(s model.Session) error {
	if s.Agent != "aider" {
		return errors.New("aider delete is only used for Aider")
	}
	path := filepath.Clean(s.SourcePath)
	if path == "" || path != filepath.Clean(s.NativeID) || filepath.Base(path) != ".aider.chat.history.md" {
		return errors.New("refusing to delete a file that is not an Aider chat history")
	}
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSymlink != 0 || !fi.Mode().IsRegular() {
		return errors.New("refusing to delete a non-regular file")
	}
	return os.Remove(path)
}

func safeSegment(id string) bool {
	return userArg("task id", id) == nil && !strings.ContainsAny(id, `/\`)
}

// userArg rejects values that an agent CLI would parse as a flag or that
// could truncate an argument at a NUL.
func userArg(label, value string) error {
	if value == "" || value == "." || value == ".." || strings.HasPrefix(value, "-") || strings.ContainsRune(value, 0) {
		return fmt.Errorf("%s cannot be passed to the agent CLI", label)
	}
	return nil
}

// sameRegularFile reports the real path when path and want are the same
// regular file. A symlink, including one reached through a parent directory,
// is refused so a rewrite cannot land outside the agent file.
func sameRegularFile(path, want string) (string, error) {
	path = filepath.Clean(path)
	want = filepath.Clean(want)
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", errors.New("refusing to rewrite a file that is not a regular file")
	}
	resolved, err := resolveFile(path)
	if err != nil {
		return "", err
	}
	resolvedWant, err := resolveFile(want)
	if err != nil {
		return "", err
	}
	if resolved != resolvedWant {
		return "", errors.New("refusing to rewrite a file outside the agent directory")
	}
	return resolved, nil
}

func resolveFile(path string) (string, error) {
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(path)), nil
}

func writeAtom(path string, body []byte) error {
	mode := os.FileMode(0o600)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	tmp := path + ".air9s.tmp"
	if err := os.WriteFile(tmp, body, mode); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func cleanWithin(path, root string) (string, error) {
	if path == "" || root == "" {
		return "", errors.New("refusing to delete without a session path")
	}
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	if !inside(root, path) {
		return "", errors.New("refusing to delete outside the agent session directory")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("refusing to delete a symlink")
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	resolvedParent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	resolved := filepath.Join(resolvedParent, filepath.Base(path))
	if !inside(resolvedRoot, resolved) {
		return "", errors.New("refusing to delete outside the agent session directory")
	}
	return resolved, nil
}

func inside(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

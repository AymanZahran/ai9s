package act

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AymanZahran/air9s/internal/discover"
	"github.com/AymanZahran/air9s/internal/model"
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
func (c Command) Run() error {
	cmd := exec.Command(c.Name, c.Args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if c.Dir != "" {
		cmd.Dir = c.Dir
	}
	return cmd.Run()
}

// Plan builds the agent's own resume command.
func Plan(s model.Session, yolo bool) (Command, error) {
	if s.NativeID == "" {
		return Command{}, errors.New("session has no id")
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
	default:
		return fmt.Errorf("deletion is disabled for %s", s.Agent)
	}
}

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
	return id != "" && id != "." && id != ".." && !strings.ContainsAny(id, `/\`)
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
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", errors.New("refusing to delete outside the agent session directory")
	}
	return path, nil
}

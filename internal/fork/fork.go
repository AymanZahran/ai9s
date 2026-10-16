// Package fork copies one local session into a second file the scanner can index.
package fork

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/AymanZahran/ai9s/internal/discover"
	"github.com/AymanZahran/ai9s/internal/model"
	"github.com/google/uuid"
)

// newForkID is the native id of a fork. Tests pin it.
var newForkID = uuid.NewString

// Copy writes a second session next to s and returns its native id.
// The original file is left unchanged. A harness whose transcript lives in a
// shared database, or only on a server, is refused.
// When opt.Worktree is set, the copy records a new git checkout.
func Copy(s model.Session, opt Options) (string, error) {
	if err := usableID(s.NativeID); err != nil {
		return "", err
	}
	if strings.TrimSpace(s.SourcePath) == "" {
		return "", errors.New("this session has no file to fork")
	}
	move, cleanup, err := prepareWorktree(s, opt)
	if err != nil {
		return "", err
	}
	id, err := copyFiles(s, move)
	if err != nil {
		cleanup()
		return "", err
	}
	return id, nil
}

func copyFiles(s model.Session, move dirMove) (string, error) {
	switch s.Agent {
	case "claude":
		return copyTranscript(s, discover.ClaudeProjects(), claudeName, false, move)
	case "codex":
		return copyTranscript(s, discover.CodexSessions(), codexName, true, move)
	case "pi":
		return copyTranscript(s, discover.PiSessions(), jsonlName, false, move)
	case "omp":
		return copyTranscript(s, discover.OmpSessions(), jsonlName, false, move)
	case "droid":
		return copyDroid(s, move)
	case "qodercli":
		return copyTranscript(s, discover.QoderProjects(), jsonlName, false, move)
	case "cursor":
		return copyCursor(s, move)
	case "copilot":
		return copyCopilot(s, move)
	case "grok":
		return copyGrok(s, move)
	case "junie":
		return copyJunie(s)
	case "letta":
		return copyLetta(s)
	default:
		return "", errors.New("fork is not available for this harness yet")
	}
}

func claudeName(id string) string { return id + ".jsonl" }

func codexName(id string) string { return "rollout-" + id + ".jsonl" }

func jsonlName(id string) string { return id + ".jsonl" }

func copyTranscript(s model.Session, root string, name func(string) string, mustRewrite bool, move dirMove) (string, error) {
	src, err := withinRoot(s.SourcePath, root)
	if err != nil {
		return "", err
	}
	st, err := os.Lstat(src)
	if err != nil {
		return "", err
	}
	if st.Mode()&os.ModeSymlink != 0 || !st.Mode().IsRegular() || !strings.HasSuffix(src, ".jsonl") {
		return "", errors.New("refusing to fork a file that is not a transcript")
	}
	id := newForkID()
	dst := filepath.Join(filepath.Dir(src), name(id))
	if err := destWithin(dst, root); err != nil {
		return "", err
	}
	n, err := rewriteJSONL(src, dst, s.NativeID, id, st.Mode().Perm(), move.swaps())
	if err != nil {
		return "", err
	}
	if !identityMoved(src, s.NativeID, n, mustRewrite) {
		_ = os.Remove(dst)
		return "", errors.New("fork could not give this session its own id")
	}
	return id, nil
}

// identityMoved reports that the copy will scan as a different session.
// A filename id, such as Claude's, moves when the file is renamed.
// An id stored inside the file has to be rewritten, or the copy collides
// with the original on the next index.
func identityMoved(src, native string, replaced int, mustRewrite bool) bool {
	if mustRewrite {
		return replaced > 0
	}
	if replaced > 0 {
		return true
	}
	return strings.TrimSuffix(filepath.Base(src), ".jsonl") == native
}

func copyDroid(s model.Session, move dirMove) (string, error) {
	id, err := copyTranscript(s, discover.DroidSessions(), jsonlName, false, move)
	if err != nil {
		return "", err
	}
	src, err := withinRoot(s.SourcePath, discover.DroidSessions())
	if err != nil {
		return "", err
	}
	settings := strings.TrimSuffix(src, ".jsonl") + ".settings.json"
	st, err := os.Lstat(settings)
	if err != nil {
		if os.IsNotExist(err) {
			return id, nil
		}
		_ = os.Remove(filepath.Join(filepath.Dir(src), jsonlName(id)))
		return "", err
	}
	if st.Mode()&os.ModeSymlink != 0 || !st.Mode().IsRegular() {
		_ = os.Remove(filepath.Join(filepath.Dir(src), jsonlName(id)))
		return "", errors.New("refusing to fork a file that is not a regular file")
	}
	dst := filepath.Join(filepath.Dir(src), strings.TrimSuffix(jsonlName(id), ".jsonl")+".settings.json")
	if err := destWithin(dst, discover.DroidSessions()); err != nil {
		_ = os.Remove(filepath.Join(filepath.Dir(src), jsonlName(id)))
		return "", err
	}
	if _, err := rewriteJSONFile(settings, dst, s.NativeID, id, st.Mode().Perm(), move.swaps()); err != nil {
		_ = os.Remove(dst)
		_ = os.Remove(filepath.Join(filepath.Dir(src), jsonlName(id)))
		return "", err
	}
	return id, nil
}

func copyCursor(s model.Session, move dirMove) (string, error) {
	root := discover.CursorProjects()
	file, err := withinRoot(s.SourcePath, root)
	if err != nil {
		return "", err
	}
	st, err := os.Lstat(file)
	if err != nil {
		return "", err
	}
	if st.Mode()&os.ModeSymlink != 0 || !st.Mode().IsRegular() || !strings.HasSuffix(file, ".jsonl") {
		return "", errors.New("refusing to fork a file that is not a transcript")
	}
	parent := filepath.Dir(file)
	id := newForkID()
	if move.active() {
		return copyCursorWorktree(file, root, id, s.NativeID, st.Mode().Perm(), move)
	}
	if filepath.Base(parent) == "agent-transcripts" {
		dst := filepath.Join(parent, id+".jsonl")
		if err := destWithin(dst, root); err != nil {
			return "", err
		}
		if _, err := rewriteJSONL(file, dst, s.NativeID, id, st.Mode().Perm(), nil); err != nil {
			return "", err
		}
		return id, nil
	}
	if filepath.Base(parent) != s.NativeID {
		return "", errors.New("refusing to fork a cursor transcript outside its session directory")
	}
	parent, err = withinRoot(parent, root)
	if err != nil {
		return "", err
	}
	dst := filepath.Join(filepath.Dir(parent), id)
	if err := destWithin(dst, root); err != nil {
		return "", err
	}
	if _, err := copyTree(parent, dst, s.NativeID, id, true, nil); err != nil {
		return "", err
	}
	return id, nil
}

func copyCopilot(s model.Session, move dirMove) (string, error) {
	root := discover.CopilotState()
	dir, err := withinRoot(s.SourcePath, root)
	if err != nil {
		return "", err
	}
	if filepath.Base(filepath.Dir(dir)) != "session-state" {
		return "", errors.New("refusing to fork outside a copilot session-state directory")
	}
	st, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	if st.Mode()&os.ModeSymlink != 0 || !st.IsDir() {
		return "", errors.New("refusing to fork a non-directory")
	}
	if _, err := os.Lstat(filepath.Join(dir, "events.jsonl")); err != nil {
		if _, err2 := os.Lstat(filepath.Join(dir, "workspace.yaml")); err2 != nil {
			return "", errors.New("refusing to fork a directory that is not a copilot session")
		}
	}
	yamlID, hasID := "", false
	if body, err := os.ReadFile(filepath.Join(dir, "workspace.yaml")); err == nil {
		yamlID, hasID = firstYAMLID(string(body))
	}
	if hasID && yamlID != s.NativeID {
		return "", errors.New("fork could not give this session its own id")
	}
	if !hasID && filepath.Base(dir) != s.NativeID {
		return "", errors.New("fork could not give this session its own id")
	}
	id := newForkID()
	dst := filepath.Join(filepath.Dir(dir), id)
	if err := destWithin(dst, root); err != nil {
		return "", err
	}
	n, err := copyTree(dir, dst, s.NativeID, id, true, move.swaps())
	if err != nil {
		return "", err
	}
	if hasID && n == 0 {
		_ = os.RemoveAll(dst)
		return "", errors.New("fork could not give this session its own id")
	}
	return id, nil
}

func copyGrok(s model.Session, move dirMove) (string, error) {
	if filepath.Base(s.SourcePath) != "summary.json" {
		return "", errors.New("refusing to fork a file that is not a grok session summary")
	}
	dir := filepath.Dir(s.SourcePath)
	if filepath.Base(dir) != s.NativeID {
		return "", errors.New("refusing to fork a directory that is not that grok session")
	}
	root := discover.GrokSessions()
	dir, err := withinRoot(dir, root)
	if err != nil {
		return "", err
	}
	st, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	if st.Mode()&os.ModeSymlink != 0 || !st.IsDir() {
		return "", errors.New("refusing to fork a non-directory")
	}
	summary := filepath.Join(dir, "summary.json")
	info, err := os.Lstat(summary)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", errors.New("refusing to fork a directory that is not a grok session")
	}
	id := newForkID()
	dst := filepath.Join(filepath.Dir(dir), id)
	if err := destWithin(dst, root); err != nil {
		return "", err
	}
	n, err := copyTree(dir, dst, s.NativeID, id, true, move.swaps())
	if err != nil {
		return "", err
	}
	if n == 0 {
		_ = os.RemoveAll(dst)
		return "", errors.New("fork could not give this session its own id")
	}
	return id, nil
}

func copyJunie(s model.Session) (string, error) {
	if !strings.HasPrefix(s.NativeID, "session-") {
		return "", errors.New("refusing to fork a directory that is not a junie session")
	}
	root := discover.JunieSessions()
	file, err := withinRoot(s.SourcePath, root)
	if err != nil {
		return "", err
	}
	if filepath.Base(file) != "transcript.md" {
		return "", errors.New("refusing to fork a file that is not a junie transcript")
	}
	dir, err := withinRoot(filepath.Dir(file), root)
	if err != nil {
		return "", err
	}
	if filepath.Base(filepath.Dir(dir)) != "sessions" || filepath.Base(dir) != s.NativeID {
		return "", errors.New("refusing to fork outside a junie sessions directory")
	}
	id := "session-" + newForkID()
	dst := filepath.Join(filepath.Dir(dir), id)
	if err := destWithin(dst, root); err != nil {
		return "", err
	}
	if _, err := copyTree(dir, dst, s.NativeID, id, false, nil); err != nil {
		return "", err
	}
	return id, nil
}

func copyLetta(s model.Session) (string, error) {
	root := discover.LettaTranscripts()
	file, err := withinRoot(s.SourcePath, root)
	if err != nil {
		return "", err
	}
	if filepath.Base(file) != "transcript.jsonl" {
		return "", errors.New("refusing to fork a file that is not a transcript")
	}
	conv, err := withinRoot(filepath.Dir(file), root)
	if err != nil {
		return "", err
	}
	agent, err := withinRoot(filepath.Dir(conv), root)
	if err != nil {
		return "", err
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	if filepath.Clean(filepath.Dir(agent)) != filepath.Clean(resolvedRoot) {
		return "", errors.New("refusing to fork outside a letta transcripts directory")
	}
	id := newForkID()
	dst := filepath.Join(agent, id)
	if err := destWithin(dst, root); err != nil {
		return "", err
	}
	if _, err := copyTree(conv, dst, s.NativeID, id, false, nil); err != nil {
		return "", err
	}
	return lettaForkID(filepath.Base(agent), id), nil
}

func lettaForkID(agent, conv string) string {
	if conv == "" || conv == "default" {
		return agent
	}
	return agent + ":" + conv
}

func copyTree(src, dst, oldID, newID string, rewrite bool, swaps []textSwap) (n int, err error) {
	if err = os.Mkdir(dst, 0o700); err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(dst)
		}
	}()
	err = filepath.WalkDir(src, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.Type()&os.ModeSymlink != 0 {
			return errors.New("refusing to fork a symlink")
		}
		if path == src {
			return nil
		}
		rel, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return relErr
		}
		parts := strings.Split(rel, string(os.PathSeparator))
		for i, part := range parts {
			parts[i] = renameComponent(part, oldID, newID)
		}
		target := filepath.Join(append([]string{dst}, parts...)...)
		if d.IsDir() {
			return os.Mkdir(target, 0o700)
		}
		info, statErr := os.Lstat(path)
		if statErr != nil {
			return statErr
		}
		if !info.Mode().IsRegular() {
			return errors.New("refusing to fork a file that is not a regular file")
		}
		got, copyErr := transferFile(path, target, oldID, newID, info.Mode().Perm(), rewrite, swaps)
		n += got
		return copyErr
	})
	return n, err
}

func transferFile(src, dst, oldID, newID string, mode os.FileMode, rewrite bool, swaps []textSwap) (int, error) {
	if !rewrite && len(swaps) == 0 {
		return 0, copyBytes(src, dst, mode)
	}
	base := filepath.Base(src)
	switch {
	case strings.HasSuffix(base, ".jsonl"):
		return rewriteJSONL(src, dst, oldID, newID, mode, swaps)
	case strings.HasSuffix(base, ".json"):
		return rewriteJSONFile(src, dst, oldID, newID, mode, swaps)
	case base == "workspace.yaml":
		return rewriteYAMLFile(src, dst, oldID, newID, mode, swaps)
	default:
		return 0, copyBytes(src, dst, mode)
	}
}

// copyCursorWorktree puts the transcript under the cursor project folder for
// the new checkout. Cursor reads the working directory from that folder name.
func copyCursorWorktree(file, root, id, oldID string, mode os.FileMode, move dirMove) (string, error) {
	encoded := strings.ReplaceAll(filepath.Clean(move.newCWD), string(os.PathSeparator), "-")
	if encoded == "" || encoded == "." || strings.Contains(encoded, "..") {
		return "", errors.New("refusing to fork outside the agent session directory")
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	dst := filepath.Join(resolved, encoded, "agent-transcripts", id+".jsonl")
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return "", err
	}
	if err := destWithin(dst, root); err != nil {
		return "", err
	}
	if _, err := rewriteJSONL(file, dst, oldID, id, mode, move.swaps()); err != nil {
		return "", err
	}
	return id, nil
}

func renameComponent(name, oldID, newID string) string {
	switch name {
	case oldID:
		return newID
	case oldID + ".jsonl":
		return newID + ".jsonl"
	case oldID + ".settings.json":
		return newID + ".settings.json"
	default:
		return name
	}
}

func usableID(id string) error {
	if strings.TrimSpace(id) == "" || id == "." || id == ".." || strings.HasPrefix(id, "-") {
		return errors.New("refusing to fork this session id")
	}
	if strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") {
		return errors.New("refusing to fork this session id")
	}
	for _, r := range id {
		if unicode.IsControl(r) {
			return errors.New("refusing to fork this session id")
		}
	}
	return nil
}

func withinRoot(path, root string) (string, error) {
	if path == "" || root == "" {
		return "", errors.New("refusing to fork without a session path")
	}
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	// A caller may pass the path returned by an earlier withinRoot call.
	// That path is already resolved, so it is compared with the resolved root.
	if !inside(root, path) && !inside(resolvedRoot, path) {
		return "", errors.New("refusing to fork outside the agent session directory")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("refusing to fork a symlink")
	}
	resolvedParent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	resolved := filepath.Join(resolvedParent, filepath.Base(path))
	if !inside(resolvedRoot, resolved) {
		return "", errors.New("refusing to fork outside the agent session directory")
	}
	return resolved, nil
}

func destWithin(path, root string) error {
	if path == "" || root == "" {
		return errors.New("refusing to fork without a session path")
	}
	path = filepath.Clean(path)
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	root = resolvedRoot
	if !inside(root, path) {
		return errors.New("refusing to fork outside the agent session directory")
	}
	if _, err := os.Lstat(path); err == nil {
		return errors.New("a fork with this name already exists")
	} else if !os.IsNotExist(err) {
		return err
	}
	resolvedRoot, err = filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	parent := filepath.Dir(path)
	resolvedParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return err
	}
	resolved := filepath.Join(resolvedParent, filepath.Base(path))
	if !inside(resolvedRoot, resolved) {
		return errors.New("refusing to fork outside the agent session directory")
	}
	return nil
}

func inside(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

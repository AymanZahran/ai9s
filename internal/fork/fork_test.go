package fork

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AymanZahran/ai9s/internal/discover"
	"github.com/AymanZahran/ai9s/internal/model"
)

func TestRewriteJSONValuesKeepsProse(t *testing.T) {
	in := `{"parentId":"abc","id":"abc","note":"abc","text":"see abc in prose","extra":"abc-more"}`
	out, n := rewriteJSONValues(in, "abc", "zzz", nil)
	if n != 1 {
		t.Fatalf("replacements %d", n)
	}
	for _, want := range []string{`"id":"zzz"`, `"parentId":"abc"`, `"note":"abc"`, "see abc in prose", "abc-more"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s in %s", want, out)
		}
	}
	if strings.Contains(out, `"id":"abc"`) {
		t.Fatalf("old id value stayed %s", out)
	}
	pretty := "{\n  \"info\": {\"id\": \"abc\"},\n  \"agent_id\": \"abc\"\n}\n"
	out, n = rewriteJSONValues(pretty, "abc", "zzz", nil)
	if n != 2 || !strings.Contains(out, `"id": "zzz"`) || !strings.Contains(out, `"agent_id": "zzz"`) {
		t.Fatalf("pretty %d %s", n, out)
	}
	yaml, ok := rewriteWorkspaceYAML("  id: \"abc\"\nname: demo\n# id: abc\n", "abc", "zzz", nil)
	if !ok || yaml != "  id: zzz\nname: demo\n# id: abc\n" {
		t.Fatalf("yaml %q", yaml)
	}
}

func TestForkClaudeKeepsParentAndProse(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", root)
	id := "11111111-1111-1111-1111-111111111111"
	path := filepath.Join(root, "projects", "demo", id+".jsonl")
	body := `{"type":"user","message":{"role":"user","content":"see ` + id + ` in prose"},"cwd":"/work/app","timestamp":"2026-03-02T15:04:05Z","sessionId":"` + id + `","note":"` + id + `","parentId":"` + id + `","thread_id":"` + id + `"}` + "\n"
	writeFixture(t, path, body)
	native, child := forkAndScan(t, "claude", id, path)
	text := readFile(t, child.SourcePath)
	for _, want := range []string{`"sessionId":"` + native + `"`, `"thread_id":"` + native + `"`, `"note":"` + id + `"`, `"parentId":"` + id + `"`, "see " + id + " in prose"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %s in %s", want, text)
		}
	}
	if strings.Contains(text, `"sessionId":"`+id+`"`) {
		t.Fatalf("child kept the parent session id %s", text)
	}
}

func TestForkCodexRewritesSessionID(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CODEX_HOME", root)
	id := "s1"
	dir := filepath.Join(root, "sessions", "2026")
	path := filepath.Join(dir, "rollout-1.jsonl")
	body := `{"type":"session_meta","timestamp":"2026-03-02T15:04:05Z","payload":{"session_id":"s1","cwd":"/work","model":"gpt"}}
{"type":"response_item","timestamp":"2026-03-02T15:04:06Z","payload":{"type":"message","role":"user","content":"session s1 is the topic"}}
`
	writeFixture(t, path, body)
	native, child := forkAndScan(t, "codex", id, path)
	text := readFile(t, child.SourcePath)
	if !strings.Contains(text, `"session_id":"`+native+`"`) || !strings.Contains(text, "session s1 is the topic") {
		t.Fatalf("%s", text)
	}
	if !strings.HasPrefix(filepath.Base(child.SourcePath), "rollout-") {
		t.Fatalf("name %s", child.SourcePath)
	}
}

func TestForkCodexWithoutSessionIDLeavesNoCopy(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CODEX_HOME", root)
	dir := filepath.Join(root, "sessions")
	path := filepath.Join(dir, "rollout-1.jsonl")
	writeFixture(t, path, `{"type":"response_item","payload":{"type":"message","role":"user","content":"hello"}}`+"\n")
	_, err := Copy(model.Session{Agent: "codex", NativeID: "s1", SourcePath: path}, Options{})
	if err == nil || !strings.Contains(err.Error(), "own id") {
		t.Fatal(err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, "rollout-*.jsonl"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("files %v %v", matches, err)
	}
}

func TestForkPiKeepsShortIDInsideProse(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PI_CODING_AGENT_SESSION_DIR", root)
	path := filepath.Join(root, "proj", "2026.jsonl")
	body := `{"type":"session","id":"pi-1","timestamp":"2026-01-02T03:04:05Z","cwd":"/work/pi"}` + "\n" +
		`{"type":"message","message":{"role":"user","content":"pi-1 is the plan"},"note":"pi-1","id":"pi-1-extra"}` + "\n"
	writeFixture(t, path, body)
	native, child := forkAndScan(t, "pi", "pi-1", path)
	text := readFile(t, child.SourcePath)
	if !strings.Contains(text, `"id":"`+native+`"`) || !strings.Contains(text, "pi-1 is the plan") || !strings.Contains(text, `"note":"pi-1"`) || !strings.Contains(text, `"id":"pi-1-extra"`) {
		t.Fatalf("%s", text)
	}
	if strings.Contains(text, `"id":"pi-1"`) {
		t.Fatalf("short id was rewritten too widely %s", text)
	}
}

func TestForkOmpDroidAndQoder(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PI_CODING_AGENT_SESSION_DIR", filepath.Join(root, "omp"))
	omp := filepath.Join(root, "omp", "one.jsonl")
	writeFixture(t, omp, `{"type":"session","id":"omp-1","cwd":"/work/omp","timestamp":"2026-01-02T03:04:05Z"}`+"\n"+
		`{"type":"message","message":{"role":"user","content":"ship"}}`+"\n")
	if _, child := forkAndScan(t, "omp", "omp-1", omp); child.CWD != "/work/omp" {
		t.Fatalf("%+v", child)
	}

	t.Setenv("FACTORY_HOME", filepath.Join(root, "factory"))
	droid := filepath.Join(root, "factory", "sessions", "droid-1.jsonl")
	writeFixture(t, droid, `{"type":"session_start","id":"droid-1","title":"ship the droid","cwd":"/work/droid"}`+"\n"+
		`{"type":"message","message":{"role":"user","content":"go"}}`+"\n")
	writeFixture(t, strings.TrimSuffix(droid, ".jsonl")+".settings.json", `{"model":"gpt","id":"droid-1"}`)
	native, child := forkAndScan(t, "droid", "droid-1", droid)
	settings := strings.TrimSuffix(child.SourcePath, ".jsonl") + ".settings.json"
	if !strings.Contains(readFile(t, settings), `"id":"`+native+`"`) {
		t.Fatalf("settings %s", readFile(t, settings))
	}
	if strings.Contains(readFile(t, strings.TrimSuffix(droid, ".jsonl")+".settings.json"), native) {
		t.Fatal("parent settings changed")
	}

	t.Setenv("QODER_CONFIG_DIR", filepath.Join(root, "qoder"))
	qoder := filepath.Join(root, "qoder", "projects", "q1.jsonl")
	writeFixture(t, qoder, `{"type":"user","sessionId":"q1","message":{"role":"user","content":"plan the port"},"cwd":"/work/qoder"}`+"\n")
	native, child = forkAndScan(t, "qodercli", "q1", qoder)
	if !strings.Contains(readFile(t, child.SourcePath), `"sessionId":"`+native+`"`) {
		t.Fatal(readFile(t, child.SourcePath))
	}
}

func TestForkCursorDirectoryAndFlatFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CURSOR_HOME", root)
	id := "conv-1"
	path := filepath.Join(root, "projects", "demo", "agent-transcripts", id, id+".jsonl")
	writeFixture(t, path, `{"role":"user","message":{"content":"fix the button"},"id":"`+id+`","note":"`+id+`"}`+"\n")
	native, child := forkAndScan(t, "cursor", id, path)
	if filepath.Base(filepath.Dir(child.SourcePath)) != native {
		t.Fatalf("dir %s", child.SourcePath)
	}
	text := readFile(t, child.SourcePath)
	if !strings.Contains(text, `"id":"`+native+`"`) || !strings.Contains(text, `"note":"`+id+`"`) || !strings.Contains(text, "fix the button") {
		t.Fatalf("%s", text)
	}

	flat := filepath.Join(root, "projects", "demo", "agent-transcripts", "plain.jsonl")
	writeFixture(t, flat, `{"role":"user","message":{"content":"flat file"}}`+"\n")
	native, child = forkAndScan(t, "cursor", "plain", flat)
	if filepath.Base(filepath.Dir(child.SourcePath)) != "agent-transcripts" || filepath.Base(child.SourcePath) != native+".jsonl" {
		t.Fatalf("flat %s native %s", child.SourcePath, native)
	}
}

func TestForkCopilotRewritesYAMLID(t *testing.T) {
	root := t.TempDir()
	t.Setenv("COPILOT_HOME", root)
	dir := filepath.Join(root, "session-state", "folder")
	writeFixture(t, filepath.Join(dir, "workspace.yaml"), "id: sess\nname: demo\ncwd: /work\n")
	writeFixture(t, filepath.Join(dir, "events.jsonl"), `{"type":"user.message","timestamp":"2026-03-02T15:04:06Z","data":{"content":"sess stays in prose"}}`+"\n")
	native, child := forkAndScan(t, "copilot", "sess", dir)
	yaml := readFile(t, filepath.Join(child.SourcePath, "workspace.yaml"))
	if !strings.Contains(yaml, "id: "+native) || strings.Contains(yaml, "id: sess") {
		t.Fatalf("%s", yaml)
	}
	if !strings.Contains(readFile(t, filepath.Join(child.SourcePath, "events.jsonl")), "sess stays in prose") {
		t.Fatal("prose changed")
	}
	if readFile(t, filepath.Join(dir, "workspace.yaml")) != "id: sess\nname: demo\ncwd: /work\n" {
		t.Fatal("parent yaml changed")
	}
}

func TestForkGrokRenamesDirectory(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GROK_HOME", root)
	id := "sess-1"
	dir := filepath.Join(root, "sessions", id)
	writeFixture(t, filepath.Join(dir, "summary.json"), `{"info":{"id":"sess-1","cwd":"/work/g"},"agent_id":"sess-1","session_summary":"desk","updated_at":"2026-01-02T03:04:05Z"}`)
	writeFixture(t, filepath.Join(dir, "chat_history.jsonl"), `{"type":"user","content":"sess-1 stays in prose"}`+"\n")
	native, child := forkAndScan(t, "grok", id, filepath.Join(dir, "summary.json"))
	if filepath.Base(filepath.Dir(child.SourcePath)) != native {
		t.Fatalf("dir %s", child.SourcePath)
	}
	text := readFile(t, child.SourcePath)
	if !strings.Contains(text, `"id": "`+native+`"`) && !strings.Contains(text, `"id":"`+native+`"`) {
		t.Fatalf("%s", text)
	}
	if strings.Contains(text, `"id":"sess-1"`) || strings.Contains(text, `"id": "sess-1"`) {
		t.Fatalf("old id stayed %s", text)
	}
	if !strings.Contains(readFile(t, filepath.Join(filepath.Dir(child.SourcePath), "chat_history.jsonl")), "sess-1 stays in prose") {
		t.Fatal("history prose changed")
	}
}

func TestForkJunieAndLettaUseThePath(t *testing.T) {
	root := t.TempDir()
	t.Setenv("JUNIE_HOME", filepath.Join(root, "junie"))
	transcript := filepath.Join(root, "junie", "sessions", "session-old", "transcript.md")
	body := "# Session transcript\nuser: hello\n"
	writeFixture(t, transcript, body)
	native, child := forkAndScan(t, "junie", "session-old", transcript)
	if !strings.HasPrefix(native, "session-") || native == "session-old" {
		t.Fatalf("native %s", native)
	}
	if readFile(t, child.SourcePath) != body || readFile(t, transcript) != body {
		t.Fatal("transcript bytes changed")
	}

	t.Setenv("LETTA_HOME", filepath.Join(root, "letta"))
	letta := filepath.Join(root, "letta", "transcripts", "agent-a", "conv-a", "transcript.jsonl")
	line := `{"message":{"role":"user","content":"hello"},"id":"agent-a"}` + "\n"
	writeFixture(t, letta, line)
	native, child = forkAndScan(t, "letta", "agent-a:conv-a", letta)
	if !strings.HasPrefix(native, "agent-a:") || native == "agent-a:conv-a" {
		t.Fatalf("native %s", native)
	}
	if readFile(t, child.SourcePath) != line {
		t.Fatalf("letta content changed %s", readFile(t, child.SourcePath))
	}
}

func TestForkRefusesUnsafeCopies(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", root)
	id := "22222222-2222-2222-2222-222222222222"
	path := filepath.Join(root, "projects", "demo", id+".jsonl")
	writeFixture(t, path, `{"type":"user","message":{"content":"hi"},"sessionId":"`+id+`"}`+"\n")
	link := filepath.Join(root, "projects", "demo", "link.jsonl")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Copy(model.Session{Agent: "claude", NativeID: id, SourcePath: link}, Options{}); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatal(err)
	}

	outside := filepath.Join(t.TempDir(), "out.jsonl")
	writeFixture(t, outside, `{"type":"user","message":{"content":"x"}}`+"\n")
	if _, err := Copy(model.Session{Agent: "claude", NativeID: id, SourcePath: outside}, Options{}); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatal(err)
	}

	for _, agent := range []string{"amp", "hermes", "antigravity", "opencode"} {
		_, err := Copy(model.Session{Agent: agent, NativeID: "t1", SourcePath: path}, Options{})
		if err == nil || !strings.Contains(err.Error(), "not available") {
			t.Fatalf("%s %v", agent, err)
		}
	}

	fixed := "33333333-3333-3333-3333-333333333333"
	prev := newForkID
	newForkID = func() string { return fixed }
	t.Cleanup(func() { newForkID = prev })
	taken := filepath.Join(root, "projects", "demo", fixed+".jsonl")
	writeFixture(t, taken, "taken\n")
	if _, err := Copy(model.Session{Agent: "claude", NativeID: id, SourcePath: path}, Options{}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatal(err)
	}
	if readFile(t, taken) != "taken\n" || !strings.Contains(readFile(t, path), id) {
		t.Fatal("existing file or parent was overwritten")
	}
}

func TestForkGrokRefusesASymlinkInsideTheSession(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GROK_HOME", root)
	id := "sess-1"
	dir := filepath.Join(root, "sessions", id)
	writeFixture(t, filepath.Join(dir, "summary.json"), `{"info":{"id":"sess-1"},"session_summary":"desk"}`)
	if err := os.Symlink(filepath.Join(dir, "summary.json"), filepath.Join(dir, "extra")); err != nil {
		t.Fatal(err)
	}
	_, err := Copy(model.Session{Agent: "grok", NativeID: id, SourcePath: filepath.Join(dir, "summary.json")}, Options{})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "sessions"))
	if err != nil || len(entries) != 1 || entries[0].Name() != id {
		t.Fatalf("sessions dir %v %v", entries, err)
	}
}

func forkAndScan(t *testing.T, agent, native, path string) (string, model.Session) {
	t.Helper()
	before, err := snapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Copy(model.Session{Agent: agent, NativeID: native, SourcePath: path}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got == "" || got == native {
		t.Fatalf("native %q", got)
	}
	after, err := snapshot(path)
	if err != nil || !sameSnapshot(before, after) {
		t.Fatalf("parent changed %v", err)
	}
	var child model.Session
	n := 0
	for _, s := range scanAgent(t, agent) {
		if s.NativeID == got {
			child = s
		}
		if s.NativeID == native || s.NativeID == got {
			n++
		}
	}
	if child.NativeID != got || n < 2 {
		t.Fatalf("scan child %q matches %d", child.NativeID, n)
	}
	return got, child
}

func scanAgent(t *testing.T, agent string) []model.Session {
	t.Helper()
	for _, sc := range discover.Scanners() {
		if sc.Agent != agent {
			continue
		}
		b := sc.Scan(nil)
		if b.Err != nil {
			t.Fatal(b.Err)
		}
		return b.Sessions
	}
	t.Fatalf("no scanner %s", agent)
	return nil
}

func writeFixture(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func snapshot(path string) (map[string][]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	out := map[string][]byte{}
	if !info.IsDir() {
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		out["."] = body
		return out, nil
	}
	err = filepath.WalkDir(path, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(path, p)
		if err != nil {
			return err
		}
		out[rel] = body
		return nil
	})
	return out, err
}

func sameSnapshot(a, b map[string][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if !bytes.Equal(v, b[k]) {
			return false
		}
	}
	return true
}

func TestForkWorktreeUsesANewCheckout(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", root)
	repo := filepath.Join(root, "repo")
	sub := filepath.Join(repo, "pkg")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=ai9s",
			"GIT_AUTHOR_EMAIL=ai9s@example.com",
			"GIT_COMMITTER_NAME=ai9s",
			"GIT_COMMITTER_EMAIL=ai9s@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	git("init")
	git("commit", "--allow-empty", "-m", "init")
	id := "abc"
	body := `{"type":"user","cwd":"` + sub + `","gitBranch":"main","sessionId":"` + id + `","message":{"role":"user","content":"keep ` + id + ` in prose"}}` + "\n"
	path := filepath.Join(root, "projects", "demo", id+".jsonl")
	writeFixture(t, path, body)
	native, err := Copy(model.Session{
		Agent: "claude", NativeID: id, SourcePath: path, CWD: sub, Branch: "main",
	}, Options{Worktree: true, Name: "Side quest"})
	if err != nil {
		t.Fatal(err)
	}
	var child model.Session
	for _, s := range scanAgent(t, "claude") {
		if s.NativeID == native {
			child = s
		}
	}
	wantDir := filepath.Join(root, "repo.worktrees", "fork-side-quest", "pkg")
	if child.CWD != wantDir || child.Branch != "fork/side-quest" {
		t.Fatalf("child cwd %q branch %q", child.CWD, child.Branch)
	}
	text := readFile(t, child.SourcePath)
	if !strings.Contains(text, "keep "+id+" in prose") || !strings.Contains(text, `"cwd":"`+wantDir+`"`) {
		t.Fatalf("%s", text)
	}
	if readFile(t, path) != body {
		t.Fatal("parent changed")
	}
	if _, err := os.Stat(filepath.Join(root, "repo.worktrees", "fork-side-quest", ".git")); err != nil {
		t.Fatal(err)
	}

	_, err = Copy(model.Session{Agent: "claude", NativeID: id, SourcePath: path, CWD: filepath.Join(root, "plain")}, Options{Worktree: true, Name: "nope"})
	if err == nil || !strings.Contains(err.Error(), "git checkout") {
		t.Fatal(err)
	}
	_, err = Copy(model.Session{Agent: "junie", NativeID: "session-old", SourcePath: path, CWD: repo}, Options{Worktree: true, Name: "nope"})
	if err == nil || !strings.Contains(err.Error(), "does not record a directory") {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

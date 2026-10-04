package act

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AymanZahran/ai9s/internal/model"
)

func TestDeleteKimiQwenMistralMinimax(t *testing.T) {
	root := t.TempDir()
	t.Setenv("KIMI_CODE_HOME", filepath.Join(root, "kimi-code"))
	t.Setenv("QWEN_RUNTIME_DIR", filepath.Join(root, "qwen"))
	t.Setenv("VIBE_HOME", filepath.Join(root, "vibe"))
	t.Setenv("MINIMAX_DATA_DIR", filepath.Join(root, "minimax"))

	home := filepath.Join(root, "kimi-code")
	bucket := filepath.Join(home, "sessions", "wd_work_aaaaaaaaaaaa")
	k1 := filepath.Join(bucket, "k1")
	k2 := filepath.Join(bucket, "k2")
	writeRaw(t, filepath.Join(k1, "state.json"), `{"id":"k1","title":"plan the port"}`)
	writeRaw(t, filepath.Join(k2, "state.json"), `{"id":"k2","title":"keep"}`)
	writeRaw(t, filepath.Join(bucket, "keep.txt"), "stay")
	k1Line, err := json.Marshal(map[string]string{"sessionId": "k1", "sessionDir": k1})
	if err != nil {
		t.Fatal(err)
	}
	k2Line, err := json.Marshal(map[string]string{"sessionId": "k2", "sessionDir": k2})
	if err != nil {
		t.Fatal(err)
	}
	index := filepath.Join(home, "session_index.jsonl")
	writeRaw(t, index, string(k1Line)+"\n"+string(k2Line)+"\n")
	if err := Delete(model.Session{Agent: "kimi", NativeID: "k1", CanDelete: true, DeleteMode: "kimi", SourcePath: filepath.Join(k1, "state.json")}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(k1); !os.IsNotExist(err) {
		t.Fatal("kimi session still exists")
	}
	if _, err := os.Stat(filepath.Join(k2, "state.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(bucket, "keep.txt")); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), `"sessionId":"k1"`) || !strings.Contains(string(body), `"sessionId":"k2"`) {
		t.Fatalf("index %s", body)
	}
	outside := filepath.Join(t.TempDir(), "state.json")
	writeRaw(t, outside, `{"id":"k2"}`)
	if err := Delete(model.Session{Agent: "kimi", NativeID: "k2", CanDelete: true, DeleteMode: "kimi", SourcePath: outside}); err == nil {
		t.Fatal("deleted a kimi session outside the session root")
	}
	link := filepath.Join(bucket, "linked")
	if err := os.Symlink(k2, link); err != nil {
		t.Fatal(err)
	}
	if err := Delete(model.Session{Agent: "kimi", NativeID: "k2", CanDelete: true, DeleteMode: "kimi", SourcePath: filepath.Join(link, "state.json")}); err == nil {
		t.Fatal("deleted a kimi session through a symlink")
	}
	if _, err := os.Stat(filepath.Join(k2, "state.json")); err != nil {
		t.Fatal(err)
	}

	id := "0123456789abcdef0123456789abcdef"
	chats := filepath.Join(root, "qwen", "projects", "-work-qwen", "chats")
	chat := filepath.Join(chats, id+".jsonl")
	writeRaw(t, chat, "{}\n")
	writeRaw(t, filepath.Join(chats, ".runtime.json"), "{}")
	archived := filepath.Join(chats, "archive", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.jsonl")
	writeRaw(t, archived, "{}\n")
	if err := Delete(model.Session{Agent: "qwen", NativeID: id, CanDelete: true, DeleteMode: "qwen", SourcePath: chat}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(chat); !os.IsNotExist(err) {
		t.Fatal("qwen chat still exists")
	}
	if _, err := os.Stat(filepath.Join(chats, ".runtime.json")); err != nil {
		t.Fatal(err)
	}
	outsideChat := filepath.Join(t.TempDir(), id+".jsonl")
	writeRaw(t, outsideChat, "{}\n")
	if err := Delete(model.Session{Agent: "qwen", NativeID: id, CanDelete: true, DeleteMode: "qwen", SourcePath: outsideChat}); err == nil {
		t.Fatal("deleted a qwen chat outside the runtime directory")
	}
	linkChat := filepath.Join(chats, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.jsonl")
	if err := os.Symlink(archived, linkChat); err != nil {
		t.Fatal(err)
	}
	if err := Delete(model.Session{Agent: "qwen", NativeID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", CanDelete: true, DeleteMode: "qwen", SourcePath: linkChat}); err == nil {
		t.Fatal("deleted a symlinked qwen chat")
	}
	if err := Delete(model.Session{Agent: "qwen", NativeID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", CanDelete: true, DeleteMode: "qwen", SourcePath: archived}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(archived); !os.IsNotExist(err) {
		t.Fatal("archived qwen chat still exists")
	}

	vibe := filepath.Join(root, "vibe", "logs", "session")
	drop := filepath.Join(vibe, "drop")
	keep := filepath.Join(vibe, "keep")
	writeRaw(t, filepath.Join(drop, "meta.json"), `{"session_id":"drop"}`)
	writeRaw(t, filepath.Join(drop, "messages.jsonl"), "{}\n")
	writeRaw(t, filepath.Join(keep, "meta.json"), `{"session_id":"keep"}`)
	writeRaw(t, filepath.Join(keep, "messages.jsonl"), "{}\n")
	if err := Delete(model.Session{Agent: "mistral", NativeID: "drop", CanDelete: true, DeleteMode: "mistral", SourcePath: filepath.Join(drop, "messages.jsonl")}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(drop); !os.IsNotExist(err) {
		t.Fatal("mistral session still exists")
	}
	if _, err := os.Stat(filepath.Join(keep, "messages.jsonl")); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(t.TempDir(), "drop")
	writeRaw(t, filepath.Join(outDir, "meta.json"), `{"session_id":"drop"}`)
	writeRaw(t, filepath.Join(outDir, "messages.jsonl"), "{}\n")
	if err := Delete(model.Session{Agent: "mistral", NativeID: "drop", CanDelete: true, DeleteMode: "mistral", SourcePath: filepath.Join(outDir, "messages.jsonl")}); err == nil {
		t.Fatal("deleted a mistral session outside the save directory")
	}
	linked := filepath.Join(vibe, "linked")
	if err := os.Symlink(keep, linked); err != nil {
		t.Fatal(err)
	}
	if err := Delete(model.Session{Agent: "mistral", NativeID: "keep", CanDelete: true, DeleteMode: "mistral", SourcePath: filepath.Join(linked, "messages.jsonl")}); err == nil {
		t.Fatal("deleted a mistral session through a symlink")
	}

	dbPath := filepath.Join(root, "minimax", "v2", "sqlite", "runtime-state.sqlite")
	execSQLite(t, dbPath,
		`CREATE TABLE local_runtime_sessions (session_id TEXT PRIMARY KEY, history_relative_dir TEXT)`,
		`CREATE TABLE local_runtime_token_usage (id INTEGER PRIMARY KEY, session_id TEXT, input_tokens INTEGER)`,
		`INSERT INTO local_runtime_sessions VALUES ('mm-1','aa/bb/cc/mm1')`,
		`INSERT INTO local_runtime_sessions VALUES ('mm-2','aa/bb/cc/mm2')`,
		`INSERT INTO local_runtime_sessions VALUES ('bad','../x/y/z')`,
		`INSERT INTO local_runtime_token_usage VALUES (1,'mm-1',4)`,
		`INSERT INTO local_runtime_token_usage VALUES (2,'mm-2',8)`,
	)
	mm1 := filepath.Join(root, "minimax", "v2", "sessions", "aa", "bb", "cc", "mm1")
	mm2 := filepath.Join(root, "minimax", "v2", "sessions", "aa", "bb", "cc", "mm2")
	writeRaw(t, filepath.Join(mm1, "manifest.json"), `{"sessionId":"mm-1"}`)
	writeRaw(t, filepath.Join(mm1, "messages.jsonl"), "{}\n")
	writeRaw(t, filepath.Join(mm2, "manifest.json"), `{"sessionId":"mm-2"}`)
	writeRaw(t, filepath.Join(mm2, "messages.jsonl"), "{}\n")
	writeRaw(t, filepath.Join(root, "minimax", "v2", "sessions", "aa", "bb", "cc", "keep.txt"), "stay")
	if err := Delete(model.Session{Agent: "minimax", NativeID: "bad", CanDelete: true, DeleteMode: "minimax", SourcePath: dbPath}); err == nil {
		t.Fatal("deleted a minimax session with a history path outside the session root")
	}
	if err := Delete(model.Session{Agent: "minimax", NativeID: "mm-1", CanDelete: true, DeleteMode: "minimax", SourcePath: dbPath}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(mm1); !os.IsNotExist(err) {
		t.Fatal("minimax history still exists")
	}
	if _, err := os.Stat(filepath.Join(mm2, "manifest.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "minimax", "v2", "sessions", "aa", "bb", "cc", "keep.txt")); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM local_runtime_sessions WHERE session_id = 'mm-1'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("mm-1 rows %d %v", n, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM local_runtime_sessions WHERE session_id = 'mm-2'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("mm-2 rows %d %v", n, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM local_runtime_sessions WHERE session_id = 'bad'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("bad rows %d %v", n, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM local_runtime_token_usage WHERE session_id = 'mm-1'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("mm-1 usage %d %v", n, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM local_runtime_token_usage WHERE session_id = 'mm-2'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("mm-2 usage %d %v", n, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	manifest := filepath.Join(root, "minimax", "v2", "sessions", "aa", "bb", "cc", "file")
	writeRaw(t, filepath.Join(manifest, "manifest.json"), `{"sessionId":"file-1"}`)
	writeRaw(t, filepath.Join(manifest, "messages.jsonl"), "{}\n")
	if err := Delete(model.Session{Agent: "minimax", NativeID: "file-1", CanDelete: true, DeleteMode: "minimax", SourcePath: filepath.Join(manifest, "messages.jsonl")}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(manifest); !os.IsNotExist(err) {
		t.Fatal("manifest session still exists")
	}
	if _, err := os.Stat(mm2); err != nil {
		t.Fatal(err)
	}
}

func writeRaw(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func execSQLite(t *testing.T, path string, stmts ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
}

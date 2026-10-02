package discover

import (
	"database/sql"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestGrokUsageFiles(t *testing.T) {
	root := isolate(t)
	dir := filepath.Join(root, "grok", "sessions", "proj", "sess-1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("summary.json", `{"info":{"id":"sess-1","cwd":"/work/g"},"session_summary":"desk","current_model_id":"grok","reasoning_effort":"high","updated_at":"2026-01-02T03:04:05Z"}`)
	write("usage.json", `{"session":{"inputTokens":100,"outputTokens":20,"cachedReadTokens":5,"cacheCreationTokens":1,"reasoningTokens":7,"totalTokens":120,"costUsdTicks":189162400},"sessionId":"sess-1"}`)
	write("signals.json", `{"contextTokensUsed":1549,"contextWindowTokens":256000,"contextWindowUsage":12}`)
	batch := scanGrok(nil)
	if batch.Err != nil || len(batch.Sessions) != 1 {
		t.Fatalf("grok %+v %v", len(batch.Sessions), batch.Err)
	}
	u := batch.Sessions[0].Usage
	if u.Context != 1549 || u.Window != 256000 || u.Input != 100 || u.Output != 20 ||
		u.CacheRead != 5 || u.CacheWrite != 1 || u.Reasoning != 7 || u.Total != 120 ||
		u.Effort != "high" || u.CostUSD != 189162400/1e10 {
		t.Fatalf("grok usage %+v", u)
	}
}

func TestCursorComposerUsage(t *testing.T) {
	root := isolate(t)
	dbPath := filepath.Join(root, "cursor-state.vscdb")
	t.Setenv("CURSOR_STATE_DB", dbPath)
	execDB(t, dbPath,
		`CREATE TABLE cursorDiskKV (key TEXT PRIMARY KEY, value TEXT)`,
		`INSERT INTO cursorDiskKV VALUES ('composerData:conv-1','{"contextTokensUsed":26387,"contextTokenLimit":256000,"promptTokenBreakdown":{"totalUsedTokens":1,"maxTokens":1},"note":"not a token"}')`,
		`INSERT INTO cursorDiskKV VALUES ('composerData:conv-2','{"promptTokenBreakdown":{"totalUsedTokens":20200,"maxTokens":256000}}')`,
	)
	for _, id := range []string{"conv-1", "conv-2"} {
		dir := filepath.Join(root, "cursor", "projects", "demo", "agent-transcripts", id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body := `{"role":"user","message":{"content":"fix the button"}}` + "\n"
		if err := os.WriteFile(filepath.Join(dir, id+".jsonl"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	batch := scanCursor(nil)
	if batch.Err != nil || len(batch.Sessions) != 2 {
		t.Fatalf("cursor %+v %v", len(batch.Sessions), batch.Err)
	}
	got := map[string][2]int{}
	for _, s := range batch.Sessions {
		got[s.NativeID] = [2]int{s.Usage.Context, s.Usage.Window}
		if s.Usage.Total != 0 || s.Usage.Input != 0 {
			t.Fatalf("cursor tokens %+v", s.Usage)
		}
	}
	if got["conv-1"] != [2]int{26387, 256000} || got["conv-2"] != [2]int{20200, 256000} {
		t.Fatalf("cursor context %+v", got)
	}
}

func TestAgyGenMetadata(t *testing.T) {
	root := isolate(t)
	hist := filepath.Join(root, "gemini", "antigravity-cli", "history.jsonl")
	if err := os.MkdirAll(filepath.Dir(hist), 0o755); err != nil {
		t.Fatal(err)
	}
	line := `{"conversationId":"c1","display":"fix the tests","timestamp":1700000000000,"type":"user","workspace":"/work/app"}` + "\n"
	if err := os.WriteFile(hist, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(filepath.Dir(hist), "conversations", "c1.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE gen_metadata (idx INTEGER, data BLOB, size INTEGER)`); err != nil {
		t.Fatal(err)
	}
	blobs := [][]byte{agyBlob(28871, 128000, 5355, 283, 16253, 226, true), {0x0f}, agyBlob(71427, 128000, 10, 2, 3, 4, false)}
	for i, blob := range blobs {
		if _, err := db.Exec(`INSERT INTO gen_metadata(idx, data, size) VALUES (?,?,?)`, i, blob, len(blob)); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	batch := scanAgy(nil)
	if batch.Err != nil || len(batch.Sessions) != 1 {
		t.Fatalf("agy %+v %v", len(batch.Sessions), batch.Err)
	}
	u := batch.Sessions[0].Usage
	if u.Context != 71427 || u.Window != 128000 || u.Input != 5365 || u.Output != 285 ||
		u.CacheRead != 16256 || u.Reasoning != 230 || u.Total != 0 {
		t.Fatalf("agy usage %+v", u)
	}
}

func agyBlob(ctx, window, in, out, cache, reason int, withText bool) []byte {
	stats := pbVarint(2, in)
	stats = append(stats, pbVarint(3, out)...)
	stats = append(stats, pbVarint(5, cache)...)
	stats = append(stats, pbVarint(6, 24)...)
	stats = append(stats, pbVarint(9, reason)...)
	occupancy := append(pbVarint(1, ctx), pbVarint(4, window)...)
	inner := pbBytes(9, pbBytes(10, occupancy))
	if withText {
		inner = append(pbBytes(8, []byte(strings.Repeat("please rename the button. ", 8))), inner...)
	}
	inner = append(inner, pbBytes(4, stats)...)
	inner = append(inner, pbBytes(12, pbVarint(2, 48_000_000))...)
	return pbBytes(1, inner)
}

func pbVarint(field, v int) []byte {
	key := binary.AppendUvarint(nil, uint64(field)<<3)
	return append(key, binary.AppendUvarint(nil, uint64(v))...)
}

func pbBytes(field int, payload []byte) []byte {
	key := binary.AppendUvarint(nil, uint64(field)<<3|2)
	key = binary.AppendUvarint(key, uint64(len(payload)))
	return append(key, payload...)
}

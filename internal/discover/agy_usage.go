package discover

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"

	"github.com/AymanZahran/ai9s/internal/model"
)

// Antigravity keeps per-generation accounting in conversations/<id>.db,
// table gen_metadata, as protobuf. The latest prompt occupancy is field
// path 1.9.10.1 and the window is 1.9.10.4. Per-generation tokens are
// 1.4.2 input, 1.4.3 output, 1.4.5 cache read, and 1.4.9 reasoning.
// Field 1.4.6 and values above 20 million are not token counts.
// history.jsonl stays the session list. A conversation with no database
// keeps a dash.

const agyMaxToken = 20_000_000

func agyConversationDBs(history string) []string {
	dir := filepath.Join(filepath.Dir(history), "conversations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".db") {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	return out
}

func agyUsage(historyPath, id string) model.Usage {
	dbPath := filepath.Join(filepath.Dir(historyPath), "conversations", id+".db")
	st, err := os.Stat(dbPath)
	if err != nil || st.IsDir() {
		return model.Usage{}
	}
	db, err := openDB(dbPath)
	if err != nil {
		return model.Usage{}
	}
	defer db.Close()
	rows, err := db.Query(`SELECT data FROM gen_metadata ORDER BY idx`)
	if err != nil {
		return model.Usage{}
	}
	defer rows.Close()
	var u model.Usage
	for rows.Next() {
		var blob []byte
		if err := rows.Scan(&blob); err != nil {
			continue
		}
		addAgyBlob(&u, blob)
	}
	_ = rows.Err()
	return u
}

func addAgyBlob(u *model.Usage, blob []byte) {
	var in, out, cache, reason, ctx, window int
	walkProto(blob, nil, func(path []int, n int64) {
		v := saneCount(n)
		if v == 0 {
			return
		}
		switch {
		case pathEq(path, 1, 9, 10, 1):
			ctx = v
		case pathEq(path, 1, 9, 10, 4):
			window = v
		case pathEq(path, 1, 4, 2):
			in += v
		case pathEq(path, 1, 4, 3):
			out += v
		case pathEq(path, 1, 4, 5):
			cache += v
		case pathEq(path, 1, 4, 9):
			reason += v
		}
	})
	u.Input += in
	u.Output += out
	u.CacheRead += cache
	u.Reasoning += reason
	if ctx > 0 {
		u.Context = ctx
	}
	if window > 0 {
		u.Window = window
	}
}

func saneCount(n int64) int {
	if n <= 0 || n > agyMaxToken {
		return 0
	}
	return int(n)
}

func pathEq(path []int, want ...int) bool {
	if len(path) != len(want) {
		return false
	}
	for i := range path {
		if path[i] != want[i] {
			return false
		}
	}
	return true
}

type protoHit struct {
	path []int
	n    int64
}

// walkProto visits varints and reports whether the buffer was a complete
// message. A length-delimited child that fails to parse does not drop the
// parent's later fields. A printable chunk is prompt text and is ignored,
// unless it is a complete message that itself carries a token field.
// The path slice is copied on each step.
func walkProto(buf []byte, path []int, visit func([]int, int64)) bool {
	i := 0
	for i < len(buf) {
		tag, n := binary.Uvarint(buf[i:])
		if n <= 0 {
			return false
		}
		i += n
		field := int(tag >> 3)
		wire := int(tag & 7)
		if field <= 0 {
			return false
		}
		switch wire {
		case 0:
			v, m := binary.Uvarint(buf[i:])
			if m <= 0 {
				return false
			}
			i += m
			visit(appendPath(path, field), int64(v))
		case 1:
			if i+8 > len(buf) {
				return false
			}
			i += 8
		case 5:
			if i+4 > len(buf) {
				return false
			}
			i += 4
		case 2:
			ln, m := binary.Uvarint(buf[i:])
			if m <= 0 || ln > uint64(len(buf)-i) {
				return false
			}
			i += m
			chunk := buf[i : i+int(ln)]
			i += int(ln)
			child := appendPath(path, field)
			if mostlyText(chunk) {
				var sub []protoHit
				full := walkProto(chunk, child, func(p []int, n int64) {
					sub = append(sub, protoHit{p, n})
				})
				if full && hitsToken(sub) {
					for _, h := range sub {
						visit(h.path, h.n)
					}
				}
				continue
			}
			walkProto(chunk, child, visit)
		default:
			return false
		}
	}
	return true
}

func hitsToken(sub []protoHit) bool {
	for _, h := range sub {
		switch {
		case pathEq(h.path, 1, 9, 10, 1), pathEq(h.path, 1, 9, 10, 4),
			pathEq(h.path, 1, 4, 2), pathEq(h.path, 1, 4, 3),
			pathEq(h.path, 1, 4, 5), pathEq(h.path, 1, 4, 9):
			return true
		}
	}
	return false
}

func appendPath(path []int, field int) []int {
	out := make([]int, len(path)+1)
	copy(out, path)
	out[len(path)] = field
	return out
}

func mostlyText(b []byte) bool {
	if len(b) < 16 {
		return false
	}
	text := 0
	for _, c := range b {
		if c == '\n' || c == '\r' || c == '\t' || (c >= 32 && c < 127) {
			text++
		}
	}
	return text*5 >= len(b)*4
}

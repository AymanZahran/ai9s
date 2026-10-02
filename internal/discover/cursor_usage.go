package discover

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AymanZahran/air9s/internal/model"
)

// cursorStateDB is Cursor's global state database. Transcripts do not carry
// token counts. composerData in this database does, under the same id as the
// transcript directory. CURSOR_STATE_DB overrides the path, including when
// the file is missing, so a test does not open the live database.
func cursorStateDB() string {
	if v := strings.TrimSpace(os.Getenv("CURSOR_STATE_DB")); v != "" {
		return v
	}
	var candidates []string
	if runtime.GOOS == "darwin" {
		candidates = append(candidates, homeJoin("Library", "Application Support", "Cursor", "User", "globalStorage", "state.vscdb"))
	}
	if config := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); config != "" {
		candidates = append(candidates, filepath.Join(config, "Cursor", "User", "globalStorage", "state.vscdb"))
	}
	candidates = append(candidates, homeJoin(".config", "Cursor", "User", "globalStorage", "state.vscdb"))
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func cursorUsage(db *sql.DB, id string) model.Usage {
	var raw []byte
	err := db.QueryRow(`SELECT value FROM cursorDiskKV WHERE key = ?`, "composerData:"+id).Scan(&raw)
	if err != nil || len(raw) == 0 {
		return model.Usage{}
	}
	return usageFromComposer(raw)
}

// usageFromComposer keeps four integers. The rest of the row is prompt text
// and file paths, and it is discarded with the map.
func usageFromComposer(raw []byte) model.Usage {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return model.Usage{}
	}
	used := asInt(doc["contextTokensUsed"])
	limit := asInt(doc["contextTokenLimit"])
	var total, max int
	if breakdown, ok := doc["promptTokenBreakdown"].(map[string]any); ok {
		total = asInt(breakdown["totalUsedTokens"])
		max = asInt(breakdown["maxTokens"])
	}
	u := model.Usage{}
	if used > 0 {
		u.Context = used
	} else {
		u.Context = total
	}
	if limit > 0 {
		u.Window = limit
	} else {
		u.Window = max
	}
	return u
}

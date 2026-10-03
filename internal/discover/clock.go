package discover

import (
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/AymanZahran/ai9s/internal/sqliteuri"
	_ "modernc.org/sqlite"
)

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", sqliteuri.Path(path, "mode=ro&_pragma=busy_timeout(3000)"))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

func unixish(v float64) time.Time {
	if v <= 0 {
		return time.Time{}
	}
	if v > 1e14 {
		return time.UnixMicro(int64(v))
	}
	if v > 1e11 {
		return time.UnixMilli(int64(v))
	}
	sec, frac := int64(v), v-float64(int64(v))
	return time.Unix(sec, int64(frac*float64(time.Second)))
}

func parseTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	if n, err := strconv.ParseFloat(s, 64); err == nil {
		return unixish(n)
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02T15:04:05Z07:00",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func num(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int64:
		return int(n)
	case int:
		return n
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(n), 64)
		return int(f)
	default:
		return 0
	}
}

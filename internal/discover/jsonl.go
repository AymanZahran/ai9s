package discover

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"time"
)

// walkJSONL calls fn for each JSON object. Malformed lines are skipped.
func walkJSONL(path string, fn func(map[string]any) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(line, &obj); err != nil {
			continue
		}
		if err := fn(obj); err != nil {
			return err
		}
	}
	if err := sc.Err(); err != nil && err != io.EOF {
		return err
	}
	return nil
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.000Z"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func fileMtime(path string) int64 {
	st, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return st.ModTime().Unix()
}

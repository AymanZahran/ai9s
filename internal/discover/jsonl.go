package discover

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
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

func fileMtime(path string) int64 {
	st, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return st.ModTime().Unix()
}

// walkShallowJSONL calls fn for each regular *.jsonl file in dir and in dir's
// immediate child directories. A name that starts with "." is skipped.
// It does not walk deeper than that.
func walkShallowJSONL(dir string, fn func(path string)) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == "" || name[0] == '.' {
			continue
		}
		path := filepath.Join(dir, name)
		if entry.IsDir() {
			kids, err := os.ReadDir(path)
			if err != nil {
				continue
			}
			for _, kid := range kids {
				kn := kid.Name()
				if kn == "" || kn[0] == '.' || kid.IsDir() || !strings.HasSuffix(kn, ".jsonl") {
					continue
				}
				kp := filepath.Join(path, kn)
				if regularFile(kp) {
					fn(kp)
				}
			}
			continue
		}
		if strings.HasSuffix(name, ".jsonl") && regularFile(path) {
			fn(path)
		}
	}
}

// listTopJSONL returns regular *.jsonl files directly inside dir.
// Child directories, including an archive directory, are not walked.
func listTopJSONL(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, entry := range entries {
		name := entry.Name()
		if name == "" || name[0] == '.' || entry.IsDir() || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		path := filepath.Join(dir, name)
		if regularFile(path) {
			out = append(out, path)
		}
	}
	return out
}

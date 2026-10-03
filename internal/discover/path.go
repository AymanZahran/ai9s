package discover

import (
	"os"
	"path/filepath"
	"strings"
)

// decodeDashedPath recovers a working directory from the dashed folder name
// Claude and Cursor use. Both encodings replace the path separator with "-".
// Claude also keeps a leading "-" for the root slash. The decode is ambiguous
// when a path component itself contains "-", so a candidate is returned only
// when that directory exists.
func decodeDashedPath(encoded string) string {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return ""
	}
	body := strings.TrimPrefix(encoded, "-")
	if p := searchDashed(body, true); p != "" {
		return p
	}
	if strings.HasPrefix(encoded, "-") {
		return ""
	}
	return searchDashed(body, false)
}

func searchDashed(body string, absolute bool) string {
	parts := strings.Split(body, "-")
	if len(parts) == 0 || parts[0] == "" {
		return ""
	}
	start := parts[0]
	if absolute {
		start = string(os.PathSeparator) + parts[0]
	}
	return consumeDashed(start, parts[1:])
}

func consumeDashed(dir string, parts []string) string {
	if len(parts) == 0 {
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			return dir
		}
		return ""
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return ""
	}
	for n := len(parts); n >= 1; n-- {
		next := filepath.Join(dir, strings.Join(parts[:n], "-"))
		if n == len(parts) {
			if st, err := os.Stat(next); err == nil && st.IsDir() {
				return next
			}
			continue
		}
		if st, err := os.Stat(next); err == nil && st.IsDir() {
			if got := consumeDashed(next, parts[n:]); got != "" {
				return got
			}
		}
	}
	return ""
}

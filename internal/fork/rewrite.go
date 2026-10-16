package fork

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// idKeys are JSON fields that hold a session id. parentId is not one of them,
// so a parent pointer inside a transcript stays pointed at the original.
var idKeys = map[string]struct{}{
	"id":             {},
	"session_id":     {},
	"sessionId":      {},
	"conversationId": {},
	"agent_id":       {},
	"thread_id":      {},
}

// rewriteJSONValues replaces string values of idKeys when the value is oldID.
// Other text, including a longer string that merely contains oldID, is copied
// through with its original bytes.
func rewriteJSONValues(in, oldID, newID string, swaps []textSwap) (string, int) {
	if oldID == "" || oldID == newID {
		return in, 0
	}
	var out strings.Builder
	out.Grow(len(in) + len(newID))
	replaced := 0
	key := ""
	expectValue := false
	for i := 0; i < len(in); {
		if in[i] != '"' {
			c := in[i]
			out.WriteByte(c)
			if expectValue {
				switch c {
				case ' ', '\t', '\n', '\r', ':':
				default:
					expectValue = false
					key = ""
				}
			}
			i++
			continue
		}
		raw, val, next, ok := readJSONString(in, i)
		if !ok {
			out.WriteByte(in[i])
			i++
			continue
		}
		if expectValue {
			if _, known := idKeys[key]; known && val == oldID {
				out.WriteString(strconv.Quote(newID))
				replaced++
			} else if swapped, ok := swapValue(key, val, swaps); ok {
				out.WriteString(strconv.Quote(swapped))
			} else {
				out.WriteString(raw)
			}
			expectValue = false
			key = ""
			i = next
			continue
		}
		j := skipSpace(in, next)
		if j < len(in) && in[j] == ':' {
			key = val
			expectValue = true
		}
		out.WriteString(raw)
		i = next
	}
	return out.String(), replaced
}

func swapValue(key, val string, swaps []textSwap) (string, bool) {
	for _, s := range swaps {
		if _, ok := s.keys[key]; !ok || s.old == "" || s.old == s.new {
			continue
		}
		if val == s.old || filepath.Clean(val) == filepath.Clean(s.old) {
			return s.new, true
		}
	}
	return "", false
}

func skipSpace(in string, i int) int {
	for i < len(in) {
		switch in[i] {
		case ' ', '\t', '\n', '\r':
			i++
		default:
			return i
		}
	}
	return i
}

// readJSONString returns the raw token, the unescaped value, and the index
// after the closing quote.
func readJSONString(in string, i int) (raw, val string, next int, ok bool) {
	if i >= len(in) || in[i] != '"' {
		return "", "", i, false
	}
	var b strings.Builder
	j := i + 1
	for j < len(in) {
		c := in[j]
		if c == '\\' {
			if j+1 >= len(in) {
				return "", "", i, false
			}
			switch in[j+1] {
			case '"', '\\', '/':
				b.WriteByte(in[j+1])
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case 'u':
				if j+6 > len(in) {
					return "", "", i, false
				}
				r, err := strconv.ParseUint(in[j+2:j+6], 16, 16)
				if err != nil {
					return "", "", i, false
				}
				b.WriteRune(rune(r))
				j += 6
				continue
			default:
				return "", "", i, false
			}
			j += 2
			continue
		}
		if c == '"' {
			return in[i : j+1], b.String(), j + 1, true
		}
		if c < 0x20 {
			return "", "", i, false
		}
		b.WriteByte(c)
		j++
	}
	return "", "", i, false
}

func rewriteWorkspaceYAML(body, oldID, newID string, swaps []textSwap) (string, bool) {
	lines := strings.Split(body, "\n")
	changed := false
	for i, line := range lines {
		raw := strings.TrimRight(line, "\r")
		cr := ""
		if strings.HasSuffix(line, "\r") {
			cr = "\r"
		}
		trim := strings.TrimSpace(raw)
		key, val, ok := strings.Cut(trim, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		indent := raw[:len(raw)-len(strings.TrimLeft(raw, " \t"))]
		if key == "id" && oldID != "" && val == oldID {
			lines[i] = indent + "id: " + newID + cr
			changed = true
			continue
		}
		if swapped, ok := swapValue(key, val, swaps); ok {
			lines[i] = indent + key + ": " + quoteYAML(swapped) + cr
			changed = true
		}
	}
	return strings.Join(lines, "\n"), changed
}

func quoteYAML(val string) string {
	if strings.ContainsAny(val, ":# \t") {
		return strconv.Quote(val)
	}
	return val
}

func firstYAMLID(body string) (string, bool) {
	for _, line := range strings.Split(body, "\n") {
		trim := strings.TrimSpace(strings.TrimRight(line, "\r"))
		if !strings.HasPrefix(trim, "id:") {
			continue
		}
		val := strings.TrimSpace(strings.TrimPrefix(trim, "id:"))
		val = strings.Trim(val, `"'`)
		if val == "" {
			return "", false
		}
		return val, true
	}
	return "", false
}

func rewriteJSONL(src, dst, oldID, newID string, mode os.FileMode, swaps []textSwap) (int, error) {
	in, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer in.Close()
	return writeNew(dst, mode, func(w io.Writer) (int, error) {
		sc := bufio.NewScanner(in)
		sc.Buffer(make([]byte, 64*1024), 32*1024*1024)
		replaced := 0
		for sc.Scan() {
			line, n := rewriteJSONValues(sc.Text(), oldID, newID, swaps)
			replaced += n
			if _, err := io.WriteString(w, line); err != nil {
				return 0, err
			}
			if _, err := w.Write([]byte{'\n'}); err != nil {
				return 0, err
			}
		}
		if err := sc.Err(); err != nil {
			return 0, err
		}
		return replaced, nil
	})
}

func rewriteJSONFile(src, dst, oldID, newID string, mode os.FileMode, swaps []textSwap) (int, error) {
	body, err := os.ReadFile(src)
	if err != nil {
		return 0, err
	}
	text, n := rewriteJSONValues(string(body), oldID, newID, swaps)
	return writeNew(dst, mode, func(w io.Writer) (int, error) {
		_, err := io.WriteString(w, text)
		return n, err
	})
}

func rewriteYAMLFile(src, dst, oldID, newID string, mode os.FileMode, swaps []textSwap) (int, error) {
	body, err := os.ReadFile(src)
	if err != nil {
		return 0, err
	}
	text, changed := rewriteWorkspaceYAML(string(body), oldID, newID, swaps)
	n := 0
	if changed {
		n = 1
	}
	return writeNew(dst, mode, func(w io.Writer) (int, error) {
		_, err := io.WriteString(w, text)
		return n, err
	})
}

func copyBytes(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	_, err = writeNew(dst, mode, func(w io.Writer) (int, error) {
		_, err := io.Copy(w, in)
		return 0, err
	})
	return err
}

func writeNew(dst string, mode os.FileMode, write func(io.Writer) (int, error)) (int, error) {
	if mode == 0 {
		mode = 0o600
	}
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return 0, err
	}
	n, werr := write(f)
	cerr := f.Close()
	if werr == nil {
		werr = cerr
	}
	if werr != nil {
		_ = os.Remove(dst)
		return 0, werr
	}
	return n, nil
}

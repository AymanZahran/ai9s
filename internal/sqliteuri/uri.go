// Package sqliteuri builds a file URL that modernc.org/sqlite accepts.
// A Windows path passed as file:C:\... is read as a URI authority and rejected.
package sqliteuri

import (
	"net/url"
	"path/filepath"
	"strings"
)

// Path returns a file URL for a SQLite database. rawQuery is the unescaped
// query, such as a mode and busy timeout.
func Path(path, rawQuery string) string {
	p := filepath.ToSlash(path)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return (&url.URL{Scheme: "file", Path: p, RawQuery: rawQuery}).String()
}

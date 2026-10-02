package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/AymanZahran/ai9s/internal/model"
	"github.com/AymanZahran/ai9s/internal/query"
	_ "modernc.org/sqlite"
)

const cols = `s.id, s.native_id, s.agent, s.title, s.summary, s.cwd, s.branch, s.model, s.updated, s.messages, s.source_path, s.source_mtime, s.can_delete, s.delete_mode, s.delete_reason, s.usage`

// Source is one on-disk file or directory the indexer visited.
type Source struct {
	Path  string
	Mtime int64
	Fresh bool
}

// AgentStat is one row of ai9s stats.
type AgentStat struct {
	Agent    string    `json:"agent"`
	Sessions int       `json:"sessions"`
	Messages int       `json:"messages"`
	Oldest   time.Time `json:"oldest,omitempty"`
	Newest   time.Time `json:"newest,omitempty"`
}

// Stats is the indexed corpus.
type Stats struct {
	Sessions int         `json:"sessions"`
	Messages int         `json:"messages"`
	Agents   []AgentStat `json:"agents"`
}

// Store is the local session index.
type Store struct {
	mu  sync.Mutex
	db  *sql.DB
	fts bool
}

// DefaultPath is $AI9S_CACHE_DIR/index.db, or $XDG_CACHE_HOME/ai9s/index.db,
// or ~/.cache/ai9s/index.db.
func DefaultPath() (string, error) {
	if v := strings.TrimSpace(os.Getenv("AI9S_CACHE_DIR")); v != "" {
		return filepath.Join(v, "index.db"), nil
	}
	base := strings.TrimSpace(os.Getenv("XDG_CACHE_HOME"))
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return "", fmt.Errorf("home directory is not set")
		}
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "ai9s", "index.db"), nil
}

// Open creates the index database if needed.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: path, RawQuery: "_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"}
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS sessions (
  id TEXT PRIMARY KEY,
  native_id TEXT NOT NULL,
  agent TEXT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  summary TEXT NOT NULL DEFAULT '',
  cwd TEXT NOT NULL DEFAULT '',
  branch TEXT NOT NULL DEFAULT '',
  model TEXT NOT NULL DEFAULT '',
  updated INTEGER NOT NULL DEFAULT 0,
  messages INTEGER NOT NULL DEFAULT 0,
  source_path TEXT NOT NULL DEFAULT '',
  source_mtime INTEGER NOT NULL DEFAULT 0,
  can_delete INTEGER NOT NULL DEFAULT 0,
  delete_mode TEXT NOT NULL DEFAULT '',
  delete_reason TEXT NOT NULL DEFAULT '',
  usage TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS snippets (
  session_id TEXT NOT NULL,
  pos INTEGER NOT NULL,
  role TEXT NOT NULL DEFAULT '',
  body TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (session_id, pos)
);
CREATE TABLE IF NOT EXISTS files (
  path TEXT PRIMARY KEY,
  mtime INTEGER NOT NULL,
  agent TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS sessions_agent ON sessions(agent);
CREATE INDEX IF NOT EXISTS sessions_source ON sessions(source_path);
CREATE INDEX IF NOT EXISTS sessions_updated ON sessions(updated);
`)
	if err != nil {
		return err
	}
	added, err := ensureColumn(s.db, "sessions", "usage", `TEXT NOT NULL DEFAULT ''`)
	if err != nil {
		return err
	}
	if added {
		// Older indexes have no accounting. Force the next scan to re-read sources.
		if _, err := s.db.Exec(`UPDATE files SET mtime = 0`); err != nil {
			return err
		}
	}
	// indexRevision bumps when a parser learns a field the previous index
	// would have stored as a dash. Zeroing mtime once makes the next scan
	// re-read those files. A later open leaves recorded mtimes alone.
	const indexRevision = "2"
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		return err
	}
	var rev string
	_ = s.db.QueryRow(`SELECT value FROM meta WHERE key = 'index_revision'`).Scan(&rev)
	if rev != indexRevision {
		if _, err := s.db.Exec(`UPDATE files SET mtime = 0`); err != nil {
			return err
		}
		if _, err := s.db.Exec(`INSERT OR REPLACE INTO meta(key, value) VALUES ('index_revision', ?)`, indexRevision); err != nil {
			return err
		}
	}
	if _, err := s.db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS fts USING fts5(title, summary, body, session_id UNINDEXED)`); err != nil {
		s.fts = false
		return nil
	}
	s.fts = true
	return nil
}

// Fresh reports whether path was indexed at mtime and still has sessions.
// A matching mtime with no rows is read again, so a scan that recorded the
// file and then dropped its sessions does not stay empty.
// A row indexed while delete was disabled is read again too. Delete
// capability is not part of the file mtime, and leaving the old flag in
// place hides delete after an upgrade.
func (s *Store) Fresh(path string, mtime int64) bool {
	if mtime == 0 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var got int64
	err := s.db.QueryRow(`SELECT mtime FROM files WHERE path = ?`, path).Scan(&got)
	if err != nil || got != mtime {
		return false
	}
	var n, stale int
	if err := s.db.QueryRow(`SELECT count(*) FROM sessions WHERE source_path = ?`, path).Scan(&n); err != nil || n == 0 {
		return false
	}
	err = s.db.QueryRow(`SELECT count(*) FROM sessions WHERE source_path = ? AND (can_delete = 0 OR delete_mode = '')`, path).Scan(&stale)
	if err != nil || stale != 0 {
		return false
	}
	// Rows stored as agy are read again so the index uses the name antigravity.
	var legacy int
	err = s.db.QueryRow(`SELECT count(*) FROM sessions WHERE source_path = ? AND agent = 'agy'`, path).Scan(&legacy)
	return err == nil && legacy == 0
}

// Apply merges one agent's scan into the index.
// Files marked fresh are left untouched. Every other visited path replaces
// its sessions. Sessions whose files disappeared are removed.
func (s *Store) Apply(agent string, sessions []model.Session, files []Source) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`CREATE TEMP TABLE IF NOT EXISTS ai9s_seen (path TEXT PRIMARY KEY)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM ai9s_seen`); err != nil {
		return err
	}
	changed := map[string]int64{}
	for _, f := range files {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO ai9s_seen(path) VALUES (?)`, f.Path); err != nil {
			return err
		}
		if !f.Fresh {
			changed[f.Path] = f.Mtime
		}
	}
	for path := range changed {
		if err := deletePath(tx, s.fts, path); err != nil {
			return err
		}
	}
	for _, sess := range sessions {
		if err := insertSession(tx, s.fts, sess); err != nil {
			return err
		}
		mt := sess.SourceMtime
		if v, ok := changed[sess.SourcePath]; ok {
			mt = v
		}
		if err := upsertFile(tx, sess.SourcePath, mt, agent); err != nil {
			return err
		}
	}
	for path, mt := range changed {
		if err := upsertFile(tx, path, mt, agent); err != nil {
			return err
		}
	}
	gone, err := tx.Query(`SELECT id FROM sessions WHERE agent = ? AND source_path NOT IN (SELECT path FROM ai9s_seen)`, agent)
	if err != nil {
		return err
	}
	var drop []string
	for gone.Next() {
		var id string
		if err := gone.Scan(&id); err != nil {
			gone.Close()
			return err
		}
		drop = append(drop, id)
	}
	gone.Close()
	if err := gone.Err(); err != nil {
		return err
	}
	for _, id := range drop {
		if err := deleteID(tx, s.fts, id); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM files WHERE agent = ? AND path NOT IN (SELECT path FROM ai9s_seen)`, agent); err != nil {
		return err
	}
	if agent == "antigravity" {
		if err := dropAgent(tx, s.fts, "agy"); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// dropAgent removes every session stored under a retired agent id.
func dropAgent(tx *sql.Tx, fts bool, agent string) error {
	rows, err := tx.Query(`SELECT id FROM sessions WHERE agent = ?`, agent)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		if err := deleteID(tx, fts, id); err != nil {
			return err
		}
	}
	_, err = tx.Exec(`DELETE FROM files WHERE agent = ?`, agent)
	return err
}

func upsertFile(tx *sql.Tx, path string, mtime int64, agent string) error {
	if path == "" {
		return nil
	}
	_, err := tx.Exec(`INSERT INTO files(path, mtime, agent) VALUES (?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET mtime = excluded.mtime, agent = excluded.agent`, path, mtime, agent)
	return err
}

func deletePath(tx *sql.Tx, fts bool, path string) error {
	rows, err := tx.Query(`SELECT id FROM sessions WHERE source_path = ?`, path)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		if err := deleteID(tx, fts, id); err != nil {
			return err
		}
	}
	return nil
}

func deleteID(tx *sql.Tx, fts bool, id string) error {
	if fts {
		rows, err := tx.Query(`SELECT rowid FROM fts WHERE session_id = ?`, id)
		if err != nil {
			return err
		}
		var rowids []int64
		for rows.Next() {
			var n int64
			if err := rows.Scan(&n); err != nil {
				rows.Close()
				return err
			}
			rowids = append(rowids, n)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, n := range rowids {
			if _, err := tx.Exec(`DELETE FROM fts WHERE rowid = ?`, n); err != nil {
				return err
			}
		}
	}
	if _, err := tx.Exec(`DELETE FROM snippets WHERE session_id = ?`, id); err != nil {
		return err
	}
	_, err := tx.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}

func insertSession(tx *sql.Tx, fts bool, sess model.Session) error {
	if err := deleteID(tx, fts, sess.ID); err != nil {
		return err
	}
	can := 0
	if sess.CanDelete {
		can = 1
	}
	var updated int64
	if !sess.Updated.IsZero() {
		updated = sess.Updated.Unix()
	}
	_, err := tx.Exec(`INSERT INTO sessions(
		id, native_id, agent, title, summary, cwd, branch, model, updated, messages,
		source_path, source_mtime, can_delete, delete_mode, delete_reason, usage)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sess.ID, sess.NativeID, sess.Agent, sess.Title, sess.Summary, sess.CWD, sess.Branch, sess.Model,
		updated, sess.Messages, sess.SourcePath, sess.SourceMtime, can, sess.DeleteMode, sess.DeleteReason,
		encodeUsage(sess.Usage))
	if err != nil {
		return err
	}
	var body strings.Builder
	for i, sn := range sess.Snippets {
		if _, err := tx.Exec(`INSERT INTO snippets(session_id, pos, role, body) VALUES (?, ?, ?, ?)`, sess.ID, i, sn.Role, sn.Body); err != nil {
			return err
		}
		body.WriteString(sn.Body)
		body.WriteByte('\n')
	}
	if fts {
		_, err = tx.Exec(`INSERT INTO fts(title, summary, body, session_id) VALUES (?, ?, ?, ?)`, sess.Title, sess.Summary, body.String(), sess.ID)
	}
	return err
}

// Forget drops one session and marks its source stale so the next index re-reads it.
func (s *Store) Forget(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var path string
	err := s.db.QueryRow(`SELECT source_path FROM sessions WHERE id = ?`, id).Scan(&path)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := deleteID(tx, s.fts, id); err != nil {
		return err
	}
	if path != "" {
		if _, err := tx.Exec(`UPDATE files SET mtime = 0 WHERE path = ?`, path); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Search lists sessions that match f. Snippets are not loaded.
func (s *Store) Search(f query.Filter, limit int) ([]model.Session, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 2000 {
		limit = 2000
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var b strings.Builder
	args := []any{}
	b.WriteString(`SELECT ` + cols + ` FROM sessions s WHERE 1=1`)
	for _, w := range query.Tokens(f.Text) {
		if strings.TrimSpace(w) == "" {
			continue
		}
		p := "%" + escapeLike(w) + "%"
		b.WriteString(` AND (s.title LIKE ? ESCAPE '\' OR s.summary LIKE ? ESCAPE '\' OR s.cwd LIKE ? ESCAPE '\' OR s.branch LIKE ? ESCAPE '\' OR s.model LIKE ? ESCAPE '\' OR s.agent LIKE ? ESCAPE '\' OR s.id IN (SELECT session_id FROM snippets WHERE body LIKE ? ESCAPE '\'))`)
		args = append(args, p, p, p, p, p, p, p)
	}
	if f.Agent != "" {
		b.WriteString(` AND s.agent LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escapeLike(f.Agent)+"%")
	}
	if f.Dir != "" {
		b.WriteString(` AND s.cwd LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escapeLike(f.Dir)+"%")
	}
	if f.Branch != "" {
		b.WriteString(` AND s.branch LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escapeLike(f.Branch)+"%")
	}
	if f.Model != "" {
		b.WriteString(` AND s.model LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escapeLike(f.Model)+"%")
	}
	if !f.Since.IsZero() {
		b.WriteString(` AND s.updated >= ?`)
		args = append(args, f.Since.Unix())
	}
	if !f.Until.IsZero() {
		b.WriteString(` AND s.updated < ?`)
		args = append(args, f.Until.Unix())
	}
	switch f.Sort {
	case "oldest":
		b.WriteString(` ORDER BY s.updated ASC`)
	case "messages":
		b.WriteString(` ORDER BY s.messages DESC, s.updated DESC`)
	case "title":
		b.WriteString(` ORDER BY s.title COLLATE NOCASE ASC`)
	default:
		b.WriteString(` ORDER BY s.updated DESC`)
	}
	b.WriteString(` LIMIT ?`)
	args = append(args, limit)
	rows, err := s.db.Query(b.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Session{}
	for rows.Next() {
		sess, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

// Get loads one session, including preview snippets.
func (s *Store) Get(id string) (model.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.get(id)
}

func (s *Store) get(id string) (model.Session, error) {
	row := s.db.QueryRow(`SELECT `+cols+` FROM sessions s WHERE s.id = ?`, id)
	sess, err := scanSession(row)
	if err == sql.ErrNoRows {
		return model.Session{}, fmt.Errorf("session %s not found", id)
	}
	if err != nil {
		return model.Session{}, err
	}
	snips, err := s.snippets(id)
	if err != nil {
		return model.Session{}, err
	}
	sess.Snippets = snips
	return sess, nil
}

// Resolve finds a session by its full id or its native id.
func (s *Store) Resolve(key string) (model.Session, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return model.Session{}, fmt.Errorf("missing session id")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.Contains(key, ":") {
		sess, err := s.get(key)
		if err == nil {
			return sess, nil
		}
	}
	rows, err := s.db.Query(`SELECT `+cols+` FROM sessions s WHERE s.id = ? OR s.native_id = ?`, key, key)
	if err != nil {
		return model.Session{}, err
	}
	defer rows.Close()
	var found []model.Session
	for rows.Next() {
		sess, err := scanSession(rows)
		if err != nil {
			return model.Session{}, err
		}
		found = append(found, sess)
	}
	if err := rows.Err(); err != nil {
		return model.Session{}, err
	}
	switch len(found) {
	case 0:
		return model.Session{}, fmt.Errorf("session %s not found", key)
	case 1:
		snips, err := s.snippets(found[0].ID)
		if err != nil {
			return model.Session{}, err
		}
		found[0].Snippets = snips
		return found[0], nil
	default:
		return model.Session{}, fmt.Errorf("session %s matches more than one agent; pass agent:id", key)
	}
}

func (s *Store) snippets(id string) ([]model.Snippet, error) {
	rows, err := s.db.Query(`SELECT role, body FROM snippets WHERE session_id = ? ORDER BY pos`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Snippet
	for rows.Next() {
		var sn model.Snippet
		if err := rows.Scan(&sn.Role, &sn.Body); err != nil {
			return nil, err
		}
		out = append(out, sn)
	}
	return out, rows.Err()
}

// Stats counts sessions already in the index.
func (s *Store) Stats() (Stats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT agent, COUNT(*), COALESCE(SUM(messages), 0), MIN(updated), MAX(updated) FROM sessions GROUP BY agent ORDER BY agent`)
	if err != nil {
		return Stats{}, err
	}
	defer rows.Close()
	var st Stats
	for rows.Next() {
		var a AgentStat
		var oldest, newest sql.NullInt64
		if err := rows.Scan(&a.Agent, &a.Sessions, &a.Messages, &oldest, &newest); err != nil {
			return Stats{}, err
		}
		if oldest.Valid && oldest.Int64 > 0 {
			a.Oldest = time.Unix(oldest.Int64, 0)
		}
		if newest.Valid && newest.Int64 > 0 {
			a.Newest = time.Unix(newest.Int64, 0)
		}
		st.Sessions += a.Sessions
		st.Messages += a.Messages
		st.Agents = append(st.Agents, a)
	}
	return st, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanSession(sc scanner) (model.Session, error) {
	var sess model.Session
	var updated int64
	var can int
	var usage string
	err := sc.Scan(&sess.ID, &sess.NativeID, &sess.Agent, &sess.Title, &sess.Summary, &sess.CWD, &sess.Branch, &sess.Model, &updated, &sess.Messages, &sess.SourcePath, &sess.SourceMtime, &can, &sess.DeleteMode, &sess.DeleteReason, &usage)
	if err != nil {
		return model.Session{}, err
	}
	sess.Usage = decodeUsage(usage)
	sess.CanDelete = can != 0
	if updated > 0 {
		sess.Updated = time.Unix(updated, 0)
	}
	return sess, nil
}

func ensureColumn(db *sql.DB, table, column, decl string) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == column {
			return false, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	_, err = db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` ` + decl)
	return err == nil, err
}

func encodeUsage(u model.Usage) string {
	if u.Empty() {
		return ""
	}
	b, err := json.Marshal(u)
	if err != nil {
		return ""
	}
	return string(b)
}

func decodeUsage(s string) model.Usage {
	var u model.Usage
	if s == "" {
		return u
	}
	_ = json.Unmarshal([]byte(s), &u)
	return u
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

package store

import (
	"strings"

	"github.com/AymanZahran/ai9s/internal/model"
	"github.com/AymanZahran/ai9s/internal/query"
)

// TreeRow is one visible line of the sessions list.
// Children is the number of direct subagents, including ones still collapsed.
// Open is true when those subagents are listed under this row.
type TreeRow struct {
	Session  model.Session
	Depth    int
	Children int
	Open     bool
}

type idLink struct {
	ID       string
	ParentID string
	Messages int
}

// SessionTree builds the sessions list.
// Roots follow f's sort. A subagent stays under its parent, so it does not
// take a slot in limit. The list starts collapsed: pass expand with true to
// open a row and false to keep it closed. A text search keeps each matching
// subagent and its ancestors, and opens that branch.
// sessions and messages count every match, including collapsed subagents.
// A text search counts the matches and not the ancestors added for context.
func (s *Store) SessionTree(f query.Filter, limit int, expand map[string]bool) ([]TreeRow, int, int, error) {
	limit = clampLimit(limit)
	s.mu.Lock()
	defer s.mu.Unlock()

	structural := f
	structural.Text = ""
	ordered, err := s.orderedIDs(structural)
	if err != nil {
		return nil, 0, 0, err
	}
	parentOf := make(map[string]string, len(ordered))
	known := make(map[string]bool, len(ordered))
	messages := 0
	for _, row := range ordered {
		parentOf[row.ID] = row.ParentID
		known[row.ID] = true
		messages += row.Messages
	}
	children := map[string][]string{}
	var roots []string
	for _, row := range ordered {
		root := isRoot(row.ID, parentOf, known)
		// A cycle is two roots and is not nested. A parent outside this
		// filter still keeps the link, so a text hit can show that parent.
		if row.ParentID != "" && (!root || !known[row.ParentID]) {
			children[row.ParentID] = append(children[row.ParentID], row.ID)
		}
		if root {
			roots = append(roots, row.ID)
		}
	}

	text := strings.TrimSpace(f.Text) != ""
	match := map[string]bool{}
	loaded := map[string]model.Session{}
	sessions := len(ordered)
	if text {
		found, err := s.searchLocked(f, limit)
		if err != nil {
			return nil, 0, 0, err
		}
		sessions = len(found)
		messages = 0
		for _, sess := range found {
			match[sess.ID] = true
			loaded[sess.ID] = sess
			messages += sess.Messages
		}
		if err := s.loadAncestors(loaded); err != nil {
			return nil, 0, 0, err
		}
		visible := map[string]bool{}
		for id := range loaded {
			visible[id] = true
		}
		roots = roots[:0]
		for id := range loaded {
			if isRoot(id, parentOfVisible(loaded), visible) {
				roots = append(roots, id)
			}
		}
		order := map[string]int{}
		for i, row := range ordered {
			order[row.ID] = i
		}
		rank := map[string]int{}
		for i, sess := range found {
			rank[sess.ID] = i
		}
		sortRoots(roots, order, rank, children)
	} else if len(roots) > limit {
		roots = roots[:limit]
	}

	memo := map[string]bool{}
	visiting := map[string]bool{}
	var has func(string) bool
	has = func(id string) bool {
		if v, ok := memo[id]; ok {
			return v
		}
		if visiting[id] {
			return false
		}
		visiting[id] = true
		found := match[id]
		if !found {
			for _, child := range children[id] {
				if has(child) {
					found = true
					break
				}
			}
		}
		delete(visiting, id)
		memo[id] = found
		return found
	}

	var items []treeItem
	seen := map[string]bool{}
	var walk func(id string, depth int)
	walk = func(id string, depth int) {
		if seen[id] || depth > 16 {
			return
		}
		seen[id] = true
		n := len(children[id])
		open, manual := rowOpen(id, expand, text, n, children, has)
		items = append(items, treeItem{id: id, depth: depth, children: n, open: open})
		if !open {
			return
		}
		shown := 0
		for _, child := range children[id] {
			if shown >= treeChildCap {
				break
			}
			if manual || has(child) {
				shown++
				walk(child, depth+1)
			}
		}
	}
	for _, id := range roots {
		walk(id, 0)
	}

	need := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := loaded[item.id]; !ok {
			need = append(need, item.id)
		}
	}
	if err := s.loadIDs(loaded, need); err != nil {
		return nil, 0, 0, err
	}
	out := make([]TreeRow, 0, len(items))
	for _, item := range items {
		sess, ok := loaded[item.id]
		if !ok {
			continue
		}
		out = append(out, TreeRow{Session: sess, Depth: item.depth, Children: item.children, Open: item.open})
	}
	return out, sessions, messages, nil
}

// treeChildCap is the most subagents one expanded row lists.
// It matches the search cap, so a long Claude Code thread still fits.
const treeChildCap = 2000

type treeItem struct {
	id       string
	depth    int
	children int
	open     bool
}

func rowOpen(id string, expand map[string]bool, text bool, n int, children map[string][]string, has func(string) bool) (open, manual bool) {
	if n == 0 {
		return false, false
	}
	if expand != nil {
		if v, ok := expand[id]; ok {
			return v, v
		}
	}
	if !text {
		return false, false
	}
	for _, child := range children[id] {
		if has(child) {
			return true, false
		}
	}
	return false, false
}

func parentOfVisible(loaded map[string]model.Session) map[string]string {
	out := make(map[string]string, len(loaded))
	for id, sess := range loaded {
		out[id] = sess.ParentID
	}
	return out
}

func sortRoots(roots []string, order, rank map[string]int, children map[string][]string) {
	best := map[string]int{}
	visiting := map[string]bool{}
	var score func(string) int
	score = func(id string) int {
		if v, ok := best[id]; ok {
			return v
		}
		if visiting[id] {
			return 1 << 30
		}
		visiting[id] = true
		n, ok := rank[id]
		if !ok {
			n = 1 << 30
		}
		for _, child := range children[id] {
			if s := score(child); s < n {
				n = s
			}
		}
		delete(visiting, id)
		best[id] = n
		return n
	}
	for i := 1; i < len(roots); i++ {
		for j := i; j > 0; j-- {
			a, b := roots[j-1], roots[j]
			sa, sb := score(a), score(b)
			oa, ob := orderOf(order, a), orderOf(order, b)
			if sa < sb || (sa == sb && oa <= ob) {
				break
			}
			roots[j-1], roots[j] = b, a
		}
	}
}

func orderOf(order map[string]int, id string) int {
	if v, ok := order[id]; ok {
		return v
	}
	return 1 << 30
}

func isRoot(id string, parentOf map[string]string, known map[string]bool) bool {
	p := parentOf[id]
	if p == "" || !known[p] || p == id {
		return true
	}
	seen := map[string]bool{id: true}
	cur := p
	for hops := 0; hops < 32; hops++ {
		if cur == id || seen[cur] {
			return true
		}
		seen[cur] = true
		next := parentOf[cur]
		if next == "" || !known[next] {
			return false
		}
		cur = next
	}
	return true
}

func (s *Store) orderedIDs(f query.Filter) ([]idLink, error) {
	q, args := sessionSQL(f, `SELECT s.id, COALESCE(s.parent_id, ''), s.messages FROM sessions s WHERE 1=1`, -1)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []idLink
	for rows.Next() {
		var row idLink
		if err := rows.Scan(&row.ID, &row.ParentID, &row.Messages); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) loadAncestors(loaded map[string]model.Session) error {
	for {
		var need []string
		seen := map[string]bool{}
		for _, sess := range loaded {
			p := sess.ParentID
			if p == "" {
				continue
			}
			if _, ok := loaded[p]; ok || seen[p] {
				continue
			}
			seen[p] = true
			need = append(need, p)
		}
		if len(need) == 0 {
			return nil
		}
		before := len(loaded)
		if err := s.loadIDs(loaded, need); err != nil {
			return err
		}
		if len(loaded) == before {
			return nil
		}
	}
}

func (s *Store) loadIDs(dest map[string]model.Session, ids []string) error {
	const batch = 400
	for start := 0; start < len(ids); start += batch {
		end := start + batch
		if end > len(ids) {
			end = len(ids)
		}
		part := ids[start:end]
		holders := make([]string, len(part))
		args := make([]any, len(part))
		for i, id := range part {
			holders[i] = "?"
			args[i] = id
		}
		q := `SELECT ` + cols + ` FROM sessions s WHERE s.id IN (` + strings.Join(holders, ",") + `)`
		rows, err := s.db.Query(q, args...)
		if err != nil {
			return err
		}
		for rows.Next() {
			sess, err := scanSession(rows)
			if err != nil {
				rows.Close()
				return err
			}
			dest[sess.ID] = sess
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
	}
	return nil
}

package index

import (
	"fmt"

	"github.com/AymanZahran/air9s/internal/discover"
	"github.com/AymanZahran/air9s/internal/store"
)

// Rebuild scans every agent and updates the index.
// A scanner that fails is reported and its previous rows are left in place.
func Rebuild(st *store.Store) (store.Stats, []string, error) {
	var warnings []string
	for _, batch := range discover.Collect(st.Fresh) {
		if batch.Err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", batch.Agent, batch.Err))
			continue
		}
		files := make([]store.Source, len(batch.Files))
		for i, f := range batch.Files {
			files[i] = store.Source{Path: f.Path, Mtime: f.Mtime, Fresh: f.Fresh}
		}
		if err := st.Apply(batch.Agent, batch.Sessions, files); err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", batch.Agent, err))
		}
	}
	stats, err := st.Stats()
	return stats, warnings, err
}

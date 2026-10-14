package discover

import (
	"os"

	"github.com/AymanZahran/ai9s/internal/model"
)

func scanSQLite(agent, path string, fresh func(string, int64) bool, read func(path string, mt int64) ([]model.Session, error)) Batch {
	b := Batch{Agent: agent}
	if path == "" {
		return b
	}
	st, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return b
		}
		b.Err = err
		return b
	}
	mt := st.ModTime().Unix()
	if fresh != nil && fresh(path, mt) {
		b.Files = []File{{Path: path, Mtime: mt, Fresh: true}}
		return b
	}
	sessions, err := read(path, mt)
	if err != nil {
		b.Err = err
		return b
	}
	b.Files = []File{{Path: path, Mtime: mt, Fresh: false}}
	b.Sessions = sessions
	return b
}

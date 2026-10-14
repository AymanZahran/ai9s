package discover

import (
	"os"
	"path/filepath"
	"time"

	"github.com/AymanZahran/ai9s/internal/model"
)

// LETTA_HOME is an ai9s override. The default is ~/.letta.
// Transcripts are local files. ai9s does not call the Letta API.

func lettaHome() string {
	return envOr("LETTA_HOME", homeJoin(".letta"))
}

func lettaTranscripts() string {
	home := lettaHome()
	if home == "" {
		return ""
	}
	return filepath.Join(home, "transcripts")
}

func scanLetta(fresh func(string, int64) bool) Batch {
	b := Batch{Agent: "letta"}
	root := lettaTranscripts()
	if root == "" {
		return b
	}
	agents, err := os.ReadDir(root)
	if err != nil {
		return b
	}
	seen := map[string]bool{}
	for _, agent := range agents {
		name := agent.Name()
		if name == "" || name[0] == '.' || !agent.IsDir() {
			continue
		}
		convs, err := os.ReadDir(filepath.Join(root, name))
		if err != nil {
			continue
		}
		for _, conv := range convs {
			convName := conv.Name()
			if convName == "" || convName[0] == '.' || !conv.IsDir() {
				continue
			}
			path := filepath.Join(root, name, convName, "transcript.jsonl")
			if !regularFile(path) {
				continue
			}
			mt, skip := stamp(path, fresh)
			if mt == 0 {
				continue
			}
			native := lettaNative(name, convName)
			if skip {
				b.Files = append(b.Files, File{Path: path, Mtime: mt, Fresh: true})
				seen[native] = true
				continue
			}
			sess, err := readLettaFile(path, native, mt)
			if err != nil || sess.NativeID == "" || seen[sess.NativeID] {
				continue
			}
			seen[sess.NativeID] = true
			b.Files = append(b.Files, File{Path: path, Mtime: mt, Fresh: false})
			b.Sessions = append(b.Sessions, sess)
		}
	}
	return b
}

func lettaNative(agent, conversation string) string {
	if conversation == "" || conversation == "default" {
		return agent
	}
	return agent + ":" + conversation
}

func readLettaFile(path, native string, mt int64) (model.Session, error) {
	var buf snippetBuf
	var updated time.Time
	err := walkJSONL(path, func(obj map[string]any) error {
		role, text := nestedText(obj)
		buf.add(role, text, "")
		updated = later(updated, whenOf(obj["captured_at"]))
		return nil
	})
	if err != nil {
		return model.Session{}, err
	}
	title := snippetTitle(buf.snippets())
	if title == "" {
		title = native
	}
	if updated.IsZero() {
		updated = time.Unix(mt, 0)
	}
	return model.Session{
		ID: model.ID("letta", native), NativeID: native, Agent: "letta",
		Title: title, Updated: updated, Messages: buf.n, Snippets: buf.snippets(),
		SourcePath: path, SourceMtime: mt,
		CanDelete: true, DeleteMode: "file",
	}, nil
}

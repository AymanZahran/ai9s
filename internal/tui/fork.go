package tui

import (
	"strings"
	"unicode/utf8"

	"github.com/AymanZahran/ai9s/internal/fork"
	"github.com/AymanZahran/ai9s/internal/index"
	"github.com/AymanZahran/ai9s/internal/model"
	"github.com/AymanZahran/ai9s/internal/store"
)

// copySession writes the forked transcript. Tests replace it.
var copySession = fork.Copy

// rebuildIndex reads the new transcript into the list. Tests replace it.
var rebuildIndex = index.Rebuild

// promptFork asks for the name of a second session copied from the selection.
// Esc cancels. An empty name stays on the dialog.
func (ui *ui) promptFork() {
	if ui.busy {
		return
	}
	if !ui.onSessions() {
		ui.alert("Switch to sessions before forking.")
		return
	}
	row, _ := ui.table.GetSelection()
	if row <= 0 || row-1 >= len(ui.rows) {
		ui.alert("Select a session first.")
		return
	}
	s, ok := ui.selected()
	if !ok {
		return
	}
	ui.promptFields(" fork ", forkSuggest(sessionName(s)), true, func(text string, worktree bool) {
		name := strings.TrimSpace(text)
		if name == "" {
			ui.alert("Type a name for the fork.")
			return
		}
		ui.startFork(s, name, worktree)
	})
}

func forkSuggest(name string) string {
	const suffix = " fork"
	name = strings.TrimSpace(name)
	if name == "" || name == "-" {
		return "fork"
	}
	room := store.MaxSessionName - utf8.RuneCountInString(suffix)
	if room < 1 {
		return "fork"
	}
	if utf8.RuneCountInString(name) > room {
		name = string([]rune(name)[:room])
	}
	return name + suffix
}

func (ui *ui) startFork(s model.Session, name string, worktree bool) {
	if ui.busy {
		return
	}
	ui.busy = true
	ui.busyNote = "forking…"
	ui.paintHeader()
	go func() {
		native, err := copySession(s, fork.Options{Worktree: worktree, Name: name})
		var warnings []string
		childID := ""
		if err == nil {
			_, warnings, err = rebuildIndex(ui.store)
		}
		if err == nil {
			childID = model.ID(s.Agent, native)
			if nerr := ui.store.SetName(childID, name); nerr != nil {
				err = nerr
			} else if ferr := ui.store.SetFork(childID, s.ID); ferr != nil {
				err = ferr
			}
		}
		ui.app.QueueUpdateDraw(func() {
			ui.busy = false
			ui.busyNote = ""
			if len(warnings) > 0 {
				ui.warnings = append(ui.warnings, warnings...)
			}
			if err != nil {
				ui.reload()
				ui.alert(err.Error())
				return
			}
			ui.keepID = childID
			ui.reload()
			msg := "Forked. The list has the original and " + name + "."
			if worktree {
				msg = "Forked into a new worktree. The list has the original and " + name + "."
			}
			ui.alert(msg)
		})
	}()
}

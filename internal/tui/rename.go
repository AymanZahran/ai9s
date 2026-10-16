package tui

import (
	"unicode"
	"unicode/utf8"

	"github.com/AymanZahran/ai9s/internal/store"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func acceptSessionName(text string, last rune) bool {
	if unicode.IsControl(last) || unicode.Is(unicode.Cf, last) {
		return false
	}
	return utf8.RuneCountInString(text) <= store.MaxSessionName
}

// promptRename asks for the NAME of the selected session.
// An empty name restores the recorded title. Esc cancels.
func (ui *ui) promptRename() {
	if !ui.onSessions() {
		ui.alert("Switch to sessions before renaming.")
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
	ui.promptName(" rename ", s.Name, func(text string) {
		if err := ui.store.SetName(s.ID, text); err != nil {
			ui.alert(err.Error())
			return
		}
		ui.reload()
	})
}

// promptName opens the centered name dialog used by rename.
// apply runs after the dialog closes.
func (ui *ui) promptName(title, initial string, apply func(text string)) {
	ui.promptFields(title, initial, false, func(text string, _ bool) { apply(text) })
}

// promptFields opens the name dialog. askWorktree adds the fork checkbox.
// apply runs after the dialog closes.
func (ui *ui) promptFields(title, initial string, askWorktree bool, apply func(text string, worktree bool)) {
	bg := paintColor(ui.cfg.Skin.Body.Bg, "black")
	fg := paintColor(ui.cfg.Skin.Body.Fg, "white")
	form := tview.NewForm()
	form.SetBorder(true).SetTitle(title)
	form.SetBackgroundColor(bg)
	form.SetFieldBackgroundColor(bg)
	form.SetFieldTextColor(fg)
	form.SetLabelColor(fg)
	form.SetButtonBackgroundColor(bg)
	form.SetButtonTextColor(fg)
	form.SetBorderColor(paintColor(ui.cfg.Skin.Frame.Border.Focus, "white"))
	form.SetTitleColor(paintColor(ui.cfg.Skin.Frame.Title.Highlight, "white"))
	form.SetButtonActivatedStyle(tcell.StyleDefault.
		Foreground(paintColor(ui.cfg.Skin.Views.Table.CursorFg, "black")).
		Background(paintColor(ui.cfg.Skin.Views.Table.CursorBg, "white")).
		Bold(true))
	form.SetButtonsAlign(tview.AlignCenter)

	closed := false
	close := func() {
		if closed {
			return
		}
		closed = true
		ui.app.SetRoot(ui.layout, true)
		ui.restoreBodyFocus()
	}
	boxWidth, fieldWidth := nameDialogSize(ui)
	form.AddInputField("Name", initial, fieldWidth, acceptSessionName, nil)
	if askWorktree {
		form.AddCheckbox("Worktree", false, nil)
	}
	form.AddButton("Save", func() {
		field, ok := form.GetFormItem(0).(*tview.InputField)
		if !ok {
			close()
			return
		}
		text := field.GetText()
		checked := false
		if askWorktree {
			if box, ok := form.GetFormItem(1).(*tview.Checkbox); ok {
				checked = box.IsChecked()
			}
		}
		close()
		apply(text, checked)
	})
	form.AddButton("Cancel", close)
	form.SetCancelFunc(close)

	inner := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(nil, 0, 1, false).
		AddItem(form, boxWidth, 0, true).
		AddItem(nil, 0, 1, false)
	outer := tview.NewFlex().SetDirection(tview.FlexRow)
	outer.Box = tview.NewBox().SetBackgroundColor(bg)
	rows := 9
	if askWorktree {
		rows = 12
	}
	outer.SetFullScreen(true).
		AddItem(nil, 0, 1, false).
		AddItem(inner, rows, 0, true).
		AddItem(nil, 0, 1, false)
	ui.app.SetRoot(outer, true).SetFocus(form)
}

// nameDialogSize keeps the name dialog inside the current pane.
// A Herdr sidebar is often narrower than a full terminal.
func nameDialogSize(ui *ui) (box, field int) {
	const wide, fieldWide = 62, 36
	_, _, w, _ := ui.layout.GetRect()
	if w <= 0 || w >= wide {
		return wide, fieldWide
	}
	field = w - 12
	if field < 8 {
		field = 8
	}
	if field > fieldWide {
		field = fieldWide
	}
	return w, field
}

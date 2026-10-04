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
	bg := paintColor(ui.cfg.Skin.Body.Bg, "black")
	fg := paintColor(ui.cfg.Skin.Body.Fg, "white")
	form := tview.NewForm()
	form.SetBorder(true).SetTitle(" rename ")
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
	form.AddInputField("Name", s.Name, 36, acceptSessionName, nil)
	form.AddButton("Save", func() {
		field, ok := form.GetFormItem(0).(*tview.InputField)
		if !ok {
			close()
			return
		}
		if err := ui.store.SetName(s.ID, field.GetText()); err != nil {
			close()
			ui.alert(err.Error())
			return
		}
		close()
		ui.reload()
	})
	form.AddButton("Cancel", close)
	form.SetCancelFunc(close)

	inner := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(nil, 0, 1, false).
		AddItem(form, 62, 0, true).
		AddItem(nil, 0, 1, false)
	outer := tview.NewFlex().SetDirection(tview.FlexRow)
	outer.Box = tview.NewBox().SetBackgroundColor(bg)
	outer.SetFullScreen(true).
		AddItem(nil, 0, 1, false).
		AddItem(inner, 9, 0, true).
		AddItem(nil, 0, 1, false)
	ui.app.SetRoot(outer, true).SetFocus(form)
}

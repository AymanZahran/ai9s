package tui

import (
	"strings"

	"github.com/AymanZahran/air9s/internal/model"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func paintColor(s, fallback string) tcell.Color {
	s = strings.TrimSpace(s)
	if s == "" {
		s = fallback
	}
	if strings.EqualFold(s, "default") {
		return tcell.ColorDefault
	}
	return tcell.GetColor(s)
}

func (ui *ui) applySkin() {
	bg := paintColor(ui.cfg.Skin.Body.Bg, "black")
	fg := paintColor(ui.cfg.Skin.Body.Fg, "white")
	border := paintColor(ui.cfg.Skin.Frame.Border.Fg, "dodgerblue")
	title := paintColor(ui.cfg.Skin.Frame.Title.Fg, "aqua")
	menu := paintColor(ui.cfg.Skin.Frame.Menu.Fg, "white")
	crumbsBg := paintColor(ui.cfg.Skin.Frame.Crumbs.Bg, "steelblue")
	crumbsFg := paintColor(ui.cfg.Skin.Frame.Crumbs.Fg, "black")
	tableBg := paintColor(ui.cfg.Skin.Views.Table.Bg, "black")
	filter := paintColor(ui.cfg.Skin.Frame.Title.Filter, "orange")

	ui.header.SetBackgroundColor(bg)
	ui.header.SetTextColor(menu)
	ui.logo.SetBackgroundColor(bg)
	ui.info.SetBackgroundColor(bg)
	ui.info.SetTextColor(fg)
	ui.footer.SetBackgroundColor(bg)
	ui.footer.SetTextColor(fg)
	ui.crumbs.SetBackgroundColor(crumbsBg)
	ui.crumbs.SetTextColor(crumbsFg)
	ui.table.SetBackgroundColor(tableBg)
	ui.table.SetBorderColor(border)
	ui.table.SetTitleColor(title)
	ui.preview.SetBackgroundColor(tableBg)
	ui.preview.SetBorderColor(border)
	ui.preview.SetTitleColor(title)
	ui.table.SetSelectedStyle(tcell.StyleDefault.
		Foreground(paintColor(ui.cfg.Skin.Views.Table.CursorFg, "black")).
		Background(paintColor(ui.cfg.Skin.Views.Table.CursorBg, "cadetblue")).
		Bold(true))
	for _, field := range []*tview.InputField{ui.filter, ui.command} {
		field.SetLabelColor(filter)
		field.SetFieldBackgroundColor(bg)
		field.SetFieldTextColor(fg)
		field.SetBackgroundColor(bg)
	}
}

func (ui *ui) headerCell(text string, expand int) *tview.TableCell {
	return tview.NewTableCell(text).
		SetSelectable(false).
		SetTextColor(paintColor(ui.cfg.Skin.Views.Table.Header.Fg, "white")).
		SetBackgroundColor(paintColor(ui.cfg.Skin.Views.Table.Header.Bg, "black")).
		SetAttributes(tcell.AttrBold).
		SetExpansion(expand)
}

func (ui *ui) cell(text string) *tview.TableCell {
	return tview.NewTableCell(text).SetTextColor(paintColor(ui.cfg.Skin.Views.Table.Fg, "cadetblue"))
}

func (ui *ui) mark(agent string) string {
	if ui.cfg.Body.UI.NoIcons {
		return "  "
	}
	return Icon(agent)
}

func (ui *ui) colorOfAgent(agent string) tcell.Color {
	if c, ok := ui.cfg.Skin.Agents[strings.ToLower(agent)]; ok && strings.TrimSpace(c) != "" {
		return paintColor(c, "white")
	}
	return colorOf(agent)
}

func (ui *ui) agentTag(agent string) string {
	if c, ok := ui.cfg.Skin.Agents[strings.ToLower(agent)]; ok && strings.TrimSpace(c) != "" {
		return c
	}
	return agentColor(agent)
}

func (ui *ui) previewBody(s model.Session) string {
	body := preview(s)
	if ui.cfg.Body.UI.NoIcons {
		body = strings.Replace(body, Icon(s.Agent), "  ", 1)
	}
	tag := ui.agentTag(s.Agent)
	if tag != agentColor(s.Agent) {
		body = strings.Replace(body, "["+agentColor(s.Agent)+"]", "["+tag+"]", 1)
	}
	return body
}

func (ui *ui) modal(text string, buttons []string, done func(int, string)) *tview.Modal {
	m := tview.NewModal().SetText(text).AddButtons(buttons).SetDoneFunc(done)
	m.SetBackgroundColor(paintColor(ui.cfg.Skin.Body.Bg, "black"))
	m.SetTextColor(paintColor(ui.cfg.Skin.Body.Fg, "white"))
	m.SetButtonBackgroundColor(paintColor(ui.cfg.Skin.Frame.Border.Fg, "dodgerblue"))
	m.SetButtonTextColor(paintColor(ui.cfg.Skin.Views.Table.CursorFg, "black"))
	m.SetButtonActivatedStyle(tcell.StyleDefault.
		Foreground(paintColor(ui.cfg.Skin.Views.Table.CursorFg, "black")).
		Background(paintColor(ui.cfg.Skin.Frame.Border.Focus, "yellow")).
		Bold(true))
	return m
}

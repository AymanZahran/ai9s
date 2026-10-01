package config

// Skin is a color file. Names are tcell color names or #RRGGBB.
// The color name default means the terminal default.
type Skin struct {
	Body   SkinBody          `yaml:"body"`
	Frame  SkinFrame         `yaml:"frame"`
	Views  SkinViews         `yaml:"views"`
	Agents map[string]string `yaml:"agents"`
}

// SkinBody is the screen behind the frame.
type SkinBody struct {
	Fg   string `yaml:"fgColor"`
	Bg   string `yaml:"bgColor"`
	Logo string `yaml:"logoColor"`
}

// SkinFrame is the menu, crumbs, border, and title.
type SkinFrame struct {
	Border SkinBorder `yaml:"border"`
	Menu   SkinMenu   `yaml:"menu"`
	Crumbs SkinCrumbs `yaml:"crumbs"`
	Title  SkinTitle  `yaml:"title"`
}

// SkinBorder is the box around the table and the preview.
type SkinBorder struct {
	Fg    string `yaml:"fgColor"`
	Focus string `yaml:"focusColor"`
}

// SkinMenu is the hotkey bar. NumKey colors the 1–5 view keys.
type SkinMenu struct {
	Fg     string `yaml:"fgColor"`
	Key    string `yaml:"keyColor"`
	NumKey string `yaml:"numKeyColor"`
}

// SkinCrumbs is the path bar under the menu.
type SkinCrumbs struct {
	Fg     string `yaml:"fgColor"`
	Bg     string `yaml:"bgColor"`
	Active string `yaml:"activeColor"`
}

// SkinTitle is the frame title and the filter prompt.
type SkinTitle struct {
	Fg        string `yaml:"fgColor"`
	Bg        string `yaml:"bgColor"`
	Highlight string `yaml:"highlightColor"`
	Counter   string `yaml:"counterColor"`
	Filter    string `yaml:"filterColor"`
}

// SkinViews holds the table colors.
type SkinViews struct {
	Table SkinTable `yaml:"table"`
}

// SkinTable is the session table.
type SkinTable struct {
	Fg       string     `yaml:"fgColor"`
	Bg       string     `yaml:"bgColor"`
	CursorFg string     `yaml:"cursorFgColor"`
	CursorBg string     `yaml:"cursorBgColor"`
	Header   SkinHeader `yaml:"header"`
}

// SkinHeader is the column-name row.
type SkinHeader struct {
	Fg string `yaml:"fgColor"`
	Bg string `yaml:"bgColor"`
}

func defaultSkin() Skin {
	s := Skin{
		Body: SkinBody{Fg: "white", Bg: "black", Logo: "#2A55F4"},
		Frame: SkinFrame{
			Border: SkinBorder{Fg: "dodgerblue", Focus: "yellow"},
			Menu:   SkinMenu{Fg: "white", Key: "dodgerblue", NumKey: "aqua"},
			Crumbs: SkinCrumbs{Fg: "black", Bg: "steelblue", Active: "white"},
			Title: SkinTitle{
				Fg: "aqua", Bg: "black", Highlight: "yellow",
				Counter: "yellow", Filter: "orange",
			},
		},
		Views: SkinViews{Table: SkinTable{
			Fg: "cadetblue", Bg: "black",
			CursorFg: "black", CursorBg: "cadetblue",
			Header: SkinHeader{Fg: "white", Bg: "black"},
		}},
	}
	return s
}

func (s *Skin) fill() {
	d := defaultSkin()
	if s.Body.Fg == "" {
		s.Body.Fg = d.Body.Fg
	}
	if s.Body.Bg == "" {
		s.Body.Bg = d.Body.Bg
	}
	if s.Body.Logo == "" {
		s.Body.Logo = d.Body.Logo
	}
	if s.Frame.Border.Fg == "" {
		s.Frame.Border.Fg = d.Frame.Border.Fg
	}
	if s.Frame.Border.Focus == "" {
		s.Frame.Border.Focus = d.Frame.Border.Focus
	}
	if s.Frame.Menu.Fg == "" {
		s.Frame.Menu.Fg = d.Frame.Menu.Fg
	}
	if s.Frame.Menu.Key == "" {
		s.Frame.Menu.Key = d.Frame.Menu.Key
	}
	if s.Frame.Menu.NumKey == "" {
		s.Frame.Menu.NumKey = d.Frame.Menu.NumKey
	}
	if s.Frame.Crumbs.Fg == "" {
		s.Frame.Crumbs.Fg = d.Frame.Crumbs.Fg
	}
	if s.Frame.Crumbs.Bg == "" {
		s.Frame.Crumbs.Bg = d.Frame.Crumbs.Bg
	}
	if s.Frame.Crumbs.Active == "" {
		s.Frame.Crumbs.Active = d.Frame.Crumbs.Active
	}
	if s.Frame.Title.Fg == "" {
		s.Frame.Title.Fg = d.Frame.Title.Fg
	}
	if s.Frame.Title.Bg == "" {
		s.Frame.Title.Bg = d.Frame.Title.Bg
	}
	if s.Frame.Title.Highlight == "" {
		s.Frame.Title.Highlight = d.Frame.Title.Highlight
	}
	if s.Frame.Title.Counter == "" {
		s.Frame.Title.Counter = d.Frame.Title.Counter
	}
	if s.Frame.Title.Filter == "" {
		s.Frame.Title.Filter = d.Frame.Title.Filter
	}
	if s.Views.Table.Fg == "" {
		s.Views.Table.Fg = d.Views.Table.Fg
	}
	if s.Views.Table.Bg == "" {
		s.Views.Table.Bg = d.Views.Table.Bg
	}
	if s.Views.Table.CursorFg == "" {
		s.Views.Table.CursorFg = d.Views.Table.CursorFg
	}
	if s.Views.Table.CursorBg == "" {
		s.Views.Table.CursorBg = d.Views.Table.CursorBg
	}
	if s.Views.Table.Header.Fg == "" {
		s.Views.Table.Header.Fg = d.Views.Table.Header.Fg
	}
	if s.Views.Table.Header.Bg == "" {
		s.Views.Table.Header.Bg = d.Views.Table.Header.Bg
	}
	if s.Agents == nil {
		s.Agents = map[string]string{}
	}
}

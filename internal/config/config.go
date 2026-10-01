// Package config loads the air9s config directory.
//
// The directory is $AIR9S_CONFIG_DIR, or $XDG_CONFIG_HOME/air9s, or
// ~/.config/air9s. It is not the macOS Application Support folder.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// File is the on-disk config.yaml document.
type File struct {
	Air9s Body `yaml:"air9s"`
}

// Body is the air9s: mapping.
type Body struct {
	RefreshRate   int    `yaml:"refreshRate"`
	ReadOnly      bool   `yaml:"readOnly"`
	DefaultView   string `yaml:"defaultView"`
	NoExitOnCtrlC bool   `yaml:"noExitOnCtrlC"`
	UI            UI     `yaml:"ui"`
}

// UI holds presentation settings. A nil EnableMouse means the mouse stays on.
type UI struct {
	EnableMouse *bool  `yaml:"enableMouse"`
	Headless    bool   `yaml:"headless"`
	Logoless    bool   `yaml:"logoless"`
	NoIcons     bool   `yaml:"noIcons"`
	Skin        string `yaml:"skin"`
	Limit       int    `yaml:"limit"`
}

// Loaded is a config directory after defaults, skins, and plugins are applied.
type Loaded struct {
	Dir      string
	Path     string
	Body     Body
	Skin     Skin
	SkinName string
	Plugins  []Plugin
	Warnings []string
}

// Dir resolves the config directory. It fails when no home is available and
// neither AIR9S_CONFIG_DIR nor XDG_CONFIG_HOME is set.
func Dir() (string, error) {
	if d := strings.TrimSpace(os.Getenv("AIR9S_CONFIG_DIR")); d != "" {
		return d, nil
	}
	if d := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); d != "" {
		return filepath.Join(d, "air9s"), nil
	}
	home := strings.TrimSpace(os.Getenv("HOME"))
	if home == "" {
		home = strings.TrimSpace(os.Getenv("USERPROFILE"))
	}
	if home == "" {
		return "", errors.New("air9s: home directory is unset")
	}
	return filepath.Join(home, ".config", "air9s"), nil
}

// Defaults is the built-in config used when no file is loaded.
func Defaults() Loaded {
	return Loaded{
		Body: Body{
			DefaultView: "sessions",
			UI: UI{
				Limit: 400,
			},
		},
		Skin:     defaultSkin(),
		SkinName: "built-in",
	}
}

// Mouse reports whether the UI should capture the mouse. An absent setting is on.
func (l Loaded) Mouse() bool {
	if l.Body.UI.EnableMouse == nil {
		return true
	}
	return *l.Body.UI.EnableMouse
}

// Limit is the table cap. Zero uses 400. Values above 2000 are clamped.
func (l Loaded) Limit() int {
	n := l.Body.UI.Limit
	if n <= 0 {
		return 400
	}
	if n > 2000 {
		return 2000
	}
	return n
}

// Load reads config.yaml, creating it and the example skin and plugin files
// when they are missing. A bad file returns the built-in config plus a warning
// rather than a hard error. Dir and write failures are returned.
func Load() (Loaded, error) {
	dir, err := Dir()
	if err != nil {
		return Defaults(), err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		out := Defaults()
		out.Dir = dir
		return out, err
	}
	path := filepath.Join(dir, "config.yaml")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(path, []byte(defaultConfigYAML), 0o600); err != nil {
			out := Defaults()
			out.Dir = dir
			out.Path = path
			return out, err
		}
	} else if err != nil {
		out := Defaults()
		out.Dir = dir
		out.Path = path
		return out, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		out := Defaults()
		out.Dir = dir
		out.Path = path
		return out, err
	}
	out := Defaults()
	out.Dir = dir
	out.Path = path
	var file File
	if err := yaml.Unmarshal(data, &file); err != nil {
		out.Warnings = append(out.Warnings, "config.yaml: "+err.Error())
		return out, nil
	}
	out.Body = file.Air9s
	out.Warnings = append(out.Warnings, out.normalize()...)
	if err := os.MkdirAll(filepath.Join(dir, "skins"), 0o700); err != nil {
		out.Warnings = append(out.Warnings, err.Error())
	} else if err := writeIfMissing(filepath.Join(dir, "skins", "stock.yaml"), stockSkinYAML); err != nil {
		out.Warnings = append(out.Warnings, err.Error())
	}
	if err := os.MkdirAll(filepath.Join(dir, "plugins"), 0o700); err != nil {
		out.Warnings = append(out.Warnings, err.Error())
	}
	if err := writeIfMissing(filepath.Join(dir, "plugins.yaml"), defaultPluginsYAML); err != nil {
		out.Warnings = append(out.Warnings, err.Error())
	}
	out.Warnings = append(out.Warnings, out.loadSkin()...)
	out.Warnings = append(out.Warnings, out.loadPlugins()...)
	return out, nil
}

func (l *Loaded) normalize() []string {
	var warns []string
	view := strings.ToLower(strings.TrimSpace(l.Body.DefaultView))
	switch view {
	case "", "sessions", "providers", "directories", "branches", "models":
		if view == "" {
			view = "sessions"
		}
		l.Body.DefaultView = view
	default:
		warns = append(warns, "defaultView "+l.Body.DefaultView+" is unknown; using sessions")
		l.Body.DefaultView = "sessions"
	}
	if l.Body.RefreshRate < 0 {
		warns = append(warns, "refreshRate must be >= 0; using 0")
		l.Body.RefreshRate = 0
	}
	if l.Body.UI.Limit < 0 {
		warns = append(warns, "ui.limit must be >= 0; using 400")
		l.Body.UI.Limit = 400
	}
	if l.Body.UI.Limit > 2000 {
		warns = append(warns, "ui.limit is capped at 2000")
		l.Body.UI.Limit = 2000
	}
	return warns
}

func writeIfMissing(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o600)
}

func (l *Loaded) loadSkin() []string {
	name := strings.TrimSpace(os.Getenv("AIR9S_SKIN"))
	if name == "" {
		name = strings.TrimSpace(l.Body.UI.Skin)
	}
	if name == "" {
		l.Skin = defaultSkin()
		l.SkinName = "built-in"
		return nil
	}
	if !validSkinName(name) {
		l.Skin = defaultSkin()
		l.SkinName = "built-in"
		return []string{"skin name " + name + " is not a file name; using the built-in skin"}
	}
	path := filepath.Join(l.Dir, "skins", name+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		l.Skin = defaultSkin()
		l.SkinName = "built-in"
		return []string{fmt.Sprintf("skin %s: %s; using the built-in skin", name, err.Error())}
	}
	var wrapped struct {
		Air9s Skin `yaml:"air9s"`
	}
	if err := yaml.Unmarshal(data, &wrapped); err != nil {
		l.Skin = defaultSkin()
		l.SkinName = "built-in"
		return []string{fmt.Sprintf("skin %s: %s; using the built-in skin", name, err.Error())}
	}
	skin := wrapped.Air9s
	if skin.Body.Fg == "" && skin.Body.Bg == "" && skin.Frame.Border.Fg == "" {
		var bare Skin
		if err := yaml.Unmarshal(data, &bare); err == nil {
			skin = bare
		}
	}
	skin.fill()
	l.Skin = skin
	l.SkinName = name
	return nil
}

func validSkinName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

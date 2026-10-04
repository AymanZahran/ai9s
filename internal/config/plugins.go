package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// Plugin is one user command bound to a shortcut.
type Plugin struct {
	Name        string   `yaml:"-"`
	ShortCut    string   `yaml:"shortCut"`
	Canon       string   `yaml:"-"`
	Description string   `yaml:"description"`
	Scopes      []string `yaml:"scopes"`
	Command     string   `yaml:"command"`
	Background  bool     `yaml:"background"`
	Args        []string `yaml:"args"`
}

// Allows reports whether the plugin is offered on this view.
// An empty scope list means every view.
func (p Plugin) Allows(view string) bool {
	if len(p.Scopes) == 0 {
		return true
	}
	view = strings.ToLower(view)
	for _, s := range p.Scopes {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "all" || s == view || (view == "bookmarks" && s == "sessions") {
			return true
		}
	}
	return false
}

func (l *Loaded) loadPlugins() []string {
	var warns []string
	seenName := map[string]bool{}
	seenKey := map[string]bool{}
	add := func(p Plugin) {
		if msg := l.prepare(&p, seenName, seenKey); msg != "" {
			warns = append(warns, msg)
			return
		}
		l.Plugins = append(l.Plugins, p)
	}
	path := filepath.Join(l.Dir, "plugins.yaml")
	if data, err := os.ReadFile(path); err == nil {
		warns = append(warns, readPluginDoc(data, "plugins.yaml", "", add)...)
	} else if !os.IsNotExist(err) {
		warns = append(warns, "plugins.yaml: "+err.Error())
	}
	dir := filepath.Join(l.Dir, "plugins")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			warns = append(warns, err.Error())
		}
		return warns
	}
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		full := filepath.Join(dir, name)
		data, err := os.ReadFile(full)
		if err != nil {
			warns = append(warns, name+": "+err.Error())
			continue
		}
		base := strings.TrimSuffix(name, filepath.Ext(name))
		warns = append(warns, readPluginDoc(data, name, base, add)...)
	}
	return warns
}

func readPluginDoc(data []byte, label, fileName string, add func(Plugin)) []string {
	var doc struct {
		Plugins     map[string]Plugin `yaml:"plugins"`
		ShortCut    string            `yaml:"shortCut"`
		Description string            `yaml:"description"`
		Scopes      []string          `yaml:"scopes"`
		Command     string            `yaml:"command"`
		Background  bool              `yaml:"background"`
		Args        []string          `yaml:"args"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return []string{label + ": " + err.Error()}
	}
	if len(doc.Plugins) > 0 {
		names := make([]string, 0, len(doc.Plugins))
		for name := range doc.Plugins {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			p := doc.Plugins[name]
			p.Name = name
			add(p)
		}
	}
	if strings.TrimSpace(doc.ShortCut) == "" && strings.TrimSpace(doc.Command) == "" {
		return nil
	}
	if fileName == "" {
		return []string{label + ": a plugin needs a name"}
	}
	add(Plugin{
		Name:        fileName,
		ShortCut:    doc.ShortCut,
		Description: doc.Description,
		Scopes:      doc.Scopes,
		Command:     doc.Command,
		Background:  doc.Background,
		Args:        doc.Args,
	})
	return nil
}

func (l *Loaded) prepare(p *Plugin, seenName, seenKey map[string]bool) string {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return "plugin is missing a name"
	}
	if seenName[p.Name] {
		return "plugin " + p.Name + " is already defined"
	}
	if strings.TrimSpace(p.Command) == "" {
		return "plugin " + p.Name + " has no command"
	}
	canon, ok := CanonShortcut(p.ShortCut)
	if !ok {
		return "plugin " + p.Name + " has an unrecognized shortcut " + p.ShortCut
	}
	if Blocked(canon) {
		return "plugin " + p.Name + " shortcut " + p.ShortCut + " is reserved"
	}
	if seenKey[canon] {
		return "plugin " + p.Name + " shortcut " + p.ShortCut + " is already used"
	}
	for i, scope := range p.Scopes {
		s := strings.ToLower(strings.TrimSpace(scope))
		switch s {
		case "all", "sessions", "providers", "directories", "branches", "models", "bookmarks":
			p.Scopes[i] = s
		default:
			return fmt.Sprintf("plugin %s has unknown scope %s", p.Name, scope)
		}
	}
	p.Canon = canon
	seenName[p.Name] = true
	seenKey[canon] = true
	return ""
}

// CanonShortcut normalizes a shortcut. A single letter keeps its case.
// Ctrl-E and shift-e are accepted. The bool is false when the text is not a shortcut.
func CanonShortcut(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	low := strings.ToLower(s)
	switch low {
	case "enter", "return", "tab", "backtab", "esc", "escape":
		return low, true
	}
	rs := []rune(low)
	if strings.HasPrefix(low, "ctrl-") && len(rs) == 6 {
		r := rs[5]
		if r >= 'a' && r <= 'z' {
			return "ctrl-" + string(r), true
		}
		return "", false
	}
	if strings.HasPrefix(low, "shift-") && len(rs) == 7 {
		r := rs[6]
		if r >= 'a' && r <= 'z' {
			return "shift-" + string(r), true
		}
		return "", false
	}
	one := []rune(s)
	if len(one) == 1 && unicode.IsGraphic(one[0]) && !unicode.IsSpace(one[0]) {
		return string(one), true
	}
	return "", false
}

// Blocked reports shortcuts the UI already uses.
func Blocked(canon string) bool {
	switch canon {
	case "q", "/", ":", "d", "f", "s", "n", "u", "?", "a", "p", "o", "j", "k", "h", "l", "g", "G",
		"1", "2", "3", "4", "5", "6",
		"enter", "return", "tab", "backtab", "esc", "escape",
		"ctrl-c", "ctrl-d", "shift-g":
		return true
	default:
		return false
	}
}

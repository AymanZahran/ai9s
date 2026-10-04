package tui

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"

	"github.com/AymanZahran/ai9s/internal/act"
	"github.com/AymanZahran/ai9s/internal/config"
	"github.com/AymanZahran/ai9s/internal/model"
	"github.com/gdamore/tcell/v2"
)

func (ui *ui) tryPlugin(ev *tcell.EventKey) bool {
	if ui.busy {
		return false
	}
	p, ok := ui.pluginFor(ev)
	if !ok {
		return false
	}
	view := ui.view
	if view == "" {
		view = viewSessions
	}
	if !p.Allows(view) {
		ui.alert(p.Name + " is not available in this view.")
		return true
	}
	ui.execPlugin(p)
	return true
}

func (ui *ui) pluginFor(ev *tcell.EventKey) (config.Plugin, bool) {
	for _, p := range ui.cfg.Plugins {
		if matchShortcut(p.Canon, ev) {
			return p, true
		}
	}
	return config.Plugin{}, false
}

func matchShortcut(canon string, ev *tcell.EventKey) bool {
	if strings.HasPrefix(canon, "ctrl-") && len(canon) == 6 {
		r := rune(canon[5])
		if r < 'a' || r > 'z' {
			return false
		}
		return ev.Key() == tcell.KeyCtrlA+tcell.Key(r-'a')
	}
	if canon == "shift-g" || canon == "G" {
		return ev.Key() == tcell.KeyRune && (ev.Rune() == 'G' || (ev.Rune() == 'g' && ev.Modifiers()&tcell.ModShift != 0))
	}
	if strings.HasPrefix(canon, "shift-") && len(canon) == 7 {
		r := unicode.ToUpper(rune(canon[6]))
		return ev.Key() == tcell.KeyRune && (ev.Rune() == r || (ev.Rune() == unicode.ToLower(r) && ev.Modifiers()&tcell.ModShift != 0))
	}
	rs := []rune(canon)
	return len(rs) == 1 && ev.Key() == tcell.KeyRune && ev.Rune() == rs[0]
}

func (ui *ui) execPlugin(p config.Plugin) {
	env := map[string]string{
		"FILTER": VisibleLine(ui.filter.GetText()),
		"NAME":   VisibleLine(p.Name),
	}
	var cwd string
	if ui.onSessions() {
		if s, ok := ui.selected(); ok {
			fillPluginEnv(env, s)
			cwd = s.CWD
		}
	} else {
		row, _ := ui.table.GetSelection()
		if row > 0 && row-1 < len(ui.groups) {
			g := ui.groups[row-1]
			env["NAME"] = VisibleLine(g.key)
			if g.sampleID != "" {
				if s, err := ui.store.Get(g.sampleID); err == nil {
					fillPluginEnv(env, s)
					env["NAME"] = VisibleLine(g.key)
					cwd = s.CWD
				}
			}
		}
	}
	binName := os.Expand(p.Command, func(k string) string { return env[k] })
	if strings.IndexFunc(binName, unicode.IsControl) >= 0 {
		ui.alert(p.Name + ": command contains a control character")
		return
	}
	bin, err := resolvePluginCommand(binName, ui.cfg.Dir)
	if err != nil {
		ui.alert(p.Name + ": " + err.Error())
		return
	}
	args := make([]string, len(p.Args))
	for i, arg := range p.Args {
		args[i] = os.Expand(arg, func(k string) string { return env[k] })
	}
	cmd := exec.Command(bin, args...)
	if cwd != "" && strings.IndexFunc(cwd, unicode.IsControl) < 0 {
		if info, err := os.Stat(cwd); err == nil && info.IsDir() {
			cmd.Dir = cwd
		}
	}
	if p.Background {
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		if err := cmd.Start(); err != nil {
			ui.alert(p.Name + ": " + err.Error())
			return
		}
		go func() { _ = cmd.Wait() }()
		return
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	act.WithTerminal(func() {
		ui.app.Suspend(func() {
			err = cmd.Run()
		})
	})
	if err != nil {
		ui.alert(p.Name + ": " + err.Error())
	}
}

// resolvePluginCommand finds a plugin program. A bare name is taken from
// PATH, then from the config plugins directory, so a script copied next to
// its yaml file runs without an install step. A path is used as given.
func resolvePluginCommand(command, configDir string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", fmt.Errorf("empty command")
	}
	if strings.ContainsAny(command, `/\`) {
		return exec.LookPath(command)
	}
	if path, err := exec.LookPath(command); err == nil {
		return path, nil
	}
	candidate := filepath.Join(configDir, "plugins", command)
	if executableFile(candidate) {
		return candidate, nil
	}
	return "", fmt.Errorf("executable %s not found", command)
}

func executableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}

func fillPluginEnv(env map[string]string, s model.Session) {
	env["ID"] = VisibleLine(s.ID)
	env["NATIVE_ID"] = VisibleLine(s.NativeID)
	env["AGENT"] = VisibleLine(s.Agent)
	env["CWD"] = VisibleLine(s.CWD)
	env["TITLE"] = VisibleLine(s.Title)
	env["BRANCH"] = VisibleLine(s.Branch)
	env["MODEL"] = VisibleLine(s.Model)
	env["NAME"] = VisibleLine(sessionName(s))
}

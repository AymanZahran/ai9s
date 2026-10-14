package tui

import (
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"unicode"

	"github.com/AymanZahran/ai9s/internal/act"
	"github.com/AymanZahran/ai9s/internal/model"
)

// launchHerdr starts AI9S_HERDR_OPEN for the selected session or harness.
// It reports whether that variable handled the key. Directory, branch, and
// model enters still pin inside the list.
func (ui *ui) launchHerdr(yolo bool) bool {
	hook := strings.TrimSpace(os.Getenv("AI9S_HERDR_OPEN"))
	if hook == "" {
		return false
	}
	if strings.IndexFunc(hook, unicode.IsControl) >= 0 {
		ui.alert("AI9S_HERDR_OPEN contains a control character")
		return true
	}
	if ui.onSessions() {
		s, ok := ui.selected()
		if !ok {
			return true
		}
		cmd, err := act.Plan(s, yolo)
		if err != nil {
			ui.alert(err.Error())
			return true
		}
		label := model.HarnessName(s.Agent)
		if name := strings.TrimSpace(s.Name); name != "" {
			label = label + " · " + name
		}
		ui.startHerdr(hook, map[string]string{
			"AI9S_ID":        s.ID,
			"AI9S_NATIVE_ID": s.NativeID,
			"AI9S_AGENT":     s.Agent,
			"AI9S_HARNESS":   model.HarnessName(s.Agent),
			"AI9S_BIN":       cmd.Name,
			"AI9S_CWD":       s.CWD,
			"AI9S_LABEL":     label,
			"AI9S_YOLO":      yoloFlag(yolo),
		})
		return true
	}
	if ui.view != viewAgents || yolo {
		return false
	}
	row, _ := ui.table.GetSelection()
	if row <= 0 || row-1 >= len(ui.groups) {
		return true
	}
	agent := ui.groups[row-1].key
	if agent == "" || agent == "(none)" {
		ui.alert("That row has no harness to open.")
		return true
	}
	bin, ok := act.Installed(agent)
	if !ok {
		ui.alert(model.HarnessName(agent) + " is not on PATH.")
		return true
	}
	ui.startHerdr(hook, map[string]string{
		"AI9S_ID":      "",
		"AI9S_AGENT":   agent,
		"AI9S_HARNESS": model.HarnessName(agent),
		"AI9S_BIN":     bin,
		"AI9S_CWD":     "",
		"AI9S_LABEL":   model.HarnessName(agent),
		"AI9S_YOLO":    "0",
	})
	return true
}

func yoloFlag(yolo bool) string {
	if yolo {
		return "1"
	}
	return "0"
}

func (ui *ui) startHerdr(hook string, fields map[string]string) {
	for key, value := range fields {
		fields[key] = VisibleLine(value)
	}
	cmd := exec.Command(hook)
	cmd.Env = mergeEnv(os.Environ(), fields)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		ui.alert(err.Error())
		return
	}
	go func() { _ = cmd.Wait() }()
}

func mergeEnv(base []string, extra map[string]string) []string {
	skip := map[string]bool{}
	for key := range extra {
		skip[key] = true
	}
	out := make([]string, 0, len(base)+len(extra))
	for _, pair := range base {
		key, _, ok := strings.Cut(pair, "=")
		if ok && skip[key] {
			continue
		}
		out = append(out, pair)
	}
	keys := make([]string, 0, len(extra))
	for key := range extra {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		out = append(out, key+"="+extra[key])
	}
	return out
}

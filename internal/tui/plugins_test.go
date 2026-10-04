package tui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/AymanZahran/ai9s/internal/config"
	"github.com/AymanZahran/ai9s/internal/store"
	"github.com/rivo/tview"
)

func TestResolvePluginCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no executable bit, and LookPath requires a PATHEXT suffix")
	}
	dir := t.TempDir()
	plug := filepath.Join(dir, "plugins")
	if err := os.MkdirAll(plug, 0o700); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(plug, "open-editor")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := resolvePluginCommand("open-editor", dir); err == nil {
		t.Fatal("a non-executable script was accepted")
	}
	if err := os.Chmod(script, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := resolvePluginCommand("open-editor", dir)
	if err != nil || got != script {
		t.Fatalf("resolve %q %v", got, err)
	}
	if _, err := resolvePluginCommand("  ", dir); err == nil {
		t.Fatal("empty command")
	}

	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	pathCmd := filepath.Join(bin, "open-editor")
	if err := os.WriteFile(pathCmd, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	got, err = resolvePluginCommand("open-editor", dir)
	if err != nil || got != pathCmd {
		t.Fatalf("PATH should win, got %q %v", got, err)
	}
}

func TestExamplePluginsResolve(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	src := filepath.Join(filepath.Dir(file), "..", "..", "examples", "plugins")
	dir := t.TempDir()
	plug := filepath.Join(dir, "plugins")
	if err := os.MkdirAll(plug, 0o700); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, ent := range entries {
		name := ent.Name()
		if name == "README.md" || ent.IsDir() {
			continue
		}
		body, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatal(err)
		}
		mode := os.FileMode(0o644)
		if filepath.Ext(name) == "" {
			mode = 0o755
		}
		if err := os.WriteFile(filepath.Join(plug, name), body, mode); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("AI9S_CONFIG_DIR", dir)
	t.Setenv("AI9S_SKIN", "")
	loaded, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"e": "open-editor",
		"c": "copy-session",
		"b": "git-story",
		"t": "new-terminal",
	}
	if len(loaded.Plugins) != len(want) {
		t.Fatalf("plugins %+v warnings %v", loaded.Plugins, loaded.Warnings)
	}
	for _, p := range loaded.Plugins {
		command, ok := want[p.Canon]
		if !ok || p.Command != command {
			t.Fatalf("plugin %+v", p)
		}
		path, err := resolvePluginCommand(p.Command, dir)
		if err != nil || path != filepath.Join(plug, command) {
			t.Fatalf("%s resolved to %q %v", p.Name, path, err)
		}
	}
}

func TestPluginHintOnMenu(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cfg := config.Defaults()
	cfg.Plugins = []config.Plugin{{
		Name: "open-editor", ShortCut: "e", Canon: "e", Description: "edit",
		Command: "open-editor", Scopes: []string{"sessions"},
	}}
	ui := newUI(tview.NewApplication(), st, cfg)
	text := ui.header.GetText(true)
	lines := strings.Split(text, "\n")
	if len(lines) < 3 {
		t.Fatalf("header %q", text)
	}
	if strings.Contains(lines[0], "<e>") || !strings.Contains(lines[0], "<1>") {
		t.Fatalf("view row %q", lines[0])
	}
	var sawPlugin bool
	for _, line := range lines[1:] {
		if strings.Contains(line, "<e>") && strings.Contains(line, "edit") && !strings.Contains(line, "<1>") {
			sawPlugin = true
		}
	}
	if !sawPlugin {
		t.Fatalf("plugin row missing %q", text)
	}
	ui.view = "agents"
	ui.paintHeader()
	if strings.Contains(ui.header.GetText(true), "<e>") {
		t.Fatal("sessions plugin shown on agents")
	}
}

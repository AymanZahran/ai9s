package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AIR9S_CONFIG_DIR", dir)
	got, err := Dir()
	if err != nil || got != dir {
		t.Fatalf("dir %q %v", got, err)
	}
}

func TestDirRequiresHome(t *testing.T) {
	t.Setenv("AIR9S_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	if _, err := Dir(); err == nil {
		t.Fatal("expected an error when home is unset")
	}
}

func TestLoadWritesOnceAndKeepsEdits(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AIR9S_CONFIG_DIR", dir)
	t.Setenv("AIR9S_SKIN", "")
	first, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !first.Mouse() || first.Limit() != 400 || first.SkinName != "built-in" {
		t.Fatalf("defaults %+v mouse %v", first.Body, first.Mouse())
	}
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "skins", "stock.yaml")); err != nil {
		t.Fatal(err)
	}
	edited := []byte("air9s:\n  readOnly: true\n  ui:\n    enableMouse: false\n    skin: stock\n    limit: 12\n")
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), edited, 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !second.Body.ReadOnly || second.Mouse() || second.Limit() != 12 || second.SkinName != "stock" {
		t.Fatalf("reload %+v mouse %v skin %s", second.Body, second.Mouse(), second.SkinName)
	}
	if second.Skin.Body.Bg != "black" || second.Skin.Body.Fg != "white" || second.Skin.Frame.Crumbs.Bg != "black" || second.Skin.Frame.Menu.Key != "white" || second.Skin.Frame.Menu.NumKey != "navajowhite" || second.Skin.Views.Table.CursorBg != "white" || second.Skin.Body.Logo != "white" {
		t.Fatalf("skin %+v %+v", second.Skin.Frame, second.Skin.Views.Table)
	}
}

func TestPluginCollisions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AIR9S_CONFIG_DIR", dir)
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	body := []byte(`plugins:
  ok:
    shortCut: Ctrl-E
    description: echo the directory
    scopes: [sessions]
    command: echo
    args: ["$CWD"]
  reserved:
    shortCut: d
    command: echo
  again:
    shortCut: ctrl-e
    command: echo
`)
	if err := os.WriteFile(filepath.Join(dir, "plugins.yaml"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	snap := []byte("shortCut: e\ncommand: echo\ndescription: from a file\nscopes: [nope]\n")
	if err := os.WriteFile(filepath.Join(dir, "plugins", "fileplug.yaml"), snap, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Plugins) != 1 || got.Plugins[0].Canon != "ctrl-e" {
		t.Fatalf("plugins %+v", got.Plugins)
	}
	text := strings.Join(got.Warnings, "\n")
	if !strings.Contains(text, "reserved") || !strings.Contains(text, "already used") || !strings.Contains(text, "unknown scope") {
		t.Fatalf("warnings %q", text)
	}
}

func TestBlockedShortcuts(t *testing.T) {
	if !Blocked("ctrl-d") || !Blocked("G") || !Blocked("shift-g") || Blocked("ctrl-e") || Blocked("E") {
		t.Fatal("shortcut reservation")
	}
	if c, ok := CanonShortcut("Shift-E"); !ok || c != "shift-e" {
		t.Fatalf("canon %q %v", c, ok)
	}
}

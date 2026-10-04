package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AI9S_CONFIG_DIR", dir)
	got, err := Dir()
	if err != nil || got != dir {
		t.Fatalf("dir %q %v", got, err)
	}
}

func TestDirRequiresHome(t *testing.T) {
	t.Setenv("AI9S_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	if _, err := Dir(); err == nil {
		t.Fatal("expected an error when home is unset")
	}
}

func TestLoadWritesOnceAndKeepsEdits(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AI9S_CONFIG_DIR", dir)
	t.Setenv("AI9S_SKIN", "")
	first, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !first.Mouse() || first.Limit() != 400 || first.SkinName != "built-in" || first.Body.RefreshRate != DefaultRefreshRate {
		t.Fatalf("defaults %+v mouse %v", first.Body, first.Mouse())
	}
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "skins", "stock.yaml")); err != nil {
		t.Fatal(err)
	}
	edited := []byte("ai9s:\n  readOnly: true\n  ui:\n    enableMouse: false\n    skin: stock\n    limit: 12\n")
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
	t.Setenv("AI9S_CONFIG_DIR", dir)
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

func TestDefaultSkinIsTrueBlack(t *testing.T) {
	s := Defaults().Skin
	if s.Body.Bg != "#000000" || s.Frame.Crumbs.Bg != "#000000" || s.Views.Table.Bg != "#000000" || s.Views.Table.Header.Bg != "#000000" {
		t.Fatalf("background %+v %+v", s.Body, s.Views.Table)
	}
	if s.Body.Fg != "lightskyblue" || s.Body.Logo != "orange" || s.Views.Table.CursorBg != "aqua" || s.Frame.Menu.NumKey != "fuchsia" || s.Frame.Border.Fg != "dodgerblue" {
		t.Fatalf("accents body %+v frame %+v table %+v", s.Body, s.Frame, s.Views.Table)
	}
}

func TestDefaultViewProvidersIsAgents(t *testing.T) {
	l := &Loaded{Body: Body{DefaultView: "Providers"}}
	if warns := l.normalize(); len(warns) != 0 || l.Body.DefaultView != "agents" {
		t.Fatalf("view %q warns %v", l.Body.DefaultView, warns)
	}
}

func TestProviderScopeIsAgents(t *testing.T) {
	p := Plugin{Name: "open-editor", ShortCut: "e", Command: "open-editor", Scopes: []string{"Providers"}}
	if msg := (&Loaded{}).prepare(&p, map[string]bool{}, map[string]bool{}); msg != "" {
		t.Fatal(msg)
	}
	if len(p.Scopes) != 1 || p.Scopes[0] != "agents" || !p.Allows("agents") || !p.Allows("providers") {
		t.Fatalf("scopes %+v allows agents %v providers %v", p.Scopes, p.Allows("agents"), p.Allows("providers"))
	}
}

func TestDefaultViewBookmarks(t *testing.T) {
	l := &Loaded{Body: Body{DefaultView: "Bookmarks"}}
	if warns := l.normalize(); len(warns) != 0 || l.Body.DefaultView != "bookmarks" {
		t.Fatalf("view %q warns %v", l.Body.DefaultView, warns)
	}
}

func TestBookmarksScopeMatchesSessions(t *testing.T) {
	sessions := Plugin{Scopes: []string{"sessions"}}
	if !sessions.Allows("bookmarks") || !sessions.Allows("sessions") || sessions.Allows("agents") || sessions.Allows("providers") {
		t.Fatal("sessions scope")
	}
	marks := Plugin{Scopes: []string{"bookmarks"}}
	if !marks.Allows("bookmarks") || marks.Allows("sessions") {
		t.Fatal("bookmarks scope")
	}
}

func TestBlockedShortcuts(t *testing.T) {
	if !Blocked("ctrl-d") || !Blocked("G") || !Blocked("shift-g") || !Blocked("h") || !Blocked("l") || !Blocked("n") || !Blocked("u") || !Blocked("f") || !Blocked("y") || !Blocked("6") || Blocked("r") || Blocked("b") || Blocked("e") || Blocked("ctrl-e") || Blocked("E") {
		t.Fatal("shortcut reservation")
	}
	if c, ok := CanonShortcut("Shift-E"); !ok || c != "shift-e" {
		t.Fatalf("canon %q %v", c, ok)
	}
}

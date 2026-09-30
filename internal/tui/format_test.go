package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/AymanZahran/ai9s/internal/model"
)

func TestSetTokenAndAge(t *testing.T) {
	got := setToken(`agent:claude "auth bug"`, "agent", "grok")
	if !strings.Contains(got, "agent:grok") || !strings.Contains(got, "auth bug") {
		t.Fatalf("token %q", got)
	}
	if setToken("sort:oldest ship", "sort", "") != "ship" {
		t.Fatalf("clear %+v", setToken("sort:oldest ship", "sort", ""))
	}
	if relAge(time.Time{}) != "-" {
		t.Fatal(relAge(time.Time{}))
	}
	text := preview(model.Session{Title: "hi", Agent: "claude", ID: "claude:1", CanDelete: true})
	if !strings.Contains(text, "hi") || !strings.Contains(text, "delete: yes") {
		t.Fatalf("preview %s", text)
	}
}

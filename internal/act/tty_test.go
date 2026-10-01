package act

import "testing"

func TestWithTerminalNested(t *testing.T) {
	n := 0
	WithTerminal(func() {
		n++
		WithTerminal(func() { n++ })
		if n != 2 {
			t.Fatalf("inner %d", n)
		}
	})
	if n != 2 {
		t.Fatalf("outer %d", n)
	}
}

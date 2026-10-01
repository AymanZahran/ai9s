package tui

import "testing"

func TestLayoutThumbReachesBothEnds(t *testing.T) {
	top := layoutBar(2, 5, 10, 0, 29, 5, 30)
	if top.top != 0 || top.length < 1 {
		t.Fatalf("top thumb %+v", top)
	}
	if got := top.posForThumbTop(0); got != 0 {
		t.Fatalf("top position %d", got)
	}
	bottom := layoutBar(2, 5, 10, 29, 29, 5, 30)
	if bottom.top+bottom.length != 10 {
		t.Fatalf("bottom thumb %+v", bottom)
	}
	if got := bottom.posForThumbTop(bottom.h); got != 29 {
		t.Fatalf("bottom position %d", got)
	}
	full := layoutBar(0, 0, 8, 0, 0, 8, 3)
	if full.length != 8 || full.maxPos != 0 {
		t.Fatalf("full thumb %+v", full)
	}
}

func TestWrappedRowsCountsShortLines(t *testing.T) {
	if got := wrappedRows("one\ntwo\nthree", 20); got != 3 {
		t.Fatalf("rows %d", got)
	}
	if got := wrappedRows("", 20); got != 0 {
		t.Fatalf("empty %d", got)
	}
	if got := oneLineRows("abcdefghij", 4); got < 3 {
		t.Fatalf("wide line %d", got)
	}
}

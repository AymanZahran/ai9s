package discover

import (
	"testing"

	"github.com/AymanZahran/ai9s/internal/model"
)

func TestScannerRosterIsTheHarnessSet(t *testing.T) {
	want := model.HarnessNames()
	got := Scanners()
	if len(got) != len(want) {
		t.Fatalf("scanners %d names %d", len(got), len(want))
	}
	for i, sc := range got {
		if sc.Agent != want[i][0] {
			t.Fatalf("order %d got %s want %s", i, sc.Agent, want[i][0])
		}
		if model.HarnessName(sc.Agent) == sc.Agent {
			t.Fatalf("missing name %s", sc.Agent)
		}
		if sc.Scan == nil {
			t.Fatalf("nil scan %s", sc.Agent)
		}
	}
}

package sqliteuri

import (
	"runtime"
	"strings"
	"testing"
)

func TestPath(t *testing.T) {
	if got := Path("/tmp/ai9s index.db", "mode=ro"); got != "file:///tmp/ai9s%20index.db?mode=ro" {
		t.Fatalf("slash %s", got)
	}
	if got := Path("C:/Users/ada/index.db", "_pragma=busy_timeout(5000)"); !strings.HasPrefix(got, "file:///C:/Users/ada/index.db?") {
		t.Fatalf("volume %s", got)
	}
	if runtime.GOOS == "windows" {
		if got := Path(`C:\Users\ada\index.db`, "mode=ro"); !strings.HasPrefix(got, "file:///C:/Users/ada/index.db?") {
			t.Fatalf("windows %s", got)
		}
	}
}

//go:build unix

package act

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

var (
	ttyMu   sync.Mutex
	ttyHeld int
)

// WithTerminal ignores SIGTTOU and SIGTTIN for the duration of fn.
// Nested calls keep the signals ignored until the outermost call returns.
// A TUI holds the outer call across screen.Resume, which writes to the
// terminal immediately after the child exits. Resetting inside the child
// wait lets that write stop ai9s with "suspended (tty output)".
// The child inherits the ignore. SIGINT and SIGTSTP stay at their defaults.
func WithTerminal(fn func()) {
	ttyMu.Lock()
	if ttyHeld == 0 {
		signal.Ignore(syscall.SIGTTOU, syscall.SIGTTIN)
	}
	ttyHeld++
	ttyMu.Unlock()
	defer func() {
		ttyMu.Lock()
		ttyHeld--
		if ttyHeld == 0 {
			signal.Reset(syscall.SIGTTOU, syscall.SIGTTIN)
		}
		ttyMu.Unlock()
	}()
	fn()
}

// reclaimForeground makes this process group the terminal foreground again.
// The group captured before the child started can be stale once that group
// has exited, and the ioctl itself raises SIGTTOU unless it is ignored.
func reclaimForeground() {
	target := unix.Getpgrp()
	if target <= 0 {
		return
	}
	try := func(fd int) bool {
		for i := 0; i < 5; i++ {
			if unix.IoctlSetPointerInt(fd, unix.TIOCSPGRP, target) == nil {
				return true
			}
			time.Sleep(20 * time.Millisecond)
		}
		return false
	}
	if tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		ok := try(int(tty.Fd()))
		_ = tty.Close()
		if ok {
			return
		}
	}
	_ = try(int(os.Stdin.Fd()))
}

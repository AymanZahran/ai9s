//go:build unix

package act

import (
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

// runAttached starts cmd in its own process group and, when stdin is a
// terminal, makes that group the foreground. Ctrl-C then goes to the agent.
// The previous foreground group is restored before returning, so a caller
// that suspended a TUI can draw again.
func runAttached(cmd *exec.Cmd) error {
	fd := int(os.Stdin.Fd())
	old, err := unix.IoctlGetInt(fd, unix.TIOCGPGRP)
	if err != nil {
		return cmd.Run()
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	if err := cmd.Start(); err != nil {
		return err
	}
	pgid, gerr := syscall.Getpgid(cmd.Process.Pid)
	if gerr == nil {
		_ = unix.IoctlSetPointerInt(fd, unix.TIOCSPGRP, pgid)
	}
	waitErr := cmd.Wait()
	_ = unix.IoctlSetPointerInt(fd, unix.TIOCSPGRP, old)
	return waitErr
}

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
// This process group is put back in the foreground before returning.
// Callers that draw after this returns must already be inside WithTerminal,
// because the restore and the following write both raise SIGTTOU otherwise.
func runAttached(cmd *exec.Cmd) error {
	fd := int(os.Stdin.Fd())
	if _, err := unix.IoctlGetInt(fd, unix.TIOCGPGRP); err != nil {
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
	reclaimForeground()
	return waitErr
}

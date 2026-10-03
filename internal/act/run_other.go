//go:build !unix

package act

import "os/exec"

func runAttached(cmd *exec.Cmd) error {
	return cmd.Run()
}

func WithTerminal(fn func()) { fn() }

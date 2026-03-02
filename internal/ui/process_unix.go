//go:build !windows

package ui

import (
	"os/exec"
	"syscall"
)

// setDetachedProcessAttrs configures detached child process attributes on Unix systems.
func setDetachedProcessAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

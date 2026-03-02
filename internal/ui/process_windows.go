//go:build windows

package ui

import (
	"os/exec"
	"syscall"
)

// setDetachedProcessAttrs configures detached child process attributes on Windows.
func setDetachedProcessAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

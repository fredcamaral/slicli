//go:build !windows
// +build !windows

package main

import (
	"os/exec"
	"syscall"
)

// setProcessGroupUnix sets up process group on Unix-like systems for easier cleanup
func setProcessGroupUnix(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// setProcessGroupForCleanup configures process group settings for cleanup
func setProcessGroupForCleanup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	cmd.SysProcAttr.Pgid = 0
}

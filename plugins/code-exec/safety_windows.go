//go:build windows
// +build windows

package main

import (
	"os/exec"
)

// setProcessGroupUnix is a no-op on Windows (process groups work differently)
func setProcessGroupUnix(cmd *exec.Cmd) {
	// Windows handles process isolation differently through job objects
	// No need to set process group here
}

// setProcessGroupForCleanup is a no-op on Windows
func setProcessGroupForCleanup(cmd *exec.Cmd) {
	// Windows handles process cleanup differently
}

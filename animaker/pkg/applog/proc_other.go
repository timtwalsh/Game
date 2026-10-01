//go:build !windows

package applog

import (
	"os"
	"syscall"
)

// processRunning reports whether a process with this ID is still running.
func processRunning(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}

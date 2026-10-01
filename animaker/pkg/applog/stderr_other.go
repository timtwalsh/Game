//go:build !windows

package applog

import (
	"os"
	"runtime/debug"
)

// captureRuntimeOutput sends Go's crash output to f as well as stderr.
// Unlike the Windows version it can miss a fatal error's message line
// (the stack is still recorded); the editor's target platform is Windows.
func captureRuntimeOutput(f *os.File) (restore func(), err error) {
	if err := debug.SetCrashOutput(f, debug.CrashOptions{}); err != nil {
		return nil, err
	}
	return func() { debug.SetCrashOutput(nil, debug.CrashOptions{}) }, nil
}

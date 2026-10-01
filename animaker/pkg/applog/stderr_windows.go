package applog

import (
	"os"
	"runtime/debug"
	"syscall"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procSetStdHandle = kernel32.NewProc("SetStdHandle")
)

// stdErrorHandle is STD_ERROR_HANDLE, (DWORD)-12.
const stdErrorHandle = ^uint32(11)

// captureRuntimeOutput points the process's standard-error handle at f.
//
// debug.SetCrashOutput alone isn't enough: for a runtime fatal error (e.g.
// "fatal error: concurrent map iteration and map write") it records the
// stack but the message line still goes only to stderr. On Windows the Go
// runtime fetches the stderr handle afresh for every write, so redirecting
// the handle captures everything the runtime prints, message included.
// It returns a function that restores the original handle.
func captureRuntimeOutput(f *os.File) (restore func(), err error) {
	orig, err := syscall.GetStdHandle(syscall.STD_ERROR_HANDLE)
	if err != nil {
		return nil, err
	}
	if r, _, e := procSetStdHandle.Call(uintptr(stdErrorHandle), f.Fd()); r == 0 {
		// Fall back to the portable mechanism rather than capturing nothing.
		if err := debug.SetCrashOutput(f, debug.CrashOptions{}); err != nil {
			return nil, err
		}
		_ = e
		return func() { debug.SetCrashOutput(nil, debug.CrashOptions{}) }, nil
	}
	return func() { procSetStdHandle.Call(uintptr(stdErrorHandle), uintptr(orig)) }, nil
}

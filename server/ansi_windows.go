package main

import (
	"os"
	"syscall"
	"unsafe"
)

// enableANSI turns on virtual-terminal processing for stdout so the
// status board can redraw in place in a plain console window. It reports
// false when stdout isn't a console (e.g. redirected to a file).
func enableANSI() bool {
	const enableVirtualTerminalProcessing = 0x0004
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getMode := kernel32.NewProc("GetConsoleMode")
	setMode := kernel32.NewProc("SetConsoleMode")

	h := os.Stdout.Fd()
	var mode uint32
	if r, _, _ := getMode.Call(h, uintptr(unsafe.Pointer(&mode))); r == 0 {
		return false
	}
	if mode&enableVirtualTerminalProcessing != 0 {
		return true
	}
	r, _, _ := setMode.Call(h, uintptr(mode|enableVirtualTerminalProcessing))
	return r != 0
}

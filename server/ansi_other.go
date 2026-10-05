//go:build !windows

package main

import "os"

// enableANSI reports whether stdout is a terminal; Unix terminals
// understand ANSI cursor movement without any setup.
func enableANSI() bool {
	fi, err := os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

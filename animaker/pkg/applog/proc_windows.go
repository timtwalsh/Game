package applog

import "syscall"

// stillActive is STILL_ACTIVE, the exit code Windows reports for a process
// that hasn't exited.
const stillActive = 259

// processRunning reports whether a process with this ID is still running.
func processRunning(pid int) bool {
	const queryLimitedInformation = 0x1000 // PROCESS_QUERY_LIMITED_INFORMATION
	h, err := syscall.OpenProcess(queryLimitedInformation, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(h)
	var code uint32
	if err := syscall.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == stillActive
}

// Package applog gives each editor session a log file, so that a crash
// leaves something behind. Before this, a crash printed its stack trace to
// the console window build_local.ps1 opens, and that window closed the
// moment the program died - so a crash left no trace at all.
//
// A session log receives:
//   - Go's crash output (see captureRuntimeOutput): the message and stack
//     for an unrecovered panic or a runtime fatal error such as a
//     concurrent map write, which can't be recovered in-process;
//   - everything written through the standard log package, which is where
//     Fyne reports its own errors and warnings;
//   - errors the editor shows the artist (see Errorf).
//
// The file ends with cleanExitMarker when the editor shuts down normally,
// which is how the next session tells that the previous one crashed.
package applog

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	cleanExitMarker = "=== clean exit ==="
	filePrefix      = "animaker-"
	fileSuffix      = ".log"
	// KeepSessions is how many session logs are kept; older ones are
	// deleted at startup so the folder doesn't grow forever.
	KeepSessions = 20
)

// Session is one run's log file.
type Session struct {
	Path    string
	f       *os.File
	restore func() // undoes captureRuntimeOutput

	// PreviousCrashLog is the previous session's log if that session
	// didn't exit cleanly, else "".
	PreviousCrashLog string
}

// DefaultDir is a "logs" folder beside the executable (bin\logs when built
// by build_local.ps1), falling back to the working directory.
func DefaultDir() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "logs")
	}
	return "logs"
}

// Start opens a new session log in dir and routes crash output and the
// standard logger into it.
func Start(dir string) (*Session, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating log folder: %w", err)
	}
	prev := sessionLogs(dir)

	name := filePrefix + time.Now().Format("20060102-150405") + fileSuffix
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("creating log file: %w", err)
	}
	s := &Session{Path: path, f: f}
	if len(prev) > 0 {
		if last := prev[len(prev)-1]; last != path && crashed(last) {
			s.PreviousCrashLog = last
		}
	}

	// The file first: io.MultiWriter stops at the first failing writer,
	// and a GUI process's stderr may not be writable at all.
	log.SetOutput(io.MultiWriter(f, os.Stderr))
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	if restore, err := captureRuntimeOutput(f); err != nil {
		log.Printf("applog: crash output not captured: %v", err)
	} else {
		s.restore = restore
	}
	log.Printf("=== animaker session started (log %s) %s%d ===", path, pidField, os.Getpid())

	prune(dir, KeepSessions)
	return s, nil
}

// Errorf logs an error the editor is about to show the artist, so it's in
// the session log alongside whatever led up to it.
func Errorf(format string, args ...any) {
	log.Printf("ERROR: "+format, args...)
}

// Close marks the session as having exited cleanly and releases the file.
// Call it on normal shutdown only - its absence is what flags a crash.
func (s *Session) Close() error {
	log.Print(cleanExitMarker)
	log.SetOutput(os.Stderr)
	if s.restore != nil {
		s.restore()
	}
	return s.f.Close()
}

// sessionLogs lists dir's session logs, oldest first (the timestamped
// names sort chronologically).
func sessionLogs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var logs []string
	for _, e := range entries {
		if n := e.Name(); !e.IsDir() && strings.HasPrefix(n, filePrefix) && strings.HasSuffix(n, fileSuffix) {
			logs = append(logs, filepath.Join(dir, n))
		}
	}
	sort.Strings(logs)
	return logs
}

// pidField tags the session's process ID in its first line, so a later
// session can tell a log that's unfinished because its editor is still
// open from one that's unfinished because its editor died.
const pidField = "pid="

// crashed reports whether a session log is from an editor that died: it
// lacks the clean-exit marker, and the process that wrote it isn't still
// running. Without the second check, opening a second editor while one
// was already open reported the open one as crashed.
func crashed(path string) bool {
	if endedCleanly(path) {
		return false
	}
	if pid, ok := sessionPID(path); ok && processRunning(pid) {
		return false
	}
	return true
}

// sessionPID reads the process ID from a session log's first line.
func sessionPID(path string) (int, bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, false
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	line := string(buf[:n])
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	i := strings.Index(line, pidField)
	if i < 0 {
		return 0, false
	}
	var pid int
	if _, err := fmt.Sscanf(line[i+len(pidField):], "%d", &pid); err != nil {
		return 0, false
	}
	return pid, true
}

// endedCleanly reports whether a session log's last line is the clean-exit
// marker. Only the tail is read; a crash dump can be long.
func endedCleanly(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return true // can't tell; don't cry wolf
	}
	defer f.Close()
	const tail = 512
	if info, err := f.Stat(); err == nil && info.Size() > tail {
		f.Seek(-tail, io.SeekEnd)
	}
	b, _ := io.ReadAll(f)
	return strings.HasSuffix(strings.TrimRight(string(b), "\r\n"), cleanExitMarker)
}

// prune deletes all but the newest keep session logs.
func prune(dir string, keep int) {
	logs := sessionLogs(dir)
	for i := 0; i < len(logs)-keep; i++ {
		os.Remove(logs[i])
	}
}

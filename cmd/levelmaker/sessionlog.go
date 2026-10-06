package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"time"
)

// The level maker's session log follows the animaker's convention
// (animaker/pkg/applog): one log file per run in a "logs" folder beside the
// executable (bin\logs when built by build_local.ps1), recording errors
// shown to the artist, everything sent through the standard log package,
// and a panic's message and stack. The file ends with cleanExitMarker on a
// normal exit, which is how the next run tells the previous one crashed.
//
// Unlike applog it doesn't redirect the process's stderr, so a Go fatal
// error that can't be recovered (such as a concurrent map write) isn't
// captured; the editor runs on raylib's single main goroutine, so a
// recovered panic covers the crashes it can actually have.
const (
	logPrefix       = "levelmaker-"
	logSuffix       = ".log"
	cleanExitMarker = "=== clean exit ==="
	keepLogs        = 20
)

type sessionLog struct {
	Path string
	f    *os.File
	// PreviousCrash is the previous run's log if it didn't exit cleanly.
	PreviousCrash string
}

// logDir is a "logs" folder beside the executable, falling back to the
// working directory.
func logDir() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "logs")
	}
	return "logs"
}

// startLog opens a new session log in dir, routes the standard logger into
// it (and stderr), and prunes old logs.
func startLog(dir string) (*sessionLog, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating log folder: %w", err)
	}
	prev := sessionLogs(dir)
	var err error
	// Names are timestamps, so they sort in session order. Created
	// exclusively: two sessions started in the same millisecond (two
	// editors, or a test) must not share a file, so a clash waits for the
	// next millisecond's name.
	var path string
	var f *os.File
	for tries := 0; ; tries++ {
		path = filepath.Join(dir, logPrefix+time.Now().Format("20060102-150405.000")+logSuffix)
		f, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			break
		}
		if !os.IsExist(err) || tries >= 100 {
			return nil, fmt.Errorf("creating log file: %w", err)
		}
		time.Sleep(time.Millisecond)
	}
	s := &sessionLog{Path: path, f: f}
	if len(prev) > 0 && !cleanlyClosed(prev[len(prev)-1]) {
		s.PreviousCrash = prev[len(prev)-1]
	}
	// Prune, keeping room for this session.
	for len(prev) >= keepLogs {
		os.Remove(prev[0])
		prev = prev[1:]
	}
	log.SetOutput(io.MultiWriter(f, os.Stderr))
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	log.Printf("level maker session started")
	return s, nil
}

// sessionLogs lists the logs in dir, oldest first (names sort by time).
func sessionLogs(dir string) []string {
	entries, _ := os.ReadDir(dir)
	var out []string
	for _, e := range entries {
		if n := e.Name(); strings.HasPrefix(n, logPrefix) && strings.HasSuffix(n, logSuffix) {
			out = append(out, filepath.Join(dir, n))
		}
	}
	sort.Strings(out)
	return out
}

func cleanlyClosed(path string) bool {
	data, err := os.ReadFile(path)
	return err == nil && bytes.HasSuffix(bytes.TrimSpace(data), []byte(cleanExitMarker))
}

// Errorf logs an error shown to the artist and returns it.
func (s *sessionLog) Errorf(format string, args ...any) error {
	err := fmt.Errorf(format, args...)
	log.Printf("error: %v", err)
	return err
}

// Crashed records a recovered panic with its stack.
func (s *sessionLog) Crashed(r any) {
	log.Printf("panic: %v\n%s", r, debug.Stack())
	s.f.Sync()
}

// Close marks the session as cleanly ended.
func (s *sessionLog) Close() {
	log.SetOutput(os.Stderr)
	fmt.Fprintln(s.f, cleanExitMarker)
	s.f.Close()
}

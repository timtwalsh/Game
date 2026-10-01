package applog

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Session log names are second-resolution timestamps, so consecutive
// sessions in a test need a different second to get different files.
func nextSecond() {
	time.Sleep(time.Until(time.Now().Truncate(time.Second).Add(time.Second + 10*time.Millisecond)))
}

func TestCleanExitIsNotReportedAsACrash(t *testing.T) {
	dir := t.TempDir()
	s, err := Start(dir)
	if err != nil {
		t.Fatal(err)
	}
	log.Print("doing work")
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	nextSecond()
	s2, err := Start(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if s2.PreviousCrashLog != "" {
		t.Errorf("clean previous session reported as crashed: %s", s2.PreviousCrashLog)
	}
}

// A session that never reached Close - the process died - is reported to
// the next one, with its log path.
func TestUncleanSessionIsReportedNextTime(t *testing.T) {
	dir := t.TempDir()
	s, err := Start(dir)
	if err != nil {
		t.Fatal(err)
	}
	Errorf("something broke: %v", "boom")
	crashed := s.Path
	// Simulate dying: release the file without the clean-exit marker, and
	// make the log's writer a process that has exited.
	log.SetOutput(os.Stderr)
	s.restore()
	s.f.Close()
	setLogPID(t, crashed, deadPID(t))

	nextSecond()
	s2, err := Start(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if s2.PreviousCrashLog != crashed {
		t.Errorf("PreviousCrashLog = %q, want %q", s2.PreviousCrashLog, crashed)
	}

	data, _ := os.ReadFile(crashed)
	if !strings.Contains(string(data), "ERROR: something broke: boom") {
		t.Errorf("crashed log lacks the logged error:\n%s", data)
	}
}

func TestOldLogsArePruned(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < KeepSessions+5; i++ {
		name := fmt.Sprintf("%s20260101-0000%02d%s", filePrefix, i, fileSuffix)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(cleanExitMarker), 0644); err != nil {
			t.Fatal(err)
		}
	}
	s, err := Start(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	logs := sessionLogs(dir)
	if len(logs) != KeepSessions {
		t.Errorf("%d logs kept, want %d", len(logs), KeepSessions)
	}
	if logs[len(logs)-1] != s.Path {
		t.Errorf("newest log %s isn't this session's %s", logs[len(logs)-1], s.Path)
	}
}

// Reported as a false alarm: a second editor opened while the first was
// still open said the first had crashed, since its log isn't finished yet.
func TestStillRunningSessionIsNotACrash(t *testing.T) {
	dir := t.TempDir()
	s, err := Start(dir)
	if err != nil {
		t.Fatal(err)
	}
	open := s.Path // this process, still running, never Closed
	log.SetOutput(os.Stderr)
	s.restore()
	s.f.Close()

	nextSecond()
	s2, err := Start(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if s2.PreviousCrashLog != "" {
		t.Errorf("a still-running session (%s) was reported as crashed", open)
	}
}

// deadPID is the ID of a process that has already exited.
func deadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	return cmd.Process.Pid
}

// setLogPID rewrites a session log's recorded process ID.
func setLogPID(t *testing.T, path string, pid int) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	i := strings.Index(s, pidField)
	j := i + len(pidField)
	for j < len(s) && s[j] >= '0' && s[j] <= '9' {
		j++
	}
	s = s[:i] + fmt.Sprintf("%s%d", pidField, pid) + s[j:]
	if err := os.WriteFile(path, []byte(s), 0644); err != nil {
		t.Fatal(err)
	}
}

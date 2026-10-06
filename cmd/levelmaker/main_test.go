package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTextFieldSelectsOnFocus(t *testing.T) {
	f := textField{Text: "meadow", Allow: levelNameRune}
	f.Type('x') // not focused: appends
	if f.Text != "meadowx" {
		t.Fatalf("Text = %q", f.Text)
	}
	f.Focus()
	f.Type('l')
	f.Type('a')
	if f.Text != "la" {
		t.Errorf("typing after focus = %q, want the old value replaced", f.Text)
	}
	f.Type('Q') // filtered out
	f.Type(' ')
	if f.Text != "la" {
		t.Errorf("filtered runes got in: %q", f.Text)
	}
	f.Backspace()
	if f.Text != "l" {
		t.Errorf("Backspace = %q", f.Text)
	}
	f.Focus()
	f.Backspace()
	if f.Text != "" {
		t.Errorf("Backspace on a selection = %q, want empty", f.Text)
	}
}

func TestTrimZoom(t *testing.T) {
	for z, want := range map[float32]string{2: "2", 0.5: "0.5", 1.0000001: "1", 1.189207: "1.19", 0.25: "0.25"} {
		if got := trimZoom(z); got != want {
			t.Errorf("trimZoom(%v) = %q, want %q", z, got, want)
		}
	}
}

func TestTextFieldMaxLen(t *testing.T) {
	f := textField{MaxLen: 3, Allow: intRune}
	for _, r := range "-12345" {
		f.Type(r)
	}
	if f.Text != "-12" {
		t.Errorf("Text = %q, want capped at 3", f.Text)
	}
}

func TestSessionLogDetectsCrashAndPrunes(t *testing.T) {
	dir := t.TempDir()
	defer log.SetOutput(os.Stderr)

	s, err := startLog(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.PreviousCrash != "" {
		t.Error("first session reported a previous crash")
	}
	s.Errorf("can't save %s", "meadow")
	s.Close()
	data, _ := os.ReadFile(s.Path)
	if !strings.Contains(string(data), "error: can't save meadow") || !cleanlyClosed(s.Path) {
		t.Errorf("log = %q", data)
	}

	// A session that never closes looks like a crash to the next one.
	crashed, _ := startLog(dir)
	crashed.Crashed("boom")
	next, _ := startLog(dir)
	if next.PreviousCrash != crashed.Path {
		t.Errorf("PreviousCrash = %q, want %q", next.PreviousCrash, crashed.Path)
	}
	data, _ = os.ReadFile(crashed.Path)
	if !strings.Contains(string(data), "panic: boom") || !strings.Contains(string(data), "goroutine") {
		t.Errorf("crash log lacks the panic and stack: %q", data)
	}
	next.Close()
	crashed.f.Close()

	// Old logs are pruned to keepLogs.
	for i := 0; i < keepLogs+5; i++ {
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s19990101-000000.%03d%s", logPrefix, i, logSuffix)), []byte(cleanExitMarker), 0o644)
	}
	last, _ := startLog(dir)
	last.Close()
	if n := len(sessionLogs(dir)); n != keepLogs {
		t.Errorf("%d logs kept, want %d", n, keepLogs)
	}
}

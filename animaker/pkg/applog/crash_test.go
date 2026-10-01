package applog

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// crashChildEnv makes the test binary act as a crashing editor: start a
// session, then die in the requested way. Real crashes can't be caught
// in-process, so the parent test runs this as a separate process.
const crashChildEnv = "APPLOG_CRASH_CHILD"

func TestMain(m *testing.M) {
	if kind := os.Getenv(crashChildEnv); kind != "" {
		if _, err := Start(os.Getenv("APPLOG_CRASH_DIR")); err != nil {
			os.Exit(3)
		}
		switch kind {
		case "panic":
			panic("simulated editor panic")
		case "concurrent-map":
			// The suspected real-world crash: a map read on one goroutine
			// while another writes it. This is a runtime fatal error, which
			// no recover() can catch.
			m := map[int]int{}
			go func() {
				for i := 0; ; i++ {
					m[i%64] = i
				}
			}()
			for {
				for range m {
				}
			}
		}
	}
	os.Exit(m.Run())
}

func TestCrashIsWrittenToTheSessionLog(t *testing.T) {
	for _, tc := range []struct{ kind, want string }{
		{"panic", "panic: simulated editor panic"},
		{"concurrent-map", "fatal error: concurrent map"}, // the message line, not just the stack
	} {
		t.Run(tc.kind, func(t *testing.T) {
			dir := t.TempDir()
			cmd := exec.Command(os.Args[0], "-test.run=^$")
			cmd.Env = append(os.Environ(), crashChildEnv+"="+tc.kind, "APPLOG_CRASH_DIR="+dir)
			if err := cmd.Run(); err == nil {
				t.Fatal("child exited cleanly; expected a crash")
			}

			logs := sessionLogs(dir)
			if len(logs) != 1 {
				t.Fatalf("%d session logs, want 1", len(logs))
			}
			data, _ := os.ReadFile(logs[0])
			out := string(data)
			if !strings.Contains(out, tc.want) {
				t.Errorf("log lacks %q:\n%s", tc.want, out)
			}
			if !strings.Contains(out, "goroutine ") {
				t.Errorf("log lacks a stack trace:\n%s", out)
			}
			if endedCleanly(logs[0]) {
				t.Error("crashed session's log looks like a clean exit")
			}
		})
	}
}

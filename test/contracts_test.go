package test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// TestRawJSONAnswer preserves agent key order and JSON spelling through runs and hooks.
func TestRawJSONAnswer(t *testing.T) {
	s := fresh(t)
	answer := `{"z":"<>&🐧","a":"\ud800","2":2,"1":1}`
	s.hook("seen", []string{"result"}, ":")
	r := s.run([]string{"-m", "fake:small", "--json", "go"}, "", map[string]string{"FAKE_ANSWER": answer})
	eq(t, r.code, 0)
	eq(t, r.stdout, answer+"\n")
	eq(t, s.ask("show", runID(t, r.stderr)).stdout, answer+"\n")
	stored := s.read(filepath.Join(s.home, "runs", s.latest(), "results.json"))
	if strings.Index(stored, `"z"`) > strings.Index(stored, `"1"`+":") {
		t.Fatal("reordered raw answer")
	}
}

// TestUTF8Chunks checks decoding across reads and replacement of a truncated multibyte character.
func TestUTF8Chunks(t *testing.T) {
	s := fresh(t)
	s.localAgent("stream", `cat >/dev/null; printf '\360\237'; sleep .01; printf '\220\247\342\202'`)
	r := s.ask("-m", "stream:small", "go")
	eq(t, r.code, 0)
	eq(t, r.stdout, "🐧�\n")
}

// TestResolvedBinary checks that commands receive the same resolved ask binary through a symlink.
func TestResolvedBinary(t *testing.T) {
	s := fresh(t)
	link := filepath.Join(s.tmp, "ask-link")
	if e := os.Symlink(askBin, link); e != nil {
		t.Fatal(e)
	}
	s.script("commands", "seen", `printf '%s\n' "$ASK_BIN"`)
	old := askBin
	askBin = link
	t.Cleanup(func() { askBin = old })
	r := s.ask("seen")
	eq(t, r.code, 0)
	// The binary's own path, symlinks resolved too: macOS's temp folder sits under a symlink.
	want, e := filepath.EvalSymlinks(old)
	if e != nil {
		t.Fatal(e)
	}
	eq(t, r.stdout, want+"\n")
}

// TestParentDeath checks the Linux guarantee that a killed ask cannot leave its direct agent alive.
func TestParentDeath(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux Pdeathsig contract")
	}
	s := fresh(t)
	run := s.start([]string{"-m", "fake:small", "hang"}, "", map[string]string{"FAKE_HANG": "hang", "FAKE_IGNORE_TERM": "1"})
	until(t, func() bool { return len(s.calls()) == 1 })
	pid := int(s.calls()[0].n("pid"))
	if e := run.cmd.Process.Signal(syscall.SIGKILL); e != nil {
		t.Fatal(e)
	}
	eq(t, run.wait(t).code, -1)
	until(t, func() bool {
		b, e := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
		// The process can vanish while its stat is read; either way it is gone.
		if os.IsNotExist(e) || errors.Is(e, syscall.ESRCH) {
			return true
		}
		if e != nil {
			t.Fatal(e)
		}
		_, state, _ := strings.Cut(string(b), ") ")
		return strings.HasPrefix(state, "Z ")
	})
}

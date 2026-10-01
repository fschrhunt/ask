package test

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// TestJSONBytes protects JSON property order, literal Unicode and surrogate escaping.
func TestJSONBytes(t *testing.T) {
	s := fresh(t)
	r := s.run([]string{"-m", "fake:small", "--json", "go"}, "", map[string]string{"FAKE_ANSWER": `{"z":"<>&🐧","a":"\ud800","2":2,"1":1}`})
	eq(t, r.code, 0)
	eq(t, r.stdout, "{\"1\":1,\"2\":2,\"z\":\"<>&🐧\",\"a\":\"\\ud800\"}\n")
}

// TestDecimalRounding pins status formatting where JavaScript and Go's usual rounding disagree.
func TestDecimalRounding(t *testing.T) {
	s := fresh(t)
	s.localAgent("echo", `echo '{"input":1250,"output":2500,"cost":1.125}' > "$ASK_REPORT"; echo fine`)
	r := s.ask("-m", "echo:small", "go")
	eq(t, r.code, 0)
	match(t, r.stderr, `1\.3k in · 2\.5k out · \$1\.13`)
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
		if os.IsNotExist(e) {
			return true
		}
		if e != nil {
			t.Fatal(e)
		}
		_, state, _ := strings.Cut(string(b), ") ")
		return strings.HasPrefix(state, "Z ")
	})
}

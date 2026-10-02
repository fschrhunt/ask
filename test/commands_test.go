package test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestCommands pins ask NAME: user commands, their arguments, exit codes and signals.
func TestCommands(t *testing.T) {
	t.Run("a command gets the resolved ask binary, even through a symlink", func(t *testing.T) {
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
	})
	t.Run("a command replaces ask's process, so it gets signals directly", func(t *testing.T) {
		s := fresh(t)
		s.script("commands", "hold", `echo $$ > "$HOME/command-pid"; exec sleep 30`)
		run := s.start([]string{"hold"}, "", nil)
		until(t, func() bool { return exists(filepath.Join(s.tmp, "command-pid")) })
		if e := run.cmd.Process.Signal(syscall.SIGTERM); e != nil {
			t.Fatal(e)
		}
		select {
		case <-run.done:
		case <-time.After(2 * time.Second):
			t.Fatal("command survived ask signal")
		}
	})
	t.Run("a command runs as ask NAME with its arguments and exit code, and can run ask itself", func(t *testing.T) {
		s := fresh(t)
		s.script("commands", "twice", "# ask-command: ask the same question twice\n\"$ASK_BIN\" -m fake:small \"$@\" 2>/dev/null\n\"$ASK_BIN\" -m fake:small \"$@\" 2>/dev/null\nexit 3")
		r := s.ask("twice", "hello")
		eq(t, r.code, 3)
		eq(t, r.stdout, "fake: hello\nfake: hello\n")
		match(t, s.ask("--help").stdout, `Your commands\n {2}twice +ask the same question twice`)
	})
	t.Run("ask's own commands win over yours of the same name", func(t *testing.T) {
		s := fresh(t)
		s.script("commands", "models", "echo mine")
		match(t, s.ask("models").stdout, `(?m)^fake:small$`)
	})
}

package test

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// TestSubagent pins follow-ups, git changes, worktrees, saved results and locks.
func TestSubagent(t *testing.T) {
	t.Run("-c continues the agent session where it ran, with the same model", func(t *testing.T) {
		s := fresh(t)
		work := filepath.Join(s.tmp, "work")
		s.mkdir(work)
		first := s.ask("-m", "fake:big#high", "-C", work, "remember PELICAN")
		second := s.ask("-c", runID(t, first.stderr), "what word?")
		eq(t, second.code, 0)
		a, b := s.calls()[0], s.calls()[1]
		eq(t, b.s("session"), fmt.Sprintf("s-%.0f", a.n("pid")))
		eq(t, b.s("cwd"), work)
		eq(t, b.s("model"), "big")
		eq(t, b.s("effort"), "high")
		eq(t, b.s("stdin"), "what word?")
		match(t, second.stderr, `follow-up `+runID(t, first.stderr))
	})
	t.Run("a follow-up keeps write access unless it says -r, and may change the model within the agent", func(t *testing.T) {
		s := fresh(t)
		first := s.ask("-m", "fake:big", "-w", "change it")
		id := runID(t, first.stderr)
		s.ask("-c", id, "more")
		s.ask("-c", id, "-r", "-m", "fake:small", "look")
		eq(t, fields(s.calls(), "access", "model"), [][]any{{"write", "big"}, {"write", "big"}, {"read", "small"}})
	})
	t.Run("a follow-up to another agent, or to a run with no session, is refused", func(t *testing.T) {
		s := fresh(t)
		s.script("agents", "other", "echo hi")
		fake := s.ask("-m", "fake:small", "hi")
		other := s.ask("-m", "other:x", "hi")
		switched := s.ask("-c", runID(t, fake.stderr), "-m", "other:x", "more")
		eq(t, switched.code, 2)
		match(t, switched.stderr, `must use the same agent`)
		sessionless := s.ask("-c", runID(t, other.stderr), "more")
		eq(t, sessionless.code, 2)
		match(t, sessionless.stderr, `reported no session`)
	})
	t.Run("a run that timed out can still be continued", func(t *testing.T) {
		s := fresh(t)
		first := s.run([]string{"-m", "fake:small", "-t", "0.5", "hang"}, "", map[string]string{"FAKE_HANG": "hang"})
		match(t, first.stderr, `timed out`)
		second := s.ask("-c", runID(t, first.stderr), "finish")
		eq(t, second.code, 0)
		eq(t, s.calls()[1].s("session"), fmt.Sprintf("s-%.0f", s.calls()[0].n("pid")))
	})
	t.Run("a batch task is continued as RUN/TASK, from a follow-up or another batch", func(t *testing.T) {
		s := fresh(t)
		batch := s.run([]string{"batch", "-m", "fake:small", "-"}, `[{"id":"a","prompt":"x"},{"id":"b","prompt":"y"}]`, nil)
		id := runID(t, batch.stderr)
		bare := s.ask("-c", id, "more")
		eq(t, bare.code, 2)
		match(t, bare.stderr, `continue one of them, like `+id+`/a`)
		s.ask("-c", id+"/b", "more")
		input, _ := json.Marshal([]object{{"continue": id + "/a", "prompt": "again"}})
		s.run([]string{"batch", "-"}, string(input), nil)
		by := func(prompt string) object {
			for _, c := range s.calls() {
				if strings.HasSuffix(c.s("stdin"), prompt) {
					return c
				}
			}
			t.Fatalf("no call ending %q", prompt)
			return nil
		}
		eq(t, by("more").s("session"), fmt.Sprintf("s-%.0f", by("y").n("pid")))
		eq(t, by("again").s("session"), fmt.Sprintf("s-%.0f", by("x").n("pid")))
	})
	t.Run("a write run reports the files it changed, not ones already changed before it", func(t *testing.T) {
		s := fresh(t)
		dir, _ := s.repo()
		s.write(filepath.Join(dir, "a.txt"), "edited before\n")
		s.write(filepath.Join(dir, "untouched.txt"), "new before\n")
		r := s.run([]string{"batch", "-w", "-m", "fake:small", "-C", dir, "-"}, `[{"prompt":"go"}]`, map[string]string{"FAKE_WRITE": "b.txt=new"})
		result := objects(t, r.stdout)[0]
		jsonEqual(t, result["changes"], []object{{"path": "b.txt", "change": "added"}})
		eq(t, result.n("commits"), 0.0)
		match(t, r.stderr, ` · ok · Fake 1\.0 · [\d.]+s · 1 file changed · `)
	})
	t.Run("a write run that commits reports its commits and the files they changed", func(t *testing.T) {
		s := fresh(t)
		dir, _ := s.repo()
		s.write(filepath.Join(dir, "a.txt"), "two\n")
		r := s.run([]string{"batch", "-w", "-m", "fake:small", "-C", dir, "-"}, `[{"prompt":"go"}]`, map[string]string{"FAKE_COMMIT": "1"})
		result := objects(t, r.stdout)[0]
		jsonEqual(t, result["changes"], []object{})
		eq(t, result.n("commits"), 1.0)
		edits := s.run([]string{"batch", "-w", "-m", "fake:small", "-C", dir, "-"}, `[{"prompt":"go"}]`, map[string]string{"FAKE_WRITE": "a.txt=three", "FAKE_COMMIT": "1"})
		jsonEqual(t, objects(t, edits.stdout)[0]["changes"], []object{{"path": "a.txt", "change": "modified"}})
	})
	t.Run("--worktree runs in a new worktree and branch, kept with the changes, leaving the checkout alone", func(t *testing.T) {
		s := fresh(t)
		dir, git := s.repo()
		r := s.run([]string{"-m", "fake:small", "-w", "--worktree", "-C", dir, "go"}, "", map[string]string{"FAKE_WRITE": "b.txt=new"})
		eq(t, r.code, 0)
		id := runID(t, r.stderr)
		path := filepath.Join(s.home, "worktrees", id)
		eq(t, s.calls()[0].s("cwd"), path)
		eq(t, s.read(filepath.Join(path, "b.txt")), "new")
		eq(t, exists(filepath.Join(dir, "b.txt")), false)
		match(t, git("branch", "--list", "ask/"+id), `ask/`+id)
		match(t, r.stderr, `branch ask/`+id)
	})
	t.Run("a worktree run that changed nothing removes its worktree and branch, and a follow-up recreates it", func(t *testing.T) {
		s := fresh(t)
		dir, git := s.repo()
		r := s.ask("-m", "fake:small", "-w", "--worktree", "-C", dir, "look")
		id := runID(t, r.stderr)
		eq(t, exists(filepath.Join(s.home, "worktrees", id)), false)
		eq(t, strings.TrimSpace(git("branch", "--list", "ask/"+id)), "")
		s.run([]string{"-c", id, "now change it"}, "", map[string]string{"FAKE_WRITE": "c.txt=x"})
		eq(t, s.calls()[1].s("cwd"), filepath.Join(s.home, "worktrees", id))
		match(t, git("branch", "--list", "ask/"+id), `ask/`+id)
	})
	t.Run("--worktree needs -w and a git repository", func(t *testing.T) {
		s := fresh(t)
		read := s.ask("-m", "fake:small", "--worktree", "go")
		eq(t, read.code, 2)
		match(t, read.stderr, `a worktree is for write runs; add -w`)
		outside := s.ask("-m", "fake:small", "-w", "--worktree", "-C", s.tmp, "go")
		eq(t, outside.code, 1)
		match(t, outside.stderr, `--worktree needs a git repository`)
	})
	t.Run("show prints a run again: its answer, or with --json its whole result", func(t *testing.T) {
		s := fresh(t)
		first := s.ask("-m", "fake:small", "hello")
		id := runID(t, first.stderr)
		shown := s.ask("show", id)
		eq(t, shown.stdout, "fake: hello\n")
		match(t, shown.stderr, `^ask `+id+` · ok · Fake 1\.0`)
		full := obj(t, s.ask("show", id, "--json").stdout)
		eq(t, full.s("run"), id)
		eq(t, full.s("session"), fmt.Sprintf("s-%.0f", s.calls()[0].n("pid")))
		batch := s.run([]string{"batch", "-m", "fake:small", "-"}, `[{"prompt":"a"},{"prompt":"b"}]`, nil)
		batchID := runID(t, batch.stderr)
		shownBatch := s.ask("show", batchID)
		eq(t, shownBatch.stderr, "ask "+batchID+" · 2/2 ok\n")
	})
	t.Run("stop stops a run that is going, and its agents", func(t *testing.T) {
		s := fresh(t)
		run := s.start([]string{"-m", "fake:small", "hang"}, "", map[string]string{"FAKE_HANG": "hang"})
		until(t, func() bool { return len(s.calls()) == 1 })
		parts := strings.Split(s.latest(), "-")
		id := parts[len(parts)-1]
		stopped := s.ask("stop", id)
		match(t, stopped.stderr, `^ask `+id+` · stopped`)
		eq(t, run.wait(t).code, 130)
		eq(t, s.ask("stop", id).code, 1)
	})
	t.Run("a run another ask is running cannot be resumed at the same time", func(t *testing.T) {
		s := fresh(t)
		run := s.start([]string{"batch", "-m", "fake:small", "-"}, `[{"prompt":"hang"}]`, map[string]string{"FAKE_HANG": "hang"})
		until(t, func() bool { return len(s.calls()) == 1 })
		parts := strings.Split(s.latest(), "-")
		id := parts[len(parts)-1]
		second := s.ask("batch", "--resume", id)
		eq(t, second.code, 2)
		match(t, second.stderr, `run `+id+` is running`)
		if e := run.cmd.Process.Signal(syscall.SIGTERM); e != nil {
			t.Fatal(e)
		}
		run.wait(t)
	})
	t.Run("a damaged results.json is reported, never taken as no results", func(t *testing.T) {
		s := fresh(t)
		first := s.run([]string{"batch", "-m", "fake:small", "-"}, `[{"prompt":"a"}]`, nil)
		id := runID(t, first.stderr)
		s.write(filepath.Join(s.home, "runs", s.latest(), "results.json"), `[{"ok": tr`)
		r := s.ask("batch", "--resume", id)
		eq(t, r.code, 2)
		match(t, r.stderr, `results\.json is damaged`)
		eq(t, len(s.calls()), 1)
	})
	t.Run("--resume reruns a batch as recorded, refusing options that would change it", func(t *testing.T) {
		s := fresh(t)
		first := s.run([]string{"batch", "-m", "fake:small", "-"}, `[{"prompt":"a"}]`, nil)
		r := s.ask("batch", "--resume", runID(t, first.stderr), "-r")
		eq(t, r.code, 2)
		match(t, r.stderr, `takes only -j and --no-hooks, not -r`)
	})
	t.Run("an agent inherits no contract variable from an ask further up", func(t *testing.T) {
		s := fresh(t)
		path := filepath.Join(s.tmp, "s.json")
		s.json(path, object{"type": "object"})
		s.run([]string{"-m", "fake:small", "hi"}, "", map[string]string{"ASK_SCHEMA": path, "ASK_SESSION": "leaked"})
		_, hasSchema := s.calls()[0]["schema"]
		_, hasSession := s.calls()[0]["session"]
		eq(t, hasSchema, false)
		eq(t, hasSession, false)
	})
}

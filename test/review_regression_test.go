package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestCallerStdinRemainsUnread keeps argument prompts from consuming the next shell-loop line.
func TestCallerStdinRemainsUnread(t *testing.T) {
	s := fresh(t)
	cmd := exec.Command("sh", "-c", `printf 'a\nb\n' | while read -r p; do "$ASK_TEST_BIN" -m fake:small "$p"; done`)
	cmd.Env = append(os.Environ(), "ASK_TEST_BIN="+askBin, "ASK_HOME="+s.home, "HOME="+s.tmp, "FAKE_LOG="+s.log)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err, string(out))
	}
	eq(t, string(out), "fake: a\nfake: b\n")
}

// TestWorktreeKeepsMetadataChanges protects a mode-only edit and a dangling symlink.
func TestWorktreeKeepsMetadataChanges(t *testing.T) {
	for _, tc := range []struct{ name, body, path, change string }{{"mode", `chmod +x a.txt`, "a.txt", "modified"}, {"symlink", `ln -s missing dangling`, "dangling", "added"}} {
		t.Run(tc.name, func(t *testing.T) {
			s := fresh(t)
			dir, _ := s.repo()
			s.localAgent("edit", `cat >/dev/null; `+tc.body+`; echo done`)
			r := s.ask("-m", "edit:small", "-w", "--worktree", "-C", dir, "go")
			eq(t, r.code, 0)
			result := obj(t, s.ask("show", runID(t, r.stderr), "--json").stdout)
			if result["worktree"] == nil {
				t.Fatal("worktree removed")
			}
			found := false
			for _, v := range result["changes"].([]any) {
				x := v.(map[string]any)
				if x["path"] == tc.path && x["change"] == tc.change {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing %s %s: %v", tc.path, tc.change, result["changes"])
			}
		})
	}
}

// TestWorktreeSymlinkedDirectory keeps the agent inside the corresponding subdirectory.
func TestWorktreeSymlinkedDirectory(t *testing.T) {
	s := fresh(t)
	dir, _ := s.repo()
	s.write(filepath.Join(dir, "sub", "keep"), "x")
	s.gitAt(dir)("add", ".")
	s.gitAt(dir)("commit", "-qm", "sub")
	link := filepath.Join(s.tmp, "alias")
	if e := os.Symlink(dir, link); e != nil {
		t.Fatal(e)
	}
	r := s.ask("-m", "fake:small", "-w", "--worktree", "-C", filepath.Join(link, "sub"), "go")
	eq(t, r.code, 0)
	eq(t, s.calls()[0].s("cwd"), filepath.Join(s.home, "worktrees", runID(t, r.stderr), "sub"))
}

// TestPrivateRunState requires new run records and directories to be private.
func TestPrivateRunState(t *testing.T) {
	s := fresh(t)
	r := s.ask("-m", "fake:small", "go")
	eq(t, r.code, 0)
	dir := filepath.Join(s.home, "runs", s.latest())
	for _, p := range []string{filepath.Join(s.home, "runs"), dir, filepath.Join(dir, "tasks.json"), filepath.Join(dir, "results.json")} {
		info, e := os.Stat(p)
		if e != nil {
			t.Fatal(e)
		}
		if info.Mode().Perm()&0077 != 0 {
			t.Errorf("%s mode %o", p, info.Mode().Perm())
		}
	}
}

// TestBatchWorktreeNamesUsePosition keeps sanitized task ids from sharing a branch.
func TestBatchWorktreeNamesUsePosition(t *testing.T) {
	s := fresh(t)
	dir, _ := s.repo()
	s.localAgent("edit", `cat >/dev/null; echo "$ASK_MODEL" > "$ASK_MODEL.txt"; echo done`)
	batch := `[{"id":"a/b","prompt":"one"},{"id":"a_b","prompt":"two"}]`
	r := s.run([]string{"batch", "-m", "edit:small", "-w", "--worktree", "-C", dir, "-"}, batch, nil)
	eq(t, r.code, 0)
	results := objects(t, r.stdout)
	a := results[0]["worktree"].(map[string]any)
	b := results[1]["worktree"].(map[string]any)
	if a["path"] == b["path"] || a["branch"] == b["branch"] {
		t.Fatalf("shared worktree: %v %v", a, b)
	}
}

// TestStoppedTaskHasEmptyResult lets a stopped run resume without a failure result, result hook or printed results.
func TestStoppedTaskHasEmptyResult(t *testing.T) {
	s := fresh(t)
	s.hook("observe", []string{"result"}, `echo '{}'`)
	run := s.start([]string{"batch", "-m", "fake:small", "-"}, `[{"prompt":"hang"}]`, map[string]string{"FAKE_HANG": "hang"})
	until(t, func() bool { return len(s.calls()) == 1 })
	id := strings.TrimPrefix(s.latest()[strings.LastIndex(s.latest(), "-"):], "-")
	eq(t, s.ask("stop", id).code, 0)
	out := run.wait(t)
	eq(t, out.code, 130)
	eq(t, out.stdout, "")
	path := filepath.Join(s.home, "runs", s.latest(), "results.json")
	if exists(path) && strings.Contains(s.read(path), `"ok": false`) {
		t.Fatal("stopped task saved as failed")
	}
	if len(s.hookCalls()) != 0 {
		t.Fatalf("result hook called: %v", s.hookCalls())
	}
	match(t, s.ask("runs").stdout, `stopped.*resume: ask batch --resume `+id)
}

// TestHookRoutedFollowup inherits the agent and access that actually produced the session.
func TestHookRoutedFollowup(t *testing.T) {
	s := fresh(t)
	s.localAgent("other", `cat >/dev/null; echo '{"session":"other-session"}' > "$ASK_REPORT"; echo fine`)
	s.hook("route", []string{"task"}, `if [ ! -e "$HOME/routed" ]; then touch "$HOME/routed"; echo '{"task":{"model":"other:small","write":true}}'; fi`)
	r := s.ask("-m", "fake:small", "go")
	eq(t, r.code, 0)
	follow := s.ask("-c", runID(t, r.stderr), "again")
	eq(t, follow.code, 0)
	result := obj(t, s.ask("show", runID(t, follow.stderr), "--json").stdout)
	eq(t, result.s("model"), "other:small")
	eq(t, s.calls(), []object(nil))
}

// TestReadFollowupReusesWorktree allows read access when the prior worktree is kept.
func TestReadFollowupReusesWorktree(t *testing.T) {
	s := fresh(t)
	dir, _ := s.repo()
	first := s.run([]string{"-m", "fake:small", "-w", "--worktree", "-C", dir, "go"}, "", map[string]string{"FAKE_WRITE": "b.txt=new"})
	eq(t, first.code, 0)
	r := s.ask("-c", runID(t, first.stderr), "-r", "inspect")
	eq(t, r.code, 0)
	eq(t, s.calls()[1].s("access"), "read")
	eq(t, s.calls()[1].s("cwd"), filepath.Join(s.home, "worktrees", runID(t, first.stderr)))
}

// TestTitleKeepsLeadingRun matches ask's prompt parsing for an explicit run word.
func TestTitleKeepsLeadingRun(t *testing.T) {
	s := fresh(t)
	eq(t, s.ask("title", "--command", `ask run -m fake:small fix`).stdout, "Small One · Run fix\n")
}

// TestTitleBatchPathRelativeToInvocation resolves a batch file before applying -C.
func TestTitleBatchPathRelativeToInvocation(t *testing.T) {
	s := fresh(t)
	s.mkdir(filepath.Join(s.tmp, "other"))
	s.write(filepath.Join(s.tmp, "jobs.json"), `[{"prompt":"fix","model":"fake:small"}]`)
	eq(t, s.ask("title", "--command", `ask batch -C other jobs.json`).stdout, "Batch of 1 · Small One · Fix\n")
}

// TestShowFailedBatch exits with failure when any saved task failed.
func TestShowFailedBatch(t *testing.T) {
	s := fresh(t)
	r := s.run([]string{"batch", "-m", "fake:small", "-"}, `[{"prompt":"pass"},{"prompt":"fail"}]`, map[string]string{"FAKE_FAIL": "fail"})
	eq(t, r.code, 1)
	eq(t, s.ask("show", runID(t, r.stderr)).code, 1)
}

// TestEmptyTasksRunIsUnreadable skips an empty task record in recent runs.
func TestEmptyTasksRunIsUnreadable(t *testing.T) {
	s := fresh(t)
	s.write(filepath.Join(s.home, "runs", "20260101T000000000-zzzzzz", "tasks.json"), `[]`)
	r := s.ask("runs")
	eq(t, r.code, 0)
	noMatch(t, r.stdout, `zzzzzz`)
}

// TestUserCommandReceivesSignal directly requires a command to replace ask's process.
func TestUserCommandReceivesSignal(t *testing.T) {
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
}

// TestSignalFailureNamesSignal reports a killed agent without an exit-null message.
func TestSignalFailureNamesSignal(t *testing.T) {
	s := fresh(t)
	s.localAgent("killed", `kill -TERM $$`)
	r := s.ask("-m", "killed:small", "go")
	eq(t, r.code, 1)
	match(t, r.stderr, `killed by SIGTERM`)
	noMatch(t, r.stderr, `exit null`)
}

// TestHookTimeoutIsClamped keeps a large accepted hook timeout from expiring immediately.
func TestHookTimeoutIsClamped(t *testing.T) {
	s := fresh(t)
	s.hook("long", []string{"task"}, `echo '{"task":{"timeout":3000000}}'`)
	s.localAgent("slow", `cat >/dev/null; sleep 0.05; echo done`)
	r := s.ask("-m", "slow:small", "go")
	eq(t, r.code, 0)
	noMatch(t, r.stderr, `timed out`)
}

// TestPlainStatusStripsControls removes agent and hook controls from pipe status lines.
func TestPlainStatusStripsControls(t *testing.T) {
	s := fresh(t)
	s.localAgent("noisy", `printf '{"name":"Bad\\u001b[31mName"}' > "$ASK_REPORT"; echo done`)
	s.hook("note", []string{"task"}, `printf '{"note":"hello\\u001b[31mred"}'`)
	r := s.ask("-m", "noisy:small", "go")
	eq(t, r.code, 0)
	noMatch(t, r.stderr, `\x1b`)
}

// TestWorktreeNewlinePath reports a changed filename containing a newline.
func TestWorktreeNewlinePath(t *testing.T) {
	s := fresh(t)
	dir, _ := s.repo()
	s.localAgent("edit", `cat >/dev/null; printf changed > 'line
break'; echo done`)
	r := s.ask("-m", "edit:small", "-w", "--worktree", "-C", dir, "go")
	eq(t, r.code, 0)
	result := obj(t, s.ask("show", runID(t, r.stderr), "--json").stdout)
	changes := result["changes"].([]any)
	eq(t, len(changes), 1)
	eq(t, changes[0].(map[string]any)["path"], "line\nbreak")
}

// TestStopIgnoresStalePid never signals an unrelated process named by an unlocked record.
func TestStopIgnoresStalePid(t *testing.T) {
	s := fresh(t)
	first := s.ask("-m", "fake:small", "go")
	eq(t, first.code, 0)
	cmd := exec.Command("sleep", "30")
	if e := cmd.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() { cmd.Process.Kill(); cmd.Wait() }()
	lock := filepath.Join(s.home, "runs", s.latest(), "lock")
	s.write(lock, strconv.Itoa(cmd.Process.Pid))
	r := s.ask("stop", runID(t, first.stderr))
	eq(t, r.code, 1)
	if !alive(cmd.Process.Pid) {
		t.Fatal("unrelated process was signaled")
	}
}

// TestReadFollowupCannotCreateWorktree requires write access if the prior tree was removed.
func TestReadFollowupCannotCreateWorktree(t *testing.T) {
	s := fresh(t)
	dir, _ := s.repo()
	first := s.ask("-m", "fake:small", "-w", "--worktree", "-C", dir, "go")
	eq(t, first.code, 0)
	r := s.ask("-c", runID(t, first.stderr), "-r", "inspect")
	eq(t, r.code, 2)
	match(t, r.stderr, `a worktree is for write runs`)
}

// TestHookSelectedReadAccessCarriesForward saves a false access value distinctly from old records.
func TestHookSelectedReadAccessCarriesForward(t *testing.T) {
	s := fresh(t)
	s.hook("route", []string{"task"}, `if [ ! -e "$HOME/routed" ]; then touch "$HOME/routed"; echo '{"task":{"write":false}}'; fi`)
	first := s.ask("-m", "fake:small", "-w", "go")
	eq(t, first.code, 0)
	follow := s.ask("-c", runID(t, first.stderr), "again")
	eq(t, follow.code, 0)
	eq(t, s.calls()[1].s("access"), "read")
}

// TestPrivateWorktreeParent keeps new isolated trees under a private state directory.
func TestPrivateWorktreeParent(t *testing.T) {
	s := fresh(t)
	dir, _ := s.repo()
	r := s.ask("-m", "fake:small", "-w", "--worktree", "-C", dir, "go")
	eq(t, r.code, 0)
	info, e := os.Stat(filepath.Join(s.home, "worktrees"))
	if e != nil {
		t.Fatal(e)
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Fatalf("worktrees parent mode %o", info.Mode().Perm())
	}
}

// TestRunLockIsKernelExclusive refuses a second writer while an OS lock is held, naming the
// holder the kernel reports rather than whatever the lock file contains.
func TestRunLockIsKernelExclusive(t *testing.T) {
	s := fresh(t)
	first := s.run([]string{"batch", "-m", "fake:small", "-"}, `[{"prompt":"fail"}]`, map[string]string{"FAKE_FAIL": "fail"})
	eq(t, first.code, 1)
	path := filepath.Join(s.home, "runs", s.latest(), "lock")
	f, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	if e := f.Truncate(0); e != nil {
		t.Fatal(e)
	}
	if e := syscall.FcntlFlock(f.Fd(), syscall.F_SETLK, &syscall.Flock_t{Type: syscall.F_WRLCK}); e != nil {
		t.Fatal(e)
	}
	r := s.ask("batch", "--resume", runID(t, first.stderr))
	eq(t, r.code, 2)
	match(t, r.stderr, `is running \(pid `+strconv.Itoa(os.Getpid())+`\)`)
}

// TestFollowupCannotMoveIntoWorktree refuses --worktree for a follow-up of a run in the checkout.
func TestFollowupCannotMoveIntoWorktree(t *testing.T) {
	s := fresh(t)
	dir, _ := s.repo()
	first := s.ask("-m", "fake:small", "-C", dir, "look")
	r := s.run([]string{"-c", runID(t, first.stderr), "-w", "--worktree", "edit"}, "", map[string]string{"FAKE_WRITE": "a.txt=changed\n"})
	eq(t, r.code, 2)
	match(t, r.stderr, `not a worktree; drop --worktree`)
	eq(t, s.read(filepath.Join(dir, "a.txt")), "one\n")
}

// TestDottedTaskIDWorktree turns task ids git refuses in branch names, like v1..v2, into valid ones.
func TestDottedTaskIDWorktree(t *testing.T) {
	s := fresh(t)
	dir, _ := s.repo()
	r := s.run([]string{"batch", "-m", "fake:small", "-w", "--worktree", "-C", dir, "-"}, `[{"id":"v1..v2","prompt":"a"},{"id":"api.lock","prompt":"b"}]`, nil)
	eq(t, r.code, 0)
}

// TestContinueByDirectory continues a single-task run named by its folder.
func TestContinueByDirectory(t *testing.T) {
	s := fresh(t)
	s.ask("-m", "fake:small", "hello")
	eq(t, s.ask("-c", filepath.Join(s.home, "runs", s.latest()), "more").code, 0)
}

// TestContinueUnfinished says a run without a result has not finished, not that it lacks a session.
func TestContinueUnfinished(t *testing.T) {
	s := fresh(t)
	s.json(filepath.Join(s.home, "runs", "20260101T000000000-abc123-hung", "tasks.json"), []object{{"id": "1", "prompt": "x", "model": "fake:small", "write": false, "json": false, "dir": s.tmp, "timeout": 900}})
	r := s.ask("-c", "hung", "more")
	eq(t, r.code, 2)
	match(t, r.stderr, `hung cannot be continued: it has not finished`)
}

// TestWriteRunCannotPlantFsmonitor keeps ask's own git calls from running a core.fsmonitor an agent set.
func TestWriteRunCannotPlantFsmonitor(t *testing.T) {
	s := fresh(t)
	dir, _ := s.repo()
	planted := filepath.Join(s.tmp, "planted")
	s.localAgent("plant", `cat >/dev/null; git config core.fsmonitor "touch `+planted+`; false"; echo two >> a.txt; echo fine`)
	r := s.ask("-m", "plant:small", "-w", "-C", dir, "go")
	eq(t, r.code, 0)
	eq(t, exists(planted), false)
}

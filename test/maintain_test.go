package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWait pins ask wait: it returns what ask show would once runs finish, and gives up on -t.
func TestWait(t *testing.T) {
	t.Run("waits for a running run, then prints its answer; several runs print a JSON array", func(t *testing.T) {
		s := fresh(t)
		slow := s.start([]string{"-m", "fake:small", "slow one"}, "", map[string]string{"FAKE_SLOW": "slow"})
		until(t, func() bool { return exists(filepath.Join(s.home, "runs")) && len(s.dirs()) == 1 })
		r := s.ask("wait", "slow-one")
		eq(t, r.code, 0)
		eq(t, r.stdout, "fake: slow one\n")
		slow.wait(t)
		s.ask("-m", "fake:small", "second")
		both := s.ask("wait", "slow-one", "second")
		eq(t, both.code, 0)
		eq(t, len(objects(t, both.stdout)), 2)
	})
	t.Run("-t gives up on a run still going", func(t *testing.T) {
		s := fresh(t)
		s.start([]string{"-m", "fake:small", "hang here"}, "", map[string]string{"FAKE_HANG": "hang"})
		until(t, func() bool { return exists(filepath.Join(s.home, "runs")) && len(s.dirs()) == 1 })
		r := s.ask("wait", "hang", "-t", "0.3")
		eq(t, r.code, 1)
		match(t, r.stderr, `ask hang · still running after 0.3s`)
	})
}

// TestClean pins ask clean: landed worktrees and old runs go, anything else stays.
func TestClean(t *testing.T) {
	s := fresh(t)
	dir, git := s.repo()
	s.run([]string{"-m", "fake:small", "-w", "--worktree", "-C", dir, "change readme"}, "", map[string]string{"FAKE_WRITE": "b.txt=new"})
	path := filepath.Join(s.home, "worktrees", "change-readme")
	gitAt := s.gitAt(path)
	gitAt("add", ".")
	gitAt("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "change")
	plan := s.ask("clean", "--dry-run")
	match(t, plan.stdout, `keep +~/home/worktrees/change-readme +ask/change-readme · not merged into main`)
	git("-c", "user.email=t@t", "-c", "user.name=t", "merge", "-q", "--squash", "ask/change-readme")
	git("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "squashed")
	old := filepath.Join(s.home, "runs", "20200101T000000000-abc123-old")
	s.mkdir(old)
	s.write(filepath.Join(old, "tasks.json"), `[{"id":"1","prompt":"x","model":"fake:small","write":false,"json":false,"dir":"/","timeout":900}]`)
	r := s.ask("clean", "--yes")
	eq(t, r.code, 0)
	match(t, r.stderr, `removed ~/home/worktrees/change-readme and ask/change-readme`)
	match(t, r.stderr, `removed 1 run\n`)
	eq(t, exists(path), false)
	eq(t, exists(old), false)
	eq(t, strings.TrimSpace(git("branch", "--list", "ask/change-readme")), "")
	eq(t, len(s.dirs()), 1)
	if _, e := os.Stat(filepath.Join(dir, "b.txt")); e != nil {
		t.Fatal("the merged change is gone from the checkout")
	}
}

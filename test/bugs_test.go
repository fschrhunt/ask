package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestFalseSchema protects boolean schemas at the task and array-item boundaries.
func TestFalseSchema(t *testing.T) {
	for _, schema := range []any{false, object{"items": false}} {
		s := fresh(t)
		path := filepath.Join(s.tmp, "schema.json")
		s.json(path, schema)
		r := s.run([]string{"-m", "fake:small", "--schema", path, "go"}, "", map[string]string{"FAKE_ANSWER": "[1]"})
		eq(t, r.code, 1)
		match(t, r.stderr, `no value is allowed here`)
		jsonEqual(t, s.calls()[0]["schema"], schema)
	}
}

// TestWorktreeFollowup protects changes and commits from earlier rounds when a later round is idle.
func TestWorktreeFollowup(t *testing.T) {
	for _, commit := range []bool{false, true} {
		s := fresh(t)
		dir, _ := s.repo()
		s.localAgent("edit", `cat >/dev/null
if [ -z "$ASK_SESSION" ]; then echo changed > a.txt; [ "$COMMIT" != yes ] || git commit -qam changed; fi
echo '{"session":"same"}' > "$ASK_REPORT"
echo fine`)
		s.hook("again", []string{"result"}, `case "$input" in *'"followups"'*) ;; *) echo '{"followup":"look again"}';; esac`)
		env := map[string]string{}
		if commit {
			env["COMMIT"] = "yes"
		}
		r := s.run([]string{"-m", "edit:small", "-w", "--worktree", "-C", dir, "go"}, "", env)
		eq(t, r.code, 0)
		path := filepath.Join(s.home, "worktrees", runID(t, r.stderr))
		eq(t, s.read(filepath.Join(path, "a.txt")), "changed\n")
		eq(t, s.ask("-c", runID(t, r.stderr), "look once more").code, 0)
		eq(t, exists(path), true)
	}
}

// TestDescendantPipes requires completion after the agent exits even if a child retains stdout.
func TestDescendantPipes(t *testing.T) {
	s := fresh(t)
	s.localAgent("fork", `cat >/dev/null; sleep 30 & echo fine`)
	start := time.Now()
	r := s.ask("-m", "fork:small", "-t", "2", "go")
	eq(t, r.code, 0)
	eq(t, r.stdout, "fine\n")
	if time.Since(start) >= 2*time.Second {
		t.Fatal("waited for descendant pipes")
	}
}

// TestHookStartFailure keeps invalid executables fail-open during discovery and invocation.
func TestHookStartFailure(t *testing.T) {
	s := fresh(t)
	path := filepath.Join(s.home, "hooks", "broken")
	s.write(path, "not an executable format\n")
	if err := os.Chmod(path, 0755); err != nil {
		t.Fatal(err)
	}
	s.hook("vanish", []string{"task"}, `echo '{"task":{"write":true}}'`)
	// The declaration succeeds but the event's interpreter cannot start.
	s.script("hooks", "vanish", `if [ "$1" = events ]; then echo task; printf '#!/no/such/interpreter\n' > "$0"; exit 0; fi`)
	r := s.ask("-m", "fake:small", "go")
	eq(t, r.code, 0)
	eq(t, s.calls()[0].s("access"), "read")
	match(t, r.stderr, `hook broken · failed:`)
	match(t, r.stderr, `hook vanish · failed:`)
}

// TestHookStartStatus requires the status to describe the task after hooks route it.
func TestHookStartStatus(t *testing.T) {
	s := fresh(t)
	s.hook("route", []string{"task"}, `echo '{"task":{"model":"fake:big","write":true}}'`)
	r := s.ask("-m", "fake:small", "go")
	match(t, r.stderr, ` · started · Big · write · `)
}

// TestSurplusArguments rejects extra operands of built-in commands before doing any work.
func TestSurplusArguments(t *testing.T) {
	for _, command := range []string{"models", "runs", "packages", "show", "stop", "install", "remove"} {
		s := fresh(t)
		args := []string{command, "extra"}
		if !strings.Contains("models runs packages", command) {
			args = append(args, "another")
		}
		eq(t, s.ask(args...).code, 2)
	}
}

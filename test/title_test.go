package test

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestTitle describes each task form without starting its agent or recording a run.
func TestTitle(t *testing.T) {
	s := fresh(t)
	for _, c := range []struct{ command, description, want string }{
		{`cd /tmp && A=value ask -m fake:small#high -w "fix login" 2>&1 | tail -20`, "", "Small One (high) · Fix login · write\n"},
		{`ask -m fake:big -w --worktree 'fix it'`, "  add a test  ", "Big · Add a test · worktree\n"},
		{`ask -m fake:small 'first line
second line'`, "", "Small One · First line\n"},
		{`ask -m fake:small '` + strings.Repeat("x", 70) + `'`, "", "Small One · X" + strings.Repeat("x", 58) + "…\n"},
		{`ask batch -m fake:small -`, "check tests", "Batch · Check tests\n"},
		{`ask batch --resume abc123`, "finish the batch", "Resume abc123 · Finish the batch\n"},
	} {
		r := s.ask("title", "--command", c.command, "--description", c.description)
		eq(t, r.code, 0)
		eq(t, r.stdout, c.want)
	}
	eq(t, len(s.calls()), 0)
	path := filepath.Join(s.tmp, "tasks.json")
	s.json(path, []object{{"prompt": "check the API"}, {"prompt": "check tests", "model": "fake:big"}, {"prompt": "review"}})
	eq(t, s.ask("title", "--command", "ask batch -m fake:small "+path).stdout, "Batch of 3 · Small One, Big · Check the API\n")
	first := s.ask("-m", "fake:small#high", "-w", "remember")
	id := runID(t, first.stderr)
	eq(t, s.ask("title", "--command", "ask -c "+id+" 'go again'").stdout, "Small One (high) · Go again · write\n")
	eq(t, s.ask("title", "--command", "ask -c "+id+" -m fake:big -r 'look'").stdout, "Big · Look\n")
}

// TestTitleNonRuns leaves host input alone for commands that do not start tasks or cannot be parsed.
func TestTitleNonRuns(t *testing.T) {
	s := fresh(t)
	s.script("commands", "review", "echo fine")
	for _, command := range []string{"echo ask -m fake:small hi", "ask", "ask --help", "ask -m fake:small --help hi", "ask review", `ask -m fake:small "bad`, "ask -m fake:small hi |", "ask title --command hi", "ask help batch", "ask show x", "ask runs", "ask models", "ask stop x", "ask install", "ask packages", "ask remove x"} {
		r := s.ask("title", "--command", command)
		eq(t, r.code, 0)
		eq(t, r.stdout, "")
	}
	eq(t, s.ask("title").code, 2)
	eq(t, s.ask("title", "--command", "ask", "extra").code, 2)
	eq(t, len(s.calls()), 0)
}

// TestTitleHook passes the parsed invocation to declared hooks and honors --no-hooks.
func TestTitleHook(t *testing.T) {
	s := fresh(t)
	s.hook("host", []string{"title"}, `echo '{"title":"A better title"}'`)
	r := s.ask("title", "--command", `A=1 ask -m fake:small 'go now' 2>&1`, "--description", "do it")
	eq(t, r.stdout, "A better title\n")
	input := s.hookCalls()[0]["input"].(map[string]any)
	jsonEqual(t, input, object{"title": "Small One · Do it", "command": []string{"ask", "-m", "fake:small", "go now"}, "description": "do it"})
	eq(t, s.ask("title", "--command", "ask -m fake:small --no-hooks hi").stdout, "Small One · Hi\n")
	eq(t, len(s.hookCalls()), 1)
}

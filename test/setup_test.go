package test

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestSetup pins what makes ask easy to set up and adapt: official agent names, readiness checks,
// settings, runs per repository and the host title hook.
func TestSetup(t *testing.T) {
	t.Run("ask install NAME installs fschrhunt/ask-NAME and says whether each agent is ready", func(t *testing.T) {
		s := fresh(t)
		repo := filepath.Join(s.tmp, "fschrhunt", "ask-demo")
		s.mkdir(repo)
		git := s.gitAt(repo)
		git("init", "-q", "-b", "main")
		s.scriptAt(repo, "agents", "demo", `[ "$1" = models ] && printf 'm1\tM1\n'`)
		s.scriptAt(repo, "agents", "broken", `echo "broken needs its CLI: install it from example.com" >&2; exit 1`)
		s.scriptAt(repo, "agents", "fake", `echo shadowed`)
		git("add", ".")
		git("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "v1")
		github := map[string]string{"GIT_CONFIG_COUNT": "1", "GIT_CONFIG_KEY_0": "url." + s.tmp + "/.insteadOf", "GIT_CONFIG_VALUE_0": "https://github.com/"}
		r := s.run([]string{"install", "demo"}, "", github)
		eq(t, r.code, 1)
		match(t, r.stderr, `installed github.com/fschrhunt/ask-demo`)
		match(t, r.stderr, `(?m)^ask: demo is ready: 1 model, see ask models$`)
		match(t, r.stderr, `(?m)^ask: broken is not ready: broken needs its CLI: install it from example.com$`)
		match(t, r.stderr, `(?m)^ask: fake: .*agents/fake is used instead of this package's`)
	})
	t.Run("settings choose the default model, worktree folders and branch names", func(t *testing.T) {
		s := fresh(t)
		dir, git := s.repo()
		s.write(filepath.Join(s.home, "settings.json"), `{"model": "fake:big", "worktrees": "`+s.tmp+`/wt/ask-{name}", "branches": "agents/{name}"}`)
		r := s.run([]string{"-w", "--worktree", "-C", dir, "go"}, "", map[string]string{"FAKE_WRITE": "b.txt=new"})
		eq(t, r.code, 0)
		match(t, r.stderr, ` · started · Big · write · worktree ~/wt/ask-go\n`)
		match(t, r.stderr, ` · branch agents/go · `)
		git("rev-parse", "--verify", "-q", "refs/heads/agents/go")
		s.write(filepath.Join(s.home, "settings.json"), `{"jobs": 0}`)
		bad := s.ask("-m", "fake:small", "hi")
		eq(t, bad.code, 2)
		match(t, bad.stderr, `"jobs" must be a whole number of tasks, 1 or more`)
	})
	t.Run("ask runs in a repository lists its runs, from any of its checkouts; --all lists every run", func(t *testing.T) {
		s := fresh(t)
		dir, git := s.repo()
		other := filepath.Join(s.tmp, "other")
		s.mkdir(other)
		s.ask("-m", "fake:small", "-C", dir, "in repo")
		s.ask("-m", "fake:small", "-C", other, "elsewhere")
		linked := filepath.Join(s.tmp, "linked")
		git("worktree", "add", "-q", "--detach", linked)
		here := s.runIn(linked, "runs")
		match(t, here.stdout, `in repo`)
		eq(t, strings.Contains(here.stdout, "elsewhere"), false)
		match(t, s.runIn(linked, "runs", "--all").stdout, `elsewhere`)
	})
	t.Run("ask title --hook answers a PreToolUse event for ask commands and stays silent otherwise", func(t *testing.T) {
		s := fresh(t)
		event := `{"tool_name":"Bash","tool_input":{"command":"ask -m fake:small hi","description":"Say hi","timeout":5}}`
		eq(t, s.run([]string{"title", "--hook"}, event, nil).stdout, `{"hookSpecificOutput":{"hookEventName":"PreToolUse","updatedInput":{"command":"ask -m fake:small hi","description":"Small One · Say hi","run_in_background":true,"timeout":5}}}`+"\n")
		other := s.run([]string{"title", "--hook"}, `{"tool_name":"Bash","tool_input":{"command":"ls","description":"List"}}`, nil)
		eq(t, other.stdout+other.stderr, "")
		eq(t, other.code, 0)
	})
}

// runIn runs ask in dir, for behavior that depends on the working directory.
func (s *setup) runIn(dir string, args ...string) output {
	s.t.Helper()
	s.cwd = dir
	defer func() { s.cwd = "" }()
	return s.ask(args...)
}

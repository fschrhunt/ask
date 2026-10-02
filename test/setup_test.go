package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSetup pins what makes ask easy to set up and adapt: official agent names, readiness checks,
// settings, runs per repository and the host title hook.
func TestSetup(t *testing.T) {
	t.Run("ask install NAME installs the official agent built into ask, even beside a folder NAME", func(t *testing.T) {
		s := fresh(t)
		s.mkdir(filepath.Join(s.tmp, "claude"))
		r := s.ask("install", "claude")
		match(t, r.stderr, `(?m)^ask: installed ask/packages/claude: agents: claude$`)
		match(t, r.stderr, `(?m)^ask: claude is (ready|not ready)`)
		agent := filepath.Join(s.home, "packages", "ask", "packages", "claude", "agents", "claude")
		info, e := os.Stat(agent)
		if e != nil || info.Mode()&0111 == 0 {
			t.Fatalf("agent not executable: %v %v", info, e)
		}
		match(t, s.ask("packages").stdout, `(?m)^ask/packages/claude +agents: claude$`)
		match(t, s.ask("install", "claude").stderr, `ask/packages/claude: up to date`)
		os.WriteFile(agent, []byte("#!/bin/sh\necho stale\n"), 0755)
		os.WriteFile(filepath.Join(filepath.Dir(filepath.Dir(agent)), ".ask-version"), []byte("old\n"), 0644)
		s.ask("runs")
		eq(t, strings.Contains(s.read(agent), "stale"), false)
		bad := s.ask("install", "gemini")
		eq(t, bad.code, 2)
		match(t, bad.stderr, `no official agent "gemini"; ask installs claude, codex, opencode by name`)
	})
	t.Run("ask install says whether each agent a package brings is ready", func(t *testing.T) {
		s := fresh(t)
		repo := filepath.Join(s.tmp, "acme", "ask-demo")
		s.mkdir(repo)
		git := s.gitAt(repo)
		git("init", "-q", "-b", "main")
		s.scriptAt(repo, "agents", "demo", `[ "$1" = models ] && printf 'm1\tM1\n'`)
		s.scriptAt(repo, "agents", "broken", `echo "broken needs its CLI: install it from example.com" >&2; exit 1`)
		s.scriptAt(repo, "agents", "fake", `echo shadowed`)
		git("add", ".")
		git("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "v1")
		github := map[string]string{"GIT_CONFIG_COUNT": "1", "GIT_CONFIG_KEY_0": "url." + s.tmp + "/.insteadOf", "GIT_CONFIG_VALUE_0": "https://github.com/"}
		r := s.run([]string{"install", "acme/ask-demo"}, "", github)
		eq(t, r.code, 1)
		match(t, r.stderr, `installed github.com/acme/ask-demo`)
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

// TestSetupCommand pins ask setup for machines: its report, its exit status and its flags.
func TestSetupCommand(t *testing.T) {
	t.Run("--check reports every agent and setting, and exits 1 while an installed agent cannot run", func(t *testing.T) {
		s := fresh(t)
		r := s.ask("setup", "--check")
		eq(t, r.code, 0)
		match(t, r.stdout, `(?m)^  ✓ fake +ready · \d+ models?$`)
		match(t, r.stdout, `(?m)^  timeout +900 \(default\)$`)
		s.script("agents", "claude", `echo "Claude Code not found: install it" >&2; exit 1`)
		r = s.ask("setup", "--check", "--json")
		eq(t, r.code, 1)
		match(t, r.stdout, `"reason": "Claude Code not found: install it"`)
	})
	t.Run("flags set defaults, skills and the Claude Code hook without asking", func(t *testing.T) {
		s := fresh(t)
		s.mkdir(filepath.Join(s.tmp, ".claude"))
		r := s.ask("setup", "--model", "fake:small", "-j", "6", "--worktrees", "~/wt/ask-{name}", "--skills", "claude-code", "--hook")
		eq(t, r.code, 0)
		settings := obj(t, s.read(filepath.Join(s.home, "settings.json")))
		eq(t, settings.s("model")+" "+settings.s("worktrees"), "fake:small ~/wt/ask-{name}")
		eq(t, settings.n("jobs"), 6.0)
		match(t, s.read(filepath.Join(s.tmp, ".claude", "skills", "ask", "SKILL.md")), `(?m)^name: ask$`)
		match(t, s.read(filepath.Join(s.tmp, ".claude", "settings.json")), `"command": "ask title --hook"`)
		eq(t, s.ask("-C", s.tmp, "hi").stdout, "fake: hi\n")
		eq(t, s.ask("setup", "-m", "").code, 0)
		eq(t, strings.Contains(s.read(filepath.Join(s.home, "settings.json")), "model"), false)
	})
	t.Run("bad values and flag combinations are usage errors", func(t *testing.T) {
		s := fresh(t)
		for _, args := range [][]string{{"setup", "-j", "0"}, {"setup", "--json"}, {"setup", "--hook", "--no-hook"}, {"setup", "--skills", "emacs"}, {"setup", "extra"}} {
			eq(t, s.ask(args...).code, 2)
		}
	})
	t.Run("without a terminal or flags it reports and says how to change things", func(t *testing.T) {
		s := fresh(t)
		r := s.ask("setup")
		eq(t, r.code, 0)
		match(t, r.stdout, `(?m)^Agents$`)
		match(t, r.stderr, `run ask setup in a terminal`)
	})
}

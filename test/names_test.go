package test

import (
	"strings"
	"testing"
)

// TestNames pins run names: made from the prompt, kept apart, taken over by follow-ups, and hookable.
func TestNames(t *testing.T) {
	t.Run("a run is named after its prompt, and a later run with the same name gets a number", func(t *testing.T) {
		s := fresh(t)
		first := s.ask("-m", "fake:small", "Please add rate limiting to the login route.")
		eq(t, runID(t, first.stderr), "add-rate-limiting-login")
		second := s.ask("-m", "fake:small", "Add rate limiting to login")
		eq(t, runID(t, second.stderr), "add-rate-limiting-login-2")
	})
	t.Run("a follow-up takes over the name, so -c NAME reaches the latest turn and the ID an earlier one", func(t *testing.T) {
		s := fresh(t)
		s.ask("-m", "fake:small", "remember PELICAN")
		id := strings.Fields(strings.Split(s.ask("runs").stdout, "\n")[1])[1]
		second := s.ask("-c", "remember-pelican", "and FALCON")
		eq(t, runID(t, second.stderr), "remember-pelican")
		s.ask("-c", "remember-pelican", "what words?")
		calls := s.calls()
		eq(t, calls[2].s("session"), calls[1].s("session"))
		eq(t, s.ask("show", "remember-pelican").stdout, "fake: what words?\n")
		eq(t, s.ask("show", id).stdout, "fake: remember PELICAN\n")
		eq(t, len(s.dirs()), 3)
	})
	t.Run("a name hook can rename a run from its prompts; a failing one keeps the prompt's name", func(t *testing.T) {
		s := fresh(t)
		s.hook("namer", []string{"name"}, `echo '{"name":"Login Throttle"}'`)
		r := s.ask("-m", "fake:small", "Add rate limiting to login")
		eq(t, runID(t, r.stderr), "login-throttle")
		eq(t, s.hookCalls()[0].s("event"), "name")
		s.hook("namer", []string{"name"}, `exit 3`)
		r = s.ask("-m", "fake:small", "Add rate limiting to login")
		eq(t, runID(t, r.stderr), "add-rate-limiting-login")
		match(t, r.stderr, `(?m)^ask add-rate-limiting-login · note · hook namer · failed`)
	})
}

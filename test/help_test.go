package test

import (
	"strings"
	"testing"
)

// TestHelpOverview gives callers commands, options and local models in one screen.
func TestHelpOverview(t *testing.T) {
	s := fresh(t)
	s.script("agents", "broken", "echo 'not logged in' >&2; exit 1")
	r := s.ask("--help")
	eq(t, r.code, 0)
	if len(strings.Split(strings.TrimSpace(r.stdout), "\n")) > 40 {
		t.Fatal("help is too long")
	}
	match(t, r.stdout, `(?m)^  fake +small  +big$`)
	match(t, r.stdout, `(?m)^  broken +\(could not list: not logged in\)$`)
	eq(t, len(s.calls()), 0)
	for topic, page := range map[string]string{"batch": "batches", "hooks": "hooks", "agents": "agents"} {
		r := s.ask("help", topic)
		eq(t, r.code, 0)
		match(t, r.stdout, `(?m)^ask `+topic+` · `)
		match(t, r.stdout, `\ndocs  github.com/fschrhunt/ask/tree/main/docs/`+page+`\.md\n$`)
	}
}

// TestModelNames exposes listed and fallback display names while retaining script ids.
func TestModelNames(t *testing.T) {
	s := fresh(t)
	eq(t, s.ask("models").stdout, "fake:small\nfake:big\n")
	eq(t, s.ask("models", "--names").stdout, "fake:small\tSmall One\nfake:big\tBig\n")
}

// TestUsagePointer keeps ordinary caller errors to one actionable line with the right topic.
func TestUsagePointer(t *testing.T) {
	s := fresh(t)
	r := s.ask("batch", "--typo")
	eq(t, r.code, 2)
	eq(t, r.stderr, "ask: unknown option --typo; see ask batch --help\n")
	eq(t, s.ask("help", "nope").code, 2)
	r = s.ask("-m", "unknown:small", "hi")
	eq(t, r.code, 2)
	match(t, r.stderr, `(?m)^  fake +small  +big$`)
}

// TestCommandHelp makes command pages and help flags resolve to the same concise text.
func TestCommandHelp(t *testing.T) {
	s := fresh(t)
	for _, command := range []string{"run", "batch", "show", "runs", "stop", "models", "title", "install", "packages", "remove"} {
		fromTopic := s.ask("help", command)
		eq(t, fromTopic.code, 0)
		fromFlag := s.ask(command, "--help")
		eq(t, fromFlag.stdout, fromTopic.stdout)
		eq(t, s.ask(command, "-h").stdout, fromTopic.stdout)
		match(t, fromTopic.stdout, `(?m)^Usage$`)
	}
	overview := s.ask().stdout
	for _, argv := range [][]string{{"help"}, {"-h"}, {"--help"}} {
		eq(t, s.ask(argv...).stdout, overview)
	}
}

// TestOptionSuggestions distinguishes near misses from unrelated invalid flags.
func TestOptionSuggestions(t *testing.T) {
	s := fresh(t)
	eq(t, s.ask("-help").stderr, "ask: unknown option -help; did you mean --help?\n")
	eq(t, s.ask("-version").stderr, "ask: unknown option -version; did you mean --version?\n")
	eq(t, s.ask("batch", "--resme").stderr, "ask: unknown option --resme; did you mean --resume?; see ask batch --help\n")
	eq(t, s.ask("--version").stdout, s.ask("-V").stdout)
}

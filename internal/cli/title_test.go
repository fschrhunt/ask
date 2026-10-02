package cli

import (
	"reflect"
	"testing"
)

// TestShellCommands pins literal quoting, escapes, separators, assignments and redirects.
func TestShellCommands(t *testing.T) {
	cases := []struct {
		input string
		want  [][]string
	}{
		{`cd ~/code/app && VAR='two words' /bin/ask -m fake:x "Fix \"login\"" 2>&1 | tail -20`, [][]string{{"cd", "~/code/app"}, {"VAR=two words", "/bin/ask", "-m", "fake:x", `Fix "login"`}, {"tail", "-20"}}},
		{"ask -m fake:x 'a|b' escaped\\ word; ask 'it'\ntrue || false", [][]string{{"ask", "-m", "fake:x", "a|b", "escaped word"}, {"ask", "it"}, {"true"}, {"false"}}},
		{"A=1 ask -m fake:x \"a\\qb\" \\\n 'c\\d' > out", [][]string{{"A=1", "ask", "-m", "fake:x", `a\qb`, `c\d`}}},
	}
	for _, c := range cases {
		got, ok := shellCommands(c.input)
		if !ok || !reflect.DeepEqual(got, c.want) {
			t.Fatalf("%q: got %#v, %v; want %#v", c.input, got, ok, c.want)
		}
	}
	for _, input := range []string{`ask "unfinished`, "ask trailing\\", `ask $(echo hi)`, `ask go >`, `ask go ||`, `ask go <<EOF`} {
		if _, ok := shellCommands(input); ok {
			t.Fatalf("accepted %q", input)
		}
	}
}

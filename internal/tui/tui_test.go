package tui

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// prompter reads the given keys and draws into a buffer, without color.
func prompter(keys string) (*Prompter, *bytes.Buffer) {
	var out bytes.Buffer
	return &Prompter{In: strings.NewReader(keys), Out: &out}, &out
}

// TestPrompts pins each prompt's keys and its answered line.
func TestPrompts(t *testing.T) {
	p, out := prompter("\x1b[B\x1b[B\x1b[A\r")
	i, e := p.Select("Pick", []Option{{Label: "a"}, {Label: "b"}, {Label: "c"}}, 0)
	if e != nil || i != 1 || !strings.HasSuffix(out.String(), "? Pick b\r\n") {
		t.Fatalf("select: %d %v %q", i, e, out.String())
	}
	p, _ = prompter(" \x01\r")
	picked, e := p.MultiSelect("Pick", []Option{{Label: "a"}, {Label: "b"}}, []bool{false, true})
	if e != nil || picked[0] || picked[1] {
		t.Fatalf("space then ctrl-a toggles everything off: %v %v", picked, e)
	}
	p, out = prompter("ta\r")
	i, e = p.Select("Pick", []Option{{Label: "alpha"}, {Label: "beta"}, {Label: "gamma", Note: "third"}}, 0)
	if e != nil || i != 1 || !strings.Contains(out.String(), "filter: ta") {
		t.Fatalf("typing filters, enter takes the first match: %d %v", i, e)
	}
	p, out = prompter("ab\x7fc\r")
	text, e := p.Input("Name", "x")
	if e != nil || text != "ac" || !strings.HasSuffix(out.String(), "? Name ac\r\n") {
		t.Fatalf("input: %q %v", text, e)
	}
	p, _ = prompter("\r")
	if text, _ = p.Input("Name", "x"); text != "x" {
		t.Fatalf("empty input keeps the default: %q", text)
	}
	p, _ = prompter("\r")
	if yes, _ := p.Confirm("Go?", true); !yes {
		t.Fatal("enter takes the default")
	}
	p, _ = prompter("\x03")
	if _, e = p.Select("Pick", []Option{{Label: "a"}}, 0); !errors.Is(e, ErrInterrupted) {
		t.Fatalf("ctrl-c: %v", e)
	}
}

// TestDiff pins the review's line diff: shared lines left out, removals before additions.
func TestDiff(t *testing.T) {
	got := strings.Join(Diff("{\n  \"a\": 1,\n  \"b\": 2\n}\n", "{\n  \"a\": 1,\n  \"b\": 3\n}\n"), "|")
	if got != `-   "b": 2|+   "b": 3` {
		t.Fatalf("%q", got)
	}
	var out bytes.Buffer
	Review(&out, []Change{{Path: "x.json", Before: "a\n", After: "b\n"}}, false, 80)
	if out.String() != "x.json  +1 -1\n- a\n+ b\n\n1 file changed, 1 insertion(+), 1 deletion(-)\n" {
		t.Fatalf("%q", out.String())
	}
}

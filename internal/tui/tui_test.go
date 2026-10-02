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
	p, out := prompter("\x1b[Bj\x1b[A\r")
	i, e := p.Select("Pick", []Option{{Label: "a"}, {Label: "b"}, {Label: "c"}}, 0)
	if e != nil || i != 1 || !strings.HasSuffix(out.String(), "? Pick b\r\n") {
		t.Fatalf("select: %d %v %q", i, e, out.String())
	}
	p, _ = prompter(" a\r")
	picked, e := p.MultiSelect("Pick", []Option{{Label: "a"}, {Label: "b"}}, []bool{false, true})
	if e != nil || picked[0] || picked[1] {
		t.Fatalf("space then a toggles everything off: %v %v", picked, e)
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

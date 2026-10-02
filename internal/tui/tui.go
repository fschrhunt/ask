// Package tui draws gh-style prompts on a terminal: confirm, select, multi-select and text input.
// Prompts read keys from In and draw on Out, so tests drive them with plain byte streams; Raw
// puts a real terminal in the mode they need.
package tui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

// ErrInterrupted reports Ctrl-C during a prompt; the caller should stop and exit 130.
var ErrInterrupted = errors.New("interrupted")

// Option is one choice, with an optional dim note shown after its label.
type Option struct{ Label, Note string }

// Prompter asks questions on one terminal. Color adds gh's accents; without it output is plain.
type Prompter struct {
	In    io.Reader
	Out   io.Writer
	Color bool
	lines int
}

// Raw switches a terminal to unbuffered, unechoed input and returns the function that restores it.
func Raw(f *os.File) (func(), error) {
	var old syscall.Termios
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(getTermios), uintptr(unsafe.Pointer(&old))); e != 0 {
		return nil, e
	}
	raw := old
	raw.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	raw.Iflag &^= syscall.ICRNL | syscall.IXON
	raw.Cc[syscall.VMIN], raw.Cc[syscall.VTIME] = 1, 0
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(setTermios), uintptr(unsafe.Pointer(&raw))); e != 0 {
		return nil, e
	}
	return func() {
		syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(setTermios), uintptr(unsafe.Pointer(&old)))
	}, nil
}

// key reads one keypress: a printable rune, or "up", "down", "enter", "space", "backspace", "ctrl-c".
func (p *Prompter) key() (string, error) {
	b := make([]byte, 1)
	if _, e := p.In.Read(b); e != nil {
		return "", e
	}
	switch b[0] {
	case 3:
		return "ctrl-c", nil
	case '\r', '\n':
		return "enter", nil
	case ' ':
		return "space", nil
	case 127, 8:
		return "backspace", nil
	case 27:
		seq := make([]byte, 2)
		if _, e := io.ReadFull(p.In, seq); e != nil || seq[0] != '[' {
			return "", nil
		}
		switch seq[1] {
		case 'A':
			return "up", nil
		case 'B':
			return "down", nil
		}
		return "", nil
	}
	if b[0] < 0x80 {
		return string(b), nil
	}
	rest := 1
	if b[0] >= 0xE0 {
		rest = 2
	}
	if b[0] >= 0xF0 {
		rest = 3
	}
	more := make([]byte, rest)
	io.ReadFull(p.In, more)
	return string(append(b, more...)), nil
}

// paint styles text with an ANSI code when color is on.
func (p *Prompter) paint(code, text string) string {
	if !p.Color || text == "" {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

// draw replaces the lines this prompt drew last with lines.
func (p *Prompter) draw(lines []string) {
	if p.lines > 0 {
		fmt.Fprintf(p.Out, "\x1b[%dA\r\x1b[J", p.lines)
	}
	for _, l := range lines {
		fmt.Fprint(p.Out, l, "\r\n")
	}
	p.lines = len(lines)
}

// done replaces the prompt with its answered form, "? Question answer", and forgets its lines.
func (p *Prompter) done(question, answer string) {
	p.draw([]string{p.paint("32", "?") + " " + p.paint("1", question) + " " + p.paint("36", answer)})
	p.lines = 0
}

// question is a prompt's first line, with a dim hint.
func (p *Prompter) question(q, hint string) string {
	line := p.paint("32", "?") + " " + p.paint("1", q)
	if hint != "" {
		line += " " + p.paint("2", hint)
	}
	return line
}

// Confirm asks a yes-or-no question; enter takes the default.
func (p *Prompter) Confirm(q string, yes bool) (bool, error) {
	hint := "(y/N)"
	if yes {
		hint = "(Y/n)"
	}
	p.draw([]string{p.question(q, hint)})
	for {
		k, e := p.key()
		if e != nil {
			return false, e
		}
		switch strings.ToLower(k) {
		case "ctrl-c":
			return false, ErrInterrupted
		case "y":
			yes = true
		case "n":
			yes = false
		case "enter":
		default:
			continue
		}
		answer := "No"
		if yes {
			answer = "Yes"
		}
		p.done(q, answer)
		return yes, nil
	}
}

// window is the slice of n options shown around cursor, at most ten at a time.
func window(n, cursor int) (int, int) {
	const size = 10
	if n <= size {
		return 0, n
	}
	start := min(max(cursor-size/2, 0), n-size)
	return start, start + size
}

// labelWidth is the widest label, so notes line up in a column.
func labelWidth(options []Option) int {
	w := 0
	for _, o := range options {
		w = max(w, len([]rune(o.Label)))
	}
	return w
}

// optionLine renders one option, its note aligned after width, marked when it has the cursor.
func (p *Prompter) optionLine(o Option, current bool, box string, width int) string {
	mark := "  "
	label := o.Label
	if o.Note != "" {
		label += strings.Repeat(" ", width-len([]rune(o.Label)))
	}
	if current {
		mark = p.paint("36", "> ")
		label = p.paint("36", label)
	}
	line := mark + box + label
	if o.Note != "" {
		line += "  " + p.paint("2", o.Note)
	}
	return strings.TrimRight(line, " ")
}

// Select asks for one of options with arrow keys (or j and k), starting at start.
func (p *Prompter) Select(q string, options []Option, start int) (int, error) {
	cursor := start
	for {
		lines := []string{p.question(q, "[↑↓ to move, enter to choose]")}
		from, to := window(len(options), cursor)
		for i := from; i < to; i++ {
			lines = append(lines, p.optionLine(options[i], i == cursor, "", labelWidth(options)))
		}
		p.draw(lines)
		k, e := p.key()
		if e != nil {
			return 0, e
		}
		switch k {
		case "ctrl-c":
			return 0, ErrInterrupted
		case "up", "k":
			cursor = (cursor + len(options) - 1) % len(options)
		case "down", "j":
			cursor = (cursor + 1) % len(options)
		case "enter":
			p.done(q, options[cursor].Label)
			return cursor, nil
		}
	}
}

// MultiSelect asks for any of options: space toggles, a toggles all, enter confirms.
func (p *Prompter) MultiSelect(q string, options []Option, chosen []bool) ([]bool, error) {
	picked := append([]bool(nil), chosen...)
	cursor := 0
	for {
		lines := []string{p.question(q, "[space to select, a for all, enter to confirm]")}
		from, to := window(len(options), cursor)
		for i := from; i < to; i++ {
			box := "[ ] "
			if picked[i] {
				box = p.paint("32", "[x]") + " "
			}
			lines = append(lines, p.optionLine(options[i], i == cursor, box, labelWidth(options)))
		}
		p.draw(lines)
		k, e := p.key()
		if e != nil {
			return nil, e
		}
		switch k {
		case "ctrl-c":
			return nil, ErrInterrupted
		case "up", "k":
			cursor = (cursor + len(options) - 1) % len(options)
		case "down", "j":
			cursor = (cursor + 1) % len(options)
		case "space":
			picked[cursor] = !picked[cursor]
		case "a":
			all := true
			for _, x := range picked {
				all = all && x
			}
			for i := range picked {
				picked[i] = !all
			}
		case "enter":
			names := []string{}
			for i, x := range picked {
				if x {
					names = append(names, options[i].Label)
				}
			}
			answer := strings.Join(names, ", ")
			if answer == "" {
				answer = "None"
			}
			p.done(q, answer)
			return picked, nil
		}
	}
}

// Input asks for a line of text; enter on an empty line takes the default.
func (p *Prompter) Input(q, def string) (string, error) {
	var text []rune
	hint := ""
	if def != "" {
		hint = "(" + def + ")"
	}
	for {
		p.draw([]string{p.question(q, hint) + " " + string(text)})
		k, e := p.key()
		if e != nil {
			return "", e
		}
		switch k {
		case "ctrl-c":
			return "", ErrInterrupted
		case "enter":
			answer := strings.TrimSpace(string(text))
			if answer == "" {
				answer = def
			}
			p.done(q, answer)
			return answer, nil
		case "backspace":
			if len(text) > 0 {
				text = text[:len(text)-1]
			}
		case "space":
			text = append(text, ' ')
		case "up", "down", "":
		default:
			text = append(text, []rune(k)...)
		}
	}
}

// Say prints a finished line, like "✓ Installed claude", that later prompts never redraw.
func (p *Prompter) Say(mark, text string) {
	code := map[string]string{"✓": "32", "!": "33", "✗": "31", "•": "2"}[mark]
	fmt.Fprint(p.Out, p.paint(code, mark)+" "+text+"\r\n")
}

// Bold styles a heading.
func (p *Prompter) Bold(text string) string { return p.paint("1", text) }

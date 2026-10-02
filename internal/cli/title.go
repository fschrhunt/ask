package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/fschrhunt/ask/internal/agent"
	"github.com/fschrhunt/ask/internal/find"
	"github.com/fschrhunt/ask/internal/home"
	"github.com/fschrhunt/ask/internal/hooks"
	"github.com/fschrhunt/ask/internal/runs"
)

var assignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

// shellInput is what a simple command reads from a heredoc and the file its stdout goes to.
// Stdin is set only for a heredoc ask can read literally: a quoted delimiter, or a body
// without expansions.
type shellInput struct {
	Stdin    string
	HasStdin bool
	Out      string
}

// heredoc is a heredoc whose body starts after the line that opened it.
type heredoc struct {
	command      int
	delim        string
	strip, quote bool
}

// shellCommands splits literal simple commands without executing expansions or shell code,
// with each command's heredoc and stdout file in inputs.
// Unsupported syntax and incomplete quotes, escapes, redirects or heredocs make the whole input unusable.
func shellCommands(s string) (commands [][]string, inputs []shellInput, ok bool) {
	commands = [][]string{}
	words := []string{}
	var word strings.Builder
	var input shellInput
	var pending []heredoc
	active, redirect, skip, out, descriptor := false, false, false, false, true
	quote := byte(0)
	flush := func() {
		if active {
			if skip {
				if out {
					input.Out = word.String()
				}
				skip, out = false, false
			} else {
				words = append(words, word.String())
			}
			word.Reset()
			active = false
			descriptor = true
		}
	}
	finish := func() bool {
		flush()
		if skip || redirect {
			return false
		}
		if len(words) > 0 {
			commands = append(commands, words)
			inputs = append(inputs, input)
			words = nil
		}
		input = shellInput{}
		return true
	}
	// bodies reads the pending heredocs' bodies after the newline at i, returning the index of
	// the newline that ends the last delimiter line, or -1 when a delimiter never comes.
	bodies := func(i int) int {
		for _, h := range pending {
			var body strings.Builder
			for {
				if i >= len(s) {
					return -1
				}
				end := strings.IndexByte(s[i+1:], '\n')
				if end < 0 {
					end = len(s)
				} else {
					end += i + 1
				}
				line := s[i+1 : end]
				i = end
				if h.strip {
					line = strings.TrimLeft(line, "\t")
				}
				if line == h.delim {
					break
				}
				body.WriteString(line + "\n")
			}
			text := body.String()
			if h.command < len(commands) && (h.quote || !strings.ContainsAny(text, "$`\\")) {
				inputs[h.command].Stdin, inputs[h.command].HasStdin = text, true
			}
		}
		pending = nil
		return i
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote == '\'' {
			if c == '\'' {
				quote = 0
			} else {
				word.WriteByte(c)
			}
			continue
		}
		if c == '\\' {
			if i+1 == len(s) {
				return nil, nil, false
			}
			i++
			next := s[i]
			if next == '\n' {
				continue
			}
			active = true
			descriptor = false
			redirect = false
			if quote == '"' && !strings.ContainsRune("$`\"\\", rune(next)) {
				word.WriteByte('\\')
			}
			word.WriteByte(next)
			continue
		}
		if quote == '"' {
			if c == '"' {
				quote = 0
			} else {
				if c == '`' || c == '$' {
					return nil, nil, false
				}
				word.WriteByte(c)
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
			descriptor = false
			active = true
			redirect = false
		case ' ', '\t', '\r':
			flush()
		case '#':
			if active {
				word.WriteByte(c)
			} else {
				for i < len(s) && s[i] != '\n' {
					i++
				}
				i--
			}
		case '$', '`', '(', ')', '{', '}':
			return nil, nil, false
		case '<', '>':
			fd := ""
			if active && descriptor && allDigits(word.String()) {
				fd = word.String()
				word.Reset()
				active = false
			} else {
				flush()
			}
			if skip {
				return nil, nil, false
			}
			if c == '<' && strings.HasPrefix(s[i:], "<<") {
				h, next, ok := heredocStart(s, i+2)
				if !ok {
					return nil, nil, false
				}
				h.command = len(commands)
				pending = append(pending, h)
				i = next - 1
				continue
			}
			skip = true
			out = c == '>' && (fd == "" || fd == "1")
			if i+1 < len(s) && (s[i+1] == c || s[i+1] == '&' || s[i+1] == '|') {
				i++
				out = out && s[i] != '&'
			}
		case ';', '|', '&', '\n':
			if c == '\n' && redirect {
				if len(pending) > 0 {
					return nil, nil, false
				}
				continue
			}
			if (c == '|' || c == '&' || c == ';') && len(words) == 0 && !active {
				return nil, nil, false
			}
			if !finish() {
				return nil, nil, false
			}
			if c == '\n' && len(pending) > 0 {
				if i = bodies(i); i < 0 {
					return nil, nil, false
				}
				continue
			}
			if (c == '|' || c == '&') && i+1 < len(s) && s[i+1] == c {
				i++
				redirect = true
			}
			if c == '|' {
				redirect = true
			}
		default:
			redirect = false
			active = true
			if c < '0' || c > '9' {
				descriptor = false
			}
			word.WriteByte(c)
		}
	}
	if quote != 0 || !finish() || len(pending) > 0 {
		return nil, nil, false
	}
	return commands, inputs, true
}

// heredocStart reads a heredoc's optional - and its delimiter word after <<, returning the
// index after the word. A quoted delimiter, wholly or in part, keeps the body literal.
func heredocStart(s string, i int) (heredoc, int, bool) {
	h := heredoc{}
	if i < len(s) && s[i] == '<' {
		return h, i, false // a here-string
	}
	if i < len(s) && s[i] == '-' {
		h.strip = true
		i++
	}
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	var delim strings.Builder
	quote := byte(0)
	for ; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			} else {
				delim.WriteByte(c)
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote, h.quote = c, true
			continue
		}
		if c == '\\' && i+1 < len(s) {
			i++
			delim.WriteByte(s[i])
			h.quote = true
			continue
		}
		if strings.IndexByte(" \t\n;|&<>()$`", c) >= 0 {
			break
		}
		delim.WriteByte(c)
	}
	h.delim = delim.String()
	return h, i, quote == 0 && h.delim != ""
}

// allDigits recognizes a redirect's optional file descriptor.
func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// titleJob uses a supplied description, or a clipped first prompt line, as one safe line.
func titleJob(description, prompt string) string {
	text := strings.TrimSpace(description)
	if text == "" {
		text, _, _ = strings.Cut(strings.TrimSpace(prompt), "\n")
		r := []rune(text)
		if len(r) > 60 {
			text = string(r[:59]) + "…"
		}
	}
	text = strings.Join(strings.Fields(text), " ")
	r := []rune(text)
	if len(r) > 0 {
		r[0] = unicode.ToUpper(r[0])
	}
	return string(r)
}

// titlePath resolves literal host paths relative to preceding cd commands.
func titlePath(dir, path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		h, _ := os.UserHomeDir()
		path = filepath.Join(h, strings.TrimPrefix(path, "~"))
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	return path
}

// invocationTitle describes task-running invocations without preparing or executing tasks.
// in is the invocation's heredoc, and files holds files earlier commands write from heredocs,
// which do not exist yet when a host asks for the title.
func invocationTitle(a *agent.Registry, argv []string, description, dir string, in shellInput, files map[string]string) (string, home.Object) {
	args := argv[1:]
	command := "run"
	if len(args) == 0 {
		return "", nil
	}
	if _, ok := options[args[0]]; ok && args[0] != "run" {
		command = args[0]
		args = args[1:]
	} else if find.Path(a.Paths, "commands", args[0]) != "" {
		return "", nil
	}
	if command != "run" && command != "batch" {
		return "", nil
	}
	opts, words, err := parse(command, args)
	if err != nil || opts.B("help") {
		return "", nil
	}
	for _, f := range []string{"-t", "-j"} {
		if opts.Has(f) {
			if _, err := number(f, opts.S(f), f == "-j"); err != nil {
				return "", nil
			}
		}
	}
	invocationDir := dir
	if opts.Has("-C") {
		dir = titlePath(dir, opts.S("-C"))
	}
	prompt := strings.Join(words, " ")
	label := ""
	if command == "batch" {
		if opts.B("--resume") {
			if len(words) > 0 {
				return "", nil
			}
			for key := range opts {
				if !has([]string{"--resume", "-j", "--no-hooks"}, key) {
					return "", nil
				}
			}
			label = "Resume " + opts.S("--resume")
			if r, _, err := runs.Open(a.Paths, opts.S("--resume")); err == nil && len(r.Tasks) > 0 {
				prompt = r.Tasks[0].S("prompt")
			}
		} else {
			if len(words) > 1 {
				return "", nil
			}
			label = "Batch"
			prompt = ""
			text, found := in.Stdin, in.HasStdin
			if len(words) == 1 && words[0] != "-" {
				path := titlePath(invocationDir, words[0])
				text, found = files[path]
				if !found {
					b, err := os.ReadFile(path)
					text, found = string(b), err == nil
				}
			}
			if items, err := readTasks(text); found && err == nil {
				names := []string{}
				known := true
				for _, item := range items {
					t, ok := item.(home.Object)
					if !ok {
						known = false
						break
					}
					if prompt == "" {
						prompt = t.S("prompt")
					}
					model := t.S("model")
					if model == "" {
						model = opts.S("-m")
					}
					if model == "" && t.B("continue") {
						if r, i, e := runs.Open(a.Paths, t.S("continue")); e == nil && i >= 0 {
							model = r.Results[i].S("model")
						}
					}
					m, err := agent.Parse(a.Paths, model)
					if err != nil {
						known = false
						break
					}
					name := a.Name(m, "")
					if !has(names, name) {
						names = append(names, name)
					}
				}
				if known {
					label = "Batch of " + home.String(len(items))
					if len(names) <= 3 {
						label += " · " + strings.Join(names, ", ")
					}
				}
			}
		}
	} else {
		model := opts.S("-m")
		write := opts.B("-w")
		worktree := opts.B("--worktree")
		if opts.B("-c") {
			r, i, e := runs.Open(a.Paths, opts.S("-c"))
			if e != nil || i < 0 {
				return "", nil
			}
			old := r.Tasks[i]
			if model == "" {
				model = r.Results[i].S("model")
			}
			if !opts.B("-r") && !opts.B("-w") {
				write = r.Results[i].B("write") || !r.Results[i].Has("write") && old.B("write")
			}
			worktree = worktree || old.B("worktree")
			if prompt == "" || prompt == "-" {
				prompt = old.S("prompt")
			}
		}
		m, err := agent.Parse(a.Paths, model)
		if err != nil {
			return "", nil
		}
		label = a.Name(m, "")
		job := titleJob(description, prompt)
		if job != "" && (prompt != "-" || strings.TrimSpace(description) != "") {
			label += " · " + job
		}
		if worktree {
			if !write {
				return "", nil
			}
			label += " · worktree"
		} else if write {
			label += " · write"
		}
		return label, opts
	}
	if job := titleJob(description, prompt); job != "" {
		label += " · " + job
	}
	return label, opts
}

// commandTitle finds the first literal ask program and applies optional host-title hooks.
func commandTitle(a *agent.Registry, command, description string) string {
	commands, inputs, ok := shellCommands(command)
	if !ok {
		return ""
	}
	dir, _ := os.Getwd()
	files := map[string]string{}
	for i, argv := range commands {
		for len(argv) > 0 && assignment.MatchString(argv[0]) {
			argv = argv[1:]
		}
		if len(argv) == 0 {
			continue
		}
		if argv[0] == "cd" && len(argv) == 2 {
			dir = titlePath(dir, argv[1])
			continue
		}
		if argv[0] == "cat" && len(argv) == 1 && inputs[i].HasStdin && inputs[i].Out != "" {
			files[titlePath(dir, inputs[i].Out)] = inputs[i].Stdin
			continue
		}
		if filepath.Base(argv[0]) != "ask" {
			continue
		}
		title, opts := invocationTitle(a, argv, description, dir, inputs[i], files)
		if title != "" && !opts.B("--no-hooks") {
			var notes []hooks.Note
			title, notes = hooks.New(a.Paths).Title(title, argv, description, dir)
			for _, n := range notes {
				fmt.Fprintf(os.Stderr, "ask: hook %s · %s\n", n.Name, strings.Join(strings.Fields(n.Text), " "))
			}
		}
		return title
	}
	return ""
}

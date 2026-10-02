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

// shellCommands splits literal simple commands without executing expansions or shell code.
// Unsupported syntax and incomplete quotes, escapes or redirects make the whole input unusable.
func shellCommands(s string) ([][]string, bool) {
	commands := [][]string{}
	words := []string{}
	var word strings.Builder
	active, redirect, skip, descriptor := false, false, false, true
	quote := byte(0)
	flush := func() {
		if active {
			if skip {
				skip = false
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
			words = nil
		}
		return true
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
				return nil, false
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
					return nil, false
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
			return nil, false
		case '<', '>':
			if active && descriptor && allDigits(word.String()) {
				word.Reset()
				active = false
			} else {
				flush()
			}
			if skip {
				return nil, false
			}
			skip = true
			if i+1 < len(s) && (s[i+1] == c || s[i+1] == '&' || s[i+1] == '|') {
				i++
				if c == '<' && s[i] == '<' {
					return nil, false
				}
			}
		case ';', '|', '&', '\n':
			if c == '\n' && redirect {
				continue
			}
			if (c == '|' || c == '&' || c == ';') && len(words) == 0 && !active {
				return nil, false
			}
			if !finish() {
				return nil, false
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
	if quote != 0 || !finish() {
		return nil, false
	}
	return commands, true
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
func invocationTitle(a *agent.Registry, argv []string, description, dir string) (string, home.Object) {
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
			if len(words) == 1 && words[0] != "-" {
				if b, err := os.ReadFile(titlePath(invocationDir, words[0])); err == nil {
					if items, err := readTasks(string(b)); err == nil {
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
	commands, ok := shellCommands(command)
	if !ok {
		return ""
	}
	dir, _ := os.Getwd()
	for _, argv := range commands {
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
		if filepath.Base(argv[0]) != "ask" {
			continue
		}
		title, opts := invocationTitle(a, argv, description, dir)
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

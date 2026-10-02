// Package cli implements ask's commands, option validation, stdout/stderr contract and exit codes.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fschrhunt/ask/internal/agent"
	"github.com/fschrhunt/ask/internal/find"
	"github.com/fschrhunt/ask/internal/home"
	"github.com/fschrhunt/ask/internal/packages"
	"github.com/fschrhunt/ask/internal/process"
	"github.com/fschrhunt/ask/internal/runs"
	"github.com/fschrhunt/ask/internal/status"
)

var options = map[string][]string{
	"run":   {"-m", "-r", "-w", "--worktree", "-c", "--json", "--schema", "-C", "-t", "--no-hooks"},
	"batch": {"-m", "-r", "-w", "--worktree", "--json", "--schema", "-C", "-t", "-j", "--resume", "--no-hooks"},
	"title": {"--command", "--description"},
	"show":  {"--json"}, "runs": {"-n"}, "stop": {}, "models": {"--names"}, "help": {}, "install": {}, "packages": {}, "remove": {},
}
var long = map[string]string{"--model": "-m", "--read": "-r", "--write": "-w", "--continue": "-c", "--dir": "-C", "--timeout": "-t"}
var value = map[string]bool{"-m": true, "-c": true, "--schema": true, "-C": true, "-t": true, "-j": true, "-n": true, "--resume": true, "--command": true, "--description": true}

// suggestion returns a valid option only when the spelling is one edit away.
func suggestion(command, wrong string) string {
	choices := append([]string{"--help", "-h", "--version", "-V"}, options[command]...)
	for alias, short := range long {
		if has(options[command], short) {
			choices = append(choices, alias)
		}
	}
	for _, right := range choices {
		if oneEdit(wrong, right) {
			return right
		}
	}
	return ""
}

// oneEdit recognizes one inserted, removed, changed or transposed character.
func oneEdit(a, b string) bool {
	if a == b {
		return false
	}
	if len(a) > len(b) {
		a, b = b, a
	}
	if len(b)-len(a) > 1 {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] == b[i] {
			continue
		}
		if len(a) == len(b) {
			return a[i+1:] == b[i+1:] || i+1 < len(a) && a[i] == b[i+1] && a[i+1] == b[i] && a[i+2:] == b[i+2:]
		}
		return a[i:] == b[i+1:]
	}
	return len(b) == len(a)+1
}

// has tests membership in a command's accepted option list.
func has(list []string, item string) bool {
	for _, x := range list {
		if x == item {
			return true
		}
	}
	return false
}

// number validates a numeric option and reports the original text on failure.
func number(flag, text string, whole bool) (float64, error) {
	n := home.Number(text)
	if !(n > 0) || (whole && (math.IsInf(n, 0) || math.Trunc(n) != n)) {
		kind := "number"
		if whole {
			kind = "whole number"
		}
		return 0, home.Usage("%s needs a %s above 0, not \"%s\"", flag, kind, text)
	}
	return n, nil
}

// integer bounds allocation counts without truncating the set of available tasks or runs.
func integer(n float64) int {
	if n > float64(math.MaxInt32) {
		return math.MaxInt32
	}
	return int(n)
}

// parse validates options per command and treats every word after -- as literal input.
func parse(command string, argv []string) (home.Object, []string, error) {
	opts := home.Object{}
	words := []string{}
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		if arg == "--" {
			words = append(words, argv[i+1:]...)
			break
		}
		if arg == "-h" || arg == "--help" {
			opts.Set("help", true)
		} else if strings.HasPrefix(arg, "-") && arg != "-" {
			flag := arg
			if alias, ok := long[arg]; ok {
				flag = alias
			}
			if !has(options[command], flag) {
				if right := suggestion(command, arg); right != "" {
					return nil, nil, home.Usage("unknown option %s; did you mean %s?", arg, right)
				}
				return nil, nil, home.Usage("unknown option %s", arg)
			}
			if !value[flag] {
				opts.Set(flag, true)
			} else if i+1 >= len(argv) {
				return nil, nil, home.Usage("missing value for %s", arg)
			} else {
				i++
				opts.Set(flag, argv[i])
			}
		} else {
			words = append(words, arg)
		}
	}
	if opts.B("-r") && opts.B("-w") {
		return nil, nil, home.Usage("choose one of -r (read) or -w (read and write)")
	}
	return opts, words, nil
}

// taskOptions reads the task fields shared by single and batch commands.
func taskOptions(opts home.Object) (home.Object, error) {
	t := home.Object{}
	if opts.B("-m") {
		t.Set("model", opts.Get("-m"))
	}
	if opts.B("-r") || opts.B("-w") {
		t.Set("write", opts.B("-w"))
	}
	if opts.B("--worktree") {
		t.Set("worktree", true)
	}
	if opts.B("--json") {
		t.Set("json", true)
	}
	if opts.B("--schema") {
		text, e := home.ReadInput(opts.S("--schema"), "schema")
		if e != nil {
			return nil, e
		}
		schema, e := home.ParseJSON(text)
		if e != nil {
			return nil, home.Usage("cannot parse schema %s: %s", opts.S("--schema"), e)
		}
		_ = schema
		t.Set("schema", json.RawMessage(text))
	}
	if opts.B("-C") {
		t.Set("dir", opts.Get("-C"))
	}
	if opts.B("-t") {
		n, e := number("-t", opts.S("-t"), false)
		if e != nil {
			return nil, e
		}
		t.Set("timeout", n)
	}
	return t, nil
}

// readTasks accepts a JSON array or nonblank JSON lines, with source-specific diagnostics.
func readTasks(text string) ([]any, error) {
	trimmed := home.Trim(text)
	if trimmed == "" {
		return nil, home.Usage("no tasks: pass a JSON array or JSON lines in FILE or on stdin")
	}
	decode := func(text, where string) (any, error) {
		v, e := home.ParseJSON(text)
		if e != nil {
			return nil, home.Usage("cannot parse %s: %s", where, e)
		}
		return v, nil
	}
	items := []any{}
	if strings.HasPrefix(trimmed, "[") {
		v, e := decode(trimmed, "the batch")
		if e != nil {
			return nil, e
		}
		items, _ = v.([]any)
	} else {
		for i, line := range strings.Split(trimmed, "\n") {
			if home.Trim(line) == "" {
				continue
			}
			v, e := decode(line, fmt.Sprintf("line %d", i+1))
			if e != nil {
				return nil, e
			}
			items = append(items, v)
		}
	}
	if len(items) == 0 {
		return nil, home.Usage("no tasks: the batch is empty")
	}
	return items, nil
}

// prepare validates task fields and resolves follow-ups without starting model processes.
func prepare(a *agent.Registry, items []any, defaults home.Object, id string, single bool) ([]home.Object, error) {
	return runs.Prepare(a.Paths, items, defaults, id, single, func(where string) error { return home.Usage("%s needs a model (-m)", where) })
}

// reporter serializes complete status lines from parallel tasks to stderr.
func reporter(a *agent.Registry, r *runs.Run, live *status.Live) func(runs.Event) {
	var mu sync.Mutex
	return func(e runs.Event) {
		mu.Lock()
		defer mu.Unlock()
		if live == nil {
			fmt.Fprintln(os.Stderr, status.StyledLine(status.EventLine(r, e), os.Stderr))
			return
		}
		name := ""
		if e.Kind == "start" {
			name = e.Started.Name
		}
		live.Update(e, name, status.EventLine(r, e))
	}
}

// answerText prints JSON task values as JSON and ordinary answers as text.
func answerText(t, r home.Object) string {
	if t.B("json") || t.Has("schema") && t.Get("schema") != nil {
		return home.JSON(r.Get("answer"), false)
	}
	answer := r.Get("answer")
	if raw, ok := answer.(json.RawMessage); ok {
		var text string
		if json.Unmarshal(raw, &text) == nil {
			return text
		}
		return string(raw)
	}
	return home.String(answer)
}

// runOne records and executes one prompt, printing only a successful answer.
func runOne(a *agent.Registry, opts home.Object, words []string) (int, error) {
	prompt := strings.Join(words, " ")
	if prompt == "" || prompt == "-" {
		if stdinTerminal() {
			return 0, home.Usage("no prompt: give it as an argument, or pipe it in")
		}
		var e error
		prompt, e = home.ReadInput("-", "prompt")
		if e != nil {
			return 0, e
		}
	}
	id := runs.NewID()
	t, e := taskOptions(opts)
	if e != nil {
		return 0, e
	}
	t.Set("prompt", prompt)
	if opts.Has("-c") {
		t.Set("continue", opts.Get("-c"))
	}
	tasks, e := prepare(a, []any{t}, nil, id, true)
	if e != nil {
		return 0, e
	}
	r, e := runs.Create(a.Paths, id, tasks)
	if e != nil {
		return 0, e
	}
	live := status.NewLive(r, "", false)
	process.OnStop(func() { live.Finish("ask "+r.ID+" · stopped", true) })
	defer process.OnStop(nil)
	results, e := runs.Execute(a, r, 1, !opts.B("--no-hooks"), reporter(a, r, live))
	final := ""
	if e == nil {
		final = status.Done(results[0])
	}
	live.Finish(final, false)
	if e != nil {
		return 0, e
	}
	if !results[0].B("ok") {
		return 1, nil
	}
	fmt.Fprintln(os.Stdout, answerText(tasks[0], results[0]))
	return 0, nil
}

// batch records or resumes tasks and prints all results in task order.
func batch(a *agent.Registry, opts home.Object, words []string) (int, error) {
	jobs := 4
	if opts.B("-j") {
		n, e := number("-j", opts.S("-j"), true)
		if e != nil {
			return 0, e
		}
		jobs = integer(n)
	}
	var r *runs.Run
	var e error
	if opts.B("--resume") {
		extra := []string{}
		for key := range opts {
			if !has([]string{"--resume", "-j", "--no-hooks"}, key) {
				extra = append(extra, key)
			}
		}
		extra = append(extra, words...)
		if len(extra) > 0 {
			return 0, home.Usage("--resume reruns a batch as it was recorded; it takes only -j and --no-hooks, not %s", strings.Join(extra, " "))
		}
		r, _, e = runs.Open(a.Paths, opts.S("--resume"))
		if e != nil {
			return 0, e
		}
	} else {
		if len(words) > 1 {
			return 0, home.Usage("ask batch takes one file of tasks, not %d", len(words))
		}
		path := "-"
		if len(words) > 0 && words[0] != "" {
			path = words[0]
		}
		text, e := home.ReadInput(path, "tasks")
		if e != nil {
			return 0, e
		}
		items, e := readTasks(text)
		if e != nil {
			return 0, e
		}
		defaults, e := taskOptions(opts)
		if e != nil {
			return 0, e
		}
		id := runs.NewID()
		tasks, e := prepare(a, items, defaults, id, false)
		if e != nil {
			return 0, e
		}
		r, e = runs.Create(a.Paths, id, tasks)
		if e != nil {
			return 0, e
		}
	}
	todo := 0
	for _, x := range r.Results {
		if !x.B("ok") {
			todo++
		}
	}
	begin := time.Now()
	header := status.BatchStart(r, jobs, todo)
	live := status.NewLive(r, header, true)
	if live == nil {
		fmt.Fprintln(os.Stderr, status.StyledLine(header, os.Stderr))
	}
	process.OnStop(func() { live.Finish("ask "+r.ID+" · stopped", true) })
	defer process.OnStop(nil)
	results, e := runs.Execute(a, r, jobs, !opts.B("--no-hooks"), reporter(a, r, live))
	summary := status.BatchEnd(r, float64(time.Since(begin).Milliseconds())/1000)
	live.Finish(summary, false)
	if e != nil {
		return 0, e
	}
	if live == nil {
		fmt.Fprintln(os.Stderr, status.StyledLine(summary, os.Stderr))
	}
	fmt.Fprintln(os.Stdout, home.JSON(home.ResultRecords(results), true))
	for _, x := range results {
		if !x.B("ok") {
			return 1, nil
		}
	}
	return 0, nil
}

// show prints a saved answer or complete result without running its agent again.
func show(p home.Paths, opts home.Object, words []string) (int, error) {
	if len(words) != 1 {
		return 0, home.Usage("ask show takes one run, like ask show k3f9a2")
	}
	r, i, e := runs.Open(p, words[0])
	if e != nil {
		return 0, e
	}
	if i < 0 {
		fmt.Fprintln(os.Stderr, status.StyledLine(status.BatchSaved(r), os.Stderr))
		fmt.Fprintln(os.Stdout, home.JSON(home.ResultRecords(r.Results), true))
		return 0, nil
	}
	result := r.Results[i]
	if result == nil {
		return 0, home.Usage("%s has not finished; see `ask runs`", words[0])
	}
	fmt.Fprintln(os.Stderr, status.StyledLine(status.Done(result), os.Stderr))
	if opts.B("--json") {
		fmt.Fprintln(os.Stdout, home.JSON(home.ResultRecords([]home.Object{result})[0], true))
	} else if result.B("ok") {
		fmt.Fprintln(os.Stdout, answerText(r.Tasks[i], result))
	}
	if !result.B("ok") {
		return 1, nil
	}
	return 0, nil
}

// stop resolves a run and reports whether its owner was stopped.
func stop(p home.Paths, words []string) (int, error) {
	if len(words) != 1 {
		return 0, home.Usage("ask stop takes one run, like ask stop k3f9a2")
	}
	r, _, e := runs.Open(p, words[0])
	if e != nil {
		return 0, e
	}
	stopped, e := runs.Stop(r)
	if e != nil {
		return 0, e
	}
	if stopped {
		fmt.Fprintf(os.Stderr, "ask %s · stopped\n", r.ID)
		return 0, nil
	}
	fmt.Fprintf(os.Stderr, "ask %s · not running\n", r.ID)
	return 1, nil
}

// listRuns prints the newest readable records as a status table.
func listRuns(p home.Paths, opts home.Object) (int, error) {
	limit := 20
	if opts.B("-n") {
		n, e := number("-n", opts.S("-n"), true)
		if e != nil {
			return 0, e
		}
		limit = integer(n)
	}
	list := runs.Recent(p, limit)
	if len(list) == 0 {
		fmt.Fprintln(os.Stderr, "ask: no runs yet")
	} else {
		fmt.Fprintln(os.Stdout, status.Table(list, time.Now(), status.CanStyle(os.Stdout)))
	}
	return 0, nil
}

// models lists installed model ids and reports declaration failures without failing the command.
func models(a *agent.Registry, names bool) (int, error) {
	config, e := a.Paths.ReadModels()
	if e != nil {
		return 0, e
	}
	ids, errors := a.List(config)
	for _, e := range errors {
		fmt.Fprintln(os.Stderr, "ask: could not list the models of "+e)
	}
	if len(ids) == 0 {
		fmt.Fprintf(os.Stderr, "ask: no models; add an agent to %s (see docs/agents.md)\n", a.Paths.Agents)
	}
	previous := ""
	for _, id := range ids {
		if names {
			m, _ := agent.Parse(a.Paths, id)
			fmt.Fprintf(os.Stdout, "%s\t%s\n", id, a.Name(m, ""))
		} else if status.IsTerminal(os.Stdout) {
			owner, model, _ := strings.Cut(id, ":")
			if owner != previous {
				if previous != "" {
					fmt.Fprintln(os.Stdout)
				}
				if status.CanStyle(os.Stdout) {
					fmt.Fprintf(os.Stdout, "\x1b[1m%s\x1b[0m\n", owner)
				} else {
					fmt.Fprintln(os.Stdout, owner)
				}
				previous = owner
			}
			m, _ := agent.Parse(a.Paths, id)
			label := fmt.Sprintf("  %-24s %s", model, a.Name(m, ""))
			if status.CanStyle(os.Stdout) {
				label = fmt.Sprintf("  %-24s \x1b[2m%s\x1b[0m", model, a.Name(m, ""))
			}
			fmt.Fprintln(os.Stdout, label)
		} else {
			fmt.Fprintln(os.Stdout, id)
		}
	}
	return 0, nil
}

// install clones one source or updates every package, retaining individual update failures.
func install(p home.Paths, words []string) (int, error) {
	if len(words) > 1 {
		return 0, home.Usage("ask install takes one source, like ask install owner/repo")
	}
	if len(words) > 0 {
		line, e := packages.Install(p, words[0])
		if e != nil {
			return 0, e
		}
		fmt.Fprintln(os.Stderr, "ask: "+line)
		return 0, nil
	}
	all := packages.Installed(p)
	if len(all) == 0 {
		fmt.Fprintln(os.Stderr, "ask: no packages installed; ask install OWNER/REPO adds one")
	}
	code := 0
	for _, x := range all {
		line, e := packages.Update(p, x.Path)
		if e != nil {
			fmt.Fprintf(os.Stderr, "ask: %s: could not update: %s\n", x.Name, e)
			code = 1
		} else {
			fmt.Fprintln(os.Stderr, "ask: "+line)
		}
	}
	return code, nil
}

// listPackages prints each installation and its directory contents.
func listPackages(p home.Paths) (int, error) {
	all := packages.Installed(p)
	if len(all) == 0 {
		fmt.Fprintln(os.Stderr, "ask: no packages installed; ask install OWNER/REPO adds one")
	}
	width := 0
	for _, x := range all {
		if len(x.Name) > width {
			width = len(x.Name)
		}
	}
	for _, x := range all {
		fmt.Fprintf(os.Stdout, "%-*s  %s\n", width, x.Name, packages.Contents(x.Path))
	}
	return 0, nil
}

// remove validates one package argument and removes its unambiguous installation.
func remove(p home.Paths, words []string) (int, error) {
	if len(words) != 1 {
		return 0, home.Usage("ask remove takes one package, like ask remove owner/repo")
	}
	line, e := packages.Remove(p, words[0])
	if e != nil {
		return 0, e
	}
	fmt.Fprintln(os.Stderr, "ask: "+line)
	return 0, nil
}

// runCommand gives a user executable the terminal streams, arguments and contract environment.
func runCommand(p home.Paths, path string, args []string) (int, error) {
	cmd := exec.Command(path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = p.Env(nil)
	e := cmd.Run()
	if e == nil {
		return 0, nil
	}
	if cmd.ProcessState != nil {
		if status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return 128 + int(status.Signal()), nil
		}
		return cmd.ProcessState.ExitCode(), nil
	}

	fmt.Fprintf(os.Stderr, "ask: cannot run %s: %s\n", path, e.Error())
	return 1, nil
}

// main selects built-in or user commands before parsing command-specific options.
func main(argv []string, version string) (int, error) {
	p := home.New()
	if len(argv) == 1 && (argv[0] == "--version" || argv[0] == "-V") {
		fmt.Fprintln(os.Stdout, version)
		return 0, nil
	}
	command := "run"
	if len(argv) > 0 {
		if _, ok := options[argv[0]]; ok && argv[0] != "run" {
			command = argv[0]
		}
		if command == "run" && !strings.HasPrefix(argv[0], "-") {
			if path := find.Path(p, "commands", argv[0]); path != "" {
				return runCommand(p, path, argv[1:])
			}
		}
	}
	args := argv
	if command != "run" {
		args = argv[1:]
	}
	opts, words, e := parse(command, args)
	if e != nil {
		return 0, e
	}
	if len(argv) == 0 || command == "run" && len(argv) == 1 && opts.B("help") || command == "help" && len(words) == 0 {
		printHelp(overview(agent.New(p)))
		return 0, nil
	}
	if opts.B("help") {
		topic := command
		if topic == "help" {
			topic = "run"
		}
		printHelp(topicHelp(topic))
		return 0, nil
	}
	if has([]string{"models", "runs", "packages"}, command) && len(words) != 0 {
		return 0, home.Usage("ask %s takes no arguments", command)
	}
	a := agent.New(p)
	switch command {
	case "title":
		if len(words) != 0 || !opts.Has("--command") {
			return 0, home.Usage("ask title needs --command STRING and optional --description TEXT")
		}
		if title := commandTitle(a, opts.S("--command"), opts.S("--description")); title != "" {
			fmt.Fprintln(os.Stdout, title)
		}
		return 0, nil
	case "batch":
		return batch(a, opts, words)
	case "show":
		return show(p, opts, words)
	case "stop":
		return stop(p, words)
	case "runs":
		return listRuns(p, opts)
	case "models":
		return models(a, opts.B("--names"))
	case "help":
		if len(words) != 1 {
			return 0, home.Usage("ask help takes one command")
		}
		text := topicHelp(words[0])
		if text == "" {
			return 0, home.Usage("unknown help topic %q", words[0])
		}
		printHelp(text)
		return 0, nil
	case "install":
		return install(p, words)
	case "packages":
		return listPackages(p)
	case "remove":
		return remove(p, words)
	default:
		return runOne(a, opts, words)
	}
}

// Main runs one CLI invocation and reports errors with the original exit-status contract.
func Main(argv []string, version string) int {
	process.Listen()
	defer finishStdin()
	code, e := main(argv, version)
	process.AwaitShutdown()
	if e != nil {
		var usage *home.UsageError
		if errors.As(e, &usage) {
			topic := "run"
			if len(argv) > 0 {
				if _, ok := options[argv[0]]; ok {
					topic = argv[0]
				}
			}
			message := strings.Join(strings.Fields(e.Error()), " ")
			if message == "unknown option -help; did you mean --help?" || message == "unknown option -version; did you mean --version?" {
				fmt.Fprintf(os.Stderr, "ask: %s\n", message)
			} else {
				fmt.Fprintf(os.Stderr, "ask: %s; see ask %s --help\n", message, topic)
			}
			if strings.Contains(e.Error(), "needs a model") || strings.Contains(e.Error(), "bad model") || strings.Contains(e.Error(), "no agent") {
				fmt.Fprintln(os.Stderr, modelLines(agent.New(home.New())))
			}
			return 2
		}
		fmt.Fprintln(os.Stderr, "ask: "+e.Error())
		return 1
	}
	return code
}

// finishStdin lets pipe writers close cleanly before a fast command exits, without waiting on long-lived input.
func finishStdin() {
	if stdinTerminal() {
		return
	}
	done := make(chan struct{})
	go func() { _, _ = io.Copy(io.Discard, os.Stdin); close(done) }()
	select {
	case <-done:
	case <-time.After(25 * time.Millisecond):
	}
}

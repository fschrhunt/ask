// Package cli implements ask's commands, option validation, stdout/stderr contract and exit codes.
package cli

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"regexp"
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
	"show":  {"--json"}, "runs": {"-n"}, "stop": {}, "models": {}, "install": {}, "packages": {}, "remove": {},
}
var long = map[string]string{"--model": "-m", "--read": "-r", "--write": "-w", "--continue": "-c", "--dir": "-C", "--timeout": "-t"}
var value = map[string]bool{"-m": true, "-c": true, "--schema": true, "-C": true, "-t": true, "-j": true, "-n": true, "--resume": true}

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
				prefix := ""
				if command != "run" {
					prefix = "ask " + command + ": "
				}
				return nil, nil, home.Usage("%sunknown option %s", prefix, arg)
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
		t.Set("schema", schema)
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

// prepare lists available models only when needed, then validates and resolves tasks.
func prepare(a *agent.Registry, items []any, defaults home.Object, id string, single bool) ([]home.Object, error) {
	lacking := false
	for _, v := range items {
		t, _ := v.(home.Object)
		if !t.B("model") && !defaults.B("model") && !t.B("continue") {
			lacking = true
		}
	}
	ids := []string{}
	if lacking {
		config, e := a.Paths.ReadModels()
		if e != nil {
			return nil, e
		}
		var fatal error
		ids, _, fatal = a.List(config)
		if fatal != nil {
			return nil, fatal
		}
	}
	return runs.Prepare(a.Paths, items, defaults, id, single, func(where string) error {
		available := strings.Join(ids, "\n  ")
		if available == "" {
			available = "none; add an agent to " + a.Paths.Agents
		}
		return home.Usage("%s needs a model (-m); available:\n  %s", where, available)
	})
}

// reporter serializes complete status lines from parallel tasks to stderr.
func reporter(r *runs.Run) func(runs.Event) {
	var mu sync.Mutex
	return func(e runs.Event) { mu.Lock(); defer mu.Unlock(); fmt.Fprintln(os.Stderr, status.EventLine(r, e)) }
}

// answerText prints JSON task values as JSON and ordinary answers as text.
func answerText(t, r home.Object) string {
	if t.B("json") || t.B("schema") {
		return home.JSON(r.Get("answer"), false)
	}
	return home.String(r.Get("answer"))
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
	results, e := runs.Execute(a, r, 1, !opts.B("--no-hooks"), reporter(r))
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
		for _, f := range opts {
			if !has([]string{"--resume", "-j", "--no-hooks"}, f.Key) {
				extra = append(extra, f.Key)
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
	fmt.Fprintln(os.Stderr, status.BatchStart(r, jobs, todo))
	results, e := runs.Execute(a, r, jobs, !opts.B("--no-hooks"), reporter(r))
	if e != nil {
		return 0, e
	}
	fmt.Fprintln(os.Stderr, status.BatchEnd(r, float64(time.Since(begin).Milliseconds())/1000))
	fmt.Fprintln(os.Stdout, home.JSON(results, true))
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
		fmt.Fprintln(os.Stdout, home.JSON(r.Results, true))
		return 0, nil
	}
	result := r.Results[i]
	if result == nil {
		return 0, home.Usage("%s has not finished; see `ask runs`", words[0])
	}
	fmt.Fprintln(os.Stderr, status.Done(result))
	if opts.B("--json") {
		fmt.Fprintln(os.Stdout, home.JSON(result, true))
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
		fmt.Fprintln(os.Stdout, status.Table(list))
	}
	return 0, nil
}

// models lists installed model ids and reports declaration failures without failing the command.
func models(a *agent.Registry) (int, error) {
	config, e := a.Paths.ReadModels()
	if e != nil {
		return 0, e
	}
	ids, errors, fatal := a.List(config)
	if fatal != nil {
		return 0, fatal
	}
	for _, e := range errors {
		fmt.Fprintln(os.Stderr, "ask: could not list the models of "+e)
	}
	if len(ids) == 0 {
		fmt.Fprintf(os.Stderr, "ask: no models; add an agent to %s (see docs/agents.md)\n", a.Paths.Agents)
	}
	for _, id := range ids {
		fmt.Fprintln(os.Stdout, id)
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
	for _, x := range all {
		fmt.Fprintf(os.Stdout, "%s\t%s\n", x.Name, packages.Contents(x.Path))
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

var description = regexp.MustCompile(`ask-command:\s*(.+)`)

// help appends descriptions of discovered commands to the stable built-in help.
func help(p home.Paths) string {
	commands := find.Sorted(p, "commands")
	if len(commands) == 0 {
		return helpText
	}
	width := 0
	for _, c := range commands {
		if len(c.Name) > width {
			width = len(c.Name)
		}
	}
	lines := []string{}
	for _, c := range commands {
		desc := ""
		if b, e := os.ReadFile(c.Path); e == nil {
			s := string(b)
			if len(s) > 4096 {
				s = s[:4096]
			}
			if m := description.FindStringSubmatch(s); m != nil {
				desc = home.Trim(m[1])
			}
		}
		lines = append(lines, strings.TrimRight(fmt.Sprintf("  ask %-*s  %s", width, c.Name, desc), " "))
	}
	return helpText + "\n\nyour commands\n" + strings.Join(lines, "\n")
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
	if errors.Is(e, syscall.ENOEXEC) || errors.Is(e, syscall.ENOTDIR) {
		return 0, fmt.Errorf("%s", process.SpawnError(path, e))
	}
	fmt.Fprintf(os.Stderr, "ask: cannot run %s: %s\n", path, process.SpawnError(path, e))
	return 1, nil
}

// main selects built-in or user commands before parsing command-specific options.
func main(argv []string, version string) (int, error) {
	p := home.New()
	if len(argv) == 1 && argv[0] == "--version" {
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
	if opts.B("help") || len(argv) == 0 {
		fmt.Fprintln(os.Stdout, help(p))
		return 0, nil
	}
	a := agent.New(p)
	switch command {
	case "batch":
		return batch(a, opts, words)
	case "show":
		return show(p, opts, words)
	case "stop":
		return stop(p, words)
	case "runs":
		return listRuns(p, opts)
	case "models":
		return models(a)
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
		fmt.Fprintln(os.Stderr, "ask: "+e.Error())
		if errors.As(e, &usage) {
			fmt.Fprintln(os.Stderr, "run `ask --help`")
			return 2
		}
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

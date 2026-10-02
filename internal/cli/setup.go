package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fschrhunt/ask/internal/agent"
	"github.com/fschrhunt/ask/internal/find"
	"github.com/fschrhunt/ask/internal/home"
	"github.com/fschrhunt/ask/internal/packages"
	"github.com/fschrhunt/ask/internal/setup"
	"github.com/fschrhunt/ask/internal/status"
	"github.com/fschrhunt/ask/internal/tui"
)

// agentState is one official agent here: its CLI, whether an agent of its name is installed,
// and whether that agent can run.
type agentState struct {
	setup.Agent
	CLI, Path, Why string
	Installed      bool
	Ready          bool
	Models         string
}

// machine is everything ask setup reports and changes.
type machine struct {
	Agents   []agentState
	Settings home.Object
	Hosts    []setup.Host
	Skills   map[string]string
	Hook     bool
}

// survey reads the machine's state: the official agents, then any other agent installed; checking
// agents runs each installed one's models command.
func survey(p home.Paths) (*machine, error) {
	s, e := p.ReadSettings()
	if e != nil {
		return nil, e
	}
	m := &machine{Settings: s, Skills: map[string]string{}, Hook: setup.HookOn()}
	all := append([]setup.Agent{}, setup.Agents...)
	for _, x := range find.Sorted(p, "agents") {
		official := false
		for _, a := range setup.Agents {
			official = official || a.Name == x.Name
		}
		if !official {
			all = append(all, setup.Agent{Name: x.Name, Title: x.Name})
		}
	}
	for _, a := range all {
		x := agentState{Agent: a, Path: find.Path(p, "agents", a.Name)}
		if a.CLI != "" {
			x.CLI = setup.FindCLI(a.CLI)
		}
		x.Installed = x.Path != ""
		if x.Installed {
			x.Ready, x.Why = readiness(p, a.Name, x.Path)
			if x.Ready {
				x.Models, x.Why = strings.TrimPrefix(strings.TrimSuffix(x.Why, ", see ask models"), ": "), ""
				if strings.HasPrefix(x.Models, ";") {
					x.Models = "names models as you use them"
				}
			}
		}
		m.Agents = append(m.Agents, x)
	}
	for _, h := range setup.Hosts() {
		if h.Present() {
			m.Hosts = append(m.Hosts, h)
			m.Skills[h.Name] = setup.SkillState(h)
		}
	}
	return m, nil
}

// settingText shows a setting's value with unit, or its built-in default marked as such.
func settingText(s home.Object, key string, unit ...string) string {
	defaults := map[string]string{"model": "none: give -m", "timeout": "900", "jobs": "4", "worktrees": "~/.ask/worktrees/{name}", "branches": "ask/{name}"}
	text, suffix := defaults[key], " (default)"
	if s.Has(key) {
		text, suffix = home.String(s.Get(key)), ""
	}
	if len(unit) > 0 && !(key == "model" && !s.Has(key)) {
		text += " " + unit[0]
	}
	return text + suffix
}

// report prints the machine's state for people, or as JSON with asJSON.
func report(w io.Writer, p home.Paths, m *machine, asJSON bool) {
	if asJSON {
		agents := []any{}
		for _, a := range m.Agents {
			agents = append(agents, home.O("name", a.Name, "cli", a.CLI, "installed", a.Installed, "ready", a.Ready, "models", a.Models, "reason", a.Why))
		}
		skills := home.Object{}
		for k, v := range m.Skills {
			skills[k] = v
		}
		fmt.Fprintln(w, home.JSON(home.O("agents", agents, "settings", m.Settings, "skills", skills, "hook", m.Hook), true))
		return
	}
	color := status.CanStyle(os.Stdout)
	dim := func(s string) string {
		if color {
			return "\x1b[2m" + s + "\x1b[0m"
		}
		return s
	}
	mark := func(ok bool) string {
		switch {
		case ok && color:
			return "\x1b[32m✓\x1b[0m"
		case ok:
			return "✓"
		case color:
			return "\x1b[2m•\x1b[0m"
		}
		return "•"
	}
	fmt.Fprintln(w, "Agents")
	for _, a := range m.Agents {
		text := ""
		switch {
		case a.Ready:
			text = "ready · " + a.Models
		case a.Installed:
			text = "not ready: " + a.Why
		case a.CLI != "":
			text = dim(a.Title + " is installed; ask setup --agents " + a.Name + " connects it")
		default:
			text = dim("not installed")
		}
		fmt.Fprintf(w, "  %s %-9s %s\n", mark(a.Ready), a.Name, text)
	}
	fmt.Fprintf(w, "\nSettings %s\n", dim(home.Tilde(filepath.Join(p.Home, "settings.json"))))
	for _, k := range home.Settings {
		fmt.Fprintf(w, "  %-10s %s\n", k, settingText(m.Settings, k))
	}
	fmt.Fprintln(w, "\nThe ask skill")
	if len(m.Hosts) == 0 {
		fmt.Fprintln(w, dim("  no app that reads skills is installed"))
	}
	for _, h := range m.Hosts {
		state := map[string]string{"current": "added", "outdated": "added, an older version", "yours": "yours, left as it is", "missing": "not added"}[m.Skills[h.Name]]
		fmt.Fprintf(w, "  %s %-12s %s\n", mark(m.Skills[h.Name] != "missing"), h.Title, state)
	}
	if hasHost(m, "claude-code") {
		on := "off"
		if m.Hook {
			on = "on"
		}
		fmt.Fprintf(w, "\nTask titles in Claude Code  %s\n", on)
	}
}

// hasHost reports whether the named host is installed.
func hasHost(m *machine, name string) bool {
	for _, h := range m.Hosts {
		if h.Name == name {
			return true
		}
	}
	return false
}

// healthy is --check's verdict: some agent is ready and no installed agent is broken.
func healthy(m *machine) bool {
	ready := false
	for _, a := range m.Agents {
		if a.Installed && !a.Ready {
			return false
		}
		ready = ready || a.Ready
	}
	return ready
}

// installAgent installs an official agent and says how it went, in the TUI when there is one.
func installAgent(p home.Paths, say func(string, string), name string) bool {
	line, e := packages.Install(p, name)
	if e != nil {
		say("✗", name+": "+e.Error())
		return false
	}
	_, dir, _ := packages.Locate(p, name)
	path := filepath.Join(dir, "agents", name)
	if used := find.Path(p, "agents", name); used != path {
		say("•", fmt.Sprintf("%s: %s, but your %s is used instead", name, line, home.Tilde(used)))
		return true
	}
	ok, text := readiness(p, name, path)
	if ok {
		say("✓", name+" is ready"+text)
	} else {
		say("!", name+" is installed but not ready: "+text)
	}
	return ok
}

// setSetting validates and saves one setting; an empty value returns it to the built-in default.
func setSetting(p home.Paths, key, value string) error {
	s, e := p.ReadSettings()
	if e != nil {
		return e
	}
	s = s.Clone()
	var v any = value
	if value == "" {
		v = nil
	} else if key == "timeout" || key == "jobs" {
		n, e := strconv.ParseFloat(value, 64)
		if e != nil {
			return home.Usage("%s must be a number, not %q", key, value)
		}
		v = n
	}
	if name, id, _ := strings.Cut(value, ":"); key == "model" && value != "" && (name == "" || id == "") {
		return home.Usage("model must be agent:id[#effort], like claude:sonnet-5.5, not %q", value)
	}
	s.Set(key, v)
	return p.WriteSettings(s)
}

// setupFlags applies ask setup's flags without prompting, for scripts and agents.
func setupFlags(p home.Paths, opts home.Object, say func(string, string)) (int, error) {
	code := 0
	if opts.Has("--agents") {
		for _, name := range strings.Split(opts.S("--agents"), ",") {
			if name = strings.TrimSpace(name); name != "" && !installAgent(p, say, name) {
				code = 1
			}
		}
	}
	for _, f := range [][2]string{{"-m", "model"}, {"-t", "timeout"}, {"-j", "jobs"}, {"--worktrees", "worktrees"}, {"--branches", "branches"}} {
		flag, key := f[0], f[1]
		if opts.Has(flag) {
			if e := setSetting(p, key, opts.S(flag)); e != nil {
				return 0, e
			}
			say("✓", key+": "+settingText(mustSettings(p), key))
			if name, _, _ := strings.Cut(opts.S(flag), ":"); key == "model" && name != "" && find.Path(p, "agents", name) == "" {
				say("!", "no "+name+" agent is installed yet; ask setup --agents "+name+" adds it")
			}
		}
	}
	if opts.Has("--skills") {
		want := map[string]bool{}
		for _, name := range strings.Split(opts.S("--skills"), ",") {
			want[strings.TrimSpace(name)] = true
		}
		known := map[string]bool{}
		for _, h := range setup.Hosts() {
			known[h.Name] = true
			if !want[h.Name] && !want["all"] {
				continue
			}
			if !h.Present() {
				if !want["all"] {
					say("!", h.Title+" is not installed here; no skill added")
				}
				continue
			}
			if setup.SkillState(h) == "yours" {
				say("•", h.Title+" has its own ask skill; left as it is")
			} else if e := setup.WriteSkill(h); e != nil {
				return 0, e
			} else {
				say("✓", "added the ask skill to "+h.Title)
			}
		}
		for name := range want {
			if name != "all" && name != "" && !known[name] {
				return 0, home.Usage("--skills takes claude-code, codex, opencode, cursor, pi or all, not %q", name)
			}
		}
	}
	if opts.B("--hook") || opts.B("--no-hook") {
		if e := setup.SetHook(opts.B("--hook")); e != nil {
			return 0, e
		}
		if opts.B("--hook") {
			say("✓", "Claude Code titles ask runs in its task list")
		} else {
			say("✓", "removed ask's title hook from Claude Code")
		}
	}
	return code, nil
}

// mustSettings reads settings that were just written and checked.
func mustSettings(p home.Paths) home.Object { s, _ := p.ReadSettings(); return s }

// recommended applies --yes: agents for the CLIs found, the skill where it is missing or old,
// and Claude Code's title hook.
func recommended(p home.Paths, say func(string, string)) (int, error) {
	m, e := survey(p)
	if e != nil {
		return 0, e
	}
	agents, skills := []string{}, []string{}
	for _, a := range m.Agents {
		if a.CLI != "" && !a.Installed {
			agents = append(agents, a.Name)
		}
	}
	for _, h := range m.Hosts {
		if st := m.Skills[h.Name]; st == "missing" || st == "outdated" {
			skills = append(skills, h.Name)
		}
	}
	opts := home.Object{}
	if len(agents) > 0 {
		opts.Set("--agents", strings.Join(agents, ","))
	}
	if len(skills) > 0 {
		opts.Set("--skills", strings.Join(skills, ","))
	}
	if hasHost(m, "claude-code") && !m.Hook {
		opts.Set("--hook", true)
	}
	if len(opts) == 0 {
		say("✓", "nothing to do; ask is set up")
	}
	return setupFlags(p, opts, say)
}

// setupCommand runs ask setup: flags apply without prompts; a terminal gets the walkthrough the
// first time and the settings screen after; anything else gets a status report.
func setupCommand(p home.Paths, opts home.Object) (int, error) {
	plain := func(mark, text string) { fmt.Fprintln(os.Stderr, mark+" "+text) }
	acting := false
	for _, f := range []string{"--agents", "-m", "-t", "-j", "--worktrees", "--branches", "--skills", "--hook", "--no-hook"} {
		acting = acting || opts.Has(f)
	}
	if opts.B("--hook") && opts.B("--no-hook") {
		return 0, home.Usage("ask setup takes --hook or --no-hook, not both")
	}
	if opts.B("--json") && !opts.B("--check") {
		return 0, home.Usage("--json goes with --check")
	}
	code := 0
	var e error
	switch {
	case opts.B("--yes"):
		code, e = recommended(p, plain)
	case acting:
		code, e = setupFlags(p, opts, plain)
	case !opts.B("--check") && status.IsTerminal(os.Stdin) && status.IsTerminal(os.Stdout):
		return interactive(p)
	}
	if e != nil {
		return 0, e
	}
	m, e := survey(p)
	if e != nil {
		return 0, e
	}
	if opts.B("--check") {
		report(os.Stdout, p, m, opts.B("--json"))
		if !healthy(m) {
			return 1, nil
		}
		return code, nil
	}
	if !acting && !opts.B("--yes") {
		report(os.Stdout, p, m, false)
		fmt.Fprintln(os.Stderr, "\nask: run ask setup in a terminal to change these, or see ask setup --help for flags")
	}
	return code, nil
}

// interactive runs the walkthrough or the settings screen on the terminal.
func interactive(p home.Paths) (int, error) {
	restore, e := tui.Raw(os.Stdin)
	if e != nil {
		return 0, e
	}
	defer restore()
	t := &tui.Prompter{In: os.Stdin, Out: os.Stdout, Color: status.CanStyle(os.Stdout)}
	m, e := survey(p)
	if e != nil {
		return 0, e
	}
	_, statErr := os.Stat(filepath.Join(p.Home, "settings.json"))
	first := os.IsNotExist(statErr)
	for _, a := range m.Agents {
		first = first && !a.Installed
	}
	if first {
		e = walkthrough(p, t, m)
	} else {
		e = settingsScreen(p, t)
	}
	if errors.Is(e, tui.ErrInterrupted) {
		fmt.Fprint(os.Stdout, "\r\n")
		return 130, nil
	}
	return 0, e
}

// walkthrough sets ask up the first time, one question at a time, applying each answer.
func walkthrough(p home.Paths, t *tui.Prompter, m *machine) error {
	fmt.Fprint(t.Out, t.Bold("Welcome to ask.")+" Let's connect your coding agents.\r\n\r\n")
	if e := chooseAgents(p, t, m); e != nil {
		return e
	}
	if e := chooseModel(p, t); e != nil {
		return e
	}
	if e := chooseWorktrees(p, t); e != nil {
		return e
	}
	m, _ = survey(p)
	if e := chooseSkills(t, m); e != nil {
		return e
	}
	if hasHost(m, "claude-code") && !m.Hook {
		on, e := t.Confirm("Show ask runs by model and task in Claude Code's task list?", true)
		if e != nil {
			return e
		}
		if on {
			if e := setup.SetHook(true); e != nil {
				return e
			}
			t.Say("✓", "Claude Code titles ask runs, like Sonnet 5.5 · Fix the login test")
		}
	}
	s := mustSettings(p)
	if e := p.WriteSettings(s); e != nil {
		return e
	}
	example := "ask -m MODEL \"What does this project do?\""
	if s.Has("model") {
		example = "ask \"What does this project do?\""
	}
	fmt.Fprint(t.Out, "\r\n"+t.Bold("ask is set up.")+" Try it in a project:\r\n\r\n  "+example+"\r\n\r\nRun ask setup again to change any of this.\r\n")
	return nil
}

// settingsScreen lets you change one thing at a time until you are done.
func settingsScreen(p home.Paths, t *tui.Prompter) error {
	for {
		m, e := survey(p)
		if e != nil {
			return e
		}
		ready := []string{}
		for _, a := range m.Agents {
			if a.Ready {
				ready = append(ready, a.Name)
			} else if a.Installed {
				ready = append(ready, a.Name+" (not ready)")
			}
		}
		skills := []string{}
		for _, h := range m.Hosts {
			if m.Skills[h.Name] != "missing" {
				skills = append(skills, h.Title)
			}
		}
		agentsText := strings.Join(ready, ", ")
		if agentsText == "" {
			agentsText = "none"
		}
		skillText := strings.Join(skills, ", ")
		if skillText == "" {
			skillText = "not added"
		}
		items := []tui.Option{
			{Label: "Agents", Note: agentsText},
			{Label: "Default model", Note: settingText(m.Settings, "model")},
			{Label: "Worktrees", Note: settingText(m.Settings, "worktrees")},
			{Label: "Branches", Note: settingText(m.Settings, "branches")},
			{Label: "Timeout", Note: settingText(m.Settings, "timeout", "seconds")},
			{Label: "Batch jobs", Note: settingText(m.Settings, "jobs", "at once")},
			{Label: "The ask skill", Note: skillText},
		}
		if hasHost(m, "claude-code") {
			on := "off"
			if m.Hook {
				on = "on"
			}
			items = append(items, tui.Option{Label: "Task titles in Claude Code", Note: on})
		}
		items = append(items, tui.Option{Label: "Done"})
		i, e := t.Select("What do you want to change?", items, 0)
		if e != nil {
			return e
		}
		switch items[i].Label {
		case "Agents":
			e = chooseAgents(p, t, m)
		case "Default model":
			e = chooseModel(p, t)
		case "Worktrees":
			e = chooseWorktrees(p, t)
		case "Branches":
			e = askSetting(p, t, "branches", "Branch name for a run's worktree, with {name}", "ask/{name}")
		case "Timeout":
			e = askSetting(p, t, "timeout", "Seconds a task may run", "900")
		case "Batch jobs":
			e = askSetting(p, t, "jobs", "Batch tasks at once", "4")
		case "The ask skill":
			e = chooseSkills(t, m)
		case "Task titles in Claude Code":
			on, err := t.Confirm("Show ask runs by model and task in Claude Code's task list?", !m.Hook)
			if err == nil {
				err = setup.SetHook(on)
			}
			e = err
		case "Done":
			return nil
		}
		if e != nil {
			if errors.Is(e, tui.ErrInterrupted) {
				return e
			}
			t.Say("✗", e.Error())
		}
	}
}

// chooseAgents installs the official agents you pick; installed ones stay, and a package you
// unpick is removed. Agents of your own are listed by ask setup --check, not offered here.
func chooseAgents(p home.Paths, t *tui.Prompter, m *machine) error {
	options, chosen := []tui.Option{}, []bool{}
	official := []agentState{}
	for _, a := range m.Agents {
		if a.Agent.CLI == "" {
			continue
		}
		official = append(official, a)
	}
	for _, a := range official {
		note := a.Title + " not found"
		if a.CLI != "" {
			note = a.Title + " · " + home.Tilde(a.CLI)
		}
		if a.Installed {
			note = "installed · " + note
		}
		options = append(options, tui.Option{Label: a.Name, Note: note})
		chosen = append(chosen, a.Installed || a.CLI != "")
	}
	picked, e := t.MultiSelect("Which coding agents should ask use?", options, chosen)
	if e != nil {
		return e
	}
	for i, a := range official {
		_, dir, _ := packages.Locate(p, a.Name)
		_, statErr := os.Stat(dir)
		official := statErr == nil
		switch {
		case picked[i]:
			installAgent(p, t.Say, a.Name)
		case official:
			line, e := packages.Remove(p, strings.TrimPrefix(packages.Official, "fschrhunt/")+a.Name)
			if e != nil {
				return e
			}
			t.Say("✓", line)
		}
	}
	return nil
}

// chooseModel picks the default model from what the installed agents list.
func chooseModel(p home.Paths, t *tui.Prompter) error {
	config, _ := p.ReadModels()
	ids, _ := agent.New(p).List(config)
	if len(ids) == 0 {
		return nil
	}
	current := mustSettings(p).S("model")
	options := []tui.Option{{Label: "None", Note: "give -m every time"}}
	at := 0
	a := agent.New(p)
	for i, id := range ids {
		m, _ := agent.Parse(p, id)
		options = append(options, tui.Option{Label: id, Note: a.Name(m, "")})
		if id == current {
			at = i + 1
		}
	}
	i, e := t.Select("Default model, when you don't give -m", options, at)
	if e != nil {
		return e
	}
	value := ""
	if i > 0 {
		value = ids[i-1]
	}
	return setSetting(p, "model", value)
}

// chooseWorktrees picks where --worktree works: ask's home, or a folder you name.
func chooseWorktrees(p home.Paths, t *tui.Prompter) error {
	current := mustSettings(p).S("worktrees")
	options := []tui.Option{{Label: "In ask's home", Note: "~/.ask/worktrees/{name}"}, {Label: "Somewhere else", Note: "like ~/code/worktrees/ask-{name}"}}
	at := 0
	if current != "" {
		at = 1
	}
	i, e := t.Select("Where should --worktree runs work?", options, at)
	if e != nil {
		return e
	}
	if i == 0 {
		return setSetting(p, "worktrees", "")
	}
	def := current
	if def == "" {
		def = "~/code/worktrees/ask-{name}"
	}
	return askSetting(p, t, "worktrees", "Folder for each worktree, with {name}", def)
}

// askSetting asks for one setting's value until it is valid; an empty answer keeps def.
func askSetting(p home.Paths, t *tui.Prompter, key, question, def string) error {
	if cur := mustSettings(p); cur.Has(key) {
		def = home.String(cur.Get(key))
	}
	for {
		v, e := t.Input(question, def)
		if e != nil {
			return e
		}
		if e := setSetting(p, key, v); e != nil {
			t.Say("✗", e.Error())
			continue
		}
		return nil
	}
}

// chooseSkills adds the ask skill to the apps you pick; skills you wrote are never replaced.
func chooseSkills(t *tui.Prompter, m *machine) error {
	if len(m.Hosts) == 0 {
		return nil
	}
	options, chosen := []tui.Option{}, []bool{}
	for _, h := range m.Hosts {
		note := map[string]string{"current": "added", "outdated": "added, will update", "yours": "has your own; left as it is", "missing": ""}[m.Skills[h.Name]]
		options = append(options, tui.Option{Label: h.Title, Note: note})
		chosen = append(chosen, m.Skills[h.Name] != "yours")
	}
	picked, e := t.MultiSelect("Teach these apps to use ask (adds the ask skill)", options, chosen)
	if e != nil {
		return e
	}
	for i, h := range m.Hosts {
		if !picked[i] || m.Skills[h.Name] == "yours" {
			continue
		}
		if e := setup.WriteSkill(h); e != nil {
			return e
		}
		if m.Skills[h.Name] != "current" {
			t.Say("✓", "added the ask skill to "+h.Title)
		}
	}
	return nil
}

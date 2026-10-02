package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

// draft holds every change a settings session makes. Nothing is written until review shows the
// changes and you save them, so leaving discards them.
type draft struct {
	p               home.Paths
	settings        home.Object
	models          home.Models
	hook            bool
	skills          map[string]bool
	install, remove map[string]bool
}

// newDraft starts a draft from what is saved now.
func newDraft(p home.Paths) (*draft, error) {
	s, e := p.ReadSettings()
	if e != nil {
		return nil, e
	}
	m, e := p.ReadModels()
	if e != nil {
		return nil, e
	}
	return &draft{p, s.Clone(), m.Clone(), setup.HookOn(), map[string]bool{}, map[string]bool{}, map[string]bool{}}, nil
}

// mustSettings reads saved settings, or none when they can't be read.
func mustSettings(p home.Paths) home.Object { s, _ := p.ReadSettings(); return s }

// settingKeys are what ask settings set takes: settings.json's keys, the Claude Code hook and
// the apps that get the ask skill.
var settingKeys = append(append([]string{}, home.Settings...), "hook", "skills")

// set changes one setting in the draft; an empty value returns it to its default.
func (d *draft) set(key, value string) error {
	switch key {
	case "hook":
		switch value {
		case "on", "true", "yes":
			d.hook = true
		case "off", "false", "no", "":
			d.hook = false
		default:
			return home.Usage("hook is on or off, not %q", value)
		}
		return nil
	case "skills":
		known := map[string]bool{}
		for _, h := range setup.Hosts() {
			known[h.Name] = true
		}
		for _, name := range strings.Split(value, ",") {
			name = strings.TrimSpace(name)
			switch {
			case name == "all":
				for _, h := range setup.Hosts() {
					if h.Present() {
						d.skills[h.Name] = true
					}
				}
			case known[name]:
				d.skills[name] = true
			case name != "":
				return home.Usage("skills are claude-code, codex, opencode, cursor, pi or all, not %q", name)
			}
		}
		return nil
	}
	s := d.settings.Clone()
	var v any = value
	if value == "" {
		v = nil
	} else if key == "timeout" || key == "jobs" || key == "max_cost" {
		n, e := strconv.ParseFloat(strings.TrimPrefix(value, "$"), 64)
		if e != nil {
			return home.Usage("%s must be a number, not %q", key, value)
		}
		v = n
	}
	if name, id, _ := strings.Cut(value, ":"); key == "model" && value != "" && (name == "" || id == "") {
		return home.Usage("model must be agent:id[#effort], like claude:sonnet-5.5, not %q", value)
	}
	s.Set(key, v)
	if v == nil {
		s.Delete(key)
	}
	if _, e := d.p.SettingsText(s); e != nil {
		return e
	}
	d.settings = s
	return nil
}

// read returns a file's text, or "" when it doesn't exist.
func read(path string) string { b, _ := os.ReadFile(path); return string(b) }

// changes lists what saving the draft would change, file by file.
func (d *draft) changes() ([]tui.Change, error) {
	out := []tui.Change{}
	names := []string{}
	for name := range d.install {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		out = append(out, tui.Change{Path: home.Tilde(packages.BuiltinDir(d.p, name)), Summary: "install the " + name + " agent, built into ask"})
	}
	names = names[:0]
	for name := range d.remove {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		out = append(out, tui.Change{Path: home.Tilde(packages.BuiltinDir(d.p, name)), Summary: "remove the " + name + " agent"})
	}
	settingsPath := filepath.Join(d.p.Home, "settings.json")
	after, e := d.p.SettingsText(d.settings)
	if e != nil {
		return nil, e
	}
	if before := read(settingsPath); before != after {
		out = append(out, tui.Change{Path: home.Tilde(settingsPath), Before: before, After: after})
	}
	modelsPath := filepath.Join(d.p.Home, "models.json")
	if before, after := read(modelsPath), home.ModelsText(d.models); before != after {
		saved, _ := d.p.ReadModels()
		if home.ModelsText(saved) != after {
			out = append(out, tui.Change{Path: home.Tilde(modelsPath), Before: before, After: after})
		}
	}
	if d.hook != setup.HookOn() {
		path, before, after, e := setup.HookEdit(d.hook)
		if e != nil {
			return nil, e
		}
		out = append(out, tui.Change{Path: home.Tilde(path), Before: string(before), After: string(after)})
	}
	for _, h := range setup.Hosts() {
		if !d.skills[h.Name] {
			continue
		}
		path := filepath.Join(h.SkillDir, "SKILL.md")
		switch setup.SkillState(h) {
		case "missing":
			out = append(out, tui.Change{Path: home.Tilde(path), After: setup.Skill})
		case "outdated":
			out = append(out, tui.Change{Path: home.Tilde(path), Before: read(path), After: setup.Skill})
		}
	}
	return out, nil
}

// mustChanges is changes, or none when they can't be worked out.
func (d *draft) mustChanges() []tui.Change { c, _ := d.changes(); return c }

// apply saves the draft, saying how each part went, and starts a new draft from the result. It
// returns false when an agent it installed isn't ready.
func (d *draft) apply(say func(string, string)) bool {
	ready := true
	for name := range d.remove {
		if line, e := packages.Remove(d.p, "ask/packages/"+name); e != nil {
			say("✗", e.Error())
		} else {
			say("✓", line)
		}
	}
	for name := range d.install {
		ready = installAgent(d.p, say, name) && ready
	}
	fail := func(e error) {
		if e != nil {
			say("✗", e.Error())
		}
	}
	fail(d.p.WriteSettings(d.settings))
	if saved, _ := d.p.ReadModels(); home.ModelsText(saved) != home.ModelsText(d.models) {
		fail(d.p.WriteModels(d.models))
	}
	if d.hook != setup.HookOn() {
		fail(setup.SetHook(d.hook))
	}
	for _, h := range setup.Hosts() {
		if d.skills[h.Name] {
			fail(setup.WriteSkill(h))
		}
	}
	if next, e := newDraft(d.p); e == nil {
		*d = *next
	}
	return ready
}

// review shows what saving would change and asks; it returns whether the draft was saved.
func (d *draft) review(t *tui.Prompter) (bool, error) {
	changes, e := d.changes()
	if e != nil {
		return false, e
	}
	if len(changes) == 0 {
		t.Say("✓", "nothing to save")
		return true, nil
	}
	fmt.Fprint(t.Out, "\r\n")
	tui.Review(t.Out, changes, t.Color, termWidth())
	fmt.Fprint(t.Out, "\r\n")
	yes, e := t.Confirm("Save these changes?", true)
	if e != nil || !yes {
		return false, e
	}
	d.apply(t.Say)
	t.Say("✓", "saved")
	return true, nil
}

// finish ends a session with changes pending: save them, throw them away, or keep editing.
// It returns errBack to keep editing.
func (d *draft) finish(t *tui.Prompter) error {
	n := len(d.mustChanges())
	if n == 0 {
		return nil
	}
	options := []tui.Option{{Label: "Review and save", Note: status.Plural(n, "change")}, {Label: "Discard", Note: "keep what is saved"}, {Label: "Keep editing"}}
	i, e := t.Select("You have unsaved changes", options, 0)
	if e != nil {
		return e
	}
	switch i {
	case 0:
		saved, e := d.review(t)
		if e == nil && !saved {
			return errBack
		}
		return e
	case 1:
		t.Say("•", "discarded "+status.Plural(n, "change"))
		return nil
	}
	return errBack
}

// errBack returns to the menu a choice came from.
var errBack = errors.New("back")

// edited marks a menu note whose part of the draft differs from what is saved.
func edited(note string, changed bool) string {
	if changed {
		return note + " · edited"
	}
	return note
}

// changedSetting reports whether a settings key differs between the draft and what is saved.
func (d *draft) changedSetting(keys ...string) bool {
	saved := mustSettings(d.p)
	for _, k := range keys {
		if home.JSON(saved.Get(k), false) != home.JSON(d.settings.Get(k), false) {
			return true
		}
	}
	return false
}

// mainMenu is ask settings: everything, grouped, with Save once something changed.
func (d *draft) mainMenu(t *tui.Prompter, m *machine) error {
	at := 0
	for {
		installed := []string{}
		for _, a := range m.Agents {
			if (a.Installed && !d.remove[a.Name]) || d.install[a.Name] {
				installed = append(installed, a.Name)
			}
		}
		agentsNote := strings.Join(installed, ", ")
		if agentsNote == "" {
			agentsNote = "none yet"
		}
		model := d.settings.S("model")
		if model == "" {
			model = "no default model"
		}
		defaults := model + " · " + strings.TrimSuffix(settingText(d.settings, "max_cost"), " (default)")
		if !d.settings.Has("max_cost") || d.settings.N("max_cost") == 0 {
			defaults = model + " · no cost limit"
		}
		skills, ownSkills := 0, 0
		for _, h := range m.Hosts {
			if m.Skills[h.Name] == "yours" {
				ownSkills++
			} else if m.Skills[h.Name] != "missing" || d.skills[h.Name] {
				skills++
			}
		}
		apps := fmt.Sprintf("ask skill in %d of %d apps", skills+ownSkills, len(m.Hosts))
		if hasHost(m, "claude-code") {
			apps += " · task titles " + map[bool]string{true: "on", false: "off"}[d.hook]
		}
		items := []tui.Option{
			{Label: "Agents", Note: edited(agentsNote, len(d.install)+len(d.remove) > 0)},
			{Label: "Defaults", Note: edited(defaults, d.changedSetting("model", "timeout", "jobs", "max_cost"))},
			{Label: "Worktrees", Note: edited(settingText(d.settings, "worktrees"), d.changedSetting("worktrees", "branches"))},
			{Label: "Apps", Note: edited(apps, len(d.skills) > 0 || d.hook != setup.HookOn())},
		}
		n := len(d.mustChanges())
		if n > 0 {
			items = append(items, tui.Option{Label: "Review and save", Note: status.Plural(n, "change")})
		}
		items = append(items, tui.Option{Label: "Done"})
		i, e := t.Select("What do you want to change?", items, min(at, len(items)-1))
		if e != nil {
			return e
		}
		at = i
		switch items[i].Label {
		case "Agents":
			e = d.agentsMenu(t, m)
		case "Defaults":
			e = d.defaultsMenu(t)
		case "Worktrees":
			e = d.worktreesMenu(t)
		case "Apps":
			e = d.appsMenu(t, m)
		case "Review and save":
			_, e = d.review(t)
			if e == nil {
				m, e = survey(d.p)
			}
		case "Done":
			if e = d.finish(t); e == nil {
				return nil
			}
		}
		if errors.Is(e, tui.ErrInterrupted) {
			return e
		}
		if e != nil && !errors.Is(e, errBack) {
			t.Say("✗", e.Error())
		}
	}
}

// agentsMenu lists the agents, official and yours, and opens the one you pick.
func (d *draft) agentsMenu(t *tui.Prompter, m *machine) error {
	options := []tui.Option{}
	for _, a := range m.Agents {
		note := ""
		switch {
		case d.install[a.Name]:
			note = "will be installed"
		case d.remove[a.Name]:
			note = "will be removed"
		case a.Ready:
			note = "ready · " + a.Models
		case a.Installed:
			note = "not ready: " + a.Why
		case a.CLI != "":
			note = "not installed · " + a.Title + " found"
		default:
			note = "not installed"
		}
		options = append(options, tui.Option{Label: a.Name, Note: note})
	}
	options = append(options, tui.Option{Label: "Back"})
	i, e := t.Select("Which agent?", options, 0)
	if e != nil || options[i].Label == "Back" {
		return e
	}
	return d.agentMenu(t, m, options[i].Label)
}

// agentMenu is one agent: install or remove it, which of its models are on, and their limits.
func (d *draft) agentMenu(t *tui.Prompter, m *machine, name string) error {
	var a *agentState
	for i := range m.Agents {
		if m.Agents[i].Name == name {
			a = &m.Agents[i]
		}
	}
	if a == nil {
		return home.Usage("no agent %q here", name)
	}
	official := packages.Builtin(name)
	at := 0
	for {
		items := []tui.Option{}
		if !a.Installed {
			if !official {
				return home.Usage("no %s agent is installed", name)
			}
			note := "from ask, to " + home.Tilde(packages.BuiltinDir(d.p, name))
			if d.install[name] {
				note = "will be installed · choose again to cancel"
			}
			items = append(items, tui.Option{Label: "Install", Note: note})
		} else {
			entries, failures := agent.New(d.p).Catalog(d.models)
			on, total, limited := 0, 0, 0
			for _, x := range entries {
				if owner, id, _ := strings.Cut(x.ID, ":"); owner == name {
					total++
					if !x.Off {
						on++
					}
					if c, _ := d.models.Get(name, id); c.MaxCost > 0 {
						limited++
					}
				}
			}
			for _, f := range failures {
				if strings.HasPrefix(f, name+": ") {
					t.Say("!", "could not list the models of "+f)
				}
			}
			limits := "your default for every model"
			if limited > 0 {
				limits = status.Plural(limited, "model") + " with their own"
			}
			items = append(items, tui.Option{Label: "Models", Note: fmt.Sprintf("%d of %d on", on, total)}, tui.Option{Label: "Cost limits", Note: limits})
			if official && strings.HasPrefix(a.Path, filepath.Dir(packages.BuiltinDir(d.p, name))) {
				note := "from " + home.Tilde(packages.BuiltinDir(d.p, name))
				if d.remove[name] {
					note = "will be removed · choose again to cancel"
				}
				items = append(items, tui.Option{Label: "Remove", Note: note})
			}
		}
		items = append(items, tui.Option{Label: "Back"})
		i, e := t.Select(name, items, min(at, len(items)-1))
		if e != nil {
			return e
		}
		at = i
		switch items[i].Label {
		case "Install":
			d.install[name] = !d.install[name]
			if !d.install[name] {
				delete(d.install, name)
			}
		case "Remove":
			d.remove[name] = !d.remove[name]
			if !d.remove[name] {
				delete(d.remove, name)
			}
		case "Models":
			e = d.chooseModelsOn(t, name)
		case "Cost limits":
			e = d.chooseCostLimit(t, name)
		case "Back":
			return nil
		}
		if e != nil {
			return e
		}
	}
}

// defaultsMenu edits the defaults every run starts from.
func (d *draft) defaultsMenu(t *tui.Prompter) error {
	at := 0
	for {
		model := d.settings.S("model")
		if model == "" {
			model = "none: give -m"
		}
		items := []tui.Option{
			{Label: "Model", Note: model},
			{Label: "Cost limit", Note: settingText(d.settings, "max_cost")},
			{Label: "Timeout", Note: settingText(d.settings, "timeout", "seconds")},
			{Label: "Batch jobs", Note: settingText(d.settings, "jobs", "at once")},
			{Label: "Back"},
		}
		i, e := t.Select("Defaults", items, at)
		if e != nil {
			return e
		}
		at = i
		switch items[i].Label {
		case "Model":
			e = d.chooseModel(t)
		case "Cost limit":
			e = d.askSetting(t, "max_cost", "Dollars a task may spend, 0 for no limit", "0")
		case "Timeout":
			e = d.askSetting(t, "timeout", "Seconds a task may run", "900")
		case "Batch jobs":
			e = d.askSetting(t, "jobs", "Batch tasks at once", "4")
		case "Back":
			return nil
		}
		if e != nil {
			return e
		}
	}
}

// worktreesMenu edits where --worktree runs work and their branch names.
func (d *draft) worktreesMenu(t *tui.Prompter) error {
	at := 0
	for {
		items := []tui.Option{
			{Label: "Folder", Note: settingText(d.settings, "worktrees")},
			{Label: "Branch", Note: settingText(d.settings, "branches")},
			{Label: "Back"},
		}
		i, e := t.Select("Worktrees", items, at)
		if e != nil {
			return e
		}
		at = i
		switch items[i].Label {
		case "Folder":
			e = d.chooseWorktrees(t)
		case "Branch":
			e = d.askSetting(t, "branches", "Branch name, with {name}", "ask/{name}")
		case "Back":
			return nil
		}
		if e != nil {
			return e
		}
	}
}

// appsMenu edits what the apps you work in know about ask.
func (d *draft) appsMenu(t *tui.Prompter, m *machine) error {
	at := 0
	for {
		items := []tui.Option{{Label: "The ask skill", Note: "tells their agents how to use ask"}}
		if hasHost(m, "claude-code") {
			items = append(items, tui.Option{Label: "Task titles in Claude Code", Note: map[bool]string{true: "on", false: "off"}[d.hook]})
		}
		items = append(items, tui.Option{Label: "Back"})
		i, e := t.Select("Apps", items, at)
		if e != nil {
			return e
		}
		at = i
		switch items[i].Label {
		case "The ask skill":
			e = d.chooseSkills(t, m)
		case "Task titles in Claude Code":
			on, err := t.Confirm("Show ask runs by model and task in Claude Code's task list?", !d.hook)
			d.hook, e = on, err
		case "Back":
			return nil
		}
		if e != nil {
			return e
		}
	}
}

// chooseModel picks the default model from what the agents offer.
func (d *draft) chooseModel(t *tui.Prompter) error {
	ids, _ := agent.New(d.p).List(d.models)
	if len(ids) == 0 {
		return nil
	}
	current := d.settings.S("model")
	options := []tui.Option{{Label: "None", Note: "give -m every time"}}
	at := 0
	a := agent.New(d.p)
	for i, id := range ids {
		m, _ := agent.Parse(d.p, id)
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
	return d.set("model", value)
}

// chooseWorktrees picks where --worktree works: ask's home, or a folder you name.
func (d *draft) chooseWorktrees(t *tui.Prompter) error {
	current := d.settings.S("worktrees")
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
		return d.set("worktrees", "")
	}
	def := current
	if def == "" {
		def = "~/code/worktrees/ask-{name}"
	}
	return d.askSetting(t, "worktrees", "Folder for each worktree, with {name}", def)
}

// askSetting asks for one setting's value until it is valid; an empty answer keeps def.
func (d *draft) askSetting(t *tui.Prompter, key, question, def string) error {
	if d.settings.Has(key) {
		def = home.String(d.settings.Get(key))
	}
	for {
		v, e := t.Input(question, def)
		if e != nil {
			return e
		}
		if e := d.set(key, v); e != nil {
			t.Say("✗", e.Error())
			continue
		}
		return nil
	}
}

// chooseSkills picks the apps that get the ask skill; one you wrote yourself is never replaced.
func (d *draft) chooseSkills(t *tui.Prompter, m *machine) error {
	if len(m.Hosts) == 0 {
		return nil
	}
	options, chosen := []tui.Option{}, []bool{}
	for _, h := range m.Hosts {
		note := map[string]string{"current": "added", "outdated": "added · will update", "yours": "has your own; left as it is", "missing": ""}[m.Skills[h.Name]]
		options = append(options, tui.Option{Label: h.Title, Note: note})
		chosen = append(chosen, m.Skills[h.Name] != "missing" || d.skills[h.Name])
	}
	picked, e := t.MultiSelect("Teach these apps to use ask", options, chosen)
	if e != nil {
		return e
	}
	for i, h := range m.Hosts {
		if picked[i] && m.Skills[h.Name] != "yours" && m.Skills[h.Name] != "current" {
			d.skills[h.Name] = true
		} else {
			delete(d.skills, h.Name)
		}
	}
	return nil
}

// chooseModelsOn turns an agent's models on or off; models.json records the ones that are off.
func (d *draft) chooseModelsOn(t *tui.Prompter, name string) error {
	entries, _ := agent.New(d.p).Catalog(d.models)
	mine := []agent.Entry{}
	for _, x := range entries {
		if owner, _, _ := strings.Cut(x.ID, ":"); owner == name {
			mine = append(mine, x)
		}
	}
	if len(mine) == 0 {
		t.Say("•", name+" lists no models; add one with ask models "+name+":MODEL --enable")
		return nil
	}
	options, chosen := []tui.Option{}, []bool{}
	for _, x := range mine {
		_, id, _ := strings.Cut(x.ID, ":")
		options = append(options, tui.Option{Label: id, Note: x.Name})
		chosen = append(chosen, !x.Off)
	}
	picked, e := t.MultiSelect("Which "+name+" models should ask use?", options, chosen)
	if e != nil {
		return e
	}
	for i, x := range mine {
		_, id, _ := strings.Cut(x.ID, ":")
		c, _ := d.models.Get(name, id)
		c.Off = !picked[i]
		keep(d.models, name, id, c, x.Listed)
	}
	return nil
}

// chooseCostLimit sets one model's cost limit, which replaces your default for its runs.
func (d *draft) chooseCostLimit(t *tui.Prompter, name string) error {
	entries, _ := agent.New(d.p).Catalog(d.models)
	options, listed := []tui.Option{}, map[string]bool{}
	for _, x := range entries {
		owner, id, _ := strings.Cut(x.ID, ":")
		if owner != name || x.Off {
			continue
		}
		listed[id] = x.Listed
		note := "your default: " + settingText(d.settings, "max_cost")
		if c, _ := d.models.Get(name, id); c.MaxCost > 0 {
			note = "$" + home.Dollars(c.MaxCost) + " a task"
		}
		options = append(options, tui.Option{Label: id, Note: note})
	}
	if len(options) == 0 {
		return nil
	}
	i, e := t.Select("Which model's limit?", options, 0)
	if e != nil {
		return e
	}
	id := options[i].Label
	c, _ := d.models.Get(name, id)
	def := "0"
	if c.MaxCost > 0 {
		def = home.Dollars(c.MaxCost)
	}
	for {
		v, e := t.Input("Dollars a "+id+" task may spend, 0 for your default", def)
		if e != nil {
			return e
		}
		n, err := strconv.ParseFloat(strings.TrimPrefix(v, "$"), 64)
		if err != nil || n < 0 {
			t.Say("✗", "give dollars, like 10")
			continue
		}
		c.MaxCost = n
		keep(d.models, name, id, c, listed[id])
		return nil
	}
}

// settingsCommand runs ask settings: the menu in a terminal, or get, set, unset and NAME for
// scripts and agents. set and unset show what they change, then save; --dry-run only shows it.
func settingsCommand(p home.Paths, opts home.Object, words []string) (int, error) {
	terminal := status.IsTerminal(os.Stdin) && status.IsTerminal(os.Stdout)
	if len(words) == 0 {
		if terminal {
			return interactive(p, "", false)
		}
		return getSettings(p, "", opts.B("--json"))
	}
	switch words[0] {
	case "get":
		if len(words) > 2 {
			return 0, home.Usage("ask settings get takes at most one key, like ask settings get model")
		}
		key := ""
		if len(words) == 2 {
			key = words[1]
		}
		return getSettings(p, key, opts.B("--json"))
	case "set", "unset":
		want := 3
		if words[0] == "unset" {
			want = 2
		}
		if len(words) != want {
			return 0, home.Usage("ask settings set KEY VALUE, or ask settings unset KEY; keys: %s", strings.Join(settingKeys, ", "))
		}
		key, value := words[1], ""
		if want == 3 {
			value = words[2]
		}
		if !has(settingKeys, key) {
			return 0, home.Usage("no setting %q; keys: %s", key, strings.Join(settingKeys, ", "))
		}
		d, e := newDraft(p)
		if e != nil {
			return 0, e
		}
		if key == "skills" && value == "" {
			return 0, home.Usage("ask settings set skills APP,... or all; a skill you no longer want is a file to delete")
		}
		if e := d.set(key, value); e != nil {
			return 0, e
		}
		changes, e := d.changes()
		if e != nil {
			return 0, e
		}
		if len(changes) == 0 {
			fmt.Fprintln(os.Stderr, "ask: "+key+" is already that")
			return 0, nil
		}
		tui.Review(os.Stderr, changes, status.CanStyle(os.Stderr), termWidth())
		if opts.B("--dry-run") {
			return 0, nil
		}
		d.apply(func(mark, text string) { fmt.Fprintln(os.Stderr, mark+" "+text) })
		return 0, nil
	}
	if len(words) > 1 {
		return 0, home.Usage("ask settings takes get, set, unset or an agent's name; see ask settings --help")
	}
	name := words[0]
	if find.Path(p, "agents", name) == "" && !packages.Builtin(name) {
		return 0, home.Usage("no agent %q; ask settings opens all your settings", name)
	}
	if terminal {
		return interactive(p, name, false)
	}
	config, e := p.ReadModels()
	if e != nil {
		return 0, e
	}
	entries, _ := agent.New(p).Catalog(config)
	for _, x := range entries {
		if owner, id, _ := strings.Cut(x.ID, ":"); owner == name {
			state := "on"
			if x.Off {
				state = "off"
			}
			if c, _ := config.Get(owner, id); c.MaxCost > 0 {
				state += "\tmax $" + home.Dollars(c.MaxCost)
			}
			fmt.Fprintf(os.Stdout, "%s\t%s\n", x.ID, state)
		}
	}
	return 0, nil
}

// getSettings prints every setting, or one key's value, with defaults marked; --json prints the
// values that apply.
func getSettings(p home.Paths, key string, asJSON bool) (int, error) {
	s, e := p.ReadSettings()
	if e != nil {
		return 0, e
	}
	defaults := home.O("timeout", 900.0, "jobs", 4.0, "max_cost", 0.0, "worktrees", "~/.ask/worktrees/{name}", "branches", "ask/{name}")
	values := home.Object{}
	for _, k := range home.Settings {
		if s.Has(k) {
			values.Set(k, s.Get(k))
		} else if defaults.Has(k) {
			values.Set(k, defaults.Get(k))
		} else {
			values.Set(k, nil)
		}
	}
	values.Set("hook", setup.HookOn())
	skills := home.Object{}
	for _, h := range setup.Hosts() {
		if h.Present() {
			skills.Set(h.Name, setup.SkillState(h))
		}
	}
	values.Set("skills", skills)
	if key != "" {
		if !has(settingKeys, key) {
			return 0, home.Usage("no setting %q; keys: %s", key, strings.Join(settingKeys, ", "))
		}
		v := values.Get(key)
		switch {
		case asJSON:
			fmt.Fprintln(os.Stdout, home.JSON(v, false))
		case v == nil:
			fmt.Fprintln(os.Stdout)
		case key == "hook":
			fmt.Fprintln(os.Stdout, map[bool]string{true: "on", false: "off"}[v == true])
		default:
			fmt.Fprintln(os.Stdout, home.String(v))
		}
		return 0, nil
	}
	if asJSON {
		fmt.Fprintln(os.Stdout, home.JSON(values, true))
		return 0, nil
	}
	for _, k := range home.Settings {
		fmt.Fprintf(os.Stdout, "%-10s %s\n", k, settingText(s, k))
	}
	fmt.Fprintf(os.Stdout, "%-10s %s\n", "hook", map[bool]string{true: "on", false: "off"}[setup.HookOn()])
	added := []string{}
	for _, h := range setup.Hosts() {
		if h.Present() && setup.SkillState(h) != "missing" {
			added = append(added, h.Name)
		}
	}
	fmt.Fprintf(os.Stdout, "%-10s %s\n", "skills", strings.Join(added, ", "))
	return 0, nil
}

// termWidth is the terminal's width, for full-width review rows.
func termWidth() int { w, _ := status.TerminalSize(); return w }

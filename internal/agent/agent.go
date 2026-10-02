// Package agent implements local model discovery and the version 1 agent contract.
package agent

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/fschrhunt/ask/internal/find"
	"github.com/fschrhunt/ask/internal/home"
	"github.com/fschrhunt/ask/internal/process"
)

// Model splits the agent:model#effort specification without changing its spelling.
type Model struct{ Spec, Agent, ID, Effort string }

var specPattern = regexp.MustCompile(`^([a-z0-9][a-z0-9_-]*):([^#\s]+)(?:#(\S+))?$`)

// Parse validates a model specification and requires its agent to be installed.
func Parse(p home.Paths, spec string) (Model, error) {
	m := specPattern.FindStringSubmatch(spec)
	if m != nil {
		for _, r := range m[2] + m[3] {
			if home.Space(r) {
				m = nil
				break
			}
		}
	}
	if m == nil {
		return Model{}, home.Usage("bad model \"%s\": expected agent:id[#effort]; see `ask models`", spec)
	}
	if find.Path(p, "agents", m[1]) == "" {
		names := []string{}
		for _, x := range find.Sorted(p, "agents") {
			names = append(names, x.Name)
		}
		text := strings.Join(names, ", ")
		if text == "" {
			text = "none"
		}
		return Model{}, home.Usage("no agent \"%s\" in %s; installed: %s; see docs/agents.md", m[1], p.Agents, text)
	}
	return Model{spec, m[1], m[2], m[3]}, nil
}

type listedModel struct{ id, name string }
type listing struct {
	models []listedModel
	error  string
	ready  chan struct{}
}

// Registry caches agent-owned lists during an invocation; models.json ids are added per call.
type Registry struct {
	Paths  home.Paths
	mu     sync.Mutex
	listed map[string]*listing
}

// New creates an invocation-local registry.
func New(p home.Paths) *Registry { return &Registry{Paths: p, listed: map[string]*listing{}} }

// models caches one agent's declaration without serializing different agents' listings.
func (a *Registry) models(name string, config home.Object) listing {
	a.mu.Lock()
	own, ok := a.listed[name]
	if !ok {
		own = &listing{ready: make(chan struct{})}
		a.listed[name] = own
	}
	a.mu.Unlock()
	if !ok {
		r := process.Run(find.Path(a.Paths, "agents", name), []string{"models"}, process.Options{Env: a.Paths.Env(nil), Timeout: 10 * time.Second})
		if r.Code != 0 {

			own.error = process.Reason(r.Stderr)
			if own.error == "" {
				if r.TimedOut {
					own.error = "timed out"
				} else {
					own.error = exitText(r)
					if r.Signal == 0 && r.Code >= 0 {
						own.error = "exit " + own.error
					}
				}
			}
		} else {
			for _, line := range strings.Split(r.Stdout, "\n") {
				if home.Trim(line) == "" {
					continue
				}
				bits := strings.Split(line, "\t")
				m := listedModel{id: home.Trim(bits[0])}
				if len(bits) > 1 {
					m.name = home.Trim(bits[1])
				}
				own.models = append(own.models, m)
			}
		}
		close(own.ready)
	}
	<-own.ready
	out := listing{models: append([]listedModel{}, own.models...), error: own.error}
	if extra, ok := config.Get(name).([]any); ok {
		for _, id := range extra {
			out.models = append(out.models, listedModel{id: home.String(id)})
		}
	}
	return out
}

// List returns model ids in agent order and each listing failure as "agent: reason".
// Failed declarations are reported per agent, including executable start errors.
func (a *Registry) List(config home.Object) ([]string, []string) {
	names := find.Sorted(a.Paths, "agents")
	all := make([]listing, len(names))
	var wg sync.WaitGroup
	for i, x := range names {
		wg.Add(1)
		go func(i int, name string) { defer wg.Done(); all[i] = a.models(name, config) }(i, x.Name)
	}
	wg.Wait()
	ids, errors := []string{}, []string{}
	for i, x := range names {

		for _, m := range all[i].models {
			ids = append(ids, x.Name+":"+m.id)
		}
		if all[i].error != "" {
			errors = append(errors, x.Name+": "+all[i].error)
		}
	}
	return ids, errors
}

// Name prefers the report, then the listed model name, then a title-cased model id.
func (a *Registry) Name(m Model, reported string) string {
	name := reported
	if name == "" {
		listed := a.models(m.Agent, nil)

		for _, x := range listed.models {
			if strings.EqualFold(x.id, m.ID) {
				name = x.name
				break
			}
		}
	}
	if name == "" {
		bits := strings.FieldsFunc(m.ID, func(r rune) bool { return r == '/' })
		name = m.ID
		if len(bits) > 0 {
			words := strings.FieldsFunc(bits[len(bits)-1], func(r rune) bool { return r == '-' })
			for i, w := range words {
				r := []rune(w)
				r[0] = unicode.ToUpper(r[0])
				words[i] = string(r)
			}
			name = strings.Join(words, " ")
		}
	}
	if m.Effort != "" {
		name += " (" + m.Effort + ")"
	}
	return name
}

// exitText describes a process exit, including termination by a signal.
func exitText(r process.Result) string {
	if r.Signal != 0 {
		return "killed by " + process.SignalName(r.Signal)
	}
	if r.Code < 0 {
		return "could not start"
	}
	return fmt.Sprint(r.Code)
}

// Run sends the prompt on stdin and reads the optional report even after failure or timeout.
// While the agent runs, progress (if not nil) gets each new usage the agent rewrites its report with.
func (a *Registry) Run(m Model, prompt string, t home.Object, progress func(home.Object)) home.Object {
	work, err := os.MkdirTemp("", "ask-")
	if err != nil {
		return home.O("ok", false, "note", err.Error())
	}
	defer os.RemoveAll(work)
	access := "read"
	if t.B("write") {
		access = "write"
	}
	vars := map[string]string{"ASK_MODEL": m.ID, "ASK_EFFORT": m.Effort, "ASK_ACCESS": access, "ASK_REPORT": filepath.Join(work, "report.json")}
	if t.Has("schema") && t.Get("schema") != nil {
		vars["ASK_SCHEMA"] = filepath.Join(work, "schema.json")
		if e := os.WriteFile(vars["ASK_SCHEMA"], []byte(home.JSON(t.Get("schema"), false)), 0600); e != nil {
			return home.O("ok", false, "note", e.Error())
		}
	}
	if t.B("session") {
		vars["ASK_SESSION"] = t.S("session")
	}
	timeout := t.N("timeout")
	if timeout == 0 {
		timeout = 900
	}
	done := make(chan struct{})
	watched := make(chan struct{})
	go func() {
		defer close(watched)
		if progress == nil {
			return
		}
		tick := time.NewTicker(500 * time.Millisecond)
		defer tick.Stop()
		last := ""
		for {
			select {
			case <-done:
				return
			case <-tick.C:
			}
			if b, err := os.ReadFile(vars["ASK_REPORT"]); err == nil && string(b) != last {
				var report Report
				if json.Unmarshal(b, &report) == nil {
					last = string(b)
					if usage := report.usage(); usage != nil {
						progress(usage)
					}
				}
			}
		}
	}()
	r := process.Run(find.Path(a.Paths, "agents", m.Agent), nil, process.Options{Input: prompt, Dir: t.S("dir"), Env: a.Paths.Env(vars), Timeout: process.Timeout(timeout)})
	close(done)
	<-watched

	report := Report{}
	if b, err := os.ReadFile(vars["ASK_REPORT"]); err == nil {
		_ = json.Unmarshal(b, &report)
	}
	meta := home.O("usage", nil, "name", home.Trim(report.Name), "session", home.Trim(report.Session))
	if !meta.B("session") {
		meta.Set("session", t.Get("session"))
	}
	if usage := report.usage(); usage != nil {
		meta.Set("usage", usage)
	}

	meta.Set("ok", false)
	switch {
	case r.TimedOut:
		meta.Set("note", "timed out")
	case r.Code != 0:
		why := process.Reason(r.Stderr)
		if why == "" {
			why = exitText(r)
			if r.Signal == 0 && r.Code >= 0 {
				why = "exit " + why
			}
		}
		meta.Set("note", why)
	case home.Trim(r.Stdout) == "":
		meta.Set("note", "no answer")
	default:
		meta.Set("ok", true)
		meta.Set("text", r.Stdout)
		meta.Set("note", home.Trim(report.Note))
	}
	return meta
}

// Report is the agent's optional version 1 metadata; unknown fields are ignored.
// Pointer counts distinguish an omitted metric from a reported zero.
type Report struct {
	Session string   `json:"session"`
	Name    string   `json:"name"`
	Input   *float64 `json:"input"`
	Output  *float64 `json:"output"`
	Cached  *float64 `json:"cached"`
	Cost    *float64 `json:"cost"`
	Note    string   `json:"note"`
}

// usage returns the report's valid counts and cost, or nil when it has none.
func (report Report) usage() home.Object {
	usage := home.Object{}
	for key, n := range map[string]*float64{"input": report.Input, "output": report.Output, "cached": report.Cached, "cost": report.Cost} {
		if n != nil && *n >= 0 && !math.IsInf(*n, 0) && !math.IsNaN(*n) {
			usage.Set(key, *n)
		}
	}
	if len(usage) == 0 {
		return nil
	}
	return usage
}

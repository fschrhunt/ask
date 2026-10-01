// Package hooks implements ordered task/result hooks with fail-open errors and cached declarations.
package hooks

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/fschrhunt/ask/internal/find"
	"github.com/fschrhunt/ask/internal/home"
	"github.com/fschrhunt/ask/internal/process"
)

// Hooks keeps an invocation's declared events and paths.
type Hooks struct {
	Paths    home.Paths
	mu       sync.Mutex
	declared map[string][]string
}

// New creates an invocation-local hook cache.
func New(p home.Paths) *Hooks { return &Hooks{Paths: p, declared: map[string][]string{}} }

// forEvent discovers hooks in name order and caches their declared events.
func (h *Hooks) forEvent(event string) ([]find.Executable, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	all := find.Sorted(h.Paths, "hooks")
	chosen := []find.Executable{}
	for _, x := range all {
		events, ok := h.declared[x.Path]
		if !ok {
			r := process.Run(x.Path, []string{"events"}, process.Options{Env: h.Paths.Env(nil), Timeout: 10 * time.Second})
			if r.Fatal {
				return nil, fmt.Errorf("%s", home.Trim(r.Stderr))
			}
			if r.Code == 0 {
				events = strings.Fields(r.Stdout)
			}
			h.declared[x.Path] = events
		}
		for _, e := range events {
			if e == event {
				chosen = append(chosen, x)
				break
			}
		}
	}
	return chosen, nil
}

// call sends a hook its event, input and contract environment, returning fail-open errors.
func (h *Hooks) call(path, event string, input home.Object, dir, ref string) home.Object {
	r := process.Run(path, []string{event}, process.Options{Input: home.JSON(input, false), Dir: dir, Env: h.Paths.Env(map[string]string{"ASK_RUN": ref, "ASK_EVENT": event}), Timeout: 600 * time.Second})
	if r.Fatal {
		return home.O("error", home.Trim(r.Stderr), "fatal", true)
	}
	if r.TimedOut {
		return home.O("error", "timed out")
	}
	if r.Code != 0 {
		why := process.Reason(r.Stderr)
		if why == "" {
			code := "null"
			if r.Code >= 0 {
				code = fmt.Sprint(r.Code)
			}
			why = "exit " + code
		}
		return home.O("error", why)
	}
	if home.Trim(r.Stdout) == "" {
		return home.Object{}
	}
	v, e := home.ParseJSON(r.Stdout)
	if e != nil {
		return home.O("error", "printed something other than JSON")
	}
	if o, ok := v.(home.Object); ok {
		return o
	}
	return home.O("error", "printed something other than a JSON object")
}

// Note is a hook name and its note, follow-up or failure text.
type Note struct{ Name, Text string }

// notes collects explicit notes and failed-hook diagnostics without deciding the task outcome.
func notes(answer home.Object, name string, all []Note) []Note {
	if answer.B("error") {
		all = append(all, Note{name, "failed: " + home.String(answer.Get("error"))})
	}
	if text := home.Trim(answer.S("note")); text != "" {
		all = append(all, Note{name, text})
	}
	return all
}

// Before applies task changes in name order, returning a refusal only when explicit.
// Invalid executable formats and working directories preserve the original fatal start errors.
func (h *Hooks) Before(t home.Object, ref string) (home.Object, string, []Note, error) {
	all := []Note{}
	found, err := h.forEvent("task")
	if err != nil {
		return t, "", all, err
	}
	for _, x := range found {
		a := h.call(x.Path, "task", home.O("task", t), t.S("dir"), ref)
		if a.B("fatal") {
			return t, "", all, fmt.Errorf("%s", a.S("error"))
		}
		all = notes(a, x.Name, all)
		if why, ok := a.Get("refuse").(string); ok {
			if why == "" {
				why = "refused"
			}
			return t, x.Name + ": " + why, all, nil
		}
		change, _ := a.Get("task").(home.Object)
		if array, ok := a.Get("task").([]any); ok {
			for i, v := range array {
				change.Set(fmt.Sprint(i), v)
			}
		}
		for _, f := range change {
			want := map[string]string{"prompt": "string", "model": "string", "write": "boolean", "json": "boolean", "timeout": "number", "schema": "object"}[f.Key]
			kind := home.Kind(f.Value)
			if kind == "array" {
				kind = "object"
			}
			if want != "" && kind == want && f.Value != nil && (kind != "number" || home.Number(f.Value) > 0) {
				t = t.Clone()
				t.Set(f.Key, f.Value)
			} else {
				all = append(all, Note{x.Name, fmt.Sprintf("ignored a change to \"%s\"", f.Key)})
			}
		}
	}
	return t, "", all, nil
}

// After checks a result, stopping at the first explicit failure or follow-up or fatal start error.
func (h *Hooks) After(t, result home.Object, ref string) (*Note, *Note, []Note, error) {
	all := []Note{}
	dir := result.S("dir")
	if dir == "" {
		dir = t.S("dir")
	}
	found, err := h.forEvent("result")
	if err != nil {
		return nil, nil, all, err
	}
	for _, x := range found {
		a := h.call(x.Path, "result", home.O("task", t, "result", result), dir, ref)
		if a.B("fatal") {
			return nil, nil, all, fmt.Errorf("%s", a.S("error"))
		}
		all = notes(a, x.Name, all)
		if why, ok := a.Get("fail").(string); ok {
			if why == "" {
				why = "failed"
			}
			return nil, &Note{x.Name, why}, all, nil
		}
		if prompt := home.Trim(a.S("followup")); prompt != "" {
			return &Note{x.Name, prompt}, nil, all, nil
		}
	}
	return nil, nil, all, nil
}

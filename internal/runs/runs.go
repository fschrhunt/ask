// Package runs records, validates, follows up, resumes and stops runs under ASK_HOME/runs.
package runs

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fschrhunt/ask/internal/agent"
	"github.com/fschrhunt/ask/internal/find"
	"github.com/fschrhunt/ask/internal/git"
	"github.com/fschrhunt/ask/internal/home"
	"github.com/fschrhunt/ask/internal/hooks"
	"github.com/fschrhunt/ask/internal/task"
)

// Run is a saved run with immutable tasks and position-matched results (nil means unfinished).
type Run struct {
	ID, Dir, Created string
	Tasks, Results   []home.Object
}

// NewID returns six lowercase letters and digits.
func NewID() string {
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 6)
	for i := range b {
		var v [1]byte
		for {
			_, _ = rand.Read(v[:])
			if v[0] < 252 {
				break
			}
		}
		b[i] = alphabet[int(v[0])%36]
	}
	return string(b)
}

// Ref names a single task as RUN, or a batch task as RUN/TASK.
func Ref(r *Run, index int) string {
	if len(r.Tasks) == 1 {
		return r.ID
	}
	return r.ID + "/" + r.Tasks[index].S("id")
}

// readJSON returns nil for missing or unreadable records; load distinguishes damaged results.
func readJSON(path string) any {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil
	}
	v, _ := home.ParseJSON(home.UTF8(b))
	return v
}

// load reads task-position results and refuses damaged results instead of silently rerunning them.
func load(dir string) (*Run, error) {
	b, err := os.ReadFile(filepath.Join(dir, "tasks.json"))
	var records []home.TaskRecord
	if err != nil || json.Unmarshal(b, &records) != nil || records == nil {
		return nil, home.Usage("%s is not a readable run", dir)
	}
	tasks := make([]home.Object, len(records))
	results := make([]home.Object, len(records))
	for i, t := range records {
		tasks[i], _ = home.ParsePayload(home.JSON(t, false))
	}
	path := filepath.Join(dir, "results.json")
	if b, err := os.ReadFile(path); err == nil {
		var saved []*home.ResultRecord
		if json.Unmarshal(b, &saved) != nil || saved == nil {
			return nil, home.Usage("%s is damaged; move it away to rerun every task", path)
		}
		for i := range tasks {
			if i < len(saved) && saved[i] != nil {
				results[i], _ = home.ParsePayload(home.JSON(saved[i], false))
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	name := filepath.Base(dir)
	at := strings.LastIndex(name, "-")
	id, created := name, ""
	if at >= 0 {
		id = name[at+1:]
		created = name[:at]
	}
	return &Run{id, dir, created, tasks, results}, nil
}

// Create writes prepared tasks once and returns their run folder.
func Create(p home.Paths, id string, tasks []home.Object) (*Run, error) {
	stamp := time.Now().UTC().Format("20060102T150405.000")
	stamp = strings.ReplaceAll(stamp, ".", "")
	dir := filepath.Join(p.Runs, stamp+"-"+id)
	if e := os.MkdirAll(dir, 0777); e != nil {
		return nil, e
	}
	if e := home.WriteJSON(filepath.Join(dir, "tasks.json"), home.TaskRecords(tasks)); e != nil {
		return nil, e
	}
	return load(dir)
}

// Open resolves RUN[/TASK] or a directory; index -1 selects all results.
func Open(p home.Paths, ref string) (*Run, int, error) {
	if strings.HasPrefix(ref, ".") || strings.HasPrefix(ref, "/") || strings.HasPrefix(ref, "~") {
		info, e := os.Stat(ref)
		if e != nil || !info.IsDir() {
			return nil, -1, home.Usage("no run at %s; see `ask runs`", ref)
		}
		dir, _ := filepath.Abs(ref)
		r, e := load(dir)
		return r, -1, e
	}
	id, part, has := strings.Cut(ref, "/")
	name := ""
	for _, n := range find.List(p.Runs) {
		if n == id || strings.HasSuffix(n, "-"+id) {
			name = n
			break
		}
	}
	if id == "" || name == "" {
		return nil, -1, home.Usage("no run %s; see `ask runs`", id)
	}
	r, e := load(filepath.Join(p.Runs, name))
	if e != nil {
		return nil, -1, e
	}
	if !has {
		if len(r.Tasks) == 1 {
			return r, 0, nil
		}
		return r, -1, nil
	}
	matches := []int{}
	for i, t := range r.Tasks {
		if home.String(t.Get("id")) == part {
			matches = append(matches, i)
		}
	}
	if len(matches) > 1 {
		return nil, -1, home.Usage("run %s has %d tasks named %s; use their position, like %s/%d", id, len(matches), part, id, matches[0]+1)
	}
	if len(matches) == 1 {
		return r, matches[0], nil
	}
	if digits.MatchString(part) {
		n, e := strconv.Atoi(part)
		if e == nil && n >= 1 && n <= len(r.Tasks) {
			return r, n - 1, nil
		}
	}
	return nil, -1, home.Usage("run %s has no task %s", id, part)
}

var digits = regexp.MustCompile(`^\d+$`)

// safeWorktreeID replaces characters unsafe in branch names and filesystem paths.
func safeWorktreeID(id string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("_.-", r) {
			return r
		}
		return '_'
	}, id)
}

// Alive tests process existence, treating permission errors as alive.
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	e := syscall.Kill(pid, 0)
	return e == nil || e == syscall.EPERM
}

// Owner returns a live lock owner, or zero if a run is not currently running.
func Owner(r *Run) int {
	n := home.Number(readJSON(filepath.Join(r.Dir, "lock")))
	if math.IsNaN(n) || math.IsInf(n, 0) || n != math.Trunc(n) || n > math.MaxInt32 {
		return 0
	}
	pid := int(n)
	if Alive(pid) {
		return pid
	}
	return 0
}

// lock refuses a live owner and takes over a lock whose owner is gone.
func lock(r *Run) error {
	if pid := Owner(r); pid != 0 && pid != os.Getpid() {
		return home.Usage("run %s is running (pid %d); stop it with `ask stop %s`", r.ID, pid, r.ID)
	}
	return os.WriteFile(filepath.Join(r.Dir, "lock"), []byte(strconv.Itoa(os.Getpid())), 0666)
}

// Prepare validates task fields and inherits a followed-up task's agent, access and location.
func Prepare(p home.Paths, items []any, defaults home.Object, id string, single bool, missing func(string) error) ([]home.Object, error) {
	out := []home.Object{}
	for i, item := range items {
		where := fmt.Sprintf("task %d", i+1)
		if single {
			where = "ask"
		}
		original, ok := item.(home.Object)
		if !ok {
			return nil, home.Usage("%s must be a JSON object", where)
		}
		t := defaults.Clone()
		for key, val := range original {
			if val != nil {
				t.Set(key, val)
			}
		}
		if _, ok := t.Get("prompt").(string); !ok || home.Trim(t.S("prompt")) == "" {
			return nil, home.Usage("%s has no \"prompt\"", where)
		}
		for _, key := range []string{"write", "json", "worktree"} {
			if t.Has(key) {
				if _, ok := t.Get(key).(bool); !ok {
					return nil, home.Usage("%s: \"%s\" must be true or false", where, key)
				}
			}
		}
		for _, key := range []string{"model", "dir", "continue"} {
			if t.Has(key) {
				if _, ok := t.Get(key).(string); !ok {
					return nil, home.Usage("%s: \"%s\" must be a string", where, key)
				}
			}
		}
		if t.Has("schema") && t.Get("schema") != nil && home.Kind(t.Get("schema")) != "object" && home.Kind(t.Get("schema")) != "boolean" {
			return nil, home.Usage("%s: \"schema\" must be a JSON Schema object", where)
		}
		timeout := 900.0
		if t.Has("timeout") {
			n, ok := t.Get("timeout").(float64)
			if !ok || !(n > 0 && n <= 2000000) {
				return nil, home.Usage("%s: timeout must be a number of seconds above 0", where)
			}
			timeout = n
		}
		taskID := home.String(original.Get("id"))
		if original.Get("id") == nil {
			taskID = strconv.Itoa(i + 1)
		}
		dir := t.S("dir")
		if !t.Has("dir") {
			dir = "."
		}
		dir, _ = filepath.Abs(dir)
		x := home.O("id", taskID, "prompt", t.Get("prompt"), "model", t.Get("model"), "write", t.Get("write"))
		x.Set("json", t.B("json"))
		if t.Has("schema") {
			x.Set("schema", t.Get("schema"))
		}
		x.Set("dir", dir)
		x.Set("timeout", timeout)
		if t.B("continue") {
			prev, at, e := Open(p, t.S("continue"))
			if e != nil {
				return nil, e
			}
			if at < 0 {
				return nil, home.Usage("run %s has %d tasks; continue one of them, like %s/%s", prev.ID, len(prev.Tasks), prev.ID, prev.Tasks[0].S("id"))
			}
			old, result := prev.Tasks[at], prev.Results[at]
			if !result.B("session") {
				return nil, home.Usage("%s cannot be continued: its agent reported no session", t.S("continue"))
			}
			if x.Get("model") == nil {
				x.Set("model", old.Get("model"))
			}
			m, e := agent.Parse(p, x.S("model"))
			if e != nil {
				return nil, e
			}
			prior, e := agent.Parse(p, old.S("model"))
			if e != nil {
				return nil, e
			}
			if m.Agent != prior.Agent {
				return nil, home.Usage("%s ran on %s; a follow-up must use the same agent", t.S("continue"), old.S("model"))
			}
			if x.Get("write") == nil {
				x.Set("write", old.Get("write"))
			}
			x.Set("dir", old.Get("dir"))
			if old.Has("worktree") {
				x.Set("worktree", old.Get("worktree"))
			}
			x.Set("session", result.Get("session"))
			x.Set("continues", Ref(prev, at))
		} else if t.B("worktree") {
			name := id
			if !single {
				name += "-" + safeWorktreeID(taskID)
			}
			x.Set("worktree", name)
		}
		if !x.B("model") {
			return nil, missing(where)
		}
		if _, e := agent.Parse(p, x.S("model")); e != nil {
			return nil, e
		}
		x.Set("write", x.B("write"))
		if x.B("worktree") && !x.B("write") {
			return nil, home.Usage("%s: a worktree is for write runs; add -w", where)
		}
		if _, e := os.Stat(x.S("dir")); e != nil {
			return nil, home.Usage("%s: no directory %s", where, x.S("dir"))
		}
		out = append(out, x)
	}
	return out, nil
}

// chain combines two file changes, dropping an added file that was subsequently deleted.
func chain(first, second string) string {
	if first == "added" {
		if second == "deleted" {
			return ""
		}
		return "added"
	}
	if first == "deleted" {
		if second == "added" {
			return "modified"
		}
		return "deleted"
	}
	return second
}

// combine keeps the last answer while summing time, usage, commits and cumulative file changes.
func combine(a, b home.Object) home.Object {
	var usage any
	parts := []home.Object{}
	for _, v := range []any{a.Get("usage"), b.Get("usage")} {
		if home.Truth(v) {
			part, _ := v.(home.Object)
			parts = append(parts, part)
		}
	}
	if len(parts) > 0 {
		sum := home.Object{}
		for _, part := range parts {
			for _, key := range []string{"input", "output", "cached"} {
				sum.Set(key, sum.N(key)+part.N(key))
			}
			if part.Has("cost") {
				sum.Set("cost", sum.N("cost")+part.N("cost"))
			}
		}
		usage = sum
	}
	x := b.Clone()
	x.Set("seconds", home.Number(home.Fixed(a.N("seconds")+b.N("seconds"), 1)))
	x.Set("usage", usage)
	x.Set("followups", a.N("followups")+1)
	if a.Has("changes") || b.Has("changes") {
		files := map[string]string{}
		if list, ok := a.Get("changes").([]any); ok {
			for _, v := range list {
				c, _ := v.(home.Object)
				files[c.S("path")] = c.S("change")
			}
		}
		if list, ok := b.Get("changes").([]any); ok {
			for _, v := range list {
				c, _ := v.(home.Object)
				path := c.S("path")
				if first, ok := files[path]; ok {
					files[path] = chain(first, c.S("change"))
				} else {
					files[path] = c.S("change")
				}
			}
		}
		paths := []string{}
		for p, c := range files {
			if c != "" {
				paths = append(paths, p)
			}
		}
		sort.Strings(paths)
		list := []any{}
		for _, p := range paths {
			list = append(list, home.O("path", p, "change", files[p]))
		}
		x.Set("changes", list)
		x.Set("commits", a.N("commits")+b.N("commits"))
		if a.B("worktree") && !b.B("worktree") && len(list) > 0 {
			x.Set("worktree", a.Get("worktree"))
		}
	}
	return x
}

// Event is a task's start, hook note/follow-up, or final result, with its position.
type Event struct {
	Kind    string
	Index   int
	Started task.Started
	Note    hooks.Note
	Result  home.Object
}

// withHooks applies refusal and result decisions, allowing at most three session follow-ups.
func withHooks(a *agent.Registry, h *hooks.Hooks, t home.Object, ref string, report func(Event)) (final home.Object) {
	var worktree *git.Worktree
	defer func() {
		if worktree == nil {
			return
		}
		files, commits := git.Changes(&git.Snapshot{Root: worktree.Path, Head: worktree.Base})
		if len(files) == 0 && commits == 0 {
			git.Remove(worktree)
			final.Delete("worktree")
		} else if final != nil {
			final.Set("worktree", home.O("path", worktree.Path, "branch", worktree.Branch))
		}
	}()
	if h != nil {
		changed, refused, notes := h.Before(t, ref)
		for _, n := range notes {
			report(Event{Kind: "note", Note: n})
		}
		t = changed
		if refused != "" {
			return home.O("id", t.Get("id"), "model", t.Get("model"), "name", t.Get("model"), "ok", false, "error", "refused by hook "+refused, "seconds", 0, "usage", nil, "session", nil, "dir", t.Get("dir"))
		}
	}
	started := func(info task.Started) { worktree = info.Worktree; report(Event{Kind: "start", Started: info}) }
	result := task.Run(a, t, started)
	for round := 0; h != nil; round++ {
		x := home.O("run", ref)
		for key, val := range result {
			x.Set(key, val)
		}
		followup, fail, notes := h.After(t, x, ref)
		for _, n := range notes {
			report(Event{Kind: "note", Note: n})
		}
		if fail != nil {
			result = result.Clone()
			result.Delete("answer")
			result.Delete("note")
			result.Set("ok", false)
			result.Set("error", "failed by hook "+fail.Name+": "+fail.Text)
			return result
		}
		if followup == nil {
			break
		}
		if round == 3 || !result.B("session") {
			text := "asked for a follow-up, but the agent reported no session"
			if round == 3 {
				text = "asked for a follow-up after 3; stopping"
			}
			report(Event{Kind: "note", Note: hooks.Note{Name: followup.Name, Text: text}})
			break
		}
		report(Event{Kind: "followup", Note: *followup})
		next := t.Clone()
		next.Set("prompt", followup.Text)
		next.Set("session", result.Get("session"))
		result = combine(result, task.Run(a, next, started))
	}
	return result
}

// Execute runs unfinished tasks up to jobs at once and atomically saves each completed result.
func Execute(a *agent.Registry, r *Run, jobs int, enabled bool, report func(Event)) ([]home.Object, error) {
	if e := lock(r); e != nil {
		return nil, e
	}
	defer os.Remove(filepath.Join(r.Dir, "lock"))
	var h *hooks.Hooks
	if enabled {
		h = hooks.New(a.Paths)
	}
	ordered := !enabled || len(find.Find(a.Paths, "hooks")) == 0
	todo := []int{}
	for i, x := range r.Results {
		if !x.B("ok") {
			todo = append(todo, i)
		}
	}
	if jobs > len(todo) {
		jobs = len(todo)
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	next := 0
	var failure error
	starts := make([]chan struct{}, len(todo)+1)
	for i := range starts {
		starts[i] = make(chan struct{})
	}
	close(starts[0])
	for w := 0; w < jobs; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				mu.Lock()
				if next >= len(todo) || failure != nil {
					mu.Unlock()
					return
				}
				position := next
				i := todo[position]
				next++
				mu.Unlock()
				var start sync.Once
				release := func() { start.Do(func() { close(starts[position+1]) }) }
				if ordered {
					<-starts[position]
				}
				result := withHooks(a, h, r.Tasks[i], Ref(r, i), func(e Event) {
					e.Index = i
					report(e)
					if e.Kind == "start" {
						release()
					}
				})
				release()

				x := home.O("run", Ref(r, i))
				for key, val := range result {
					x.Set(key, val)
				}
				mu.Lock()
				r.Results[i] = x
				e := home.WriteJSON(filepath.Join(r.Dir, "results.json"), home.ResultRecords(r.Results))
				if e != nil {
					failure = e
				}
				mu.Unlock()
				if e != nil {
					return
				}
				report(Event{Kind: "done", Index: i, Result: x})
			}
		}()
	}
	wg.Wait()
	return r.Results, failure
}

// Recent returns readable runs newest first, considering at most limit folders.
func Recent(p home.Paths, limit int) []*Run {
	names := []string{}
	for _, name := range find.List(p.Runs) {
		if strings.Contains(name, "-") {
			names = append(names, name)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	if len(names) > limit {
		names = names[:limit]
	}
	out := []*Run{}
	for _, name := range names {
		if r, e := load(filepath.Join(p.Runs, name)); e == nil {
			out = append(out, r)
		}
	}
	return out
}

// Stop sends SIGTERM to the live run owner and waits up to ten seconds for shutdown.
func Stop(r *Run) (bool, error) {
	pid := Owner(r)
	if pid == 0 {
		return false, nil
	}
	if e := syscall.Kill(pid, syscall.SIGTERM); e != nil {
		return false, e
	}
	for i := 0; i < 200 && Alive(pid); i++ {
		time.Sleep(50 * time.Millisecond)
	}
	return true, nil
}

// Package status formats the status lines and recent-run table shared by CLI commands.
package status

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/fschrhunt/ask/internal/home"
	"github.com/fschrhunt/ask/internal/runs"
)

// Duration prints seconds, minutes and seconds, or hours and minutes.
func Duration(seconds float64) string {
	if seconds < 60 {
		return home.Fixed(seconds, 1) + "s"
	}
	m := int(math.Floor(seconds / 60))
	if m < 60 {
		return fmt.Sprintf("%dm %02ds", m, int(math.Floor(math.Mod(seconds, 60)+0.5)))
	}
	return fmt.Sprintf("%dh %02dm", m/60, m%60)
}

// Usage prints token totals and optional cost, omitting empty reports.
func Usage(v any) string {
	u, _ := v.(home.Object)
	if !u.B("input") && !u.B("output") && !u.Has("cost") {
		return ""
	}
	k := func(n float64) string {
		if n >= 1000 {
			return home.Fixed(n/1000, 1) + "k"
		}
		return home.String(n)
	}
	text := k(u.N("input")) + " in · " + k(u.N("output")) + " out"
	if u.Has("cost") {
		digits := 4
		if u.N("cost") >= 1 {
			digits = 2
		}
		text += " · $" + home.Fixed(u.N("cost"), digits)
	}
	return text
}

// line joins nonempty status parts using the stable run-prefix format.
func line(ref string, parts ...string) string {
	all := []string{ref}
	for _, p := range parts {
		if p != "" {
			all = append(all, p)
		}
	}
	return "ask " + strings.Join(all, " · ")
}

// plural formats counts for changed files and commits.
func plural(n int, word string) string {
	s := ""
	if n != 1 {
		s = "s"
	}
	return fmt.Sprintf("%d %s%s", n, word, s)
}

// length measures text as UTF-16 units, matching the original table widths.
func length(s string) int { return len(utf16.Encode([]rune(s))) }

// clip truncates text at the original UTF-16 limit.
func clip(s string, n int) string {
	units := utf16.Encode([]rune(s))
	if len(units) <= n {
		return s
	}
	return string(utf16.Decode(units[:n]))
}

// EventLine formats a task start, hook event, or completed result.
func EventLine(r *runs.Run, e runs.Event) string {
	if e.Kind == "done" {
		return Done(e.Result)
	}
	ref := runs.Ref(r, e.Index)
	if e.Kind == "start" {
		t := r.Tasks[e.Index]
		where := home.Tilde(e.Started.Dir)
		if e.Started.Worktree != nil {
			where = "worktree " + home.Tilde(e.Started.Worktree.Path)
		}
		access := "read"
		if t.B("write") {
			access = "write"
		}
		continues := ""
		if t.B("continues") {
			continues = "continues " + t.S("continues")
		}
		text := line(ref, t.S("model"), access, where, continues, "started")
		if e.Started.Worktree != nil && e.Started.Worktree.Dirty {
			text += "\n" + line(ref, "the worktree starts from HEAD; uncommitted changes in "+home.Tilde(t.S("dir"))+" are not in it")
		}
		return text
	}
	first, _, _ := strings.Cut(e.Note.Text, "\n")
	if length(first) > 100 {
		first = clip(first, 99) + "…"
	}
	if e.Kind == "followup" {
		first = "follow-up: " + first
	}
	return line(ref, "hook "+e.Note.Name, first)
}

// Done formats a completed result, including its failure reason or successful note.
func Done(r home.Object) string {
	outcome := "failed"
	note := r.S("error")
	if r.B("ok") {
		outcome = "ok"
		note = r.S("note")
	}
	changed := ""
	if r.Has("changes") {
		a, _ := r.Get("changes").([]any)
		parts := []string{}
		if len(a) > 0 {
			parts = append(parts, plural(len(a), "file")+" changed")
		}
		if r.N("commits") != 0 {
			parts = append(parts, plural(int(r.N("commits")), "commit"))
		}
		changed = strings.Join(parts, ", ")
		if changed == "" {
			changed = "no changes"
		}
	}
	branch := ""
	if w, ok := r.Get("worktree").(home.Object); ok {
		branch = "branch " + w.S("branch")
	}
	return line(r.S("run"), r.S("name"), outcome, Duration(r.N("seconds")), changed, branch, Usage(r.Get("usage")), note)
}

// BatchStart prints the batch size and effective parallelism, including resume state.
func BatchStart(r *runs.Run, jobs, todo int) string {
	resume := ""
	if todo < len(r.Tasks) {
		resume = fmt.Sprintf("resuming %d unfinished", todo)
	}
	if jobs > todo {
		jobs = todo
	}
	return line(r.ID, fmt.Sprintf("batch of %d", len(r.Tasks)), resume, fmt.Sprintf("%d at a time", jobs))
}

// BatchEnd prints outcome counts, elapsed time and summed usage.
func BatchEnd(r *runs.Run, seconds float64) string {
	ok := 0
	total := home.Object{}
	for _, x := range r.Results {
		if x.B("ok") {
			ok++
		}
		if u, yes := x.Get("usage").(home.Object); yes {
			for _, key := range []string{"input", "output", "cached"} {
				total.Set(key, total.N(key)+u.N(key))
			}
			if u.Has("cost") {
				total.Set("cost", total.N("cost")+u.N("cost"))
			}
		}
	}
	return line(r.ID, fmt.Sprintf("%d/%d ok", ok, len(r.Tasks)), Duration(seconds), Usage(total))
}

// when renders a UTC record timestamp in the local timezone, using today's shorter form.
func when(stamp string) string {
	if len(stamp) < 15 {
		return ""
	}
	date, e := time.Parse("20060102T150405", stamp[:15])
	if e != nil {
		return ""
	}
	date = date.In(time.Local)
	clock := date.Format("15:04")
	now := time.Now()
	if date.Format("2006-01-02") == now.Format("2006-01-02") {
		return clock
	}
	return date.Format("Jan ") + strconv.Itoa(date.Day()) + " " + clock
}

// Table prints aligned recent runs with a 120-column default outside a terminal.
func Table(list []*runs.Run) string {
	rows := [][]string{{"RUN", "STARTED", "STATUS", "MODEL", "TIME", "TASK"}}
	for _, r := range list {
		done := []home.Object{}
		ok := 0
		for _, x := range r.Results {
			if x != nil {
				done = append(done, x)
				if x.B("ok") {
					ok++
				}
			}
		}
		single := len(r.Tasks) == 1
		state := ""
		switch {
		case runs.Owner(r) != 0:
			state = "running"
		case len(done) < len(r.Tasks):
			state = "stopped"
		case single:
			state = "failed"
			if done[0].B("ok") {
				state = "ok"
			}
		default:
			state = fmt.Sprintf("%d/%d ok", ok, len(r.Tasks))
		}
		first := r.Tasks[0]
		model := fmt.Sprintf("%d tasks", len(r.Tasks))
		elapsed := ""
		if single {
			model = first.S("model")
			if len(done) > 0 {
				if done[0].B("name") {
					model = done[0].S("name")
				}
				elapsed = Duration(done[0].N("seconds"))
			}
		}
		about, _, _ := strings.Cut(first.S("prompt"), "\n")
		if first.B("continues") {
			about = "↪ " + first.S("continues") + " " + about
		}
		if state == "stopped" {
			about = "resume: ask batch --resume " + r.ID
		}
		rows = append(rows, []string{r.ID, when(r.Created), state, model, elapsed, about})
	}
	widths := make([]int, 6)
	for _, row := range rows {
		for c, cell := range row {
			if length(cell) > widths[c] {
				widths[c] = length(cell)
			}
		}
	}
	room := terminalWidth()
	for _, w := range widths[:5] {
		room -= w + 2
	}
	if room < 20 {
		room = 20
	}
	out := []string{}
	for _, row := range rows {
		cells := []string{}
		for c, cell := range row[:5] {
			cells = append(cells, cell+strings.Repeat(" ", widths[c]-length(cell)))
		}
		about := row[5]
		if length(about) > room {
			about = clip(about, room-1) + "…"
		}
		cells = append(cells, about)
		out = append(out, strings.TrimRight(strings.Join(cells, "  "), " "))
	}
	return strings.Join(out, "\n")
}

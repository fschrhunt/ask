package status

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/fschrhunt/ask/internal/home"
	"github.com/fschrhunt/ask/internal/runs"
)

// LiveRow holds one task's latest state; Started is reset for each follow-up round.
type LiveRow struct {
	Ref, Name, Access, Dir, State, Reason, Final string
	Started                                      time.Time
	Notes                                        []string
	Usage                                        any // usage so far while running, then the result's
}

// LiveState is a terminal frame's data, independent of processes and terminal I/O.
type LiveState struct {
	Header  string
	Rows    []LiveRow
	Batch   bool
	Started time.Time
}

// cells measures common terminal characters, including combining marks and wide CJK/emoji.
func cells(r rune) int {
	if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || r == 0x200d || r == 0xfe0f {
		return 0
	}
	if r >= 0x1100 && (r <= 0x115f || r == 0x2329 || r == 0x232a || (r >= 0x2e80 && r <= 0xa4cf) || (r >= 0xac00 && r <= 0xd7a3) || (r >= 0xf900 && r <= 0xfaff) || (r >= 0xfe10 && r <= 0xfe6f) || (r >= 0xff01 && r <= 0xff60) || (r >= 0xffe0 && r <= 0xffe6) || (r >= 0x1f300 && r <= 0x1faff) || (r >= 0x20000 && r <= 0x3ffff)) {
		return 2
	}
	return 1
}

// ClipLine removes terminal controls and clips a line to display cells, reserving room for ellipsis.
func ClipLine(text string, width int) string {
	clean := []rune(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, text))
	total := 0
	for _, r := range clean {
		total += cells(r)
	}
	if total <= width {
		return string(clean)
	}
	if width < 1 {
		return ""
	}
	used := 0
	end := 0
	for i, r := range clean {
		if used+cells(r) > width-1 {
			break
		}
		used += cells(r)
		end = i + 1
	}
	return string(clean[:end]) + "…"
}

// Frame renders live state without terminal I/O; width reserves the last column to avoid wrapping.
// Height bounds redraws to a screen, showing a count when a large batch needs more rows.
func Frame(s LiveState, now time.Time, tick, width, height int, color, utf8 bool) []string {
	lines := []string{}
	if s.Batch {
		header := ClipLine(s.Header, width-1)
		if color {
			header = "\x1b[1m" + header + "\x1b[0m"
		}
		lines = append(lines, header)
	}
	nameWidth := 0
	for _, r := range s.Rows {
		if n := length(r.Name); n > nameWidth {
			nameWidth = n
		}
	}
	if nameWidth > 24 {
		nameWidth = 24
	}
	// A batch's header names the run, so its rows show only each task's own id, in one column.
	taskID := func(ref string) string { return ref[strings.Index(ref, "/")+1:] }
	idWidth := 0
	for _, r := range s.Rows {
		idWidth = min(max(idWidth, length(taskID(r.Ref))), 16)
	}
	ok, failed := 0, 0
	for _, r := range s.Rows {
		state := r.State
		style := ""
		text := ""
		mark := "·"
		if !utf8 {
			mark = "-"
		}
		switch state {
		case "running":
			mark = "*"
			if utf8 {
				mark = string([]rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")[tick%10])
			}
			text = r.Access + " · " + r.Dir + " · " + Clock(now.Sub(r.Started))
			if u := Usage(r.Usage); u != "" {
				text += " · " + u
			}
		case "ok", "failed":
			if state == "ok" {
				style = "32"
				mark = "+"
				if utf8 {
					mark = "✓"
				}
				ok++
			} else {
				style = "31"
				mark = "x"
				if utf8 {
					mark = "✗"
				}
				failed++
			}
			text = strings.TrimPrefix(r.Final, "ask "+r.Ref+" · "+state+" · ")
			text = strings.TrimPrefix(text, r.Name+" · ")
			if text == "" {
				text = r.Reason
			}
		default:
			text = "queued"
		}
		if s.Batch {
			text = fmt.Sprintf("%s %-*s  %-*s  %s", mark, idWidth, ClipLine(taskID(r.Ref), idWidth), nameWidth, r.Name, text)
		} else {
			left := fmt.Sprintf("%s %s   %s", mark, r.Name, text)
			gap := width - 1 - length(left) - length(r.Ref)
			if gap < 2 {
				gap = 2
			}
			text = left + strings.Repeat(" ", gap) + r.Ref
		}
		text = ClipLine(text, width-1)
		if color && style != "" && strings.HasPrefix(text, mark) {
			text = "\x1b[" + style + "m" + mark + "\x1b[0m" + strings.TrimPrefix(text, mark)
		}
		lines = append(lines, text)
		for _, note := range r.Notes {
			note = ClipLine("  "+note, width-1)
			if color {
				note = "\x1b[2m" + note + "\x1b[0m"
			}
			lines = append(lines, note)
		}
	}
	if s.Batch {
		footer := fmt.Sprintf("%d/%d ok", ok, len(s.Rows))
		if failed > 0 {
			footer += fmt.Sprintf(" · %d failed", failed)
		}
		footer += " · " + Clock(now.Sub(s.Started))
		all := []any{}
		for _, r := range s.Rows {
			all = append(all, r.Usage)
		}
		if u := Usage(runs.AddUsage(all...)); u != "" {
			footer += " · " + u
		}
		lines = append(lines, ClipLine(footer, width-1))
	}
	if height > 1 && len(lines) >= height {
		hidden := len(lines) - height + 2
		lines = append(lines[:height-2], ClipLine(fmt.Sprintf("%d more lines", hidden), width-1))
	}
	return lines
}

// Live owns a stderr frame and serializes updates with refreshes and final output.
// Call Finish before writing answers to stdout, including when stdout shares the terminal.
type Live struct {
	mu                  sync.Mutex
	state               LiveState
	lines, tick         int
	color, utf8, closed bool
	stop                chan struct{}
}

// NewLive returns nil for pipes or dumb terminals, preserving the plain status contract.
func NewLive(r *runs.Run, header string, batch bool) *Live {
	if !IsTerminal(os.Stderr) {
		return nil
	}
	l := &Live{state: LiveState{Header: header, Batch: batch, Started: time.Now()}, color: CanStyle(os.Stderr), utf8: localeUTF8(), stop: make(chan struct{})}
	for i, t := range r.Tasks {
		row := LiveRow{Ref: runs.Ref(r, i), Name: t.S("model"), State: "queued"}
		if result := r.Results[i]; result.B("ok") {
			row.Name = result.S("name")
			row.State = "ok"
			row.Final = Done(result)
			row.Usage = result.Get("usage")
		}
		l.state.Rows = append(l.state.Rows, row)
	}
	fmt.Fprint(os.Stderr, "\x1b[?25l")
	l.draw()
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				l.mu.Lock()
				if !l.closed {
					l.tick++
					l.draw()
				}
				l.mu.Unlock()
			case <-l.stop:
				return
			}
		}
	}()
	return l
}

// erase returns to the start of the previous bounded frame and clears it.
func (l *Live) erase() {
	if l.lines > 0 {
		fmt.Fprintf(os.Stderr, "\x1b[%dA\r\x1b[J", l.lines)
		l.lines = 0
	}
}

// draw emits complete clipped rows with wrapping disabled during the redraw.
func (l *Live) draw() {
	l.erase()
	width, height, _ := terminalSize(os.Stderr)
	if width < 2 {
		width = 2
	}
	frame := Frame(l.state, time.Now(), l.tick, width, height, l.color, l.utf8)
	fmt.Fprint(os.Stderr, "\x1b[?7l")
	for _, line := range frame {
		fmt.Fprint(os.Stderr, "\r\x1b[2K", line, "\r\n")
	}
	fmt.Fprint(os.Stderr, "\x1b[?7h")
	l.lines = len(frame)
}

// Update replaces a task's state or prints a hook note above the live frame.
func (l *Live) Update(e runs.Event, name, note string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return
	}
	row := &l.state.Rows[e.Index]
	switch e.Kind {
	case "start":
		row.Name = name
		row.Access = "read"
		if e.Started.Task.B("write") {
			row.Access = "write"
		}
		row.Dir = home.Tilde(e.Started.Dir)
		if e.Started.Worktree != nil {
			row.Dir = "worktree " + row.Dir
		}
		row.State = "running"
		row.Started = time.Now()
		if _, extra, ok := strings.Cut(note, "\n"); ok {
			row.Notes = append(row.Notes, strings.TrimPrefix(extra, "ask "+row.Ref+" · note · "))
		}
	case "usage":
		row.Usage = e.Result
	case "done":
		row.Usage = e.Result.Get("usage")
		row.Name = e.Result.S("name")
		row.State = "failed"
		if e.Result.B("ok") {
			row.State = "ok"
		}
		row.Final = Done(e.Result)
	default:
		row.Notes = append(row.Notes, strings.TrimPrefix(note, "ask "+row.Ref+" · "))
	}
	l.draw()
}

// Finish stops refreshes, leaves the final state visible and restores the cursor exactly once.
func (l *Live) Finish(summary string, stopped bool) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return
	}
	l.closed = true
	close(l.stop)
	if stopped {
		for i := range l.state.Rows {
			row := &l.state.Rows[i]
			if row.State == "running" || row.State == "queued" {
				row.State = "failed"
				row.Final = ""
				row.Reason = "stopped"
			}
		}
	}
	l.draw()
	fmt.Fprint(os.Stderr, "\x1b[?25h")
	if summary != "" && (stopped || l.state.Batch) {
		fmt.Fprintln(os.Stderr, summary)
	}
}

// localeUTF8 chooses Unicode marks only when the active locale can display them.
func localeUTF8() bool {
	for _, key := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if locale, ok := os.LookupEnv(key); ok && locale != "" {
			locale = strings.ToLower(locale)
			return strings.Contains(locale, "utf-8") || strings.Contains(locale, "utf8")
		}
	}
	return false
}

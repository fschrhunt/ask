// Package process runs agents and hooks in process groups, with bounded shutdown.
// SIGINT and SIGTERM stop all groups and exit 130; no new process starts afterwards.
package process

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"

	"github.com/fschrhunt/ask/internal/home"
)

var groups = struct {
	sync.Mutex
	stopping bool
	pids     map[int]bool
}{pids: map[int]bool{}}

// Listen installs ask's signal handlers before any agent or hook is started.
func Listen() {
	c := make(chan os.Signal, 2)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		groups.Lock()
		groups.stopping = true
		pids := []int{}
		for pid := range groups.pids {
			pids = append(pids, pid)
		}
		groups.Unlock()
		stopGroups(pids)
		os.Exit(130)
	}()
}

// AwaitShutdown keeps the entry point alive until a signal handler has stopped every group.
func AwaitShutdown() {
	groups.Lock()
	stopping := groups.stopping
	groups.Unlock()
	if stopping {
		select {}
	}
}

// groupAlive checks whether any member of a process group remains.
func groupAlive(pid int) bool { return syscall.Kill(-pid, 0) == nil }

// stopGroups sends TERM, waits five seconds, then sends KILL and waits briefly.
func stopGroups(pids []int) {
	live := []int{}
	for _, pid := range pids {
		if groupAlive(pid) {
			live = append(live, pid)
			_ = syscall.Kill(-pid, syscall.SIGTERM)
		}
	}
	anyAlive := func() bool {
		for _, pid := range live {
			if groupAlive(pid) {
				return true
			}
		}
		return false
	}
	for deadline := time.Now().Add(5 * time.Second); anyAlive() && time.Now().Before(deadline); {
		time.Sleep(50 * time.Millisecond)
	}
	for _, pid := range live {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}
	for deadline := time.Now().Add(time.Second); anyAlive() && time.Now().Before(deadline); {
		time.Sleep(20 * time.Millisecond)
	}
}

// Options specifies stdin, working directory, environment and the hard time limit.
type Options struct {
	Input, Dir string
	Env        []string
	Timeout    time.Duration
}

// Result contains decoded output, exit code (-1 for signals/spawn errors), and timeout state.
// Fatal identifies start errors that bypassed the original runtime's process error callback.
type Result struct {
	Code           int
	Stdout, Stderr string
	TimedOut       bool
	Fatal          bool
}

type tail struct{ b []byte }

// Write retains enough raw stderr for the last 64K UTF-16 units after decoding.
func (t *tail) Write(b []byte) (int, error) {
	n := len(b)
	t.b = append(t.b, b...)
	if len(t.b) > 4*64*1024 {
		t.b = t.b[len(t.b)-4*64*1024:]
	}
	return n, nil
}

// Run resolves after the child and its leftovers stop; failures become Result, never panics.
// Open descendant pipes retain the original timeout behavior even after the agent has exited.
func Run(command string, args []string, o Options) Result {
	r := Result{Code: -1}
	cmd := exec.Command(command, args...)
	cmd.Dir = o.Dir
	cmd.Env = o.Env
	cmd.SysProcAttr = attributes()
	inR, inW, e := os.Pipe()
	if e != nil {
		r.Stderr = e.Error()
		return r
	}
	defer inR.Close()
	outR, outW, e := os.Pipe()
	if e != nil {
		inW.Close()
		r.Stderr = e.Error()
		return r
	}
	defer outR.Close()
	errR, errW, e := os.Pipe()
	if e != nil {
		inW.Close()
		outW.Close()
		r.Stderr = e.Error()
		return r
	}
	defer errR.Close()
	cmd.Stdin = inR
	cmd.Stdout = outW
	cmd.Stderr = errW
	groups.Lock()
	if groups.stopping {
		groups.Unlock()
		inW.Close()
		outW.Close()
		errW.Close()
		r.Stderr = "ask was stopped"
		return r
	}
	e = cmd.Start()
	if e == nil {
		groups.pids[cmd.Process.Pid] = true
	}
	groups.Unlock()
	outW.Close()
	errW.Close()
	if e != nil {
		inW.Close()
		r.Stderr = SpawnError(command, e) + "\n"
		r.Fatal = errors.Is(e, syscall.ENOEXEC) || errors.Is(e, syscall.ENOTDIR)
		return r
	}
	go func() { defer inW.Close(); _, _ = io.WriteString(inW, o.Input) }()
	outDone, errDone := make(chan string, 1), make(chan string, 1)
	go func() { b, _ := io.ReadAll(outR); outDone <- home.UTF8(b) }()
	go func() {
		t := &tail{}
		_, _ = io.Copy(t, errR)
		units := utf16.Encode([]rune(home.UTF8(t.b)))
		if len(units) > 64*1024 {
			units = units[len(units)-64*1024:]
		}
		errDone <- string(utf16.Decode(units))
	}()
	closed := make(chan Result, 1)
	go func() {
		_ = cmd.Wait()
		closed <- Result{Code: cmd.ProcessState.ExitCode(), Stdout: <-outDone, Stderr: <-errDone}
	}()
	d := o.Timeout
	if d < time.Millisecond || d > 2147483647*time.Millisecond {
		d = time.Millisecond
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case r = <-closed:
	case <-timer.C:
		stopGroups([]int{cmd.Process.Pid})
		r = <-closed
		r.TimedOut = true
	}
	if !r.TimedOut {
		stopGroups([]int{cmd.Process.Pid})
	}
	groups.Lock()
	delete(groups.pids, cmd.Process.Pid)
	groups.Unlock()
	return r
}

// SpawnError renders process-start errors in the existing agent and command vocabulary.
func SpawnError(command string, err error) string {
	code := "ENOENT"
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case syscall.EACCES:
			code = "EACCES"
		case syscall.ENOTDIR:
			code = "ENOTDIR"
		case syscall.ENOEXEC:
			code = "ENOEXEC"
		case syscall.EPERM:
			code = "EPERM"
		case syscall.EMFILE:
			code = "EMFILE"
		case syscall.ENFILE:
			code = "ENFILE"
		case syscall.E2BIG:
			code = "E2BIG"
		}
	}
	if code == "ENOEXEC" || code == "ENOTDIR" {
		return "spawn " + code
	}
	return fmt.Sprintf("spawn %s %s", command, code)
}

var errorLine = regexp.MustCompile(`^(\w*(Error|Exception)|panic)\b[^:]*:`)

// Reason selects the last error line ahead of runtime stack traces and closing banners.
func Reason(stderr string) string {
	last, found := "", ""
	for _, line := range strings.Split(stderr, "\n") {
		line = home.Trim(line)
		if line == "" {
			continue
		}
		last = line
		if errorLine.MatchString(line) {
			found = line
		}
	}
	if found != "" {
		last = found
	}
	r := utf16.Encode([]rune(last))
	if len(r) > 300 {
		return string(utf16.Decode(r[:300]))
	}
	return last
}

package test

import (
	"fmt"
	"path/filepath"
	"syscall"
	"testing"
)

// TestCLI pins option validation, prompt/answer formatting and process shutdown.
func TestCLI(t *testing.T) {
	t.Run("no arguments print the help", func(t *testing.T) { s := fresh(t); r := s.ask(); eq(t, r.code, 0); match(t, r.stdout, `ask -m MODEL`) })
	t.Run("a missing -m is a usage error that lists the models", func(t *testing.T) {
		s := fresh(t)
		s.json(filepath.Join(s.home, "models.json"), object{"fake": []string{"extra"}})
		r := s.ask("hello")
		eq(t, r.code, 2)
		match(t, r.stderr, `needs a model`)
		match(t, r.stderr, `fake +small  +big`)
		match(t, r.stderr, `fake +small  +big  +extra`)
		eq(t, len(s.calls()), 0)
	})
	t.Run("a malformed model or an unknown agent is a usage error", func(t *testing.T) {
		s := fresh(t)
		bad := s.ask("-m", "opus", "hello")
		eq(t, bad.code, 2)
		match(t, bad.stderr, `expected agent:id`)
		unknown := s.ask("-m", "gemini:x", "hello")
		eq(t, unknown.code, 2)
		match(t, unknown.stderr, `no agent "gemini" in .*; installed: fake;`)
	})
	t.Run("-r and -w together are refused", func(t *testing.T) {
		s := fresh(t)
		r := s.ask("-m", "fake:small", "-r", "-w", "hello")
		eq(t, r.code, 2)
		match(t, r.stderr, `choose one of -r`)
		eq(t, len(s.calls()), 0)
	})
	t.Run("a models.json that does not parse is a usage error", func(t *testing.T) {
		s := fresh(t)
		s.write(filepath.Join(s.home, "models.json"), "{")
		r := s.ask("models")
		eq(t, r.code, 2)
		match(t, r.stderr, `cannot parse`)
	})
	t.Run("a JSON string answer is printed as JSON", func(t *testing.T) {
		s := fresh(t)
		r := s.run([]string{"-m", "fake:big", "--json", "go"}, "", map[string]string{"FAKE_ANSWER": `"hello"`})
		eq(t, r.stdout, "\"hello\"\n")
	})
	t.Run("--json strips a code fence around the answer", func(t *testing.T) {
		s := fresh(t)
		r := s.run([]string{"-m", "fake:big", "--json", "go"}, "", map[string]string{"FAKE_ANSWER": "```json\n{\"a\": 1}\n```"})
		jsonEqual(t, obj(t, r.stdout), object{"a": 1})
		match(t, s.calls()[0].s("stdin"), `Answer ONLY with JSON`)
	})
	t.Run("an answer that is not valid JSON fails under --json", func(t *testing.T) {
		s := fresh(t)
		r := s.run([]string{"-m", "fake:big", "--json", "go"}, "", map[string]string{"FAKE_ANSWER": "sure thing"})
		eq(t, r.code, 1)
		match(t, r.stderr, `not valid JSON`)
	})
	t.Run("-C sets the directory the model works in", func(t *testing.T) {
		s := fresh(t)
		s.mkdir(filepath.Join(s.tmp, "work"))
		s.ask("-m", "fake:small", "-C", filepath.Join(s.tmp, "work"), "hi")
		eq(t, s.calls()[0].s("cwd"), filepath.Join(s.tmp, "work"))
	})
	t.Run("the prompt is read from stdin when none is given", func(t *testing.T) {
		s := fresh(t)
		r := s.run([]string{"-m", "fake:small"}, "from stdin\n", nil)
		eq(t, r.stdout, "fake: from stdin\n")
	})
	t.Run("a hung model times out and its process is killed", func(t *testing.T) {
		s := fresh(t)
		r := s.run([]string{"-m", "fake:small", "-t", "0.5", "hang"}, "", map[string]string{"FAKE_HANG": "hang"})
		eq(t, r.code, 1)
		match(t, r.stderr, `timed out`)
		eq(t, alive(int(s.calls()[0].n("pid"))), false)
	})
	t.Run("stopping ask with SIGINT stops the models it started", func(t *testing.T) {
		s := fresh(t)
		run := s.start([]string{"-m", "fake:big", "hang"}, "", map[string]string{"FAKE_HANG": "hang"})
		until(t, func() bool { return len(s.calls()) == 1 })
		pid := int(s.calls()[0].n("pid"))
		if e := run.cmd.Process.Signal(syscall.SIGINT); e != nil {
			t.Fatal(e)
		}
		eq(t, run.wait(t).code, 130)
		until(t, func() bool { return !alive(pid) })
	})
	t.Run("a failing model exits 1 with its message", func(t *testing.T) {
		s := fresh(t)
		r := s.run([]string{"-m", "fake:big", "break"}, "", map[string]string{"FAKE_FAIL": "break"})
		eq(t, r.code, 1)
		match(t, r.stderr, `(?m) · failed · .*fake boom$`)
		eq(t, r.stdout, "")
	})
	t.Run("stopping ask kills a model that ignores SIGTERM before exiting", func(t *testing.T) {
		s := fresh(t)
		run := s.start([]string{"-m", "fake:big", "hang"}, "", map[string]string{"FAKE_HANG": "hang", "FAKE_IGNORE_TERM": "1"})
		until(t, func() bool { return len(s.calls()) == 1 })
		pid := int(s.calls()[0].n("pid"))
		if e := run.cmd.Process.Signal(syscall.SIGINT); e != nil {
			t.Fatal(e)
		}
		eq(t, run.wait(t).code, 130)
		eq(t, alive(pid), false)
	})
	t.Run("agents get the prompt as given, with the access in ASK_ACCESS", func(t *testing.T) {
		s := fresh(t)
		s.ask("-m", "fake:small", "where is main?")
		s.ask("-m", "fake:small", "-w", "fix it")
		calls := s.calls()
		eq(t, calls[0].s("stdin")+","+calls[1].s("stdin"), "where is main?,fix it")
		eq(t, calls[0].s("access")+","+calls[1].s("access"), "read,write")
	})
	t.Run("the answer goes to stdout and a status line with usage to stderr", func(t *testing.T) {
		s := fresh(t)
		r := s.ask("-m", "fake:small", "hi")
		eq(t, r.stdout, "fake: hi\n")
		id := runID(t, r.stderr)
		match(t, r.stderr, fmt.Sprintf(`^ask %s · started · Small One · read · .+\nask %s · ok · Fake 1\.0 · [\d.]+s · 10 in · 5 out · \$0\.01\n$`, id, id))
	})
	t.Run("a run that reports no usage prints no usage", func(t *testing.T) {
		s := fresh(t)
		r := s.run([]string{"-m", "fake:small", "hi"}, "", map[string]string{"FAKE_NO_USAGE": "1"})
		eq(t, r.code, 0)
		noMatch(t, r.stderr, ` in .* out`)
	})
	t.Run("an answer from an agent that exited nonzero fails", func(t *testing.T) {
		s := fresh(t)
		r := s.run([]string{"-m", "fake:small", "hi"}, "", map[string]string{"FAKE_EXIT": "1"})
		eq(t, r.code, 1)
		match(t, r.stderr, `(?m) · failed · .*exit 1$`)
		eq(t, r.stdout, "")
	})
	t.Run("--schema answers are checked, then printed as JSON", func(t *testing.T) {
		s := fresh(t)
		path := filepath.Join(s.tmp, "s.json")
		s.json(path, object{"type": "object", "properties": object{"a": object{"type": "number"}}, "required": []string{"a"}})
		good := s.run([]string{"-m", "fake:small", "--schema", path, "go"}, "", map[string]string{"FAKE_ANSWER": `{"a": 1}`})
		jsonEqual(t, obj(t, good.stdout), object{"a": 1})
		match(t, s.calls()[0].s("stdin"), `Answer ONLY with JSON matching this JSON Schema`)
		bad := s.run([]string{"-m", "fake:small", "--schema", path, "go"}, "", map[string]string{"FAKE_ANSWER": `{"a": "one"}`})
		eq(t, bad.code, 1)
		match(t, bad.stderr, `does not match the schema: \$\.a: expected number, got string`)
		eq(t, bad.stdout, "")
	})
	t.Run("a relative -C is resolved once, against where ask runs", func(t *testing.T) {
		s := fresh(t)
		s.mkdir(filepath.Join(s.tmp, "work"))
		r := s.ask("-m", "fake:small", "-C", "work", "hi")
		eq(t, r.code, 0)
		eq(t, s.calls()[0].s("cwd"), filepath.Join(s.tmp, "work"))
	})
	t.Run("numbers are checked: -t, -j and a task timeout must be above 0, and -t at most 2000000", func(t *testing.T) {
		s := fresh(t)
		for _, args := range [][]string{{"-m", "fake:small", "-t", "abc", "hi"}, {"batch", "-j", "0", "-m", "fake:small", "-"}} {
			r := s.run(args, `[{"prompt": "a"}]`, nil)
			eq(t, r.code, 2)
			match(t, r.stderr, `needs a (whole )?number above 0`)
		}
		long := s.ask("-m", "fake:small", "-t", "inf", "hi")
		eq(t, long.code, 2)
		match(t, long.stderr, `-t needs at most 2000000 seconds, not "inf"`)
		task := s.run([]string{"batch", "-m", "fake:small", "-"}, `[{"prompt": "a", "timeout": "soon"}]`, nil)
		match(t, task.stderr, `task 1: timeout must be a number of seconds above 0, at most 2000000`)
		eq(t, len(s.calls()), 0)
	})
	t.Run("words after -- are the prompt, even ones that look like options", func(t *testing.T) {
		s := fresh(t)
		r := s.ask("-m", "fake:small", "--", "-w", "is a flag")
		eq(t, r.stdout, "fake: -w is a flag\n")
		eq(t, s.calls()[0].s("access"), "read")
	})
	t.Run("schema checks use own properties and compare enums by value", func(t *testing.T) {
		s := fresh(t)
		req := filepath.Join(s.tmp, "req.json")
		s.json(req, object{"type": "object", "required": []string{"constructor"}})
		missing := s.run([]string{"-m", "fake:small", "--schema", req, "go"}, "", map[string]string{"FAKE_ANSWER": "{}"})
		match(t, missing.stderr, `missing "constructor"`)
		enum := filepath.Join(s.tmp, "enum.json")
		s.json(enum, object{"enum": []object{{"a": 1, "b": 2}}})
		reordered := s.run([]string{"-m", "fake:small", "--schema", enum, "go"}, "", map[string]string{"FAKE_ANSWER": `{"b": 2, "a": 1}`})
		eq(t, reordered.code, 0)
	})
	t.Run("a report that is not an object is ignored, not fatal", func(t *testing.T) {
		s := fresh(t)
		s.script("agents", "odd", `echo null > "$ASK_REPORT"; echo fine`)
		r := s.ask("-m", "odd:x", "go")
		eq(t, r.code, 0)
		eq(t, r.stdout, "fine\n")
	})
}

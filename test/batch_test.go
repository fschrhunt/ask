package test

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestBatch pins position-based recording, defaults, parallel results and resumption.
func TestBatch(t *testing.T) {
	t.Run("batch keeps task order and records tasks.json and results.json", func(t *testing.T) {
		s := fresh(t)
		r := s.run([]string{"batch", "-j", "2", "-"}, `[{"id":"slow","prompt":"one slow","model":"fake:small"},{"id":"fast","prompt":"two","model":"fake:big"}]`, map[string]string{"FAKE_SLOW": "slow"})
		eq(t, r.code, 0)
		eq(t, fields(objects(t, r.stdout), "id", "ok", "answer"), [][]any{{"slow", true, "fake: one slow"}, {"fast", true, "fake: two"}})
		eq(t, len(s.dirs()), 1)
		eq(t, stringsField(s.runFile("tasks.json"), "id"), []string{"slow", "fast"})
		eq(t, stringsField(s.runFile("results.json"), "id"), []string{"slow", "fast"})
	})
	t.Run("batch accepts JSON lines, with -m as the default model and a task model winning", func(t *testing.T) {
		s := fresh(t)
		r := s.run([]string{"batch", "-m", "fake:small"}, "{\"prompt\": \"a\"}\n{\"prompt\": \"b\", \"model\": \"fake:big\"}", nil)
		eq(t, fields(objects(t, r.stdout), "id", "model"), [][]any{{"1", "fake:small"}, {"2", "fake:big"}})
	})
	t.Run("a batch task whose write is not a boolean is a usage error", func(t *testing.T) {
		s := fresh(t)
		r := s.run([]string{"batch", "-m", "fake:small"}, `[{"prompt":"a","write":"false"}]`, nil)
		eq(t, r.code, 2)
		match(t, r.stderr, `"write" must be true or false`)
		eq(t, len(s.calls()), 0)
	})
	t.Run("batch -w is the default write access, and a task may override it", func(t *testing.T) {
		s := fresh(t)
		s.run([]string{"batch", "-w", "-m", "fake:small"}, `[{"prompt":"a"},{"prompt":"b","write":false}]`, nil)
		eq(t, sorted(stringsField(s.calls(), "access")), []string{"read", "write"})
		eq(t, fields(s.runFile("tasks.json"), "write"), [][]any{{true}, {false}})
	})
	t.Run("a batch task without a model is a usage error that lists the models", func(t *testing.T) {
		s := fresh(t)
		r := s.run([]string{"batch"}, `[{"prompt":"a"}]`, nil)
		eq(t, r.code, 2)
		match(t, r.stderr, `task 1 needs a model`)
		match(t, r.stderr, `fake:small`)
	})
	t.Run("an empty batch is a usage error", func(t *testing.T) {
		s := fresh(t)
		for _, input := range []string{"", "[]"} {
			r := s.run([]string{"batch", "-m", "fake:small"}, input, nil)
			eq(t, r.code, 2)
			match(t, r.stderr, `no tasks`)
		}
	})
	t.Run("a failed batch task is reported and the batch exits 1", func(t *testing.T) {
		s := fresh(t)
		r := s.run([]string{"batch", "-m", "fake:small"}, `[{"prompt":"fine"},{"prompt":"break"}]`, map[string]string{"FAKE_FAIL": "break"})
		eq(t, r.code, 1)
		result := objects(t, r.stdout)
		eq(t, result[0].b("ok"), true)
		eq(t, result[1].b("ok"), false)
		eq(t, result[1].s("error"), "fake boom")
	})
	t.Run("batch --resume reruns only the tasks that did not finish", func(t *testing.T) {
		s := fresh(t)
		s.run([]string{"batch", "-m", "fake:small"}, `[{"prompt":"fine"},{"prompt":"break"}]`, map[string]string{"FAKE_FAIL": "break"})
		eq(t, len(s.calls()), 2)
		r := s.ask("batch", "--resume", filepath.Join(s.home, "runs", s.dirs()[0]))
		eq(t, r.code, 0)
		eq(t, len(s.calls()), 3)
		match(t, s.calls()[2].s("stdin"), `break$`)
		eq(t, fields(objects(t, r.stdout), "ok"), [][]any{{true}, {true}})
	})
	t.Run("batch --resume reruns a failed task even when a finished one shares its id", func(t *testing.T) {
		s := fresh(t)
		s.run([]string{"batch", "-m", "fake:small"}, `[{"id":"x","prompt":"fine"},{"id":"x","prompt":"break"}]`, map[string]string{"FAKE_FAIL": "break"})
		r := s.ask("batch", "--resume", filepath.Join(s.home, "runs", s.dirs()[0]))
		eq(t, len(s.calls()), 3)
		match(t, s.calls()[2].s("stdin"), `break$`)
		eq(t, stringsField(objects(t, r.stdout), "answer"), []string{"fake: fine", "fake: break"})
	})
	t.Run("batch --resume of an unknown run is a usage error", func(t *testing.T) {
		s := fresh(t)
		r := s.ask("batch", "--resume", filepath.Join(s.tmp, "nope"))
		eq(t, r.code, 2)
		match(t, r.stderr, `no run at`)
	})
	t.Run("runs lists recorded runs, newest first, with their outcome and model", func(t *testing.T) {
		s := fresh(t)
		s.ask("-m", "fake:small", "first question")
		s.run([]string{"batch", "-m", "fake:small"}, `[{"prompt":"fine"},{"prompt":"break"}]`, map[string]string{"FAKE_FAIL": "break"})
		rows := strings.Split(strings.TrimSpace(s.ask("runs").stdout), "\n")
		eq(t, len(rows), 3)
		match(t, rows[0], `^RUN +STARTED +STATUS +MODEL +TIME +TASK$`)
		match(t, rows[1], `^\w{6} .* 1/2 ok +2 tasks +fine$`)
		match(t, rows[2], `^\w{6} .* ok +Fake 1\.0 +[\d.]+s +first question$`)
	})
	t.Run("runs shows an unfinished run whose ask is gone as stopped, with how to resume", func(t *testing.T) {
		s := fresh(t)
		dir := filepath.Join(s.home, "runs", "20260101T000000-zzzzzz")
		s.json(filepath.Join(dir, "tasks.json"), []object{{"id": "1", "model": "fake:small", "prompt": "p"}, {"id": "2", "model": "fake:small", "prompt": "p"}})
		s.json(filepath.Join(dir, "results.json"), []any{object{"id": "1", "ok": true}, nil})
		match(t, s.ask("runs").stdout, `(?m)^zzzzzz .* stopped .* resume: ask batch --resume zzzzzz$`)
	})
}

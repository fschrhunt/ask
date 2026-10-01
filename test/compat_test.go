package test

import (
	"path/filepath"
	"testing"
)

// TestCompat keeps released records readable, listed and continuable without rewriting fixtures.
func TestCompat(t *testing.T) {
	t.Run("a version 1 run record can be shown, listed and continued", func(t *testing.T) {
		s := fresh(t)
		dir := filepath.Join(s.home, "runs", "20261001T120000000-abc123")
		for _, name := range []string{"tasks.json", "results.json"} {
			s.write(filepath.Join(dir, name), s.read(filepath.Join(root, "test", "fixtures", "run-v1", name)))
		}
		shown := s.ask("show", "abc123")
		eq(t, shown.stdout, "The token expiry is compared in seconds against milliseconds.\n")
		match(t, shown.stderr, `(?m)^ask abc123 · Fake 1\.0 · ok · 41\.2s · 1 file changed · 52\.1k in · 2\.0k out · \$0\.3100$`)
		match(t, s.ask("runs").stdout, `(?m)^abc123 .* ok +Fake 1\.0 +41\.2s +Why does the login test fail\?$`)
		followup := s.ask("-c", "abc123", "Fix it.")
		eq(t, followup.code, 0)
		eq(t, s.calls()[0].s("session"), "s-fixture")
		eq(t, s.calls()[0].s("access"), "write")
	})
}

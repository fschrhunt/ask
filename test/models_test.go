package test

import (
	"path/filepath"
	"testing"
	"time"
)

// TestModelsOnOff pins models.json: models can be turned off and on, off ones are hidden and
// refused, a listed model back on leaves no entry, and the older list form still adds models.
func TestModelsOnOff(t *testing.T) {
	s := fresh(t)
	all := s.ask("models").stdout
	match(t, all, `(?m)^fake:small$`)
	eq(t, s.ask("models", "fake:small", "--disable").code, 0)
	noMatch(t, s.ask("models").stdout, `(?m)^fake:small$`)
	match(t, s.ask("models", "--all").stdout, `(?m)^fake:small\toff$`)
	r := s.ask("-m", "fake:small", "hi")
	eq(t, r.code, 2)
	match(t, r.stderr, `fake:small is off in models.json; turn it on with ask models fake:small --enable`)
	s.ask("models", "fake:small", "--enable")
	eq(t, s.ask("-m", "fake:small", "hi").code, 0)
	noMatch(t, s.read(filepath.Join(s.home, "models.json")), `small`)
	s.write(filepath.Join(s.home, "models.json"), `{"fake": ["extra"]}`)
	match(t, s.ask("models").stdout, `(?m)^fake:extra$`)
}

// TestCostLimit pins cost limits: the run's own, else its model's, else the setting; ask stops an
// agent whose reported cost passes it and says how to continue.
func TestCostLimit(t *testing.T) {
	s := fresh(t)
	s.write(filepath.Join(s.home, "settings.json"), `{"max_cost": 2}`)
	start := time.Now()
	r := s.run([]string{"-m", "fake:small", "spend it"}, "", map[string]string{"FAKE_SPEND": "spend"})
	eq(t, r.code, 1)
	match(t, r.stderr, `· failed · .* · stopped at the \$2\.00 cost limit; ask -c spend continues it`)
	if time.Since(start) > 10*time.Second {
		t.Fatal("the agent was not stopped at its limit")
	}
	s.ask("models", "fake:small", "--max-cost", "5")
	s.ask("-m", "fake:small", "a")
	s.ask("-m", "fake:small", "--max-cost", "1", "b")
	s.ask("-m", "fake:big", "c")
	calls := s.calls()
	eq(t, []string{calls[1].s("max_cost"), calls[2].s("max_cost"), calls[3].s("max_cost")}, []string{"5", "1", "2"})
}

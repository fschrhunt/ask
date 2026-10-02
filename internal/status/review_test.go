package status

import (
	"io"
	"os"
	"strings"
	"testing"
)

// TestLiveBatchFinishPrintsTotals prints the same batch summary on a terminal.
func TestLiveBatchFinishPrintsTotals(t *testing.T) {
	old := os.Stderr
	r, w, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	os.Stderr = w
	defer func() { os.Stderr = old }()
	l := &Live{state: LiveState{Batch: true}, stop: make(chan struct{})}
	l.Finish("ask run · 2/2 ok · 10 in · 5 out · $0.02", false)
	w.Close()
	b, _ := io.ReadAll(r)
	if !strings.Contains(string(b), "10 in · 5 out · $0.02") {
		t.Fatalf("missing totals: %q", b)
	}
}

// TestTerminalSizeUsesColumns uses a positive environment width when ioctl fails.
func TestTerminalSizeUsesColumns(t *testing.T) {
	t.Setenv("COLUMNS", "132")
	f, e := os.CreateTemp(t.TempDir(), "file")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	width, _, _ := terminalSize(f)
	if width != 132 {
		t.Fatalf("width %d", width)
	}
}

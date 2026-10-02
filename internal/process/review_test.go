package process

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestDetachedPipeHolderCannotHangRun bounds waiting for a detached output-pipe holder.
func TestDetachedPipeHolderCannotHangRun(t *testing.T) {
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "pid")
	begin := time.Now()
	r := Run("sh", []string{"-c", `setsid sh -c 'echo $$ > "$1"; exec sleep 30' sh "$1" & while [ ! -f "$1" ]; do sleep 0.01; done; echo done`, "sh", pidfile}, Options{Env: os.Environ(), Timeout: 2 * time.Second})
	if b, e := os.ReadFile(pidfile); e == nil {
		if pid, e := strconv.Atoi(strings.TrimSpace(string(b))); e == nil {
			defer syscall.Kill(pid, syscall.SIGKILL)
		}
	}
	if r.Code != 0 || strings.TrimSpace(r.Stdout) != "done" {
		t.Fatalf("result: %#v", r)
	}
	if time.Since(begin) > 3*time.Second {
		t.Fatal("waited too long for detached pipe")
	}
}

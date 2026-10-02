package runs

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// TestOwnerRequiresHeldLock treats a stale PID, even a reused live PID, as idle.
func TestOwnerRequiresHeldLock(t *testing.T) {
	r := &Run{Dir: t.TempDir()}
	if e := os.WriteFile(filepath.Join(r.Dir, "lock"), []byte(strconv.Itoa(os.Getpid())), 0600); e != nil {
		t.Fatal(e)
	}
	if got := Owner(r); got != 0 {
		t.Fatalf("stale lock owned by %d", got)
	}

}

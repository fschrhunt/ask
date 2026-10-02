package git

import (
	"os"
	"path/filepath"
	"testing"
)

// TestInspectionDisablesOptionalLocks passes git's read-only lock setting to inspection commands.
func TestInspectionDisablesOptionalLocks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "git")
	if e := os.WriteFile(path, []byte("#!/bin/sh\n[ \"$GIT_OPTIONAL_LOCKS\" = 0 ] || exit 3\necho clean\n"), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if _, ok := git(dir, []string{"status"}, ""); !ok {
		t.Fatal("inspection did not disable optional locks")
	}
}

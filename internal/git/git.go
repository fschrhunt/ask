// Package git compares working-file contents across write runs and manages isolated worktrees.
package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/fschrhunt/ask/internal/home"
)

// git returns output and success, silently handling repositories without a HEAD.
func git(dir string, args []string, input string) (string, bool) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Stdin = strings.NewReader(input)
	b, e := cmd.Output()
	return string(b), e == nil
}

// head resolves HEAD or returns an empty string for an unborn repository.
func head(root string) string {
	h, _ := git(root, []string{"rev-parse", "--verify", "-q", "HEAD"}, "")
	return home.Trim(h)
}

// dirtyPaths decodes porcelain paths, including both sides of renames and copies.
func dirtyPaths(root string) []string {
	out, _ := git(root, []string{"status", "--porcelain=v1", "-z", "--untracked-files=all"}, "")
	fields := strings.Split(out, "\x00")
	paths := []string{}
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		if f == "" {
			continue
		}
		if len(f) >= 3 {
			paths = append(paths, f[3:])
		}
		if f[0] == 'R' || f[0] == 'C' {
			i++
			if i < len(fields) {
				paths = append(paths, fields[i])
			}
		}
	}
	return paths
}

// hashFiles fingerprints present working files, retaining empty hashes for missing paths.
func hashFiles(root string, paths []string) map[string]string {
	present := []string{}
	hashes := []string{}
	for _, path := range paths {
		if _, e := os.Stat(filepath.Join(root, path)); e == nil {
			present = append(present, path)
		}
	}
	if len(present) > 0 {
		out, _ := git(root, []string{"hash-object", "--stdin-paths"}, strings.Join(present, "\n")+"\n")
		hashes = strings.Split(home.Trim(out), "\n")
	}
	m := map[string]string{}
	for _, path := range paths {
		m[path] = ""
	}
	for i, path := range present {
		if i < len(hashes) {
			m[path] = hashes[i]
		}
	}
	return m
}

// hashAt resolves starting commit blobs for files that were initially clean.
func hashAt(root, commit string, paths []string) map[string]string {
	m := map[string]string{}
	for _, path := range paths {
		m[path] = ""
	}
	if commit == "" || len(paths) == 0 {
		return m
	}
	out, _ := git(root, append([]string{"ls-tree", "-z", commit, "--"}, paths...), "")
	for _, entry := range strings.Split(out, "\x00") {
		meta, path, ok := strings.Cut(entry, "\t")
		bits := strings.Fields(meta)
		if ok && len(bits) == 3 && bits[1] == "blob" {
			m[path] = bits[2]
		}
	}
	return m
}

// Snapshot holds HEAD and the hashes of files already dirty before a task.
type Snapshot struct {
	Root, Head string
	Files      map[string]string
	Paths      []string
}

// Take returns nil outside a git repository; otherwise it captures the current working state.
func Take(dir string) *Snapshot {
	root, ok := git(dir, []string{"rev-parse", "--show-toplevel"}, "")
	root = home.Trim(root)
	if !ok || root == "" {
		return nil
	}
	paths := dirtyPaths(root)
	return &Snapshot{root, head(root), hashFiles(root, paths), paths}
}

// Changes returns content changes only, plus commits made on top of the starting HEAD.
func Changes(before *Snapshot) ([]any, int) {
	h := head(before.Root)
	paths := []string{}
	seen := map[string]bool{}
	add := func(path string) {
		if path != "" && !seen[path] {
			paths = append(paths, path)
			seen[path] = true
		}
	}
	for _, p := range before.Paths {
		add(p)
	}
	for _, p := range dirtyPaths(before.Root) {
		add(p)
	}
	commits := 0
	if h != "" && before.Head != "" && h != before.Head {
		out, _ := git(before.Root, []string{"diff", "--name-only", "-z", before.Head, h}, "")
		for _, p := range strings.Split(out, "\x00") {
			add(p)
		}
		count, _ := git(before.Root, []string{"rev-list", "--count", before.Head + ".." + h}, "")
		commits, _ = strconv.Atoi(home.Trim(count))
	}
	clean := []string{}
	for _, p := range paths {
		if _, ok := before.Files[p]; !ok {
			clean = append(clean, p)
		}
	}
	start := hashAt(before.Root, before.Head, clean)
	for p, h := range before.Files {
		start[p] = h
	}
	end := hashFiles(before.Root, paths)
	sort.Strings(paths)
	files := []any{}
	for _, path := range paths {
		if start[path] != end[path] {
			change := "modified"
			if start[path] == "" {
				change = "added"
			} else if end[path] == "" {
				change = "deleted"
			}
			files = append(files, home.O("path", path, "change", change))
		}
	}
	return files, commits
}

// Worktree describes a task's branch, location and omitted source changes.
type Worktree struct {
	Path, Branch, Dir string
	Dirty             bool
}

// Add creates a branch from HEAD or reuses a follow-up's worktree at the recorded name.
func Add(p home.Paths, dir, name string) (*Worktree, error) {
	root, ok := git(dir, []string{"rev-parse", "--show-toplevel"}, "")
	root = home.Trim(root)
	if !ok || root == "" {
		return nil, fmt.Errorf("--worktree needs a git repository; %s is not in one", dir)
	}
	path := filepath.Join(p.Worktrees, name)
	branch := "ask/" + name
	if _, e := os.Stat(path); e != nil {
		if e = os.MkdirAll(p.Worktrees, 0777); e != nil {
			return nil, e
		}
		_, exists := git(root, []string{"rev-parse", "--verify", "-q", "refs/heads/" + branch}, "")
		args := []string{"worktree", "add", path, branch}
		if !exists {
			args = []string{"worktree", "add", "-b", branch, path, "HEAD"}
		}
		if _, ok := git(root, args, ""); !ok {
			return nil, fmt.Errorf("could not create a worktree for %s; does it have a commit?", root)
		}
	}
	rel, _ := filepath.Rel(root, dir)
	return &Worktree{path, branch, filepath.Join(path, rel), len(dirtyPaths(root)) > 0}, nil
}

// Remove removes an unchanged worktree and attempts to delete its branch.
func Remove(w *Worktree) {
	root, _ := git(w.Path, []string{"rev-parse", "--git-common-dir"}, "")
	root = home.Trim(root)
	git(w.Path, []string{"worktree", "remove", "--force", w.Path}, "")
	if root != "" {
		git(filepath.Join(root, ".."), []string{"branch", "-D", w.Branch}, "")
	}
}

// Package home owns ask's local paths, contract environment and atomic records.
package home

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Paths holds the state directories and the resolved executable for this ask.
type Paths struct{ Home, Runs, Worktrees, Agents, Packages, Bin string }

// New resolves paths once, honoring ASK_HOME and the current user's HOME.
func New() Paths {
	h, _ := os.UserHomeDir()
	root := os.Getenv("ASK_HOME")
	if root == "" {
		root = filepath.Join(h, ".ask")
	}
	bin, _ := os.Executable()
	if real, err := filepath.EvalSymlinks(bin); err == nil {
		bin = real
	}
	return Paths{root, filepath.Join(root, "runs"), filepath.Join(root, "worktrees"), filepath.Join(root, "agents"), filepath.Join(root, "packages"), bin}
}

// Env clears inherited per-task variables and supplies contract version 1.
func (p Paths) Env(vars map[string]string) []string {
	drop := map[string]bool{}
	for _, k := range []string{"ASK_MODEL", "ASK_EFFORT", "ASK_ACCESS", "ASK_SCHEMA", "ASK_SESSION", "ASK_REPORT", "ASK_RUN", "ASK_EVENT", "ASK_CONTRACT", "ASK_BIN", "ASK_HOME"} {
		drop[k] = true
	}
	env := []string{}
	for _, e := range os.Environ() {
		k, _, _ := strings.Cut(e, "=")
		if !drop[k] {
			env = append(env, e)
		}
	}
	env = append(env, "ASK_CONTRACT=1", "ASK_BIN="+p.Bin, "ASK_HOME="+p.Home)
	for k, v := range vars {
		env = append(env, k+"="+v)
	}
	return env
}

// UsageError is a caller error, printed with a help pointer and exit status 2.
type UsageError struct{ Message string }

// Error returns the usage message without its CLI prefix or help pointer.
func (e *UsageError) Error() string { return e.Message }

// Usage constructs a formatted command-line error.
func Usage(format string, args ...any) error { return &UsageError{fmt.Sprintf(format, args...)} }

// WriteJSON atomically replaces a complete, pretty-printed JSON record.
func WriteJSON(path string, v any) error {
	if err := os.WriteFile(path+".tmp", []byte(JSON(v, true)+"\n"), 0666); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

// ReadInput reads a named file or stdin, turning I/O failures into usage errors.
func ReadInput(path, what string) (string, error) {
	var b []byte
	var err error
	if path == "-" {
		b, err = io.ReadAll(os.Stdin)
	} else {
		b, err = os.ReadFile(path)
	}
	if err != nil {
		why := FileError(err, "open", path)
		if os.IsNotExist(err) {
			why = "no such file"
		}
		return "", Usage("cannot read %s %s: %s", what, path, why)
	}
	return UTF8(b), nil
}

// FileError renders filesystem errors in the existing CLI's vocabulary.
func FileError(err error, op, path string) string {
	code, text := "", ""
	switch {
	case os.IsNotExist(err):
		code, text = "ENOENT", "no such file or directory"
	case os.IsPermission(err):
		code, text = "EACCES", "permission denied"
	}
	if info, e := os.Stat(path); e == nil && info.IsDir() {
		return "EISDIR: illegal operation on a directory, read"
	}
	if code != "" {
		return fmt.Sprintf("%s: %s, %s '%s'", code, text, op, path)
	}
	return err.Error()
}

// Tilde abbreviates a home prefix as the status-line contract does.
func Tilde(path string) string {
	h, _ := os.UserHomeDir()
	if h != "" && strings.HasPrefix(path, h) {
		return "~" + strings.TrimPrefix(path, h)
	}
	return path
}

// ReadModels reads optional agent-to-model lists, rejecting damaged or malformed files.
func (p Paths) ReadModels() (Object, error) {
	path := filepath.Join(p.Home, "models.json")
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Object{}, nil
	}
	if err != nil {
		return nil, Usage("cannot read %s: %s", path, FileError(err, "open", path))
	}
	v, err := ParseJSON(UTF8(b))
	if err != nil {
		return nil, Usage("cannot parse %s: %s", path, err)
	}
	o, ok := v.(Object)
	if ok {
		for _, val := range o {
			a, good := val.([]any)
			if !good {
				ok = false
				break
			}
			for _, id := range a {
				if _, good := id.(string); !good {
					ok = false
					break
				}
			}
		}
	}
	if !ok {
		return nil, Usage(`%s must map agent names to lists of model ids, like {"mycli": ["atlas-2.1"]}`, path)
	}
	return o, nil
}

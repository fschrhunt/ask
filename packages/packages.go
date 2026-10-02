// Package packages holds the official agent packages, built into ask: each folder here (claude,
// codex, opencode) is a package that ask install NAME writes to ~/.ask/packages/ask/packages/NAME.
// Their tests run with node --test inside each folder.
package packages

import "embed"

// FS holds what an installed package needs: its agents, the code they run and its README.
//
//go:embed */agents */lib */README.md
var FS embed.FS

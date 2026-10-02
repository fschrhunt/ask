// Package docs holds ask's documentation pages, built into ask so ask docs can show them offline,
// matching the ask that is installed.
package docs

import "embed"

// FS holds every page, by file name, like usage.md.
//
//go:embed *.md
var FS embed.FS

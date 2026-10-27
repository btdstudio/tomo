// Package tomo embeds the canonical ICM scaffold so the tomo binary is
// self-contained: no source checkout is required at runtime. `tomo init`
// copies this tree out; `tomo update` refreshes managed files against it.
//
// `all:` includes dot-files (.tomo/, .gitignore, .gitkeep), which a plain
// go:embed pattern would silently ignore.
package tomo

import "embed"

// ScaffoldFS is the embedded ICM scaffold tree.
//
//go:embed all:scaffold
var ScaffoldFS embed.FS

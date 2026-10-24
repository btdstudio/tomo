// Package tomo embeds the canonical content store so the tomo binary is
// self-contained: no source checkout is required at runtime. The store is the
// single source of truth — content/ (canonical command bodies + the shared
// approval gate) and src/ (command contracts, persona profiles, templates).
package tomo

import "embed"

// FS is the embedded content store. `go:embed` ignores dot-files, so the
// embedded tree is exactly the committed sources.
//
//go:embed content src
var FS embed.FS

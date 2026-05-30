// Package web embeds the built React single-page application so the Go binary
// can serve the UI without any external assets.
//
// The contents of dist/ are produced by building the Vite app in the repo-root
// frontend/ directory and copying its dist output here (see the Makefile). A
// minimal dist/ is committed so the Go module always builds; production builds
// overwrite it with the real bundle.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// FS returns the built SPA assets rooted at the dist directory.
func FS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}

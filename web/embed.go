// Package web carries the built Svelte bundle into the binary.
//
// //go:embed is what makes the whole distribution promise work: the binary *is* the web
// app, so there is nothing to deploy separately and a venue with no internet can still
// hand a phone the application (docs/tech-stack.md §1).
//
// dist/ is produced by `npm run build` in this directory. A committed .gitkeep keeps the
// directory present so this embed compiles on a checkout that has not built the frontend
// yet -- the server then reports that the web app is not built into the binary rather than
// failing mysteriously. `vite build` empties the directory, which would delete that
// placeholder, so the build script writes it back afterwards.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// The participant's offline signup app (issue #91). One self-contained file of plain
// HTML, embedded separately from the bundle because it is not part of the application:
// it is a thing the organizer downloads and emails to people, and it has to work from a
// downloads folder with no server anywhere.
//
//go:embed signup/signup.html
var signupApp embed.FS

// SignupApp is the participant app as it ships, before the event is written into it.
func SignupApp() ([]byte, error) {
	return signupApp.ReadFile("signup/signup.html")
}

// Assets is the bundle rooted at dist/.
func Assets() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}

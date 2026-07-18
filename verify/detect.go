package verify

import (
	"os"
	"path/filepath"
)

// DefaultValidators picks the checks a Gate runs against a staged,
// patched repository at root. A Go module gets its real build and
// tests; anything else falls back to a repository sanity check. PIC-1
// (docs/04-guides/M0-IMPLEMENTATION-BACKLOG.md) replaces this detection
// with pinned, project-specific commands once budgets and configuration
// exist.
//
// Extracted so every composition root — the one-shot `foundry do`
// command and the interactive session alike — shares one detection
// rule instead of each reimplementing it.
func DefaultValidators(root string) []*Validator {
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
		return []*Validator{
			// A bare "go build ./..." compiles any package main it finds
			// straight into a binary left sitting in the working tree
			// after every verification — real dogfooding surfaced this
			// as a leaked, untracked artifact on every single Act. Only
			// redirect to a throwaway -o directory when a main package
			// actually exists: "go build -o <dir> ./..." errors with "no
			// main packages to build" on a library-only module, which
			// would turn every such module's real, passing build into a
			// false verification failure.
			{Name: "go-build", Cmd: `if go list -f "{{.Name}}" ./... 2>/dev/null | grep -qx main; then go build -o "$(mktemp -d)/" ./...; else go build ./...; fi`},
			{Name: "go-test", Cmd: "go test ./..."},
		}
	}
	return []*Validator{{Name: "repo-sanity", Cmd: "git rev-parse HEAD"}}
}

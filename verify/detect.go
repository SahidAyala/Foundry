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
			{Name: "go-build", Cmd: "go build ./..."},
			{Name: "go-test", Cmd: "go test ./..."},
		}
	}
	return []*Validator{{Name: "repo-sanity", Cmd: "git rev-parse HEAD"}}
}

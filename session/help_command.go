package session

import (
	"context"
	"fmt"
)

// HelpCommand backs /help: it lists every slash command registered in
// Registry, by name and one-line description, in registration order.
// This is ADR-0009 Decision 6 — the startup banner has always told users
// to "type /help for a list of commands," but no such command existed
// until this one.
type HelpCommand struct {
	// Registry is the CommandRegistry HelpCommand lists — set by
	// DefaultCommandRegistry to the same registry HelpCommand is itself
	// registered into, so /help's own entry appears alongside every
	// other command's.
	Registry *CommandRegistry
}

var _ CommandHandler = HelpCommand{}

// Describe returns HelpCommand's own one-line /help description.
func (HelpCommand) Describe() string {
	return "List the slash commands this session understands."
}

// Run writes one line per registered command: its name and description.
func (h HelpCommand) Run(ctx context.Context, s *Session, args string) error {
	for _, info := range h.Registry.List() {
		fmt.Fprintf(s.Out, "/%s — %s\n", info.Name, info.Description)
	}
	return nil
}

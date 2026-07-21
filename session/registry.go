package session

import (
	"context"
	"fmt"
)

// CommandHandler is what a slash command does once dispatched: given the
// Session it is running against and the raw text typed after the
// command's name, it performs the command's action.
type CommandHandler interface {
	Run(ctx context.Context, s *Session, args string) error

	// Describe returns the command's one-line description, as listed by
	// /help (ADR-0009 Decision 6).
	Describe() string
}

// CommandInfo names one registered slash command and its one-line
// description, as returned by CommandRegistry.List for /help to render.
type CommandInfo struct {
	Name        string
	Description string
}

// CommandRegistry holds named CommandHandlers, mirroring — not
// importing — engine.PipelineRegistry's own shape: register-once,
// lookup-by-name, refuse duplicate registration. A session's set of
// slash commands is fixed for the registry's lifetime once built.
type CommandRegistry struct {
	handlers map[string]CommandHandler
	order    []string
}

// NewCommandRegistry returns an empty CommandRegistry.
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{handlers: make(map[string]CommandHandler)}
}

// Register adds h under name. It returns an error, leaving the registry
// unchanged, if a handler is already registered under that name.
func (r *CommandRegistry) Register(name string, h CommandHandler) error {
	if _, exists := r.handlers[name]; exists {
		return fmt.Errorf("session: command %q is already registered", name)
	}
	r.handlers[name] = h
	r.order = append(r.order, name)
	return nil
}

// List returns every registered command's name and description, in
// registration order — the vocabulary /help lists (ADR-0009 Decision 6).
func (r *CommandRegistry) List() []CommandInfo {
	infos := make([]CommandInfo, 0, len(r.order))
	for _, name := range r.order {
		infos = append(infos, CommandInfo{Name: name, Description: r.handlers[name].Describe()})
	}
	return infos
}

// Dispatch runs the handler registered under name with args against s,
// or returns an error naming the unknown command if none is registered.
func (r *CommandRegistry) Dispatch(ctx context.Context, s *Session, name string, args string) error {
	h, ok := r.handlers[name]
	if !ok {
		return fmt.Errorf("session: unknown command %q", name)
	}
	return h.Run(ctx, s, args)
}

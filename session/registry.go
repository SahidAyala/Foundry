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
}

// CommandRegistry holds named CommandHandlers, mirroring — not
// importing — engine.PipelineRegistry's own shape: register-once,
// lookup-by-name, refuse duplicate registration. A session's set of
// slash commands is fixed for the registry's lifetime once built.
type CommandRegistry struct {
	handlers map[string]CommandHandler
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
	return nil
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

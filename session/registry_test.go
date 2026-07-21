package session_test

import (
	"context"
	"testing"

	"foundry/session"
)

// recordingHandler is a fake CommandHandler for testing CommandRegistry
// dispatch without any real Pipeline execution.
type recordingHandler struct {
	calls []string
	err   error
}

func (h *recordingHandler) Run(ctx context.Context, s *session.Session, args string) error {
	h.calls = append(h.calls, args)
	return h.err
}

func (h *recordingHandler) Describe() string {
	return "a fake command for testing"
}

func TestCommandRegistry_RegisterThenDispatch(t *testing.T) {
	registry := session.NewCommandRegistry()
	handler := &recordingHandler{}

	if err := registry.Register("feature", handler); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := registry.Dispatch(context.Background(), nil, "feature", "add refresh tokens"); err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}
	if len(handler.calls) != 1 || handler.calls[0] != "add refresh tokens" {
		t.Errorf("calls = %v, want [\"add refresh tokens\"]", handler.calls)
	}
}

func TestCommandRegistry_DuplicateRegistrationFails(t *testing.T) {
	registry := session.NewCommandRegistry()
	if err := registry.Register("feature", &recordingHandler{}); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}

	err := registry.Register("feature", &recordingHandler{})
	if err == nil {
		t.Fatal("second Register under the same name returned nil error")
	}
}

func TestCommandRegistry_DispatchUnknownCommandFails(t *testing.T) {
	registry := session.NewCommandRegistry()

	err := registry.Dispatch(context.Background(), nil, "nonexistent", "")
	if err == nil {
		t.Fatal("Dispatch of an unregistered command returned nil error")
	}
}

func TestCommandRegistry_ListReturnsCommandsInRegistrationOrder(t *testing.T) {
	registry := session.NewCommandRegistry()
	if err := registry.Register("feature", &recordingHandler{}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := registry.Register("bug", &recordingHandler{}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got := registry.List()
	want := []session.CommandInfo{
		{Name: "feature", Description: "a fake command for testing"},
		{Name: "bug", Description: "a fake command for testing"},
	}
	if len(got) != len(want) {
		t.Fatalf("List() = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("List()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestCommandRegistry_HandlerErrorPropagates(t *testing.T) {
	registry := session.NewCommandRegistry()
	wantErr := context.DeadlineExceeded
	if err := registry.Register("feature", &recordingHandler{err: wantErr}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	err := registry.Dispatch(context.Background(), nil, "feature", "")
	if err != wantErr {
		t.Errorf("Dispatch error = %v, want %v", err, wantErr)
	}
}

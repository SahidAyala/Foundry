package engine

import (
	"context"
	"testing"

	"foundry/domain"
)

type fakeRegistryApplier struct{ name string }

func (fakeRegistryApplier) Apply(ctx context.Context, workspace string, act *domain.Act) error {
	return nil
}

func TestApplierRegistry_RegisterAndGet(t *testing.T) {
	r := NewApplierRegistry()
	a := fakeRegistryApplier{name: "a"}
	if err := r.Register("knowledge-note", a); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	got, err := r.Get("knowledge-note")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got != a {
		t.Errorf("Get returned %#v, want %#v", got, a)
	}
}

func TestApplierRegistry_DuplicateRegistrationFails(t *testing.T) {
	r := NewApplierRegistry()
	if err := r.Register("knowledge-note", fakeRegistryApplier{}); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}
	if err := r.Register("knowledge-note", fakeRegistryApplier{}); err == nil {
		t.Error("second Register succeeded, want an error for a duplicate target")
	}
}

func TestApplierRegistry_UnregisteredTargetFails(t *testing.T) {
	r := NewApplierRegistry()
	if _, err := r.Get("project-doc"); err == nil {
		t.Error("Get succeeded for an unregistered target, want an error")
	}
}

package engine_test

import (
	"testing"

	"foundry/engine"
)

func TestExecutorRegistry_RegisterAndGet(t *testing.T) {
	registry := engine.NewExecutorRegistry()
	exec := &captureExecutor{patches: []string{"diff"}}

	if err := registry.Register("primary", exec); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got, err := registry.Get("primary")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got != exec {
		t.Errorf("Get(%q) returned a different Executor than was registered", "primary")
	}
}

func TestExecutorRegistry_DuplicateRegistrationFails(t *testing.T) {
	registry := engine.NewExecutorRegistry()
	first := &captureExecutor{}
	second := &captureExecutor{}

	if err := registry.Register("primary", first); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}
	err := registry.Register("primary", second)
	if err == nil {
		t.Fatal("second Register under the same name returned nil error")
	}

	got, getErr := registry.Get("primary")
	if getErr != nil {
		t.Fatalf("Get failed: %v", getErr)
	}
	if got != first {
		t.Error("duplicate Register replaced the originally registered Executor")
	}
}

func TestExecutorRegistry_GetUnknownNameFails(t *testing.T) {
	registry := engine.NewExecutorRegistry()

	_, err := registry.Get("nonexistent")
	if err == nil {
		t.Fatal("Get with an unregistered name returned nil error")
	}
}

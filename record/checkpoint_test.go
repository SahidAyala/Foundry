package record

import (
	"context"
	"testing"
	"time"
)

func TestCheckpointStore_SaveLoadRoundTrip(t *testing.T) {
	store, err := NewCheckpointStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewCheckpointStore failed: %v", err)
	}
	act := newAct("act-1", "add a feature", time.Now())

	if err := store.Save(context.Background(), act); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	got, err := store.Load(context.Background(), "act-1")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if got.ID != act.ID || got.Intent != act.Intent {
		t.Errorf("Load = %+v, want ID/Intent matching %+v", got, act)
	}
}

func TestCheckpointStore_SaveOverwritesPreviousCheckpoint(t *testing.T) {
	store, err := NewCheckpointStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewCheckpointStore failed: %v", err)
	}
	act := newAct("act-1", "add a feature", time.Now())

	if err := store.Save(context.Background(), act); err != nil {
		t.Fatalf("first Save failed: %v", err)
	}
	act.Iterations = 2
	if err := store.Save(context.Background(), act); err != nil {
		t.Fatalf("second Save to the same act ID failed: %v (checkpoints must be overwritable, unlike FileStore.Write)", err)
	}

	got, err := store.Load(context.Background(), "act-1")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if got.Iterations != 2 {
		t.Errorf("Iterations = %d, want 2 (overwrite did not take)", got.Iterations)
	}
}

func TestCheckpointStore_LoadMissingCheckpointFails(t *testing.T) {
	store, err := NewCheckpointStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewCheckpointStore failed: %v", err)
	}
	if _, err := store.Load(context.Background(), "nonexistent"); err == nil {
		t.Fatal("Load of a missing checkpoint returned nil error")
	}
}

func TestCheckpointStore_DeleteThenLoadFails(t *testing.T) {
	store, err := NewCheckpointStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewCheckpointStore failed: %v", err)
	}
	act := newAct("act-1", "add a feature", time.Now())
	if err := store.Save(context.Background(), act); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if err := store.Delete(context.Background(), "act-1"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := store.Load(context.Background(), "act-1"); err == nil {
		t.Fatal("Load after Delete returned nil error")
	}
}

func TestCheckpointStore_DeleteMissingCheckpointIsNotAnError(t *testing.T) {
	store, err := NewCheckpointStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewCheckpointStore failed: %v", err)
	}
	if err := store.Delete(context.Background(), "nonexistent"); err != nil {
		t.Errorf("Delete of a missing checkpoint returned an error: %v", err)
	}
}

func TestCheckpointStore_ListReturnsOnlyInterruptedActs(t *testing.T) {
	root := t.TempDir()
	checkpoints, err := NewCheckpointStore(root)
	if err != nil {
		t.Fatalf("NewCheckpointStore failed: %v", err)
	}
	store, err := NewFileStore(root)
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}

	interrupted := newAct("act-interrupted", "add a feature", time.Now())
	if err := checkpoints.Save(context.Background(), interrupted); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	completed := newAct("act-completed", "fix a bug", time.Now())
	if err := store.Write(context.Background(), completed); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	acts, err := checkpoints.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(acts) != 1 || acts[0].ID != "act-interrupted" {
		t.Errorf("List = %+v, want only act-interrupted (act-completed has no checkpoint)", acts)
	}
}

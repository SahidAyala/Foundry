package record

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

// TestCheckpointStore_Save_RetriesCleanlyAfterSimulatedCrash covers the
// concrete failure a non-atomic Save produced: a crash between opening
// and writing checkpoint.json (simulated here by leaving a stray temp
// file, exactly what Save's own temp-file step can leave behind) must
// not corrupt the checkpoint a later Save/Load depends on.
func TestCheckpointStore_Save_RetriesCleanlyAfterSimulatedCrash(t *testing.T) {
	root := t.TempDir()
	store, err := NewCheckpointStore(root)
	if err != nil {
		t.Fatalf("NewCheckpointStore failed: %v", err)
	}
	ctx := context.Background()

	act := newAct("act-1", "will retry after a crash", time.Now())
	dir := filepath.Join(root, act.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("simulate crash: create act directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "checkpoint.json.tmp-simulated-crash"), []byte("{incomplete"), 0o644); err != nil {
		t.Fatalf("simulate crash: leave stray temp file: %v", err)
	}

	if err := store.Save(ctx, act); err != nil {
		t.Fatalf("Save after a simulated crash failed: %v", err)
	}
	got, err := store.Load(ctx, act.ID)
	if err != nil {
		t.Fatalf("Load after retry failed: %v", err)
	}
	if got.Intent != act.Intent {
		t.Errorf("Load after retry = %q, want %q", got.Intent, act.Intent)
	}
}

// TestCheckpointStore_Save_LeavesNoStrayTempFile confirms the temp file
// Save creates for the atomic publish is cleaned up on the success path.
func TestCheckpointStore_Save_LeavesNoStrayTempFile(t *testing.T) {
	root := t.TempDir()
	store, err := NewCheckpointStore(root)
	if err != nil {
		t.Fatalf("NewCheckpointStore failed: %v", err)
	}
	act := newAct("act-1", "no stray temp file", time.Now())
	if err := store.Save(context.Background(), act); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(root, act.ID))
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Errorf("stray temp file left behind: %s", entry.Name())
		}
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

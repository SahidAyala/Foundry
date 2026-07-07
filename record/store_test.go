package record

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"foundry/domain"
)

func newAct(id, intent string, createdAt time.Time) *domain.Act {
	return &domain.Act{
		ID:        id,
		Intent:    intent,
		CreatedAt: createdAt,
	}
}

func TestNewFileStore_CreatesRootDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "nested", "acts")

	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("precondition failed: %s already exists", root)
	}

	if _, err := NewFileStore(root); err != nil {
		t.Fatalf("NewFileStore returned error: %v", err)
	}

	info, err := os.Stat(root)
	if err != nil {
		t.Fatalf("root directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("root %s exists but is not a directory", root)
	}
}

func TestFileStore_WriteReadRoundTrip(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}

	ctx := context.Background()
	original := newAct("a1b2c3d4e5f6a7b8", "add logging to main.go", time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	original.ConsideredFiles = []string{"main.go"}
	original.Patch = "diff --git a/main.go b/main.go"
	original.CheckedFindings = []string{"go-build: pass", "go-test: pass"}

	if err := store.Write(ctx, original); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	got, err := store.Read(ctx, original.ID)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got.ID != original.ID {
		t.Errorf("ID = %q, want %q", got.ID, original.ID)
	}
	if got.Intent != original.Intent {
		t.Errorf("Intent = %q, want %q", got.Intent, original.Intent)
	}
	if !got.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, original.CreatedAt)
	}
	if got.Patch != original.Patch {
		t.Errorf("Patch = %q, want %q", got.Patch, original.Patch)
	}
	if len(got.CheckedFindings) != len(original.CheckedFindings) || got.CheckedFindings[0] != original.CheckedFindings[0] {
		t.Errorf("CheckedFindings = %v, want %v", got.CheckedFindings, original.CheckedFindings)
	}
	if len(got.ConsideredFiles) != len(original.ConsideredFiles) || got.ConsideredFiles[0] != original.ConsideredFiles[0] {
		t.Errorf("ConsideredFiles = %v, want %v", got.ConsideredFiles, original.ConsideredFiles)
	}
}

// TestFileStore_ReadWriteProperty checks Read(Write(act)) == act across
// several distinct Acts, including zero-value optional fields.
func TestFileStore_ReadWriteProperty(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}
	ctx := context.Background()

	approvedAt := time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)
	acts := []*domain.Act{
		newAct("0000000000000001", "first intent", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
		{
			ID:              "0000000000000002",
			Intent:          "second intent",
			CreatedAt:       time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
			ConsideredFiles: []string{"a.go", "b.go"},
			CheckedFindings: []string{"go-build: fail\nboom"},
			Patch:           "diff",
			JudgmentVerdict: "fail",
			ApprovedBy:      "bob",
			ApprovedAt:      &approvedAt,
		},
	}

	for _, act := range acts {
		if err := store.Write(ctx, act); err != nil {
			t.Fatalf("Write(%s) failed: %v", act.ID, err)
		}

		got, err := store.Read(ctx, act.ID)
		if err != nil {
			t.Fatalf("Read(%s) failed: %v", act.ID, err)
		}

		if got.ID != act.ID || got.Intent != act.Intent || !got.CreatedAt.Equal(act.CreatedAt) {
			t.Errorf("Read(%s) = %+v, want %+v", act.ID, got, act)
		}
		if (got.ApprovedAt == nil) != (act.ApprovedAt == nil) {
			t.Errorf("Read(%s).ApprovedAt presence mismatch: got %v, want %v", act.ID, got.ApprovedAt, act.ApprovedAt)
		}
		if got.ApprovedAt != nil && act.ApprovedAt != nil && !got.ApprovedAt.Equal(*act.ApprovedAt) {
			t.Errorf("Read(%s).ApprovedAt = %v, want %v", act.ID, *got.ApprovedAt, *act.ApprovedAt)
		}
	}
}

func TestFileStore_WriteTwice_Fails(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}
	ctx := context.Background()
	act := newAct("dead00000000beef", "first version", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	if err := store.Write(ctx, act); err != nil {
		t.Fatalf("first Write failed: %v", err)
	}

	second := newAct("dead00000000beef", "second version", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	err = store.Write(ctx, second)
	if err == nil {
		t.Fatal("second Write to the same ID succeeded, want error")
	}
	if !errors.Is(err, ErrAlreadyExists) {
		t.Errorf("second Write error = %v, want errors.Is(err, ErrAlreadyExists)", err)
	}

	got, readErr := store.Read(ctx, act.ID)
	if readErr != nil {
		t.Fatalf("Read after failed overwrite: %v", readErr)
	}
	if got.Intent != "first version" {
		t.Errorf("Intent after failed overwrite = %q, want %q (immutability violated)", got.Intent, "first version")
	}
}

func TestFileStore_Read_NotFound(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}

	if _, err := store.Read(context.Background(), "missing"); err == nil {
		t.Fatal("Read of missing act returned nil error")
	}
}

func TestFileStore_List_CreationOrder(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}
	ctx := context.Background()

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	third := newAct("cccccccccccccccc", "third", base.Add(2*time.Hour))
	first := newAct("aaaaaaaaaaaaaaaa", "first", base)
	second := newAct("bbbbbbbbbbbbbbbb", "second", base.Add(1*time.Hour))

	// Write out of creation order to prove List sorts by CreatedAt, not
	// filesystem enumeration order.
	for _, act := range []*domain.Act{third, first, second} {
		if err := store.Write(ctx, act); err != nil {
			t.Fatalf("Write(%s) failed: %v", act.ID, err)
		}
	}

	acts, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(acts) != 3 {
		t.Fatalf("List returned %d acts, want 3", len(acts))
	}

	wantOrder := []string{first.ID, second.ID, third.ID}
	for i, want := range wantOrder {
		if acts[i].ID != want {
			t.Errorf("acts[%d].ID = %q, want %q", i, acts[i].ID, want)
		}
	}
}

func TestFileStore_List_Empty(t *testing.T) {
	store, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}

	acts, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(acts) != 0 {
		t.Errorf("List on empty store returned %d acts, want 0", len(acts))
	}
}

func TestEncode_GoldenShape(t *testing.T) {
	act := &domain.Act{
		ID:              "a1b2c3d4e5f6a7b8",
		Intent:          "add logging to main.go",
		CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		ConsideredFiles: []string{"main.go"},
		CheckedFindings: []string{"go-build: pass", "go-test: pass"},
		Patch:           "diff --git a/main.go b/main.go",
		JudgmentVerdict: "pass",
		ApprovedBy:      "",
		ApprovedAt:      nil,
		Iterations:      1,
		CostEstimateUSD: 0.50,
	}

	data, err := encode(act)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	want := `{
  "id": "a1b2c3d4e5f6a7b8",
  "intent": "add logging to main.go",
  "created_at": "2026-01-01T00:00:00Z",
  "considered_files": [
    "main.go"
  ],
  "checked_findings": [
    "go-build: pass",
    "go-test: pass"
  ],
  "patch": "diff --git a/main.go b/main.go",
  "judgment_verdict": "pass",
  "approved_by": "",
  "approved_at": null,
  "iterations": 1,
  "cost_estimate_usd": 0.5
}`

	if string(data) != want {
		t.Errorf("encode golden mismatch:\ngot:\n%s\nwant:\n%s", data, want)
	}
}

func TestDecode_RejectsInvalidJSON(t *testing.T) {
	if _, err := decode([]byte("not json")); err == nil {
		t.Fatal("decode of invalid JSON returned nil error")
	}
}

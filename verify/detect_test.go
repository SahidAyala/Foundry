package verify_test

import (
	"os"
	"path/filepath"
	"testing"

	"foundry/verify"
)

func TestDefaultValidators_GoModuleGetsBuildAndTest(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n"), 0o644); err != nil {
		t.Fatalf("write go.mod failed: %v", err)
	}

	got := verify.DefaultValidators(dir)
	if len(got) != 2 || got[0].Name != "go-build" || got[1].Name != "go-test" {
		t.Errorf("DefaultValidators(go module) = %+v, want [go-build, go-test]", got)
	}
}

func TestDefaultValidators_NonGoFallsBackToRepoSanity(t *testing.T) {
	got := verify.DefaultValidators(t.TempDir())
	if len(got) != 1 || got[0].Name != "repo-sanity" {
		t.Errorf("DefaultValidators(non-Go) = %+v, want [repo-sanity]", got)
	}
}

package engine_test

import (
	"context"
	"errors"
	"testing"

	"foundry/domain"
	"foundry/engine"
	"foundry/executor"
)

// fakeCheckpointSaver is an in-memory engine.CheckpointSaver: it captures
// every checkpoint an interrupted attempt left behind, so a test can assert
// what survived (and what was deleted) without touching the filesystem.
type fakeCheckpointSaver struct {
	saved   map[string]*domain.Act
	deleted map[string]bool
}

func newFakeCheckpointSaver() *fakeCheckpointSaver {
	return &fakeCheckpointSaver{saved: map[string]*domain.Act{}, deleted: map[string]bool{}}
}

func (f *fakeCheckpointSaver) Save(ctx context.Context, act *domain.Act) error {
	cp := *act
	cp.Steps = append([]domain.StepRecord(nil), act.Steps...)
	f.saved[act.ID] = &cp
	return nil
}

func (f *fakeCheckpointSaver) Delete(ctx context.Context, actID string) error {
	delete(f.saved, actID)
	f.deleted[actID] = true
	return nil
}

var _ engine.CheckpointSaver = (*fakeCheckpointSaver)(nil)

// TestEngine_InterruptedVerifyLeavesCheckpoint_ResumeCompletes proves the
// actual mechanism resume depends on: a Verifier failing with a real Go
// error (not a "fail" Judgment) mid-Pipeline leaves a checkpoint with
// exactly the Steps that completed before the failure, and Resume — given
// that checkpoint and a working Verifier — continues from there rather
// than re-running the generate Step or starting a new Act.
func TestEngine_InterruptedVerifyLeavesCheckpoint_ResumeCompletes(t *testing.T) {
	checkpoints := newFakeCheckpointSaver()
	gatherer := &fakeGatherer{files: []string{"main.go"}}
	brokenVerifier := &fakeVerifier{err: errors.New("verify boom")}
	exec := executor.NewScriptedExecutor(scriptedPatch)

	eng := engine.NewEngine(gatherer, exec, brokenVerifier, "", engine.DefaultPipeline())
	eng.SetCheckpointSaver(checkpoints)

	if _, err := eng.Run(context.Background(), &domain.Intent{Text: "add a feature"}); err == nil {
		t.Fatal("Run with a failing Verifier returned nil error")
	}

	if len(checkpoints.saved) != 1 {
		t.Fatalf("checkpoints.saved = %+v, want exactly 1 interrupted Act", checkpoints.saved)
	}
	var checkpointed *domain.Act
	for _, act := range checkpoints.saved {
		checkpointed = act
	}
	if len(checkpointed.Steps) != 1 || checkpointed.Steps[0].Kind != domain.StepKindGenerate {
		t.Fatalf("checkpointed Steps = %+v, want exactly one generate Step", checkpointed.Steps)
	}

	workingVerifier := &fakeVerifier{verdict: "pass"}
	resumeEng := engine.NewEngine(gatherer, exec, workingVerifier, "", engine.DefaultPipeline())
	resumeEng.SetCheckpointSaver(checkpoints)

	resumed, err := resumeEng.Resume(context.Background(), checkpointed)
	if err != nil {
		t.Fatalf("Resume failed: %v", err)
	}
	if resumed.JudgmentVerdict != "pass" {
		t.Errorf("JudgmentVerdict = %q, want %q", resumed.JudgmentVerdict, "pass")
	}
	wantKinds := []string{domain.StepKindGenerate, domain.StepKindVerify}
	if len(resumed.Steps) != len(wantKinds) {
		t.Fatalf("Steps = %+v, want %d entries (no re-run of the completed generate Step)", resumed.Steps, len(wantKinds))
	}
	for i, want := range wantKinds {
		if resumed.Steps[i].Kind != want {
			t.Errorf("Steps[%d].Kind = %q, want %q", i, resumed.Steps[i].Kind, want)
		}
	}
	if !checkpoints.deleted[resumed.ID] {
		t.Error("checkpoint was not deleted after Resume reached a terminal Judgment")
	}
}

// TestEngine_Resume_NothingToResumeIsAClearError verifies Resume refuses an
// Act whose recorded Steps already cover its Pipeline's full declared
// sequence, instead of silently doing nothing or re-running a Step.
func TestEngine_Resume_NothingToResumeIsAClearError(t *testing.T) {
	gatherer := &fakeGatherer{files: []string{"main.go"}}
	verifier := &fakeVerifier{verdict: "pass"}
	exec := executor.NewScriptedExecutor(scriptedPatch)
	eng := engine.NewEngine(gatherer, exec, verifier, "", engine.DefaultPipeline())
	eng.SetCheckpointSaver(newFakeCheckpointSaver())

	complete := &domain.Act{
		ID:     "already-done",
		Intent: "add a feature",
		Steps: []domain.StepRecord{
			{StepID: "1", Kind: domain.StepKindGenerate, Produced: []string{scriptedPatch}},
			{StepID: "2", Kind: domain.StepKindVerify, JudgmentVerdict: "pass"},
		},
	}

	if _, err := eng.Resume(context.Background(), complete); !errors.Is(err, engine.ErrCannotResume) {
		t.Fatalf("error = %v, want ErrCannotResume", err)
	}
}

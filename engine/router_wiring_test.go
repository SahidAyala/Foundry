package engine_test

import (
	"context"
	"reflect"
	"testing"

	"foundry/domain"
	"foundry/engine"
)

// orderExecutor records its own name to a shared log each time it is
// called, so a test can assert both which Executor ran and in what order.
type orderExecutor struct {
	name  string
	log   *[]string
	patch string
}

func (o *orderExecutor) Execute(ctx context.Context, intent *domain.Intent, considered []string) (*domain.Outcome, error) {
	*o.log = append(*o.log, o.name)
	return &domain.Outcome{Patch: o.patch}, nil
}

// TestEngine_TwoGenerateStepsPinnedToDifferentExecutors proves the Router,
// once wired into runSteps, actually resolves each Generate Step's declared
// executor pin independently — not just that Resolve returns the right
// value in isolation (router_test.go), but that runSteps calls through it.
func TestEngine_TwoGenerateStepsPinnedToDifferentExecutors(t *testing.T) {
	var log []string
	execA := &orderExecutor{name: "exec-a", log: &log, patch: "patch-a"}
	execB := &orderExecutor{name: "exec-b", log: &log, patch: "patch-b"}
	def := &orderExecutor{name: "default", log: &log, patch: "patch-default"}

	registry := engine.NewExecutorRegistry()
	if err := registry.Register("exec-a", execA); err != nil {
		t.Fatalf("Register(exec-a) failed: %v", err)
	}
	if err := registry.Register("exec-b", execB); err != nil {
		t.Fatalf("Register(exec-b) failed: %v", err)
	}

	pipeline := engine.Pipeline{
		Name: "multi-executor",
		Steps: []engine.Step{
			{ID: "gen-a", Kind: domain.StepKindGenerate, Executor: "exec-a"},
			{ID: "gen-b", Kind: domain.StepKindGenerate, Executor: "exec-b"},
			{ID: "verify", Kind: domain.StepKindVerify},
		},
	}

	gatherer := &fakeGatherer{files: []string{"main.go"}}
	verifier := &fakeVerifier{verdict: "pass"}
	eng := engine.NewEngine(gatherer, def, verifier, "", pipeline)
	eng.SetRouter(engine.NewRouter(registry, def))

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "do work"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	wantLog := []string{"exec-a", "exec-b"}
	if !reflect.DeepEqual(log, wantLog) {
		t.Errorf("execution order = %v, want %v (never the default Executor)", log, wantLog)
	}
	if act.Patch != "patch-b" {
		t.Errorf("Patch = %q, want %q (the last Generate Step's Outcome)", act.Patch, "patch-b")
	}
}

// TestEngine_UnpinnedPipelineRoutesToDefaultExecutor pins the
// backward-compatibility guarantee the whole Router migration rests on: a
// Pipeline whose Steps declare no executor pin — every Pipeline shipped
// before Router existed — behaves exactly as if Router did not exist,
// whether or not SetRouter is ever called.
func TestEngine_UnpinnedPipelineRoutesToDefaultExecutor(t *testing.T) {
	var log []string
	def := &orderExecutor{name: "default", log: &log, patch: "patch-default"}

	pipeline := engine.Pipeline{
		Name: "unpinned",
		Steps: []engine.Step{
			{ID: "generate", Kind: domain.StepKindGenerate},
			{ID: "verify", Kind: domain.StepKindVerify},
		},
	}

	gatherer := &fakeGatherer{files: []string{"main.go"}}
	verifier := &fakeVerifier{verdict: "pass"}
	eng := engine.NewEngine(gatherer, def, verifier, "", pipeline)

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "do work"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if !reflect.DeepEqual(log, []string{"default"}) {
		t.Errorf("execution log = %v, want [\"default\"]", log)
	}
	if act.Patch != "patch-default" {
		t.Errorf("Patch = %q, want %q", act.Patch, "patch-default")
	}
}

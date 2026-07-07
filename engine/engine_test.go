package engine_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"foundry/domain"
	"foundry/engine"
	"foundry/executor"
)

type fakeGatherer struct {
	files []string
	err   error
}

func (g *fakeGatherer) Gather(ctx context.Context, intent *domain.Intent) ([]string, error) {
	return g.files, g.err
}

type fakeVerifier struct {
	verdict string
	err     error
}

func (v *fakeVerifier) Verify(ctx context.Context, outcome *domain.Outcome, workspace string) (*domain.Judgment, error) {
	if v.err != nil {
		return nil, v.err
	}
	return &domain.Judgment{Verdict: v.verdict}, nil
}

const scriptedPatch = "diff --git a/main.go b/main.go\n+// scripted\n"

func newEngine(verdict string) *engine.Engine {
	gatherer := &fakeGatherer{files: []string{"main.go"}}
	verifier := &fakeVerifier{verdict: verdict}
	exec := executor.NewScriptedExecutor(scriptedPatch)
	return engine.NewEngine(gatherer, exec, verifier, "")
}

func TestEngine_Run_ProducesMachineJudgedAct(t *testing.T) {
	eng := newEngine("pass")

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "add a comment to main.go"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if act.ID == "" {
		t.Error("Act.ID is empty")
	}
	if act.Intent != "add a comment to main.go" {
		t.Errorf("Intent = %q, want %q", act.Intent, "add a comment to main.go")
	}
	if len(act.ConsideredFiles) != 1 || act.ConsideredFiles[0] != "main.go" {
		t.Errorf("ConsideredFiles = %v, want [main.go]", act.ConsideredFiles)
	}
	if act.Patch != scriptedPatch {
		t.Errorf("Patch = %q, want %q", act.Patch, scriptedPatch)
	}
	if act.JudgmentVerdict != "pass" {
		t.Errorf("JudgmentVerdict = %q, want %q", act.JudgmentVerdict, "pass")
	}

	// Run does not capture an Authority; approval happens above the Engine.
	if act.ApprovedBy != "" || act.ApprovedAt != nil {
		t.Errorf("Run captured approval (%q, %v); it must not", act.ApprovedBy, act.ApprovedAt)
	}
}

func TestEngine_Run_PassingValidator(t *testing.T) {
	act, err := newEngine("pass").Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if act.JudgmentVerdict != "pass" {
		t.Errorf("JudgmentVerdict = %q, want %q", act.JudgmentVerdict, "pass")
	}
}

func TestEngine_Run_FailingValidator(t *testing.T) {
	act, err := newEngine("fail").Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if act.JudgmentVerdict != "fail" {
		t.Errorf("JudgmentVerdict = %q, want %q", act.JudgmentVerdict, "fail")
	}
}

func TestEngine_Run_GatherErrorStopsLifecycle(t *testing.T) {
	gatherer := &fakeGatherer{err: errors.New("gather boom")}
	verifier := &fakeVerifier{verdict: "pass"}
	exec := executor.NewScriptedExecutor(scriptedPatch)
	eng := engine.NewEngine(gatherer, exec, verifier, "")

	if _, err := eng.Run(context.Background(), &domain.Intent{Text: "test"}); err == nil {
		t.Fatal("Run with failing Gatherer returned nil error")
	}
}

func TestEngine_Run_VerifyErrorStopsLifecycle(t *testing.T) {
	gatherer := &fakeGatherer{files: []string{"main.go"}}
	verifier := &fakeVerifier{err: errors.New("verify boom")}
	exec := executor.NewScriptedExecutor(scriptedPatch)
	eng := engine.NewEngine(gatherer, exec, verifier, "")

	if _, err := eng.Run(context.Background(), &domain.Intent{Text: "test"}); err == nil {
		t.Fatal("Run with failing Verifier returned nil error")
	}
}

func TestEngine_Run_RecordsBudgetUsage(t *testing.T) {
	act, err := newEngine("pass").Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if act.Iterations != 1 {
		t.Errorf("Iterations = %d, want 1", act.Iterations)
	}
	if act.CostEstimateUSD <= 0 || act.CostEstimateUSD > engine.DefaultBudget().MaxCostUSD {
		t.Errorf("CostEstimateUSD = %v, want in (0, %v]", act.CostEstimateUSD, engine.DefaultBudget().MaxCostUSD)
	}
}

func TestEngine_RunBudgeted_ExhaustedIterationsHaltsBeforeExecute(t *testing.T) {
	eng := newEngine("pass")

	act, err := eng.RunBudgeted(context.Background(), &domain.Intent{Text: "test"},
		&domain.Budget{MaxIterations: 0, MaxCostUSD: 1.00})

	if !errors.Is(err, engine.ErrBudgetExceeded) {
		t.Fatalf("error = %v, want ErrBudgetExceeded", err)
	}
	if act == nil {
		t.Fatal("RunBudgeted returned nil Act on budget exhaustion")
	}
	if act.JudgmentVerdict != engine.VerdictBudgetExceeded {
		t.Errorf("JudgmentVerdict = %q, want %q", act.JudgmentVerdict, engine.VerdictBudgetExceeded)
	}
	if act.Patch != "" {
		t.Errorf("halted Act carries a patch %q; Execute must not have run", act.Patch)
	}
	if act.Iterations != 0 {
		t.Errorf("Iterations = %d, want 0", act.Iterations)
	}
}

func TestEngine_RunBudgeted_CostCapHalts(t *testing.T) {
	eng := newEngine("pass")

	act, err := eng.RunBudgeted(context.Background(), &domain.Intent{Text: "test"},
		&domain.Budget{MaxIterations: 2, MaxCostUSD: 0.01})

	if !errors.Is(err, engine.ErrBudgetExceeded) {
		t.Fatalf("error = %v, want ErrBudgetExceeded", err)
	}
	if act.JudgmentVerdict != engine.VerdictBudgetExceeded {
		t.Errorf("JudgmentVerdict = %q, want %q", act.JudgmentVerdict, engine.VerdictBudgetExceeded)
	}
}

func TestEngine_RunBudgeted_DefaultBudgetPasses(t *testing.T) {
	act, err := newEngine("pass").RunBudgeted(context.Background(), &domain.Intent{Text: "test"},
		engine.DefaultBudget())
	if err != nil {
		t.Fatalf("RunBudgeted failed: %v", err)
	}
	if act.JudgmentVerdict != "pass" {
		t.Errorf("JudgmentVerdict = %q, want %q", act.JudgmentVerdict, "pass")
	}
}

// seqVerifier returns one canned Judgment per Verify call, in order.
type seqVerifier struct {
	judgments []*domain.Judgment
	calls     int
}

func (v *seqVerifier) Verify(ctx context.Context, outcome *domain.Outcome, workspace string) (*domain.Judgment, error) {
	j := v.judgments[v.calls]
	v.calls++
	return j, nil
}

// captureExecutor returns one canned patch (or error) per Execute call and
// records the considered context each call received.
type captureExecutor struct {
	patches []string
	errs    []error
	calls   [][]string
}

func (x *captureExecutor) Execute(ctx context.Context, intent *domain.Intent, considered []string) (*domain.Outcome, error) {
	i := len(x.calls)
	x.calls = append(x.calls, considered)
	if i < len(x.errs) && x.errs[i] != nil {
		return nil, x.errs[i]
	}
	return &domain.Outcome{Patch: x.patches[i]}, nil
}

func TestEngine_Run_RepairAfterFailPasses(t *testing.T) {
	exec := &captureExecutor{patches: []string{"first-patch", "repaired-patch"}}
	verifier := &seqVerifier{judgments: []*domain.Judgment{
		{Verdict: "fail", Checked: []string{"tests: fail\n1 test failed"}},
		{Verdict: "pass", Checked: []string{"tests: pass"}},
	}}
	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, exec, verifier, "")

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if act.JudgmentVerdict != "pass" {
		t.Errorf("JudgmentVerdict = %q, want %q", act.JudgmentVerdict, "pass")
	}
	if act.Patch != "repaired-patch" {
		t.Errorf("Patch = %q, want the repaired patch", act.Patch)
	}
	if act.Iterations != 2 {
		t.Errorf("Iterations = %d, want 2", act.Iterations)
	}
	if act.CostEstimateUSD != 1.00 {
		t.Errorf("CostEstimateUSD = %v, want 1.00", act.CostEstimateUSD)
	}
	if len(exec.calls) != 2 {
		t.Fatalf("Executor called %d times, want 2", len(exec.calls))
	}
	repairCtx := exec.calls[1][len(exec.calls[1])-1]
	if !strings.Contains(repairCtx, "1 test failed") {
		t.Errorf("repair Execute did not receive the findings, got context %q", repairCtx)
	}
	// Evidence shows both iterations: the recorded considered context
	// includes the findings entry the repair worked from.
	last := act.ConsideredFiles[len(act.ConsideredFiles)-1]
	if !strings.Contains(last, "1 test failed") {
		t.Errorf("Act Evidence missing repair findings, got %q", last)
	}
	// The recorded checked Evidence reflects the final (repaired) round's
	// findings, not the failed first attempt's.
	if len(act.CheckedFindings) != 1 || act.CheckedFindings[0] != "tests: pass" {
		t.Errorf("CheckedFindings = %v, want the repaired round's findings [\"tests: pass\"]", act.CheckedFindings)
	}
}

func TestEngine_Run_RecordsCheckedFindingsOnFirstPass(t *testing.T) {
	verifier := &seqVerifier{judgments: []*domain.Judgment{
		{Verdict: "pass", Checked: []string{"go-build: pass", "go-test: pass"}},
	}}
	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, executor.NewScriptedExecutor(scriptedPatch), verifier, "")

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	want := []string{"go-build: pass", "go-test: pass"}
	if !reflect.DeepEqual(act.CheckedFindings, want) {
		t.Errorf("CheckedFindings = %v, want %v", act.CheckedFindings, want)
	}
}

func TestEngine_Run_NoRepairWhenFirstAttemptPasses(t *testing.T) {
	exec := &captureExecutor{patches: []string{"first-patch"}}
	verifier := &seqVerifier{judgments: []*domain.Judgment{{Verdict: "pass"}}}
	eng := engine.NewEngine(&fakeGatherer{}, exec, verifier, "")

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if len(exec.calls) != 1 {
		t.Errorf("Executor called %d times, want 1", len(exec.calls))
	}
	if act.Iterations != 1 {
		t.Errorf("Iterations = %d, want 1", act.Iterations)
	}
}

func TestEngine_Run_RepairStillFailingIsFinal(t *testing.T) {
	exec := &captureExecutor{patches: []string{"first-patch", "second-patch"}}
	verifier := &seqVerifier{judgments: []*domain.Judgment{
		{Verdict: "fail", Checked: []string{"build: fail"}},
		{Verdict: "fail", Checked: []string{"build: fail"}},
	}}
	eng := engine.NewEngine(&fakeGatherer{}, exec, verifier, "")

	act, err := eng.Run(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if act.JudgmentVerdict != "fail" {
		t.Errorf("JudgmentVerdict = %q, want %q", act.JudgmentVerdict, "fail")
	}
	if act.Iterations != 2 {
		t.Errorf("Iterations = %d, want 2 (exactly one repair, never more)", act.Iterations)
	}
	if len(exec.calls) != 2 {
		t.Errorf("Executor called %d times, want 2", len(exec.calls))
	}
}

func TestEngine_RunBudgeted_RepairRefusedByExhaustedBudget(t *testing.T) {
	exec := &captureExecutor{patches: []string{"first-patch"}}
	verifier := &seqVerifier{judgments: []*domain.Judgment{
		{Verdict: "fail", Checked: []string{"tests: fail"}},
	}}
	eng := engine.NewEngine(&fakeGatherer{}, exec, verifier, "")

	act, err := eng.RunBudgeted(context.Background(), &domain.Intent{Text: "test"},
		&domain.Budget{MaxIterations: 1, MaxCostUSD: 1.00})
	if err != nil {
		t.Fatalf("RunBudgeted failed: %v (budget-refused repair is not an error)", err)
	}
	if act.JudgmentVerdict != "fail" {
		t.Errorf("JudgmentVerdict = %q, want %q", act.JudgmentVerdict, "fail")
	}
	if len(exec.calls) != 1 {
		t.Errorf("Executor called %d times, want 1 (no budget for repair)", len(exec.calls))
	}
	if act.Iterations != 1 {
		t.Errorf("Iterations = %d, want 1", act.Iterations)
	}
}

func TestEngine_Run_RepairExecuteErrorStopsLifecycle(t *testing.T) {
	exec := &captureExecutor{
		patches: []string{"first-patch", ""},
		errs:    []error{nil, errors.New("execute boom")},
	}
	verifier := &seqVerifier{judgments: []*domain.Judgment{
		{Verdict: "fail", Checked: []string{"tests: fail"}},
	}}
	eng := engine.NewEngine(&fakeGatherer{}, exec, verifier, "")

	if _, err := eng.Run(context.Background(), &domain.Intent{Text: "test"}); err == nil {
		t.Fatal("Run with failing repair Execute returned nil error")
	}
}

func TestScriptedExecutor_Deterministic(t *testing.T) {
	exec := executor.NewScriptedExecutor("fixed-patch")

	outcome1, err := exec.Execute(context.Background(), &domain.Intent{Text: "a"}, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	outcome2, err := exec.Execute(context.Background(), &domain.Intent{Text: "totally different intent"}, []string{"other.go"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if outcome1.Patch != "fixed-patch" {
		t.Errorf("outcome1.Patch = %q, want %q", outcome1.Patch, "fixed-patch")
	}
	if outcome2.Patch != "fixed-patch" {
		t.Errorf("outcome2.Patch = %q, want %q", outcome2.Patch, "fixed-patch")
	}
}

// fakeReporter records every engine.Reporter event, in call order, as
// human-readable strings for assertions.
type fakeReporter struct {
	events []string
}

func (r *fakeReporter) Gathering() { r.events = append(r.events, "gathering") }
func (r *fakeReporter) Executing(iteration int) {
	r.events = append(r.events, fmt.Sprintf("executing:%d", iteration))
}
func (r *fakeReporter) Verifying(iteration int) {
	r.events = append(r.events, fmt.Sprintf("verifying:%d", iteration))
}
func (r *fakeReporter) Verified(iteration int, judgment *domain.Judgment) {
	r.events = append(r.events, fmt.Sprintf("verified:%d:%s", iteration, judgment.Verdict))
}
func (r *fakeReporter) Repairing() { r.events = append(r.events, "repairing") }
func (r *fakeReporter) RepairSkipped(reason string) {
	r.events = append(r.events, "repair-skipped:"+reason)
}
func (r *fakeReporter) BudgetExceeded(reason string) {
	r.events = append(r.events, "budget-exceeded:"+reason)
}

func TestEngine_Run_ReportsPassWithoutRepair(t *testing.T) {
	eng := newEngine("pass")
	reporter := &fakeReporter{}
	eng.SetReporter(reporter)

	if _, err := eng.Run(context.Background(), &domain.Intent{Text: "test"}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	want := []string{"gathering", "executing:1", "verifying:1", "verified:1:pass"}
	if !reflect.DeepEqual(reporter.events, want) {
		t.Errorf("events = %v, want %v", reporter.events, want)
	}
}

func TestEngine_Run_ReportsRepairRound(t *testing.T) {
	exec := &captureExecutor{patches: []string{"first-patch", "repaired-patch"}}
	verifier := &seqVerifier{judgments: []*domain.Judgment{
		{Verdict: "fail", Checked: []string{"tests: fail"}},
		{Verdict: "pass"},
	}}
	eng := engine.NewEngine(&fakeGatherer{files: []string{"main.go"}}, exec, verifier, "")
	reporter := &fakeReporter{}
	eng.SetReporter(reporter)

	if _, err := eng.Run(context.Background(), &domain.Intent{Text: "test"}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	want := []string{
		"gathering", "executing:1", "verifying:1", "verified:1:fail",
		"repairing", "executing:2", "verifying:2", "verified:2:pass",
	}
	if !reflect.DeepEqual(reporter.events, want) {
		t.Errorf("events = %v, want %v", reporter.events, want)
	}
}

func TestEngine_RunBudgeted_ReportsBudgetExceededBeforeExecute(t *testing.T) {
	eng := newEngine("pass")
	reporter := &fakeReporter{}
	eng.SetReporter(reporter)

	_, err := eng.RunBudgeted(context.Background(), &domain.Intent{Text: "test"},
		&domain.Budget{MaxIterations: 0, MaxCostUSD: 1.00})
	if !errors.Is(err, engine.ErrBudgetExceeded) {
		t.Fatalf("error = %v, want ErrBudgetExceeded", err)
	}

	want := []string{"gathering", "budget-exceeded:budget exceeded: iteration 1 over limit 0"}
	if !reflect.DeepEqual(reporter.events, want) {
		t.Errorf("events = %v, want %v", reporter.events, want)
	}
}

func TestEngine_RunBudgeted_ReportsRepairSkippedWhenBudgetRefuses(t *testing.T) {
	exec := &captureExecutor{patches: []string{"first-patch"}}
	verifier := &seqVerifier{judgments: []*domain.Judgment{{Verdict: "fail", Checked: []string{"tests: fail"}}}}
	eng := engine.NewEngine(&fakeGatherer{}, exec, verifier, "")
	reporter := &fakeReporter{}
	eng.SetReporter(reporter)

	_, err := eng.RunBudgeted(context.Background(), &domain.Intent{Text: "test"},
		&domain.Budget{MaxIterations: 1, MaxCostUSD: 1.00})
	if err != nil {
		t.Fatalf("RunBudgeted failed: %v (budget-refused repair is not an error)", err)
	}

	want := []string{
		"gathering", "executing:1", "verifying:1", "verified:1:fail", "repairing",
		"repair-skipped:budget exceeded: iteration 2 over limit 1",
	}
	if !reflect.DeepEqual(reporter.events, want) {
		t.Errorf("events = %v, want %v", reporter.events, want)
	}
}

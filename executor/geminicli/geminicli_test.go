package geminicli

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	"foundry/domain"
)

// fakeRunner is an injectable runner that returns canned output and
// captures what it was invoked with — mirrors executor/claude's own test
// double exactly.
type fakeRunner struct {
	stdout, stderr string
	err            error

	gotDir, gotName, gotStdin string
	gotArgs                   []string
}

func (f *fakeRunner) Run(ctx context.Context, dir, name string, args []string, stdin string) (string, string, error) {
	f.gotDir, f.gotName, f.gotArgs, f.gotStdin = dir, name, args, stdin
	return f.stdout, f.stderr, f.err
}

func newExecutor(r runner) *Executor {
	return &Executor{
		workspace:  "/repo",
		executable: "gemini",
		timeout:    time.Minute,
		runner:     r,
	}
}

// sampleDiff ends in a newline because git apply rejects a patch whose last
// line is unterminated; executor.ParsePatch guarantees this normalization.
const sampleDiff = `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1 +1,2 @@
 package main
+// added
`

func jsonOutput(response string) string {
	return `{"response": ` + quoteJSON(response) + `}`
}

// quoteJSON is a tiny, dependency-free JSON string quoter for building
// fixture output in tests (equivalent to what json.Marshal would produce
// for a bare string).
func quoteJSON(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func TestExecute_Success(t *testing.T) {
	r := &fakeRunner{stdout: jsonOutput("```diff\n" + sampleDiff + "```\n")}
	e := newExecutor(r)

	outcome, err := e.Execute(context.Background(), &domain.Intent{Text: "add a comment"}, []string{"main.go contents"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if outcome.Patch != sampleDiff {
		t.Errorf("Patch = %q, want %q", outcome.Patch, sampleDiff)
	}

	if r.gotDir != "/repo" {
		t.Errorf("runner dir = %q, want %q", r.gotDir, "/repo")
	}
	if r.gotName != "gemini" {
		t.Errorf("runner name = %q, want %q", r.gotName, "gemini")
	}
	wantArgs := []string{"--output-format", "json"}
	if len(r.gotArgs) != len(wantArgs) {
		t.Fatalf("runner args = %v, want %v", r.gotArgs, wantArgs)
	}
	for i := range wantArgs {
		if r.gotArgs[i] != wantArgs[i] {
			t.Errorf("runner args = %v, want %v", r.gotArgs, wantArgs)
		}
	}
	if !strings.Contains(r.gotStdin, "add a comment") {
		t.Errorf("prompt missing intent, got:\n%s", r.gotStdin)
	}
	if !strings.Contains(r.gotStdin, "main.go contents") {
		t.Errorf("prompt missing gathered context, got:\n%s", r.gotStdin)
	}
}

func TestExecute_ModelFlagAppendedWhenSet(t *testing.T) {
	r := &fakeRunner{stdout: jsonOutput("```diff\n" + sampleDiff + "```\n")}
	e := newExecutor(r)
	e.model = "gemini-3.5-flash"

	if _, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	wantArgs := []string{"--output-format", "json", "-m", "gemini-3.5-flash"}
	if len(r.gotArgs) != len(wantArgs) {
		t.Fatalf("runner args = %v, want %v", r.gotArgs, wantArgs)
	}
	for i := range wantArgs {
		if r.gotArgs[i] != wantArgs[i] {
			t.Errorf("runner args = %v, want %v", r.gotArgs, wantArgs)
		}
	}
}

// TestExecute_ToleratesDiagnosticNoiseBeforeJSON covers the CLI printing
// "Loaded cached credentials." (or similar) to stdout ahead of the JSON
// blob when reusing a cached "Sign in with Google" login.
func TestExecute_ToleratesDiagnosticNoiseBeforeJSON(t *testing.T) {
	r := &fakeRunner{stdout: "Loaded cached credentials.\n" + jsonOutput("```diff\n"+sampleDiff+"```\n")}
	e := newExecutor(r)

	outcome, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if outcome.Patch != sampleDiff {
		t.Errorf("Patch = %q, want %q", outcome.Patch, sampleDiff)
	}
}

func TestExecute_ExecutableMissing(t *testing.T) {
	e := newExecutor(&fakeRunner{err: exec.ErrNotFound})

	_, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil)
	if err == nil {
		t.Fatal("Execute returned nil error for missing executable")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want it to mention 'not found'", err)
	}
}

func TestExecute_Timeout(t *testing.T) {
	e := newExecutor(&fakeRunner{err: context.DeadlineExceeded})

	_, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil)
	if err == nil {
		t.Fatal("Execute returned nil error on timeout")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error = %q, want it to mention 'timed out'", err)
	}
}

func TestExecute_Failure(t *testing.T) {
	e := newExecutor(&fakeRunner{err: errors.New("exit status 1"), stderr: "boom on line 3"})

	_, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil)
	if err == nil {
		t.Fatal("Execute returned nil error on failure")
	}
	if !strings.Contains(err.Error(), "execution failed") {
		t.Errorf("error = %q, want it to mention 'execution failed'", err)
	}
	if !strings.Contains(err.Error(), "boom on line 3") {
		t.Errorf("error = %q, want it to include captured stderr", err)
	}
}

// TestExecute_FailureWithEmptyStreams guards against "execution failed:
// exit status 1: " with nothing to debug, the same regression
// executor/claude's own test of the same name guards against.
func TestExecute_FailureWithEmptyStreams(t *testing.T) {
	e := newExecutor(&fakeRunner{err: errors.New("exit status 1")})

	_, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil)
	if err == nil {
		t.Fatal("Execute returned nil error on failure")
	}
	if !strings.Contains(err.Error(), "gemini -p") {
		t.Errorf("error = %q, want a concrete next debugging step when both streams are empty", err)
	}
}

// TestExecute_CLIReportedError covers the documented --output-format
// json error field (geminicli.com/docs/cli/headless): a successful process
// exit that nonetheless carries an error inside the JSON body.
func TestExecute_CLIReportedError(t *testing.T) {
	e := newExecutor(&fakeRunner{stdout: `{"error": {"message": "quota exceeded"}}`})

	_, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil)
	if err == nil {
		t.Fatal("Execute returned nil error when the JSON body carries an error field")
	}
	if !strings.Contains(err.Error(), "quota exceeded") {
		t.Errorf("error = %q, want it to include the CLI's own error message", err)
	}
}

func TestExecute_EmptyResponseField(t *testing.T) {
	e := newExecutor(&fakeRunner{stdout: `{"response": ""}`})

	if _, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil); err == nil {
		t.Fatal("Execute returned nil error for an empty response field")
	}
}

func TestExecute_NoDiffInResponse(t *testing.T) {
	e := newExecutor(&fakeRunner{stdout: jsonOutput("I could not make the change.")})

	if _, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil); err == nil {
		t.Fatal("Execute returned nil error for a response without a diff")
	}
}

func TestExecute_UnparseableOutput(t *testing.T) {
	e := newExecutor(&fakeRunner{stdout: "not json at all, no braces either"})

	_, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil)
	if err == nil {
		t.Fatal("Execute returned nil error for output with no JSON at all")
	}
	if !strings.Contains(err.Error(), "no JSON output found") {
		t.Errorf("error = %q, want it to say no JSON output was found", err)
	}
}

func TestExecute_NoWorkspace(t *testing.T) {
	e := NewExecutor("", "")

	if _, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil); err == nil {
		t.Fatal("Execute returned nil error with no workspace configured")
	}
}

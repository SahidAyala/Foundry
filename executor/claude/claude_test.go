package claude

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	"foundry/domain"
)

// fakeRunner is an injectable runner that returns canned output and captures
// what it was invoked with.
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

func newExecutor(r runner) *ClaudeExecutor {
	return &ClaudeExecutor{
		workspace:  "/repo",
		executable: "claude",
		timeout:    time.Minute,
		runner:     r,
	}
}

// sampleDiff ends in a newline because git apply rejects a patch whose last
// line is unterminated; parsePatch guarantees this normalization.
const sampleDiff = `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1 +1,2 @@
 package main
+// added
`

func TestExecute_Success(t *testing.T) {
	r := &fakeRunner{stdout: "```diff\n" + sampleDiff + "```\n"}
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
	if r.gotName != "claude" {
		t.Errorf("runner name = %q, want %q", r.gotName, "claude")
	}
	if len(r.gotArgs) != 1 || r.gotArgs[0] != "-p" {
		t.Errorf("runner args = %v, want [-p]", r.gotArgs)
	}
	if !strings.Contains(r.gotStdin, "add a comment") {
		t.Errorf("prompt missing intent, got:\n%s", r.gotStdin)
	}
	if !strings.Contains(r.gotStdin, "main.go contents") {
		t.Errorf("prompt missing gathered context, got:\n%s", r.gotStdin)
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

func TestExecute_EmptyOutput(t *testing.T) {
	e := newExecutor(&fakeRunner{stdout: "   \n"})

	if _, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil); err == nil {
		t.Fatal("Execute returned nil error for empty output")
	}
}

func TestExecute_NoDiffInOutput(t *testing.T) {
	e := newExecutor(&fakeRunner{stdout: "I could not make the change.\n"})

	if _, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil); err == nil {
		t.Fatal("Execute returned nil error for output without a diff")
	}
}

func TestExecute_NoWorkspace(t *testing.T) {
	e := NewClaudeExecutor("")

	if _, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil); err == nil {
		t.Fatal("Execute returned nil error with no workspace configured")
	}
}

func TestParsePatch_Fenced(t *testing.T) {
	patch, err := parsePatch("Here is the change:\n```diff\n" + sampleDiff + "```\ntrailing prose")
	if err != nil {
		t.Fatalf("parsePatch failed: %v", err)
	}
	if patch != sampleDiff {
		t.Errorf("patch = %q, want %q", patch, sampleDiff)
	}
}

func TestParsePatch_RawDiffGit(t *testing.T) {
	patch, err := parsePatch("prose before\n" + sampleDiff)
	if err != nil {
		t.Fatalf("parsePatch failed: %v", err)
	}
	if patch != sampleDiff {
		t.Errorf("patch = %q, want %q", patch, sampleDiff)
	}
}

func TestParsePatch_RawMinusMarker(t *testing.T) {
	raw := "--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b\n"
	patch, err := parsePatch("blah\n" + raw)
	if err != nil {
		t.Fatalf("parsePatch failed: %v", err)
	}
	if patch != raw {
		t.Errorf("patch = %q, want %q", patch, raw)
	}
}

// git apply rejects a patch whose final line has no newline; parsePatch must
// terminate the extracted patch regardless of how Claude Code's output ends.
func TestParsePatch_NormalizesTrailingNewline(t *testing.T) {
	unterminated := "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b"
	for name, in := range map[string]string{
		"raw":    unterminated,
		"fenced": "```diff\n" + unterminated + "\n```",
	} {
		patch, err := parsePatch(in)
		if err != nil {
			t.Fatalf("%s: parsePatch failed: %v", name, err)
		}
		if !strings.HasSuffix(patch, "\n") || strings.HasSuffix(patch, "\n\n") {
			t.Errorf("%s: patch does not end in exactly one newline: %q", name, patch)
		}
	}
}

func TestParsePatch_Deterministic(t *testing.T) {
	in := "```diff\n" + sampleDiff + "\n```\n"
	first, err := parsePatch(in)
	if err != nil {
		t.Fatalf("parsePatch failed: %v", err)
	}
	for i := 0; i < 5; i++ {
		got, err := parsePatch(in)
		if err != nil {
			t.Fatalf("parsePatch failed on repeat: %v", err)
		}
		if got != first {
			t.Fatalf("parsePatch not deterministic: %q != %q", got, first)
		}
	}
}

func TestParsePatch_Empty(t *testing.T) {
	if _, err := parsePatch(""); err == nil {
		t.Fatal("parsePatch returned nil error for empty input")
	}
}

func TestParsePatch_NoDiff(t *testing.T) {
	if _, err := parsePatch("just some prose, no diff here"); err == nil {
		t.Fatal("parsePatch returned nil error for prose input")
	}
}

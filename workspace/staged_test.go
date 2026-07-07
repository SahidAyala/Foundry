package workspace

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"foundry/domain"
)

// gitRun runs git with args in dir and returns its trimmed output, failing
// the test on error.
func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// recordingVerifier captures the workspace it was asked to verify and
// returns a fixed Judgment, optionally inspecting the staged directory.
type recordingVerifier struct {
	sawWorkspace string
	inspect      func(t *testing.T, staged string)
	t            *testing.T
}

func (v *recordingVerifier) Verify(ctx context.Context, outcome *domain.Outcome, ws string) (*domain.Judgment, error) {
	v.sawWorkspace = ws
	if v.inspect != nil {
		v.inspect(v.t, ws)
	}
	return &domain.Judgment{Verdict: "pass", Checked: []string{"check: pass"}}, nil
}

// stagedNewFilePatch creates NEW.md as a pure addition; it applies cleanly
// to any tree lacking that file.
const stagedNewFilePatch = "diff --git a/NEW.md b/NEW.md\n" +
	"new file mode 100644\n" +
	"--- /dev/null\n" +
	"+++ b/NEW.md\n" +
	"@@ -0,0 +1 @@\n" +
	"+staged\n"

func TestStagedVerifier_ValidatorsSeePatchedState(t *testing.T) {
	repo := initGitRepo(t)

	inner := &recordingVerifier{t: t, inspect: func(t *testing.T, staged string) {
		if _, err := os.Stat(filepath.Join(staged, "NEW.md")); err != nil {
			t.Errorf("staged worktree missing the patched file: %v", err)
		}
		if _, err := os.Stat(filepath.Join(staged, "greeting.txt")); err != nil {
			t.Errorf("staged worktree missing the committed file: %v", err)
		}
	}}

	judgment, err := NewStagedVerifier(inner).Verify(
		context.Background(), &domain.Outcome{Patch: stagedNewFilePatch}, repo)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if judgment.Verdict != "pass" {
		t.Errorf("Verdict = %q, want %q", judgment.Verdict, "pass")
	}
	if inner.sawWorkspace == repo {
		t.Error("inner verifier ran against the developer's checkout, want a staged copy")
	}
}

func TestStagedVerifier_CheckoutIsNeverTouched(t *testing.T) {
	repo := initGitRepo(t)

	inner := &recordingVerifier{t: t}
	if _, err := NewStagedVerifier(inner).Verify(
		context.Background(), &domain.Outcome{Patch: stagedNewFilePatch}, repo); err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(repo, "NEW.md")); !os.IsNotExist(err) {
		t.Error("verification applied the patch to the developer's checkout")
	}
	if status := gitRun(t, repo, "status", "--porcelain"); status != "" {
		t.Errorf("verification dirtied the checkout:\n%s", status)
	}
	if worktrees := gitRun(t, repo, "worktree", "list", "--porcelain"); strings.Count(worktrees, "worktree ") != 1 {
		t.Errorf("staged worktree was not cleaned up:\n%s", worktrees)
	}
	if _, err := os.Stat(inner.sawWorkspace); !os.IsNotExist(err) {
		t.Errorf("staged directory %q was not removed", inner.sawWorkspace)
	}
}

func TestStagedVerifier_UnappliablePatchFailsWithFindings(t *testing.T) {
	repo := initGitRepo(t)

	// The context line "no such line" does not exist in greeting.txt.
	badPatch := "diff --git a/greeting.txt b/greeting.txt\n" +
		"--- a/greeting.txt\n" +
		"+++ b/greeting.txt\n" +
		"@@ -1 +1 @@\n" +
		"-no such line\n" +
		"+replacement\n"

	inner := &recordingVerifier{t: t}
	judgment, err := NewStagedVerifier(inner).Verify(
		context.Background(), &domain.Outcome{Patch: badPatch}, repo)
	if err != nil {
		t.Fatalf("Verify failed: %v (an unappliable patch is a fail verdict, not an error)", err)
	}
	if judgment.Verdict != "fail" {
		t.Errorf("Verdict = %q, want %q", judgment.Verdict, "fail")
	}
	if len(judgment.Checked) == 0 || !strings.Contains(judgment.Checked[0], "apply-patch: fail") {
		t.Errorf("Judgment missing apply findings, got %q", judgment.Checked)
	}
	if inner.sawWorkspace != "" {
		t.Error("inner verifier ran despite the patch not applying")
	}
}

func TestStagedVerifier_EmptyPatchVerifiesHead(t *testing.T) {
	repo := initGitRepo(t)

	inner := &recordingVerifier{t: t, inspect: func(t *testing.T, staged string) {
		if _, err := os.Stat(filepath.Join(staged, "greeting.txt")); err != nil {
			t.Errorf("staged worktree missing the committed file: %v", err)
		}
	}}
	judgment, err := NewStagedVerifier(inner).Verify(
		context.Background(), &domain.Outcome{Patch: ""}, repo)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if judgment.Verdict != "pass" {
		t.Errorf("Verdict = %q, want %q", judgment.Verdict, "pass")
	}
}

func TestStagedVerifier_NonGitRepoIsAnError(t *testing.T) {
	inner := &recordingVerifier{t: t}
	_, err := NewStagedVerifier(inner).Verify(
		context.Background(), &domain.Outcome{Patch: stagedNewFilePatch}, t.TempDir())
	if err == nil {
		t.Fatal("Verify on a non-git directory returned nil error")
	}
}

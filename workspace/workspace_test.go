package workspace

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// initGitRepo creates a temporary git repository with a single committed
// file, greeting.txt, containing "hello\n".
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	run("init", "-q", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(dir, "greeting.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}
	run("add", ".")
	run("commit", "-q", "-m", "initial commit")

	return dir
}

const replacePatch = `diff --git a/greeting.txt b/greeting.txt
index 0000000..1111111 100644
--- a/greeting.txt
+++ b/greeting.txt
@@ -1 +1 @@
-hello
+%s
`

// newFilePatch returns a unified diff that adds a brand-new file named
// filename with content — used where a test needs two Workspaces over one
// repo to apply non-conflicting patches.
func newFilePatch(filename, content string) string {
	return fmt.Sprintf(`diff --git a/%s b/%s
new file mode 100644
index 0000000..1111111
--- /dev/null
+++ b/%s
@@ -0,0 +1 @@
+%s
`, filename, filename, filename, content)
}

func TestNewWorkspace_RejectsNonGitRepo(t *testing.T) {
	dir := t.TempDir()

	if _, err := NewWorkspace(dir, "feature"); err == nil {
		t.Fatal("NewWorkspace on non-git directory returned nil error")
	}
}

func TestNewWorkspace_RejectsUnsafeBranchNames(t *testing.T) {
	repo := initGitRepo(t)

	unsafe := []string{
		"",
		"-flag",
		"../escape",
		"feature/../etc",
		"trailing.",
		"has space",
		"semi;colon",
		"bad~name",
		"bad^name",
		"bad:name",
		"$(rm -rf /)",
	}

	for _, name := range unsafe {
		if _, err := NewWorkspace(repo, name); err == nil {
			t.Errorf("NewWorkspace(%q) returned nil error, want error", name)
		}
	}
}

// TestNewWorkspace_FailedWorktreeAddPrunesWithoutTouchingExistingBranch
// covers the error path's cleanup: when `git worktree add` fails, cleanup
// must attempt `git worktree prune` (the same fallback cleanup() already
// uses for a stale registration whose directory is gone), but must never
// delete a same-named branch that already existed before the failed call
// — that branch was not created by this attempt and cleaning up after a
// failure must not destroy state it doesn't own.
func TestNewWorkspace_FailedWorktreeAddPrunesWithoutTouchingExistingBranch(t *testing.T) {
	repo := initGitRepo(t)
	ctx := context.Background()

	const preexisting = "already-exists"
	if _, err := gitOutput(ctx, repo, "branch", preexisting); err != nil {
		t.Fatalf("create pre-existing branch: %v", err)
	}

	// `worktree add -b <name>` fails because the branch already exists —
	// this is not a worktree registered by us, so it must survive.
	if _, err := NewWorkspace(repo, preexisting); err == nil {
		t.Fatal("NewWorkspace with an already-existing branch name returned nil error")
	}

	if _, err := gitOutput(ctx, repo, "rev-parse", "--verify", preexisting); err != nil {
		t.Errorf("pre-existing branch %q was deleted by NewWorkspace's failed-attempt cleanup: %v", preexisting, err)
	}

	worktrees, err := gitOutput(ctx, repo, "worktree", "list", "--porcelain")
	if err != nil {
		t.Fatalf("worktree list: %v", err)
	}
	if strings.Contains(worktrees, preexisting) {
		t.Errorf("stray worktree registration left behind: %s", worktrees)
	}
}

// TestWorkspace_Apply_ValidPatch verifies Apply's isolation guarantee: the
// developer's actual repo directory must be completely unaffected by Apply
// — the patch is only visible in the isolated worktree.
func TestWorkspace_Apply_ValidPatch(t *testing.T) {
	repo := initGitRepo(t)

	ws, err := NewWorkspace(repo, "ws-valid-patch")
	if err != nil {
		t.Fatalf("NewWorkspace failed: %v", err)
	}

	patch := strings.Replace(replacePatch, "%s", "hello, world", 1)
	if err := ws.Apply(context.Background(), patch); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	repoData, err := os.ReadFile(filepath.Join(repo, "greeting.txt"))
	if err != nil {
		t.Fatalf("read repo file: %v", err)
	}
	if string(repoData) != "hello\n" {
		t.Errorf("repo file contents = %q, want unchanged %q — Apply must not touch the developer's checkout", repoData, "hello\n")
	}

	worktreeData, err := os.ReadFile(filepath.Join(ws.worktreeDir, "greeting.txt"))
	if err != nil {
		t.Fatalf("read worktree file: %v", err)
	}
	if string(worktreeData) != "hello, world\n" {
		t.Errorf("worktree file contents = %q, want %q", worktreeData, "hello, world\n")
	}

	if err := ws.Clean(context.Background()); err != nil {
		t.Fatalf("Clean failed: %v", err)
	}
}

func TestWorkspace_Apply_Conflict(t *testing.T) {
	repo := initGitRepo(t)

	ws, err := NewWorkspace(repo, "ws-conflict")
	if err != nil {
		t.Fatalf("NewWorkspace failed: %v", err)
	}
	defer ws.Clean(context.Background())

	patch := `diff --git a/greeting.txt b/greeting.txt
index 0000000..1111111 100644
--- a/greeting.txt
+++ b/greeting.txt
@@ -1 +1 @@
-this line does not exist in the file
+hello, world
`

	if err := ws.Apply(context.Background(), patch); err == nil {
		t.Fatal("Apply with conflicting patch returned nil error")
	}
}

// TestWorkspace_Clean_RestoresOriginalState verifies Clean discards a
// worktree's applied change and leaves no trace: no leftover branch, no
// leftover worktree registration, no leftover patch file. Unlike before
// this package used real worktrees, repo's own branch was never touched in
// the first place, so there is nothing to actively "restore."
func TestWorkspace_Clean_RestoresOriginalState(t *testing.T) {
	repo := initGitRepo(t)

	ws, err := NewWorkspace(repo, "temp-branch")
	if err != nil {
		t.Fatalf("NewWorkspace failed: %v", err)
	}

	patch := strings.Replace(replacePatch, "%s", "goodbye", 1)
	if err := ws.Apply(context.Background(), patch); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if err := ws.Clean(context.Background()); err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	branch, err := gitOutput(context.Background(), repo, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse failed: %v", err)
	}
	if branch != "main" {
		t.Errorf("current branch = %q, want %q", branch, "main")
	}

	data, err := os.ReadFile(filepath.Join(repo, "greeting.txt"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "hello\n" {
		t.Errorf("file contents = %q, want original %q", data, "hello\n")
	}

	list, err := gitOutput(context.Background(), repo, "branch", "--list", "temp-branch")
	if err != nil {
		t.Fatalf("branch --list failed: %v", err)
	}
	if list != "" {
		t.Errorf("branch %q still exists after Clean", "temp-branch")
	}

	if _, err := os.Stat(ws.patchPath); !os.IsNotExist(err) {
		t.Errorf("patch file %q was not removed by Clean", ws.patchPath)
	}

	assertNoLeftoverWorktrees(t, repo)
}

// TestWorkspace_Land_KeepsChangeAndReturnsToOriginalBranch guards against a
// throwaway `foundry/act-<id>` branch (or worktree) being left behind after
// a successful apply: Land must carry the applied change onto repo's
// actual working directory while leaving repo's branch untouched
// throughout, unlike Clean which discards the change.
func TestWorkspace_Land_KeepsChangeAndReturnsToOriginalBranch(t *testing.T) {
	repo := initGitRepo(t)

	ws, err := NewWorkspace(repo, "temp-branch")
	if err != nil {
		t.Fatalf("NewWorkspace failed: %v", err)
	}

	patch := strings.Replace(replacePatch, "%s", "goodbye", 1)
	if err := ws.Apply(context.Background(), patch); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	branchDuringApply, err := gitOutput(context.Background(), repo, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse failed: %v", err)
	}
	if branchDuringApply != "main" {
		t.Errorf("repo's branch changed to %q during Apply, want it to stay %q throughout", branchDuringApply, "main")
	}

	if err := ws.Land(context.Background()); err != nil {
		t.Fatalf("Land failed: %v", err)
	}

	branch, err := gitOutput(context.Background(), repo, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse failed: %v", err)
	}
	if branch != "main" {
		t.Errorf("current branch = %q, want %q", branch, "main")
	}

	data, err := os.ReadFile(filepath.Join(repo, "greeting.txt"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "goodbye\n" {
		t.Errorf("file contents = %q, want the applied change %q", data, "goodbye\n")
	}

	list, err := gitOutput(context.Background(), repo, "branch", "--list", "temp-branch")
	if err != nil {
		t.Fatalf("branch --list failed: %v", err)
	}
	if list != "" {
		t.Errorf("branch %q still exists after Land", "temp-branch")
	}

	if _, err := os.Stat(ws.patchPath); !os.IsNotExist(err) {
		t.Errorf("patch file %q was not removed by Land", ws.patchPath)
	}

	assertNoLeftoverWorktrees(t, repo)
}

func TestWorkspace_TwoWorkspaces_NoInterference(t *testing.T) {
	repoA := initGitRepo(t)
	repoB := initGitRepo(t)

	wsA, err := NewWorkspace(repoA, "feature-a")
	if err != nil {
		t.Fatalf("NewWorkspace(A) failed: %v", err)
	}
	wsB, err := NewWorkspace(repoB, "feature-b")
	if err != nil {
		t.Fatalf("NewWorkspace(B) failed: %v", err)
	}

	patchA := strings.Replace(replacePatch, "%s", "from A", 1)
	patchB := strings.Replace(replacePatch, "%s", "from B", 1)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errs[0] = wsA.Apply(context.Background(), patchA)
	}()
	go func() {
		defer wg.Done()
		errs[1] = wsB.Apply(context.Background(), patchB)
	}()
	wg.Wait()

	if errs[0] != nil {
		t.Fatalf("wsA.Apply failed: %v", errs[0])
	}
	if errs[1] != nil {
		t.Fatalf("wsB.Apply failed: %v", errs[1])
	}

	if err := wsA.Land(context.Background()); err != nil {
		t.Fatalf("wsA.Land failed: %v", err)
	}
	if err := wsB.Land(context.Background()); err != nil {
		t.Fatalf("wsB.Land failed: %v", err)
	}

	dataA, err := os.ReadFile(filepath.Join(repoA, "greeting.txt"))
	if err != nil {
		t.Fatalf("read repoA file: %v", err)
	}
	if string(dataA) != "from A\n" {
		t.Errorf("repoA file contents = %q, want %q", dataA, "from A\n")
	}

	dataB, err := os.ReadFile(filepath.Join(repoB, "greeting.txt"))
	if err != nil {
		t.Fatalf("read repoB file: %v", err)
	}
	if string(dataB) != "from B\n" {
		t.Errorf("repoB file contents = %q, want %q", dataB, "from B\n")
	}
}

// TestWorkspace_TwoWorkspaces_SameRepo_NoInterference is the core new
// capability this package's worktree redesign delivers: two Workspaces
// constructed over the SAME repo (simulating two independent Foundry
// processes/sessions), each applying and landing a non-conflicting patch
// concurrently, never interfering with each other or with repo's own
// checked-out branch. Run with -race to mean anything.
func TestWorkspace_TwoWorkspaces_SameRepo_NoInterference(t *testing.T) {
	repo := initGitRepo(t)

	wsA, err := NewWorkspace(repo, "foundry/act-aaa")
	if err != nil {
		t.Fatalf("NewWorkspace(A) failed: %v", err)
	}
	wsB, err := NewWorkspace(repo, "foundry/act-bbb")
	if err != nil {
		t.Fatalf("NewWorkspace(B) failed: %v", err)
	}

	patchA := newFilePatch("from-a.txt", "hello from A")
	patchB := newFilePatch("from-b.txt", "hello from B")

	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errs[0] = wsA.Apply(context.Background(), patchA)
	}()
	go func() {
		defer wg.Done()
		errs[1] = wsB.Apply(context.Background(), patchB)
	}()
	wg.Wait()

	if errs[0] != nil {
		t.Fatalf("wsA.Apply failed: %v", errs[0])
	}
	if errs[1] != nil {
		t.Fatalf("wsB.Apply failed: %v", errs[1])
	}

	branchDuringApply, err := gitOutput(context.Background(), repo, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse failed: %v", err)
	}
	if branchDuringApply != "main" {
		t.Errorf("repo's branch changed to %q while two Acts were applying concurrently, want it to stay %q", branchDuringApply, "main")
	}

	if _, err := os.Stat(filepath.Join(repo, "from-a.txt")); !os.IsNotExist(err) {
		t.Error("repo already has from-a.txt before either Land — Apply must not have touched repo directly")
	}
	if _, err := os.Stat(filepath.Join(repo, "from-b.txt")); !os.IsNotExist(err) {
		t.Error("repo already has from-b.txt before either Land — Apply must not have touched repo directly")
	}

	wg.Add(2)
	go func() {
		defer wg.Done()
		errs[0] = wsA.Land(context.Background())
	}()
	go func() {
		defer wg.Done()
		errs[1] = wsB.Land(context.Background())
	}()
	wg.Wait()

	if errs[0] != nil {
		t.Fatalf("wsA.Land failed: %v", errs[0])
	}
	if errs[1] != nil {
		t.Fatalf("wsB.Land failed: %v", errs[1])
	}

	dataA, err := os.ReadFile(filepath.Join(repo, "from-a.txt"))
	if err != nil {
		t.Fatalf("read from-a.txt: %v", err)
	}
	if string(dataA) != "hello from A\n" {
		t.Errorf("from-a.txt contents = %q, want %q", dataA, "hello from A\n")
	}
	dataB, err := os.ReadFile(filepath.Join(repo, "from-b.txt"))
	if err != nil {
		t.Fatalf("read from-b.txt: %v", err)
	}
	if string(dataB) != "hello from B\n" {
		t.Errorf("from-b.txt contents = %q, want %q", dataB, "hello from B\n")
	}

	branch, err := gitOutput(context.Background(), repo, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse failed: %v", err)
	}
	if branch != "main" {
		t.Errorf("repo's branch = %q after both Lands, want %q", branch, "main")
	}

	assertNoLeftoverWorktrees(t, repo)
}

// TestWorkspace_Clean_PreservesPreexistingUncommittedChanges verifies a
// latent bug fixed by this package's worktree redesign: previously, Clean
// ran `git reset --hard` + `git clean -fd` directly in repo, which would
// have destroyed any of the developer's own pre-existing uncommitted
// changes. Since repo is never touched at all now, they must survive.
func TestWorkspace_Clean_PreservesPreexistingUncommittedChanges(t *testing.T) {
	repo := initGitRepo(t)

	if err := os.WriteFile(filepath.Join(repo, "greeting.txt"), []byte("hello, developer's own edit\n"), 0o644); err != nil {
		t.Fatalf("write developer's own edit: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "untracked.txt"), []byte("developer's own untracked file\n"), 0o644); err != nil {
		t.Fatalf("write developer's own untracked file: %v", err)
	}

	ws, err := NewWorkspace(repo, "temp-branch")
	if err != nil {
		t.Fatalf("NewWorkspace failed: %v", err)
	}

	patch := newFilePatch("from-act.txt", "hello from the Act")
	if err := ws.Apply(context.Background(), patch); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if err := ws.Clean(context.Background()); err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repo, "greeting.txt"))
	if err != nil {
		t.Fatalf("read greeting.txt: %v", err)
	}
	if string(data) != "hello, developer's own edit\n" {
		t.Errorf("greeting.txt = %q, want the developer's own uncommitted edit preserved: %q", data, "hello, developer's own edit\n")
	}

	if _, err := os.Stat(filepath.Join(repo, "untracked.txt")); err != nil {
		t.Errorf("untracked.txt was removed by Clean, want the developer's own untracked file preserved: %v", err)
	}
}

// TestWorkspace_Land_RefusesIfBranchChanged verifies Land's safety check:
// if repo's checked-out branch has changed since NewWorkspace (e.g. the
// developer switched branches while an Act was mid-flight), Land must
// refuse rather than silently apply the patch onto the wrong branch — and
// must leave the worktree, branch, and patch file in place for manual
// recovery rather than destroying them.
func TestWorkspace_Land_RefusesIfBranchChanged(t *testing.T) {
	repo := initGitRepo(t)

	ws, err := NewWorkspace(repo, "temp-branch")
	if err != nil {
		t.Fatalf("NewWorkspace failed: %v", err)
	}

	patch := strings.Replace(replacePatch, "%s", "goodbye", 1)
	if err := ws.Apply(context.Background(), patch); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if _, err := gitOutput(context.Background(), repo, "checkout", "-b", "developer-switched-here"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	if err := ws.Land(context.Background()); err == nil {
		t.Fatal("Land succeeded despite repo's branch having changed since NewWorkspace, want an error")
	}

	if _, err := os.Stat(ws.worktreeDir); err != nil {
		t.Errorf("worktree %q was removed despite Land failing, want it preserved for recovery: %v", ws.worktreeDir, err)
	}
	list, err := gitOutput(context.Background(), repo, "branch", "--list", "temp-branch")
	if err != nil {
		t.Fatalf("branch --list failed: %v", err)
	}
	if list == "" {
		t.Error("branch \"temp-branch\" was deleted despite Land failing, want it preserved for recovery")
	}
	if _, err := os.Stat(ws.patchPath); err != nil {
		t.Errorf("patch file %q was removed despite Land failing, want it preserved for recovery: %v", ws.patchPath, err)
	}
}

// assertNoLeftoverWorktrees fails t if repo has any registered worktree
// beyond its own main one.
func assertNoLeftoverWorktrees(t *testing.T, repo string) {
	t.Helper()
	list, err := gitOutput(context.Background(), repo, "worktree", "list", "--porcelain")
	if err != nil {
		t.Fatalf("worktree list failed: %v", err)
	}
	if n := strings.Count(list, "worktree "); n != 1 {
		t.Errorf("repo has %d registered worktree(s), want exactly 1 (the main one): %s", n, list)
	}
}

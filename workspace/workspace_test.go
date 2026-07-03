package workspace

import (
	"context"
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

	data, err := os.ReadFile(filepath.Join(repo, "greeting.txt"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "hello, world\n" {
		t.Errorf("file contents = %q, want %q", data, "hello, world\n")
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

	if err := wsA.Clean(context.Background()); err != nil {
		t.Fatalf("wsA.Clean failed: %v", err)
	}
	if err := wsB.Clean(context.Background()); err != nil {
		t.Fatalf("wsB.Clean failed: %v", err)
	}
}

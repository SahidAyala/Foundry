package gatherer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"foundry/domain"
)

// newRepo creates a temp repository root containing files, keyed by relative
// path.
func newRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		full := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", name, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return root
}

func gather(t *testing.T, repo, intent string) []string {
	t.Helper()
	got, err := NewNaiveGatherer(repo).Gather(context.Background(), &domain.Intent{Text: intent})
	if err != nil {
		t.Fatalf("Gather failed: %v", err)
	}
	return got
}

func TestGather_ReadsNamedFile(t *testing.T) {
	repo := newRepo(t, map[string]string{"main.go": "package main\n"})

	got := gather(t, repo, "add logging to main.go")

	want := []string{"main.go:\npackage main\n"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Gather = %q, want %q", got, want)
	}
}

func TestGather_MissingFileReportedNotFound(t *testing.T) {
	repo := newRepo(t, nil)

	got := gather(t, repo, "update missing.go")

	want := []string{"missing.go: not found"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Gather = %q, want %q", got, want)
	}
}

func TestGather_MultipleFilesInMentionOrderDeduped(t *testing.T) {
	repo := newRepo(t, map[string]string{
		"cli/cli.go": "package cli\n",
		"main.go":    "package main\n",
	})

	got := gather(t, repo, "sync cli/cli.go with main.go, then gofmt cli/cli.go")

	want := []string{"cli/cli.go:\npackage cli\n", "main.go:\npackage main\n"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Gather = %q, want %q", got, want)
	}
}

func TestGather_TrailingPunctuationTrimmed(t *testing.T) {
	repo := newRepo(t, map[string]string{"main.go": "x"})

	got := gather(t, repo, "add logging to main.go.")

	want := []string{"main.go:\nx"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Gather = %q, want %q", got, want)
	}
}

func TestGather_NoFileNames(t *testing.T) {
	repo := newRepo(t, nil)

	if got := gather(t, repo, "make the tests faster"); len(got) != 0 {
		t.Errorf("Gather = %q, want empty", got)
	}
}

func TestGather_RefusesEscapingPaths(t *testing.T) {
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("s3cret"), 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	repo := newRepo(t, nil)

	got := gather(t, repo, "read ../secret.txt and "+secret)

	for _, entry := range got {
		if strings.Contains(entry, "s3cret") {
			t.Fatalf("Gather leaked content from outside the repository: %q", entry)
		}
		if !strings.Contains(entry, "refused") {
			t.Errorf("entry %q does not mark the path as refused", entry)
		}
	}
	if len(got) != 2 {
		t.Errorf("Gather returned %d entries, want 2 refusals: %q", len(got), got)
	}
}

func TestGather_BoundsTotalOutput(t *testing.T) {
	big := strings.Repeat("a", maxContextBytes)
	repo := newRepo(t, map[string]string{"big.txt": big, "next.txt": "never reached"})

	got := gather(t, repo, "summarize big.txt and next.txt")

	if len(got) != 1 {
		t.Fatalf("Gather returned %d entries, want 1 (budget exhausted): %q", len(got), got)
	}
	if !strings.HasSuffix(got[0], "[truncated]") {
		t.Errorf("oversized entry not marked truncated: %q", got[0][len(got[0])-40:])
	}
	if len(got[0]) > maxContextBytes+len("\n[truncated]") {
		t.Errorf("entry length %d exceeds bound", len(got[0]))
	}
}

// TestReadBounded_NeverReadsPastLimitPlusOne covers the concrete gap a bare
// os.ReadFile call had: it loads a file's entire content regardless of
// size, before any budget check ever runs. A file much larger than the
// caller's limit (here, 50x) must still only ever produce limit+1 bytes —
// readBounded's contract — not a copy of the whole file in memory.
func TestReadBounded_NeverReadsPastLimitPlusOne(t *testing.T) {
	const limit = 1024
	path := filepath.Join(t.TempDir(), "huge.bin")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", limit*50)), 0o644); err != nil {
		t.Fatalf("write huge file: %v", err)
	}

	got, err := readBounded(path, limit)
	if err != nil {
		t.Fatalf("readBounded failed: %v", err)
	}
	if len(got) != limit+1 {
		t.Errorf("readBounded returned %d bytes, want exactly %d (limit+1)", len(got), limit+1)
	}
}

// TestReadBounded_ReturnsFullContentWhenUnderLimit confirms the common
// case — a file smaller than limit — is returned unchanged, byte for
// byte, not truncated to limit+1.
func TestReadBounded_ReturnsFullContentWhenUnderLimit(t *testing.T) {
	const content = "small file, well under any limit"
	path := filepath.Join(t.TempDir(), "small.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write small file: %v", err)
	}

	got, err := readBounded(path, maxContextBytes)
	if err != nil {
		t.Fatalf("readBounded failed: %v", err)
	}
	if string(got) != content {
		t.Errorf("readBounded = %q, want %q", got, content)
	}
}

func TestReadBounded_MissingFileReturnsError(t *testing.T) {
	if _, err := readBounded(filepath.Join(t.TempDir(), "missing"), maxContextBytes); err == nil {
		t.Fatal("readBounded of a missing file returned nil error")
	}
}

func TestGather_IncludesReadme(t *testing.T) {
	repo := newRepo(t, map[string]string{
		"main.go":   "package main\n",
		"README.md": "# Project\n",
	})

	got := gather(t, repo, "add logging to main.go")

	want := []string{"main.go:\npackage main\n", "README.md:\n# Project\n"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Gather = %q, want %q", got, want)
	}
}

func TestGather_IncludesNearbyFilesByPriority(t *testing.T) {
	repo := newRepo(t, map[string]string{
		"pkg/a.go":        "package pkg // a\n",
		"pkg/b.go":        "package pkg // b\n",
		"pkg/config.yaml": "key: value\n",
		"pkg/NOTES.md":    "notes\n",
	})

	got := gather(t, repo, "update pkg/a.go")

	// The named file first, then neighbors by priority: config, docs, code.
	want := []string{
		"pkg/a.go:\npackage pkg // a\n",
		"pkg/config.yaml:\nkey: value\n",
		"pkg/NOTES.md:\nnotes\n",
		"pkg/b.go:\npackage pkg // b\n",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Gather = %q, want %q", got, want)
	}
}

func TestGather_ReadmeNamedInIntentNotDuplicated(t *testing.T) {
	repo := newRepo(t, map[string]string{"README.md": "# Project\n"})

	got := gather(t, repo, "improve README.md")

	want := []string{"README.md:\n# Project\n"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Gather = %q, want %q", got, want)
	}
}

func TestGather_SupplementaryRespectsBudget(t *testing.T) {
	repo := newRepo(t, map[string]string{
		"a.txt":    "tiny",
		"huge.txt": strings.Repeat("x", 2*maxContextBytes),
	})

	got := gather(t, repo, "summarize a.txt")

	if len(got) != 2 {
		t.Fatalf("Gather returned %d entries, want 2: named + truncated neighbor", len(got))
	}
	if !strings.HasSuffix(got[1], "[truncated]") {
		t.Errorf("oversized supplementary entry not truncated: %q", got[1][len(got[1])-40:])
	}
	total := len(got[0]) + len(got[1])
	if total > maxContextBytes+len("\n[truncated]") {
		t.Errorf("total gathered %d exceeds bound", total)
	}
}

func TestGather_Deterministic(t *testing.T) {
	repo := newRepo(t, map[string]string{"main.go": "package main\n"})
	intent := "add logging to main.go and missing.go"

	first := gather(t, repo, intent)
	for i := 0; i < 5; i++ {
		if got := gather(t, repo, intent); !reflect.DeepEqual(got, first) {
			t.Fatalf("Gather not deterministic: %q != %q", got, first)
		}
	}
}

func TestGather_CancelledContext(t *testing.T) {
	repo := newRepo(t, map[string]string{"main.go": "x"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := NewNaiveGatherer(repo).Gather(ctx, &domain.Intent{Text: "read main.go"}); err == nil {
		t.Fatal("Gather returned nil error with cancelled context")
	}
}

// TestGather_FallsBackToIdentifierWhenNoFileNamed is the exact shape of the
// interview demo script: "rename User to Account" names an entity, not a
// file, so the naive filename extraction alone finds nothing to gather.
// other/widget.go sits in a different directory so the assertion isolates
// the identifier match from the pre-existing nearby-files supplement.
func TestGather_FallsBackToIdentifierWhenNoFileNamed(t *testing.T) {
	repo := newRepo(t, map[string]string{
		"user.go":         "package demo\n\ntype User struct{}\n",
		"other/widget.go": "package other\n\ntype Widget struct{}\n",
	})

	got := gather(t, repo, "rename User to Account")

	want := []string{"user.go:\npackage demo\n\ntype User struct{}\n"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Gather = %q, want %q", got, want)
	}
}

// TestGather_IdentifierFallbackSkippedWhenNamedFileResolves guards against
// the fallback firing when the naive extraction already found something. It
// puts the identifier-matching file in a directory the named file's
// directory-neighbor supplement would never reach, so if the fallback
// incorrectly ran anyway, this file — and only this file — would appear.
func TestGather_IdentifierFallbackSkippedWhenNamedFileResolves(t *testing.T) {
	repo := newRepo(t, map[string]string{
		"user.go":        "package demo\n\ntype User struct{}\n",
		"pkg/account.go": "package pkg\n\ntype Account struct{}\n",
	})

	got := gather(t, repo, "rename the User struct in user.go to Account")

	want := []string{"user.go:\npackage demo\n\ntype User struct{}\n"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Gather = %q, want %q (identifier fallback must not also run)", got, want)
	}
}

// TestGather_IdentifierFallbackCapsMatches guards against a common
// identifier flooding the gathered context with every file that mentions
// it. Each candidate lives in its own directory so the pre-existing nearby-
// files supplement (which uncappedly lists a whole directory) contributes
// nothing extra — this isolates the fallback's own cap.
func TestGather_IdentifierFallbackCapsMatches(t *testing.T) {
	files := map[string]string{}
	for i := 0; i < maxIdentifierMatches+5; i++ {
		files[fmt.Sprintf("d%02d/file.go", i)] = "package demo\n\ntype User struct{}\n"
	}
	repo := newRepo(t, files)

	got := gather(t, repo, "rename User to Account")

	if len(got) != maxIdentifierMatches {
		t.Errorf("Gather returned %d entries, want the capped %d: %q", len(got), maxIdentifierMatches, got)
	}
}

// TestGather_IdentifierFallbackNoMatchesStillEmpty covers an Intent with no
// resolvable file and no identifier any file mentions.
func TestGather_IdentifierFallbackNoMatchesStillEmpty(t *testing.T) {
	repo := newRepo(t, map[string]string{"user.go": "package demo\n"})

	got := gather(t, repo, "make the tests faster")

	if len(got) != 0 {
		t.Errorf("Gather = %q, want empty", got)
	}
}

func TestExtractIdentifiers(t *testing.T) {
	for _, tc := range []struct {
		text string
		want []string
	}{
		{"rename User to Account", []string{"User", "Account"}},
		{"make the tests faster", nil},
		{"dedupe User User", []string{"User"}},
		{"Ok", nil}, // shorter than the 3-character minimum
	} {
		if got := extractIdentifiers(tc.text); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("extractIdentifiers(%q) = %q, want %q", tc.text, got, tc.want)
		}
	}
}

func TestExtractFileNames(t *testing.T) {
	for _, tc := range []struct {
		text string
		want []string
	}{
		{"add logging to main.go", []string{"main.go"}},
		{"fix `cli/cli.go` and \"main.go\"", []string{"cli/cli.go", "main.go"}},
		{"no files here", nil},
		{"trailing sentence main.go.", []string{"main.go"}},
		{"dedupe main.go main.go", []string{"main.go"}},
	} {
		if got := extractFileNames(tc.text); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("extractFileNames(%q) = %q, want %q", tc.text, got, tc.want)
		}
	}
}

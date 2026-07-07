package gatherer

import (
	"context"
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

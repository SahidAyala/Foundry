package cli

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

// withFakePager swaps pagerRunner for a fake that records its arguments
// instead of launching a real subprocess, restoring the original on
// cleanup — so a test can force maybePage's paging branch without a real
// terminal or a real `less`.
func withFakePager(t *testing.T, fn func(name string, args []string, stdin string, out io.Writer) error) {
	t.Helper()
	original := pagerRunner
	pagerRunner = fn
	t.Cleanup(func() { pagerRunner = original })
}

func manyLines(n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = "line"
	}
	return strings.Join(lines, "\n")
}

func TestMaybePage_ShortContentNeverInvokesPager(t *testing.T) {
	called := false
	withFakePager(t, func(name string, args []string, stdin string, out io.Writer) error {
		called = true
		return nil
	})

	var out bytes.Buffer
	content := manyLines(pagerThresholdLines)
	if err := maybePage(&out, true, content); err != nil {
		t.Fatalf("maybePage failed: %v", err)
	}
	if called {
		t.Error("pagerRunner was called for content at the threshold, want direct write")
	}
	if out.String() != content {
		t.Errorf("out = %q, want %q", out.String(), content)
	}
}

func TestMaybePage_PageFalseNeverInvokesPagerRegardlessOfLength(t *testing.T) {
	called := false
	withFakePager(t, func(name string, args []string, stdin string, out io.Writer) error {
		called = true
		return nil
	})

	var out bytes.Buffer
	content := manyLines(500)
	if err := maybePage(&out, false, content); err != nil {
		t.Fatalf("maybePage failed: %v", err)
	}
	if called {
		t.Error("pagerRunner was called with page=false, want direct write unconditionally")
	}
	if out.String() != content {
		t.Errorf("out = %q, want %q", out.String(), content)
	}
}

func TestMaybePage_LongContentAndPageTrueInvokesPager(t *testing.T) {
	var gotName string
	var gotArgs []string
	var gotStdin string
	withFakePager(t, func(name string, args []string, stdin string, out io.Writer) error {
		gotName, gotArgs, gotStdin = name, args, stdin
		return nil
	})
	t.Setenv("PAGER", "")

	var out bytes.Buffer
	content := manyLines(pagerThresholdLines + 1)
	if err := maybePage(&out, true, content); err != nil {
		t.Fatalf("maybePage failed: %v", err)
	}
	if gotName != "less" || len(gotArgs) != 1 || gotArgs[0] != "-R" {
		t.Errorf("pager invoked as %q %v, want \"less\" [\"-R\"] (the default)", gotName, gotArgs)
	}
	if gotStdin != content {
		t.Errorf("pager stdin = %q, want %q", gotStdin, content)
	}
	if out.Len() != 0 {
		t.Errorf("out = %q, want empty — content goes to the pager, not directly to out", out.String())
	}
}

func TestMaybePage_RespectsPagerEnvVar(t *testing.T) {
	var gotName string
	var gotArgs []string
	withFakePager(t, func(name string, args []string, stdin string, out io.Writer) error {
		gotName, gotArgs = name, args
		return nil
	})
	t.Setenv("PAGER", "more -c")

	var out bytes.Buffer
	if err := maybePage(&out, true, manyLines(pagerThresholdLines+1)); err != nil {
		t.Fatalf("maybePage failed: %v", err)
	}
	if gotName != "more" || len(gotArgs) != 1 || gotArgs[0] != "-c" {
		t.Errorf("pager invoked as %q %v, want \"more\" [\"-c\"] from $PAGER", gotName, gotArgs)
	}
}

func TestMaybePage_PagerFailureFallsBackToDirectWrite(t *testing.T) {
	withFakePager(t, func(name string, args []string, stdin string, out io.Writer) error {
		return errors.New("exec: \"less\": executable file not found in $PATH")
	})

	var out bytes.Buffer
	content := manyLines(pagerThresholdLines + 1)
	if err := maybePage(&out, true, content); err != nil {
		t.Fatalf("maybePage failed: %v", err)
	}
	if out.String() != content {
		t.Errorf("out = %q, want %q (fallback to direct write when the pager fails)", out.String(), content)
	}
}

func TestCountLines(t *testing.T) {
	cases := []struct {
		s    string
		want int
	}{
		{"", 1},
		{"one line", 1},
		{"line one\nline two", 2},
		{"line one\nline two\n", 2}, // trailing newline is not a further empty line
	}
	for _, c := range cases {
		if got := countLines(c.s); got != c.want {
			t.Errorf("countLines(%q) = %d, want %d", c.s, got, c.want)
		}
	}
}

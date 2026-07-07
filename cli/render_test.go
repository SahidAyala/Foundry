package cli

import (
	"bytes"
	"testing"
)

const renderSample = "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1,2 @@\n context\n-old\n+new"

func TestRenderDiff_PlainIsVerbatim(t *testing.T) {
	if got := renderDiff(renderSample, false); got != renderSample {
		t.Errorf("plain renderDiff altered the patch:\n%q", got)
	}
}

func TestRenderDiff_ColoredGolden(t *testing.T) {
	want := "\x1b[1mdiff --git a/x b/x\x1b[0m\n" +
		"\x1b[1m--- a/x\x1b[0m\n" +
		"\x1b[1m+++ b/x\x1b[0m\n" +
		"\x1b[36m@@ -1 +1,2 @@\x1b[0m\n" +
		" context\n" +
		"\x1b[31m-old\x1b[0m\n" +
		"\x1b[32m+new\x1b[0m"

	if got := renderDiff(renderSample, true); got != want {
		t.Errorf("colored renderDiff golden mismatch:\ngot:\n%q\nwant:\n%q", got, want)
	}
}

func TestRenderVerdict(t *testing.T) {
	for _, tc := range []struct {
		verdict string
		color   bool
		want    string
	}{
		{"pass", false, "✓ pass"},
		{"fail", false, "✗ fail"},
		{"budget-exceeded", false, "✗ budget-exceeded"},
		{"pass", true, "\x1b[32m✓ pass\x1b[0m"},
		{"fail", true, "\x1b[31m✗ fail\x1b[0m"},
	} {
		if got := renderVerdict(tc.verdict, tc.color); got != tc.want {
			t.Errorf("renderVerdict(%q, %v) = %q, want %q", tc.verdict, tc.color, got, tc.want)
		}
	}
}

func TestColorEnabled_FalseForBuffer(t *testing.T) {
	if colorEnabled(&bytes.Buffer{}) {
		t.Error("colorEnabled returned true for a non-terminal writer")
	}
}

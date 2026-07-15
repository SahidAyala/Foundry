package executor

import "testing"

// sampleDiff ends in a newline because git apply rejects a patch whose last
// line is unterminated; ParsePatch guarantees this normalization.
const sampleDiff = `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1 +1,2 @@
 package main
+// added
`

func TestParsePatch_Fenced(t *testing.T) {
	patch, err := ParsePatch("Here is the change:\n```diff\n" + sampleDiff + "```\ntrailing prose")
	if err != nil {
		t.Fatalf("ParsePatch failed: %v", err)
	}
	if patch != sampleDiff {
		t.Errorf("patch = %q, want %q", patch, sampleDiff)
	}
}

func TestParsePatch_RawDiffGit(t *testing.T) {
	patch, err := ParsePatch("prose before\n" + sampleDiff)
	if err != nil {
		t.Fatalf("ParsePatch failed: %v", err)
	}
	if patch != sampleDiff {
		t.Errorf("patch = %q, want %q", patch, sampleDiff)
	}
}

func TestParsePatch_RawMinusMarker(t *testing.T) {
	raw := "--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b\n"
	patch, err := ParsePatch("blah\n" + raw)
	if err != nil {
		t.Fatalf("ParsePatch failed: %v", err)
	}
	if patch != raw {
		t.Errorf("patch = %q, want %q", patch, raw)
	}
}

// git apply rejects a patch whose final line has no newline; ParsePatch must
// terminate the extracted patch regardless of how the model's output ends.
func TestParsePatch_NormalizesTrailingNewline(t *testing.T) {
	unterminated := "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b"
	for name, in := range map[string]string{
		"raw":    unterminated,
		"fenced": "```diff\n" + unterminated + "\n```",
	} {
		patch, err := ParsePatch(in)
		if err != nil {
			t.Fatalf("%s: ParsePatch failed: %v", name, err)
		}
		if len(patch) == 0 || patch[len(patch)-1] != '\n' || (len(patch) > 1 && patch[len(patch)-2] == '\n') {
			t.Errorf("%s: patch does not end in exactly one newline: %q", name, patch)
		}
	}
}

func TestParsePatch_Deterministic(t *testing.T) {
	in := "```diff\n" + sampleDiff + "\n```\n"
	first, err := ParsePatch(in)
	if err != nil {
		t.Fatalf("ParsePatch failed: %v", err)
	}
	for i := 0; i < 5; i++ {
		got, err := ParsePatch(in)
		if err != nil {
			t.Fatalf("ParsePatch failed on repeat: %v", err)
		}
		if got != first {
			t.Fatalf("ParsePatch not deterministic: %q != %q", got, first)
		}
	}
}

func TestParsePatch_Empty(t *testing.T) {
	if _, err := ParsePatch(""); err == nil {
		t.Fatal("ParsePatch returned nil error for empty input")
	}
}

func TestParsePatch_NoDiff(t *testing.T) {
	if _, err := ParsePatch("just some prose, no diff here"); err == nil {
		t.Fatal("ParsePatch returned nil error for prose input")
	}
}

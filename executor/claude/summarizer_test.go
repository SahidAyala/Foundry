package claude

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func newSummarizer(r runner) *Summarizer {
	return &Summarizer{
		workspace:  "/repo",
		executable: "claude",
		runner:     r,
	}
}

func TestSummarize_Success(t *testing.T) {
	r := &fakeRunner{stdout: "TITLE: Fix greeting typo\nBODY:\nReplaces 'hello' with 'goodbye' in greeting.txt."}
	s := newSummarizer(r)

	title, body, err := s.Summarize(context.Background(), "Fix the greeting", "diff --git a/greeting.txt b/greeting.txt")
	if err != nil {
		t.Fatalf("Summarize failed: %v", err)
	}
	if title != "Fix greeting typo" {
		t.Errorf("title = %q, want %q", title, "Fix greeting typo")
	}
	if body != "Replaces 'hello' with 'goodbye' in greeting.txt." {
		t.Errorf("body = %q, want %q", body, "Replaces 'hello' with 'goodbye' in greeting.txt.")
	}
	if r.gotDir != "/repo" {
		t.Errorf("runner dir = %q, want %q", r.gotDir, "/repo")
	}
	if !strings.Contains(r.gotStdin, "Fix the greeting") {
		t.Errorf("prompt missing intent, got:\n%s", r.gotStdin)
	}
}

func TestSummarize_PassesModelFlagWhenSet(t *testing.T) {
	r := &fakeRunner{stdout: "TITLE: x\nBODY:\ny"}
	s := newSummarizer(r)
	s.model = "haiku"

	if _, _, err := s.Summarize(context.Background(), "x", "y"); err != nil {
		t.Fatalf("Summarize failed: %v", err)
	}
	wantArgs := []string{"-p", "--model", "haiku"}
	if len(r.gotArgs) != len(wantArgs) {
		t.Fatalf("runner args = %v, want %v", r.gotArgs, wantArgs)
	}
	for i := range wantArgs {
		if r.gotArgs[i] != wantArgs[i] {
			t.Errorf("runner args = %v, want %v", r.gotArgs, wantArgs)
		}
	}
}

func TestSummarize_NoWorkspaceFails(t *testing.T) {
	s := &Summarizer{runner: &fakeRunner{}}
	_, _, err := s.Summarize(context.Background(), "x", "y")
	if err == nil {
		t.Fatal("Summarize with no workspace configured returned nil error")
	}
}

func TestSummarize_ExecutableMissingFails(t *testing.T) {
	s := newSummarizer(&fakeRunner{err: exec.ErrNotFound})
	_, _, err := s.Summarize(context.Background(), "x", "y")
	if err == nil {
		t.Fatal("Summarize with a missing executable returned nil error")
	}
}

func TestSummarize_TruncatesLargePatch(t *testing.T) {
	r := &fakeRunner{stdout: "TITLE: x\nBODY:\ny"}
	s := newSummarizer(r)
	hugePatch := strings.Repeat("a", maxSummarizePatchChars*2)

	if _, _, err := s.Summarize(context.Background(), "x", hugePatch); err != nil {
		t.Fatalf("Summarize failed: %v", err)
	}
	if strings.Count(r.gotStdin, "a") >= len(hugePatch) {
		t.Error("prompt was not truncated for a very large patch")
	}
	if !strings.Contains(r.gotStdin, "(truncated)") {
		t.Error("prompt missing the truncation marker")
	}
}

func TestParseSummary_MissingTitleMarkerFails(t *testing.T) {
	_, _, err := parseSummary("BODY:\nsome body text")
	if err == nil {
		t.Fatal("parseSummary with no TITLE: marker returned nil error")
	}
}

func TestParseSummary_MissingBodyMarkerFails(t *testing.T) {
	_, _, err := parseSummary("TITLE: a title")
	if err == nil {
		t.Fatal("parseSummary with no BODY: marker returned nil error")
	}
}

func TestParseSummary_EmptyTitleFails(t *testing.T) {
	_, _, err := parseSummary("TITLE:   \nBODY:\nsome body text")
	if err == nil {
		t.Fatal("parseSummary with an empty TITLE: returned nil error")
	}
}

func TestParseSummary_EmptyBodyFails(t *testing.T) {
	_, _, err := parseSummary("TITLE: a title\nBODY:   ")
	if err == nil {
		t.Fatal("parseSummary with an empty BODY: returned nil error")
	}
}

func TestParseSummary_MultilineBody(t *testing.T) {
	title, body, err := parseSummary("TITLE: a title\nBODY:\nline one\nline two")
	if err != nil {
		t.Fatalf("parseSummary failed: %v", err)
	}
	if title != "a title" {
		t.Errorf("title = %q, want %q", title, "a title")
	}
	if body != "line one\nline two" {
		t.Errorf("body = %q, want %q", body, "line one\nline two")
	}
}

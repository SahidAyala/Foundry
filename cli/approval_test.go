package cli_test

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"foundry/cli"
	"foundry/domain"
)

func TestPromptForApproval_Yes(t *testing.T) {
	t.Setenv("USER", "alice")
	act := &domain.Act{Patch: "some patch", JudgmentVerdict: "pass"}

	var out bytes.Buffer
	authority, approved, err := cli.PromptForApproval(strings.NewReader("y\n"), &out, act)
	if err != nil {
		t.Fatalf("PromptForApproval failed: %v", err)
	}
	if !approved {
		t.Error("approved = false, want true")
	}
	if authority != "alice" {
		t.Errorf("authority = %q, want %q", authority, "alice")
	}
}

func TestPromptForApproval_YesWord(t *testing.T) {
	t.Setenv("USER", "alice")
	act := &domain.Act{Patch: "p", JudgmentVerdict: "pass"}

	_, approved, err := cli.PromptForApproval(strings.NewReader("YES\n"), &bytes.Buffer{}, act)
	if err != nil {
		t.Fatalf("PromptForApproval failed: %v", err)
	}
	if !approved {
		t.Error("approved = false for \"YES\", want true")
	}
}

func TestPromptForApproval_No(t *testing.T) {
	act := &domain.Act{Patch: "p", JudgmentVerdict: "pass"}

	authority, approved, err := cli.PromptForApproval(strings.NewReader("n\n"), &bytes.Buffer{}, act)
	if err != nil {
		t.Fatalf("PromptForApproval failed: %v", err)
	}
	if approved {
		t.Error("approved = true, want false")
	}
	if authority != "" {
		t.Errorf("authority = %q, want empty on decline", authority)
	}
}

func TestPromptForApproval_EmptyDeclines(t *testing.T) {
	act := &domain.Act{Patch: "p", JudgmentVerdict: "pass"}

	_, approved, err := cli.PromptForApproval(strings.NewReader("\n"), &bytes.Buffer{}, act)
	if err != nil {
		t.Fatalf("PromptForApproval failed: %v", err)
	}
	if approved {
		t.Error("approved = true for empty input, want false")
	}
}

// TestPromptForApproval_SharedBufioReaderSurvivesMultipleCalls proves a
// caller that owns one *bufio.Reader for a whole session — an
// interactive shell issuing more than one approval prompt over the same
// stdin — gets every decision, not just the first: passing a raw
// io.Reader would have bufio.NewReader greedily drain everything
// available on the first call, silently discarding every byte after the
// first line (see the doc comment on PromptForApproval).
func TestPromptForApproval_SharedBufioReaderSurvivesMultipleCalls(t *testing.T) {
	t.Setenv("USER", "alice")
	shared := bufio.NewReader(strings.NewReader("y\nn\ny\n"))
	act := &domain.Act{Patch: "p", JudgmentVerdict: "pass"}

	wantApproved := []bool{true, false, true}
	for i, want := range wantApproved {
		_, approved, err := cli.PromptForApproval(shared, &bytes.Buffer{}, act)
		if err != nil {
			t.Fatalf("call %d: PromptForApproval failed: %v", i, err)
		}
		if approved != want {
			t.Errorf("call %d: approved = %v, want %v", i, approved, want)
		}
	}
}

// TestPromptForApproval_AuthorityFallback forces the whoami fallback by
// clearing USER; the captured Authority must still be non-empty.
func TestPromptForApproval_AuthorityFallback(t *testing.T) {
	t.Setenv("USER", "")
	act := &domain.Act{Patch: "p", JudgmentVerdict: "pass"}

	authority, approved, err := cli.PromptForApproval(strings.NewReader("y\n"), &bytes.Buffer{}, act)
	if err != nil {
		t.Fatalf("PromptForApproval failed: %v", err)
	}
	if !approved {
		t.Fatal("approved = false, want true")
	}
	if authority == "" {
		t.Error("authority is empty; whoami fallback did not capture a user")
	}
}

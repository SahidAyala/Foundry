package cli

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/SahidAyala/Foundry/domain"
)

// A bytes.Buffer is never a character device, so ProgressReporter emits
// plain, uncolored text in tests — matching what a piped or captured
// `foundry do` run produces.

func TestProgressReporter_Gathering(t *testing.T) {
	var out bytes.Buffer
	NewProgressReporter(&out).Gathering()
	if got := out.String(); !strings.Contains(got, "Gathering repository context") {
		t.Errorf("Gathering() = %q, missing expected text", got)
	}
}

func TestProgressReporter_ExecutingFirstAttemptVsRepair(t *testing.T) {
	var first, repair bytes.Buffer
	NewProgressReporter(&first).Executing(1)
	NewProgressReporter(&repair).Executing(2)

	if strings.Contains(first.String(), "repair") {
		t.Errorf("Executing(1) = %q, should not mention repair", first.String())
	}
	if !strings.Contains(repair.String(), "repair the failed attempt (round 2)") {
		t.Errorf("Executing(2) = %q, missing repair round", repair.String())
	}
}

func TestProgressReporter_VerifiedRendersVerdict(t *testing.T) {
	var pass, fail bytes.Buffer
	NewProgressReporter(&pass).Verified(1, &domain.Judgment{Verdict: "pass"})
	NewProgressReporter(&fail).Verified(1, &domain.Judgment{Verdict: "fail", Checked: []string{"go-build: fail\nboom"}})

	if !strings.Contains(pass.String(), "✓ pass") {
		t.Errorf("Verified(pass) = %q, want it to contain %q", pass.String(), "✓ pass")
	}
	if !strings.Contains(fail.String(), "✗ fail") {
		t.Errorf("Verified(fail) = %q, want it to contain %q", fail.String(), "✗ fail")
	}
}

// TestProgressReporter_VerifiedShowsFailureFindings is the point of
// carrying the Judgment into Verified: a demo audience should see *why*
// verification failed, not just that it did.
func TestProgressReporter_VerifiedShowsFailureFindings(t *testing.T) {
	var out bytes.Buffer
	NewProgressReporter(&out).Verified(1, &domain.Judgment{
		Verdict: "fail",
		Checked: []string{"go-build: fail\nuser.go:5: undefined: User"},
	})

	got := out.String()
	if !strings.Contains(got, "undefined: User") {
		t.Errorf("Verified(fail) = %q, want it to show the compiler finding", got)
	}
}

// TestProgressReporter_VerifiedOmitsFindingsOnPass keeps a passing run's
// output to a single line.
func TestProgressReporter_VerifiedOmitsFindingsOnPass(t *testing.T) {
	var out bytes.Buffer
	NewProgressReporter(&out).Verified(1, &domain.Judgment{
		Verdict: "pass",
		Checked: []string{"go-build: pass", "go-test: pass"},
	})

	if lines := strings.Count(strings.TrimRight(out.String(), "\n"), "\n") + 1; lines != 1 {
		t.Errorf("Verified(pass) printed %d lines, want 1:\n%s", lines, out.String())
	}
}

// TestProgressReporter_VerifiedTruncatesLongFindings keeps one verbose
// validator from flooding the live demo terminal.
func TestProgressReporter_VerifiedTruncatesLongFindings(t *testing.T) {
	var findingLines []string
	for i := 0; i < 30; i++ {
		findingLines = append(findingLines, "error line")
	}

	var out bytes.Buffer
	NewProgressReporter(&out).Verified(1, &domain.Judgment{
		Verdict: "fail",
		Checked: []string{strings.Join(findingLines, "\n")},
	})

	got := out.String()
	if strings.Count(got, "error line") != maxFindingLines {
		t.Errorf("printed %d finding lines, want the capped %d", strings.Count(got, "error line"), maxFindingLines)
	}
	if !strings.Contains(got, "more lines") {
		t.Errorf("output missing truncation notice:\n%s", got)
	}
}

func TestProgressReporter_RepairingAndSkipped(t *testing.T) {
	var repairing, skipped, exceeded bytes.Buffer
	NewProgressReporter(&repairing).Repairing("verification failed")
	NewProgressReporter(&skipped).RepairSkipped("budget exceeded: iteration 2 over limit 1")
	NewProgressReporter(&exceeded).BudgetExceeded("budget exceeded: iteration 1 over limit 0")

	if !strings.Contains(repairing.String(), "attempting one bounded repair") {
		t.Errorf("Repairing() = %q, missing expected text", repairing.String())
	}
	if !strings.Contains(skipped.String(), "Repair skipped: budget exceeded") {
		t.Errorf("RepairSkipped() = %q, missing reason", skipped.String())
	}
	if !strings.Contains(exceeded.String(), "Budget exceeded: budget exceeded") {
		t.Errorf("BudgetExceeded() = %q, missing reason", exceeded.String())
	}
}

// TestProgressReporter_RepairingNarratesTheRealCause is the human-facing
// half of engine.Reporter.Repairing carrying a reason: a repair round
// earned by a timed-out Executor must print that timeout, not the
// "Verification failed" this line used to print unconditionally — for
// three consecutive rounds, in the run that motivated this, while the real
// cause appeared nowhere until the final error.
func TestProgressReporter_RepairingNarratesTheRealCause(t *testing.T) {
	var out bytes.Buffer
	NewProgressReporter(&out).Repairing("generate step failed: engine: execute: claude: timed out after 5m0s")

	got := out.String()
	if !strings.Contains(got, "timed out after 5m0s") {
		t.Errorf("Repairing() = %q, want it to carry the real cause", got)
	}
	if strings.Contains(strings.ToLower(got), "verification") {
		t.Errorf("Repairing() = %q, must not blame verification for a generate failure", got)
	}
}

// TestProgressReporter_ExecutingNamesNoVendor guards against the narration
// re-acquiring a hardcoded vendor name: which Executor runs is decided by
// the Router per Step, and is reported by ExecutorStarting from what was
// actually resolved.
func TestProgressReporter_ExecutingNamesNoVendor(t *testing.T) {
	var out bytes.Buffer
	NewProgressReporter(&out).Executing(1)

	if got := strings.ToLower(out.String()); strings.Contains(got, "claude") {
		t.Errorf("Executing(1) = %q, must not name one vendor: any Executor may be resolved", out.String())
	}
}

func TestProgressReporter_StepStartingShowsThePipelineWalk(t *testing.T) {
	var out bytes.Buffer
	NewProgressReporter(&out).StepStarting(1, 2, 5, "verify-tests", "verify")

	got := out.String()
	for _, want := range []string{"2/5", "verify-tests", "verify"} {
		if !strings.Contains(got, want) {
			t.Errorf("StepStarting() = %q, want it to contain %q", got, want)
		}
	}
}

func TestProgressReporter_ExecutorStartingNamesExecutorAndDeadline(t *testing.T) {
	var named, anonymous bytes.Buffer
	NewProgressReporter(&named).ExecutorStarting("implement", "claude-opus-4-6", 5*time.Minute)
	NewProgressReporter(&anonymous).ExecutorStarting("implement", "default executor", 0)

	if got := named.String(); !strings.Contains(got, "claude-opus-4-6") || !strings.Contains(got, "5m0s") {
		t.Errorf("ExecutorStarting() = %q, want the resolved executor and its timeout", got)
	}
	// An Executor advertising no deadline must not be described as having
	// one — a zero timeout is "unknown", never "no time at all".
	if got := anonymous.String(); strings.Contains(got, "timeout") {
		t.Errorf("ExecutorStarting() = %q, want no timeout claim when none is advertised", got)
	}
}

// TestProgressReporter_ExecutorFinishedReportsFailureAtOnce covers the
// signal that was missing entirely: a failed Execute call said nothing at
// the moment it failed, so a multi-round run showed only whichever failure
// happened to be last.
func TestProgressReporter_ExecutorFinishedReportsFailureAtOnce(t *testing.T) {
	var ok, failed bytes.Buffer
	NewProgressReporter(&ok).ExecutorFinished("implement", "opus", 42*time.Second, nil)
	NewProgressReporter(&failed).ExecutorFinished("implement", "opus", 5*time.Minute, errors.New("claude: timed out after 5m0s"))

	if got := ok.String(); !strings.Contains(got, "42s") {
		t.Errorf("ExecutorFinished(nil) = %q, want it to report how long the call took", got)
	}
	got := failed.String()
	if !strings.Contains(got, "timed out after 5m0s") {
		t.Errorf("ExecutorFinished(err) = %q, want it to report the failure", got)
	}
	if !strings.Contains(got, "5m0s") {
		t.Errorf("ExecutorFinished(err) = %q, want it to report the elapsed time", got)
	}
}

// TestProgressReporter_ExecutorFailureIndentsItsDetail keeps a multi-line
// Executor failure (a timeout carrying the last events before the deadline,
// a non-zero exit carrying stderr) reading as one failure with detail,
// rather than as several unrelated top-level narration lines.
func TestProgressReporter_ExecutorFailureIndentsItsDetail(t *testing.T) {
	var out bytes.Buffer
	NewProgressReporter(&out).ExecutorFinished("opus", "opus", 5*time.Minute,
		errors.New("claude: timed out after 5m0s\nlast output:\n  tool Edit routes.go"))

	for _, line := range strings.Split(strings.TrimRight(out.String(), "\n"), "\n")[1:] {
		if !strings.HasPrefix(line, "      ") {
			t.Errorf("continuation line %q is not indented under the failure", line)
		}
	}
}

// TestProgressReporter_HeartbeatShowsElapsedWhileACallIsInFlight is the
// core of this change: during the minutes an Executor runs, the terminal
// must keep proving the run is alive and show how much of the deadline is
// gone. Before the heartbeat existed, a working run and a hung run were
// indistinguishable for that entire window.
func TestProgressReporter_HeartbeatShowsElapsedWhileACallIsInFlight(t *testing.T) {
	out := &syncBuffer{}
	p := NewProgressReporter(out)
	p.interval = 5 * time.Millisecond

	p.ExecutorStarting("implement", "opus", 5*time.Minute)
	waitFor(t, out, "still running")
	p.ExecutorFinished("implement", "opus", time.Second, nil)

	if got := out.String(); !strings.Contains(got, "elapsed of 5m0s") {
		t.Errorf("heartbeat = %q, want it to show elapsed time against the deadline", got)
	}

	// ExecutorFinished stopped the heartbeat: nothing further may appear,
	// or a finished call would keep narrating over whatever runs next.
	settled := out.String()
	time.Sleep(50 * time.Millisecond)
	if got := out.String(); got != settled {
		t.Errorf("heartbeat kept printing after the call ended:\nbefore: %q\nafter:  %q", settled, got)
	}
}

// TestProgressReporter_CloseStopsAHeartbeatNoEventEnded covers the one path
// no event method covers: a Verifier failing with an infrastructure error
// ends the Act from inside the Engine, with no further Reporter event. In
// the interactive session — one long-lived process, one Act per slash
// command — a heartbeat left running would print over the next prompt.
func TestProgressReporter_CloseStopsAHeartbeatNoEventEnded(t *testing.T) {
	out := &syncBuffer{}
	p := NewProgressReporter(out)
	p.interval = 5 * time.Millisecond

	p.Verifying(1)
	waitFor(t, out, "still verifying")
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	settled := out.String()
	time.Sleep(50 * time.Millisecond)
	if got := out.String(); got != settled {
		t.Errorf("heartbeat kept printing after Close:\nbefore: %q\nafter:  %q", settled, got)
	}
	// Idempotent: a caller that closes a Reporter whose heartbeat already
	// stopped must not block or panic.
	if err := p.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// TestProgressReporter_NoHeartbeatWhenDisabled documents the interval
// escape hatch every other test in this file relies on: with no interval,
// output is exactly the lines the events themselves print.
func TestProgressReporter_NoHeartbeatWhenDisabled(t *testing.T) {
	out := &syncBuffer{}
	p := NewProgressReporter(out)
	p.interval = 0

	p.ExecutorStarting("implement", "opus", time.Minute)
	time.Sleep(20 * time.Millisecond)
	p.ExecutorFinished("implement", "opus", time.Second, nil)

	if got := out.String(); strings.Contains(got, "still running") {
		t.Errorf("output = %q, want no heartbeat lines with the interval disabled", got)
	}
}

// syncBuffer is a bytes.Buffer safe for the heartbeat goroutine to write to
// while a test reads it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// waitFor blocks until out contains want, failing the test rather than
// hanging if it never does.
func waitFor(t *testing.T, out *syncBuffer, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(out.String(), want) {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("output never contained %q:\n%s", want, out.String())
}

func TestProgressReporter_NoColorOnNonTerminal(t *testing.T) {
	var out bytes.Buffer
	NewProgressReporter(&out).Gathering()
	if strings.Contains(out.String(), "\x1b[") {
		t.Errorf("output contains ANSI escapes for a non-terminal writer: %q", out.String())
	}
}

// TestProgressReporter_StylesAreNotNoOps guards against the lipgloss v4
// restyling silently degrading to plain text: it checks each style's own
// declared properties (Style.GetBold/GetForeground — pure data, no
// renderer or TTY detection involved) rather than rendered output, since
// lipgloss.Style.Render is itself color-profile-aware and correctly
// prints plain text once its renderer detects no real terminal — exactly
// what `go test` looks like with no tty, and exactly why forcing an
// explicit profile on a scratch renderer still rendered plain in an
// earlier version of this test (lipgloss additionally checks TTY-ness,
// not just the configured profile, before ever emitting an escape).
// ProgressReporter's own p.color gate (TestProgressReporter_NoColorOnNonTerminal)
// and the live pty validation this feature's own commit records already
// cover that the real, end-to-end rendering path works against a genuine
// terminal — this test only guards against a typo leaving one of these
// styles accidentally trivial.
func TestProgressReporter_StylesAreNotNoOps(t *testing.T) {
	unset := lipgloss.NoColor{}
	if !progressActionStyle.GetBold() || progressActionStyle.GetForeground() == unset {
		t.Error("progressActionStyle is not bold/colored")
	}
	if !progressRepairStyle.GetBold() || progressRepairStyle.GetForeground() == unset {
		t.Error("progressRepairStyle is not bold/colored")
	}
	if !progressErrorStyle.GetBold() || progressErrorStyle.GetForeground() == unset {
		t.Error("progressErrorStyle is not bold/colored")
	}
	if progressDimStyle.GetForeground() == unset {
		t.Error("progressDimStyle has no foreground color set")
	}
}

// TestProgressReporter_IntentIsRestatedAtTheTop covers the ask this came
// from: the Intent must not be lost. It used to print only in the summary
// block a successful run ends with, so a failing run never showed it.
func TestProgressReporter_IntentIsRestatedAtTheTop(t *testing.T) {
	var out bytes.Buffer
	NewProgressReporter(&out).IntentDeclared("add a health endpoint\nreturning 200")

	got := out.String()
	if !strings.Contains(got, "add a health endpoint") {
		t.Errorf("output = %q, want the Intent restated", got)
	}
	// A multi-line Intent stays visibly one block, not several top-level
	// narration lines.
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[1], "          ") {
		t.Errorf("output = %q, want continuation lines indented under the first", got)
	}
}

func TestProgressReporter_ContextGatheredShowsTheSizeOfThePrompt(t *testing.T) {
	var out bytes.Buffer
	NewProgressReporter(&out).ContextGathered(42, 98304)

	got := out.String()
	for _, want := range []string{"42", "96 KB"} {
		if !strings.Contains(got, want) {
			t.Errorf("output = %q, want it to contain %q", got, want)
		}
	}
}

func TestHumanBytes(t *testing.T) {
	for _, tc := range []struct {
		n    int
		want string
	}{{512, "512 B"}, {1024, "1 KB"}, {98304, "96 KB"}, {2 << 20, "2.0 MB"}} {
		if got := humanBytes(tc.n); got != tc.want {
			t.Errorf("humanBytes(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

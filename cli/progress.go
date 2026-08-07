package cli

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/SahidAyala/Foundry/domain"
	"github.com/SahidAyala/Foundry/engine"
	"github.com/SahidAyala/Foundry/model"
)

// maxFindingLines bounds how many lines of a failed Judgment's Checked
// findings ProgressReporter prints live, so one verbose validator (a full
// compiler dump) cannot flood the terminal during a demo. The recorded Act
// always carries the findings in full (`foundry show`).
const maxFindingLines = 12

// defaultHeartbeatInterval is how often ProgressReporter reprints an
// elapsed-time line while a single long call — an Executor's Execute, a
// Verifier's Verify — is in flight. An Executor backed by a coding-agent
// CLI routinely runs for minutes against a default 5-minute timeout, and
// until this existed the terminal showed one line and then nothing at all
// for that entire window: a run that was working and a run that had hung
// looked identical, and a run that eventually died on its timeout gave no
// hint beforehand that it was heading there. 30 seconds is chosen to keep
// a 5-minute call to roughly ten lines — frequent enough to prove liveness
// and show how much of the deadline is left, rare enough not to bury the
// narration that carries actual decisions.
const defaultHeartbeatInterval = 30 * time.Second

// Styled via lipgloss (ADR-0012's v4 slice) so the live narration matches
// the interactive session's other chrome — the startup banner and the
// "/" command menu — rather than the raw ANSI-escape-constant approach
// render.go's renderVerdict/renderDiff still use elsewhere. Those are
// deliberately left alone: they're shared by `foundry show`/`replay`
// output with existing byte-exact golden tests, so restyling them is a
// separate, riskier change with no functional benefit — this file's own
// narration lines have no such tests pinning their exact escape
// sequences, making them the safe, contained place to start.
var (
	progressActionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	progressRepairStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))
	progressErrorStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1"))
	progressDimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

// ProgressReporter narrates an Act's lifecycle to out as the Engine runs it:
// gathering, each Execute/Verify round, and repair. It satisfies
// engine.Reporter, which is telemetry-only — a ProgressReporter only
// describes what the Engine already decided, never influences it (I1).
type ProgressReporter struct {
	out   io.Writer
	color bool

	// interval is how often a running heartbeat reprints elapsed time; 0
	// disables the heartbeat entirely (what every test that isn't
	// specifically about the heartbeat wants, so no test depends on
	// wall-clock timing).
	interval time.Duration

	// mu guards both out and beat: the heartbeat writes from its own
	// goroutine, so every write to out goes through p.line under this
	// lock. No method holds mu across a call that writes.
	mu   sync.Mutex
	beat *heartbeat
}

// NewProgressReporter returns a Reporter that writes human-readable progress
// lines to out, colored when out is an interactive terminal.
func NewProgressReporter(out io.Writer) *ProgressReporter {
	return &ProgressReporter{out: out, color: colorEnabled(out), interval: defaultHeartbeatInterval}
}

var (
	_ engine.Reporter         = (*ProgressReporter)(nil)
	_ engine.StepReporter     = (*ProgressReporter)(nil)
	_ engine.ExecutorReporter = (*ProgressReporter)(nil)
	_ engine.FailoverReporter = (*ProgressReporter)(nil)
	_ io.Closer               = (*ProgressReporter)(nil)
)

func (p *ProgressReporter) Gathering() {
	p.stopBeat()
	p.line(progressActionStyle, "→ Gathering repository context...")
}

// Executing names the round without naming a vendor: which Executor is
// actually about to run is reported by ExecutorStarting, from what the
// Router resolved. This line used to say "Asking Claude Code for a patch"
// unconditionally — false for every Act routed to any other Executor, and
// exactly the kind of unverifiable narration that makes a live run harder
// to trust rather than easier.
func (p *ProgressReporter) Executing(iteration int) {
	p.stopBeat()
	label := "Asking the Executor for a patch..."
	if iteration > 1 {
		label = fmt.Sprintf("Asking the Executor to repair the failed attempt (round %d)...", iteration)
	}
	p.line(progressActionStyle, "→ "+label)
}

// StepStarting narrates the Pipeline walk itself, so a run shows which of
// its declared Steps it is on rather than only the generate/verify events.
func (p *ProgressReporter) StepStarting(attempt, index, total int, stepID, kind string) {
	p.stopBeat()
	p.line(progressDimStyle, fmt.Sprintf("  step %d/%d · %s (%s)", index, total, stepID, kind))
}

// ExecutorStarting reports what was resolved and how long it may run, then
// starts the elapsed-time heartbeat that covers the call.
func (p *ProgressReporter) ExecutorStarting(stepID, executor string, timeout time.Duration) {
	p.stopBeat()
	line := "  · executor: " + executor
	if timeout > 0 {
		line += fmt.Sprintf(" · timeout %s", timeout)
	}
	p.line(progressDimStyle, line)
	p.startBeat("still running", timeout)
}

// ExecutorFinished stops the heartbeat and reports the outcome of the call
// at the moment it happened — in particular a failure, which previously
// stayed invisible until it resurfaced (or was replaced by a later round's
// failure) in the Act's final error.
func (p *ProgressReporter) ExecutorFinished(stepID, executor string, elapsed time.Duration, err error) {
	p.stopBeat()
	if err != nil {
		// An Executor's failure is routinely multi-line — a timeout carries
		// the last events before the deadline, a non-zero exit carries
		// stderr — so continuation lines are indented under the first
		// rather than printed flush left, where they read as separate
		// top-level narration.
		first, rest := splitFirstLine(err.Error())
		p.line(progressErrorStyle, fmt.Sprintf("  ✗ %s failed after %s: %s", executor, roundDuration(elapsed), first))
		for _, line := range rest {
			p.line(progressDimStyle, "      "+line)
		}
		return
	}
	p.line(progressDimStyle, fmt.Sprintf("  · %s responded in %s", executor, roundDuration(elapsed)))
}

// ExecutorLine renders one line of an Executor's own live output (the tools
// a coding-agent CLI is running, the turns it is taking) while a call is in
// flight. Indented and dimmed one level deeper than Foundry's own
// narration, because it is the Executor talking, not the Engine — and the
// distinction matters: only Foundry's own lines describe decisions the
// Engine made.
//
// It deliberately does not stop the heartbeat: these lines arrive
// throughout a call that is still running, and the elapsed-time line
// remains the only thing that says how much of the deadline is left.
func (p *ProgressReporter) ExecutorLine(s string) {
	p.line(progressDimStyle, "    │ "+s)
}

// ModelFailover narrates automatic model failover. The Engine has emitted
// this event since ADR-0013's sixth increment, but no Reporter in this
// repository implemented FailoverReporter, so a switch between models was
// never actually shown to anyone.
func (p *ProgressReporter) ModelFailover(stepID, from, to string, class model.FailureClass, cause error) {
	p.stopBeat()
	p.line(progressRepairStyle, fmt.Sprintf("↻ %s failed (%s: %s) — switching to %s...", from, class, cause, to))
}

// Executed is intentionally silent: ADR-0011's actual-cost signal is
// reported Evidence for `foundry show`/FOUNDRY_LOG, not live human
// narration — ProgressReporter's own progress lines are unchanged.
func (p *ProgressReporter) Executed(iteration int, actualCostUSD *float64) {}

// Verifying starts a heartbeat too: a Verifier runs a real build and test
// suite, which on a large repository is its own multi-minute silence.
func (p *ProgressReporter) Verifying(iteration int) {
	p.stopBeat()
	p.line(progressActionStyle, "→ Verifying the proposed patch...")
	p.startBeat("still verifying", 0)
}

func (p *ProgressReporter) Verified(iteration int, judgment *domain.Judgment) {
	p.stopBeat()
	p.mu.Lock()
	fmt.Fprintf(p.out, "  %s\n", renderVerdict(judgment.Verdict, p.color))
	p.mu.Unlock()
	if judgment.Verdict == "pass" {
		return
	}

	lines := findingLines(judgment.Checked)
	shown, remaining := lines, 0
	if len(lines) > maxFindingLines {
		shown, remaining = lines[:maxFindingLines], len(lines)-maxFindingLines
	}
	for _, line := range shown {
		fmt.Fprintf(p.out, "    %s\n", line)
	}
	if remaining > 0 {
		p.line(progressDimStyle, fmt.Sprintf("    ... (%d more lines; see `foundry show` for the full findings)", remaining))
	}
}

// findingLines flattens a Judgment's Checked entries into individual lines,
// in order, for a compact live rendering.
func findingLines(checked []string) []string {
	var lines []string
	for _, c := range checked {
		lines = append(lines, strings.Split(strings.TrimRight(c, "\n"), "\n")...)
	}
	return lines
}

// Repairing states the actual cause of the repair round. It used to print
// "Verification failed" for every round, including the rounds a failed
// generate Step earned, where no verification had run at all — see
// engine.Reporter.Repairing.
func (p *ProgressReporter) Repairing(reason string) {
	p.stopBeat()
	p.line(progressRepairStyle, "↻ "+reason+" — attempting one bounded repair...")
}

func (p *ProgressReporter) RepairSkipped(reason string) {
	p.stopBeat()
	p.line(progressErrorStyle, "✗ Repair skipped: "+reason)
}

func (p *ProgressReporter) BudgetExceeded(reason string) {
	p.stopBeat()
	p.line(progressErrorStyle, "✗ Budget exceeded: "+reason)
}

// Close stops any heartbeat still running and waits for its goroutine to
// exit. Every event method already stops the current heartbeat, so this
// matters for the one case none of them cover: a call that ends the whole
// run without a further event — a Verifier returning an infrastructure
// error, which ends the Act from inside runSteps. In a long-lived process
// (the interactive session's REPL, which reuses one Reporter per Act) a
// heartbeat left running would print elapsed-time lines over the next
// prompt; in a one-shot `foundry do` it would merely outlive its purpose.
// Close is idempotent and always returns nil — the io.Closer signature
// exists so engine.MultiReporter can fan Close out without knowing which
// concrete Reporters hold anything open.
func (p *ProgressReporter) Close() error {
	p.stopBeat()
	return nil
}

// heartbeat is one in-flight elapsed-time ticker: the goroutine printing
// the lines, plus the channels that stop it and confirm it has stopped.
type heartbeat struct {
	stop chan struct{}
	done chan struct{}
}

// startBeat begins reprinting "<what> — 1m30s elapsed" every p.interval
// until stopBeat. timeout, when non-zero, is included so the line shows how
// much of the Executor's own deadline has been consumed — the difference
// between "this is taking a while" and "this is 20 seconds from dying on
// its timeout". A zero p.interval disables the heartbeat entirely.
func (p *ProgressReporter) startBeat(what string, timeout time.Duration) {
	if p.interval <= 0 {
		return
	}

	hb := &heartbeat{stop: make(chan struct{}), done: make(chan struct{})}
	p.mu.Lock()
	p.beat = hb
	p.mu.Unlock()

	go func() {
		defer close(hb.done)
		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()
		started := time.Now()
		for {
			select {
			case <-hb.stop:
				return
			case <-ticker.C:
				elapsed := roundDuration(time.Since(started))
				s := fmt.Sprintf("  · %s — %s elapsed", what, elapsed)
				if timeout > 0 {
					s = fmt.Sprintf("  · %s — %s elapsed of %s", what, elapsed, timeout)
				}
				p.beatLine(hb, s)
			}
		}
	}()
}

// stopBeat ends the current heartbeat, if any, and waits for its goroutine
// to exit before returning — so the next line a caller prints can never be
// preceded by a stale elapsed-time line from the call that just ended. The
// wait happens after mu is released: the goroutine may be blocked on mu
// inside beatLine, and holding it here would deadlock.
func (p *ProgressReporter) stopBeat() {
	p.mu.Lock()
	hb := p.beat
	p.beat = nil
	p.mu.Unlock()

	if hb == nil {
		return
	}
	close(hb.stop)
	<-hb.done
}

// beatLine writes one heartbeat line, but only while hb is still the
// current heartbeat. A goroutine already blocked on mu when stopBeat ran
// would otherwise print its line *after* the line that superseded it.
func (p *ProgressReporter) beatLine(hb *heartbeat, s string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.beat != hb {
		return
	}
	p.write(progressDimStyle, s)
}

// splitFirstLine separates s's first line from the rest, with trailing
// blank lines dropped — the shape needed to print a multi-line message as
// one narration line plus indented continuation.
func splitFirstLine(s string) (string, []string) {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	return lines[0], lines[1:]
}

// roundDuration renders d at whole-second precision — the resolution a
// human reading progress lines cares about, and stable enough for a test
// to assert on.
func roundDuration(d time.Duration) time.Duration {
	return d.Round(time.Second)
}

// line writes s to out, styled with style when color is enabled.
func (p *ProgressReporter) line(style lipgloss.Style, s string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.write(style, s)
}

// write is line's body without the lock, for the callers that already hold
// mu. Every write to p.out goes through here or line, so the heartbeat
// goroutine can never interleave mid-line with the main flow.
func (p *ProgressReporter) write(style lipgloss.Style, s string) {
	if p.color {
		s = style.Render(s)
	}
	fmt.Fprintln(p.out, s)
}

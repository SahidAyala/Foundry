package claude

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/SahidAyala/Foundry/domain"
)

// fakeStreamRunner is a runner that also streams: it feeds canned
// stream-json lines to the caller's callback, the way Claude Code's own
// `--output-format stream-json` output arrives line by line.
type fakeStreamRunner struct {
	lines  []string
	stderr string
	err    error

	gotArgs []string
}

func (f *fakeStreamRunner) Run(ctx context.Context, dir, name string, args []string, stdin string) (string, string, error) {
	f.gotArgs = args
	return strings.Join(f.lines, "\n"), f.stderr, f.err
}

func (f *fakeStreamRunner) RunStream(ctx context.Context, dir, name string, args []string, stdin string, onLine func(string)) (string, string, error) {
	f.gotArgs = args
	var full strings.Builder
	for _, line := range f.lines {
		full.WriteString(line + "\n")
		if onLine != nil {
			onLine(line)
		}
	}
	return full.String(), f.stderr, f.err
}

// resultEvent renders a terminal result event carrying text as the CLI's
// answer.
func resultEvent(text string) string {
	return `{"type":"result","subtype":"success","num_turns":3,"total_cost_usd":0.0731,"result":` + quote(text) + `}`
}

// quote JSON-encodes a string without pulling encoding/json into every
// caller.
func quote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return `"` + s + `"`
}

func TestExecute_StreamingNarratesAndTakesThePatchFromTheResultEvent(t *testing.T) {
	r := &fakeStreamRunner{lines: []string{
		`{"type":"system","subtype":"init","model":"claude-opus-4-6"}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"main.go"}}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"I will add the comment.\nsecond line"}]}}`,
		resultEvent("```diff\n" + sampleDiff + "```"),
	}}

	var narrated []string
	e := newExecutor(r)
	e.SetProgress(func(s string) { narrated = append(narrated, s) })

	outcome, err := e.Execute(context.Background(), &domain.Intent{Text: "add a comment"}, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if outcome.Patch != sampleDiff {
		t.Errorf("Patch = %q, want the diff from the result event (%q)", outcome.Patch, sampleDiff)
	}

	// The invocation must switch into event mode, or there is no stream to
	// read: --verbose is mandatory alongside stream-json for a -p run.
	joinedArgs := strings.Join(r.gotArgs, " ")
	for _, want := range []string{"-p", "--output-format stream-json", "--verbose"} {
		if !strings.Contains(joinedArgs, want) {
			t.Errorf("args = %v, want them to contain %q", r.gotArgs, want)
		}
	}

	joined := strings.Join(narrated, "|")
	for _, want := range []string{"claude-opus-4-6", "tool Read main.go", "I will add the comment.", "3 turn(s)", "$0.0731"} {
		if !strings.Contains(joined, want) {
			t.Errorf("narration = %v, want it to include %q", narrated, want)
		}
	}
	// Only the first line of a long assistant turn is narrated: an
	// assistant turn can carry an entire patch as its text.
	if strings.Contains(joined, "second line") {
		t.Errorf("narration = %v, want each turn summarized to one line", narrated)
	}
}

// TestExecute_StreamingReportsTheCallsOwnCost covers the one piece of
// evidence only the streaming path can obtain: the CLI reports what it
// charged in its result event (ADR-0011's actual cost), where a buffered
// call has nothing to read it from.
func TestExecute_StreamingReportsTheCallsOwnCost(t *testing.T) {
	r := &fakeStreamRunner{lines: []string{resultEvent(sampleDiff)}}
	e := newExecutor(r)
	e.SetProgress(func(string) {})

	outcome, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if outcome.ActualCostUSD == nil {
		t.Fatal("ActualCostUSD = nil, want the cost the CLI reported")
	}
	if *outcome.ActualCostUSD != 0.0731 {
		t.Errorf("ActualCostUSD = %v, want 0.0731", *outcome.ActualCostUSD)
	}
}

// TestExecute_NoSinkKeepsTheBufferedInvocation is the guarantee that makes
// streaming safe to add: with no progress sink installed, the flags and the
// output parsing are exactly what they were before stream.go existed.
func TestExecute_NoSinkKeepsTheBufferedInvocation(t *testing.T) {
	r := &fakeStreamRunner{lines: []string{"```diff", sampleDiff + "```"}}
	e := newExecutor(r)

	outcome, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if outcome.Patch != sampleDiff {
		t.Errorf("Patch = %q, want %q", outcome.Patch, sampleDiff)
	}
	if strings.Contains(strings.Join(r.gotArgs, " "), "stream-json") {
		t.Errorf("args = %v, want no streaming flags without a progress sink", r.gotArgs)
	}
}

// TestExecute_SinkWithANonStreamingRunnerStillRuns covers the mixed case:
// a runner predating stream.go (every existing test fake) cannot stream, so
// Execute keeps the buffered path rather than losing the call.
func TestExecute_SinkWithANonStreamingRunnerStillRuns(t *testing.T) {
	r := &fakeRunner{stdout: sampleDiff}
	e := newExecutor(r)
	e.SetProgress(func(string) { t.Error("narrated through a runner that cannot stream") })

	outcome, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if outcome.Patch != sampleDiff {
		t.Errorf("Patch = %q, want %q", outcome.Patch, sampleDiff)
	}
	if strings.Contains(strings.Join(r.gotArgs, " "), "stream-json") {
		t.Errorf("args = %v, want no streaming flags when the runner cannot stream", r.gotArgs)
	}
}

func TestExecute_StreamEndingWithoutAResultIsAnError(t *testing.T) {
	r := &fakeStreamRunner{lines: []string{
		`{"type":"system","subtype":"init","model":"claude-opus-4-6"}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"working"}]}}`,
	}}
	e := newExecutor(r)
	e.SetProgress(func(string) {})

	_, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil)
	if err == nil {
		t.Fatal("Execute with a stream that never produced a result returned nil error")
	}
	if !strings.Contains(err.Error(), "without a result") {
		t.Errorf("error = %q, want it to name the missing result event", err)
	}
}

func TestExecute_StreamResultReportingAnErrorIsAnError(t *testing.T) {
	r := &fakeStreamRunner{lines: []string{
		`{"type":"result","subtype":"error","is_error":true,"result":"credit balance is too low"}`,
	}}
	e := newExecutor(r)
	e.SetProgress(func(string) {})

	_, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil)
	if err == nil {
		t.Fatal("Execute with an error result returned nil error")
	}
	if !strings.Contains(err.Error(), "credit balance is too low") {
		t.Errorf("error = %q, want it to carry what the CLI reported", err)
	}
}

// TestExecute_MalformedStreamLinesAreIgnored keeps an Act from failing over
// output the CLI prints alongside its event stream (an update notice, a
// warning): a line that isn't a JSON event is narrated as nothing, not
// treated as a failure.
func TestExecute_MalformedStreamLinesAreIgnored(t *testing.T) {
	r := &fakeStreamRunner{lines: []string{
		"npm notice: a new version is available",
		"",
		resultEvent(sampleDiff),
	}}
	e := newExecutor(r)
	e.SetProgress(func(string) {})

	if _, err := e.Execute(context.Background(), &domain.Intent{Text: "x"}, nil); err != nil {
		t.Fatalf("Execute failed on non-JSON output alongside the stream: %v", err)
	}
}

// TestTimeoutError_CarriesWhatTheCallWasDoing is the diagnostic that was
// missing from the failure that motivated this work: three consecutive
// 5-minute timeouts reported only "claude: timed out after 5m0s", with
// everything the CLI had done buffered and discarded.
func TestTimeoutError_CarriesWhatTheCallWasDoing(t *testing.T) {
	reader := &streamReader{}
	reader.narrate("started · model claude-opus-4-6")
	reader.narrate("tool Read main.go")

	got := timeoutError(5*time.Minute, reader, "", "").Error()
	if !strings.Contains(got, "5m0s") {
		t.Errorf("error = %q, want the deadline named", got)
	}
	if !strings.Contains(got, "tool Read main.go") {
		t.Errorf("error = %q, want the last events before the deadline", got)
	}
}

// TestTimeoutError_FallsBackToRawOutput covers a non-streaming call: there
// is no narration, so the tail of whatever the CLI wrote stands in.
func TestTimeoutError_FallsBackToRawOutput(t *testing.T) {
	got := timeoutError(time.Minute, nil, "line one\nline two\n", "a warning").Error()
	if !strings.Contains(got, "line two") {
		t.Errorf("error = %q, want the tail of stdout", got)
	}
	if !strings.Contains(got, "a warning") {
		t.Errorf("error = %q, want stderr included", got)
	}
}

// TestTimeoutError_NoOutputAtAllSuggestsWhatToCheck keeps the genuinely
// silent case actionable rather than merely honest.
func TestTimeoutError_NoOutputAtAllSuggestsWhatToCheck(t *testing.T) {
	got := timeoutError(5*time.Minute, nil, "", "").Error()
	for _, want := range []string{"no output at all", "authentication", "timeout_seconds"} {
		if !strings.Contains(got, want) {
			t.Errorf("error = %q, want it to mention %q", got, want)
		}
	}
}

// TestLastLines_KeepsOnlyTheTrailingNonEmptyLines pins the bound that keeps
// a diagnostic from becoming a transcript.
func TestLastLines_KeepsOnlyTheTrailingNonEmptyLines(t *testing.T) {
	got := lastLines("a\n\nb\nc\nd\n", 2)
	if got != "c\nd" {
		t.Errorf("lastLines = %q, want %q", got, "c\nd")
	}
}

// TestStreamReader_RetainsOnlyTheMostRecentNarration bounds what a timeout
// diagnostic carries even for a call that ran for minutes.
func TestStreamReader_RetainsOnlyTheMostRecentNarration(t *testing.T) {
	reader := &streamReader{}
	for i := 0; i < maxRecentNarration*3; i++ {
		reader.narrate("event")
	}
	if len(reader.recent) != maxRecentNarration {
		t.Errorf("retained %d narration lines, want the capped %d", len(reader.recent), maxRecentNarration)
	}
}

// TestExecRunner_RunStreamDeliversLinesAndReturnsFullOutput exercises the
// real subprocess path — the one piece the fake runner above cannot cover:
// that stdout is consumed line by line as it arrives and still returned in
// full for the error diagnostics.
func TestExecRunner_RunStreamDeliversLinesAndReturnsFullOutput(t *testing.T) {
	var got []string
	stdout, _, err := execRunner{}.RunStream(context.Background(), ".", "sh",
		[]string{"-c", "echo one; echo two"}, "", func(s string) { got = append(got, s) })
	if err != nil {
		t.Fatalf("RunStream failed: %v", err)
	}
	if strings.Join(got, "|") != "one|two" {
		t.Errorf("streamed lines = %v, want [one two]", got)
	}
	if stdout != "one\ntwo\n" {
		t.Errorf("stdout = %q, want the full output too", stdout)
	}
}

// TestExecRunner_RunStreamReportsTheSubprocessFailure keeps a non-zero exit
// reported as the error, with whatever was streamed before it still
// returned.
func TestExecRunner_RunStreamReportsTheSubprocessFailure(t *testing.T) {
	stdout, stderr, err := execRunner{}.RunStream(context.Background(), ".", "sh",
		[]string{"-c", "echo partial; echo boom 1>&2; exit 3"}, "", nil)
	if err == nil {
		t.Fatal("RunStream with a failing subprocess returned nil error")
	}
	if !strings.Contains(stdout, "partial") {
		t.Errorf("stdout = %q, want what arrived before the failure", stdout)
	}
	if !strings.Contains(stderr, "boom") {
		t.Errorf("stderr = %q, want the subprocess's stderr", stderr)
	}
}

package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Streaming exists because a Claude Code call is opaque for as long as it
// runs — minutes, against a default 5-minute deadline — and everything it
// did during that window used to be discarded unless the process happened
// to exit non-zero: execRunner.Run buffers stdout and stderr and Execute
// returns them only inside an error. A run that timed out therefore reported
// nothing at all about what the CLI had actually been doing.
//
// Claude Code's own headless streaming mode (`--output-format stream-json
// --verbose`, documented for `-p` runs) emits one JSON event per line as it
// works: an init event naming the resolved model, an event per assistant
// turn (including each tool it invokes), and a final result event carrying
// the answer text. Foundry reads that stream, narrates a one-line summary
// per event through the sink SetProgress installed, and takes the final
// patch from the result event instead of from raw stdout.
//
// This mode is opt-in per Executor (SetProgress). With no sink installed,
// Execute passes exactly the flags it always did and parses raw stdout — so
// the default path is unchanged, and the JSON event shape below is only
// load-bearing for a caller that asked for streaming.

// streamJSONArgs are the flags that switch Claude Code into line-delimited
// event output. --verbose is required alongside stream-json for a `-p` run;
// without it the CLI rejects the combination.
var streamJSONArgs = []string{"--output-format", "stream-json", "--verbose"}

// maxStreamLineBytes bounds one JSON event. Events embedding a tool result
// (a whole file read, a long command's output) are far larger than
// bufio.Scanner's 64KiB default, which would otherwise abort the scan
// mid-run with ErrTooLong and lose the result event entirely.
const maxStreamLineBytes = 8 << 20

// maxRecentNarration bounds how many narrated lines streamReader retains
// for diagnostics. They exist to describe what a call was doing when it
// died on its deadline — the last few events answer that; the whole
// transcript would bury it.
const maxRecentNarration = 8

// maxNarrationChars truncates one narrated line. An assistant turn can
// carry an entire patch as its text; narration is a liveness and progress
// signal, not a transcript.
const maxNarrationChars = 120

// streamEvent is the subset of Claude Code's stream-json event shape this
// package reads. Every field is optional: an unrecognized or future event
// type decodes into a zero value and is narrated as nothing, so a CLI
// upgrade that adds event types degrades to less narration rather than to a
// failed Act.
type streamEvent struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	Model   string `json:"model"`

	// Result and its siblings appear only on the terminal result event.
	Result       string   `json:"result"`
	IsError      bool     `json:"is_error"`
	NumTurns     int      `json:"num_turns"`
	TotalCostUSD *float64 `json:"total_cost_usd"`

	Message struct {
		Content []struct {
			Type  string         `json:"type"`
			Text  string         `json:"text"`
			Name  string         `json:"name"`
			Input map[string]any `json:"input"`
		} `json:"content"`
	} `json:"message"`
}

// streamReader consumes Claude Code's stream-json output line by line,
// narrating each event through sink and retaining what Execute needs
// afterwards: the final result text, whether the CLI reported the run as an
// error, the cost it charged, and the most recent narration for a
// diagnostic if the call never finishes.
type streamReader struct {
	sink func(string)

	result    string
	sawResult bool
	isError   bool
	costUSD   *float64
	recent    []string
}

// line consumes one raw stream-json line. A line that is not valid JSON is
// not an error: Claude Code may print unstructured output (a warning, an
// update notice) alongside the stream, and dropping it from narration is
// strictly better than failing the Act over it.
func (r *streamReader) line(raw string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}

	var ev streamEvent
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		return
	}

	if ev.Type == "result" {
		r.result, r.sawResult, r.isError = ev.Result, true, ev.IsError
		r.costUSD = ev.TotalCostUSD
	}
	for _, s := range narrate(ev) {
		r.narrate(s)
	}
}

// narrate emits one summary line and retains it for diagnostics.
func (r *streamReader) narrate(s string) {
	if len(r.recent) == maxRecentNarration {
		r.recent = r.recent[1:]
	}
	r.recent = append(r.recent, s)
	if r.sink != nil {
		r.sink(s)
	}
}

// narrate renders the human-readable summary lines for one event — zero
// lines for an event with nothing worth reporting.
func narrate(ev streamEvent) []string {
	switch ev.Type {
	case "system":
		if ev.Subtype == "init" && ev.Model != "" {
			return []string{"started · model " + ev.Model}
		}
	case "assistant":
		var lines []string
		for _, c := range ev.Message.Content {
			switch c.Type {
			case "text":
				if s := firstLine(c.Text); s != "" {
					lines = append(lines, truncate(s, maxNarrationChars))
				}
			case "tool_use":
				lines = append(lines, "tool "+c.Name+toolHint(c.Input))
			}
		}
		return lines
	case "result":
		s := fmt.Sprintf("finished · %d turn(s)", ev.NumTurns)
		if ev.TotalCostUSD != nil {
			s += fmt.Sprintf(" · $%.4f", *ev.TotalCostUSD)
		}
		if ev.IsError {
			s += " · reported an error"
		}
		return []string{s}
	}
	return nil
}

// toolHint renders the one input field that says what a tool call is acting
// on, when the call carries one — enough to follow along ("tool Read
// main.go") without dumping a tool's whole input.
func toolHint(input map[string]any) string {
	for _, key := range []string{"file_path", "path", "pattern", "command", "url", "description"} {
		if v, ok := input[key].(string); ok && v != "" {
			return " " + truncate(firstLine(v), maxNarrationChars)
		}
	}
	return ""
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// streamRunner is the seam for a subprocess whose stdout is consumed line
// by line while it runs, rather than read once at exit. It is separate from
// runner (which Reviewer and Summarizer also use, and which has no reason
// to grow a callback) so those callers and every existing test fake stay
// untouched: Execute uses this only when a caller installed a progress
// sink and the configured runner actually supports streaming.
type streamRunner interface {
	RunStream(ctx context.Context, dir, name string, args []string, stdin string, onLine func(string)) (stdout, stderr string, err error)
}

var _ streamRunner = execRunner{}

// RunStream invokes the subprocess and calls onLine for each complete line
// of stdout as it arrives, while still returning the full stdout and stderr
// the non-streaming Run would have — the caller's error diagnostics
// (executionError, timeoutError) are unchanged by streaming.
//
// onLine runs on RunStream's own goroutine, before Wait returns, so a sink
// that writes to a terminal is already serialized against the caller; it
// must not block indefinitely, or the subprocess's stdout pipe fills and
// the call stalls.
func (execRunner) RunStream(ctx context.Context, dir, name string, args []string, stdin string, onLine func(string)) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)

	var stderr strings.Builder
	cmd.Stderr = &stderr

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", stderr.String(), err
	}
	if err := cmd.Start(); err != nil {
		return "", stderr.String(), err
	}

	var stdout strings.Builder
	scanErr := scanLines(pipe, &stdout, onLine)

	waitErr := cmd.Wait()
	if waitErr != nil {
		// The subprocess's own failure (a non-zero exit, a killed
		// process on deadline) is always the more useful error; a scan
		// error is reported only when the process itself succeeded.
		return stdout.String(), stderr.String(), waitErr
	}
	return stdout.String(), stderr.String(), scanErr
}

// scanLines copies r line by line into full while calling onLine for each,
// returning any read error other than a clean EOF.
func scanLines(r io.Reader, full *strings.Builder, onLine func(string)) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64<<10), maxStreamLineBytes)
	for scanner.Scan() {
		line := scanner.Text()
		full.WriteString(line)
		full.WriteByte('\n')
		if onLine != nil {
			onLine(line)
		}
	}
	return scanner.Err()
}

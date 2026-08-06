package executor

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ParsePatch deterministically extracts a unified diff from a
// text-generating model's raw output — shared by every Executor that asks
// a model to emit a diff (executor/claude, executor/openai): it prefers a
// fenced ```diff block, otherwise takes everything from the first
// unified-diff marker to the end. The result is normalized to end in
// exactly one newline, which `git apply` requires.
//
// It also validates every hunk header's declared line counts against the
// body that follows. A model that miscounts a `@@ -a,b +c,d @@` header
// produces a diff `git apply` calls "corrupt" — but only once it overruns
// into a later, unrelated hunk, so the reported line number is nowhere
// near the actual defect and a bounded repair retry has no useful signal
// to act on. Catching the miscount here, at the hunk that actually caused
// it, turns that into an error engine.Strategy already retries (see
// generateStepFailure in engine/strategy.go).
func ParsePatch(out string) (string, error) {
	if strings.TrimSpace(out) == "" {
		return "", errors.New("executor: empty output; no patch produced")
	}
	patch, ok := fencedDiff(out)
	if !ok {
		patch, ok = rawDiff(out)
	}
	if !ok {
		return "", errors.New("executor: no unified diff found in output")
	}
	patch = strings.TrimRight(patch, "\n") + "\n"
	if err := validateHunks(patch); err != nil {
		return "", err
	}
	return patch, nil
}

// hunkHeaderRe matches a unified-diff hunk header, capturing the declared
// old-side and new-side line counts (each defaults to 1 when the diff tool
// omits a count of exactly 1).
var hunkHeaderRe = regexp.MustCompile(`^@@ -\d+(?:,(\d+))? \+\d+(?:,(\d+))? @@`)

// validateHunks recounts each hunk's actual context/added/removed lines
// and compares them against its header's declared counts.
func validateHunks(patch string) error {
	lines := strings.Split(strings.TrimSuffix(patch, "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		m := hunkHeaderRe.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		wantOld, wantNew := 1, 1
		if m[1] != "" {
			wantOld, _ = strconv.Atoi(m[1])
		}
		if m[2] != "" {
			wantNew, _ = strconv.Atoi(m[2])
		}

		gotOld, gotNew := 0, 0
		j := i + 1
		for ; j < len(lines); j++ {
			line := lines[j]
			if strings.HasPrefix(line, "@@ ") || strings.HasPrefix(line, "diff --git ") {
				break
			}
			switch {
			case strings.HasPrefix(line, "\\"):
				// "\ No newline at end of file" — not a content line.
			case strings.HasPrefix(line, "-"):
				gotOld++
			case strings.HasPrefix(line, "+"):
				gotNew++
			case line == "" || strings.HasPrefix(line, " "):
				gotOld++
				gotNew++
			default:
				return fmt.Errorf("executor: malformed diff — line %q inside hunk %q has no +/-/context marker", line, lines[i])
			}
		}
		if gotOld != wantOld || gotNew != wantNew {
			return fmt.Errorf("executor: hunk %q declares -%d,+%d lines but its body has %d,%d — the model miscounted the diff", lines[i], wantOld, wantNew, gotOld, gotNew)
		}
		i = j - 1
	}
	return nil
}

// fencedDiff returns the content of the first ```diff fenced block, if any.
func fencedDiff(out string) (string, bool) {
	lines := strings.Split(out, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "```diff" {
			start = i + 1
			break
		}
	}
	if start == -1 {
		return "", false
	}
	for i := start; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "```" {
			return strings.Join(lines[start:i], "\n"), true
		}
	}
	return "", false
}

// rawDiff returns everything from the first unified-diff marker to the end.
func rawDiff(out string) (string, bool) {
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "diff --git ") || strings.HasPrefix(line, "--- ") {
			return strings.Join(lines[i:], "\n"), true
		}
	}
	return "", false
}

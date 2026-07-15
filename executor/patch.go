package executor

import (
	"errors"
	"strings"
)

// ParsePatch deterministically extracts a unified diff from a
// text-generating model's raw output — shared by every Executor that asks
// a model to emit a diff (executor/claude, executor/openai): it prefers a
// fenced ```diff block, otherwise takes everything from the first
// unified-diff marker to the end. The result is normalized to end in
// exactly one newline, which `git apply` requires.
func ParsePatch(out string) (string, error) {
	if strings.TrimSpace(out) == "" {
		return "", errors.New("executor: empty output; no patch produced")
	}
	if patch, ok := fencedDiff(out); ok {
		return strings.TrimRight(patch, "\n") + "\n", nil
	}
	if patch, ok := rawDiff(out); ok {
		return strings.TrimRight(patch, "\n") + "\n", nil
	}
	return "", errors.New("executor: no unified diff found in output")
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

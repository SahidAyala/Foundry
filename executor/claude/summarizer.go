package claude

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// maxSummarizePatchChars bounds how much of a Patch Summarize includes in
// its prompt — a deliberately simple, fixed cap (not a smart diff
// summarizer of its own) to keep a very large Act's summarization prompt
// small and cheap, the same reasoning gatherer.NaiveGatherer's own byte
// budget already applies to gathered Context.
const maxSummarizePatchChars = 8000

// Summarizer generates a short pull request title and body from an Act's
// Intent and Patch by asking Claude Code for one — vcs.GitHubPRApplier's
// optional alternative to its own mechanical "echo the Intent verbatim"
// default (ADR-0013's post-ratification Claude model selection note; a
// distinct, lower-stakes use of the same CLI ClaudeExecutor already
// wraps). Unlike Execute, this never produces or parses a diff — it asks
// for, and parses, a small structured text response instead.
type Summarizer struct {
	workspace  string
	model      string
	executable string
	runner     runner
}

// NewSummarizer returns a Summarizer that runs Claude Code in workspace,
// passing model to the CLI's own --model flag when non-empty — the same
// convention NewClaudeExecutor's own model argument already establishes.
// An empty model defers to Claude Code's own configured default.
func NewSummarizer(workspace, model string) *Summarizer {
	return &Summarizer{
		workspace:  workspace,
		model:      model,
		executable: defaultExecutable,
		runner:     execRunner{},
	}
}

// Summarize asks Claude Code to render intent and patch as a short pull
// request title and body, satisfying vcs.PRSummarizer. A response Claude
// doesn't format as requested (see summarizePrompt) is a clear, named
// error — never silently accepted as a garbled title — so callers can
// fall back to their own mechanical default instead of publishing
// confusing text.
func (s *Summarizer) Summarize(ctx context.Context, intent, patch string) (title, body string, err error) {
	if s.workspace == "" {
		return "", "", errors.New("claude: summarize: no workspace configured")
	}

	prompt := summarizePrompt(intent, patch)
	args := []string{"-p"}
	if s.model != "" {
		args = append(args, "--model", s.model)
	}
	stdout, stderr, err := s.runner.Run(ctx, s.workspace, s.executable, args, prompt)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", "", fmt.Errorf("claude: summarize: executable %q not found in PATH", s.executable)
		}
		return "", "", executionError(err, stdout, stderr)
	}

	return parseSummary(stdout)
}

// summarizePrompt asks for a strict, two-part response so parseSummary
// can extract it reliably — the same "ask for one specific shape, refuse
// to guess at anything looser" discipline executor.ParsePatch's own diff
// extraction already follows for Execute's prompt. patch is truncated to
// maxSummarizePatchChars: this is a short-summary aid, not a full-context
// review, so a very large diff doesn't need to be sent in full.
func summarizePrompt(intent, patch string) string {
	if len(patch) > maxSummarizePatchChars {
		patch = patch[:maxSummarizePatchChars] + "\n... (truncated)"
	}
	var b strings.Builder
	b.WriteString("You are writing a pull request title and description for a code change.\n\n")
	b.WriteString("Intent:\n")
	b.WriteString(intent)
	b.WriteString("\n\nDiff:\n")
	b.WriteString(patch)
	b.WriteString("\n\nRespond in exactly this format, nothing else:\n")
	b.WriteString("TITLE: <a concise one-line title, under 72 characters>\n")
	b.WriteString("BODY:\n<a short paragraph (2-4 sentences) explaining what changed and why>")
	return b.String()
}

// parseSummary extracts a "TITLE: ..." line and everything after a
// "BODY:" line from out, the shape summarizePrompt asks for. Either
// marker missing is a clear, named error — a caller falls back to its own
// mechanical default rather than publish a response that doesn't actually
// carry a well-formed title and body.
func parseSummary(out string) (title, body string, err error) {
	titleIdx := strings.Index(out, "TITLE:")
	bodyIdx := strings.Index(out, "BODY:")
	if titleIdx == -1 || bodyIdx == -1 || bodyIdx < titleIdx {
		return "", "", fmt.Errorf("claude: summarize: response did not contain both TITLE: and BODY: markers in order: %s", strings.TrimSpace(out))
	}

	title = strings.TrimSpace(out[titleIdx+len("TITLE:") : bodyIdx])
	body = strings.TrimSpace(out[bodyIdx+len("BODY:"):])
	if title == "" {
		return "", "", fmt.Errorf("claude: summarize: TITLE: was empty in response: %s", strings.TrimSpace(out))
	}
	if body == "" {
		return "", "", fmt.Errorf("claude: summarize: BODY: was empty in response: %s", strings.TrimSpace(out))
	}
	return title, body, nil
}

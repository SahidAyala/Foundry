package claude

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/SahidAyala/Foundry/domain"
	"github.com/SahidAyala/Foundry/engine"
)

// reviewInstructions is prepended to every review prompt, asking Claude
// Code to respond in a fixed, easily-parsed shape — never a diff of its
// own, never free-form prose. Mirrors verify/aireview's own systemPrompt;
// unlike that package's HTTP call, the CLI's `-p` mode has no separate
// system-role message, so the instruction is folded into the one prompt
// sent over stdin, the same flat-prompt convention summarizePrompt and
// buildPrompt already use.
const reviewInstructions = `You are a code reviewer. You will be shown a proposed unified diff and asked whether it should be accepted.
Respond in exactly this format and nothing else:

VERDICT: pass
FINDINGS:
- (one finding per line, or "none" if there are no findings)

VERDICT must be exactly "pass" or "fail". Use "fail" for anything you would ask a human engineer to fix before merging — bugs, missing error handling, security issues, or anything unclear enough that you cannot confirm it is correct.`

// Reviewer asks Claude Code to review an Outcome's Patch and reports a
// domain.Judgment, satisfying engine.Verifier — verify/aireview's
// Verifier shape (systemPrompt discipline, VERDICT/FINDINGS parsing), but
// invoked as a `claude -p` subprocess against the caller's own Claude
// Code subscription (like Summarizer and ClaudeExecutor already are),
// instead of aireview's direct HTTP call against an OpenAI-Chat-
// Completions-compatible endpoint.
//
// Unlike Summarizer/ClaudeExecutor, Reviewer's workspace is not fixed at
// construction: engine.Verifier.Verify carries its own workspace argument
// (the staged worktree workspace.StagedVerifier already applied the
// Outcome's patch into, which differs on every call), so Verify uses that
// argument as the subprocess's working directory instead of storing one
// of its own — Reviewer itself is constructed once, at wiring time, and
// reused for every Act.
//
// Reviewing a diff with the same subscription that may have written it is
// a weaker check than an independent vendor reviewing it (docs/04-guides/
// getting-started.md's "AI code review" section recommends a third,
// independent model so review isn't the same model re-checking its own
// work) — Reviewer exists for a project that has explicitly chosen this
// tradeoff (e.g. it has no separate API key or vendor available), not as
// the recommended default; verify/aireview remains the independent-vendor
// path.
type Reviewer struct {
	model      string
	executable string
	runner     runner
}

// NewReviewer returns a Reviewer that runs Claude Code's `-p` mode,
// passing model to the CLI's own --model flag when non-empty — the same
// convention NewClaudeExecutor's own model argument already establishes.
// An empty model defers to Claude Code's own configured default.
func NewReviewer(model string) *Reviewer {
	return &Reviewer{
		model:      model,
		executable: defaultExecutable,
		runner:     execRunner{},
	}
}

var _ engine.Verifier = (*Reviewer)(nil)

// Verify runs Claude Code inside workspace — already staged with
// outcome's patch applied by workspace.StagedVerifier — and parses its
// response into a Judgment. A response Reviewer cannot confidently parse
// as "pass" is treated as "fail": an ambiguous review must never look
// identical to a clean bill of health, the same discipline verify/
// aireview's own parseReview follows.
func (r *Reviewer) Verify(ctx context.Context, outcome *domain.Outcome, workspace string) (*domain.Judgment, error) {
	if workspace == "" {
		return nil, errors.New("claude: review: no workspace configured")
	}

	prompt := reviewPrompt(outcome.Intent, outcome.Patch)
	args := []string{"-p"}
	if r.model != "" {
		args = append(args, "--model", r.model)
	}
	stdout, stderr, err := r.runner.Run(ctx, workspace, r.executable, args, prompt)
	if err != nil {
		switch {
		case errors.Is(err, exec.ErrNotFound):
			return nil, fmt.Errorf("claude: review: executable %q not found in PATH", r.executable)
		case errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded):
			return nil, fmt.Errorf("claude: review: timed out")
		default:
			return nil, executionError(err, stdout, stderr)
		}
	}

	return parseReview(stdout), nil
}

// reviewPrompt renders the prompt asking Claude Code to review patch,
// mirroring verify/aireview's own reviewPrompt wording exactly: when
// intent is non-empty, the original request is included so the model can
// judge whether the diff actually satisfies it, not only whether it looks
// internally reasonable.
func reviewPrompt(intent, patch string) string {
	var b strings.Builder
	b.WriteString(reviewInstructions)
	b.WriteString("\n\n")
	if intent == "" {
		b.WriteString("Review this diff:\n\n")
	} else {
		b.WriteString("The original request was:\n\n")
		b.WriteString(intent)
		b.WriteString("\n\nReview whether the following diff satisfies it:\n\n")
	}
	b.WriteString(patch)
	return b.String()
}

// parseReview extracts a Judgment from Claude Code's response, expecting
// reviewInstructions' documented "VERDICT: .../FINDINGS: ..." shape —
// byte-for-byte the same parsing discipline verify/aireview.parseReview
// already implements. Checked entries are prefixed "claude-review:"
// rather than aireview's "ai-review:" so a Judgment recording both
// (verify.Compose'd together) still lets a human reading recorded
// Evidence (`foundry show`) tell which layer reported which finding.
func parseReview(content string) *domain.Judgment {
	lines := strings.Split(content, "\n")

	verdict := ""
	findingsStart := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case verdict == "" && strings.HasPrefix(strings.ToUpper(trimmed), "VERDICT:"):
			verdict = strings.ToLower(strings.TrimSpace(trimmed[len("VERDICT:"):]))
		case findingsStart == -1 && strings.HasPrefix(strings.ToUpper(trimmed), "FINDINGS:"):
			findingsStart = i + 1
		}
	}

	if verdict != "pass" {
		return &domain.Judgment{
			Verdict: "fail",
			Checked: []string{"claude-review: fail\n" + strings.TrimSpace(content)},
		}
	}

	var findings []string
	if findingsStart >= 0 {
		for _, line := range lines[findingsStart:] {
			f := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
			f = strings.TrimSpace(f)
			if f == "" || strings.EqualFold(f, "none") || strings.EqualFold(f, "(none)") {
				continue
			}
			findings = append(findings, f)
		}
	}

	if len(findings) == 0 {
		return &domain.Judgment{Verdict: "pass", Checked: []string{"claude-review: pass"}}
	}
	return &domain.Judgment{Verdict: "pass", Checked: append([]string{"claude-review: pass"}, findings...)}
}

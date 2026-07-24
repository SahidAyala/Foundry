package verify

import (
	"context"

	"foundry/domain"
	"foundry/engine"
)

// Compose returns an engine.Verifier that runs each of verifiers in order
// against the same Outcome, ANDs their Verdicts (any non-"pass" verdict
// fails the whole composition), and concatenates their Checked findings —
// so more than one kind of verification (a deterministic Gate, an AI
// review) can judge one Outcome without any change to engine.Verifier or
// engine.Engine, which still holds exactly one. This mirrors
// gatherer.Compose (RFC-0005 §2.2) and ExecutorRegistry/Router
// (RFC-0004 Piece 1): the port stays a single method; a small, additive
// seam lets a composition root wire more than one concrete implementation
// behind it. Compose of a single verifier behaves identically to using it
// directly; a verifier that returns an error stops the whole composition
// immediately, in order, exactly as a single Verifier failing would.
func Compose(verifiers ...engine.Verifier) engine.Verifier {
	return composedVerifier{verifiers: verifiers}
}

type composedVerifier struct {
	verifiers []engine.Verifier
}

func (c composedVerifier) Verify(ctx context.Context, outcome *domain.Outcome, workspace string) (*domain.Judgment, error) {
	verdict := "pass"
	var checked []string
	for _, v := range c.verifiers {
		judgment, err := v.Verify(ctx, outcome, workspace)
		if err != nil {
			return nil, err
		}
		checked = append(checked, judgment.Checked...)
		if judgment.Verdict != "pass" {
			verdict = "fail"
		}
	}
	return &domain.Judgment{Verdict: verdict, Checked: checked}, nil
}

var _ engine.Verifier = composedVerifier{}

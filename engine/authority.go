package engine

import (
	"context"
	"errors"

	"foundry/domain"
)

// Authority is the port an approve Step calls to accept or reject an Act's
// Outcome so far (RFC-0002 §4.2: "an Authority — human, or an explicitly
// delegated policy — accepts/rejects"). Decide returns the accountable
// identity that decided (empty on rejection) and whether it approved,
// mirroring cli.PromptForApproval's own return shape so that function can
// become an Authority implementation rather than a bespoke after-the-fact
// call.
type Authority interface {
	Decide(ctx context.Context, act *domain.Act) (authority string, approved bool, err error)
}

// VerdictRejected is the Act-level JudgmentVerdict recorded when an approve
// Step's Authority declines: distinct from a Verify Step's "pass"/"fail",
// since a human rejection is not something a bounded repair round can fix.
const VerdictRejected = "rejected"

// stepVerdictAccept and stepVerdictReject label an approve Step's own
// StepRecord — distinct from VerdictRejected, which labels the Act's final
// outcome once an approve Step has stopped the Pipeline.
const (
	stepVerdictAccept = "accept"
	stepVerdictReject = "reject"
)

// ErrNoAuthority is wrapped by noAuthority.Decide: a Pipeline that declares
// an approve Step requires an Engine built with SetAuthority called, so an
// approve Step never silently approves or rejects on a caller's behalf.
var ErrNoAuthority = errors.New("engine: no Authority configured for this Engine")

// noAuthority is the Engine's default Authority. It refuses every Decide
// call with a clear, named error instead of silently approving — which
// would let accountability leave a human silently — or silently rejecting,
// which would make every approve Step an unannounced dead end.
type noAuthority struct{}

func (noAuthority) Decide(ctx context.Context, act *domain.Act) (string, bool, error) {
	return "", false, ErrNoAuthority
}

var _ Authority = noAuthority{}

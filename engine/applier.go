package engine

import (
	"context"
	"errors"

	"foundry/domain"
)

// Applier is the port an apply Step calls to apply an accepted Outcome to
// Project State — a change to code (via a Workspace) or to Knowledge
// (RFC-0002 §4.2). workspace is the Engine's configured workspace
// directory, the same one Verifier already checks.
type Applier interface {
	Apply(ctx context.Context, workspace string, act *domain.Act) error
}

// ErrNoApplier is wrapped by noApplier.Apply: a Pipeline that declares an
// apply Step requires an Engine built with SetApplier called, so an apply
// Step never silently no-ops instead of mutating Project State.
var ErrNoApplier = errors.New("engine: no Applier configured for this Engine")

// noApplier is the Engine's default Applier: it refuses every Apply call
// with a clear, named error rather than silently doing nothing, which would
// let a Pipeline report success without ever having applied its Outcome.
type noApplier struct{}

func (noApplier) Apply(ctx context.Context, workspace string, act *domain.Act) error {
	return ErrNoApplier
}

var _ Applier = noApplier{}

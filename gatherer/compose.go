package gatherer

import (
	"context"

	"foundry/domain"
	"foundry/engine"
)

// Compose returns a Gatherer that runs each of sources in order and
// concatenates their considered Context, so more than one Context Source
// (docs/02-architecture/extensibility.md) can feed one Act without any
// change to engine.Gatherer, engine.Engine, or RunBudgeted's single Gather
// call (RFC-0005 §2.2, docs/01-rfcs/RFC-0005-authored-knowledge-retrieval.md).
// This mirrors ExecutorRegistry/Router (RFC-0004 Piece 1) and
// ApplierRegistry (RFC-0004 Piece 4): the port stays a single method; a
// small, additive seam lets a composition root wire more than one concrete
// implementation behind it. Compose of a single source behaves identically
// to using that source directly; a source that returns an error stops the
// whole composition immediately, in order, exactly as a single Gatherer
// failing would.
func Compose(sources ...engine.Gatherer) engine.Gatherer {
	return compositeGatherer{sources: sources}
}

type compositeGatherer struct {
	sources []engine.Gatherer
}

func (c compositeGatherer) Gather(ctx context.Context, intent *domain.Intent) ([]string, error) {
	var all []string
	for _, source := range c.sources {
		got, err := source.Gather(ctx, intent)
		if err != nil {
			return nil, err
		}
		all = append(all, got...)
	}
	return all, nil
}

var _ engine.Gatherer = compositeGatherer{}

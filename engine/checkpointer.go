package engine

import (
	"context"
	"errors"

	"foundry/domain"
)

// Checkpointer is the port a record Step calls to persist the Act's trace
// so far — RFC-0002 §4.2's "checkpoint the Act's Step trace so far to the
// Record." Checkpointer names only the one method a record Step needs, so
// any record.Recorder (record.FileStore included) already satisfies it by
// structural typing, with no adapter type required.
//
// record.FileStore.Write is write-once (an Act, once recorded, is
// immutable): a Pipeline that declares more than one record Step for the
// same Act will see the second call fail with record.ErrAlreadyExists,
// surfaced as a plain Step error rather than something PipelineStrategy
// prevents proactively. Today's Pipelines declare at most one.
type Checkpointer interface {
	Write(ctx context.Context, act *domain.Act) error
}

// ErrNoCheckpointer is wrapped by noCheckpointer.Write: a Pipeline that
// declares a record Step requires an Engine built with SetCheckpointer
// called, so a record Step never silently no-ops instead of persisting the
// Act.
var ErrNoCheckpointer = errors.New("engine: no Checkpointer configured for this Engine")

// noCheckpointer is the Engine's default Checkpointer: it refuses every
// Write call with a clear, named error rather than silently doing nothing,
// which would let a Pipeline report success without ever having recorded
// the Act.
type noCheckpointer struct{}

func (noCheckpointer) Write(ctx context.Context, act *domain.Act) error {
	return ErrNoCheckpointer
}

var _ Checkpointer = noCheckpointer{}

// CheckpointSaver persists an Act's in-progress trace after every Step —
// no Pipeline declaration required, unlike Checkpointer — so a crash or
// kill mid-Pipeline leaves state a later `foundry resume` can continue
// (docs/06-open-questions/OQ-008-in-progress-act-persistence.md). Its
// writes are overwritable, and Delete marks an Act as no longer
// interrupted once it reaches a real terminal disposition.
type CheckpointSaver interface {
	Save(ctx context.Context, act *domain.Act) error
	Delete(ctx context.Context, actID string) error
}

// noCheckpointSaver is the Engine's default CheckpointSaver: both methods
// no-op, so an Engine that never calls SetCheckpointSaver behaves exactly
// as it did before resume existed — no checkpoint file, and no error.
type noCheckpointSaver struct{}

func (noCheckpointSaver) Save(ctx context.Context, act *domain.Act) error { return nil }
func (noCheckpointSaver) Delete(ctx context.Context, actID string) error  { return nil }

var _ CheckpointSaver = noCheckpointSaver{}

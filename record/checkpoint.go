package record

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"foundry/domain"
)

// CheckpointStore persists an Act's in-progress trace before it reaches a
// terminal Judgment — mutable and overwritable, unlike FileStore's
// write-once Record (docs/06-open-questions/OQ-008-in-progress-act-persistence.md).
// A checkpoint is explicitly not the Record: it exists only so a crash or
// kill mid-Pipeline leaves state a later `foundry resume` can continue, and
// is deleted once the Act reaches a real terminal disposition.
//
// CheckpointStore shares FileStore's root, so a checkpoint sits alongside
// its eventual act.json at <root>/<act.ID>/checkpoint.json — but the two
// types are independent: a CheckpointStore never reads or writes act.json,
// and a FileStore never reads or writes checkpoint.json.
type CheckpointStore struct {
	root string
}

// NewCheckpointStore creates a CheckpointStore rooted at the given
// directory, creating it if it does not exist.
func NewCheckpointStore(root string) (*CheckpointStore, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("record: create checkpoint root directory: %w", err)
	}
	return &CheckpointStore{root: root}, nil
}

func (s *CheckpointStore) checkpointPath(actID string) string {
	return filepath.Join(s.root, actID, "checkpoint.json")
}

// Save persists act's current trace, overwriting any previous checkpoint
// for the same Act ID — repeated calls as a Pipeline's Steps complete are
// expected, not an error.
func (s *CheckpointStore) Save(ctx context.Context, act *domain.Act) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if act.ID == "" {
		return fmt.Errorf("record: checkpoint: act ID is empty")
	}

	dir := filepath.Join(s.root, act.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("record: create checkpoint directory: %w", err)
	}

	data, err := encode(act)
	if err != nil {
		return fmt.Errorf("record: encode checkpoint %s: %w", act.ID, err)
	}

	if err := os.WriteFile(s.checkpointPath(act.ID), data, 0o644); err != nil {
		return fmt.Errorf("record: write checkpoint file: %w", err)
	}
	return nil
}

// Load returns the checkpointed Act previously saved under actID.
func (s *CheckpointStore) Load(ctx context.Context, actID string) (*domain.Act, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(s.checkpointPath(actID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("record: no checkpoint for act: %s", actID)
		}
		return nil, fmt.Errorf("record: read checkpoint file: %w", err)
	}

	act, err := decode(data)
	if err != nil {
		return nil, fmt.Errorf("record: decode checkpoint %s: %w", actID, err)
	}
	return act, nil
}

// Delete removes the checkpoint for actID, if one exists. Deleting a
// checkpoint that does not exist is not an error — the Act may never have
// been interrupted, or may already have been resumed to completion.
func (s *CheckpointStore) Delete(ctx context.Context, actID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Remove(s.checkpointPath(actID)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("record: delete checkpoint file: %w", err)
	}
	return nil
}

// List returns every Act with a surviving checkpoint — the interrupted
// Acts a caller (e.g. `foundry resume` with no act ID) might continue.
func (s *CheckpointStore) List(ctx context.Context) ([]*domain.Act, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, fmt.Errorf("record: list checkpoint directories: %w", err)
	}

	acts := make([]*domain.Act, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		act, err := s.Load(ctx, entry.Name())
		if err != nil {
			// Most act directories have an act.json but no
			// checkpoint.json — a completed Act, not an interrupted one.
			continue
		}
		acts = append(acts, act)
	}
	return acts, nil
}

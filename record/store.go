// Package record persists Acts to durable storage.
package record

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"foundry/domain"
)

// ErrAlreadyExists is returned by Write when an Act with the same ID
// has already been recorded. Acts are immutable once written.
var ErrAlreadyExists = errors.New("record: act already exists")

// ErrNotFound is returned by Read (and, via List, silently skipped rather
// than propagated) when actID has no act.json yet. List treats this
// specifically — not any other Read failure — as the benign race between
// its directory scan and a Write still in progress: Write's MkdirAll
// (below) makes the act's directory visible to List before act.json is
// published, so List can observe a directory with no act.json yet for an
// Act that is simply still being written, not one that is missing or
// corrupt.
var ErrNotFound = errors.New("record: act not found")

// Recorder persists Acts.
type Recorder interface {
	Write(ctx context.Context, act *domain.Act) error
	Read(ctx context.Context, actID string) (*domain.Act, error)
	List(ctx context.Context) ([]*domain.Act, error)
}

// FileStore is a filesystem-backed, immutable Recorder.
// Each Act is written to <root>/<act.ID>/act.json exactly once.
type FileStore struct {
	root string
}

// NewFileStore creates a FileStore rooted at the given directory,
// creating the directory if it does not exist.
func NewFileStore(root string) (*FileStore, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("record: create root directory: %w", err)
	}
	return &FileStore{root: root}, nil
}

var _ Recorder = (*FileStore)(nil)

func (s *FileStore) actPath(actID string) string {
	return filepath.Join(s.root, actID, "act.json")
}

// Write persists act. It fails with ErrAlreadyExists if act.ID has
// already been recorded.
//
// act.json is published atomically: the encoded Act is written in full to
// a temp file in the same directory (so the publish below is a same-
// filesystem operation), synced, then published via a hard link rather
// than a rename. Link fails with EEXIST if act.json already exists —
// preserving the exact write-once exclusivity O_EXCL gave before — but
// unlike O_EXCL opened directly against act.json, the file a reader can
// ever observe at that path is either absent or fully written, never
// truncated by a crash between open and write. Without this, a crash mid-
// write left a zero-length act.json that Read could never decode and
// Write could never retry (O_EXCL sees the file as already existing).
func (s *FileStore) Write(ctx context.Context, act *domain.Act) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if act.ID == "" {
		return errors.New("record: act ID is empty")
	}

	dir := filepath.Join(s.root, act.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("record: create act directory: %w", err)
	}

	data, err := encode(act)
	if err != nil {
		return fmt.Errorf("record: encode act %s: %w", act.ID, err)
	}

	tmp, err := os.CreateTemp(dir, "act.json.tmp-*")
	if err != nil {
		return fmt.Errorf("record: create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // best-effort; a no-op once Link has already moved the only durable reference to act.json

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("record: write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("record: sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("record: close temp file: %w", err)
	}

	if err := os.Link(tmpPath, s.actPath(act.ID)); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("%w: %s", ErrAlreadyExists, act.ID)
		}
		return fmt.Errorf("record: publish act file: %w", err)
	}
	return nil
}

// Read returns the Act previously written under actID.
func (s *FileStore) Read(ctx context.Context, actID string) (*domain.Act, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(s.actPath(actID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, actID)
		}
		return nil, fmt.Errorf("record: read act file: %w", err)
	}

	act, err := decode(data)
	if err != nil {
		return nil, fmt.Errorf("record: decode act %s: %w", actID, err)
	}
	return act, nil
}

// List returns all recorded Acts in creation order. An Act directory that
// exists but has no act.json yet — the benign race between this scan and
// a Write still in progress (MkdirAll runs before act.json is published;
// see Write) — is skipped, not treated as an error: every other, fully-
// written Act must still be returned. Any other Read failure (a genuine
// decode error, a permissions problem) is a real problem and is still
// propagated for the whole call, exactly as before — List must never
// silently drop an Act that record actually holds.
func (s *FileStore) List(ctx context.Context) ([]*domain.Act, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, fmt.Errorf("record: list act directories: %w", err)
	}

	acts := make([]*domain.Act, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		act, err := s.Read(ctx, entry.Name())
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return nil, err
		}
		acts = append(acts, act)
	}

	sort.Slice(acts, func(i, j int) bool {
		if acts[i].CreatedAt.Equal(acts[j].CreatedAt) {
			return acts[i].ID < acts[j].ID
		}
		return acts[i].CreatedAt.Before(acts[j].CreatedAt)
	})
	return acts, nil
}

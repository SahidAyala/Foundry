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

	f, err := os.OpenFile(s.actPath(act.ID), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("%w: %s", ErrAlreadyExists, act.ID)
		}
		return fmt.Errorf("record: open act file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("record: write act file: %w", err)
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
			return nil, fmt.Errorf("record: act not found: %s", actID)
		}
		return nil, fmt.Errorf("record: read act file: %w", err)
	}

	act, err := decode(data)
	if err != nil {
		return nil, fmt.Errorf("record: decode act %s: %w", actID, err)
	}
	return act, nil
}

// List returns all recorded Acts in creation order.
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

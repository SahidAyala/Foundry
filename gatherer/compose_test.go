package gatherer_test

import (
	"context"
	"errors"
	"testing"

	"foundry/domain"
	"foundry/engine"
	"foundry/gatherer"
)

// fakeGatherer returns a canned slice or error from Gather, recording
// whether it was called.
type fakeGatherer struct {
	entries []string
	err     error
	called  bool
}

func (g *fakeGatherer) Gather(ctx context.Context, intent *domain.Intent) ([]string, error) {
	g.called = true
	if g.err != nil {
		return nil, g.err
	}
	return g.entries, nil
}

func TestCompose_NoSourcesReturnsEmpty(t *testing.T) {
	got, err := gatherer.Compose().Gather(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Gather failed: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Gather = %v, want empty", got)
	}
}

func TestCompose_OneSourceBehavesIdentically(t *testing.T) {
	source := &fakeGatherer{entries: []string{"a.go:\npackage a\n"}}

	got, err := gatherer.Compose(source).Gather(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Gather failed: %v", err)
	}
	if len(got) != 1 || got[0] != "a.go:\npackage a\n" {
		t.Errorf("Gather = %v, want the single source's own entries unchanged", got)
	}
}

func TestCompose_ConcatenatesInOrder(t *testing.T) {
	first := &fakeGatherer{entries: []string{"main.go:\npackage main\n"}}
	second := &fakeGatherer{entries: []string{".foundry/knowledge/note.md:\nsome note\n"}}

	got, err := gatherer.Compose(first, second).Gather(context.Background(), &domain.Intent{Text: "test"})
	if err != nil {
		t.Fatalf("Gather failed: %v", err)
	}
	want := []string{"main.go:\npackage main\n", ".foundry/knowledge/note.md:\nsome note\n"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("Gather = %v, want %v (in order)", got, want)
	}
	if !first.called || !second.called {
		t.Error("both sources must be called")
	}
}

func TestCompose_ErrorFromEarlierSourceStopsComposition(t *testing.T) {
	wantErr := errors.New("first source failed")
	first := &fakeGatherer{err: wantErr}
	second := &fakeGatherer{entries: []string{"never reached"}}

	_, err := gatherer.Compose(first, second).Gather(context.Background(), &domain.Intent{Text: "test"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if second.called {
		t.Error("second source must not be called once an earlier one fails")
	}
}

var _ engine.Gatherer = &fakeGatherer{}

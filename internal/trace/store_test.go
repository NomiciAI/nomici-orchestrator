package trace

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/NomiciAI/nomici-orchestrator/internal/store"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return NewStore(db)
}

func TestAppendAssignsSequence(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	if err := store.Append(ctx, &Event{RunID: "run_1", Type: EventRunStarted}); err != nil {
		t.Fatalf("append first event: %v", err)
	}
	if err := store.Append(ctx, &Event{RunID: "run_1", Type: EventRunCompleted}); err != nil {
		t.Fatalf("append second event: %v", err)
	}

	events, err := store.ListByRun(ctx, "run_1")
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected two events, got %d", len(events))
	}
	if events[0].Sequence != 1 || events[1].Sequence != 2 {
		t.Fatalf("expected sequences 1 and 2, got %d and %d", events[0].Sequence, events[1].Sequence)
	}
}

func TestAppendSeparateRunSequences(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	if err := store.Append(ctx, &Event{RunID: "run_a", Type: EventRunStarted}); err != nil {
		t.Fatalf("append run_a: %v", err)
	}
	if err := store.Append(ctx, &Event{RunID: "run_b", Type: EventRunStarted}); err != nil {
		t.Fatalf("append run_b: %v", err)
	}

	events, err := store.ListByRun(ctx, "run_b")
	if err != nil {
		t.Fatalf("list run_b: %v", err)
	}
	if events[0].Sequence != 1 {
		t.Fatalf("expected run_b sequence 1, got %d", events[0].Sequence)
	}
}

func TestPayloadRoundTrip(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	payload := json.RawMessage(`{"model":"test"}`)

	if err := store.Append(ctx, &Event{RunID: "run_1", Type: EventModelRequested, Payload: payload}); err != nil {
		t.Fatalf("append event: %v", err)
	}

	events, err := store.ListByRun(ctx, "run_1")
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if string(events[0].Payload) != string(payload) {
		t.Fatalf("expected payload %s, got %s", payload, events[0].Payload)
	}
}

func TestListRuns(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	if err := store.Append(ctx, &Event{RunID: "run_1", Type: EventRunStarted}); err != nil {
		t.Fatalf("append run_1: %v", err)
	}
	if err := store.Append(ctx, &Event{RunID: "run_1", Type: EventRunCompleted}); err != nil {
		t.Fatalf("append run_1 completion: %v", err)
	}
	if err := store.Append(ctx, &Event{RunID: "run_2", Type: EventRunStarted}); err != nil {
		t.Fatalf("append run_2: %v", err)
	}

	runs, err := store.ListRuns(ctx)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected two runs, got %d", len(runs))
	}
}

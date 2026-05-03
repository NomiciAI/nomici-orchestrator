package trace

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/ids"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (store *Store) Append(ctx context.Context, event *Event) error {
	if event == nil {
		return fmt.Errorf("append trace event: nil event")
	}
	if event.RunID == "" {
		return fmt.Errorf("append trace event: run_id is required")
	}
	if event.Type == "" {
		return fmt.Errorf("append trace event: type is required")
	}
	if event.EventID == "" {
		event.EventID = ids.New("evt")
	}
	if event.Sequence == 0 {
		sequence, err := store.nextSequence(ctx, event.RunID)
		if err != nil {
			return err
		}
		event.Sequence = sequence
	}
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	}
	if len(event.Payload) == 0 {
		event.Payload = json.RawMessage("{}")
	}
	if len(event.Metadata) == 0 {
		event.Metadata = json.RawMessage("{}")
	}
	redactions, err := json.Marshal(event.Redactions)
	if err != nil {
		return fmt.Errorf("marshal trace redactions: %w", err)
	}

	_, err = store.db.ExecContext(ctx, `
INSERT INTO trace_events (
	event_id, run_id, sequence, type, time, node_id, runtime_id,
	payload_json, redactions_json, metadata_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.EventID,
		event.RunID,
		event.Sequence,
		event.Type,
		event.Time.Format(time.RFC3339Nano),
		event.NodeID,
		event.RuntimeID,
		string(event.Payload),
		string(redactions),
		string(event.Metadata),
	)
	if err != nil {
		return fmt.Errorf("append trace event: %w", err)
	}

	return nil
}

func (store *Store) ListByRun(ctx context.Context, runID string) ([]*Event, error) {
	return store.ListByRunAfter(ctx, runID, 0)
}

func (store *Store) ListByRunAfter(ctx context.Context, runID string, afterSequence int) ([]*Event, error) {
	rows, err := store.db.QueryContext(ctx, `
SELECT event_id, run_id, sequence, type, time, node_id, runtime_id,
	payload_json, redactions_json, metadata_json
FROM trace_events
WHERE run_id = ? AND sequence > ?
ORDER BY sequence`, runID, afterSequence)
	if err != nil {
		return nil, fmt.Errorf("list trace events: %w", err)
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		var event Event
		var eventTime string
		var payloadJSON string
		var redactionsJSON string
		var metadataJSON string
		if err := rows.Scan(
			&event.EventID,
			&event.RunID,
			&event.Sequence,
			&event.Type,
			&eventTime,
			&event.NodeID,
			&event.RuntimeID,
			&payloadJSON,
			&redactionsJSON,
			&metadataJSON,
		); err != nil {
			return nil, fmt.Errorf("scan trace event: %w", err)
		}
		parsedTime, err := time.Parse(time.RFC3339Nano, eventTime)
		if err != nil {
			return nil, fmt.Errorf("parse trace event time: %w", err)
		}
		event.Time = parsedTime
		event.Payload = json.RawMessage(payloadJSON)
		event.Metadata = json.RawMessage(metadataJSON)
		if err := json.Unmarshal([]byte(redactionsJSON), &event.Redactions); err != nil {
			return nil, fmt.Errorf("decode trace redactions: %w", err)
		}
		events = append(events, &event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list trace events: %w", err)
	}

	return events, nil
}

func (store *Store) ListRuns(ctx context.Context) ([]*RunSummary, error) {
	rows, err := store.db.QueryContext(ctx, `
WITH last_events AS (
	SELECT run_id, MAX(sequence) AS max_sequence
	FROM trace_events
	GROUP BY run_id
)
SELECT
	t.run_id,
	COUNT(*) AS event_count,
	MIN(t.time) AS first_time,
	MAX(t.time) AS last_time,
	COALESCE(last.type, '') AS last_type
FROM trace_events t
JOIN last_events le ON le.run_id = t.run_id
JOIN trace_events last ON last.run_id = le.run_id AND last.sequence = le.max_sequence
GROUP BY t.run_id, last.type
ORDER BY last_time DESC`)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	defer rows.Close()

	var summaries []*RunSummary
	for rows.Next() {
		var summary RunSummary
		if err := rows.Scan(&summary.RunID, &summary.EventCount, &summary.FirstTime, &summary.LastTime, &summary.LastType); err != nil {
			return nil, fmt.Errorf("scan run summary: %w", err)
		}
		summaries = append(summaries, &summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	return summaries, nil
}

func (store *Store) nextSequence(ctx context.Context, runID string) (int, error) {
	var sequence int
	if err := store.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(sequence), 0) + 1 FROM trace_events WHERE run_id = ?", runID).Scan(&sequence); err != nil {
		return 0, fmt.Errorf("next trace sequence: %w", err)
	}
	return sequence, nil
}

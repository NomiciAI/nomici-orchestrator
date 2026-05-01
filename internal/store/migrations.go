package store

var migrations = []struct {
	Version int
	SQL     string
}{
	{
		Version: 1,
		SQL: `
CREATE TABLE IF NOT EXISTS provider_profiles (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	kind TEXT NOT NULL,
	base_url TEXT NOT NULL DEFAULT '',
	model TEXT NOT NULL DEFAULT '',
	api_key_env TEXT NOT NULL DEFAULT '',
	capabilities_json TEXT NOT NULL DEFAULT '{}',
	context_window INTEGER NOT NULL DEFAULT 0,
	cost_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS trace_events (
	event_id TEXT PRIMARY KEY,
	run_id TEXT NOT NULL,
	sequence INTEGER NOT NULL,
	type TEXT NOT NULL,
	time TEXT NOT NULL DEFAULT (datetime('now')),
	node_id TEXT NOT NULL DEFAULT '',
	runtime_id TEXT NOT NULL DEFAULT '',
	payload_json TEXT NOT NULL DEFAULT '{}',
	redactions_json TEXT NOT NULL DEFAULT '[]',
	metadata_json TEXT NOT NULL DEFAULT '{}'
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_trace_run_sequence ON trace_events(run_id, sequence);
`,
	},
	{
		Version: 2,
		SQL: `
DROP INDEX IF EXISTS idx_trace_run;
`,
	},
}

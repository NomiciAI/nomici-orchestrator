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
	{
		Version: 3,
		SQL: `
CREATE TABLE IF NOT EXISTS graph_snapshots (
	snapshot_id TEXT PRIMARY KEY,
	schema_version TEXT NOT NULL,
	project_id TEXT NOT NULL,
	source_hash TEXT NOT NULL,
	ir_json TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_graph_snapshots_created_at ON graph_snapshots(created_at DESC);
`,
	},
	{
		Version: 4,
		SQL: `
CREATE TABLE IF NOT EXISTS context_items (
	context_id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	run_id TEXT NOT NULL DEFAULT '',
	task_id TEXT NOT NULL DEFAULT '',
	agent_id TEXT NOT NULL DEFAULT '',
	agent_pair TEXT NOT NULL DEFAULT '',
	task_type TEXT NOT NULL DEFAULT '',
	scope TEXT NOT NULL,
	kind TEXT NOT NULL,
	title TEXT NOT NULL,
	body TEXT NOT NULL,
	tags_json TEXT NOT NULL DEFAULT '[]',
	subject_refs_json TEXT NOT NULL DEFAULT '[]',
	artifact_refs_json TEXT NOT NULL DEFAULT '[]',
	source_json TEXT NOT NULL DEFAULT '{}',
	confidence TEXT NOT NULL DEFAULT 'generated',
	sensitivity TEXT NOT NULL DEFAULT 'normal',
	status TEXT NOT NULL DEFAULT 'active',
	expires_at TEXT NOT NULL DEFAULT '',
	supersedes TEXT NOT NULL DEFAULT '',
	metadata_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_context_items_project ON context_items(project_id, scope, kind);
CREATE INDEX IF NOT EXISTS idx_context_items_run ON context_items(run_id);

CREATE TABLE IF NOT EXISTS context_snapshots (
	snapshot_id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	run_id TEXT NOT NULL,
	task_id TEXT NOT NULL DEFAULT '',
	from_agent TEXT NOT NULL DEFAULT '',
	to_agent TEXT NOT NULL DEFAULT '',
	summary TEXT NOT NULL,
	decisions_json TEXT NOT NULL DEFAULT '[]',
	open_issues_json TEXT NOT NULL DEFAULT '[]',
	recommendations_json TEXT NOT NULL DEFAULT '[]',
	artifact_refs_json TEXT NOT NULL DEFAULT '[]',
	context_item_refs_json TEXT NOT NULL DEFAULT '[]',
	created_by_json TEXT NOT NULL DEFAULT '{}',
	supersedes TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_context_snapshots_run ON context_snapshots(run_id);
CREATE INDEX IF NOT EXISTS idx_context_snapshots_project ON context_snapshots(project_id, created_at DESC);
`,
	},
	{
		Version: 5,
		SQL: `
CREATE TABLE IF NOT EXISTS approvals (
	approval_id TEXT PRIMARY KEY,
	run_id TEXT NOT NULL,
	action_id TEXT NOT NULL,
	action_type TEXT NOT NULL,
	action_fingerprint TEXT NOT NULL,
	status TEXT NOT NULL,
	risk TEXT NOT NULL,
	scope TEXT NOT NULL DEFAULT '',
	summary TEXT NOT NULL,
	subject_json TEXT NOT NULL DEFAULT '{}',
	requested_by_agent TEXT NOT NULL DEFAULT '',
	runtime_id TEXT NOT NULL DEFAULT '',
	reason TEXT NOT NULL DEFAULT '',
	requested_at TEXT NOT NULL,
	resolved_at TEXT NOT NULL DEFAULT '',
	consumed_at TEXT NOT NULL DEFAULT '',
	bound_run_id TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_approvals_status ON approvals(status, requested_at DESC);
CREATE INDEX IF NOT EXISTS idx_approvals_fingerprint ON approvals(action_fingerprint, status);
CREATE INDEX IF NOT EXISTS idx_approvals_run ON approvals(run_id);
`,
	},
	{
		Version: 6,
		SQL: `
CREATE TABLE IF NOT EXISTS pack_installations (
	pack_id TEXT PRIMARY KEY,
	version TEXT NOT NULL,
	kind TEXT NOT NULL,
	trust TEXT NOT NULL,
	config_path TEXT NOT NULL DEFAULT '',
	entrypoints_json TEXT NOT NULL DEFAULT '[]',
	installed_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_pack_installations_updated_at ON pack_installations(updated_at DESC);
`,
	},
	{
		Version: 7,
		SQL: `
CREATE TABLE IF NOT EXISTS run_sessions (
	session_id TEXT PRIMARY KEY,
	run_id TEXT NOT NULL UNIQUE,
	project_id TEXT NOT NULL,
	graph_snapshot_id TEXT NOT NULL,
	title TEXT NOT NULL,
	source_channel TEXT NOT NULL DEFAULT 'console',
	status TEXT NOT NULL,
	started_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	completed_at TEXT NOT NULL DEFAULT '',
	metadata_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_run_sessions_updated ON run_sessions(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_run_sessions_status ON run_sessions(status, updated_at DESC);

CREATE TABLE IF NOT EXISTS run_tasks (
	task_id TEXT PRIMARY KEY,
	run_id TEXT NOT NULL,
	parent_task_id TEXT NOT NULL DEFAULT '',
	agent_id TEXT NOT NULL,
	runtime_id TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL,
	context_snapshot_id TEXT NOT NULL DEFAULT '',
	artifact_refs_json TEXT NOT NULL DEFAULT '[]',
	approval_refs_json TEXT NOT NULL DEFAULT '[]',
	started_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	completed_at TEXT NOT NULL DEFAULT '',
	metadata_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_run_tasks_run ON run_tasks(run_id, started_at);
CREATE INDEX IF NOT EXISTS idx_run_tasks_status ON run_tasks(status, updated_at DESC);
`,
	},
	{
		Version: 8,
		SQL: `
CREATE TABLE IF NOT EXISTS sandbox_records (
	sandbox_id TEXT PRIMARY KEY,
	run_id TEXT NOT NULL UNIQUE,
	task_id TEXT NOT NULL DEFAULT '',
	provider TEXT NOT NULL,
	mode TEXT NOT NULL,
	status TEXT NOT NULL,
	workspace_root TEXT NOT NULL DEFAULT '',
	artifact_root TEXT NOT NULL DEFAULT '',
	runtime_binary TEXT NOT NULL DEFAULT '',
	cleanup_status TEXT NOT NULL DEFAULT 'active',
	message TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	released_at TEXT NOT NULL DEFAULT '',
	metadata_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_sandbox_records_status ON sandbox_records(status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_sandbox_records_cleanup ON sandbox_records(cleanup_status, updated_at DESC);
`,
	},
	{
		Version: 9,
		SQL: `
CREATE TABLE IF NOT EXISTS artifact_records (
	artifact_id TEXT PRIMARY KEY,
	session_id TEXT NOT NULL,
	run_id TEXT NOT NULL,
	task_id TEXT NOT NULL DEFAULT '',
	type TEXT NOT NULL,
	title TEXT NOT NULL,
	path TEXT NOT NULL DEFAULT '',
	revision INTEGER NOT NULL DEFAULT 1,
	review_state TEXT NOT NULL DEFAULT 'draft',
	preview TEXT NOT NULL DEFAULT '',
	metadata_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_artifacts_session ON artifact_records(session_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_artifacts_run ON artifact_records(run_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_artifacts_task ON artifact_records(task_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS upload_records (
	upload_id TEXT PRIMARY KEY,
	session_id TEXT NOT NULL,
	run_id TEXT NOT NULL,
	task_id TEXT NOT NULL DEFAULT '',
	filename TEXT NOT NULL,
	path TEXT NOT NULL,
	size_bytes INTEGER NOT NULL,
	content_type TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'ready',
	metadata_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_uploads_session ON upload_records(session_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_uploads_run ON upload_records(run_id, created_at DESC);
`,
	},
	{
		Version: 10,
		SQL: `
CREATE TABLE IF NOT EXISTS chat_threads (
	chat_id TEXT PRIMARY KEY,
	title TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'active',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	metadata_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_chat_threads_updated ON chat_threads(updated_at DESC);

CREATE TABLE IF NOT EXISTS chat_messages (
	message_id TEXT PRIMARY KEY,
	chat_id TEXT NOT NULL,
	role TEXT NOT NULL,
	content TEXT NOT NULL,
	run_id TEXT NOT NULL DEFAULT '',
	session_id TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	metadata_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_chat_messages_chat ON chat_messages(chat_id, created_at);
CREATE INDEX IF NOT EXISTS idx_chat_messages_run ON chat_messages(run_id);
`,
	},
	{
		Version: 11,
		SQL: `
ALTER TABLE run_sessions ADD COLUMN execution_state TEXT NOT NULL DEFAULT 'idle';
ALTER TABLE run_tasks ADD COLUMN blocked_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE run_tasks ADD COLUMN selected_context_snapshot_id TEXT NOT NULL DEFAULT '';
`,
	},
	{
		Version: 12,
		SQL: `
CREATE TABLE IF NOT EXISTS tool_call_records (
	tool_call_id TEXT PRIMARY KEY,
	session_id TEXT NOT NULL,
	run_id TEXT NOT NULL,
	task_id TEXT NOT NULL DEFAULT '',
	tool_id TEXT NOT NULL,
	status TEXT NOT NULL,
	risk TEXT NOT NULL DEFAULT '',
	input_preview TEXT NOT NULL DEFAULT '',
	output_preview TEXT NOT NULL DEFAULT '',
	artifact_refs_json TEXT NOT NULL DEFAULT '[]',
	approval_id TEXT NOT NULL DEFAULT '',
	error TEXT NOT NULL DEFAULT '',
	redactions_json TEXT NOT NULL DEFAULT '[]',
	metadata_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	completed_at TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_tool_calls_session ON tool_call_records(session_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_tool_calls_run ON tool_call_records(run_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_tool_calls_task ON tool_call_records(task_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_tool_calls_status ON tool_call_records(status, updated_at DESC);
`,
	},
	{
		Version: 13,
		SQL: `
CREATE TABLE IF NOT EXISTS memory_proposals (
	proposal_id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	session_id TEXT NOT NULL,
	run_id TEXT NOT NULL,
	source_type TEXT NOT NULL,
	title TEXT NOT NULL,
	body TEXT NOT NULL,
	status TEXT NOT NULL,
	context_id TEXT NOT NULL DEFAULT '',
	artifact_refs_json TEXT NOT NULL DEFAULT '[]',
	metadata_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_memory_proposals_status ON memory_proposals(status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_memory_proposals_session ON memory_proposals(session_id, updated_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_memory_proposals_run_source ON memory_proposals(run_id, source_type);
`,
	},
	{
		Version: 14,
		SQL: `
CREATE TABLE IF NOT EXISTS blocked_actions (
	blocked_action_id TEXT PRIMARY KEY,
	session_id TEXT NOT NULL,
	run_id TEXT NOT NULL,
	task_id TEXT NOT NULL DEFAULT '',
	kind TEXT NOT NULL,
	status TEXT NOT NULL,
	title TEXT NOT NULL,
	body TEXT NOT NULL DEFAULT '',
	required_action TEXT NOT NULL DEFAULT '',
	resume_target_task_id TEXT NOT NULL DEFAULT '',
	approval_id TEXT NOT NULL DEFAULT '',
	artifact_id TEXT NOT NULL DEFAULT '',
	tool_call_id TEXT NOT NULL DEFAULT '',
	metadata_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	resolved_at TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_blocked_actions_session ON blocked_actions(session_id, status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_blocked_actions_run ON blocked_actions(run_id, status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_blocked_actions_approval ON blocked_actions(approval_id);
CREATE INDEX IF NOT EXISTS idx_blocked_actions_artifact ON blocked_actions(artifact_id);
`,
	},
	{
		Version: 15,
		SQL: `
CREATE TABLE IF NOT EXISTS workspace_locks (
	lock_id TEXT PRIMARY KEY,
	run_id TEXT NOT NULL,
	task_id TEXT NOT NULL DEFAULT '',
	tool_call_id TEXT NOT NULL DEFAULT '',
	resource TEXT NOT NULL,
	mode TEXT NOT NULL DEFAULT 'exclusive',
	status TEXT NOT NULL,
	owner TEXT NOT NULL DEFAULT '',
	metadata_json TEXT NOT NULL DEFAULT '{}',
	acquired_at TEXT NOT NULL,
	released_at TEXT NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_workspace_locks_held ON workspace_locks(run_id, resource) WHERE status = 'held';
CREATE INDEX IF NOT EXISTS idx_workspace_locks_run ON workspace_locks(run_id, status, acquired_at DESC);
CREATE INDEX IF NOT EXISTS idx_workspace_locks_tool_call ON workspace_locks(tool_call_id);
`,
	},
	{
		Version: 16,
		SQL: `
CREATE TABLE IF NOT EXISTS artifact_revision_records (
	revision_id TEXT PRIMARY KEY,
	artifact_id TEXT NOT NULL,
	session_id TEXT NOT NULL,
	run_id TEXT NOT NULL,
	task_id TEXT NOT NULL DEFAULT '',
	revision INTEGER NOT NULL,
	review_state TEXT NOT NULL,
	path TEXT NOT NULL DEFAULT '',
	preview TEXT NOT NULL DEFAULT '',
	diff_preview TEXT NOT NULL DEFAULT '',
	metadata_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_artifact_revisions_artifact ON artifact_revision_records(artifact_id, revision DESC);
CREATE INDEX IF NOT EXISTS idx_artifact_revisions_session ON artifact_revision_records(session_id, created_at DESC);
`,
	},
	{
		Version: 17,
		SQL: `
CREATE TABLE IF NOT EXISTS chat_feedback (
	feedback_id TEXT PRIMARY KEY,
	chat_id TEXT NOT NULL,
	message_id TEXT NOT NULL,
	score TEXT NOT NULL,
	note TEXT NOT NULL DEFAULT '',
	metadata_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_chat_feedback_message ON chat_feedback(message_id);
CREATE INDEX IF NOT EXISTS idx_chat_feedback_chat ON chat_feedback(chat_id, updated_at DESC);
`,
	},
}

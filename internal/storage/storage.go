// Package storage implements OmG's v1 Core DB.
//
// The schema follows `.omx/plans/omg-v1-core-db.dbml`: Run, WorkItem,
// AgentSession, ContextPacket, PermissionPolicy, ToolCall, Evidence,
// VerificationResult, FailureDiagnosis, Event Ledger, Mailbox,
// ApprovalRequest, MemoryCandidate, and Report. The default DB is repo-local
// `.omg/omg.db`.
package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type EventType string

const (
	EventRunCreated           EventType = "run_created"
	EventRunStatusChanged     EventType = "run_status_changed"
	EventWorkCreated          EventType = "work_created"
	EventWorkClaimed          EventType = "work_claimed"
	EventWorkStarted          EventType = "work_started"
	EventWorkCompleted        EventType = "work_completed"
	EventWorkVerified         EventType = "work_verified"
	EventWorkFailed           EventType = "work_failed"
	EventWorkBlocked          EventType = "work_blocked"
	EventWorkRepackaged       EventType = "work_repackaged"
	EventAgentSpawned         EventType = "agent_spawned"
	EventAgentHeartbeat       EventType = "agent_heartbeat"
	EventAgentStopped         EventType = "agent_stopped"
	EventAgentFailed          EventType = "agent_failed"
	EventContextPacketCreated EventType = "context_packet_created"
	EventEvidenceCreated      EventType = "evidence_created"
	EventToolCallStarted      EventType = "tool_call_started"
	EventToolCallCompleted    EventType = "tool_call_completed"
	EventToolCallFailed       EventType = "tool_call_failed"
	EventToolCallBlocked      EventType = "tool_call_blocked"
	EventVerificationStarted  EventType = "verification_started"
	EventVerificationPassed   EventType = "verification_passed"
	EventVerificationFailed   EventType = "verification_failed"
	EventApprovalRequested    EventType = "approval_requested"
	EventApprovalGranted      EventType = "approval_granted"
	EventApprovalDenied       EventType = "approval_denied"
	EventReportCreated        EventType = "report_created"
	EventFailureDiagnosed     EventType = "failure_diagnosed"
)

type Event struct {
	ID             string
	Type           EventType
	RunID          string
	WorkID         string
	AgentSessionID string
	EvidenceID     string
	ToolCallID     string
	VerificationID string
	RequestID      string
	Provider       string
	HostEvent      string
	ActorType      string
	ActorID        string
	Status         string
	Lifecycle      string
	ProjectRoot    string
	Attributes     map[string]string
	Timestamp      time.Time
}

type Sink interface {
	RecordEvent(context.Context, Event) error
}

type Recorder struct {
	Sink  Sink
	Now   func() time.Time
	NewID func(Event) string
}

func (r Recorder) Record(ctx context.Context, event Event) error {
	if r.Sink == nil {
		return nil
	}
	event.Type = normalizeType(event.Type)
	if event.Type == "" {
		return fmt.Errorf("storage event type is required")
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = r.now()
	}
	if event.ID == "" && r.NewID != nil {
		event.ID = r.NewID(event)
	}
	return r.Sink.RecordEvent(ctx, event)
}

func (r Recorder) now() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}

func normalizeType(eventType EventType) EventType {
	return EventType(strings.TrimSpace(string(eventType)))
}

type Options struct {
	DBPath string
}

type Store struct {
	db          *sql.DB
	path        string
	projectRoot string
}

func DefaultDBPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".omg", "omg.db")
}

func OpenStore(ctx context.Context, projectRoot string, options Options) (*Store, error) {
	path := options.DBPath
	if path == "" {
		path = DefaultDBPath(projectRoot)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	store := &Store{db: db, path: path, projectRoot: projectRoot}
	if err := store.Migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Path() string { return s.path }

func (s *Store) Shutdown(ctx context.Context) error {
	_ = ctx
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Migrate(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage store is nil")
	}
	stmts := append([]string{
		`PRAGMA foreign_keys = ON`,
	}, schemaStatements...)
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate: %w\nstatement: %s", err, stmt)
		}
	}
	return nil
}

func (s *Store) RecordEvent(ctx context.Context, event Event) error {
	if s == nil || s.db == nil {
		return nil
	}
	event.Type = normalizeType(event.Type)
	if event.Type == "" {
		return fmt.Errorf("event type is required")
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	if event.RunID == "" {
		event.RunID = firstNonEmpty(event.RequestID, deterministicID("run", string(event.Type), event.ProjectRoot, event.Timestamp.Format(time.RFC3339Nano)))
	}
	if event.ID == "" {
		event.ID = deterministicID("event", event.RunID, string(event.Type), event.RequestID, event.Timestamp.Format(time.RFC3339Nano))
	}
	if event.ProjectRoot == "" {
		event.ProjectRoot = s.projectRoot
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.ensureRepo(ctx, tx, event); err != nil {
		return err
	}
	if err := s.ensureRun(ctx, tx, event); err != nil {
		return err
	}
	if event.WorkID != "" {
		if err := s.ensureWork(ctx, tx, event); err != nil {
			return err
		}
	}
	if event.AgentSessionID != "" {
		if err := s.ensureAgentSession(ctx, tx, event); err != nil {
			return err
		}
	}
	if err := s.recordSpecializedRow(ctx, tx, event); err != nil {
		return err
	}
	if err := s.insertEvent(ctx, tx, event); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ensureRepo(ctx context.Context, tx *sql.Tx, event Event) error {
	repoID := repoID(event.ProjectRoot)
	now := sqlTime(event.Timestamp)
	_, err := tx.ExecContext(ctx, `
INSERT INTO repos (id, root_path, repo_hash, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET root_path=excluded.root_path, updated_at=excluded.updated_at`, repoID, event.ProjectRoot, repoID, now, now)
	return err
}

func (s *Store) ensureRun(ctx context.Context, tx *sql.Tx, event Event) error {
	status := runStatus(event)
	now := sqlTime(event.Timestamp)
	insertStatus := status
	if insertStatus == "" {
		insertStatus = "scoped"
	}
	if status == "" {
		_, err := tx.ExecContext(ctx, `
INSERT INTO runs (id, repo_id, objective, status, current_phase, trace_id, created_by, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET updated_at=excluded.updated_at`,
			event.RunID, repoID(event.ProjectRoot), firstNonEmpty(event.Attributes["objective"], event.RequestID, string(event.Type)), insertStatus, event.HostEvent, event.Attributes["trace_id"], event.ActorID, now, now)
		return err
	}
	_, err := tx.ExecContext(ctx, `
INSERT INTO runs (id, repo_id, objective, status, current_phase, trace_id, created_by, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET status=excluded.status, current_phase=excluded.current_phase, updated_at=excluded.updated_at`,
		event.RunID, repoID(event.ProjectRoot), firstNonEmpty(event.Attributes["objective"], event.RequestID, string(event.Type)), status, event.HostEvent, event.Attributes["trace_id"], event.ActorID, now, now)
	return err
}

func (s *Store) ensureWork(ctx context.Context, tx *sql.Tx, event Event) error {
	status := workStatus(event)
	if status == "" {
		return nil
	}
	now := sqlTime(event.Timestamp)
	_, err := tx.ExecContext(ctx, `
INSERT INTO work_items (id, run_id, role, status, title, goal, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET status=excluded.status, updated_at=excluded.updated_at`,
		event.WorkID, event.RunID, firstNonEmpty(event.Attributes["role"], "executor"), status, firstNonEmpty(event.Attributes["title"], event.WorkID), firstNonEmpty(event.Attributes["goal"], string(event.Type)), now, now)
	return err
}

func (s *Store) ensureAgentSession(ctx context.Context, tx *sql.Tx, event Event) error {
	now := sqlTime(event.Timestamp)
	_, err := tx.ExecContext(ctx, `
INSERT INTO agent_sessions (id, run_id, role, status, backend_kind, backend_model, backend_session_ref, current_work_id, spawned_at, heartbeat_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET status=excluded.status, current_work_id=COALESCE(excluded.current_work_id, agent_sessions.current_work_id), heartbeat_at=excluded.heartbeat_at`,
		event.AgentSessionID, event.RunID, firstNonEmpty(event.Attributes["role"], "executor"), agentStatus(event), event.Provider, event.Attributes["backend_model"], event.Attributes["backend_session_ref"], nullable(event.WorkID), now, now)
	return err
}

func (s *Store) recordSpecializedRow(ctx context.Context, tx *sql.Tx, event Event) error {
	switch event.Type {
	case EventToolCallStarted, EventToolCallCompleted, EventToolCallFailed, EventToolCallBlocked:
		return s.upsertToolCall(ctx, tx, event)
	case EventVerificationStarted, EventVerificationPassed, EventVerificationFailed:
		return s.upsertVerification(ctx, tx, event)
	case EventEvidenceCreated:
		return s.upsertEvidence(ctx, tx, event)
	case EventFailureDiagnosed:
		return s.upsertFailureDiagnosis(ctx, tx, event)
	}
	return nil
}

func (s *Store) upsertToolCall(ctx context.Context, tx *sql.Tx, event Event) error {
	id := firstNonEmpty(event.ToolCallID, deterministicID("tool", event.ID))
	status := toolStatus(event)
	now := sqlTime(event.Timestamp)
	_, err := tx.ExecContext(ctx, `
INSERT INTO tool_calls (id, run_id, work_id, agent_session_id, tool_name, status, input_summary, output_summary, blocked_reason, error_message, started_at, completed_at, trace_id, span_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET status=excluded.status, output_summary=excluded.output_summary, blocked_reason=excluded.blocked_reason, error_message=excluded.error_message, completed_at=excluded.completed_at`,
		id, event.RunID, nullable(event.WorkID), nullable(event.AgentSessionID), firstNonEmpty(event.Attributes["tool"], "unknown"), status, event.Attributes["input_summary"], event.Attributes["output_summary"], event.Attributes["blocked_reason"], event.Attributes["error_message"], now, completedAt(event), event.Attributes["trace_id"], event.Attributes["span_id"])
	if err == nil && event.ToolCallID == "" {
		event.ToolCallID = id
	}
	return err
}

func (s *Store) upsertEvidence(ctx context.Context, tx *sql.Tx, event Event) error {
	id := firstNonEmpty(event.EvidenceID, deterministicID("evidence", event.ID))
	kind := mapVerifyEvidenceKind(firstNonEmpty(event.Attributes["kind"], "other"))
	claim := firstNonEmpty(event.Attributes["claim"], event.Attributes["summary"], string(event.Type))
	_, err := tx.ExecContext(ctx, `
INSERT INTO evidence (id, run_id, work_id, agent_session_id, kind, claim, source_kind, source_path, command, exit_code, artifact_path, observed_at, freshness)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET claim=excluded.claim, freshness=excluded.freshness`,
		id, event.RunID, nullable(event.WorkID), nullable(event.AgentSessionID), kind, claim, event.Attributes["source_kind"], event.Attributes["source_path"], event.Attributes["command"], nullableInt(event.Attributes["exit_code"]), event.Attributes["artifact_path"], sqlTime(event.Timestamp), event.Attributes["freshness"])
	return err
}

func (s *Store) upsertVerification(ctx context.Context, tx *sql.Tx, event Event) error {
	workID := event.WorkID
	if workID == "" {
		workID = deterministicID("work", event.RunID, "verification")
		event.WorkID = workID
		if err := s.ensureWork(ctx, tx, event); err != nil {
			return err
		}
	}
	id := firstNonEmpty(event.VerificationID, deterministicID("verification", event.ID))
	_, err := tx.ExecContext(ctx, `
INSERT INTO verification_results (id, run_id, work_id, status, verification_policy, summary, created_at, created_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET status=excluded.status, summary=excluded.summary`,
		id, event.RunID, workID, verificationStatus(event), event.Attributes["verification_policy"], event.Attributes["summary"], sqlTime(event.Timestamp), event.ActorID)
	return err
}

func (s *Store) upsertFailureDiagnosis(ctx context.Context, tx *sql.Tx, event Event) error {
	workID := event.WorkID
	if workID == "" {
		workID = deterministicID("work", event.RunID, "failure")
		event.WorkID = workID
		if err := s.ensureWork(ctx, tx, event); err != nil {
			return err
		}
	}
	id := deterministicID("diagnosis", event.ID)
	_, err := tx.ExecContext(ctx, `
INSERT INTO failure_diagnoses (id, run_id, work_id, agent_session_id, verification_id, type, reason, recommended_repackage, dem_trace_ref, created_at, created_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET reason=excluded.reason`,
		id, event.RunID, workID, nullable(event.AgentSessionID), nullable(event.VerificationID), firstNonEmpty(event.Attributes["failure_type"], "unknown"), firstNonEmpty(event.Attributes["reason"], event.Attributes["summary"], string(event.Type)), event.Attributes["recommended_repackage"], event.Attributes["dem_trace_ref"], sqlTime(event.Timestamp), event.ActorID)
	return err
}

func (s *Store) insertEvent(ctx context.Context, tx *sql.Tx, event Event) error {
	payload, err := json.Marshal(event.Attributes)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO events (id, run_id, type, actor_type, actor_id, work_id, agent_session_id, evidence_id, tool_call_id, verification_id, payload_json, trace_id, span_id, git_commit, git_tree_hash, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO NOTHING`,
		event.ID, event.RunID, string(event.Type), event.ActorType, event.ActorID, nullable(event.WorkID), nullable(event.AgentSessionID), nullable(event.EvidenceID), nullable(event.ToolCallID), nullable(event.VerificationID), string(payload), event.Attributes["trace_id"], event.Attributes["span_id"], event.Attributes["git_commit"], event.Attributes["git_tree_hash"], sqlTime(event.Timestamp))
	return err
}

func (s *Store) Count(ctx context.Context, table string) (int, error) {
	if !knownTable(table) {
		return 0, fmt.Errorf("unknown table %q", table)
	}
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM "+table).Scan(&count)
	return count, err
}

func (s *Store) HasTable(ctx context.Context, table string) (bool, error) {
	var name string
	err := s.db.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func repoID(root string) string { return deterministicID("repo", root) }

// mapVerifyEvidenceKind translates the verify package's internal EvidenceKind
// strings ("command"/"manual"/"artifact") into the values permitted by the
// evidence table's CHECK constraint. Unknown inputs (including the schema's
// own canonical values, which pass through) fall back to "other" only if
// they are not already in the allow-list. This keeps the verify package
// decoupled from this schema enum.
func mapVerifyEvidenceKind(verifyKind string) string {
	switch verifyKind {
	case "command":
		return "terminal_log"
	case "manual":
		return "approval"
	case "artifact":
		return "report"
	case "file_anchor", "git_diff", "test_result", "build_log",
		"terminal_log", "screenshot", "approval", "report", "other":
		return verifyKind
	default:
		return "other"
	}
}

func deterministicID(parts ...string) string {
	h := fnv.New64a()
	for _, part := range parts {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return fmt.Sprintf("%s_%x", safePrefix(parts[0]), h.Sum64())
}

func safePrefix(prefix string) string {
	prefix = strings.Trim(prefix, "_")
	if prefix == "" {
		return "id"
	}
	return prefix
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func sqlTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableInt(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func completedAt(event Event) any {
	switch event.Type {
	case EventToolCallCompleted, EventToolCallFailed, EventToolCallBlocked:
		return sqlTime(event.Timestamp)
	default:
		return nil
	}
}

func runStatus(event Event) string {
	if event.Lifecycle != "" {
		switch event.Lifecycle {
		case "running":
			return "executing"
		case "ask_user":
			return "blocked"
		case "finished":
			return "completed"
		default:
			return event.Lifecycle
		}
	}
	switch event.Type {
	case EventRunCreated:
		return "scoped"
	case EventAgentSpawned, EventWorkStarted, EventToolCallStarted, EventToolCallCompleted, EventWorkCompleted:
		return "executing"
	case EventVerificationStarted:
		return "verifying"
	case EventVerificationPassed:
		return "completed"
	case EventVerificationFailed, EventToolCallFailed, EventWorkFailed, EventAgentFailed:
		return "failed"
	case EventToolCallBlocked:
		return "blocked"
	default:
		return ""
	}
}

func workStatus(event Event) string {
	switch event.Type {
	case EventWorkCreated:
		return "ready"
	case EventWorkStarted, EventToolCallStarted, EventToolCallCompleted:
		return "running"
	case EventWorkCompleted:
		return "completed"
	case EventVerificationPassed:
		return "verified"
	case EventWorkFailed, EventVerificationFailed, EventToolCallFailed:
		return "failed"
	case EventToolCallBlocked:
		return "blocked"
	default:
		return ""
	}
}

func agentStatus(event Event) string {
	switch event.Type {
	case EventAgentHeartbeat:
		return "running"
	case EventAgentStopped:
		return "stopped"
	case EventAgentFailed:
		return "crashed"
	}
	if event.Status == "failed" {
		return "crashed"
	}
	return "running"
}

func toolStatus(event Event) string {
	switch event.Type {
	case EventToolCallStarted:
		if event.Status == "allowed" || event.Status == "blocked" || event.Status == "failed" {
			return event.Status
		}
		return "started"
	case EventToolCallCompleted:
		return "completed"
	case EventToolCallFailed:
		return "failed"
	case EventToolCallBlocked:
		return "blocked"
	default:
		return "started"
	}
}

func verificationStatus(event Event) string {
	switch event.Type {
	case EventVerificationStarted:
		return "running"
	case EventVerificationPassed:
		return "passed"
	case EventVerificationFailed:
		return "failed"
	default:
		return firstNonEmpty(event.Status, "pending")
	}
}

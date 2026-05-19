# DEC-010 — OmG v1 Core DB implemented as DBML-shaped repo-local SQLite

Date: 2026-05-16
Status: Accepted

## Decision

Implement `.omx/plans/omg-v1-core-db.dbml` as the concrete OmG v1 Core DB shape instead of reducing it to a trace-only event kernel.

The repo-local default database is:

- `.omg/omg.db`

## Correction

The previous implementation interpreted the storage direction as `storage.Event -> OpenTelemetry span -> VictoriaTraces` only. That was insufficient because it did not implement the DBML tables or the user-approved object structure directly.

This decision supersedes the trace-only parts of DEC-009 implementation notes.

## Implemented structure

The SQLite migration now creates DBML-shaped tables for:

- `repos`
- `runs`
- `work_items`
- `work_dependencies`
- `work_scopes`
- `work_exit_criteria`
- `work_required_outputs`
- `work_claims`
- `agent_sessions`
- `permission_policies`
- `permission_tools`
- `permission_paths`
- `approval_rules`
- `context_packets`
- `context_packet_evidence`
- `context_packet_memory`
- `evidence`
- `evidence_stale_rules`
- `tool_calls`
- `verification_results`
- `verification_evidence`
- `failure_diagnoses`
- `work_repackages`
- `events`
- `messages`
- `message_evidence`
- `approval_requests`
- `memory_candidates`
- `memory_candidate_evidence`
- `reports`
- `report_evidence`

## Runtime behavior

- `cmd/omg daemon start` opens/migrates the repo-local DB and injects it as the orchestrator storage sink.
- `internal/orchestrator` records run/tool/state events into the DB-backed Event Ledger and related specialized tables.
- `internal/verify` records verification/evidence events into the DB-backed Event Ledger and related specialized tables.
- Existing `.omg/state` snapshots remain separate compatibility state, not the v1 Core DB.

## Rejected

- JSONL source-of-truth.
- Trace-only storage that does not create DBML tables.
- PostgreSQL-first assumption.

# DEC-008 — OmG v1 Core DB provisional baseline

Date: 2026-05-15
Status: Provisional
Implemented by: DEC-010 as repo-local DBML-shaped SQLite

## Decision

Use `.omx/plans/omg-v1-core-db.dbml` as the provisional OmG v1 Core DB baseline.

This baseline centers OmG on:

- Run
- WorkItem / Work Graph
- AgentSession
- Context Packet
- Permission Policy
- ToolCall
- Evidence
- VerificationResult
- FailureDiagnosis / Repackage loop
- Event Ledger
- Mailbox
- ApprovalRequest
- MemoryCandidate
- Report

## Product meaning

OmG is treated as a **Run-centered main AI development host**.

The schema encodes the current core philosophy:

> Agent failure is usually a packaging failure before it is an intelligence failure.

Therefore the DB must preserve enough structure to diagnose whether failure came from:

- bad WorkItem packaging
- insufficient context
- wrong role
- missing permission
- unclear exit criteria
- tool/backend failure
- model failure
- real external blocker

## Provisional boundaries

This is a design baseline, not yet a migration contract.

Before implementation, review:

1. Event Ledger is implemented as the `events` table in `.omg/omg.db`.
2. DBML objects are implemented as SQLite tables with v1-compatible foreign keys and checks.
3. Repo-local `.omg` runtime/storage constraints are preserved without PostgreSQL.
4. Query paths required for run replay, active queue, agent heartbeat, and evidence lookup remain follow-up work.
5. DeM boundary for failure diagnosis remains separate; OmG records `failure_diagnoses`.

## Immediate use

Use this DBML as the shared vocabulary for upcoming OmG v1 planning.

Do not expand the architecture further until the v1 kernel is cut from this schema.

# Ralph Context Snapshot — OmG v1 VictoriaTraces Storage

## Task statement
Implement OmG v1 storage in a VictoriaTraces-backed DB direction instead of JSONL, using DEC-009. Do not paste the full DBML as migrations; cut a v1 kernel. Read existing internal/state, internal/protocol, internal/orchestrator, internal/verify, internal/config, internal/daemon, and .omg usage first.

## Desired outcome
A minimal, tested v1 storage kernel that records Run-centered semantic events/traces through a VictoriaTraces-oriented abstraction and integrates with existing code paths without imposing PostgreSQL or JSONL source-of-truth.

## Known facts/evidence
- User explicitly rejected JSONL and approved VictoriaTraces-backed DB direction.
- DEC-009 added as accepted decision.
- DEC-008 remains provisional vocabulary baseline.
- Existing code uses repo-local .omg for config/state and JSON state snapshots.

## Constraints
- Use only user-approved architecture; do not invent extra layers.
- MCP is tool bridge, not agent communication.
- Agent communication remains Work Queue / Mailbox / Event Ledger / Control Board.
- Avoid external DB assumptions like PostgreSQL.
- Do not blindly convert DBML into migrations.
- Fit existing package layout.

## Unknowns/open questions
- Exact VictoriaTraces API surface available to local tests; likely need an interface/client boundary rather than live service dependency.
- Whether repo currently uses OpenTelemetry dependencies.
- How much orchestrator integration is sufficient for v1 kernel.

## Likely codebase touchpoints
- internal/state
- internal/protocol
- internal/orchestrator
- internal/verify
- internal/config
- internal/daemon
- new minimal storage/traces package if existing layout lacks one

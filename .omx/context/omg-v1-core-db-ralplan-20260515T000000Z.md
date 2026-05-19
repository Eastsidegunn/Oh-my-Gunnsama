# Context Snapshot — OmG v1 Core DB Ralplan

## Task statement
Run `$ralplan` then `$ralph` for OmG v1 Core DB work, using only what the user explicitly accepted. Do not add architecture beyond user-approved decisions.

## Desired outcome
Create a plan that fits the existing codebase and then execute it via Ralph. The implementation should use repo-local `.omg` storage rather than requiring an external DB connection.

## User-approved decisions
- OmG is a Run-centered main AI development host.
- OmG is not a Claude/Codex/OpenCode plugin, prompt helper, or hook-only harness.
- Claude Code, Codex, OpenCode, shell, files, browser, MCP are execution backends/tools OmG may call.
- Agent failure is usually packaging failure before intelligence failure.
- Core loop: Run -> WorkItem -> Context Packet -> Agent Execution -> Evidence -> Verification -> Failure Diagnosis -> Repackaging.
- WorkItem is a work contract, not just a task.
- Agent = role + backend/model + permission policy + context packet + work item.
- MCP is a tool bridge, not agent-to-agent communication.
- Agent communication should use Work Queue / Mailbox / Event Ledger / Control Board.
- Evidence without grounding must not become trusted memory/final decision.
- Executor completed != Run completed; Verification Gate decides completion.
- Memory Steward v1 should produce candidates, not auto-promote.
- Event Ledger is semantic source of truth; Control Board is projection.
- Use `.omx/plans/omg-v1-core-db.dbml` as provisional v1 Core DB baseline.
- Use repo-local `.omg` per repo for storage; external DB connection is likely too hard for v1.

## Known facts/evidence
- Provisional DBML saved at `.omx/plans/omg-v1-core-db.dbml`.
- Decision note saved at `.omx/plans/decisions/008-omg-v1-core-db-provisional.md`.
- Existing code packages include internal/protocol, config, registry, route, prompt, guard, state, skills, mcp, verify, daemon, orchestrator.
- Existing `.omg/` directory already exists in repo.

## Constraints
- Do not implement ideas not approved by user.
- Existing codebase must be read and changes must fit current structure.
- Prefer repo-local `.omg` storage.
- DBML is provisional baseline, not a blind migration contract.
- Need planning before Ralph execution.

## Unknowns/open questions
- Exact v1 slice to implement first from DBML.
- Whether storage should be SQLite under `.omg/`, JSONL + projection, or both for first slice.
- Which existing internal packages should be extended vs new internal package added.
- How to reconcile existing state package with new Run/Event Ledger model.

## Likely codebase touchpoints
- `.omx/plans/omg-v1-core-db.dbml`
- `.omx/plans/decisions/008-omg-v1-core-db-provisional.md`
- `internal/state`
- `internal/orchestrator`
- `internal/protocol`
- `internal/verify`
- `internal/guard`
- `cmd/omg`
- `.omg/`

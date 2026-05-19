# DEC-009 — VictoriaTraces-backed OmG event DB

Date: 2026-05-15
Status: Superseded by DEC-010 for concrete DB implementation

## Decision

Use VictoriaTraces as the approved DB direction for OmG v1 event/trace storage.

This supersedes the earlier JSONL-first storage decision in `002-event-log-storage.md` for OmG v1 implementation planning.

## User-approved constraint

Do not use JSONL as the v1 storage source of truth.

OmG v1 should move toward a DB-backed event/trace store, with VictoriaTraces as the named backend to evaluate and integrate.

## Product fit

OmG is a Run-centered main AI development host. Its storage must preserve enough execution trace to diagnose whether failures are caused by packaging, context, permission, tool/backend behavior, model behavior, or real external blockers.

VictoriaTraces fits this direction as an execution trace/event backend rather than as an agent-to-agent communication channel.

MCP remains a tool bridge. Agent communication remains Work Queue / Mailbox / Event Ledger / Control Board.

## Implementation boundary

Before coding against VictoriaTraces, inspect the existing code paths first:

- `internal/state`
- `internal/protocol`
- `internal/orchestrator`
- `internal/verify`
- `internal/config`
- `internal/daemon`
- existing `.omg` usage

Do not paste the full DBML into migrations as-is. Cut a v1 kernel first.

## v1 kernel pressure

Start with the smallest Run-centered trace shape needed for:

- Run lifecycle
- WorkItem lifecycle
- AgentSession lifecycle / heartbeat
- ToolCall lifecycle
- Evidence records
- VerificationResult records
- FailureDiagnosis records
- Event Ledger query/replay needs

## Rejected / superseded

- JSONL source-of-truth for OmG v1 storage: superseded by the user-approved VictoriaTraces DB direction.
- Immediate PostgreSQL assumption: still rejected unless explicitly approved later.
- Arbitrary new architecture beyond the approved core objects: rejected.

## References

- VictoriaTraces: https://github.com/VictoriaMetrics/VictoriaTraces
- VictoriaTraces docs: https://docs.victoriametrics.com/victoriatraces/

## Implementation slice — 2026-05-15

The first v1 kernel is implemented as `internal/storage`:

- `storage.Event` is the DBML vocabulary cut for Run/Work/Agent/Tool/Evidence/Verification events.
- `storage.Recorder` writes semantic events through an injected sink, so existing tests and hosts are unaffected when no sink is configured.
- `storage.VictoriaTracesSink` exports each semantic event as an OpenTelemetry span over OTLP/HTTP, targeting VictoriaTraces by default at `http://127.0.0.1:10428/insert/opentelemetry/v1/traces`.
- `storage.VictoriaTracesConfig` keeps VictoriaTraces storage repo-local by default at `.omg/victoria-traces` via `-storageDataPath`.
- `internal/orchestrator` emits kernel events for run creation/scoping, tool-call decisions, tool completion/failure, and state transitions.
- `internal/verify` emits verification result and evidence events when a storage sink is provided.

This does not convert the full DBML into migrations and does not introduce JSONL as source-of-truth.

## Runtime wiring slice — 2026-05-15

VictoriaTraces is now wired into the daemon path as opt-in configuration:

```yaml
storage:
  victoriatraces:
    enabled: true
    endpoint: http://127.0.0.1:10428/insert/opentelemetry/v1/traces
    service_name: omg
    storage_subdir: .omg/victoria-traces
```

The daemon creates a VictoriaTraces sink only when enabled and injects it into `internal/orchestrator`. The sink is shut down when the daemon exits. If storage is not enabled, existing behavior remains no-op.

This still does not make the daemon own the VictoriaTraces process lifecycle; process startup/supervision remains a separate follow-up.

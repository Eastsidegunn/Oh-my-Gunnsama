# DEC-011: Worker adapter contract, pi as v0.1 backend

Date: 2026-05-19
Status: Accepted
Supersedes (in spirit): DEC-007

## Decision

OmG defines a minimal `Worker` interface in `internal/worker/worker.go`:

```go
type Worker interface {
    Spawn(ctx context.Context, task Task) (Session, error)
    Events(ctx context.Context, sessionID string) (<-chan Event, error)
    Wait(ctx context.Context, sessionID string) (Result, error)
    Abort(ctx context.Context, sessionID string) error
}
```

For v0.1 "Hello Run", the only concrete adapter is `PiWorker` in `internal/worker/pi/`. It shells out to `pi --mode json` and reads newline-delimited JSON events from stdout.

Future adapters (Claude CLI, Codex CLI, OmG-inline) implement the same interface. Degraded capabilities are handled per-adapter, not via flags on the interface.

## Why this shape

Depend, not fork. OmG calls pi as a subprocess rather than rewriting its agent logic in Go. This keeps the v0.1 scope small and the pi team's improvements free.

Minimal, not flag-rich. Four methods cover spawn, observe, join, and cancel. Capability negotiation is deferred; adapters that cannot stream events return a closed channel.

pi over a Go port. A Go-native agent would require porting pi's tool-call loop, context management, and model client. That is v0.2+ work at earliest.

## Locked sub-decisions

- `pi --mode json` is the subprocess invocation. Flags are fixed for v0.1.
- Binary discovery: `$PI_BIN` env var, then `PATH` lookup. No embedded binary.
- All pi events are written to the Event Ledger (`events` table in `.omg/omg.db`).
- `EventOnRun` fires when `omg run` starts; `agent_sessions.backend_kind` is set to `"pi"`.

## Non-goals for v0.1

- Tool-gate callback: OmG does not intercept pi tool calls.
- Steering: no mid-run message injection.
- Capability flags on the `Worker` interface.
- Multi-worker scheduling.
- Mailbox, verify, repackage, profile flows.
- A second adapter (Claude CLI, Codex CLI, or OmG-inline).

## Known follow-ups

- Settle `internal/adapter/` vs. `internal/worker/` as the package root before v0.2.
- Define `adapters/` directory layout for third-party adapter registration.
- Recast DEC-007 as superseded; update its status field.

## Acceptance evidence

- `omg run` completes without error on a hello-world task.
- `agent_sessions` row exists with `backend_kind = 'pi'`.
- Event Ledger contains at least 5 events for the run.
- `scripts/qa_hello_run.sh` exits 0.

# PRD: AI Dev Control Room

## Decision
Build an owner-first, productizable AI Dev Control Room inside `oh-my-gunnsama`: a Mac-hosted local web server that serves installable tablet/phone PWA clients, observes a development session and an observer session, streams live HTML/card reports to a Galaxy Tab, and routes independent messages to dev and observer channels.

## Product thesis
Do not remote-control a desktop IDE from a tablet. Let AI do the heavy development work on the Mac, while the human uses tablet/phone surfaces to observe, approve, redirect, and query the work.

## Primary user
The owner, using:
- Mac as host/execution machine.
- Galaxy Tab as live control room/report surface.
- Phone as quick/voice command surface.

The design must remain productizable: clean boundaries, documented protocol, and no hard coupling to one private machine beyond configuration.

## Goals
1. Start a local web control room from the Mac with one command.
2. Show a pairing URL/QR-like connection payload for tablet/phone onboarding.
3. Serve an installable PWA with tablet and phone layouts.
4. Auto-update the tablet dashboard without manual refresh.
5. Maintain separate dev and observer conversation channels.
6. Store events in an append-only local event log.
7. Render observer reports as structured HTML/cards, not Markdown-only text.
8. Keep session bridge replaceable: file queue first, tmux/cmux/codex direct injection later.

## Non-goals for first implementation slice
- Native Android app store distribution.
- Full cmux GUI replication on tablet.
- Production cloud sync.
- Multi-user auth/RBAC.
- Direct autonomous destructive actions from phone without host-side confirmation policy.
- Full infinite canvas as the default UI. A later map/canvas view can consume the same card/event model.

## User stories

### US-001 Host startup
As the owner, I can run a command that starts a local control-room server and prints the tablet/phone URL plus pairing token, so I can connect devices without remembering configuration.

Acceptance criteria:
- CLI command exists, e.g. `omg control-room start`.
- Server binds to a configurable host/port, defaulting to localhost or LAN-safe host by flag.
- Startup prints URL and pairing token.
- Invalid port/config returns a clear error.

### US-002 Installable tablet/phone PWA
As the owner, I can open the URL on Galaxy Tab/phone and install it to the home screen, so it behaves like a companion app.

Acceptance criteria:
- Server serves `index.html`, `manifest.webmanifest`, and a service worker or graceful no-cache shell.
- Layout adapts to tablet and phone widths.
- UI shows connection status and last update time.

### US-003 Live report stream
As the owner, I can leave the tablet open and see observer state update automatically.

Acceptance criteria:
- Browser connects to an SSE endpoint such as `/api/events`.
- New events update cards without manual refresh.
- Reconnect state is visible.
- A polling fallback is possible or documented.

### US-004 Separate dev and observer channels
As the owner, I can send a message to either dev or observer independently.

Acceptance criteria:
- UI has separate `Tell Dev` and `Ask Observer` inputs.
- Server writes each message to distinct channel events/queues.
- API validates target channel as `dev` or `observer`.
- Channel history is visible in the dashboard.

### US-005 Local event log
As the system, I can persist control room activity as JSONL so observer/UI state can replay after reconnect.

Acceptance criteria:
- Append-only JSONL event log under `.omx/control-room/events.jsonl` or `.omg/control-room/events.jsonl`.
- Events include id, time, type, channel, payload.
- Server replays recent events on client connect.
- Corrupt event lines do not crash the server; they produce warnings.

### US-006 Observer report cards
As the owner, I can read the current goal, status, changed files, tests, risks, and next actions as cards.

Acceptance criteria:
- Server exposes a snapshot endpoint, e.g. `/api/snapshot`.
- UI renders cards from structured state.
- Initial snapshot can be seeded manually or from event log.
- Markdown may appear inside a card body but is not the primary transport shape.

## Architecture principles
1. PWA-first companion app, not desktop remoting.
2. Local-first Mac host; Tailscale/LAN are transport options, hidden behind pairing UX.
3. Append-only event log is source of truth; UI state is a projection.
4. Dev and observer are separate channels with separate intent and permissions.
5. Session bridge is an adapter boundary, not embedded in the UI/server core.
6. Owner-first defaults, productizable interfaces.

## Decision drivers
1. Setup friction must be low on tablet/phone.
2. Live update/report readability matters more than full IDE control.
3. Safety: commands should be observable and channel-scoped before direct injection.

## Viable options considered

### Option A: PWA + local Go server + JSONL/SSE (chosen)
Pros: one binary, no app store, easy tablet/phone install, fits current Go repo, simple replay model.
Cons: browser/PWA limitations for native voice and background behavior.

### Option B: Native Android app + Mac daemon
Pros: deeper mobile integration, stronger voice/background features.
Cons: high setup and distribution cost; slower iteration; bad owner-first velocity.

### Option C: Existing remote desktop + overlay reports
Pros: fastest to demo visually.
Cons: keeps desktop UI problem; weak product boundary; not tablet-first.

## ADR
Decision: implement Option A first.
Drivers: installability, local-first privacy, low dependency count, productizable protocol.
Alternatives rejected: native app first and remote desktop overlay.
Consequences: first version is web/PWA constrained but highly shippable; later native wrapper can reuse APIs.
Follow-ups: add QR image generation, voice command surface, canvas/map view, and direct tmux/cmux/codex bridge adapters.

## Available agent-types roster for execution
- executor: implement Go server, APIs, PWA assets, tests.
- test-engineer: design/validate Go HTTP/event-log tests and browser-free UI checks.
- architect: verify boundaries and adapter design before completion.
- code-reviewer: review implementation diff for product boundary regressions.
- verifier: confirm evidence and acceptance criteria.

## Ralph staffing guidance
- Implementation lane: executor, medium reasoning, own `cmd/omg`, `internal/controlroom`, and static assets.
- Evidence lane: test-engineer/verifier, medium/high reasoning, own test strategy and verification commands.
- Sign-off lane: architect, high reasoning, verify architecture and product boundary.

## Suggested first implementation slice
Implement US-001 through US-005 plus minimal US-006 with static/sample report cards. Keep the session bridge as file-backed queues/events, not direct tmux injection yet.

---

## RALPLAN iteration 2: execution contracts

### Canonical runtime namespace
Runtime writes for the product use `{project}/.omg/control-room/`.

- `.omg/control-room/events.jsonl`: append-only source of truth for control-room events.
- `.omg/control-room/queues/dev.jsonl`: file-backed queue for dev-targeted messages.
- `.omg/control-room/queues/observer.jsonl`: file-backed queue for observer-targeted messages.
- `.omg/control-room/session.json`: local host/session metadata, including generated pairing token metadata but not event payload copies of the raw token.

`.omx/*` remains OMX planning/orchestration metadata and must not be used for product runtime control-room state.

### Package and file boundary plan
- `cmd/omg`: add `control-room start` command parsing.
- `internal/controlroom`: core domain types, event schema, channel validation, snapshot projection.
- `internal/controlroom/store`: JSONL event store and replay behavior.
- `internal/controlroom/bridge`: file-backed dev/observer queues; no tmux/cmux/codex direct injection in first slice.
- `internal/controlroom/server`: HTTP handlers, SSE, auth/pairing middleware.
- `internal/controlroom/assets`: embedded PWA assets, or server-owned asset constants if simpler for first slice.

Do not put browser HTTP/SSE concerns into `internal/daemon` or `internal/orchestrator`. The existing daemon stays Unix-socket hook infrastructure.

### CLI contract
Command:

```bash
omg control-room start [--host 127.0.0.1] [--port 8787] [--project <path>] [--token <token>] [--print-json] [--once]
```

Defaults:
- `--host 127.0.0.1`
- `--port 8787`
- `--project` defaults to cwd
- token is generated if omitted

Behavior:
- Prints a connect URL containing token as query param for easy QR/link sharing.
- With `--print-json`, prints machine-readable startup info.
- With `--once`, starts enough to validate config/listener creation and exits; intended for tests/smoke.
- Binding to `0.0.0.0` or a non-loopback host is allowed only when explicitly passed by the user.

### Event schema
All persisted events are JSON lines with:

```json
{
  "schema_version": 1,
  "id": "000000000000000001",
  "time": "2026-05-11T10:00:00Z",
  "type": "message.created",
  "channel": "dev",
  "source": "tablet",
  "session_id": "optional-session-id",
  "correlation_id": "optional-correlation-id",
  "payload": {}
}
```

Valid channels:
- `dev`
- `observer`
- `system`

Valid first-slice event types:
- `system.started`
- `message.created`
- `report.updated`
- `test.updated`
- `risk.updated`
- `file.changed`

IDs are zero-padded monotonic decimal strings assigned by the local store. Time is UTC RFC3339.

### Message queue contract
`POST /api/messages` writes:
1. A `message.created` event to `.omg/control-room/events.jsonl`.
2. A copy of the same message envelope to `.omg/control-room/queues/{dev|observer}.jsonl` when target channel is `dev` or `observer`.

Queue entry fields:
- `id`
- `time`
- `target`
- `source`
- `text`
- `correlation_id`
- `status`: initially `pending`

No direct command injection happens in the first slice. External dev/observer sessions can consume the queue later.

### API contracts
- `GET /`: PWA shell.
- `GET /manifest.webmanifest`: install metadata.
- `GET /sw.js`: service worker; can be minimal no-op cache strategy.
- `GET /api/snapshot`: returns projected dashboard state and recent events.
- `GET /api/events`: SSE stream; supports replay via `Last-Event-ID` header or `?since=<id>`.
- `POST /api/messages`: JSON `{ "target": "dev|observer", "text": "...", "source": "tablet|phone|host" }`.
- `GET /api/health`: simple status.

Error shape:

```json
{
  "ok": false,
  "error": { "code": "invalid_channel", "message": "target must be dev or observer" }
}
```

### Pairing/auth contract
- Startup token is required for mutation endpoints (`POST /api/messages`).
- Token can be supplied by `Authorization: Bearer <token>` or `?token=<token>` for simple PWA pairing.
- Snapshot/events may require token when server is bound to a non-loopback host; first slice may require token for all `/api/*` to keep behavior simple.
- Raw token must not be written to event payloads.
- UI stores token in localStorage after opening a tokenized pairing URL.

### Strengthened security acceptance criteria
- Default bind is loopback only.
- LAN/Tailscale exposure requires explicit `--host`.
- Mutation endpoints reject missing/invalid token.
- Invalid channel is rejected before persistence.
- Pairing token is not persisted in events or queue payloads.
- First slice has no direct tmux/cmux/codex stdin injection.

### First slice final scope
Include:
- US-001 through US-005.
- Minimal US-006 cards from snapshot projection.
- PWA install shell with tablet/phone responsive layouts.

Exclude:
- Native app.
- QR image generation; print URL/token text first, QR later.
- Voice command.
- Canvas/map view.
- Direct tmux/cmux/codex injection.

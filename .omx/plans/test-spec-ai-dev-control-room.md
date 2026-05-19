# Test Spec: AI Dev Control Room

## Completion claim to prove
The repo can start a local AI Dev Control Room server that serves an installable PWA, streams/replays events, persists channel-scoped messages, and renders tablet/phone-friendly report state.

## Unit tests
1. Event log append/replay
   - Appends valid events with stable ids/time/type/channel/payload.
   - Replays events in order.
   - Ignores or warns on corrupt lines without crashing.
2. Channel validation
   - Accepts `dev` and `observer`.
   - Rejects unknown channels.
3. Snapshot projection
   - Builds cards from event history.
   - Handles empty state with useful defaults.
4. Server handlers
   - `GET /` returns HTML.
   - `GET /manifest.webmanifest` returns valid manifest JSON.
   - `GET /api/snapshot` returns JSON snapshot.
   - `POST /api/messages` persists channel-scoped message event.
   - `GET /api/events` supports SSE formatting/replay enough for handler tests.

## Integration/smoke tests
1. `go test ./...` passes.
2. `go run ./cmd/omg control-room start --host 127.0.0.1 --port 0 --once` or equivalent smoke path validates startup without hanging, if implemented.
3. Static PWA assets are embedded/served by tests.

## Manual tablet verification later
1. Start server on Mac.
2. Open URL from Galaxy Tab over LAN/Tailscale.
3. Add to home screen.
4. Confirm cards auto-update after sending dev/observer messages.
5. Confirm phone layout shows quick command UI.

## Risk checks
- No direct command injection in first slice.
- Pairing token is not logged into event payloads unnecessarily.
- Server bind defaults do not accidentally expose broad network access without explicit host flag.

---

## RALPLAN iteration 2 additions

### Namespace tests
- Runtime store defaults to `{project}/.omg/control-room/`.
- No product runtime writes target `.omx/control-room`.

### Event schema tests
- Event ids are monotonic zero-padded strings.
- Events include schema_version, id, time, type, channel, source, session_id, correlation_id, payload.
- Invalid channels/types fail validation before persistence.

### API/auth tests
- `POST /api/messages` without token returns 401/403 and writes no event.
- `POST /api/messages` with invalid channel returns structured `invalid_channel` and writes no event.
- Valid message writes event log and target queue entry.
- Token is not included in event or queue JSON.
- `GET /api/events` emits valid SSE frames with event id and data.
- `GET /api/snapshot` can replay from event log after server recreation.

### CLI tests
- `omg control-room start --once --port 0 --project <tmp>` succeeds.
- Default host is `127.0.0.1`.
- Startup JSON includes URL, host, port, project, and token presence but does not echo token in unsafe fields except connect URL/pairing payload.

### Boundary tests/review checks
- `internal/daemon` and `internal/orchestrator` do not import `internal/controlroom/server`.
- First slice contains no tmux/cmux/codex stdin injection path.

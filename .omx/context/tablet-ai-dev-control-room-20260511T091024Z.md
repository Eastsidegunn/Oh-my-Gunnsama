# Context Snapshot: Tablet AI Dev Control Room

## Task statement
Design a product-grade local-first system where a Mac runs development and observer sessions, while a Galaxy Tab receives an auto-updating tablet-optimized HTML control room. The user can talk separately to the development session and observer session. Phone voice commands are a future/adjacent input channel.

## Desired outcome
A plan suitable for `$ralph` execution: clear architecture, staged deliverables, acceptance criteria, verification path, and handoff guidance for building the first product slice.

## Known facts/evidence
- User wants option 3: built first for their own local use, with productizable boundaries.
- Access pattern: Mac local web server exposed to Galaxy Tab via Tailscale/local network.
- HTML dashboard must auto-refresh/live update, not just static Markdown.
- Two sessions are required: development session and observer session.
- Each session needs separate conversation/input path from tablet UI.
- Observer should produce report-style UI for tablet dimensions.
- Infinite-plane/canvas view is desirable, but likely not the first core surface.
- Existing repo contains Go orchestration/harness modules under internal/* and reference oh-my-codex/oh-my-openagent directories.
- Existing .omx plans mention event log, session lifecycle, observer as single-writer, and DeM/JANUS-style observer concepts.

## Constraints
- Local-first first; avoid external hosted dependency for first slice.
- Tablet is a review/control surface, not a desktop IDE clone.
- Dev session acts; observer session narrates/reviews/summarizes.
- Keep actor/observer boundaries explicit to avoid accidental edits by observer.
- Product-grade architecture, but first implementation should be bounded.
- No destructive operations or external production effects without explicit approval.

## Unknowns/open questions
- Exact bridge mechanism to live Codex/cmux/omx sessions: tmux send-keys, command queue, cmux socket, or OMX runtime hooks.
- Whether first slice should live inside this Go repo or as sibling app/package.
- Preferred frontend stack: simple static HTML/JS, Go templates, React/Vite, or tldraw/React Flow.
- Authentication model for tablet access on local network/Tailscale.

## Likely codebase touchpoints
- cmd/omg/main.go
- internal/daemon/daemon.go
- internal/orchestrator/orchestrator.go
- internal/state/state.go
- internal/protocol/protocol.go
- internal/adapter/*
- .omx/logs/session-history.jsonl
- .omx/logs/turns-*.jsonl
- .omx/state/session.json
- reference/oh-my-codex and reference/oh-my-openagent for runtime/session patterns

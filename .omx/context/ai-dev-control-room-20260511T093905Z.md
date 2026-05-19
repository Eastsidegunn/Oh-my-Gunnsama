# Context Snapshot: AI Dev Control Room

## Task statement
Design and then implement a tablet/mobile-first AI development control room: a local Mac web server serves an auto-updating HTML dashboard to a Galaxy Tab over Tailscale/local network. The system separates a development session from an observer session, and lets the user talk to each session independently.

## Desired outcome
A product-shaped local-first tool, initially for the owner but with productizable boundaries, that lets a tablet receive live observer reports and send commands/questions to dev and observer sessions separately. Markdown-only reports are not enough; the tablet UI should be HTML/card-oriented and eventually support canvas/map-style views.

## Known facts/evidence
- User wants option 3: owner-first implementation with productizable architecture.
- User prefers Mac local web server + tablet browser access.
- HTML must auto-refresh/live update.
- Product is not a small MVP; it should be designed as an AI development control room.
- Desired roles:
  - Dev session: actor that edits code, runs commands/tests, and performs implementation.
  - Observer session: narrator/reviewer that watches the development path, produces reports, evaluates risk, and can be queried separately.
- Phone voice command is desirable as a later/adjacent input surface.
- Existing repo contains OMX runtime/state/session concepts and prior planning notes mentioning observer/event-log directions.

## Constraints
- Start owner-first, but keep boundaries clean for later productization.
- Tablet UI should not emulate a desktop IDE.
- Reports should be HTML/card/state UI, not plain Markdown.
- Dev and observer sessions need independent conversation channels.
- Local-first serving is preferred; Tailscale can provide private remote access.
- Must define verification before implementation.

## Unknowns/open questions
- Exact frontend stack: vanilla HTML/JS, React, Svelte, etc.
- Exact session bridge mechanism: tmux send-keys, Codex/OMX command queues, cmux socket, or file-backed queues first.
- Event store format: append-only JSONL, SQLite, or existing OMX state projections.
- Security/auth model for local web server.
- First target repo/use-case for dogfooding.
- How much of voice command is in v1 vs later.

## Likely codebase touchpoints
- .omx/state and .omx/logs for current state/event surfaces.
- internal/state for lifecycle/state concepts.
- internal/orchestrator, internal/daemon, internal/route for routing/orchestration patterns.
- existing HTML artifacts (codereview.html, omg-flow.html, omo-flow.html, omx-flow.html) as possible style/reference artifacts only.
- reference/oh-my-codex and reference/oh-my-openagent for prior session/event patterns.

## Planning lane
Run ralplan first to produce PRD + test spec + architecture plan. Then hand off to ralph only after the plan has concrete acceptance criteria and verification commands.

## New product constraint: easy tablet/phone install and pairing
- Tablet and phone onboarding must feel like installing/adding a simple companion app, not configuring developer infrastructure.
- Preferred client shape: PWA/web app first, installable to Galaxy Tab/phone home screen via browser.
- Preferred connection shape: Mac local server displays QR code / pairing URL; tablet/phone scans or opens it to pair.
- Tailscale/local network should be hidden behind a simple connection guide and remembered endpoint.
- Client should reconnect automatically and show clear connection status.
- Future native mobile wrapper is possible, but v1 should avoid app-store dependency unless needed.

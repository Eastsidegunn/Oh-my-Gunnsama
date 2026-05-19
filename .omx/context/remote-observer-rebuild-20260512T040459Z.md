# Context Snapshot: Remote Observer Rebuild

## Task statement
Rebuild `remote/` as a deep, isolated module using TDD. The product should be a thin observer wrapper, not a dev-control system: it fixes a development path, reads that path read-only, creates a tablet-friendly HTML/report view, and serves it so tablet/phone can open it.

## Desired outcome
A clean `remote/` module that can run an observer wrapper over an existing development workspace. Existing dev work stays elsewhere. Remote only observes and reports.

## Known facts/evidence
- User says existing development work/session already exists and should not be replaced.
- User wants: fixed path, read/report generation, tablet-openable report.
- User says one more wrapper is enough.
- Current `remote/` prototype is control-room/server/message-queue oriented and likely overbuilt for this core.
- `omg` root must not own this functionality; `remote/` must remain a deep module.

## Constraints
- TDD-first rebuild.
- `remote/` is a standalone Go module.
- Keep `omg` root clean and unaffected.
- Read-only observation: no edits to observed project, no destructive commands, no command injection.
- Tablet-friendly HTML output and local serving are core.
- Commands between user/dev/observer can stay manual; no direct dev session control in this slice.

## Unknowns/open questions
- Exact report sophistication for first slice.
- Whether to keep any existing controlroom code or fully replace it.
- Whether generated report should be static files only, HTTP server only, or both.

## Likely touchpoints
- `remote/go.mod`
- `remote/cmd/remote`
- new `remote/internal/observer`
- optional new `remote/internal/reportserver`
- remove or replace `remote/internal/controlroom` prototype if inconsistent.

## Planning lane
Run ralplan to fix PRD/test-spec/architecture, then ralph TDD implementation.

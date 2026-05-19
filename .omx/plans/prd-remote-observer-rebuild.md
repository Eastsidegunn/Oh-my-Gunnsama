# PRD: Remote Observer Rebuild

## Decision
Rebuild `remote/` as an isolated observer wrapper module. First slice command: `remote observe --project <path> --out <dir> [--serve --host 127.0.0.1 --port 8787]`. It scans a fixed project path read-only, writes a structured report (`state.json`) plus tablet-friendly `index.html`, and optionally serves the report.

## Product principle
Remote does not develop. Remote observes existing development work and turns it into a readable tablet report.

## Goals
1. Keep `remote/` as a standalone Go module, independent from root `omg`.
2. Provide a read-only scanner for a fixed project path.
3. Produce deterministic report state from git status/diff metadata and workspace facts.
4. Render a tablet-friendly HTML report from state.
5. Serve the report locally for tablet/phone access.
6. Use TDD: scanner, renderer, server, CLI tests before/with implementation.

## Non-goals
- Dev session command injection.
- Observer AI calls.
- Message queues.
- Token pairing/auth.
- QR generation.
- Infinite canvas.
- Native mobile app.

## User stories

### US-001 Observe fixed path
As the owner, I can run `remote observe --project /path/to/repo --out /tmp/report --once` and get report files for that project.

Acceptance:
- Requires valid project path.
- Does not write inside the observed project unless `--out` points there explicitly.
- Returns clear error for missing path.

### US-002 Read-only workspace scan
As the observer wrapper, remote gathers workspace facts without modifying the project.

Acceptance:
- Captures project path, timestamp, git availability, branch when available, status summary, diff stat, changed files.
- Uses read-only commands only: `git status --porcelain`, `git branch --show-current`, `git diff --stat`, `git diff --name-only`.
- Works outside git repo with degraded report instead of crashing.

### US-003 Tablet report generation
As the owner, I can open generated HTML on a tablet and quickly understand status.

Acceptance:
- Writes `state.json` and `index.html` under output directory.
- HTML uses large responsive cards.
- Cards include status, branch, changed files, diff summary, risks, and suggested next commands.

### US-004 Local report server
As the owner, I can run with `--serve` and open the report from a browser.

Acceptance:
- Serves `index.html` and `state.json`.
- `--once` generates and exits; `--serve` keeps server running and refreshes report on interval.
- Default bind is `127.0.0.1`; LAN exposure requires explicit host.

## Architecture
- `remote/cmd/remote`: CLI only.
- `remote/internal/observer`: scan model, scanner, risk/suggestion projection.
- `remote/internal/report`: writes `state.json` and `index.html`.
- `remote/internal/reportserver`: static serving + periodic refresh.

## ADR
Decision: replace current control-room/message-queue prototype with observer/report wrapper.
Drivers: user clarified existing dev sessions already exist; wrapper is enough; simpler and safer.
Rejected: keep queue/token control-room core, because it solves command routing before observation/reporting.
Consequences: first useful product is read-only and manually directed; later command routing can wrap around observer output.
Follow-ups: add QR/pairing, AI observer summarization, voice command, and optional dev command clipboard.

## Agent roster
- executor: TDD implementation in `remote/` only.
- test-engineer: test adequacy and edge cases.
- architect: final boundary verification.
- verifier: evidence collection.

---

## RALPLAN iteration 2: execution contracts

### Read-only output contract
`remote observe` must not write inside the observed `--project` tree. `--out` must be outside `--project`; if omitted, it defaults to an OS temp directory such as `/tmp/remote-observer/<project-base>`. If `--out` resolves inside `--project`, the command fails. A future explicit escape hatch may be designed later, but not in this slice.

### CLI mode contract
Command:

```bash
remote observe --project <path> [--out <dir>] [--once] [--serve] [--host 127.0.0.1] [--port 8787] [--refresh 5s]
```

- Default mode is `--once`: generate report files and exit.
- `--once` and `--serve` are mutually exclusive; passing both is an error.
- `--serve` generates once, starts HTTP server, then refreshes report on `--refresh` interval.
- `--refresh` default is `5s`, minimum accepted value is `1s`.
- Default host is `127.0.0.1`; tablet/LAN exposure requires explicit host.

### Safe git runner contract
- Use `exec.CommandContext` only; never `sh -c`, shell interpolation, or command strings.
- Use timeout per git command, default `2s`.
- Always run `git` as `git -C <absoluteProject> ...`.
- Set `GIT_PAGER=cat` and `GIT_TERMINAL_PROMPT=0`.
- Allowed commands only:
  - `git -C <project> rev-parse --is-inside-work-tree`
  - `git -C <project> branch --show-current`
  - `git -C <project> status --porcelain`
  - `git -C <project> diff --stat`
  - `git -C <project> diff --name-only`
- Project paths with spaces or leading dashes must be handled as argv, not shell text.

### `state.json` schema
Top-level fields:

```json
{
  "schema_version": 1,
  "generated_at": "RFC3339 UTC",
  "project_path": "/abs/path",
  "git": {
    "available": true,
    "inside_work_tree": true,
    "branch": "main",
    "dirty": true,
    "status_porcelain": "...",
    "diff_stat": "...",
    "changed_files": ["file.go"]
  },
  "cards": [
    {"kind":"status","title":"Status","body":"...","tone":"calm|warn|danger|neutral"}
  ],
  "risks": ["..."],
  "suggested_commands": [
    {"target":"dev|observer","text":"..."}
  ],
  "warnings": []
}
```

Non-git degraded shape sets `git.available=true` when git binary exists but `inside_work_tree=false`, includes a warning, and still renders status cards. If git binary is missing, `git.available=false` and report still renders.

### Remove command/control leftovers
The rebuild must remove these from the final `remote/` module:
- `remote/internal/controlroom`
- `control-room start` CLI
- token/pairing auth
- `/api/messages`
- message queues
- `.omg/control-room` writes
- SSE/event-log command-control model

The final first-slice runtime writes only the configured report output directory, never `.omg/control-room`.

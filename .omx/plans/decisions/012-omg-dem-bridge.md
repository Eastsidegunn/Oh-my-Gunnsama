# DEC-012: OmG <-> DeM bridge deferred to v0.2+

Date: 2026-05-20
Status: Accepted
Surfaced by: 5-agent post-implementation review (Agent 5 Context Mining, BLOCKING finding)

## Decision

The data bridge between OmG (Run orchestration, SQLite event ledger at `.omg/omg.db`) and DeM (machine-wide session ingestion, VictoriaTraces via `~/.codex/sessions/` and `~/.claude/projects/` watchers) is explicitly out of scope for v0.1.x and deferred to v0.2 or later.

## Why this decision exists

The review surfaced that the conversational framing throughout v0.1 development ("OmG = orchestration / DeM = ingestion / they're companion systems") implies a data integration that does not exist in code:

- OmG writes only to its own SQLite at `.omg/omg.db`. No JSONL output.
- DeM ingester (`DeM/src/daemon/ingester/session_parser.py`) recognizes only Codex and Claude Code session formats; a pi-format session would be rejected with `ValueError("unsupported session JSONL format")`.
- Zero cross-references: OmG has no mention of `.dem/`, DeM has no mention of `.omg/`.

A user who follows the "companion system" mental model and runs `omg run ...` followed by `dem inspect ...` will find DeM has no awareness of the OmG run. This is a contract that v0.1.5 does not honor.

## Options considered (all deferred)

1. OmG emits pi-format JSONL to `~/.codex/sessions/<run_id>.jsonl` and DeM gains a pi parser. Pro: zero changes to DeM's watcher root. Con: pollutes Codex's directory; requires teaching DeM a third format.

2. DeM reads OmG's SQLite directly via a new `dem.ingester.sqlite_reader` module. Pro: no duplicate writes; preserves OmG's schema as source of truth. Con: cross-language (DeM is Python today), couples DeM to OmG's schema.

3. Shared canonical format under `.omg/sessions/<run_id>.jsonl` that both OmG produces and DeM consumes. Pro: clean ownership boundary. Con: doubles OmG's storage writes; needs a format spec.

None of these is sufficiently obvious that v0.1.x time should be spent on it. All three would also require a corresponding `DEC-013-omg-dem-format-spec.md` to nail down field shapes.

## Consequences

- v0.1.x users: OmG and DeM are independently useful tools. The "companion system" framing in conversation and READMEs should be softened to "complementary tools" until a bridge ships.
- v0.2+ scope: bridge protocol selection is a prerequisite for any feature claiming OmG-DeM cohesion (e.g., DEC-008 thesis exploration via DeM trace data, control-room PWA showing both OmG runs and DeM-tracked sessions).
- Documentation drift: prior DECs and READMEs that mention DeM as if it integrates must be reviewed during v0.2 planning.

## Acceptance evidence

- This file exists at `.omx/plans/decisions/012-omg-dem-bridge.md`.
- No code in `internal/storage/`, `internal/orchestrator/`, or `cmd/omg/` writes JSONL outputs to DeM-watched directories.
- No code in `../DeM/` references OmG paths.
- The "companion system" framing is documented as aspirational until DEC-013 lands.

## Re-entry conditions

Revisit when ANY of the following becomes true:

1. A user reports surprise that `omg run X` does not show up in `dem inspect`.
2. A v0.2 feature requires the integration (e.g., shared run lineage, cross-tool replay).
3. DEC-013 (bridge format spec) is requested in a planning round.

At that point, pick one of the three options above (or a fourth not yet conceived), specify it in DEC-013, and implement the producer + consumer sides in a coordinated commit pair.

# Oh-My-Gunnsama Go Implementer Brief

Primary spec for reference interpretation: `.omx/plans/oh-my-gunnsama-reference-abstraction.md`.

## First-line rule

At the start of each work unit, write:

`작업 컴포넌트: <Phase/Component> — 기준 문서: oh-my-gunnsama-reference-abstraction.md`

## Current authority order

1. User instruction: Oh-My-Gunnsama is a Go-first development harness.
2. C4 diagram and implementation sequence from the user prompt.
3. `.omx/plans/oh-my-gunnsama-reference-abstraction.md`.
4. Existing `.omx/plans/greenfield-agent-harness-selective-extraction.md` only for reference-analysis history. Rust substrate assumptions are stale.
5. `reference/oh-my-codex` and `reference/oh-my-openagent` as abstraction sources, not copy targets.

## Work-unit protocol

Before editing code for any component:

1. Read the matching phase in `.omx/plans/oh-my-gunnsama-reference-abstraction.md`.
2. Read only the cited reference files/line ranges for that phase.
3. State the intended Go package boundary.
4. Implement the smallest slice.
5. Report mapping: `spec idea → Go file/package → validation evidence`.

## Decision policy

- L1 decision: changes product boundary, storage model, daemon protocol, config format, or Go-first substrate. Stop and ask.
- L2 decision: internal helper names, local error names, private struct split. Decide and write a short note if it affects future code.
- L3 decision: formatting, variable names, import order. Decide silently.

## Reference use rules

- OMO prompt builders: port the section-pipeline idea; rewrite text into OMG templates.
- OMO plugin code: use only for OpenCode bridge shape; do not let SDK types enter core.
- OMO config/fallback schema: preserve ergonomics; normalize internally.
- OMX state model: preserve authoritative state vs compatibility view split.
- OMX runtime core: preserve vocabulary; remove tmux/pane coupling from core.
- OMX mux: preserve adapter boundary; implement as Go interface.
- OMX skills: convert to skill folders; do not hardcode workflow names in core unless routing needs them.

## Phase ownership map

| Phase | Go package owners | Reference sections |
|---|---|---|
| 0 daemon/socket/CLI | `cmd/omg`, `internal/daemon`, `internal/protocol` | greenfield only |
| 1 agent registry | `internal/registry` | abstraction §4 Phase 1 |
| 2 prompt builder | `internal/prompt`, `templates/` | abstraction §3.6, §4 Phase 2 |
| 3 provider hooks | `internal/hooks`, `bridge/opencode` | abstraction §3.5, §4 Phase 3 |
| 4 routing | `internal/route` | abstraction §4 Phase 4 |
| 5 fallback/state | `internal/fallback`, `internal/state` | abstraction §3.2, §3.7, §4 Phase 5 |
| 6 skills/MCP | `internal/registry`, `internal/skills` | abstraction §4 Phase 6 |
| 7 guard/verify | `internal/guard`, `internal/verify` | abstraction §4 Phase 7 |
| 8 OpenClaw | `internal/openclaw` | abstraction §4 Phase 8 |

## Red flags

Stop if an implementation tries to:

- Create Rust crates as the main substrate.
- Put OpenCode SDK types in core Go packages.
- Make tmux required for daemon state/routing/prompt building.
- Hardcode the agent inventory in Go instead of YAML.
- Modify existing `~/.codex`, `~/.claude`, or OpenCode config without an explicit install/managed-block path.
- Treat fallback as a quality-improvement retry rather than an infrastructure-failure path.

## Architecture deepening rule

Use the `improve-codebase-architecture` philosophy when shaping Go packages:

- Say **Module**, **Interface**, **Implementation**, **Depth**, **Seam**, **Adapter**, **Leverage**, and **Locality** in design notes.
- Apply the deletion test before creating a package or interface.
- Treat one adapter as a hypothetical seam; do not over-generalize until a second adapter exists.
- Test through the Interface that callers use.
- A reference extraction is valid only when it improves both Leverage and Locality.

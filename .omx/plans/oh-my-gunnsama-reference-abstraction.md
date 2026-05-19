# Oh-My-Gunnsama Reference Abstraction Map

Status: planning reference, not implementation code  
Language decision: Go-first core; JavaScript only as a thin OpenCode bridge  
Reference repos: `reference/oh-my-codex` (OMX), `reference/oh-my-openagent` (OMO)

## 0. Purpose

Oh-My-Gunnsama (OMG) should not fork OMX or OMO wholesale. Treat both as precedent libraries of ideas:

- **OMX contributes** disciplined workflow state, mode lifecycle vocabulary, tmux/mux lessons, Codex-oriented skills/prompts, and verification culture.
- **OMO contributes** dynamic agent prompt composition, agent/category/skill metadata patterns, OpenCode plugin integration patterns, fallback model syntax, and tool/guard concepts.
- **OMG owns** the new Go daemon, Unix socket protocol, provider-neutral event model, YAML/Markdown configuration surface, and `.omg/` runtime state.

This document translates the references into abstraction-level guidance so implementers know what to keep, change, or reject without copying framework-specific internals.

## 1. Non-negotiable architecture decisions

1. **Go binary is the product boundary.**
   - `omg daemon`, `omg on-submit`, `omg prompt-build`, `omg route`, `omg state`, `omg guard`, and later `omg verify` are Go CLI/daemon commands.
   - JS exists only for OpenCode SDK hook registration and socket forwarding.

2. **`.omg/` is the runtime source of truth.**
   - User config: `~/.omg/`.
   - Project runtime: `{project}/.omg/`.
   - Do not make `~/.codex`, `~/.claude`, or OpenCode config authoritative.

3. **Prompt assembly is dynamic and uncached.**
   - Every prompt-build reads YAML agents, skills, categories, model flags, and Markdown templates fresh.
   - Cache invalidation should not become a correctness concern in early phases.

4. **Provider adapters and host adapters are not core.**
   - Core speaks OMG event/request/response structs.
   - Claude Code, Codex, and OpenCode are adapters.
   - Anthropic/OpenAI/Google model APIs are provider adapters or CLI worker targets, not core identity.

5. **References are patterns, not contracts.**
   - Source names like `oracle`, `sisyphus`, `ralph`, `team`, `ultrawork` may inspire OMG metadata.
   - Their original runtime coupling should not leak into Go core unless explicitly re-modeled.

## 2. Reference classification table

| OMG area | Primary reference | Keep | Change for OMG | Reject / avoid |
|---|---|---|---|---|
| Daemon + socket | none / Go stdlib | Unix socket, JSON request-response | Define small stable protocol in Go | Node/Bun plugin as daemon core |
| Hook dispatcher | OMX hooks + OMO plugin | Event-type dispatch; disabled hooks | Host-neutral `HookEvent` envelope | Host-specific hook payload as core schema |
| State manager | OMX state model | Per-mode authoritative files; compatibility views | `.omg/state/<mode>.json`, flock/atomic writes | `skill-active-state.json` compatibility as source of truth |
| Runtime queue | OMX runtime core | authority, dispatch, replay, snapshot vocabulary | Generalize to task/session events | tmux-only outcomes in core |
| Mux/process layer | OMX mux | adapter trait idea; unsupported operation result | Go interface for process/tmux/terminal adapters | binding all dispatch to tmux |
| Agent registry | OMX prompts + OMO agent metadata | role descriptions, aliases, cost, triggers, model chain | YAML schema under `~/.omg/agents` and project overrides | hardcoded agents in Go |
| Prompt builder | OMO dynamic builders + OMX AGENTS | section pipeline, delegation tables, hard blocks, verification | Go section builders + Markdown templates | giant bundled JS prompt source |
| Category routing | OMO category/skills guide + OMX delegation rules | 2-level routing: procedure then domain | heuristic matcher first; LLM scoring later | complex ontology before use cases exist |
| Fallback engine | OMO fallback schema | string/object/list fallback syntax | infra-failure-only transitions | fallback for low-quality answers without evidence |
| Skill loader | OMX skills + OMO skill MCP | skill folders, prompt text, optional MCP manifest | `skill.yaml`, `prompt.md`, optional `mcp.json` | implementing MCP servers inside core |
| Tool guard | OMO guard hooks + OMX safety culture | destructive command/file guard patterns | `omg guard` policy engine | silent mutation of existing provider settings |
| Verification | OMX ralph/checklist culture | claim→check→evidence loop | phase-scoped Go verifier | one universal 9-step pipeline for every task |
| OpenClaw | OMO OpenClaw | multi-channel dispatch concept | optional HTTP subsystem after core stabilizes | making Discord/Telegram part of v1 core |

## 3. Evidence-backed abstractions

### 3.1 OMX runtime core: vocabulary to keep, coupling to remove

Observed reference:

- `reference/oh-my-codex/crates/omx-runtime-core/src/lib.rs:17-43` lists command/event names around authority, dispatch, replay, snapshot, and mailbox.
- `reference/oh-my-codex/crates/omx-runtime-core/src/lib.rs:45-80` introduces `WorkerCli` and `DispatchTransportKind::Tmux`.
- `reference/oh-my-codex/crates/omx-runtime-core/src/lib.rs:96-114` renders outcomes such as `tmux_send_keys_confirmed` and `missing_tmux_target`.
- `reference/oh-my-codex/crates/omx-runtime-core/src/lib.rs:285-324` models a small snapshot with schema version, authority, backlog, replay, and readiness.

OMG abstraction:

- Keep the lifecycle nouns: **authority**, **dispatch**, **replay**, **snapshot**, **mailbox**.
- Recast them as Go structs/events under OMG, not as Rust enum clones.
- Move `WorkerCli`, enter-key policy, tmux target resolution, and pane-specific errors to host/mux adapters.
- Snapshot should remain a compact diagnostic view, not the primary event log.

Go shape to aim for:

```go
type RuntimeSnapshot struct {
    SchemaVersion int             `json:"schema_version"`
    Authority     AuthorityView   `json:"authority"`
    Backlog       BacklogView     `json:"backlog"`
    Replay        ReplayView      `json:"replay"`
    Readiness     ReadinessView   `json:"readiness"`
}
```

Do not copy exact Rust variants until a phase needs them.

### 3.2 OMX state model: authoritative state vs compatibility views

Observed reference:

- `reference/oh-my-codex/docs/STATE_MODEL.md:12-27` says per-mode files under `.omx/state/` are authoritative.
- `reference/oh-my-codex/docs/STATE_MODEL.md:29-37` treats `skill-active-state.json` as compatibility/visibility.
- `reference/oh-my-codex/docs/STATE_MODEL.md:38-47` defines session precedence over root fallback.
- `reference/oh-my-codex/docs/STATE_MODEL.md:48-73` separates internal phase from user-facing lifecycle outcome.

OMG abstraction:

- `.omg/state/<mode>.json` and `.omg/state/sessions/<session>/<mode>.json` should be authoritative.
- Any projected provider state is a compatibility view.
- Session scope wins over project/root scope.
- Public lifecycle should be explicit: `running`, `finished`, `blocked`, `failed`, `ask_user`, not inferred from miscellaneous flags unless migrating old data.

### 3.3 OMX mux: adapter boundary worth copying conceptually

Observed reference:

- `reference/oh-my-codex/crates/omx-mux/src/types.rs:235-258` defines operations like resolve target, send input, capture tail, inspect liveness, attach, detach.
- `reference/oh-my-codex/crates/omx-mux/src/types.rs:260-277` separates outcomes from errors, including unsupported operations.
- `reference/oh-my-codex/crates/omx-mux/src/types.rs:291-295` defines an adapter interface.

OMG abstraction:

- Use a Go interface for terminal/process dispatch.
- Allow adapters to say “unsupported” without failing the entire daemon.
- Keep mux below routing and state; it is an execution transport, not the agent brain.

Go shape to aim for:

```go
type MuxAdapter interface {
    Name() string
    Execute(ctx context.Context, op MuxOperation) (MuxOutcome, error)
}
```

### 3.4 OMO config loader: partial recovery and merge semantics

Observed reference:

- `reference/oh-my-openagent/src/plugin-config.ts:48-90` attempts partial config load when full validation fails.
- `reference/oh-my-openagent/src/plugin-config.ts:92-132` records config errors but can still return a partial config.
- `reference/oh-my-openagent/src/plugin-config.ts:135-193` deep-merges agents/categories and union-merges disabled lists.
- `reference/oh-my-openagent/src/plugin-config.ts:196-230` separates user-level and project-level config discovery.

OMG abstraction:

- Use YAML for agents/skills/categories and probably YAML or JSON for global config, but preserve the **behavioral pattern**:
  - Load user config.
  - Load project config.
  - Merge maps deeply.
  - Union disabled lists.
  - Keep valid sections even if one section is invalid.
  - Emit warnings; do not brick the daemon for one bad optional section.

Do not inherit OpenCode-specific config names such as `claude_code`, `sisyphus`, `tmux`, or `openclaw` as top-level core schema unless OMG intentionally defines them.

### 3.5 OMO plugin boundary: OpenCode is an adapter, not the core

Observed reference:

- `reference/oh-my-openagent/src/index.ts:1-3` imports OpenCode plugin types directly.
- `reference/oh-my-openagent/src/index.ts:23-39` constructs an OpenCode plugin by reading OpenCode input and loading config.
- `reference/oh-my-openagent/src/index.ts:94-121` creates tools/hooks and returns a plugin interface.

OMG abstraction:

- For OpenCode, write a small `plugin.js` that registers hooks/tools and forwards JSON to `/tmp/omg.sock`.
- Keep OpenCode SDK types out of Go core.
- The bridge may translate hook names, but the daemon receives a normalized envelope.

Normalized envelope example:

```json
{
  "provider": "opencode",
  "event": "on-submit",
  "project": "/repo/path",
  "session_id": "optional",
  "payload": { "input": "user prompt" }
}
```

### 3.6 OMO prompt builders: section pipeline to port, text to rewrite

Observed reference:

- `reference/oh-my-openagent/src/agents/dynamic-agent-core-sections.ts:15-24` builds explicit identity.
- `reference/oh-my-openagent/src/agents/dynamic-agent-core-sections.ts:44-75` builds a cost-ordered tool/agent table.
- `reference/oh-my-openagent/src/agents/dynamic-agent-core-sections.ts:77-170` conditionally builds explore/librarian/oracle guidance.
- `reference/oh-my-openagent/src/agents/dynamic-agent-core-sections.ts:173-230` adds model-conditional planning/delegation sections.
- `reference/oh-my-openagent/src/agents/dynamic-agent-policy-sections.ts:7-37` defines hard blocks and anti-patterns.
- `reference/oh-my-openagent/src/agents/dynamic-agent-policy-sections.ts:127-170` defines anti-duplication guidance.
- `reference/oh-my-openagent/src/agents/dynamic-agent-category-skills-guide.ts:48-130` builds category/skill delegation guidance.

OMG abstraction:

- Port the **pipeline architecture**, not the prose.
- Each section builder should be small, deterministic, and testable.
- Markdown templates hold stable text; Go chooses which templates/sections are included.
- Agent/category/skill metadata should drive tables.
- Model calibration should be a controlled conditional section, not a second copy of the whole prompt.

Suggested Go package boundary:

```text
internal/prompt/
  builder.go          // orchestrates sections
  sections.go         // deterministic section builders
  templates.go        // loads markdown templates
  model.go            // model family classification
```

### 3.7 OMO agent/fallback schema: metadata syntax worth preserving

Observed reference:

- `reference/oh-my-openagent/src/config/schema/fallback-models.ts:3-29` accepts fallback models as string/object/list/mixed list.
- `reference/oh-my-openagent/src/config/schema/agent-overrides.ts:5-56` shows agent-level model, skills, tools, permissions, token/reasoning options, and provider options.
- `reference/oh-my-openagent/src/config/schema/agent-overrides.ts:58-75` lists fixed OMO agent names.

OMG abstraction:

- Preserve flexible fallback syntax, but parse into one canonical internal list.
- Preserve agent-level overrides, but do not freeze OMG to OMO’s fixed agent set.
- OMG agents are data files with aliases, triggers, cost, posture, model chain, and tools policy.

Canonical internal fallback shape:

```go
type ModelCandidate struct {
    Model           string         `yaml:"model" json:"model"`
    Variant         string         `yaml:"variant,omitempty" json:"variant,omitempty"`
    ReasoningEffort string         `yaml:"reasoningEffort,omitempty" json:"reasoningEffort,omitempty"`
    Params          map[string]any `yaml:"params,omitempty" json:"params,omitempty"`
}
```

### 3.8 OMO tool registry: registry pattern, not OpenCode tool types

Observed reference:

- `reference/oh-my-openagent/src/plugin/tool-registry.ts:144-152` builds a registry from plugin context, config, managers, skill context, categories, and factories.
- `reference/oh-my-openagent/src/plugin/tool-registry.ts:166-220` conditionally creates background, agent, look-at, delegate, and OpenClaw-connected tools.

OMG abstraction:

- Use a host-neutral registry:
  - tool manifest
  - enabled/disabled decision
  - guard policy
  - host binding adapter
- OpenCode tool definitions are emitted by the JS bridge or an adapter generator, not stored as core structs.

## 4. Phase-by-phase abstraction guide

### Phase 0 — daemon + socket + CLI

Reference level: **greenfield**.

Use:
- Go `net.Listen("unix", path)`.
- `encoding/json` for request/response.
- PID/socket lifecycle from standard daemon practice.

Do not use:
- OMO plugin boot sequence.
- OMX tmux runtime core.

Minimum protocol:

```json
{ "event": "on-submit", "input": "hello" }
```

Response should include:

```json
{ "ok": true, "action": "inject_prompt", "output": "..." }
```

### Phase 1 — Agent Registry

Reference level: **metadata fusion**.

Use:
- OMX role prompts as role intent and constraints.
- OMO metadata as aliases, cost, triggers, and dynamic prompt fields.

Output:

```text
~/.omg/agents/*.yaml
{project}/.omg/agents/*.yaml
```

Rule:
- Merge by canonical `name`.
- Preserve source names in `aliases`.
- Go code may know the schema, but not the agent inventory.

Suggested first canonical set:

| Canonical | Aliases / source idea | Notes |
|---|---|---|
| analyst | metis, OMX analyst | requirements clarity |
| architect | oracle, OMX architect | expensive read-only design/debugging |
| critic | momus, OMX critic | challenge/review |
| planner | prometheus, OMX planner | planning/interview/spec |
| explore | explore | repo-local lookup |
| executor | OMX executor | directed implementation |
| hephaestus | OMO hephaestus | autonomous deep worker |
| librarian | OMO librarian | external/reference lookup |
| atlas | OMO atlas | agent selection / router inspiration |
| vision | multimodal-looker, OMX vision | image/screenshot analysis |

### Phase 2 — Prompt Builder

Reference level: **architecture port**.

Use OMO’s section pipeline concept:

1. always sections
2. model calibration sections
3. agent availability sections
4. category/skill sections
5. generated metadata tables
6. assembler

Use OMX’s operating content:

- autonomy directive
- working agreements
- delegation rules
- Lore commit protocol
- verification loop
- cancellation/state discipline

Do not paste the whole installed AGENTS.md into every generated prompt. Split it into templates and include only the sections the selected agent/model needs.

### Phase 3 — Provider hooks

Reference level: **adapter pattern**.

Claude Code/Codex:
- Exec hook calls `omg on-submit`.
- Must work without daemon if needed.

OpenCode:
- JS bridge starts or connects to daemon.
- Hook callbacks forward JSON to socket.
- JS should not implement routing/prompt logic.

Safety:
- Never overwrite existing provider settings by default.
- Install should add managed blocks or emit patch instructions.
- `omg disable` should be able to detach managed hooks.

### Phase 4 — Routing Engine

Reference level: **heuristic-first classifier**.

Two-stage route:

1. Procedure: interview / plan / execute / review / verify / cancel.
2. Domain: visual / backend / docs / security / quick / unknown.

Use:
- OMX delegation rules as procedure-routing inspiration.
- OMO category/skill guide as domain-routing inspiration.

Do not use:
- LLM scoring in v1.
- A large ontology before real tasks force it.

### Phase 5 — Fallback Engine + State Manager

Reference level: **state semantics and fallback syntax**.

Fallback:
- Use OMO’s flexible syntax.
- Normalize to ordered candidates.
- Trigger only on infrastructure errors: rate limit, timeout, provider unavailable, auth unavailable where a fallback provider exists.

State:
- Use OMX’s authoritative/compatibility separation.
- Use atomic writes and locking.
- Keep per-mode state readable and manually inspectable.

### Phase 6 — Skill Loader + MCP bundle

Reference level: **folder convention and lifecycle**.

Use:

```text
~/.omg/skills/<name>/
  skill.yaml
  prompt.md
  mcp.json optional
```

Rules:
- Skills are loaded on demand.
- MCP servers are started/proxied by lifecycle manager.
- Skill text can be converted from OMX `SKILL.md`, but OMG metadata should live in `skill.yaml`.

### Phase 7 — Tool Guard + Verification Pipeline

Reference level: **policy engine**.

Use:
- OMO guard hook categories: bash/file/comment/format/error recovery.
- OMX verification discipline: claim, smallest proving check, read output, report evidence.

Do not hardcode one heavyweight verification sequence for every task. Verification plans should be phase/task scoped.

### Phase 8 — OpenClaw + JS bridge finish

Reference level: **optional integration**.

Use OMO’s OpenClaw as a product concept: external channel dispatch and reply listeners.

Do not put Discord/Telegram/process spawning into v1 core. Add after the daemon, routing, prompt, state, and guards are stable.

## 5. Go package decomposition proposal

```text
cmd/omg/                 CLI entrypoint
internal/daemon/         socket server, lifecycle, PID/socket cleanup
internal/protocol/       JSON envelopes and responses
internal/hooks/          hook event normalization and dispatch
internal/config/         user/project config load and merge
internal/registry/       agents, skills, categories, providers
internal/prompt/         prompt builder and template rendering
internal/route/          procedure/domain routing
internal/state/          .omg state read/write/lock/snapshot
internal/fallback/       model chain normalization and infra failure decisions
internal/guard/          tool/file/bash policy decisions
internal/mux/            process/tmux/socket adapter interface
internal/verify/         task-scoped verification plans
internal/openclaw/       optional HTTP/multichannel subsystem later
bridge/opencode/         tiny JS bridge source
```

## 6. Source-to-OMG translation rules

1. If reference code imports OpenCode SDK types, treat it as **adapter-only inspiration**.
2. If reference code names tmux/panes directly, keep it below `internal/mux`.
3. If reference code is prompt prose, convert it into **templates**, not Go constants, unless it is a small generated table.
4. If reference code is a schema, preserve the **user ergonomics** only when it fits YAML/Markdown configuration.
5. If reference code is a workflow skill, convert to `skill.yaml` + `prompt.md`; do not embed as special-case Go code.
6. If a concept needs provider credentials or external service state, keep it out of Phase 0-2.
7. If a concept cannot be explained as an interface boundary, defer it.

## 7. Conflicts with the older greenfield plan

The existing `.omx/plans/greenfield-agent-harness-selective-extraction.md` mentions a Rust crate direction in several sections. The user’s newer instruction supersedes that: **OMG main language is Go**.

Therefore:

- Rust-specific crate names in the older plan should be treated as stale implementation substrate, not product direction.
- The selective extraction decisions remain useful at the abstraction level.
- Any future implementation plan should rewrite substrate assumptions to Go packages before code starts.

## 8. Stop condition for reference use

A reference has been used enough when the implementer can answer these four questions without copying code:

1. What behavior should OMG preserve?
2. What coupling should OMG remove?
3. Which Go package owns the behavior?
4. What file under `.omg/` or `~/.omg/` makes the behavior configurable or inspectable?

If any answer requires “because OMO/OMX did it that way,” the abstraction is not finished.

## 9. Architecture-improvement philosophy to apply during OMG design

Source philosophy: `improve-codebase-architecture` skill, using the terms **Module**, **Interface**, **Implementation**, **Depth**, **Seam**, **Adapter**, **Leverage**, and **Locality**.

OMG should be designed as a set of deep modules, not a collection of pass-through wrappers around OMX/OMO ideas.

### 9.1 Required vocabulary

Use these terms in architecture reviews and future design notes:

- **Module** — any OMG package, command, registry, loader, policy, or adapter with an interface and an implementation.
- **Interface** — everything a caller must know to use the module: request shape, invariants, error modes, ordering, config, and filesystem effects.
- **Implementation** — the hidden code behind the interface.
- **Depth** — how much behavior the caller gets through a small interface.
- **Seam** — where an interface lives and behavior can change without editing callers.
- **Adapter** — a concrete implementation satisfying an interface at a seam.
- **Leverage** — what callers gain from the module.
- **Locality** — how much change/bug knowledge is concentrated in one place.

Avoid drifting into vague words like component/service/boundary when deciding architecture. Prefer **Module**, **Interface**, **Seam**, and **Adapter**.

### 9.2 Deletion test for OMG modules

Before adding any module, apply the deletion test:

> If this module were deleted, would complexity disappear, or would it reappear across many callers?

- If complexity disappears, the module is probably shallow and should be removed or folded into its caller.
- If complexity reappears across many callers, the module is earning its keep.
- If only one adapter exists, treat the seam as hypothetical. Keep the interface small and avoid over-generalizing.
- When two real adapters exist, the seam is real and deserves stronger tests and clearer documentation.

### 9.3 Phase-specific implications

| OMG area | Deepening rule |
|---|---|
| `internal/protocol` | Deep if it hides provider-specific payload differences behind a stable hook envelope. Shallow if it only renames JSON fields. |
| `internal/daemon` | Deep if lifecycle, socket cleanup, PID handling, and direct/daemon mode are localized. Shallow if every command reimplements daemon checks. |
| `internal/registry` | Deep if callers ask for agents/skills/categories through one normalized interface. Shallow if callers manually walk YAML trees. |
| `internal/prompt` | Deep if callers provide model + registry data and receive a prompt. Shallow if every caller must understand section ordering. |
| `internal/route` | Deep if procedure/domain decisions are localized and explainable. Shallow if keyword rules scatter across hooks, prompts, and CLI commands. |
| `internal/state` | Deep if locking, atomic writes, session precedence, and compatibility views are hidden. Shallow if each mode writes JSON differently. |
| `internal/fallback` | Deep if infra-failure classification and model-chain progression are centralized. Shallow if every provider handles retry policy ad hoc. |
| `internal/guard` | Deep if destructive-command and file-protection policy are centralized. Shallow if each host adapter blocks different things. |
| `internal/mux` | Deep only after there are multiple adapters or a real need for terminal/process variation. Until then, keep the seam narrow. |
| `bridge/opencode` | Must remain shallow on purpose: a thin adapter, not a second implementation of OMG logic. |

### 9.4 Interface is the test surface

For each module, tests should target the interface rather than only extracted helper functions.

Examples:

- Test prompt builder through `BuildPrompt(input)` using fixture agents/templates, not by snapshotting every private helper.
- Test state manager through `Read/Write/Clear` behavior with session precedence and corrupt-file cases, not by testing JSON formatting helpers only.
- Test fallback through “given this error and chain, choose next candidate,” not by testing provider-specific string checks in isolation.
- Test daemon protocol through socket/direct command behavior, not only through handler internals.

### 9.5 How to use OMX/OMO references under this philosophy

When considering a reference extraction, ask:

1. What is the **Module** in OMG terms?
2. What is the smallest **Interface** that gives callers leverage?
3. What **Implementation** details from OMX/OMO should be hidden or discarded?
4. Where is the **Seam**?
5. Do we have one **Adapter** or two? If one, avoid over-design.
6. What **Locality** improves if this exists?
7. What test proves the Interface earns its keep?

Only extract or recreate a reference idea when it increases both **Leverage** and **Locality**.

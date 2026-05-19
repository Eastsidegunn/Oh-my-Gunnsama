# Oh-My-Gunnsama: Interfaces that References Cannot Close

Status: decision backlog  
Scope: items that remain open even after reading OMX/OMO references  
Reason: these are OMG product contracts, not extraction details

## Summary

> P0 update: P0 items 1-4 are now closed in `.omx/plans/oh-my-gunnsama-p0-interface-closure.md`. This file remains the backlog for P1+ and historical rationale.


OMX and OMO help with patterns, but they cannot close these Interfaces because their host assumptions differ:

- OMX is Codex/tmux/workflow-state oriented.
- OMO is OpenCode/plugin/multi-agent-tool oriented.
- OMG is Go-first, daemon/socket, three-host harness with `.omg/` as source of truth.

Therefore the items below need explicit OMG decisions before the codebase grows.

## P0 — Closed / historical rationale

P0 items 1-4 are closed in `.omx/plans/oh-my-gunnsama-p0-interface-closure.md`. They remain important as historical rationale, but they are no longer active unclosed Interfaces.

Closed P0 decisions:

1. **Daemon protocol envelope**
   - Closed as line-delimited JSON over Unix socket.
   - Host-specific data stays under `payload`.
   - Core reads normalized top-level fields only.

2. **Direct mode vs daemon mode**
   - Closed as socket-first with cheap inline fallback.
   - Hook commands do not auto-start daemon.
   - stdout remains machine-readable; diagnostics go to stderr/logs.

3. **`.omg/` path and precedence**
   - Closed as built-in → user → project for definitions.
   - Closed as session → project/root → compatibility projection for runtime state.

4. **Agent YAML schema and merge semantics**
   - Closed as data-driven canonical agents with aliases, category, autonomy, model/model_chain, tools policy, triggers, use/avoid hints, disabled state, and strict validation mode.
   - `posture` and simultaneous `tools_allowed`/`tools_blocked` were rejected for v1.

Do not re-open P0 during Phase 0/1 implementation unless implementation exposes a contradiction in the closure document.

## P1 — Closed / historical rationale

P1 items 5-8 are now closed in `.omx/plans/oh-my-gunnsama-p1-interface-closure.md`. They remain important as historical rationale, but they are no longer active unclosed Interfaces.

Closed P1 decisions:

5. **Prompt build input/output contract**
   - Closed as narrow `BuildPromptInput` with resolved `Agent`, `Model`, `Project`, optional `PromptContextSnapshot`, `RouteResult`, and flags.
   - Raw `UserMessage` and raw `SessionState` are rejected as normal builder inputs.
   - Output includes final prompt, section metadata, omitted sections, warnings, and token estimate.

6. **Route result contract**
   - Closed as pure non-mutating route result with `Procedure`, `Domain`, `Agent`, `Confidence`, optional debug `Score`, reasons, triggers, and `RequiresUser`.
   - Procedures are `interview|plan|execute|review|verify|cancel|none`.
   - `route`, `delegate`, `guard`, and `passthrough` are rejected as procedure values.

7. **Host Adapter contract**
   - Closed as thin protocol translation.
   - Adapters may translate raw host errors, but must not route, build prompts, select sections, run fallback/retry, mutate state, execute agents, or decide guard policy.

8. **Error taxonomy**
   - Closed as `ErrorClass` (`retryable|stop|non_retryable`) plus `ErrorDomain`.
   - No separate `Retryable bool`; use method derived from class.
   - `Degraded` is not an error class in v1; represent it via health/warning signals.

Do not re-open P1 during Phase 2/3 implementation unless implementation exposes a contradiction in the closure document.

## P1 — Historical rationale


### 5. Prompt build input/output contract

**Module**: `internal/prompt`  
**Unclosed Interface**: exactly what callers pass into prompt builder and what they receive.

References cannot close it because:

- OMO builds prompts inside OpenCode agent/model assumptions.
- OMX has broad AGENTS.md instructions and skills, not a neutral builder API.
- OMG needs one builder for CLI hooks, direct prompt-build, and later OpenCode transform.

Decision needed:

- `BuildPrompt` input fields.
- how selected agent is represented.
- how model family is represented.
- whether route result is input or prompt builder calls router.
- whether project rules are raw text or parsed sections.
- output metadata: included sections, warnings, source files.

Recommended default:

Prompt builder does not route. It receives already-resolved context:

```go
type BuildPromptInput struct {
    ProjectRoot string
    Model       ModelInfo
    Agent       AgentRef
    Agents      []Agent
    Skills      []Skill
    Categories  []Category
    Flags       map[string]string
    ProjectRules string
}
```

Output includes prompt plus debug metadata:

```go
type BuildPromptOutput struct {
    Prompt string
    Sections []string
    Sources []string
    Warnings []string
}
```

Why this must close:

- Without this, prompt logic leaks into route, hooks, and provider adapters.

---

### 6. Route result contract

**Module**: `internal/route`  
**Unclosed Interface**: normalized output of procedure/domain routing.

References cannot close it because:

- OMX workflow routing is skill/mode oriented.
- OMO category routing is OpenCode task/delegation oriented.
- OMG needs a host-neutral decision object.

Decision needed:

- stage names.
- confidence representation.
- explanation format.
- whether route can select an agent.
- whether route can request blocking interview.
- whether route mutates state or only returns advice.

Recommended default:

Route is pure and non-mutating:

```json
{
  "procedure": "interview|plan|execute|review|verify|cancel|none",
  "domain": "visual|backend|docs|security|quick|unknown",
  "agent": "optional canonical agent",
  "confidence": "low|medium|high",
  "reasons": [],
  "requires_user": false
}
```

Why this must close:

- It prevents keyword logic from scattering across hooks, prompt templates, and commands.

---

### 7. Host Adapter contract

**Module**: `internal/hooks`, `bridge/opencode`  
**Unclosed Interface**: responsibilities of Claude/Codex exec hooks vs OpenCode JS bridge.

References cannot close it because:

- OMO is fully inside OpenCode.
- OMX is Codex-first.
- OMG must normalize three hosts.

Decision needed:

- what each Adapter must send.
- what each Adapter may do locally.
- what each Adapter must never do.
- install/disable managed-block format.
- how host-specific capabilities are reported.

Recommended default:

Adapters may:

- collect host payload
- normalize to protocol envelope
- call socket or inline CLI
- apply response to host hook output

Adapters must not:

- route
- build prompts
- load agents/skills directly
- mutate `.omg/state` except via daemon/CLI command

Why this must close:

- Otherwise OpenCode bridge will grow into a second implementation.

---

### 8. Error taxonomy for fallback and guard

**Module**: `internal/fallback`, `internal/guard`, `internal/protocol`  
**Unclosed Interface**: normalized error classes.

References cannot close it because:

- OMO fallback knows model/provider/plugin failure shapes.
- OMX runtime failures are tmux/workflow oriented.
- OMG must classify daemon, host, provider, CLI worker, socket, filesystem, and policy errors.

Decision needed:

- error classes.
- retryable vs non-retryable.
- fallback-eligible vs not.
- user-visible vs log-only.

Recommended default classes:

- `rate_limit`
- `timeout`
- `auth_unavailable`
- `provider_unavailable`
- `socket_unavailable`
- `invalid_config`
- `policy_blocked`
- `filesystem_error`
- `unknown`

Fallback eligible:

- `rate_limit`
- `timeout`
- `provider_unavailable`

Usually not fallback eligible:

- `invalid_config`
- `policy_blocked`
- `auth_unavailable` unless another provider in chain is independently authenticated

Why this must close:

- Fallback must not become “retry until answer looks better.”

---

## P2 — Closed / historical rationale

P2 items 9-12 are now closed in `.omx/plans/oh-my-gunnsama-p2-interface-closure.md`. They remain important as historical rationale, but they are no longer active unclosed Interfaces.

Closed P2 decisions:

9. State transition model
   - lifecycle/context split
   - `cancelled` terminal state
   - stale heartbeat/daemon restart -> `blocked`, not `failed`
   - configurable heartbeat timeout

10. Skill schema and MCP lifecycle
   - skills provide capability only
   - no v1 skill-level model or hard agent binding
   - MCP lifecycle is lazy and manager-owned

11. Guard decision schema
   - structured `allow|warn|block` decisions
   - no rewrite in v1
   - P0-aligned fail policy
   - constrained project-local suppressions only

12. Verification plan contract
   - argv-first checks
   - verifier reports evidence only
   - fix/retry belongs to orchestrator
   - command/manual/artifact evidence kinds

Do not re-open P2 during Phase 4/5/6/7 implementation unless implementation exposes a contradiction in the closure document.

## P2 — Historical rationale

### 9. State transition model

**Module**: `internal/state`  
**Unclosed Interface**: legal mode transitions and lifecycle vocabulary.

References cannot close it because:

- OMX mode state is mature but tied to OMX workflows.
- OMG may keep some workflow names but should not inherit all state semantics.

Decision needed:

- canonical lifecycle values.
- allowed transitions.
- session/root reconciliation.
- stale lock/state handling.
- compatibility output shape.

Recommended default lifecycle:

- `inactive`
- `running`
- `blocked`
- `ask_user`
- `finished`
- `failed`

Why this can wait slightly:

- Phase 0 can use minimal state, but Phase 5 must close this before multiple workflows exist.

---

### 10. Skill schema and MCP lifecycle

**Module**: `internal/skills`  
**Unclosed Interface**: `skill.yaml`, prompt loading, and optional MCP process lifecycle.

References cannot close it because:

- OMX skills are Markdown-first workflows.
- OMO skill MCP is plugin/tool-manager oriented.
- OMG needs daemon-managed lifecycle without implementing MCP servers.

Decision needed:

- skill metadata schema.
- skill prompt composition rules.
- MCP start/stop ownership.
- environment allowlist.
- zombie cleanup strategy.

Recommended default:

- `skill.yaml` declares name, description, triggers, prompt file, optional `mcp.json`.
- Skill loader returns metadata and text.
- MCP manager owns process lifecycle separately.

---

### 11. Guard decision schema

**Module**: `internal/guard`  
**Unclosed Interface**: what a tool guard returns and how hosts apply it.

References cannot close it because:

- OMO guards are hook-specific.
- OMG must support bash/file/tool checks across hosts.

Decision needed:

- action names.
- reason format.
- severity.
- whether guard can rewrite arguments or only allow/block/warn.

Recommended default:

```json
{
  "action": "allow|warn|block",
  "severity": "info|warning|danger",
  "reason": "human-readable",
  "rule_id": "stable-id"
}
```

Avoid rewrite in v1 unless there is a strong use case.

---

### 12. Verification plan contract

**Module**: `internal/verify`  
**Unclosed Interface**: how verification plans are represented and executed.

References cannot close it because:

- OMX verification checklists are workflow/prompt discipline.
- OMG needs executable plans across arbitrary Go/user projects.

Decision needed:

- static checklist vs project-detected checks.
- command execution policy.
- evidence output format.
- failure continuation policy.

Recommended default:

- verification plans are data objects.
- each check has command, cwd, timeout, and claim it proves.
- verifier reports evidence; it does not silently fix.

---

## P3 — Product decisions, not architecture blockers

### 13. OpenClaw channel model

**Module**: `internal/openclaw`  
**Unclosed Interface**: Discord/Telegram/process/spawn dispatch contract.

Why references cannot close it:

- OMO’s OpenClaw is product-specific.
- OMG should decide later whether this is core, plugin, or optional daemon mode.

Default: defer.

---

### 14. Built-in agent inventory

**Module**: `~/.omg/agents`, `internal/registry`  
**Unclosed Interface**: exact first-party agent set.

Why references cannot close it:

- The OMX/OMO mapping is useful, but OMG’s product identity may not need every role.

Default:

- Start with the minimum canonical set needed by Phase 1/2.
- Add more only when route/prompt/skill behavior needs them.

---

## What is already closed enough

These decisions are closed enough for now:

- Go-first core.
- JS only as thin OpenCode bridge.
- `.omg/` is source of truth.
- references are abstraction sources, not fork targets.
- dynamic prompt assembly with no cache.
- YAML/Markdown configuration surface.
- fallback is infra-failure-only.
- hardcoded agent inventory in Go is forbidden.
- OpenClaw is late/optional.

## Recommended next action

Close P1 before Phase 2/3 integration. Suggested next closure notes:

1. `prompt-build-interface.md`
2. `route-result-interface.md`
3. `host-adapter-interface.md`
4. `error-taxonomy-interface.md`

Each should define:

- Module
- Interface
- Inputs
- Outputs
- Invariants
- Error modes
- Filesystem effects
- Adapters
- Deletion test
- Tests through Interface

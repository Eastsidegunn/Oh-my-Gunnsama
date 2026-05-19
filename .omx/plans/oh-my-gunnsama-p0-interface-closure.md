# Oh-My-Gunnsama P0 Interface Closure

Status: P0 product-interface decisions  
Scope: decisions needed before Phase 0/1 implementation  
Principle: close small Interfaces early; keep Implementations replaceable

## 0. Design philosophy

OMG is expected to become a large codebase. P0 Interfaces must therefore be small, explicit, and hard to misuse.

Use these architecture terms consistently:

- **Module**: a Go package, command, registry, loader, policy, or adapter with an Interface and Implementation.
- **Interface**: everything a caller must know to use the Module: input shape, output shape, invariants, error modes, config, ordering, and filesystem effects.
- **Implementation**: hidden code behind the Interface.
- **Seam**: where behavior can vary without editing callers.
- **Adapter**: concrete Implementation behind a Seam.
- **Depth**: leverage behind a small Interface.
- **Leverage**: what callers gain.
- **Locality**: how much change/debug knowledge is concentrated.

Deletion test:

> If deleting a Module makes complexity disappear, it was shallow. If deleting it spreads complexity across callers, it was earning its keep.

P0 goal: define Interfaces that give Leverage and Locality without pretending every future Seam is already real.

---

## 1. Daemon Protocol Envelope

### Module

`internal/protocol`

### Interface

Line-delimited JSON over Unix socket. One connection handles one request and one response in v1.

### Decision

Use a host-neutral envelope. Host-specific payloads live under `payload`; core routing reads only normalized top-level fields.

Request:

```json
{
  "version": 1,
  "request_id": "uuid-or-host-id",
  "provider": "codex|claude|opencode|direct",
  "event": "on-submit|on-tool-before|on-tool-after|on-error|on-compact|on-stop",
  "project": "/abs/project/path",
  "cwd": "/abs/current/working/dir",
  "session_id": "optional",
  "timeout_ms": 500,
  "capabilities": {},
  "payload": {}
}
```

Response:

```json
{
  "ok": true,
  "request_id": "same-id",
  "action": "none|inject_prompt|block|replace|warn",
  "output": "optional text",
  "warnings": [],
  "error": null
}
```

Error response:

```json
{
  "ok": false,
  "request_id": "same-id-if-known",
  "action": "warn",
  "output": "",
  "warnings": [],
  "error": {
    "class": "timeout|invalid_config|socket_unavailable|policy_blocked|filesystem_error|unknown",
    "message": "human-readable summary",
    "retryable": false
  }
}
```

`error.class` is string-typed, not a closed enum in the wire protocol. The classes above are the P0 known classes. P1 error taxonomy may add classes such as `rate_limit`, `auth_unavailable`, or `provider_unavailable` without changing protocol version 1. Unknown classes must be handled conservatively by callers.

### Invariants

- `version` is required.
- `provider` is a normalized host label, not a model provider.
- `event` is host-agnostic.
- `project` must be absolute when known.
- `payload` may contain host-specific raw data.
- Top-level fields must remain stable within protocol version 1.
- Diagnostics go to stderr or `.omg/logs/`, not stdout.
- `timeout_ms` means the full request budget. Missing, zero, or negative values use the event default. Host-provided values below the default may narrow the budget. Excessive values may be capped by the daemon or inline command. Internal connect timeout must fit inside the full request budget.

### Why references do not close this

- OMX payloads are Codex/OMX workflow-state oriented.
- OMO payloads are OpenCode SDK callback oriented.
- OMG needs one neutral socket contract across Claude Code, Codex, OpenCode, and direct CLI mode.

### Tests through Interface

- Decode valid request and encode valid response.
- Unknown payload fields are preserved/ignored safely.
- Missing required top-level fields return structured `invalid_config` or `unknown` errors.
- `request_id` is echoed when known.

---

## 2. Direct Mode vs Daemon Mode

### Module

`cmd/omg`, `internal/daemon`

### Interface

`omg on-submit` and other hook commands must produce deterministic machine-readable stdout even when the daemon is absent, stale, slow, or broken.

### Decision

Use socket-first with inline fallback. Do not auto-start daemon from hook commands.

Flow:

1. Try Unix socket.
2. If socket is absent/stale/slow, run inline only for safe short paths.
3. Return one JSON response to stdout.
4. Send diagnostics to stderr and/or `.omg/logs/`.

### Timeout tiers

Defaults; host adapters may pass lower `timeout_ms`.

| Event | Connect timeout | Full request timeout | Fallback |
|---|---:|---:|---|
| `on-submit` | 50-100ms | 300-800ms | inline prompt/route path |
| `on-tool-before` | 50ms | 100-300ms | simple guard path |
| `on-tool-after` | 50ms | 100-300ms | no-op/warn |
| `on-error` | 50ms | 100-300ms | no-op/warn |
| `on-compact` | 50ms | 300-800ms | inline context snapshot only if cheap |
| explicit `omg prompt-build` | n/a | user-command timeout | direct execution |

### Inline fallback may do

- protocol decode
- config load
- agent/category/skill registry load
- route
- prompt build
- simple guard checks

### Inline fallback must not do

- daemon spawn
- MCP server start
- long verification
- OpenClaw dispatch
- background agent spawn
- provider LLM call
- long filesystem scans

### Guard fail-open/fail-closed rule

- If a cheap inline guard can identify destructive risk, return `block`.
- If daemon is unavailable and no destructive risk is detected, return `warn` or `none` and log warning.

### Why references do not close this

- OMX can lean on runtime/tmux/state surfaces.
- OMO runs inside OpenCode plugin lifecycle.
- Claude Code/Codex exec hooks require predictable stdout under short timeouts.

### Tests through Interface

- Hook command returns valid JSON when daemon is absent.
- Stale socket falls back inline within timeout.
- Diagnostics do not pollute stdout.
- Destructive guard case blocks without daemon.
- Non-destructive guard case fails open with warning.

---

## 3. `.omg/` Path and Precedence

### Module

`internal/config`, `internal/state`

### Interface

All configuration and runtime state resolve through one deterministic precedence model.

### Decision

Definitions and runtime state have separate precedence.

Definition precedence:

1. built-in defaults bundled with OMG
2. user config: `~/.omg/`
3. project config: `{project}/.omg/`

Runtime state precedence:

1. session state: `{project}/.omg/state/sessions/<session_id>/`
2. project/root state: `{project}/.omg/state/`
3. compatibility projections, read-only unless explicitly generated

### Merge semantics

- Scalar fields: override.
- Map fields: deep merge by key.
- List fields: replace by default.
- Append is explicit with `*_append`.
- Remove may be added later as `*_remove`, but is not required for P0.
- Project config can scope-locally disable user-defined agents/skills.
- Disable in project does not delete user config; it shadows it only for that project.

### Examples

User agent defines tools:

```yaml
tools: [read, grep]
```

Project replaces tools:

```yaml
tools: [read, grep, glob]
```

Project appends tools:

```yaml
tools_append: [lsp_diagnostics]
```

Project disables a user agent locally:

```yaml
name: experimental-agent
disabled: true
```

### Why references do not close this

- OMX has `.omx/` state and Codex-specific installed surfaces.
- OMO has OpenCode user/project plugin config and often union-merges arrays.
- OMG needs local project authority plus user extensibility, including removal/disable semantics.

### Tests through Interface

- Project scalar overrides user scalar.
- Project list replaces user list.
- `*_append` appends deterministically.
- Project `disabled: true` shadows user agent only in that project.
- Session state wins over project/root state.

---

## 4. Agent YAML Schema and Merge Semantics

### Module

`internal/registry`

### Interface

Agent definitions are data, not Go code. The registry loads built-in, user, and project YAML into normalized canonical agents.

### Decision

Use a small author-friendly schema. Normalize internally for downstream Modules.

### Minimal schema

```yaml
# Required
name: architect

description: "System design, seams, tradeoffs, and hard debugging"

# Optional identity/routing
aliases: [oracle, arch]
category: advisor
autonomy: cautious
cost: expensive
mode: subagent

# Optional model selection: simple case
model: anthropic/claude-opus-4-7

# Optional model selection: advanced case; if present, model is ignored
model_chain:
  - model: anthropic/claude-opus-4-7
    variant: max
  - model: openai/gpt-5.5
    variant: high

# Optional tool policy: allowlist case
tools: [read, grep, glob, lsp_diagnostics]

# Optional tool policy: all-except case
tools_except: []

# Optional route hints
triggers:
  - domain: "Architecture decisions"
    pattern: "architecture|seam|interface|tradeoff"
    priority: 10

useWhen:
  - "Complex Module or Interface design"
  - "Two or more failed fix attempts"

avoidWhen:
  - "Simple file edits"
  - "First attempt at a straightforward bug"

disabled: false
```

### Field semantics

#### `name`

- Required.
- Canonical unique ID.
- Must be stable.

#### `description`

- Required for active agents.
- Used in prompt tables and `agents show`.

#### `aliases`

- Optional.
- Used for routing and compatibility with OMX/OMO names.
- Alias collision is fatal.

#### `category`

- Optional.
- Describes role/domain, for example: `advisor`, `exploration`, `execution`, `planning`, `review`, `utility`.
- Replaces the need for a separate `posture` field in v1.

#### `autonomy`

- Optional.
- Describes execution behavior and guard posture: `autonomous`, `cautious`, `ask`.
- This is the field that can influence guard/routing policy.

#### `model` and `model_chain`

- `model` is the simple authoring path.
- `model_chain` is the advanced fallback path.
- If `model_chain` exists, loader ignores `model` and emits a warning in non-strict mode.
- Internally both normalize to `[]ModelCandidate`.
- If neither exists, category/default model resolution applies.

#### `tools` and `tools_except`

Tool policy has one of two shapes:

Allowlist:

```yaml
tools: [read, grep, glob]
```

- Only listed tools are allowed.
- Tools not listed are blocked.
- `tools_except` is invalid with list-shaped `tools`.

All-except:

```yaml
tools: all
tools_except: [write, edit, apply_patch]
```

- All tools are allowed except listed tools.
- `tools_except` is valid only when `tools: all`.

Default:

- If neither is specified, inherit from category/default policy.

#### `triggers`

- Optional route hints.
- Registry treats `trigger.pattern` as an opaque string.
- The Route Module owns interpretation of pattern syntax. It may start with simple substring or regex-like matching, but that is not a registry concern.
- Higher priority wins when multiple triggers match after route interpretation.

#### `disabled`

- Optional boolean.
- Scope-local.
- Project disable shadows user/built-in definition only in that project.

### Removed from v1 schema

#### `posture`

Rejected for v1.

Reason:

- `category` already expresses role/domain.
- `autonomy` expresses execution behavior.
- Keeping both `posture` and `autonomy` creates invalid combinations such as advisor + autonomous unless extra validation rules are added.
- Removing `posture` keeps the Interface smaller and deeper.

#### `tools_allowed` / `tools_blocked`

Rejected for v1.

Reason:

- Having both creates conflict cases such as allowing and blocking the same tool.
- That requires priority rules.
- `tools` allowlist plus `tools: all` / `tools_except` covers common cases with fewer contradictions.

### Merge semantics

- `name` identifies the canonical agent.
- Aliases are indexed globally after merge.
- Scalar fields override.
- Map fields deep merge.
- List fields replace.
- `*_append` appends to list fields when supported.
- `disabled: true` shadows the agent in current scope.
- Unknown fields are warnings by default.
- Unknown fields are fatal under strict validation.

### Validation severity

Fatal:

- missing `name`
- missing `description` for active final agent
- duplicate canonical `name` at the same precedence layer when not an override
- alias collision across active final agents
- invalid enum value
- `tools_except` used when `tools` is a list

Warning:

- unknown field in non-strict mode
- both `model` and `model_chain` present; `model_chain` wins
- unused alias due to disabled final agent

Strict mode:

```bash
omg agents validate --strict
```

In strict mode, warnings become fatal unless explicitly allowlisted later.

### Why references do not close this

- OMX agent definitions are mostly installed/static role files.
- OMO agent overrides are centered on fixed OpenCode agent names.
- OMG needs user-defined agents, aliases, scope-local disables, and host-neutral tool/model policy.

### Tests through Interface

- Load a single valid agent.
- Merge built-in + user + project override.
- Project disables user agent locally.
- Alias collision fails.
- List replace works.
- `tools: all` with `tools_except` works.
- `tools_except` with list-shaped `tools` fails.
- `model_chain` overrides `model` with warning.
- Strict mode turns warning into fatal.

---

## 5. P0 Normalized Internal Types

These are not final Go structs, but they describe the target normalized shapes.

### ModelCandidate

```go
type ModelCandidate struct {
    Model           string         `json:"model" yaml:"model"`
    Variant         string         `json:"variant,omitempty" yaml:"variant,omitempty"`
    ReasoningEffort string         `json:"reasoningEffort,omitempty" yaml:"reasoningEffort,omitempty"`
    Params          map[string]any `json:"params,omitempty" yaml:"params,omitempty"`
}
```

### ToolPolicy

```go
type ToolPolicy struct {
    Mode    string   // "allowlist" or "all_except"
    Allow   []string // populated for allowlist mode
    Except  []string // populated for all_except mode
    Source  string   // built-in, user, project
}
```

### Agent

```go
type Agent struct {
    Name        string
    Description string
    Aliases     []string
    Category    string
    Autonomy    string
    Cost        string
    Mode        string
    Models      []ModelCandidate
    Tools       ToolPolicy
    Triggers    []Trigger
    UseWhen     []string
    AvoidWhen   []string
    Disabled    bool
    Sources     []string
}
```

---

## 6. Remaining P0 implementation notes

- Do not add a `posture` field unless a future real use case proves `category` and `autonomy` are insufficient.
- Do not add simultaneous allow/block tool lists.
- Keep host-specific payloads under `payload`.
- Keep daemon auto-start out of hook commands.
- Keep inline fallback cheap and deterministic.
- Keep project disable scope-local.
- Keep strict validation available before large-scale config sharing.

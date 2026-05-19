# Oh-My-Gunnsama P1 Interface Closure

Status: P1 product-interface decisions  
Scope: decisions needed before Phase 2/3 prompt/routing/adapter/error integration  
Principle: keep Interfaces deep; keep adapters thin; keep routing, prompting, fallback, and guard policy separate

## 0. Context

P0 is already closed in `.omx/plans/oh-my-gunnsama-p0-interface-closure.md`.

This P1 closure resolves:

5. Prompt build input/output contract
6. Route result contract
7. Host Adapter contract
8. Error taxonomy for fallback/guard/state decisions

These decisions incorporate the review outcome:

- Procedure means what user work is being requested, not how OMG internally handles it.
- Prompt builder receives resolved route/context, not raw user-message authority.
- Host Adapters translate protocol only; they do not make core decisions.
- Error normalization is separate from fallback/retry/state-transition policy.
- `Degraded` is not an error class in v1; use health/warning signals outside the main error flow.

---

## 1. Prompt Build Input/Output Contract

### Module

`internal/prompt`

### Interface

Prompt builder assembles a final system prompt from already-resolved context. It does not route, mutate state, call providers, or inspect host-specific payloads.

### Decision

Use a narrow build input and a debuggable build output.

```go
type BuildPromptInput struct {
    Agent       AgentRef
    Model       ModelInfo
    Project     ProjectContext
    Context     *PromptContextSnapshot
    Route       RouteResult
    Flags       map[string]string
}
```

```go
type BuildPromptOutput struct {
    SystemPrompt    string
    Sections        []PromptSection
    OmittedSections []PromptSection
    Warnings        []string
    TokenEstimate   int
}
```

```go
type PromptSection struct {
    Name     string
    Source   string
    Tokens   int
    Required bool
    Priority int
}
```

### Field semantics

#### `Agent`

- Canonical agent selected by routing or explicit CLI flag.
- Prompt builder may use agent metadata, aliases, tools policy, triggers, use/avoid hints, and model guidance already loaded by registry.

#### `Model`

- The selected model candidate after model/fallback resolution for this prompt build.
- Prompt builder may use model family for model-calibration sections.

#### `Project`

- Prompt-facing project context loaded from `.omg/` and project files.
- Contains rules/conventions/templates relevant to prompt assembly.
- Does not expose raw config loader internals.

#### `Context`

- Optional read-only prompt context snapshot.
- Replaces raw `SessionState` to avoid coupling prompt builder to state internals.
- Contains only text/metadata needed for prompt assembly.

#### `Route`

- Route result is an input.
- Prompt builder never calls router.
- Prompt builder may include route metadata in debug output or conditional sections.

#### `Flags`

- Explicit CLI or host flags that affect prompt assembly.
- Flags must be simple and documented; complex policy belongs to route/config/state modules.

### Explicitly excluded from input

#### Raw `UserMessage`

Rejected as a normal input.

Reason:

- If prompt builder sees raw user input, it will be tempted to perform routing or classification.
- Routing owns interpretation of user intent.
- If a future debug mode needs the original message, pass it as debug metadata that section selection cannot read.

#### Raw `SessionState`

Rejected.

Reason:

- State structure will evolve.
- Prompt builder only needs a read-only prompt snapshot.
- This preserves Locality in `internal/state`.

### Token budget rule

Token trimming is allowed only for optional sections.

- Required sections must never be omitted for budget reasons.
- Optional sections may be omitted by ascending priority.
- Omitted sections must be reported in `OmittedSections` and `Warnings`.
- Identity, safety, tool policy, and hard-block sections should normally be `Required: true`.

### Invariants

- `BuildPrompt` is deterministic for the same input and same template/registry files.
- It does not write files.
- It does not call router, state writer, provider, daemon, MCP, or host adapter.
- It returns section metadata for debug and testability.

### Filesystem effects

- Reads templates and prompt-facing project context.
- No writes.

### Deletion test

If `internal/prompt` were deleted, section ordering, token trimming, model calibration, project rules inclusion, and debug metadata would spread across hooks, route, CLI commands, and adapters. Therefore this Module earns its keep.

### Tests through Interface

- Build prompt from fixture agent/model/project/route.
- Verify required sections are included.
- Verify optional section omission under token budget.
- Verify `Sections`, `OmittedSections`, `Warnings`, and `TokenEstimate` are populated consistently.
- Verify raw user message is not required for build.

---

## 2. Route Result Contract

### Module

`internal/route`

### Interface

Router returns a pure, non-mutating description of what procedure the user input requests and which domain/agent appears relevant.

### Decision

Use user-work procedure names, not internal mechanism names.

```go
type RouteResult struct {
    Procedure    Procedure
    Domain       string
    Agent        string
    Confidence   Confidence
    Score        *float32
    Reasons      []string
    Triggers     []string
    RequiresUser bool
}
```

```go
type Procedure string

const (
    ProcedureInterview Procedure = "interview"
    ProcedurePlan      Procedure = "plan"
    ProcedureExecute   Procedure = "execute"
    ProcedureReview    Procedure = "review"
    ProcedureVerify    Procedure = "verify"
    ProcedureCancel    Procedure = "cancel"
    ProcedureNone      Procedure = "none"
)
```

```go
type Confidence string

const (
    ConfidenceLow    Confidence = "low"
    ConfidenceMedium Confidence = "medium"
    ConfidenceHigh   Confidence = "high"
)
```

### Rejected procedure values

#### `route`

Rejected.

Reason:

- Every output is already a route result.
- `route` describes the mechanism, not the requested work.

#### `delegate`

Rejected.

Reason:

- Delegation is an implementation strategy for `execute`, `review`, or `verify`.
- It does not describe the user-requested procedure.

#### `guard`

Rejected.

Reason:

- Guard is a tool-event policy decision, not user-intent routing.

#### `passthrough`

Rejected as a procedure.

Reason:

- Passthrough may be a host response/action, but not a user-work procedure.
- Use `ProcedureNone` with low/medium confidence when no OMG behavior is needed.

### Score and Confidence

- `Confidence` is the policy-facing field.
- `Score` is optional debug metadata for scoring experiments and threshold tuning.
- Core policies should not require `Score` to exist.
- If both exist, `Confidence` is the source of truth for v1 decisions.

### RequiresUser

`RequiresUser` means routing found a blocking ambiguity or safety concern that should ask the user before executing.

It must not be used for ordinary low-confidence internal choices that can safely become `ProcedureNone` or cautious prompt guidance.

### Invariants

- Router does not mutate state.
- Router does not build prompts.
- Router does not call providers.
- Router may read registry/category/trigger metadata.
- Route explanations must be human-debuggable.

### Deletion test

If `internal/route` were deleted, keyword and category logic would scatter across hooks, prompt templates, CLI commands, and guard rules. Therefore this Module earns its keep.

### Tests through Interface

- Clear interview request routes to `interview`.
- Clear implementation request routes to `execute`.
- Review request routes to `review`.
- Cancel request routes to `cancel`.
- Ambiguous request returns low/medium confidence and may set `RequiresUser`.
- Guard/tool events do not produce `ProcedureGuard` because no such procedure exists.

---

## 3. Host Adapter Contract

### Module

`internal/hooks`, host-specific command glue, `bridge/opencode`

### Interface

Host Adapters translate between host event/response formats and the OMG protocol envelope. They do not own routing, prompt building, fallback, state transitions, or guard policy.

### Decision

Adapters are intentionally thin.

Go-side conceptual interface:

```go
type HostAdapter interface {
    Name() string
    Capabilities() HostCapabilities
    TranslateEvent(raw []byte) (*Request, error)
    TranslateResponse(resp *Response) ([]byte, error)
    HealthCheck() HostStatus
}
```

```go
type HostCapabilities struct {
    SupportsSystemTransform bool
    SupportsExecHook        bool
    MaxHookTimeout          time.Duration
    SupportsStreaming       bool
}
```

### Important note for OpenCode bridge

The OpenCode JS bridge does not implement a Go interface. It follows the wire protocol and the same behavioral contract:

- translate OpenCode callback input into OMG request envelope
- send request to socket or invoke CLI fallback if designed
- translate OMG response into OpenCode callback output
- keep routing/prompt/fallback/guard decisions out of JS

### Adapter MUST

- Convert host event into OMG protocol envelope.
- Convert OMG response into host-expected output.
- Report host connection/capability status.
- Preserve host-specific raw data under `payload` where useful.
- Respect hook timeout budgets.

### Adapter MAY

- Perform minimal raw error wrapping into `OmgError` when the error is host-specific and cannot be understood elsewhere.
- Emit host-specific diagnostics to logs/stderr.
- Apply `inject_prompt`, `block`, `warn`, `replace`, or `none` actions in the host’s required format.

### Adapter MUST NOT

- Decide route.
- Build prompts.
- Select prompt sections.
- Mutate prompt text by local policy.
- Run fallback/retry policy.
- Mutate `.omg/state` directly.
- Load agents/skills/categories directly except as required for explicit install/validation commands.
- Execute or delegate agents.
- Decide tool guard policy.

### Prompt application clarification

Adapters may apply a completed OMG prompt response to the host. They must not generate, classify, or modify the prompt content themselves.

### Invariants

- All business decisions flow through core Modules or explicit CLI commands.
- Host-specific capability differences are data, not branches scattered through core.
- JS bridge remains a thin Adapter, not a second OMG implementation.

### Deletion test

If Host Adapters were deleted, host-specific payload decoding, timeout handling, and response formatting would spread into protocol, prompt, route, and daemon Modules. Therefore Host Adapters earn their keep, but only as thin translation Modules.

### Tests through Interface

- Codex/Claude exec payload translates to protocol request.
- OpenCode bridge fixture translates to protocol request.
- `inject_prompt` response translates to host output.
- `block` response translates to host output.
- Adapter does not call prompt builder or router in unit tests.

---

## 4. Error Taxonomy

### Module

`internal/errors`, consumed by `internal/fallback`, `internal/guard`, `internal/state`, `internal/protocol`, and Host Adapters

### Interface

Errors are normalized into one OMG error shape. Adapters and lower modules may normalize raw errors, but core policy modules decide fallback, retry, guard response, and state transition.

### Decision

Use a small ErrorClass set plus ErrorDomain. Do not use `Degraded` as an error class in v1.

```go
type ErrorClass string

const (
    ErrorRetryable    ErrorClass = "retryable"
    ErrorStop         ErrorClass = "stop"
    ErrorNonRetryable ErrorClass = "non_retryable"
)
```

```go
type ErrorDomain string

const (
    DomainModel  ErrorDomain = "model"
    DomainConfig ErrorDomain = "config"
    DomainSkill  ErrorDomain = "skill"
    DomainState  ErrorDomain = "state"
    DomainGuard  ErrorDomain = "guard"
    DomainHost   ErrorDomain = "host"
    DomainFS     ErrorDomain = "filesystem"
    DomainUnknown ErrorDomain = "unknown"
)
```

```go
type OmgError struct {
    Class   ErrorClass
    Domain  ErrorDomain
    Code    string
    Message string
    Source  string
    Cause   error
}

func (e OmgError) IsRetryable() bool {
    return e.Class == ErrorRetryable
}
```

### No `Retryable bool`

Rejected.

Reason:

- `Class` is the source of truth.
- A separate `Retryable` field can contradict `Class`.
- Use `IsRetryable()` convenience method instead.

### No `ErrDegraded` class in v1

Rejected as error class.

Reason:

- Degraded means the system can continue with reduced capability.
- That is better represented as health status, warning, or partial capability signal.
- Keeping it outside error class avoids contradictory combinations like stop+warning.

Possible future shape:

```go
type HealthStatus string

const (
    HealthOK       HealthStatus = "ok"
    HealthDegraded HealthStatus = "degraded"
    HealthDown     HealthStatus = "down"
)
```

### Adapter/core responsibility split

Adapters may:

- Convert raw host/provider error into `OmgError`.
- Attach host-specific code/source/message.

Adapters must not:

- Execute fallback.
- Retry.
- Transition workflow state.
- Decide whether to ask user.

Core policy modules own:

- fallback candidate selection
- retry/no-retry decision
- guard action mapping
- state transition mapping
- user-visible response construction

### Suggested code examples

- `rate_limit`: `Class=retryable`, `Domain=model`
- `provider_unavailable`: `Class=retryable`, `Domain=model` or `host`
- `quota_exceeded`: `Class=stop`, `Domain=model`
- `invalid_config`: `Class=non_retryable`, `Domain=config`
- `policy_blocked`: `Class=non_retryable`, `Domain=guard`
- `state_corrupt`: `Class=stop` or `non_retryable`, `Domain=state`, depending on recoverability

### Precedence

When multiple errors are aggregated:

1. `stop`
2. `non_retryable`
3. `retryable`

Warnings and degraded health signals are reported separately from this ordering.

### Wire protocol relation

P0 wire protocol keeps `error.class` string-typed. P1 internal `ErrorClass` maps to that field where appropriate. Unknown wire error classes must be handled conservatively.

### Invariants

- Fallback is triggered only by policy modules after seeing normalized error.
- Fallback remains infra-failure-only.
- Error normalization must not become answer-quality retry logic.

### Deletion test

If normalized error taxonomy were deleted, fallback, guard, host adapters, protocol responses, and state transitions would each invent incompatible error strings. Therefore this Module earns its keep.

### Tests through Interface

- Retryable model error selects fallback candidate in fallback module.
- Stop model error does not fallback.
- Invalid config error does not fallback.
- Host adapter normalizes raw timeout without executing retry.
- Aggregated stop+retryable errors produce stop precedence.
- Degraded health is reported without becoming `ErrorClass`.

---

## 5. Remaining P1 implementation notes

- Keep `BuildPromptInput` narrow.
- Keep route pure and non-mutating.
- Keep Host Adapters thin.
- Keep `Confidence` enum as policy-facing and `Score` as optional debug metadata.
- Keep `Degraded` out of the error class flow in v1.
- Keep fallback/retry/state transition in core policy modules.
- Do not let OpenCode JS bridge grow business logic.

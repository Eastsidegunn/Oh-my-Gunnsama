# Oh-My-Gunnsama P2 Interface Closure

Status: P2 product-interface decisions  
Scope: decisions needed after early implementation but before scale  
Principle: state must not lie, skills provide capability, guards decide structurally, verification reports evidence

## 0. Context

P0 is closed in `.omx/plans/oh-my-gunnsama-p0-interface-closure.md`.
P1 is closed in `.omx/plans/oh-my-gunnsama-p1-interface-closure.md`.

This P2 closure resolves:

9. State transition model
10. Skill schema and MCP lifecycle
11. Guard decision schema
12. Verification plan contract

These decisions incorporate the review outcome:

- Daemon death or stale heartbeat is not proof of task failure.
- Skill definitions should not select agents or models in v1.
- Guard errors follow the P0 fail-open/fail-closed distinction.
- Guard suppressions, if present in v1, must be project-local, scoped, and reasoned.
- Verification collects evidence; fix/retry loops belong to orchestration.

---

## 1. State Transition Model

### Module

`internal/state`

### Interface

State manager owns lifecycle persistence, legal transitions, session/root precedence, stale-owner handling, and compatibility projections.

Workflow context is separate from lifecycle. Lifecycle says what condition the session is in; context says what kind of work is running.

### Decision

Use a small lifecycle enum with an explicit cancellation terminal state.

```go
type Lifecycle string

const (
    LifecycleInactive  Lifecycle = "inactive"
    LifecycleRunning   Lifecycle = "running"
    LifecycleBlocked   Lifecycle = "blocked"
    LifecycleAskUser   Lifecycle = "ask_user"
    LifecycleFinished  Lifecycle = "finished"
    LifecycleFailed    Lifecycle = "failed"
    LifecycleCancelled Lifecycle = "cancelled"
)
```

Use context separately:

```go
type StateRecord struct {
    Lifecycle     Lifecycle        `json:"lifecycle"`
    Context       string           `json:"context,omitempty"` // plan, interview, execute, verify, etc.
    SessionID     string           `json:"session_id,omitempty"`
    OwnerID       string           `json:"owner_id,omitempty"`
    RequestID     string           `json:"request_id,omitempty"`
    UpdatedAt     time.Time        `json:"updated_at"`
    HeartbeatAt   time.Time        `json:"heartbeat_at,omitempty"`
    BlockedReason string           `json:"blocked_reason,omitempty"`
    Error         *OmgErrorSummary `json:"error,omitempty"`
    Metadata      map[string]any   `json:"metadata,omitempty"`
}
```

### Legal transitions

```text
inactive  -> running
running   -> blocked | ask_user | finished | failed | cancelled
blocked   -> running | failed | cancelled
ask_user  -> running | failed | cancelled
finished  -> terminal
failed    -> terminal
cancelled -> terminal
```

Terminal states may be cleared by an explicit admin/user state clear operation, but normal workflow transition logic must treat them as terminal.

### Stale owner / heartbeat handling

Daemon restart, stale owner, or heartbeat expiration is not proof of failed work.

Rules:

- `running + stale heartbeat` transitions to `blocked`.
- `blocked_reason` should be one of:
  - `stale_owner`
  - `daemon_restarted`
  - `heartbeat_expired`
- `failed` requires actual failure evidence or explicit error classification.
- Heartbeat timeout is configurable.
- Recommended default heartbeat timeout: 120 seconds.
- Long-running checks may set a longer timeout through state/config, but must still heartbeat when possible.

### Overlap policy

Do not inherit OMX-style workflow overlap in v1.

- One session has one lifecycle and one primary context.
- Parallel work should use separate sessions or explicit child-task records.
- If future overlap is needed, add it as an orchestrator feature, not as implicit state overlap.

### Persistence

Adopt the proven OMX pattern conceptually:

- atomic write using temporary file + rename
- per-file write queue or equivalent lock
- session state wins over project/root state
- compatibility projections are read-only unless explicitly generated

### Invariants

- State transitions are explicit and testable.
- State manager does not run agents, build prompts, or call providers.
- Stale daemon/process conditions do not become `failed` without evidence.
- `cancelled` is distinct from `failed` and `finished`.

### Filesystem effects

- Writes under `{project}/.omg/state/`.
- Writes session scoped state under `{project}/.omg/state/sessions/<session_id>/`.
- May generate read-only compatibility projections.

### Deletion test

If `internal/state` were deleted, lifecycle values, stale-owner handling, atomic writes, session precedence, cancellation semantics, and compatibility projections would spread across daemon, route, guard, verification, and adapters. Therefore this Module earns its keep.

### Tests through Interface

- `running -> blocked` on heartbeat expiration.
- Daemon restart marks stale running state as blocked, not failed.
- Explicit failure evidence transitions to failed.
- Explicit cancellation transitions to cancelled.
- Session state wins over project/root state.
- Atomic write does not leave partial state on simulated failure.

---

## 2. Skill Schema and MCP Lifecycle

### Module

`internal/skills`, `internal/registry`, `internal/mcp` or `internal/skillmcp`

### Interface

Skill loader reads skill definitions and prompt text. MCP manager owns MCP process/client lifecycle. Skill use may request MCP capability, but skill load does not start MCP.

### Decision

In v1, skills provide capability. They do not select agents or models.

Minimal `skill.yaml`:

```yaml
name: ast-search
description: "AST-based code pattern search"

triggers:
  - pattern: "ast|structure|pattern search"
    priority: 10

prompt: prompt.md

mcp:
  server: ast-grep-server
  tools: [ast_search]
  env_inherit: [PATH]
```

### Field semantics

#### `name`

- Required.
- Canonical skill ID.
- Must be unique after precedence/merge.

#### `description`

- Required.
- Used by prompt builder and `skills list`.

#### `triggers`

- Optional.
- Routing/prompt hint only.
- Does not force skill activation by itself.

#### `prompt`

- Optional but recommended.
- Path relative to skill directory.
- Loader returns prompt text to prompt builder or skill tool.

#### `mcp`

- Optional.
- Declares MCP capability needed by this skill.
- Does not start the server during skill load.

### Explicitly excluded in v1

#### `agent`

Rejected as a strong binding.

Reason:

- Skill reuse would suffer if a skill selected one agent.
- Agent selection belongs to route/prompt/orchestration.

Possible future additions may use weak compatibility hints such as `recommended_agents`, but not v1 hard binding.

#### `model`

Rejected.

Reason:

- P0 closes model selection through agent/category/model or model_chain.
- Skill-level model selection would create a second model-resolution seam.

### MCP lifecycle

Skill loader:

- reads `skill.yaml`
- reads prompt file if configured
- parses MCP metadata if configured
- returns metadata and prompt text
- does not start MCP

MCP manager:

- starts/connects lazily when skill use needs MCP
- reuses clients/processes where safe
- shuts down idle clients/processes after configured idle timeout
- default idle timeout: 5 minutes
- cleans up zombie processes during daemon heartbeat/maintenance
- restarts on next use if a process died
- inherits only explicitly allowlisted environment variables

### Hot reload

No implicit file watcher in v1.

- Skill changes are picked up by explicit `omg reload`, daemon restart, or direct command reload path.
- Avoid file watcher complexity until the daemon/core is stable.

### Invariants

- Loading a skill has no external process side effects.
- MCP lifecycle is owned by MCP manager, not skill loader.
- Skills are capabilities, not agent/model selectors.
- Environment inheritance is allowlist-based.

### Filesystem effects

- Reads `~/.omg/skills/<name>/` and `{project}/.omg/skills/<name>/`.
- Does not write during ordinary skill load.
- MCP manager may write runtime state/logs under `.omg/`.

### Deletion test

If skill loader were deleted, prompt text discovery, trigger metadata, MCP metadata parsing, and user/project skill precedence would spread across prompt builder, route, MCP manager, and adapters. Therefore the Module earns its keep.

If MCP manager were deleted, process/client lifecycle, idle cleanup, env allowlist, reconnection, and zombie cleanup would spread across each skill. Therefore MCP manager earns its keep.

### Tests through Interface

- Load skill metadata and prompt text without starting MCP.
- Skill with `mcp` triggers lazy MCP start only on use.
- Idle MCP client shuts down after configured timeout.
- Dead MCP process is removed from state and restarted on next use.
- Non-allowlisted env vars are not inherited.
- Skill schema rejects model/agent hard binding in strict mode.

---

## 3. Guard Decision Schema

### Module

`internal/guard`

### Interface

Guard evaluates tool/action input and returns a structured decision. It does not rewrite tool arguments in v1.

### Decision

Use structured decisions with action precedence.

```go
type GuardInput struct {
    RequestID string         `json:"request_id"`
    Project   string         `json:"project"`
    CWD       string         `json:"cwd"`
    Tool      string         `json:"tool"`
    Args      map[string]any `json:"args"`
    Agent     string         `json:"agent,omitempty"`
    Autonomy  string         `json:"autonomy,omitempty"`
    SessionID string         `json:"session_id,omitempty"`
}
```

```go
type GuardAction string

const (
    GuardAllow GuardAction = "allow"
    GuardWarn  GuardAction = "warn"
    GuardBlock GuardAction = "block"
)
```

```go
type GuardSeverity string

const (
    SeverityInfo    GuardSeverity = "info"
    SeverityWarning GuardSeverity = "warning"
    SeverityDanger  GuardSeverity = "danger"
)
```

```go
type GuardDecision struct {
    Action   GuardAction   `json:"action"`
    Severity GuardSeverity `json:"severity"`
    Reason   string        `json:"reason"`
    RuleID   string        `json:"rule_id"`
}
```

### Merge / precedence

All applicable guards may run. Final decision uses strictest action:

```text
block > warn > allow
```

Reasons from non-winning guard results may be retained in debug metadata, but host-facing action is the strictest result.

### Rewrite policy

No rewrite in v1.

- Guard reads input and returns a decision.
- Guard does not mutate arguments.
- If rewrite becomes necessary later, add a new explicit action and audit trail.

### Guard internal error policy

Follow P0.

- Destructive or mutating tool/event + guard internal error -> block.
- Non-destructive tool/event + guard internal error -> warn or allow with warning.
- Unknown risk should be treated conservatively, but not every guard error should halt read-only work.

### Suppression policy

Suppression is allowed only in a constrained form in v1.

Allowed:

- project-local YAML declaration
- explicit `reason` required
- scope required, preferably `paths` and/or `tools`
- no global suppressions
- no ad-hoc CLI suppression command in v1

Example:

```yaml
guard_suppressions:
  - rule_id: write-existing-file
    reason: "Generated fixture files are intentionally overwritten"
    scope: project
    paths:
      - "testdata/generated/**"
    tools:
      - write
```

Rejected in v1:

- `omg guard suppress <rule_id>` one-off command
- global user-level suppression without project review
- suppression without reason
- suppression with no path/tool scope

### Invariants

- Guard does not build prompts, route user requests, call providers, or mutate state except via explicit audit/log path.
- Guard decisions are host-neutral.
- Host adapters apply guard decisions but do not make guard policy.
- Guard input includes request/project/cwd for traceability.

### Filesystem effects

- Ordinary guard evaluation is read-only except logs/audit.
- Suppression config is read from project config.
- Guard audit may write under `.omg/logs/` if enabled.

### Deletion test

If `internal/guard` were deleted, destructive command policy, file safety, autonomy-sensitive checks, and suppression/audit rules would spread across host adapters and tools. Therefore this Module earns its keep.

### Tests through Interface

- Read-only safe tool returns allow.
- Risky mutating tool returns block.
- Warning-only guard returns warn.
- Multiple guard decisions merge as block > warn > allow.
- Guard internal error blocks destructive tool.
- Guard internal error does not block safe read-only tool by default.
- Project-local suppression with reason and path scope suppresses only matching rule/path/tool.
- Suppression without reason is rejected in strict validation.

---

## 4. Verification Plan Contract

### Module

`internal/verify`

### Interface

Verifier runs declared or detected checks and reports evidence. It does not fix failures and does not own retry/fix-loop policy.

### Decision

Use data-driven verification plans with argv-based commands by default.

```go
type VerificationPlan struct {
    Checks []VerificationCheck `json:"checks" yaml:"checks"`
}
```

```go
type VerificationCheck struct {
    ID       string        `json:"id" yaml:"id"`
    Name     string        `json:"name" yaml:"name"`
    Command  []string      `json:"command" yaml:"command"`
    Shell    bool          `json:"shell,omitempty" yaml:"shell,omitempty"`
    CWD      string        `json:"cwd,omitempty" yaml:"cwd,omitempty"`
    Timeout  time.Duration `json:"timeout" yaml:"timeout"`
    Claim    string        `json:"claim" yaml:"claim"`
    Required bool          `json:"required" yaml:"required"`
}
```

### Command execution policy

Default:

- `Command` is argv.
- Execute with `exec.Command(command[0], command[1:]...)` or equivalent.
- `Shell=false` by default.

Shell mode:

- `Shell=true` must be explicit.
- Shell commands are higher risk and should be guardable/auditable.
- Shell mode may be useful for compatibility but should not be the default.

Example `.omg/verify.yaml`:

```yaml
checks:
  - id: go-test
    name: Go tests
    command: ["go", "test", "./..."]
    timeout: 120s
    claim: "Go packages compile and tests pass"
    required: true

  - id: docs-smoke
    name: Documentation smoke check
    command: ["true"]
    timeout: 5s
    claim: "Documentation-only change has explicit smoke placeholder"
    required: false
```

### Retry / fix loop policy

Verifier does not own retry count.

- `VerificationPlan` says what to check.
- `VerificationRunner` runs checks and collects evidence.
- Orchestrator/workflow owns fix loops, max retries, and whether to ask user.

Rejected from verifier v1:

- `MaxRetries` in `VerificationPlan`
- silent fixes
- automatic re-run after modifying code

### Evidence model

```go
type EvidenceKind string

const (
    EvidenceCommand  EvidenceKind = "command"
    EvidenceManual   EvidenceKind = "manual"
    EvidenceArtifact EvidenceKind = "artifact"
)
```

```go
type CheckEvidence struct {
    CheckID         string        `json:"check_id"`
    Kind            EvidenceKind  `json:"kind"`
    Passed          bool          `json:"passed"`
    Command         []string      `json:"command,omitempty"`
    Shell           bool          `json:"shell,omitempty"`
    ExitCode        int           `json:"exit_code,omitempty"`
    Stdout          string        `json:"stdout,omitempty"`
    Stderr          string        `json:"stderr,omitempty"`
    OutputTruncated bool          `json:"output_truncated,omitempty"`
    Artifacts       []string      `json:"artifacts,omitempty"`
    StartedAt       time.Time     `json:"started_at"`
    Duration        time.Duration `json:"duration"`
    Reason          string        `json:"reason,omitempty"`
}
```

```go
type VerificationResult struct {
    Passed   bool            `json:"passed"`
    Evidence []CheckEvidence `json:"evidence"`
    Summary  string          `json:"summary"`
}
```

### Evidence rules

- Evidence is required for pass.
- Required check failure makes result fail.
- Optional check failure records warning/evidence but does not fail the whole result.
- Manual evidence is allowed but must include `Reason`.
- Artifact evidence must include at least one artifact path.
- Command evidence should include exit code, duration, and stdout/stderr, with truncation flag when applicable.

### Project plan and auto-detection

Sources:

1. project `.omg/verify.yaml`
2. built-in auto-detection fallback

Auto-detection examples:

- `go.mod` -> `go test ./...`
- `package.json` -> configured package manager test/build if safely detectable
- otherwise no command checks unless project config declares them

Auto-detection must be conservative and visible in evidence/warnings.

### Invariants

- Verifier does not modify source files.
- Verifier does not run fix loops.
- Verifier reports evidence, warnings, and failures.
- Verification command execution is guardable/auditable.
- No evidence means no pass.

### Filesystem effects

- Reads `.omg/verify.yaml`.
- Runs configured commands.
- May write verification logs/artifacts under `.omg/logs/` or `.omg/verification/` if configured.
- Does not edit project source.

### Deletion test

If `internal/verify` were deleted, check discovery, command execution safety, evidence structure, optional/required semantics, and pass/fail proof would spread across orchestrator, agents, and prompts. Therefore this Module earns its keep.

### Tests through Interface

- Required passing command produces passing result with evidence.
- Required failing command fails result.
- Optional failing command does not fail whole result but records evidence.
- Shell command is rejected or flagged unless `Shell=true`.
- Manual evidence without reason is rejected.
- Artifact evidence without artifact path is rejected.
- Output truncation sets `OutputTruncated=true`.
- Verifier does not retry or modify files.

---

## 5. Remaining P2 implementation notes

- Keep lifecycle and workflow context separate.
- Keep `cancelled` distinct from `failed` and `finished`.
- Keep stale daemon/heartbeat as blocked unless failure evidence exists.
- Keep skills as capabilities; no v1 skill-level model or hard agent binding.
- Keep MCP lifecycle lazy and manager-owned.
- Keep guard decisions structured and non-rewriting.
- Keep guard suppression constrained, project-local, reasoned, and scoped.
- Keep verification argv-first.
- Keep fix/retry loop outside verifier.
- Keep evidence required for pass.

# OmX / OmO에서 OmG로 가져올 가능성이 있는 요소 정리

작성일: 2026-05-15

## 0. 전제

이 문서는 **OmG를 메인 AI 개발 호스트로 만든다**는 방향에서, OmX / OmO의 기능을 그대로 복제할지 여부가 아니라 **어떤 개념과 패턴을 가져올 가치가 있는지** 정리한다.

핵심 기준:

- OmG의 중심은 사용자의 취향대로 동작하는 메인 호스트다.
- OmX / OmO는 참고 구현, 비교 대상, 일부 백엔드/패턴 차용 대상이다.
- MCP는 agent 간 통신망이라기보다 agent-to-tool capability layer로 본다.
- agent 간 협업은 task queue, mailbox, event log, shared state, leader/worker protocol 같은 runtime 구조가 중심이다.
- 가져올 때는 “그대로 복사”보다 “OmG식으로 재해석”한다.

---

## 1. 한 장 요약

| 영역 | OmX에서 가져올 후보 | OmO에서 가져올 후보 | OmG 판단 |
|---|---|---|---|
| Skill | skill metadata + prompt + MCP capability 분리 | skill이 context + MCP tool을 같이 가져오는 UX | **가져옴**. 단 skill은 agent/model을 강제하지 않는 capability 단위로 유지 |
| MCP | lazy MCP lifecycle manager | built-in MCP + skill-embedded MCP | **가져옴**. agent 간 통신이 아니라 tool/capability 연결로 사용 |
| Agent | role prompt catalog, team role prompts | Sisyphus/Explore/Librarian 등 강한 agent persona와 delegation rules | **선별 차용**. 고정 agent set이 아니라 OmG agent catalog로 재정의 |
| 작업 큐 | team task state, claim, transition | task system, blockedBy/blocks, background task | **강하게 가져옴**. OmG main host runtime의 중심 |
| Mailbox | team API mailbox/send/broadcast | 상대적으로 약함. background result/session 중심 | **OmX 패턴 차용**. worker 보고/질문/leader 지시 통로 |
| Event log | team event append/read/await | task/session/poller logs | **가져옴**. audit/recovery/observer의 핵심 |
| Shared state | team monitor/task/worker state | background task/session/concurrency state | **가져옴**. 현재 상황판 역할 |
| Leader/Worker protocol | team orchestrator/executor prompt | Sisyphus delegation prompt, background agent rules | **가져옴**. 규칙은 prompt, 실체는 runtime |
| Pipeline/Workflow | team pipeline, ralplan/ralph/ultraqa 등 | ultrawork, Ralph loop, review-work 등 | **재해석**. workflow는 hardcoded chain보다 reusable runbook/loop로 |
| Background agents | tmux/team workers | background-agent manager, task ids, result collection | **가져옴**. OmG agent runtime의 v1 핵심 |
| Context management | wiki/state/notepad/MCP surfaces | context window monitor, compaction, background exploration | **핵심으로 가져옴**. OmG 고유 Context Manager로 승격 |
| Safety/Guard | guard/state/workflow stop gates | tool permission restrictions by agent | **가져옴**. 중앙 policy + agent tool policy 결합 |
| Session recovery | state lifecycle, trace/notepad | session recovery, missing tool result recovery | **가져옴**. OmG main host 지속성에 필수 |

---

## 2. Skill

### OmX 쪽 관찰

OmX/현재 OmG 계획에는 skill을 다음처럼 보는 흐름이 있다.

- skill loader는 `skill.yaml`과 prompt text를 읽는다.
- skill은 MCP metadata를 가질 수 있다.
- skill load만으로 MCP server를 시작하지 않는다.
- skill은 capability이며 agent/model selector가 아니다.

근거:

- `.omx/plans/oh-my-gunnsama-p2-interface-closure.md` — “Skill Schema and MCP Lifecycle”
- `internal/skills/skills.go`
- `internal/skills/skills_test.go`

### OmO 쪽 관찰

OmO는 skill을 더 제품적으로 설명한다.

- skill은 specialized knowledge(context)와 tools(MCP)를 agent에 주입한다.
- skill-embedded MCP로 task-scoped tool을 붙인다.
- browser, git, review, frontend 등 domain workflow를 skill로 포장한다.

근거:

- `reference/oh-my-openagent/README.md` — Skill-Embedded MCPs
- `reference/oh-my-openagent/docs/reference/features.md` — Skills section

### OmG로 가져올 형태

OmG에서 skill은 다음처럼 두는 것이 좋다.

```text
Skill = 특정 상황에서 agent에게 붙일 수 있는 전문 지식 + 도구 능력 + 절차 힌트
```

권장 설계:

```text
skill
├─ name
├─ description
├─ trigger hints
├─ prompt/context packet
├─ optional MCP tools
├─ optional verification hints
└─ non-goals / guard notes
```

가져올 것:

- skill metadata + prompt 분리
- skill-embedded MCP
- lazy MCP start
- skill은 agent/model 강제 금지
- skill은 context/tool capability로 유지

가져오지 않을 것:

- skill이 특정 agent를 강제 선택하는 구조
- skill load 시 외부 프로세스를 바로 띄우는 구조
- skill이 workflow 전체를 독점하는 구조

---

## 3. MCP

### OmX 쪽 관찰

OmX team runtime은 과거 `team_*` MCP API를 사용했지만, 현재는 CLI-first interop으로 정리되어 있다.

- authoritative mutation path는 `omx team api ... --json`.
- legacy `team_*` MCP API는 deprecated.
- team state 직접 수정은 금지.

근거:

- `reference/oh-my-codex/docs/interop-team-mutation-contract.md`
- `reference/oh-my-codex/docs/release-notes-0.8.1.md`

현재 OmG 쪽 MCP 계획은 skill이 필요로 하는 capability를 lazy start하는 manager에 가깝다.

근거:

- `internal/mcp/manager.go`
- `.omx/plans/oh-my-gunnsama-p2-interface-closure.md`

### OmO 쪽 관찰

OmO는 MCP를 capability layer로 강하게 사용한다.

- built-in MCP: web search, official docs, GitHub/code search 등
- skill-embedded MCP: skill이 자기 도구 서버를 데려옴
- MCP는 context bloat를 줄이고 필요할 때만 tool capability를 공급하는 역할

근거:

- `reference/oh-my-openagent/README.md`
- `reference/oh-my-openagent/docs/reference/features.md`
- `reference/oh-my-openagent/AGENTS.md`

### OmG로 가져올 형태

OmG에서 MCP는 다음 위치가 맞다.

```text
Agent ── MCP ── Tool / External Capability
```

아래처럼 쓰지 않는다.

```text
Agent ── MCP ── Agent
```

권장 설계:

```text
MCP Manager
├─ built-in capabilities
├─ skill-embedded capabilities
├─ lazy start
├─ idle cleanup
├─ env allowlist
├─ credential redaction
└─ per-session / per-task scope
```

가져올 것:

- lazy lifecycle
- skill-embedded MCP
- built-in MCP registry
- env allowlist
- idle cleanup / zombie cleanup
- capability-scoped tool exposure

가져오지 않을 것:

- agent 간 통신을 MCP로 구현하는 방식
- team state mutation을 MCP API로 여는 방식
- MCP server를 항상 켜두는 방식

---

## 4. Agent

### OmX 쪽 관찰

OmX는 prompt catalog와 role routing이 강하다.

- team-orchestrator는 conservative staffing, bounded delegation, evidence-backed completion을 강조한다.
- team-executor는 assigned scope, verification, blocker reporting을 강조한다.
- AGENTS.md는 leader/worker 책임, specialist routing, delegation boundaries를 명확히 둔다.

근거:

- `reference/oh-my-codex/prompts/team-orchestrator.md`
- `reference/oh-my-codex/prompts/team-executor.md`
- 현재 세션 AGENTS.md / `.omx/state/sessions/.../AGENTS.md`

### OmO 쪽 관찰

OmO는 named agent persona와 delegation behavior가 강하다.

- Sisyphus: default orchestrator, planning/delegation/parallel execution
- Explore/Librarian: internal/external search specialist
- Oracle/Momus 등 read-only/review 계열
- agent별 tool restrictions가 있음

근거:

- `reference/oh-my-openagent/docs/reference/features.md`
- `reference/oh-my-openagent/src/agents/sisyphus.ts`
- `reference/oh-my-openagent/src/agents/*`

### OmG로 가져올 형태

OmG는 fixed agent set을 복제하기보다 **agent catalog**로 재구성하는 것이 맞다.

권장 agent categories:

```text
Leader        = 전체 목표/분해/통합/보고
Reader        = repo/docs/log 읽기와 근거 수집
Planner       = 실행 계획 후보 작성
Executor      = 실제 변경 수행
Verifier      = 테스트/검증/완료 증거
Critic        = 허점/리스크/반론
Summarizer    = 장기 context 압축
Researcher    = 외부 문서/공식 자료
Toolsmith     = 도구/MCP/환경 문제 해결
```

Agent definition에 들어갈 것:

```text
agent
├─ identity
├─ responsibilities
├─ allowed tools
├─ forbidden actions
├─ escalation rules
├─ expected output format
├─ useWhen / avoidWhen
├─ context requirements
└─ verification requirements
```

가져올 것:

- strong role prompt
- useWhen/avoidWhen
- tool policy per agent
- delegation prompt structure
- read-only agent restrictions
- leader/worker separation

가져오지 않을 것:

- OmO agent 이름/세계관을 그대로 고정
- 모든 작업에 무조건 다수 agent fanout
- worker가 leader처럼 전역 재계획하는 구조

---

## 5. 작업 큐 / Task System

### OmX 쪽 관찰

OmX team task는 claim/lease/transition을 갖는다.

- pending / in_progress / completed / failed
- task claim conflict 방지
- dependency readiness
- terminal transition은 claim token 기반

근거:

- `reference/oh-my-codex/src/team/state/tasks.ts`
- `reference/oh-my-codex/docs/interop-team-mutation-contract.md`

### OmO 쪽 관찰

OmO task system은 persistent task files와 dependencies를 갖는다.

- status: pending / in_progress / completed / deleted
- `blockedBy`, `blocks`
- tasks stored as JSON files
- multiple subagents collaborate or progress persists across sessions

근거:

- `reference/oh-my-openagent/docs/reference/features.md` — Task System Details

### OmG로 가져올 형태

OmG Main Host에는 작업 큐가 핵심이다.

권장 task model:

```text
task
├─ id
├─ title
├─ description
├─ status: pending | ready | in_progress | blocked | completed | failed | cancelled
├─ owner
├─ claim token / lease
├─ priority
├─ dependencies
├─ expected output
├─ required evidence
├─ context packet id
├─ result summary
└─ verification status
```

가져올 것:

- task claim/lease
- dependency graph
- terminal transition rules
- persistent JSON/state backing
- owner/worker assignment
- required evidence field

가져오지 않을 것:

- session memory only task list
- prompt-only todo tracking
- claim 없이 여러 worker가 같은 task를 잡는 구조

---

## 6. Mailbox

### OmX 쪽 관찰

OmX team에는 mailbox API가 있다.

- send-message
- broadcast
- mailbox-list
- mark-notified
- mark-delivered

근거:

- `reference/oh-my-codex/docs/interop-team-mutation-contract.md`
- `reference/oh-my-codex/src/team/state/mailbox.ts`

### OmO 쪽 관찰

OmO는 mailbox라는 팀 메시지 체계보다 background task result/session notification 쪽에 가깝다.

근거:

- `reference/oh-my-openagent/src/features/background-agent/*`
- `reference/oh-my-openagent/docs/reference/features.md` — background_output/background_cancel

### OmG로 가져올 형태

OmG는 mailbox를 명시적으로 가져오는 것이 좋다.

권장 message model:

```text
message
├─ id
├─ from
├─ to
├─ kind: report | question | blocker | instruction | review | alert
├─ related task
├─ body
├─ evidence refs
├─ delivery status
├─ read status
└─ timestamp
```

가져올 것:

- leader-to-worker instruction
- worker-to-leader report
- blocker/question escalation
- broadcast
- delivery/read markers

가져오지 않을 것:

- agent 간 자유 채팅을 무제한 허용
- mailbox를 task queue 대체물로 사용하는 구조

---

## 7. Event Log

### OmX 쪽 관찰

OmX team events는 wakeable/non-wakeable distinction이 있다.

- terminal task events
- worker state changes
- leader notifications
- worker merge conflict
- audit-only diff/report events

근거:

- `reference/oh-my-codex/docs/interop-team-mutation-contract.md`
- `reference/oh-my-codex/src/team/state/events.ts`

### OmO 쪽 관찰

OmO는 background task logs, task history, poller, stale detection, session status 쪽이 강하다.

근거:

- `reference/oh-my-openagent/src/features/background-agent/task-history.ts`
- `reference/oh-my-openagent/src/features/background-agent/task-poller.ts`
- `reference/oh-my-openagent/src/features/background-agent/session-status-classifier.ts`

### OmG로 가져올 형태

OmG event log는 append-only로 둔다.

권장 event categories:

```text
event
├─ task_created
├─ task_claimed
├─ task_completed
├─ task_failed
├─ message_sent
├─ worker_started
├─ worker_idle
├─ worker_blocked
├─ guard_blocked
├─ verification_passed
├─ verification_failed
├─ context_adopted
├─ context_stale
└─ runtime_error
```

가져올 것:

- append-only history
- wakeable event distinction
- audit event distinction
- replay/debuggability
- observer/reportserver integration 가능성

가져오지 않을 것:

- 현재 상태만 저장하고 history를 잃는 구조
- 로그 파싱이 유일한 state source가 되는 구조

---

## 8. Shared State

### OmX 쪽 관찰

OmX team state는 task/worker/mailbox/monitor/shutdown 등을 분리한다.

근거:

- `reference/oh-my-codex/src/team/state/*`
- `reference/oh-my-codex/src/team/runtime-cli.ts`

### OmO 쪽 관찰

OmO background-agent는 task status, session id, progress, concurrency group, stale timeout 등을 관리한다.

근거:

- `reference/oh-my-openagent/src/features/background-agent/types.ts`
- `reference/oh-my-openagent/src/features/background-agent/manager.ts`
- `reference/oh-my-openagent/src/features/background-agent/concurrency.ts`

### OmG로 가져올 형태

Shared state는 “현재 상황판”이다.

권장 sections:

```text
shared state
├─ current goal
├─ current phase
├─ active tasks
├─ active workers
├─ task ownership/locks
├─ mailbox unread counts
├─ known blockers
├─ verification status
├─ context packet currently in use
├─ adopted/stale context summary
└─ shutdown/recovery flags
```

가져올 것:

- monitor snapshot
- worker heartbeat/status
- task ownership
- locks/claims
- session recovery flags

가져오지 않을 것:

- event log와 shared state를 같은 파일/개념으로 뭉개기
- agent memory 안에만 현재 상태를 두기

---

## 9. Leader/Worker Protocol

### OmX 쪽 관찰

OmX는 leader/worker 책임을 프롬프트와 runtime 모두에 둔다.

프롬프트에 들어가는 것:

- leader는 mode/plan/delegation/integration/verification 책임
- worker는 assigned slice만 수행
- blocker/scope expansion/conflict는 upward report
- completion은 evidence-backed

근거:

- `reference/oh-my-codex/prompts/team-orchestrator.md`
- `reference/oh-my-codex/prompts/team-executor.md`
- AGENTS.md team/worker protocol

### OmO 쪽 관찰

OmO는 orchestrator prompt에 delegation behavior를 강하게 넣는다.

- parallel agents는 background로 띄움
- 결과는 system reminder 후 collection
- delegation prompt는 TASK / EXPECTED OUTCOME / REQUIRED TOOLS / MUST DO / MUST NOT DO / CONTEXT를 포함
- delegated work는 반드시 verify

근거:

- `reference/oh-my-openagent/src/agents/sisyphus.ts`
- `reference/oh-my-openagent/docs/reference/features.md`

### OmG로 가져올 형태

OmG는 prompt와 runtime 양쪽이 필요하다.

프롬프트에 넣을 것:

```text
- Leader만 최종 목표/완료 판단
- Worker는 맡은 task scope만 수행
- Worker는 evidence와 함께 보고
- Worker는 scope 확대 전 leader에게 요청
- Worker는 실패/막힘을 숨기지 않음
```

Runtime으로 강제할 것:

```text
- task claim
- owner
- allowed tools
- file locks / write scope
- completion transition
- verification requirement
- event logging
```

가져올 것:

- strict delegation prompt format
- anti-duplication rules
- background result collection discipline
- worker scope guard
- leader final integration

가져오지 않을 것:

- 프롬프트만 믿고 runtime lock/claim 없이 운영
- worker가 재귀적으로 orchestration하는 구조

---

## 10. Pipeline / Workflow

### OmX 쪽 후보

- team pipeline: team-plan → team-prd → team-exec → team-verify → team-fix
- ralplan / ralph / autopilot / ultraqa / ultrawork
- cancel / trace / note / wiki utilities

근거:

- AGENTS.md workflow sections
- `reference/oh-my-codex/prompts/team-orchestrator.md`
- `.omx/plans/*`

### OmO 쪽 후보

- ultrawork: one-word high-intensity workflow
- Ralph loop / self-referential completion loop
- review-work: parallel review agents
- Prometheus Planner / interview planning
- session recovery

근거:

- `reference/oh-my-openagent/README.md`
- `reference/oh-my-openagent/docs/reference/features.md`

### OmG로 가져올 형태

OmG에서는 workflow를 hardcoded chain으로 만들기보다 **runbook + state machine**으로 두는 것이 좋다.

권장 workflow classes:

```text
Workflow
├─ Plan-first workflow
├─ Direct execute workflow
├─ Investigate/debug workflow
├─ Refactor workflow
├─ Review/QA workflow
├─ Long-running autonomous loop
├─ Visual/UI loop
└─ Research workflow
```

각 workflow의 공통 계약:

```text
workflow
├─ trigger/entry condition
├─ required context
├─ allowed agents
├─ phase state machine
├─ stop condition
├─ verification gate
├─ memory update policy
└─ cancellation/recovery policy
```

가져올 것:

- team pipeline의 phase discipline
- ultrawork의 “keep going until done” UX
- review-work의 parallel review model
- planning/interview gate
- cancellation/recovery

가져오지 않을 것:

- 모든 workflow를 global keyword로만 트리거
- workflow가 host/agent 판단을 완전히 우회
- fix loop에 max retry/stop condition이 없는 구조

---

## 11. Background Agents / Parallelism

### OmX 쪽 후보

- team workers / tmux panes
- task claim + mailbox + state
- conservative staffing

근거:

- `reference/oh-my-codex/src/team/*`
- `reference/oh-my-codex/prompts/team-orchestrator.md`

### OmO 쪽 후보

- background-agent manager
- task ids
- fire-and-forget launch
- parent/child session relation
- progress polling
- stale timeout
- concurrency limit
- background_output collection

근거:

- `reference/oh-my-openagent/src/features/background-agent/*`
- `reference/oh-my-openagent/docs/reference/features.md`
- `reference/oh-my-openagent/src/agents/sisyphus.ts`

### OmG로 가져올 형태

OmG main host에는 background agents가 핵심이다.

권장 model:

```text
background agent
├─ id
├─ role
├─ task id
├─ parent run id
├─ session/process id
├─ status
├─ progress
├─ output packet
├─ evidence refs
├─ timeout/stale policy
└─ cancellation handle
```

가져올 것:

- parallel launch
- non-blocking result collection
- concurrency limits
- stale task detection
- parent-child session mapping
- worker progress heartbeat

가져오지 않을 것:

- 무조건 최대 fanout
- 결과가 오기 전에 blocking wait
- disposable background task cleanup 누락

---

## 12. Context Management

### OmX 쪽 후보

- notepad / wiki / trace / state MCP surfaces
- prompt sections and token budget
- team inspection metadata
- sparkshell summaries

근거:

- `.omx/notepad.md`
- `.omx/plans/oh-my-gunnsama-p1-interface-closure.md`
- reference/oh-my-codex docs and runtime surfaces

### OmO 쪽 후보

- context window monitor
- intelligent compaction
- background exploration to keep context lean
- session read/search
- hash-anchored edit tool

근거:

- `reference/oh-my-openagent/README.md`
- `reference/oh-my-openagent/docs/reference/features.md`
- `reference/oh-my-openagent/src/features/background-agent/*`

### OmG로 가져올 형태

OmG의 핵심은 context management이므로 별도 중심 모듈로 둔다.

권장 responsibilities:

```text
Context Manager
├─ task-specific context packet build
├─ repo/test/log evidence collection
├─ claim candidate/adoption lifecycle
├─ stale/stale-risk marking
├─ prompt budget pruning
├─ session compaction
├─ run summary
└─ memory promotion/demotion
```

가져올 것:

- context window awareness
- background reader/summarizer agents
- cited context packet
- hash/stale detection
- session search/read
- compaction/recovery

가져오지 않을 것:

- 근거 없는 대형 요약
- 모든 session history를 계속 prompt에 밀어넣기
- memory와 prompt를 구분하지 않는 구조

---

## 13. Guard / Safety / Permissions

### OmX 쪽 후보

- tool-before guard
- suppression/audit
- state transition on error/stop
- workflow stop gates

근거:

- `internal/guard/guard.go`
- `.omx/plans/oh-my-gunnsama-p2-interface-closure.md`

### OmO 쪽 후보

- agent별 blocked tools
- read-only agents cannot write/edit/delegate
- permissions passed to child sessions
- sensitive error redaction in MCP manager

근거:

- `reference/oh-my-openagent/docs/reference/features.md`
- `reference/oh-my-openagent/src/features/skill-mcp-manager/error-redaction.ts`
- `reference/oh-my-openagent/src/features/background-agent/spawner.ts`

### OmG로 가져올 형태

권장 layers:

```text
Safety
├─ global policy
├─ agent tool policy
├─ task-scoped permission
├─ file/write scope
├─ destructive action guard
├─ credential/production guard
├─ MCP env allowlist
└─ audit trail
```

가져올 것:

- per-agent tool restrictions
- read-only agent hard guard
- task/session permission propagation
- redaction
- suppression with audit

가져오지 않을 것:

- prompt-only safety
- child agent에게 parent보다 넓은 권한 주기

---

## 14. Config / Registry

### OmX 쪽 후보

- AGENTS.md as top-level operating contract
- generated model/role tables
- workflow keyword routing
- local skills/agents/prompts surfaces

근거:

- AGENTS.md generated sections
- reference/oh-my-codex prompts/skills layout

### OmO 쪽 후보

- JSONC config
- agent/category/tool/MCP/command overrides
- prompt external files
- migration helpers

근거:

- `reference/oh-my-openagent/AGENTS.md`
- `reference/oh-my-openagent/docs/reference/features.md`
- `reference/oh-my-openagent/src/plugin-config.ts`

### OmG로 가져올 형태

OmG는 config를 제품 UX로 봐야 한다.

권장 config domains:

```text
config
├─ agents
├─ roles/categories
├─ models/providers
├─ tools
├─ skills
├─ MCP servers
├─ workflows
├─ guards
├─ memory/context policy
├─ verification policy
└─ host/backend adapters
```

가져올 것:

- JSONC-like user ergonomics 또는 Rust 동등 구현
- layered config: built-in → user → project
- migration
- schema validation
- unknown field warnings

가져오지 않을 것:

- config가 곧 runtime state가 되는 구조
- agent/skill/model 결정 경계가 뒤섞이는 구조

---

## 15. 우선순위 제안

### P0 — OmG main host runtime foundation

1. 작업 큐
2. event log
3. shared state
4. leader/worker protocol
5. basic agent catalog
6. basic guard

이유: 멀티 에이전트가 실제로 굴러가려면 task/state/event/protocol이 먼저 필요하다.

### P1 — Context management foundation

1. context packet
2. evidence refs
3. candidate/adopted/stale lifecycle
4. background reader/summarizer
5. prompt/context budget pruning

이유: OmG의 차별점은 고도화된 context 관리다.

### P2 — Skill/MCP capability system

1. skill loader
2. skill prompt/context injection
3. skill-embedded MCP
4. lazy MCP manager
5. built-in MCP registry

이유: 도구 능력을 확장해야 하지만, agent 간 통신 핵심은 아니다.

### P3 — Workflows / productized modes

1. plan-first
2. execute
3. debug
4. review
5. autonomous loop
6. visual loop

이유: runtime과 context가 먼저 안정되어야 workflow가 의미 있다.

---

## 16. 가져올 때의 원칙

### 원칙 1 — Runtime 실체와 Prompt 규칙을 분리한다

```text
Prompt: agent가 어떻게 행동해야 하는지
Runtime: task/message/state/log/lock을 실제로 관리
```

### 원칙 2 — MCP는 tool capability다

```text
MCP = agent-to-tool
Mailbox/Event/Task = agent-to-agent runtime
```

### 원칙 3 — Agent 정의는 강하지만, agent set은 고정하지 않는다

OmO/OmX의 agent persona는 참고하되 OmG의 catalog로 재구성한다.

### 원칙 4 — Workflow는 loop/state machine으로 둔다

문자열 keyword나 hardcoded chain이 아니라, phase/state/stop condition/verification gate가 있어야 한다.

### 원칙 5 — Context management를 중심 기능으로 승격한다

skill, mcp, agent, workflow는 모두 context management와 task execution을 돕는 주변 부품이다.

---

## 17. 최종 결론

OmG에 가장 우선적으로 가져올 것은 “기능 이름”이 아니라 아래 구조다.

```text
OmG Main Host
├─ task queue
├─ event log
├─ shared state
├─ mailbox
├─ leader/worker protocol
├─ agent catalog
├─ context manager
├─ skill/capability system
├─ MCP tool bridge
├─ guard/safety policy
└─ workflow/runbook engine
```

OmX에서 특히 가져올 가치가 큰 것:

- team API/state/mailbox/event/runtime contract
- leader/worker protocol
- CLI-first interop discipline
- skill/MCP lazy lifecycle 계획
- guard/state lifecycle

OmO에서 특히 가져올 가치가 큰 것:

- strong agent personas
- background agent manager
- parallel delegation UX
- skill-embedded MCP product idea
- task system with dependency/persistence
- context window/recovery/compaction ideas
- tool restrictions by agent

가져오면 안 되는 것:

- MCP를 agent 간 통신망으로 쓰는 구조
- prompt-only collaboration
- 무조건 fanout하는 ultrawork 복제
- 고정 agent set 복제
- host plugin 결합을 OmG core로 가져오는 것

한 줄 요약:

> OmX에서는 팀 런타임의 질서와 상태 기계를, OmO에서는 강한 에이전트 UX와 background parallelism을 가져오되, OmG에서는 둘 다 “메인 AI 개발 호스트를 위한 context-first multi-agent runtime”으로 재구성한다.

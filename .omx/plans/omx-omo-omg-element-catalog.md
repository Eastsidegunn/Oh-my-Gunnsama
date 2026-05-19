# OmX / OmO / OmG 요소 도감

작성일: 2026-05-15

## 목적

이 문서는 설계 판단 전에, 각 프로젝트에 어떤 요소가 있는지 한눈에 보기 위한 **도감형 목록**이다.

- OmX = oh-my-codex 계열
- OmO = oh-my-openagent / oh-my-opencode 계열
- OmG = oh-my-gunnsama 현재/목표 계열

이 문서는 “가져온다/안 가져온다”를 최종 결정하는 문서가 아니다.  
먼저 **무슨 부품들이 존재하는지** 보기 위한 재료 목록이다.

---

# 1. OmX 요소 도감

## 1.1 핵심 정체성

OmX는 Codex CLI 위에서 동작하는 **workflow / team / skill / hook / state orchestration layer**에 가깝다.

주요 인상:

- Codex 세션에 붙는 운영 레이어
- workflow keyword 기반 실행
- team/tmux 기반 병렬 작업
- state/notepad/wiki/trace 같은 session persistence
- skill과 prompt surface가 많음
- runtime mode가 다양함

---

## 1.2 Workflow / Mode 계열

| 요소 | 설명 | 성격 |
|---|---|---|
| `autopilot` | `ralplan -> ralph -> code-review` 식 자동 루프 | 상위 자동화 workflow |
| `ralph` | 자기 참조 loop로 task completion까지 밀고 가는 workflow | persistent single-owner loop |
| `ralplan` | consensus planning / 계획 수립 | planning workflow |
| `team` | 여러 worker를 tmux/team state로 운용 | multi-agent/team workflow |
| `swarm` | team과 유사한 병렬 작업 계열 | high fanout workflow |
| `ultrawork` | 높은 처리량의 병렬 execution engine | throughput workflow |
| `ultraqa` | test / verify / fix 반복 | QA loop |
| `pipeline` | configurable stage sequencing | generic workflow orchestrator |
| `visual-ralph` | UI 구현을 visual-verdict와 반복 | visual implementation loop |
| `visual-verdict` | screenshot/reference 비교 verdict | visual QA tool/workflow |
| `deep-interview` | 요구사항 명확화 인터뷰 | clarification workflow |
| `plan` | 전략적 planning | planning workflow |
| `cancel` | active runtime workflow 중단 | lifecycle utility |
| `trace` | agent flow trace 조회 | observability utility |
| `note` | notepad에 기록 | memory utility |
| `wiki` | project wiki 관리 | knowledge utility |

---

## 1.3 Team Runtime 계열

| 요소 | 설명 | OmG 관점 |
|---|---|---|
| Team Leader | team run을 계획/분배/통합하는 주체 | Leader agent 참고 가능 |
| Worker | 할당된 task slice를 수행하는 agent | Worker protocol 참고 가능 |
| Team task | worker가 claim하고 수행하는 작업 단위 | OmG task queue 후보 |
| Task claim | worker가 task를 잡는 lease/lock | 필수 차용 후보 |
| Task transition | `pending -> in_progress -> completed/failed` | state machine 참고 |
| Mailbox | leader/worker message 전달 | OmG mailbox 후보 |
| Broadcast | 여러 worker에게 공지 | OmG broadcast 후보 |
| Worker heartbeat | worker alive/progress 확인 | shared state 후보 |
| Monitor snapshot | team 상태 요약 | shared state 후보 |
| Event append/read | team event 기록/조회 | event log 후보 |
| Wakeable event | leader를 깨워야 하는 event | event priority 참고 |
| Audit-only event | 기록은 하지만 깨우지는 않는 event | observability 참고 |
| Shutdown request/ack | team 종료 조정 | lifecycle 참고 |
| Task approval | task 수준 승인/보류 | guard/approval 참고 |

---

## 1.4 Team API / Interop 계열

| 요소 | 설명 |
|---|---|
| `omx team api read-task` | 현재 task 읽기 |
| `omx team api claim-task` | task claim |
| `omx team api transition-task-status` | task 완료/실패 전이 |
| `omx team api release-task-claim` | claim 해제/rollback |
| `omx team api send-message` | mailbox message 전송 |
| `omx team api broadcast` | broadcast message 전송 |
| `omx team api mailbox-list` | mailbox 조회 |
| `omx team api append-event` | event 기록 |
| `omx team api get-summary` | team summary 조회 |
| `omx team api cleanup` | team runtime cleanup |

중요한 점:

- OmX는 team mutation에서 MCP보다 CLI-first interop을 권위 경로로 둔다.
- legacy `team_*` MCP는 deprecated로 취급된다.

---

## 1.5 Skill 계열

| 요소 | 설명 | OmG 관점 |
|---|---|---|
| Skill metadata | name, description, trigger 등 | skill registry 참고 |
| Skill prompt | skill 사용 시 주입되는 전문 지식 | context packet 후보 |
| Skill scripts | skill 내부 helper script | optional capability |
| Skill MCP metadata | skill이 필요로 하는 MCP tool 선언 | skill-embedded capability |
| Skill activation | `$skill` 또는 keyword/hook 기반 | routing UX 참고 |
| Skill bodies | `SKILL.md` 중심 instructions | prompt surface 참고 |

OmX skill 예시:

| Skill | 설명 |
|---|---|
| `code-review` | code review workflow |
| `security-review` | security review workflow |
| `ai-slop-cleaner` | cleanup/refactor/deslop |
| `hatch-pet` | pet spritesheet 생성 |
| `slides-grab` | presentation workflow |
| `configure-notifications` | notification setup |
| `doctor` | install 진단 |
| `help` | OMX 사용 안내 |
| `hud` | statusline/HUD |

---

## 1.6 MCP 계열

| 요소 | 설명 | OmG 관점 |
|---|---|---|
| MCP manager | MCP process/client lifecycle 관리 | 가져올 후보 |
| Lazy start | 필요할 때만 MCP 시작 | 가져올 후보 |
| Idle timeout | 오래 안 쓰면 종료 | 가져올 후보 |
| Zombie cleanup | 죽은 process 정리 | 가져올 후보 |
| Env allowlist | 명시 env만 전달 | 가져올 후보 |
| Skill MCP | skill이 MCP capability 선언 | 가져올 후보 |
| Wiki MCP | wiki 조작 tool surface | 참고 |
| State MCP | workflow state 조작 | 참고 |
| Memory MCP | notepad/project memory | 참고 |
| Code-intel MCP | AST/LSP/search 진단 | 참고 |
| Trace MCP | flow trace 조회 | 참고 |

주의:

- OmX에서 MCP는 agent 간 주 통신망이 아니라 도구 capability에 가깝다.

---

## 1.7 Prompt / Agent Role 계열

| 요소 | 설명 |
|---|---|
| Role prompts | `explore`, `executor`, `architect`, `verifier` 등 역할 프롬프트 |
| Team prompts | team-orchestrator, team-executor |
| AGENTS.md | workspace-level operating contract |
| Runtime overlay | session/team 상태에 따라 AGENTS.md에 runtime context 삽입 |
| Keyword routing | user prompt keyword로 workflow/skill 활성화 |
| Specialist routing | explore/researcher/dependency-expert 등 역할 분기 |
| Model routing | role별 model/effort 표 |

대표 role:

| Role | 설명 |
|---|---|
| `explore` | repo search/mapping |
| `executor` | implementation/refactor |
| `architect` | design/architecture |
| `debugger` | root cause analysis |
| `verifier` | completion evidence |
| `code-reviewer` | comprehensive review |
| `security-reviewer` | security review |
| `test-engineer` | test strategy |
| `writer` | docs/migration notes |
| `designer` | UX/UI design |

---

## 1.8 State / Memory / Knowledge 계열

| 요소 | 설명 | OmG 관점 |
|---|---|---|
| `.omx/state` | mode/workflow state | state store 참고 |
| `.omx/notepad.md` | session notes | lightweight memory 참고 |
| `.omx/project-memory.json` | project memory | persistent memory 참고 |
| `.omx/wiki` | project wiki | knowledge base 참고 |
| `.omx/plans` | PRD/test-spec/plans | planning artifact 참고 |
| `.omx/logs` | runtime logs | observability 참고 |
| Trace timeline | agent flow trace | event log/reporting 참고 |

---

## 1.9 Guard / Safety 계열

| 요소 | 설명 |
|---|---|
| Tool-before guard | tool 실행 전 위험 판단 |
| Guard rules | destructive/credential/production 영향 차단 |
| Suppression | 일부 guard suppress with audit |
| Stop hook gating | workflow state에 따라 stop 차단 |
| Approval gate | user confirmation 필요한 작업 제한 |
| Lore commit protocol | commit message decision record |

---

## 1.10 OmX에서 특히 눈여겨볼 것

1. team task / claim / transition
2. mailbox / event / monitor snapshot
3. leader/worker protocol
4. CLI-first interop
5. skill + MCP lifecycle 분리
6. workflow state persistence
7. AGENTS.md as operating contract
8. trace/notepad/wiki surfaces

---

# 2. OmO 요소 도감

## 2.1 핵심 정체성

OmO는 OpenCode plugin으로 동작하는 **agent-heavy coding environment**에 가깝다.

주요 인상:

- 강한 agent persona
- background agents / parallel delegation
- skill-embedded MCP
- built-in tools
- OpenCode hook/plugin system
- session recovery / context window handling
- ultrawork 같은 고강도 UX

---

## 2.2 Agent 계열

| 요소 | 설명 | OmG 관점 |
|---|---|---|
| Sisyphus | default orchestrator, planning/delegation/parallel execution | Leader archetype 참고 |
| Hephaestus | autonomous deep worker | Executor/Deep worker 참고 |
| Oracle | read-only analysis/review 계열 | Reader/Critic 참고 |
| Librarian | external docs/reference specialist | Researcher 참고 |
| Explore | repo/internal search specialist | Reader 참고 |
| Atlas | planning/coordination 계열 | Planner 참고 |
| Momus | critique/review 성격 | Critic 참고 |
| Prometheus | planning/interview 계열 | Plan/interview workflow 참고 |

OmO agent 특징:

- agent별 tool restriction이 강함
- read-only agent는 write/edit/delegate 차단
- orchestrator는 parallel delegation을 적극 사용
- agent prompt 안에 delegation behavior가 길게 들어감

---

## 2.3 Background Agent 계열

| 요소 | 설명 | OmG 관점 |
|---|---|---|
| Background task | agent를 별도 session/task로 실행 | 핵심 차용 후보 |
| Task ID | background task 식별 | task runtime 후보 |
| Parent session | parent-child session 관계 | run tree 후보 |
| Result collection | 완료 후 결과 수집 | mailbox/result packet 후보 |
| Poller | task 진행/완료 감시 | monitor 후보 |
| Stale timeout | 오래 진행 없는 task 감지 | stale/heartbeat 후보 |
| Concurrency manager | 모델/agent별 동시성 제한 | scheduler 후보 |
| Tmux callback | background session pane 연결 | UI/runtime option |
| Toast manager | task 상태 알림 | UX 참고 |
| Cleanup | task completion/cancel cleanup | lifecycle 참고 |

---

## 2.4 Task System 계열

| 요소 | 설명 | OmG 관점 |
|---|---|---|
| TaskCreate | persistent task 생성 | 작업 큐 후보 |
| TaskUpdate | task 상태 갱신 | state transition 후보 |
| TaskList | task 목록 조회 | queue view 후보 |
| TaskGet | task 단건 조회 | queue API 후보 |
| `blockedBy` | 선행 task dependency | DAG 후보 |
| `blocks` | 후행 task relation | DAG 후보 |
| file storage | `.sisyphus/tasks` JSON | persistent queue 참고 |

상태:

```text
pending | in_progress | completed | deleted
```

OmG에서는 `failed`, `blocked`, `cancelled` 등을 추가하는 것이 적합할 수 있다.

---

## 2.5 Delegation / Parallelism 계열

| 요소 | 설명 |
|---|---|
| `task(...)` | category/subagent 기반 delegation tool |
| `call_omo_agent` | explore/librarian 계열 agent spawn |
| `run_in_background` | background 실행 |
| `background_output` | task result 조회 |
| `background_cancel` | background task 취소 |
| Parallel exploration | 여러 reader/researcher를 동시에 실행 |
| Anti-duplication rules | delegated work와 leader work 중복 방지 |
| Prompt structure | TASK / EXPECTED OUTCOME / REQUIRED TOOLS / MUST DO / MUST NOT DO / CONTEXT |

중요한 UX:

- background로 띄운 뒤 leader는 non-overlapping work를 계속한다.
- 결과가 준비되기 전 blocking wait를 피한다.
- 결과 수집 시 system reminder 이후에 가져오라는 규칙이 있다.

---

## 2.6 Skill 계열

| Skill | 설명 | OmG 관점 |
|---|---|---|
| git-master | commit/rebase/history expert | skill archetype |
| playwright | browser testing/verification | MCP/browser skill |
| agent-browser | browser automation | browser skill 후보 |
| dev-browser | persistent browser scripting | stateful skill 후보 |
| frontend-ui-ux | UI/UX design/dev persona | designer skill 후보 |
| review-work | 5 parallel review subagents | review workflow 후보 |
| ai-slop-remover | AI smell cleanup | cleanup skill 후보 |

OmO skill 특징:

- skill은 context + tools + workflow를 같이 제공한다.
- skill-embedded MCP가 핵심 UX다.
- skill이 agent의 system prompt에 내용을 주입한다.

---

## 2.7 MCP 계열

| 요소 | 설명 | OmG 관점 |
|---|---|---|
| Built-in MCPs | websearch, context7, grep_app | built-in capability 후보 |
| Skill-embedded MCP | skill별 MCP server | 강한 차용 후보 |
| MCP OAuth | remote MCP auth 관리 | 나중 후보 |
| MCP env allowlist | env 안전 전달 | 가져올 후보 |
| Skill MCP manager | session/task scoped MCP 관리 | 가져올 후보 |

주의:

- OmO에서도 MCP는 agent 간 통신보다는 도구/자료 접근에 가깝다.

---

## 2.8 Hook / Plugin 계열

| 요소 | 설명 | OmG 관점 |
|---|---|---|
| config hook | plugin config 구성 | host adapter 참고 |
| tool hook | tool 등록/수정 | tool registry 참고 |
| chat.message hook | message 처리 | input pipeline 참고 |
| chat.params hook | model/params 조정 | model router 참고 |
| tool.execute.before | tool 실행 전 처리 | guard 참고 |
| tool.execute.after | tool 실행 후 처리 | observation/logging 참고 |
| messages transform | message/context 변환 | context delivery 참고 |
| session compacting | context window 관리 | compaction 참고 |

OmG가 main host가 되면 plugin hook은 중심이 아니라 adapter 참고가 된다.

---

## 2.9 Tool 계열

| 요소 | 설명 | OmG 관점 |
|---|---|---|
| Hash-anchored edit | LINE#ID/hash 기반 stale-line 방지 | 강한 후보 |
| LSP tools | symbols/diagnostics | code-intel 후보 |
| AST-grep tools | structural search/rewrite | code-intel 후보 |
| session_read/search | session history 조회 | memory/search 후보 |
| interactive_bash | tmux terminal | runtime tool 후보 |
| look_at | multimodal file/image/PDF analysis | multimodal tool 후보 |
| task tools | persistent task mgmt | task queue 후보 |
| skill tools | skill execution | skill runtime 후보 |

---

## 2.10 Recovery / Context Window 계열

| 요소 | 설명 | OmG 관점 |
|---|---|---|
| Session recovery | missing tool results, malformed outputs 등 복구 | recovery 참고 |
| Thinking block recovery | provider format mismatch 복구 | provider adapter 참고 |
| Empty message recovery | session history reconstruction | trace/session 참고 |
| Context window exceeded | intelligent compaction | context manager 핵심 후보 |
| JSON parse recovery | malformed tool output 처리 | robust runtime 후보 |

---

## 2.11 OmO에서 특히 눈여겨볼 것

1. Sisyphus-style orchestrator UX
2. background-agent runtime
3. parallel delegation discipline
4. skill-embedded MCP
5. hash-anchored edit
6. task system with dependencies
7. context window recovery/compaction
8. agent tool restrictions
9. session read/search
10. review-work multi-review pattern

---

# 3. 현재 OmG 요소 도감

## 3.1 핵심 정체성 변화

현재 논의 기준의 OmG는 다음 방향이다.

```text
OmG = 사용자의 취향대로 만드는 메인 AI 개발 호스트
```

이제 OmG는 단순히 기존 host에 붙는 layer가 아니라, 사용자 요청을 직접 받고 여러 agent/model/tool/backend를 운용하는 main host로 간다.

---

## 3.2 현재 코드에 있는 요소

| 영역 | 현재 요소 | 위치 |
|---|---|---|
| Protocol | host event request/response | `internal/protocol` |
| Orchestrator | event dispatch, prompt injection, guard/state 연결 | `internal/orchestrator` |
| Config | layered config load | `internal/config` |
| Registry | agent registry, tool policy, triggers | `internal/registry` |
| Route | prompt text 기반 route result | `internal/route` |
| Prompt | section selection, token budget | `internal/prompt` |
| Guard | tool-before safety decision | `internal/guard` |
| State | lifecycle state transition | `internal/state` |
| Skills | skill yaml/prompt loading | `internal/skills` |
| MCP | lazy process manager | `internal/mcp` |
| Verify | verification evidence model | `internal/verify` |
| Daemon | local daemon/server | `internal/daemon` |
| Adapters | claude/codex/opencode adapters | `adapters/*` |
| CLI | `omg` command | `cmd/omg` |
| Remote observer | remote report/observer pieces | `remote/*` |

---

## 3.3 현재 계획 문서에 있는 요소

| 요소 | 문서 |
|---|---|
| Interface closure P0/P1/P2 | `.omx/plans/oh-my-gunnsama-p*.md` |
| Selective extraction plan | `.omx/plans/greenfield-agent-harness-selective-extraction.md` |
| Reference abstraction | `.omx/plans/oh-my-gunnsama-reference-abstraction.md` |
| Implementation substrate decisions | `.omx/plans/decisions/*.md` |
| Control room PRD/test spec | `.omx/plans/prd-ai-dev-control-room.md`, `test-spec-ai-dev-control-room.md` |

---

## 3.4 OmG에 아직 필요한 요소

| 요소 | 상태 | 설명 |
|---|---|---|
| Main Host Loop | 필요 | 사용자 요청을 직접 받아 목표/계획/실행/검증/보고하는 루프 |
| Agent Runtime | 필요 | leader/worker/background agents 실행 관리 |
| Task Queue | 필요 | persistent task + dependency + claim |
| Mailbox | 필요 | worker report/question/blocker 통신 |
| Event Log | 필요 | append-only run history |
| Shared State | 필요 | 현재 상황판 |
| Context Manager | 필요 | context packet, evidence, pruning, compaction |
| Memory | 필요 | session/project memory promotion |
| Workflow Engine | 필요 | runbook/state-machine 기반 workflows |
| Provider/Model Router | 필요 | 모델/백엔드 선택 |
| Tool Registry | 필요 | shell/file/code-intel/MCP/browser 등 tool catalog |
| Permission System | 필요 | agent/task/tool/file 권한 |
| Observer/UI | 필요 | status/HUD/control room/report |

---

# 4. 요소별 비교 도감

## 4.1 Skill 비교

| 프로젝트 | Skill의 성격 |
|---|---|
| OmX | workflow command + local instructions + optional MCP metadata |
| OmO | specialized knowledge + embedded MCP + domain workflow UX |
| OmG 목표 | agent에게 붙이는 전문 context/capability package |

---

## 4.2 MCP 비교

| 프로젝트 | MCP의 성격 |
|---|---|
| OmX | skill/wiki/state/code-intel capability, team MCP는 deprecated |
| OmO | built-in + skill-embedded tool capability |
| OmG 목표 | agent-to-tool capability bridge, agent-to-agent 통신 아님 |

---

## 4.3 Agent 비교

| 프로젝트 | Agent의 성격 |
|---|---|
| OmX | role prompt + specialist routing + team worker roles |
| OmO | strong personas + background subagents + tool restrictions |
| OmG 목표 | main host 내부의 worker role catalog + runtime-managed workers |

---

## 4.4 Workflow 비교

| 프로젝트 | Workflow의 성격 |
|---|---|
| OmX | `$name` skill/workflow, team/ralph/ultraqa/visual loops |
| OmO | ultrawork, Ralph loop, review-work, planner, keyword modes |
| OmG 목표 | runbook/state-machine workflow engine |

---

## 4.5 Task / Queue 비교

| 프로젝트 | Task의 성격 |
|---|---|
| OmX | team task with claim/lease/transition |
| OmO | persistent task system + background task/session |
| OmG 목표 | central work queue for all agents |

---

## 4.6 Mailbox 비교

| 프로젝트 | Mailbox의 성격 |
|---|---|
| OmX | explicit team mailbox API/state |
| OmO | less explicit; background results/session notifications |
| OmG 목표 | explicit leader/worker communication channel |

---

## 4.7 Event Log 비교

| 프로젝트 | Event의 성격 |
|---|---|
| OmX | team runtime events, wakeable/audit distinction |
| OmO | task/session/background logs and history |
| OmG 목표 | append-only run event log for replay/recovery/observer |

---

## 4.8 Shared State 비교

| 프로젝트 | Shared State의 성격 |
|---|---|
| OmX | team monitor/task/worker/mailbox/shutdown state |
| OmO | background task/session/concurrency state |
| OmG 목표 | current run state snapshot across agents/tasks/context |

---

# 5. OmG로 가져올 때의 이름 후보

| 일반 개념 | OmG식 이름 후보 |
|---|---|
| Task Queue | Work Queue / Mission Board |
| Mailbox | Agent Mailbox / Dispatch Inbox |
| Event Log | Run Ledger / Event Ledger |
| Shared State | Run Board / Control Board |
| Leader/Worker Protocol | Crew Protocol / Mission Protocol |
| Skill | Capability Pack / Skill Pack |
| MCP Manager | Tool Bridge Manager |
| Agent Runtime | Crew Runtime |
| Context Manager | Context Engine / Context Steward |
| Workflow Engine | Runbook Engine |
| Verifier | Evidence Gate / QA Gate |
| Memory | Project Memory / Run Memory |

---

# 6. 읽는 순서 추천

처음 읽을 때는 아래 순서가 좋다.

1. `1. OmX 요소 도감`
2. `2. OmO 요소 도감`
3. `3. 현재 OmG 요소 도감`
4. `4. 요소별 비교 도감`
5. `5. OmG로 가져올 때의 이름 후보`

세부 구현 후보를 보려면 이전 문서도 같이 본다.

- `.omx/plans/omg-omx-omo-import-candidates.md`

---

# 7. 한 줄 요약

OmX는 **team/state/workflow 질서**가 강하고, OmO는 **agent persona/background parallelism/tool UX**가 강하다.  
OmG는 둘을 그대로 합치는 것이 아니라, 사용자의 취향에 맞는 **main AI development host** 안에서 `task queue + mailbox + event log + shared state + context manager + agent runtime`으로 재구성해야 한다.

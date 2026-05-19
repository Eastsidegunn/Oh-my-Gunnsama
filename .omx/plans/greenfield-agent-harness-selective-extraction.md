# OmO + OmX selective extraction 기반 greenfield agent harness 설계 plan

## [구현 에이전트에게]

역할: 너는 이 plan을 코드로 옮기는 구현 에이전트다.

권한 경계:
- plan에 명시된 결정은 변경 금지다. 구현 중 변경 필요성을 발견하면 작업을 중단하고 사람에게 질문한다.
- plan에 `open questions`로 표시된 것은 구현 에이전트가 결정하되, 결정 근거를 구현 로그/커밋 메시지에 기록한다.
- plan과 실제 코드 실체가 충돌하면 작업을 중단하고 사람에게 보고한다.

첫 행동 gate:
- 작업 시작 전 반드시 “내가 이 plan을 어떻게 해석했다” 5줄 요약을 먼저 출력한다.
- 이 echo가 틀리면 사람이 잡는다.

보고 단위:
- 컴포넌트 1개 완료마다 보고한다.
- 보고에는 (1) 무엇을 했는지, (2) plan과 다르게 한 것이 있다면 무엇이며 왜인지, (3) 새로 발견한 미해결 질문을 포함한다.

중단 조건:
1. 이 plan이 인용한 라인이 실제 코드와 다르다.
2. plan의 `[추정]` 또는 `[미검증]` 태그가 결정적 구현 경로에 걸린다.
3. 차용 비용이 이 plan 추정의 2배 이상으로 커진다.

## 0. 가정한 사실

1. 합의 전제: **greenfield core + selective extraction**으로 간다.  
   근거: `/reference/oh-my-codex/crates/omx-runtime-core/src/lib.rs:190-235`는 `RuntimeCommand`가 authority, dispatch, replay, mailbox 명령으로 구성되어 있음을 보여준다.  
   이 인용에서 도출된 것: 기존 `omx-runtime-core`가 provider/model/tool 호출 core가 아니므로, provider-neutral core는 신규 작성하고 일부 부품만 선택 차용한다.

2. 합의 전제: **OmO = OpenCode 플러그인 그 자체, 분리 가능 라이브러리 아님**.  
   근거: `/reference/oh-my-openagent/src/index.ts:1-3`는 `@opencode-ai/plugin`의 `Hooks`, `Plugin`, `PluginModule` 타입을 import하고, `/reference/oh-my-openagent/src/index.ts:23-39`는 `serverPlugin: Plugin` 안에서 OpenCode input을 받아 config를 로드한다.  
   이 인용에서 도출된 것: OmO의 주요 실행 경계는 OpenCode host가 호출하는 plugin callback이다.

3. 합의 전제: **omx-runtime-core = tmux 오케스트레이터, LLM 런타임 아님**.  
   근거: `/reference/oh-my-codex/crates/omx-runtime-core/src/lib.rs:69-80`는 dispatch transport variant가 `Tmux` 하나임을 보여주고, `/reference/oh-my-codex/crates/omx-runtime-core/src/lib.rs:96-114`는 outcome 문자열이 `tmux_send_keys_*`, `leader_pane_missing_deferred`, `missing_tmux_target` 같은 tmux/pane 용어에 묶여 있음을 보여준다.  
   이 인용에서 도출된 것: 이 crate는 LLM provider 호출 런타임이 아니라 tmux dispatch 상태/결과 모델이다.

4. 합의 전제: **omx-mux = MuxAdapter trait + MuxError::Unsupported 로 확장 설계가 이미 됨**.  
   근거: `/reference/oh-my-codex/crates/omx-mux/src/types.rs:271-295`는 `MuxError::Unsupported`와 `MuxAdapter` trait의 `execute(operation)` 계약을 정의한다.  
   이 인용에서 도출된 것: tmux 외 adapter가 일부 operation을 미지원으로 표현할 길이 있으므로 mux 계층은 중상 수준의 차용 후보다.

5. 합의 전제: **sparkshell codex_bridge = codex CLI 바이너리 종속**.  
   근거: `/reference/oh-my-codex/crates/omx-sparkshell/src/codex_bridge.rs:122-148`는 `Command::new("codex")`, `exec`, `--model`, `--sandbox read-only`, `model_reasoning_effort="low"`를 직접 구성한다.  
   이 인용에서 도출된 것: summarizer bridge는 단순 LLM interface가 아니라 Codex CLI 실행 정책까지 포함하므로 그대로 차용하지 않는다.

6. 합의 전제: **Two-mode harness**.
   - Mode A (standalone, v1): 워커 CLI 자식 프로세스를 spawn한다.
   - Mode B (OpenCode plugin, v1.5): OpenCode 안에서 plugin으로 동작한다.
   근거: DEC-007은 v1 LLM 호출 경로를 워커 CLI 위임으로 결정했다.
   이 인용에서 도출된 것: v1은 standalone worker process path를 먼저 구현하고 OpenCode plugin path는 v1.5로 분리한다.

7. 합의 전제: **추적 입력 형식 — JSONL family**.
   DeM 추적 입력은 JSONL 디스크 파일이다.
   - Codex CLI: `~/.codex/sessions/*.jsonl`
   - Claude Code: `~/.claude/projects/.../*.jsonl`
   - Mode B plugin: `~/.harness/traces/*.jsonl`
   Mode B는 OpenCode lifecycle hook에서 자기 trace를 별도 JSONL로 떨어뜨린다. OpenCode SQLite는 우리 무관이다.
   근거: DEC-007은 DeM filesystem-as-truth와 JSONL family 일관성을 결정 근거로 삼았다.
   이 인용에서 도출된 것: session 추적은 host DB adapter가 아니라 JSONL trace file family를 canonical input으로 둔다.

8. 합의 전제: **LLM call trace — Langfuse 통일**.
   session 추적과 별 layer다. 도구 무관하게 Mode A, Mode B 모두 Langfuse로 통일한다.
   근거: DEC-007 사용자 결정.
   이 인용에서 도출된 것: JSONL session trace와 Langfuse LLM call trace를 분리한다.

9. 합의 전제: **Ontology 결정 정책**.
   agent / category / hook / skill ontology는 사용 시점에 결정한다. 책상에서 미리 정의하지 않는다.
   근거: DEC-007 사용자 결정.
   이 인용에서 도출된 것: ontology schema를 선행 고정하지 말고 구현 필요 시 L2/OPEN으로 결정한다.

## 1. 아키텍처 개요 — 컴포넌트 경계, 신규/차용/폐기 분류표

핵심 결정: 새 harness는 `harness-core`를 greenfield로 만들고, OmO에서는 config/registry UX 패턴을, OmX에서는 mux/explore/shell 실행 일부를 selective extraction한다. DEC-007에 따라 v1은 Mode A standalone worker CLI 위임이며 worker는 Codex CLI와 Claude Code다.

| 컴포넌트 | 분류 | 경계 | 결정 | 거부된 대안 |
|---|---|---|---|---|
| Harness Core | 신규 | provider-neutral event/state/session/workflow core | 신규 작성 | `omx-runtime-core`를 core로 승격 |
| Config & Migration | 차용+변경 | Rust `harness-config` crate: JSONC schema, user/project merge, partial parse, migration sidecar | OmO 패턴 차용, Rust 동등 구현 | TOML 중심 또는 OmO schema 그대로 사용 |
| Provider & Model Routing | 신규+부분 차용 | provider registry, capability routing, fallback chain | 신규 작성, fallback schema 패턴 차용 | OpenAI/Codex default model table 유지 |
| Agent Registry | 신규+부분 차용 | arbitrary agent definitions, permissions, skills, model class | 신규 schema 작성, OmO override 필드 일부 차용 | OmO의 고정 AgentOverridesSchema 유지 |
| Tool Registry | 신규+패턴 차용 | host-neutral tool manifest/runtime binding | OmO registry 패턴 차용, OpenCode 타입 제거 | OmO tool-registry 직접 import |
| Skill Registry | 신규+패턴 차용 | multi-source skill discovery + merge | OmO discovery/merge 순서 개념 차용 | OpenCode/Claude path hardcode 유지 |
| Mux Layer | 차용+확장 | input/tail/liveness/attach/detach adapter trait | omx-mux 중상 차용 | runtime-core tmux policy와 결합 |
| Shell Runner | 차용+변경 | argv 실행, output threshold, summarization prompt | sparkshell exec/prompt 차용, codex bridge 제거 | codex_bridge 그대로 사용 |
| Explore Runner | 차용+변경 | read-only allowlist repository lookup | omx-explore allowlist/prompt/fallback pattern 차용, Codex invocation 제거 | allowlist 없는 generic agent search |
| Host Adapters | 신규 | OpenCode/Codex/Claude/generic event bridge | 신규 작성 | OmO plugin 또는 OmX hook을 core로 삼기 |
| Compatibility Views | 신규+패턴 차용 | host별 state/config projection | OmX state authority/view 원칙 차용 | host state 파일을 authoritative로 사용 |

근거: `/reference/oh-my-openagent/src/plugin-config.ts:196-314`는 OmO가 user/project config 탐지, legacy migration, merge를 수행함을 보여준다.  
이 인용에서 도출된 것: Config & Migration은 구현 방식의 패턴 차용 가치가 있다.

근거: `/reference/oh-my-openagent/src/plugin/tool-registry.ts:144-152`와 `/reference/oh-my-openagent/src/plugin/tool-registry.ts:166-295`는 tool registry가 `PluginContext`, managers, skill context를 받아 OpenCode `ToolDefinition` record를 조립함을 보여준다.  
이 인용에서 도출된 것: Tool Registry는 패턴은 차용하되 host-neutral contract로 재작성해야 한다.

근거: `/reference/oh-my-codex/crates/omx-mux/src/types.rs:235-295`는 mux operation/outcome/error/adapter trait를 정의한다.  
이 인용에서 도출된 것: Mux Layer는 직접 차용 및 확장 후보다.

근거: `/reference/oh-my-codex/crates/omx-sparkshell/src/exec.rs:29-48`는 argv 기반 command 실행과 output capture를 분리한다.  
이 인용에서 도출된 것: Shell Runner의 argv 실행 부분은 selective extraction 가치가 있다.

## Harness Core

### 책임
Provider-neutral event log, session lifecycle, workflow state, authority, task dispatch, replay를 소유한다.

### 인접 계약
입력/출력 schema 수준:

OmX `omx-runtime-core`의 `RuntimeCommand`/`RuntimeEvent` 중 selective extraction 대상으로 코드 짜며 L2 결정한다. Harness Core는 L1에서 새 `CoreCommand`/`CoreEvent` union을 발명하지 않는다.

Bridge 계약:
- Rust core가 authoritative command/event payload를 소유한다.
- TypeScript는 OpenCode adapter가 v1 필수일 때 그 adapter package 한 곳에만 둔다.
- broad TypeScript bridge generation은 core 기본 전제가 아니다.
- Rust에서 JSONC/schema/migration 동등 구현 불가 또는 v1 OpenCode adapter가 TS package를 adapter 하나로 제한할 수 없음은 DEC-001 정정의 깨지는 조건이다.

Storage 계약:
- JSONL event log가 ground truth다.
- snapshot JSON은 replay/inspection용 요약 상태다.
- raw event log 직접 query, multi-writer, “마지막 줄 버리기”보다 강한 crash recovery는 DEC-002 깨지는 조건이다.

### 차용 출처
근거: `/reference/oh-my-codex/crates/omx-runtime-core/src/lib.rs:17-43`는 command/event 이름 목록이 authority, dispatch, replay, snapshot, mailbox 범위임을 보여준다.  
이 인용에서 도출된 것: authority/dispatch/replay/mailbox vocabulary는 참고하되 provider-neutral core 전체로 일반화하지 않는다.

근거: `/reference/oh-my-codex/crates/omx-runtime-core/src/lib.rs:285-324`는 snapshot이 schema version, authority, backlog, replay, readiness로 구성됨을 보여준다.  
이 인용에서 도출된 것: snapshot shape의 “작은 상태 요약” 패턴은 차용할 수 있다.

근거: `/reference/oh-my-codex/docs/STATE_MODEL.md:12-47`는 authoritative state와 compatibility visibility layer를 분리하고 session precedence를 정의한다.  
이 인용에서 도출된 것: core authoritative state와 host compatibility view를 분리해야 한다.

### keep / change / remove
- keep: authority, backlog/dispatch, replay, snapshot이라는 개념 이름.
- change: transport-specific outcome을 provider-neutral task/session event로 재설계.
- remove: `WorkerCli`, `submit_presses_for_worker_cli`, `DispatchTransportKind::Tmux`를 core에 넣는 설계.

근거: `/reference/oh-my-codex/crates/omx-runtime-core/src/lib.rs:45-67`는 `WorkerCli`와 Codex/Claude별 enter press 정책이 runtime-core에 있음을 보여준다.  
이 인용에서 도출된 것: submit 정책은 새 core의 host/mux policy layer로 이동하고 provider-neutral core에는 넣지 않는다.

### 왜 차용 불가능한가
근거: `/reference/oh-my-codex/crates/omx-runtime-core/src/lib.rs:69-80`와 `/reference/oh-my-codex/crates/omx-runtime-core/src/lib.rs:96-114`는 transport와 outcome이 tmux/pane 의미론에 묶여 있음을 보여준다.  
이 인용에서 도출된 것: 기존 runtime-core를 그대로 쓰면 새 harness가 provider-neutral LLM runtime이 아니라 tmux dispatch machine이 된다.

### trade-off
- 장점: 새 core는 provider, host, mux를 분리할 수 있다.
- 비용: 기존 runtime-core의 테스트와 상태 코드를 대부분 직접 재사용하지 못한다.
- 거부된 대안: `omx-runtime-core`를 fork해서 provider-neutral로 확장한다. 거부 이유는 tmux transport와 outcome이 public enum/string에 이미 노출되어 migration 비용이 신규 작성보다 커질 가능성이 높기 때문이다.

## Config & Migration

### 책임
User/project JSONC config를 로드·검증·부분 복구·마이그레이션하고 canonical normalized config를 출력한다.

### 인접 계약
구현 위치:
- Rust crate `crates/harness-config` (crate name 예시: `harness-config`).
- Provider & Model Routing, Agent Registry, Tool Registry, Skill Registry는 Rust `HarnessConfig` / `ConfigLoadResult` contract를 소비한다.
- TypeScript package는 OpenCode adapter가 v1 필수로 결정될 때만 별도 adapter package로 둔다.
- 아래 `type` block은 schema-level notation이며 TypeScript 구현 위치를 뜻하지 않는다.

입력 schema 수준:
```ts
type ConfigLoadInput = { projectRoot: string; userConfigDir: string; env: Record<string,string|undefined> };
```
출력 schema 수준:
```ts
type HarnessConfig = {
  providers: Record<string, ProviderConfig>;
  models: Record<string, ModelClassConfig>;
  agents: Record<string, AgentConfig>;
  tools: Record<string, ToolConfig>;
  skills: SkillSourcesConfig;
  runtime: RuntimeConfig;
  disabled?: { tools?: string[]; hooks?: string[]; skills?: string[]; agents?: string[] };
};
type ConfigLoadResult = { config: HarnessConfig; warnings: ConfigWarning[]; migrationsApplied: string[] };
```

### 차용 출처
근거: `/reference/oh-my-openagent/src/plugin-config.ts:92-132`는 JSONC parse, migration, schema safeParse, partial parse fallback, error recording을 수행한다.  
이 인용에서 도출된 것: 새 loader는 전체 schema 실패 시 valid section만 살리는 partial-load 전략을 차용한다.

근거: `/reference/oh-my-openagent/src/plugin-config.ts:135-193`는 agents/categories deep merge와 disabled_* set union을 구현한다.  
이 인용에서 도출된 것: user config를 base로, project config를 override로 merge하되 배열형 disable 목록은 set union한다.

근거: `/reference/oh-my-openagent/src/plugin-config.ts:196-314`는 user-level config, project-level config, legacy basename migration, agent definition path resolution, final merge 흐름을 보여준다.  
이 인용에서 도출된 것: 새 harness도 user/project 2단계 config와 legacy migration hook을 둔다.

근거: `/reference/oh-my-openagent/src/shared/migration/config-migration.ts:9-45`는 migration이 raw config copy를 만들고 agent names migration을 적용함을 보여준다.  
이 인용에서 도출된 것: migration은 schema validation 전 raw object 단계에서 수행한다.

근거: `/reference/oh-my-openagent/src/shared/migration/config-migration.ts:75-100`와 `/reference/oh-my-openagent/src/shared/migration/config-migration.ts:151-215`는 migration sidecar와 backup/atomic write/in-memory update를 보여준다.  
이 인용에서 도출된 것: migration history는 config body에 영구 노출하지 말고 sidecar로 이동시키며, 파일 write 실패 시 in-memory migration은 유지한다.

### keep / change / remove
- keep: JSONC parse, partial parse, user/project precedence, legacy migration hook, sidecar migration history, backup+atomic write pattern.
- change: schema는 OmO의 OpenCode-specific fields가 아니라 provider-neutral `providers/models/agents/tools/runtime` 중심으로 새로 작성.
- remove: OpenCode config dir 탐지와 legacy `oh-my-opencode` basename 고정.

### 왜 차용 불가능한가
근거: `/reference/oh-my-openagent/src/config/schema/oh-my-opencode-config.ts:27-79`는 schema가 `claude_code`, `openclaw`, `sisyphus`, `tmux`, `git_master`, `browser_automation_engine` 등 OmO 기능명에 묶여 있음을 보여준다.  
이 인용에서 도출된 것: schema 전체를 그대로 쓰면 새 harness config가 OpenCode/OmO 개념을 public API로 상속한다.

### trade-off
- 장점: 사용자는 JSONC, partial recovery, migration safety를 얻는다.
- 비용: schema를 새로 쓰므로 OmO config와 1:1 호환은 제공하지 않는다.
- 거부된 대안: TOML로 통일한다. 거부 이유는 OmO의 nested agent/model/tool override가 이미 JSONC/Zod 구조로 검증되고 부분 로드 패턴이 검증되어 있기 때문이다.

## Provider & Model Routing

### 책임
Provider 등록, model capability, fallback chain, reasoning/sandbox policy를 provider-neutral하게 해석한다.

### 인접 계약
입력 schema 수준:
```ts
type ModelRequest = {
  capability: "frontier" | "standard" | "fast" | "vision" | "long_context" | string;
  constraints?: { maxCost?: string; reasoning?: string; sandbox?: "read-only" | "workspace-write" };
  fallbackAllowed: boolean;
};
```
출력 schema 수준:
```ts
type ModelResolution = {
  provider: string;
  model: string;
  params: Record<string, unknown>;
  fallbackChain: Array<{ provider: string; model: string; params?: Record<string, unknown> }>;
};
```

### 차용 출처
근거: `/reference/oh-my-openagent/src/config/schema/fallback-models.ts:3-29`는 fallback model을 string, object, string array, object array, mixed array로 받을 수 있음을 보여준다.  
이 인용에서 도출된 것: fallback syntax는 그대로 차용하되 provider-neutral model id로 해석한다.

근거: `/reference/oh-my-openagent/src/config/schema/agent-overrides.ts:5-56`는 model, fallback_models, category, skills, temperature, top_p, prompt, tools, permission, maxTokens, thinking, reasoningEffort, providerOptions를 agent override에 둔다.  
이 인용에서 도출된 것: model params와 providerOptions는 agent-level override syntax로 차용할 수 있다.

근거: `/reference/oh-my-codex/src/config/models.ts:87-90`와 `/reference/oh-my-codex/src/config/models.ts:203-237`는 frontier/standard/spark 기본 모델과 mode-specific/default model resolution을 보여준다.  
이 인용에서 도출된 것: frontier/standard/fast lane 개념은 차용하되 OpenAI default string은 새 config default로 하드코딩하지 않는다.

### keep / change / remove
- keep: OmO fallback model object shape, OmX frontier/standard/fast lane naming idea.
- change: model id는 `provider/model` 형태를 권장하고, capability resolver가 provider/model/params를 반환한다.
- remove: OmX의 OpenAI default model 상수에 의존하는 resolution.

근거: `/reference/oh-my-codex/src/config/models.ts:249-260`는 spark/low-complexity model이 env/config/default로 resolution됨을 보여준다.  
이 인용에서 도출된 것: fast-lane resolution precedence 아이디어는 차용 가능하지만 default model 값은 provider-neutral config로 이동해야 한다.

### 왜 차용 불가능한가
근거: `/reference/oh-my-codex/src/config/models.ts:87-90`는 defaults가 `gpt-*` 문자열임을 보여준다.  
이 인용에서 도출된 것: OmX model config를 그대로 쓰면 OpenAI default가 새 harness public behavior가 된다.

### trade-off
- 장점: OpenAI, Anthropic, Google, local OpenAI-compatible 등 provider를 같은 `ModelResolution`으로 다룰 수 있다.
- 비용: provider capability metadata와 error normalization을 새로 작성해야 한다. [추정]
- 거부된 대안: Codex/OpenAI-compatible API만 공식 지원한다. 거부 이유는 새 harness의 목적이 vendor-neutral greenfield core이기 때문이다.

## Agent Registry

### 책임
역할/에이전트 정의, tool permission, model capability class, prompt/skill binding을 저장한다.

### 인접 계약
입력 schema 수준:
```ts
type AgentRegistryInput = { configAgents: Record<string, AgentConfig>; builtins: AgentDefinition[] };
```
출력 schema 수준:
```ts
type AgentDefinition = {
  name: string;
  description: string;
  modelClass: string;
  permissions: PermissionSet;
  tools: string[];
  skills: string[];
  prompt: { base?: string; append?: string; files?: string[] };
};
```

### 차용 출처
근거: `/reference/oh-my-openagent/src/config/schema/agent-overrides.ts:5-56`는 per-agent override 필드의 유용한 단위를 보여준다.  
이 인용에서 도출된 것: override field vocabulary는 차용한다.

근거: `/reference/oh-my-openagent/src/config/schema/agent-overrides.ts:58-75`는 `AgentOverridesSchema`가 build/plan/sisyphus/hephaestus/prometheus/metis/momus/oracle/librarian/explore/multimodal-looker/atlas 등 고정 key만 열어둠을 보여준다.  
이 인용에서 도출된 것: arbitrary role name을 허용하려면 OmO schema를 그대로 쓰면 안 된다.

### keep / change / remove
- keep: override fields: `fallback_models`, `category`, `skills`, `temperature`, `top_p`, `prompt`, `prompt_append`, `tools`, `permission`, `reasoningEffort`, `providerOptions`.
- change: `agents` schema는 `Record<string, AgentConfig>`로 새로 작성한다.
- remove: fixed Greek-agent key set을 public schema로 유지하는 방식.

### 왜 차용 불가능한가
근거: `/reference/oh-my-openagent/src/config/schema/agent-overrides.ts:58-75`는 agent override keys가 고정 object shape임을 보여준다.  
이 인용에서 도출된 것: `architect`, `executor`, 사용자 정의 역할명을 config에서 자연스럽게 쓰려면 신규 registry schema가 필요하다.

### trade-off
- 장점: 사용자는 역할명을 자유롭게 추가할 수 있다.
- 비용: built-in role migration과 typo detection을 새로 설계해야 한다. [추정]
- 거부된 대안: OmO의 고정 agent set을 baseline으로 유지한다. 거부 이유는 새 harness가 특정 브랜드/신화 역할명에 묶인다.

## Tool Registry

### 책임
Tool manifest를 host-neutral schema로 등록하고, runtime binding을 host adapter 또는 local executor에 연결한다.

### 인접 계약
입력 schema 수준:
```ts
type ToolManifest = {
  name: string;
  description: string;
  inputSchema: JsonSchema;
  outputSchema?: JsonSchema;
  permission: "read" | "write" | "shell" | "network" | "mcp";
  runtime: "local-rust" | "local-js" | "mcp" | "host" | "external";
};
```
출력 schema 수준:
```ts
type ToolCallResult = { ok: true; output: unknown; metadata?: Record<string,unknown> } | { ok: false; error: ToolError };
```

### 차용 출처
근거: `/reference/oh-my-openagent/src/plugin/tool-registry.ts:1-40`는 OpenCode `ToolDefinition`, plugin context, tool factories, task/session/skill/background/grep/glob/ast-grep tools를 한 registry에서 조립함을 보여준다.  
이 인용에서 도출된 것: tool registry composition pattern은 차용한다.

근거: `/reference/oh-my-openagent/src/plugin/tool-registry.ts:82-142`는 low-priority tool order와 max_tools trimming을 구현한다.  
이 인용에서 도출된 것: context/tool-count budget을 위한 priority trimming 개념은 차용한다.

근거: `/reference/oh-my-openagent/src/plugin/tool-registry.ts:144-152`는 registry input이 `PluginContext`, `Managers`, `SkillContext`에 의존함을 보여준다.  
이 인용에서 도출된 것: 기존 함수는 host-neutral registry로 직접 재사용할 수 없다.

근거: `/reference/oh-my-openagent/src/plugin/tool-registry.ts:166-295`는 background, call agent, delegate task, skill_mcp, skill, interactive_bash, task, hashline edit를 결합하고 disabled_tools로 필터링함을 보여준다.  
이 인용에서 도출된 것: available tool classes와 disabled filter pattern은 차용한다.

### keep / change / remove
- keep: factory registry, disabled_tools filter, max_tools trimming, skill/tool aggregation pattern.
- change: OpenCode `ToolDefinition` 대신 internal `ToolManifest` + adapter-specific projection.
- remove: `ctx.client`, `PluginContext`, OpenCode `ToolDefinition`을 core registry contract에 노출.

### 왜 차용 불가능한가
근거: `/reference/oh-my-openagent/src/plugin/types.ts:1-17`는 plugin types가 `@opencode-ai/plugin`의 `Plugin`/`ToolDefinition`으로부터 만들어짐을 보여준다.  
이 인용에서 도출된 것: OmO tool registry를 직접 import하면 OpenCode SDK가 core dependency가 된다.

### trade-off
- 장점: core tools를 Codex/OpenCode/Claude/generic host로 projection할 수 있다.
- 비용: 모든 host adapter별 tool projection layer가 필요하다. [추정]
- 거부된 대안: OpenCode tool registry를 canonical로 삼고 다른 host가 따라오게 한다. 거부 이유는 새 harness가 OpenCode plugin lifecycle에 종속된다.

## Skill Registry

### 책임
여러 source의 skill을 발견, provider/host 조건으로 필터링, 충돌 해결 후 available skill list를 만든다.

### 인접 계약
입력 schema 수준:
```ts
type SkillSource = { type: "builtin" | "user" | "project" | "config" | "plugin"; path?: string; enabled?: boolean };
```
출력 schema 수준:
```ts
type SkillDefinition = { name: string; description: string; scope: string; files: string[]; mcp?: Record<string,unknown> };
type SkillRegistryResult = { mergedSkills: SkillDefinition[]; availableSkills: Array<{name:string;description:string;location:string}> };
```

### 차용 출처
근거: `/reference/oh-my-openagent/src/plugin/skill-context.ts:50-72`는 builtin skills, disabledSkills, system MCP name filtering을 구성한다.  
이 인용에서 도출된 것: disabled skill과 MCP collision filtering 패턴은 차용한다.

근거: `/reference/oh-my-openagent/src/plugin/skill-context.ts:74-118`는 config source, user Claude, global OpenCode, project Claude, project OpenCode, project Agents, global Agents skills를 parallel discovery 후 merge한다.  
이 인용에서 도출된 것: multi-source discovery와 merge ordering은 차용한다.

근거: `/reference/oh-my-openagent/src/plugin/skill-context.ts:120-132`는 merged skills를 available skill list로 projection한다.  
이 인용에서 도출된 것: internal skill definition과 user-visible available list를 분리한다.

### keep / change / remove
- keep: multi-source discovery, provider-gated filtering, availableSkills projection.
- change: source names를 host-neutral로 바꾸고 OpenCode/Claude-specific discovery는 adapter plugin으로 이동.
- remove: default browser provider와 host-specific paths를 core registry에 하드코딩.

### 왜 차용 불가능한가
근거: `/reference/oh-my-openagent/src/plugin/skill-context.ts:74-87`는 discovery 함수들이 Claude/OpenCode/Agents source에 직접 묶여 있음을 보여준다.  
이 인용에서 도출된 것: skill context 함수 전체를 그대로 쓰면 core가 특정 host directory conventions를 상속한다.

### trade-off
- 장점: 기존 생태계의 SKILL.md류 자산을 흡수할 수 있다.
- 비용: source adapter와 conflict policy를 새로 작성해야 한다. [추정]
- 거부된 대안: 새 harness 전용 skill 폴더만 지원한다. 거부 이유는 기존 OmO/OmX 사용성의 핵심인 multi-source skill discovery를 버리게 된다.

## Mux Layer

### 책임
입력 전송, tail capture, liveness check, attach/detach를 adapter trait로 제공한다.

### 인접 계약
입력 schema 수준:
```ts
type MuxOperation =
  | { type: "resolve-target"; target: MuxTarget }
  | { type: "send-input"; target: MuxTarget; envelope: { literalText: string; submit: SubmitPolicy; replaceNewlinesWithSpaces: boolean } }
  | { type: "capture-tail"; target: MuxTarget; visibleLines: number }
  | { type: "inspect-liveness"; target: MuxTarget }
  | { type: "attach" | "detach"; target: MuxTarget };
```
출력 schema 수준:
```ts
type MuxOutcome =
  | { type: "target-resolved"; resolvedHandle: string }
  | { type: "input-accepted"; bytesWritten: number }
  | { type: "tail-captured"; visibleLines: number; body: string }
  | { type: "liveness-checked"; alive: boolean }
  | { type: "attached" | "detached"; handle: string };
```

### 차용 출처
근거: `/reference/oh-my-codex/crates/omx-mux/src/types.rs:5-13`는 mux operation names와 target kinds를 정의한다.  
이 인용에서 도출된 것: operation vocabulary는 차용한다.

근거: `/reference/oh-my-codex/crates/omx-mux/src/types.rs:37-76`는 `SubmitPolicy`와 `InputEnvelope`가 enter presses/delay와 literal text를 파라미터로 받음을 보여준다.  
이 인용에서 도출된 것: mux는 submit 정책을 결정하지 않고 받은 policy를 실행하는 메커니즘 계층이다.

근거: `/reference/oh-my-codex/crates/omx-mux/src/types.rs:235-295`는 MuxOperation/MuxOutcome/MuxError/MuxAdapter trait를 정의한다.  
이 인용에서 도출된 것: mux trait와 `Unsupported` error는 중상 수준으로 차용한다.

근거: `/reference/oh-my-codex/crates/omx-mux/src/tmux.rs:103-130`는 `SubmitPolicy::Enter { presses, delay_ms }`를 받아 tmux enter key를 실행한다.  
이 인용에서 도출된 것: policy lives in runtime-core/orchestrator, mechanism lives in mux 원칙을 유지한다.

### keep / change / remove
- keep: `MuxAdapter`, `MuxOperation`, `MuxOutcome`, `MuxError::Unsupported`, `SubmitPolicy`, `InputEnvelope`.
- change: Rust module name/package는 새 harness namespace로 옮기고 serde casing을 TS schema와 맞춘다.
- remove: `TmuxAdapter`를 유일 adapter라고 가정하는 상위 코드.

근거: `/reference/oh-my-codex/crates/omx-mux/src/tmux.rs:175-192`는 현재 구현 adapter가 `TmuxAdapter`임을 보여준다.  
이 인용에서 도출된 것: 기존 tmux adapter는 keep하되 pty/process adapter를 추가할 수 있게 registry를 새로 둔다.

### 왜 차용 가능한가
근거: `/reference/oh-my-codex/crates/omx-mux/src/types.rs:271-295`는 unsupported error와 trait abstraction을 이미 제공한다.  
이 인용에서 도출된 것: runtime-core와 달리 mux는 policy와 mechanism 분리가 충분히 되어 있어 selective extraction 가치가 높다.

### trade-off
- 장점: tmux path를 빠르게 지원하면서 pty/process adapter 확장 길을 유지한다.
- 비용: `MuxTarget` semantics를 pty/process에 맞게 일반화해야 한다. [추정]
- 거부된 대안: core가 직접 tmux command를 호출한다. 거부 이유는 mux abstraction과 `Unsupported` 확장 지점을 버리게 된다.

## Shell Runner

### 책임
Shell-native command를 argv로 실행하고, 출력이 큰 경우 provider-neutral summarizer에 보낼 prompt payload를 만든다.

### 인접 계약
입력 schema 수준:
```ts
type ShellRunRequest = { argv: string[]; cwd?: string; env?: Record<string,string>; summarizeWhen?: { maxVisibleLines: number } };
```
출력 schema 수준:
```ts
type ShellRunResult = { exitCode: number; stdout: Uint8Array; stderr: Uint8Array; summary?: string; summaryWarnings?: string[] };
```

### 차용 출처
근거: `/reference/oh-my-codex/crates/omx-sparkshell/src/exec.rs:8-27`는 command status/stdout/stderr와 text conversion helpers를 정의한다.  
이 인용에서 도출된 것: `CommandOutput` shape는 차용한다.

근거: `/reference/oh-my-codex/crates/omx-sparkshell/src/exec.rs:29-48`는 empty argv rejection과 `Command::new(argv[0]).args(argv[1..]).output()` 실행을 보여준다.  
이 인용에서 도출된 것: shell metacharacter parsing 없이 argv를 직접 실행하는 메커니즘은 차용한다.

근거: `/reference/oh-my-codex/crates/omx-sparkshell/src/main.rs:42-80`는 tmux pane capture 또는 direct command를 실행하고 line threshold 초과 시 summarize, 실패 시 raw output fallback을 수행한다.  
이 인용에서 도출된 것: raw-vs-summary behavior와 summary failure fallback은 차용한다.

근거: `/reference/oh-my-codex/crates/omx-sparkshell/src/prompt.rs:6-101`는 command family classification을 정의한다.  
이 인용에서 도출된 것: summary prompt에 command family context를 넣는 패턴은 차용한다.

근거: `/reference/oh-my-codex/crates/omx-sparkshell/src/prompt.rs:111-152`는 command, family, exit code, stdout/stderr line/byte counts와 excerpt를 포함하는 summary prompt를 만든다.  
이 인용에서 도출된 것: summarizer input contract는 차용한다.

근거: `/reference/oh-my-codex/crates/omx-sparkshell/src/prompt.rs:170-223`는 line/byte 기준으로 stdout/stderr를 truncate한다.  
이 인용에서 도출된 것: large output prompt bounding은 차용한다.

### keep / change / remove
- keep: argv direct execution, `CommandOutput`, raw-vs-summary fallback, prompt family classification, output truncation.
- change: summarizer call은 Provider & Model Routing의 `ModelRequest { capability: "fast", sandbox: "read-only" }`로 호출한다.
- remove: `codex_bridge.rs`의 Codex CLI invocation.

근거: `/reference/oh-my-codex/crates/omx-sparkshell/src/codex_bridge.rs:58-103`는 summarize가 prompt를 만든 뒤 `run_codex_exec`를 primary/fallback model로 호출함을 보여준다.  
이 인용에서 도출된 것: bridge 호출부는 provider-neutral summarizer interface로 대체해야 한다.

근거: `/reference/oh-my-codex/crates/omx-sparkshell/src/codex_bridge.rs:122-148`는 Codex CLI, read-only sandbox, low reasoning policy를 하드코딩한다.  
이 인용에서 도출된 것: Codex bridge는 remove하고, “read-only + low reasoning summary” 정책만 새 provider adapter로 옮긴다.

### 왜 차용 불가능한가
Codex bridge는 직접 차용 불가다.  
근거: `/reference/oh-my-codex/crates/omx-sparkshell/src/codex_bridge.rs:10-41`는 default model env가 OMX/Codex naming에 묶여 있음을 보여주고, `/reference/oh-my-codex/crates/omx-sparkshell/src/codex_bridge.rs:122-148`는 `codex exec`를 직접 실행한다.  
이 인용에서 도출된 것: summarizer backend는 새 provider abstraction으로 재작성해야 한다.

### trade-off
- 장점: shell execution/prompt bounding은 빠르게 확보한다.
- 비용: provider-neutral summarizer adapter, sandbox policy, model params mapping을 새로 작성한다. [추정]
- 거부된 대안: `codex_bridge.rs`를 그대로 두고 나중에 provider abstraction을 추가한다. 거부 이유는 초기에 Codex CLI dependency가 public runtime behavior로 굳어진다.

## Explore Runner

### 책임
Read-only repository lookup을 allowlisted command 환경에서 실행하고 markdown result를 반환한다.

### 인접 계약
입력 schema 수준:
```ts
type ExploreRequest = { cwd: string; prompt: string; policy: { allowedCommands: string[]; sandbox: "read-only"; modelClass: "fast"; fallbackClass?: "standard" } };
```
출력 schema 수준:
```ts
type ExploreResult = { markdown: string; fallbackUsed?: boolean; warnings: string[] };
```

### 차용 출처
근거: `/reference/oh-my-codex/crates/omx-explore/src/main.rs:11-23`는 allowlist wrapper flags, env cleanup keys, allowed direct commands를 정의한다.  
이 인용에서 도출된 것: allowlisted read-only command boundary는 차용한다.

근거: `/reference/oh-my-codex/crates/omx-explore/src/main.rs:103-145`는 spark attempt 실패 시 fallback attempt를 실행하고 실패 정보를 명시한다.  
이 인용에서 도출된 것: fast model fallback reporting pattern은 차용한다.

근거: `/reference/oh-my-codex/crates/omx-explore/src/main.rs:203-263`는 required args가 cwd/prompt/prompt-file/instructions-file/model-spark/model-fallback임을 보여준다.  
이 인용에서 도출된 것: CLI-level contract는 새 harness에서 config-driven request로 바꾸되 필수 입력 범위는 유지한다.

근거: `/reference/oh-my-codex/crates/omx-explore/src/main.rs:265-313`는 Codex exec를 read-only sandbox, low reasoning, instructions file, restricted PATH로 실행한다.  
이 인용에서 도출된 것: backend invocation은 제거하지만 read-only/low-reasoning/restricted-PATH 정책은 새 adapter로 옮긴다.

근거: `/reference/oh-my-codex/crates/omx-explore/src/main.rs:531-546`는 read-only repository exploration prompt를 구성한다.  
이 인용에서 도출된 것: prompt contract wording과 markdown-only output requirement는 차용한다.

근거: `/reference/oh-my-codex/crates/omx-explore/src/main.rs:548-599`와 `/reference/oh-my-codex/crates/omx-explore/src/main.rs:761-855`는 allowlist wrapper 환경과 shell fragment rejection을 구현한다.  
이 인용에서 도출된 것: wrapper-based allowlist와 shell fragment denylist는 차용한다.

### keep / change / remove
- keep: allowlisted commands, shell startup env sanitization, restricted PATH, fallback event semantics, read-only prompt contract.
- change: `invoke_codex`는 provider-neutral model runner call로 교체한다.
- remove: `CODEX_BIN_ENV`, `codex_support_dir_args`, `codex exec` specific args.

### 왜 차용 불가능한가
근거: `/reference/oh-my-codex/crates/omx-explore/src/main.rs:265-313`는 `invoke_codex`가 Codex CLI args와 output file behavior를 직접 구성함을 보여준다.  
이 인용에서 도출된 것: backend invocation은 selective extraction 대상이 아니라 rewrite 대상이다.

### trade-off
- 장점: safe read-only repository lookup UX를 빠르게 제공한다.
- 비용: allowlist wrapper는 POSIX shell assumptions가 있어 Windows/PTY support는 별도 adapter가 필요하다. [추정]
- 거부된 대안: 일반 shell tool에 “read-only로 사용하라”는 prompt만 준다. 거부 이유는 allowlist enforcement가 없어 안전 경계가 약하다.

## Host Adapters

### 책임
Codex/OpenCode/Claude/generic host lifecycle을 internal event/tool/session schema로 변환한다.

### v1 LLM 호출 경로
DEC-007: v1은 Mode A standalone worker CLI 위임이다. v1 worker는 Codex CLI와 Claude Code다. Mode B OpenCode plugin은 v1.5로 분리한다.

추적 입력:
- Codex CLI: `~/.codex/sessions/*.jsonl`
- Claude Code: `~/.claude/projects/.../*.jsonl`
- Mode B plugin: `~/.harness/traces/*.jsonl`

LLM call trace는 session 추적과 별 layer이며 Mode A/Mode B 모두 Langfuse로 통일한다.

### 인접 계약
입력 schema 수준:
```ts
type HostEvent = { host: HostKind; rawType: string; raw: unknown; sessionId?: string };
```
출력 schema 수준:
```ts
type HarnessEventEnvelope = { host: HostKind; type: CoreCommand["type"] | "tool.call" | "prompt.submit"; payload: unknown };
```

### 차용 출처
근거: `/reference/oh-my-openagent/src/index.ts:121-138`는 OpenCode plugin interface와 `experimental.session.compacting` hook을 반환한다.  
이 인용에서 도출된 것: OpenCode adapter는 plugin return object를 구현해야 하지만 core는 이 return shape를 알면 안 된다.

근거: `/reference/oh-my-codex/src/config/codex-hooks.ts:3-9`와 `/reference/oh-my-codex/src/config/codex-hooks.ts:58-89`는 Codex hook events와 command-hook wrapper 설치 방식을 보여준다.  
이 인용에서 도출된 것: Codex adapter는 SessionStart/PreToolUse/PostToolUse/UserPromptSubmit/Stop을 internal event로 변환한다.

근거: `/reference/oh-my-openagent/src/plugin/types.ts:1-17`는 OpenCode plugin types가 SDK에서 파생됨을 보여준다.  
이 인용에서 도출된 것: OpenCode SDK types는 adapter package에만 위치해야 한다.

### keep / change / remove
- keep: lifecycle event mapping idea, compaction event bridge idea, hook wrapper installation idea.
- change: every host maps into `HarnessEventEnvelope` and receives projected tools/config from core.
- remove: host SDK types from `harness-core`, `harness-config`, `harness-tool-registry`.

### 왜 차용 불가능한가
근거: `/reference/oh-my-openagent/src/index.ts:23-119`는 config, managers, tools, hooks, plugin interface가 OpenCode plugin load 안에서 조립됨을 보여준다.  
이 인용에서 도출된 것: OmO plugin entry를 그대로 core로 쓰면 host adapter boundary가 사라진다.

### trade-off
- 장점: host별 lifecycle 차이를 adapter에 격리한다.
- 비용: OpenCode/Codex/Claude adapter별 projection과 test fixture가 필요하다. [추정]
- 거부된 대안: OpenCode plugin을 primary host로 삼고 Codex를 compatibility mode로 둔다. 거부 이유는 greenfield provider-neutral 목표와 충돌한다.

## Compatibility Views

### 책임
Core authoritative state를 host별 파일/metadata view로 projection하고, host가 필요로 하는 visibility layer를 제공한다.

### 인접 계약
입력 schema 수준:
```ts
type ViewProjectionInput = { coreSnapshot: CoreSnapshot; host: HostKind; projectRoot: string; sessionId?: string };
```
출력 schema 수준:
```ts
type ViewProjection = { files: Array<{ path: string; content: string; authoritative: false }> };
```

### 차용 출처
근거: `/reference/oh-my-codex/docs/STATE_MODEL.md:12-47`는 authoritative per-mode state와 compatibility `skill-active-state.json` visibility layer를 분리한다.  
이 인용에서 도출된 것: 새 harness에서도 authoritative core state와 host view를 분리한다.

근거: `/reference/oh-my-codex/docs/STATE_MODEL.md:75-83`는 transition/state/hook files가 각각 담당하는 역할을 문서화한다.  
이 인용에서 도출된 것: compatibility view는 hook/native messaging을 돕는 projection이어야 하며 transition source of truth가 되면 안 된다.

### keep / change / remove
- keep: authoritative state vs compatibility layer 원칙, session precedence 원칙.
- change: `.omx` path는 새 harness namespace로 바꾸고 host-specific view directory를 둔다.
- remove: compatibility file을 core state source로 읽는 설계.

### 왜 차용 불가능한가
기존 `.omx/state` 파일명은 OmX mode names에 묶여 있다.  
근거: `/reference/oh-my-codex/docs/STATE_MODEL.md:16-27`는 `.omx/state/<mode>-state.json`과 예시 `ralplan`, `ralph`, `team` 파일을 보여준다.  
이 인용에서 도출된 것: 파일 경로와 mode names는 그대로 쓰지 않고 projection policy만 차용한다.

### trade-off
- 장점: host migration과 backward compatibility가 쉬워진다.
- 비용: projection consistency tests가 필요하다. [추정]
- 거부된 대안: host별 state 파일을 authoritative로 삼는다. 거부 이유는 여러 host를 동시에 지원할 때 conflict resolution이 어려워진다.

## 3. 의존 그래프 — 무엇이 무엇보다 먼저 존재해야 하는가

1. `Config & Migration`의 Rust crate `crates/harness-config`가 먼저 있어야 `Provider & Model Routing`, `Agent Registry`, `Tool Registry`, `Skill Registry`가 normalized config를 받을 수 있다.
2. `Harness Core` event/session schema가 먼저 있어야 `Host Adapters`와 `Compatibility Views`가 같은 command/event vocabulary를 projection할 수 있다.
3. `Provider & Model Routing`이 먼저 있어야 `Shell Runner`와 `Explore Runner`가 Codex bridge 없이 summary/explore model call을 할 수 있다.
4. `Mux Layer`가 먼저 있어야 `Host Adapters` 또는 orchestration layer가 tmux/pty/process delivery를 같은 operation으로 호출할 수 있다.
5. `Tool Registry`와 `Skill Registry`가 먼저 있어야 `Agent Registry`의 tools/skills binding을 검증할 수 있다.
6. `Host Adapters`는 마지막에 붙인다. 이유는 OmO와 OmX 모두 host lifecycle에 강하게 묶인 부분이 있어 core 먼저 고정해야 adapter가 core를 오염시키지 않는다.

근거: `/reference/oh-my-openagent/src/index.ts:86-119`는 managers, tools, hooks, pluginInterface가 plugin load 안에서 순서대로 조립됨을 보여준다.  
이 인용에서 도출된 것: 새 harness는 이 조립 순서를 adapter 내부로 제한하고 core/config/registry를 먼저 고정해야 한다.

근거: `/reference/oh-my-codex/crates/omx-sparkshell/src/codex_bridge.rs:58-103`는 summary backend가 model/fallback resolution과 execution bridge를 필요로 함을 보여준다.  
이 인용에서 도출된 것: Shell Runner에서 Codex bridge를 제거하려면 Provider & Model Routing이 선행되어야 한다.

## 4. 거부된 대안 — 결정별 최소 1개

| 결정 | 거부된 대안 | 거부 이유 |
|---|---|---|
| Greenfield core | `omx-runtime-core`를 확장 | transport/outcome이 tmux/pane에 묶임 |
| Config schema 신규 | OmO schema 그대로 사용 | OmO-specific field가 public API로 노출됨 |
| Arbitrary Agent Registry | OmO 고정 AgentOverridesSchema 유지 | 사용자 역할명이 schema에 자연스럽게 열려 있지 않음 |
| Host-neutral Tool Registry | OpenCode `ToolDefinition` canonical | core가 `@opencode-ai/plugin`에 종속됨 |
| Mux Layer 차용 | core가 직접 tmux 호출 | 이미 `MuxAdapter`와 `Unsupported`가 있으므로 직접 호출은 확장성을 버림 |
| Shell summarizer backend 신규 | `codex_bridge.rs` 그대로 사용 | `codex exec --sandbox read-only`에 묶임 |
| Explore backend 신규 | `invoke_codex` 그대로 사용 | Codex CLI args/output file behavior에 묶임 |
| Compatibility view 분리 | host state를 authoritative로 사용 | OmX도 authoritative state와 compatibility layer를 분리함 |

근거: `/reference/oh-my-codex/crates/omx-runtime-core/src/lib.rs:69-114`는 runtime-core transport/outcome이 tmux/pane vocabulary에 묶여 있음을 보여준다.  
이 인용에서 도출된 것: `omx-runtime-core` 확장 대신 greenfield core를 선택한다.

근거: `/reference/oh-my-openagent/src/config/schema/oh-my-opencode-config.ts:27-79`는 OmO schema가 OmO/OpenCode-specific fields를 포함함을 보여준다.  
이 인용에서 도출된 것: OmO schema 그대로 사용하지 않고 새 schema를 작성한다.

근거: `/reference/oh-my-openagent/src/config/schema/agent-overrides.ts:58-75`는 AgentOverridesSchema가 fixed key object임을 보여준다.  
이 인용에서 도출된 것: arbitrary agent registry를 위해 fixed schema를 거부한다.

근거: `/reference/oh-my-openagent/src/plugin/types.ts:1-17`는 OmO plugin/tool types가 `@opencode-ai/plugin`에서 파생됨을 보여준다.  
이 인용에서 도출된 것: OpenCode `ToolDefinition`을 core canonical type으로 삼지 않는다.

근거: `/reference/oh-my-codex/crates/omx-mux/src/types.rs:271-295`는 `Unsupported` error와 `MuxAdapter` trait를 제공한다.  
이 인용에서 도출된 것: core 직접 tmux 호출 대신 mux abstraction을 차용한다.

근거: `/reference/oh-my-codex/crates/omx-sparkshell/src/codex_bridge.rs:122-148`는 shell summarizer backend가 `codex exec --sandbox read-only`에 묶여 있음을 보여준다.  
이 인용에서 도출된 것: `codex_bridge.rs` 직접 차용을 거부한다.

근거: `/reference/oh-my-codex/crates/omx-explore/src/main.rs:265-313`는 explore backend가 Codex CLI args와 output file behavior를 직접 구성함을 보여준다.  
이 인용에서 도출된 것: `invoke_codex` 직접 차용을 거부한다.

근거: `/reference/oh-my-codex/docs/STATE_MODEL.md:12-47`는 authoritative state와 compatibility layer를 분리함을 보여준다.  
이 인용에서 도출된 것: host state를 authoritative로 삼지 않고 compatibility view로 둔다.

## 5. 차용 비용 표

[추정] 시간 단위는 숙련 구현 에이전트 기준 working day다. 이 표는 0 비용을 쓰지 않는다. 각 항목의 정확한 코드 위치는 바로 아래 “차용 항목별 출처” 목록에 citation + 도출문 형태로 둔다.

| 차용 항목 | 신규 작성 대비 절감 시간 | 추출/분리 비용 | keep/change/remove 요약 |
|---|---:|---:|---|
| Config load/merge/partial parse | [추정] 3-5일 | [추정] 2-4일 | keep JSONC/merge/partial, change schema/path, remove OpenCode basename |
| Migration sidecar/backup | [추정] 2-3일 | [추정] 1-2일 | keep sidecar/backup, change migration keys, remove OmO agent migration map |
| Fallback model schema | [추정] 1-2일 | [추정] 0.5-1일 | keep mixed syntax, change model id semantics, remove OpenCode assumptions |
| Agent override field vocabulary | [추정] 2-3일 | [추정] 1-2일 | keep fields, change to Record schema, remove fixed keys |
| Tool registry pattern | [추정] 4-6일 | [추정] 4-7일 | keep composition/trimming/filtering, change ToolManifest, remove PluginContext |
| Skill discovery/merge pattern | [추정] 3-5일 | [추정] 3-5일 | keep multi-source merge, change source adapters, remove host-specific hardcode |
| Mux trait/types | [추정] 3-4일 | [추정] 1-2일 | keep trait/types, change namespace/casing, remove single-adapter assumption |
| Tmux adapter | [추정] 2-3일 | [추정] 1-2일 | keep tmux commands, change error shape if needed, remove top-level tmux policy |
| Shell exec | [추정] 1-2일 | [추정] 0.5-1일 | keep argv execution, change env/cwd policy, remove no policy assumptions |
| Shell summary prompt/truncation | [추정] 2-3일 | [추정] 1-2일 | keep family/prompt/truncate, change provider call, remove Codex bridge |
| Explore allowlist/prompt | [추정] 4-6일 | [추정] 3-5일 | keep allowlist/wrappers/prompt, change model backend, remove Codex exec |
| Codex hook event mapping | [추정] 1-2일 | [추정] 1-2일 | keep event names/wrapper idea, change adapter projection, remove core hook ownership |

### 차용 항목별 출처

근거: `/reference/oh-my-openagent/src/plugin-config.ts:48-132`, `/reference/oh-my-openagent/src/plugin-config.ts:135-193`, `/reference/oh-my-openagent/src/plugin-config.ts:196-314`는 config partial parse, merge, user/project load를 보여준다.  
이 인용에서 도출된 것: Config load/merge/partial parse는 차용하되 schema/path는 변경한다.

근거: `/reference/oh-my-openagent/src/shared/migration/config-migration.ts:9-45`, `/reference/oh-my-openagent/src/shared/migration/config-migration.ts:75-100`, `/reference/oh-my-openagent/src/shared/migration/config-migration.ts:151-215`는 raw migration, sidecar migration, backup/write behavior를 보여준다.  
이 인용에서 도출된 것: Migration sidecar/backup은 차용하되 migration keys는 새 schema에 맞춘다.

근거: `/reference/oh-my-openagent/src/config/schema/fallback-models.ts:3-29`는 fallback model union syntax를 보여준다.  
이 인용에서 도출된 것: Fallback model schema는 거의 그대로 차용할 수 있다.

근거: `/reference/oh-my-openagent/src/config/schema/agent-overrides.ts:5-56`는 agent override field vocabulary를 보여준다.  
이 인용에서 도출된 것: Agent override field vocabulary는 차용하되 fixed key schema는 제거한다.

근거: `/reference/oh-my-openagent/src/plugin/tool-registry.ts:82-152`, `/reference/oh-my-openagent/src/plugin/tool-registry.ts:166-295`는 tool trimming, composition, filtering을 보여준다.  
이 인용에서 도출된 것: Tool registry pattern은 차용하되 OpenCode PluginContext를 제거한다.

근거: `/reference/oh-my-openagent/src/plugin/skill-context.ts:50-132`는 skill discovery, merge, projection을 보여준다.  
이 인용에서 도출된 것: Skill discovery/merge pattern은 차용하되 source adapters를 새로 둔다.

근거: `/reference/oh-my-codex/crates/omx-mux/src/types.rs:37-76`, `/reference/oh-my-codex/crates/omx-mux/src/types.rs:235-295`는 mux input envelope, operation/outcome/error/adapter trait를 보여준다.  
이 인용에서 도출된 것: Mux trait/types는 중상 재사용 후보로 차용한다.

근거: `/reference/oh-my-codex/crates/omx-mux/src/tmux.rs:7-25`, `/reference/oh-my-codex/crates/omx-mux/src/tmux.rs:48-68`, `/reference/oh-my-codex/crates/omx-mux/src/tmux.rs:103-130`, `/reference/oh-my-codex/crates/omx-mux/src/tmux.rs:175-192`는 tmux command execution, send/capture args, submit policy execution, adapter implementation을 보여준다.  
이 인용에서 도출된 것: Tmux adapter는 차용하되 policy decision은 상위 layer에 둔다.

근거: `/reference/oh-my-codex/crates/omx-sparkshell/src/exec.rs:8-48`는 command output shape와 argv execution을 보여준다.  
이 인용에서 도출된 것: Shell exec는 차용하되 env/cwd policy는 새 harness에 맞게 추가한다.

근거: `/reference/oh-my-codex/crates/omx-sparkshell/src/prompt.rs:6-152`, `/reference/oh-my-codex/crates/omx-sparkshell/src/prompt.rs:170-223`는 summary prompt construction과 truncation을 보여준다.  
이 인용에서 도출된 것: Shell summary prompt/truncation은 차용하되 provider call은 변경한다.

근거: `/reference/oh-my-codex/crates/omx-explore/src/main.rs:11-23`, `/reference/oh-my-codex/crates/omx-explore/src/main.rs:531-599`, `/reference/oh-my-codex/crates/omx-explore/src/main.rs:761-855`는 allowlist constants, read-only prompt, wrapper/shell validation을 보여준다.  
이 인용에서 도출된 것: Explore allowlist/prompt는 차용하되 Codex exec backend는 제거한다.

근거: `/reference/oh-my-codex/src/config/codex-hooks.ts:3-9`, `/reference/oh-my-codex/src/config/codex-hooks.ts:58-89`는 Codex hook event names와 managed hook command config를 보여준다.  
이 인용에서 도출된 것: Codex hook event mapping은 adapter에서 차용한다.

## 6. 미해결 질문 (open questions)

1. [미검증] Provider API 최소 표준: streaming, tool calling, structured output을 v1에서 모두 요구할지, text-only + tool proxy부터 시작할지 결정해야 한다.
2. [미검증] `MuxTarget` 일반화: pty/process adapter가 `DeliveryHandle(String)`을 그대로 쓸지, `{ adapter, handle }` 구조로 확장할지 결정해야 한다.
3. [미검증] Skill file compatibility: SKILL.md frontmatter를 그대로 받을지, 새 manifest를 요구할지 결정해야 한다.
4. [미검증] Tool permission vocabulary: OmO permission schema를 참고할지 새 permission DSL을 만들지 결정해야 한다.
5. [미검증] Windows support: omx-explore가 POSIX wrapper 제약을 가진다는 사실을 v1 limitation으로 둘지, powershell allowlist를 v1에 포함할지 결정해야 한다.
6. [결정됨: DEC-002] Event log storage: JSONL event log + snapshot JSON을 사용한다. SQLite/sled 같은 embedded store는 raw event log 직접 query, multi-writer, 또는 “마지막 줄 버리기”보다 강한 crash recovery가 필요해질 때 재검토한다.
7. [결정됨: DEC-007] v1 LLM 호출 경로: 워커 CLI 위임. v1 worker는 Codex CLI와 Claude Code다. Mode B OpenCode plugin은 v1.5로 분리한다. OPEN-006은 v1 폐기, v1.5에서 부활 가능.
8. [미검증] Telemetry: OmO에는 PostHog startup event가 있지만 새 harness에서 telemetry를 둘지 결정해야 한다.

근거: `/reference/oh-my-openagent/src/index.ts:40-67`는 OmO plugin load 시 telemetry event를 시도함을 보여준다.  
이 인용에서 도출된 것: telemetry는 기존 코드에 존재하지만 새 harness에서 채택 여부는 별도 결정이다.

## 7. 검증 기준 — 컴포넌트별 설계 완료 판단

### 공통 검증 기준 형식

모든 컴포넌트의 구현 완료 보고는 아래 3개 필드를 반드시 포함한다.

1. **충족 조건**: 한 문장으로 작성하며, 이 plan의 schema/type/contract 용어를 사용한다.
2. **충족 증거 형식**: 다음 셋 중 하나를 명시한다.
   - 실행 가능한 테스트
   - 실제 실행 산출
   - 외부 비교
3. **동작 동등성 증거**: 차용 컴포넌트는 출처와의 동작 동등성 증거를 최소 1개 포함한다. 신규 컴포넌트는 `신규 작성 — 동작 동등성 대상 없음`이라고 명시한다.

차용 컴포넌트의 동작 동등성 증거 예시:
- 같은 입력 fixture에 대해 새 구현과 reference 구현의 normalized output이 같다.
- reference에서 거부하던 invalid input을 새 구현도 같은 class의 error로 거부한다.
- reference의 public contract를 재현한 golden test가 통과한다.

근거: `/reference/oh-my-codex/crates/omx-mux/src/types.rs:235-295`는 mux operation/outcome/error/adapter trait라는 public contract를 정의한다.  
이 인용에서 도출된 것: 차용 컴포넌트는 source-level 구현 복사가 아니라 public contract 동등성을 검증해야 한다.

근거: `/reference/oh-my-codex/crates/omx-sparkshell/src/prompt.rs:289-341`는 command family selection과 prompt truncation behavior를 테스트로 검증한다.  
이 인용에서 도출된 것: 차용 컴포넌트의 동작 동등성은 실행 가능한 테스트 또는 golden fixture로 증명할 수 있다.

근거: `/reference/oh-my-openagent/src/plugin-config.ts:48-132`는 invalid config section을 partial parse로 복구하는 behavior를 정의한다.  
이 인용에서 도출된 것: Config & Migration처럼 behavior를 차용하는 컴포넌트는 동일 invalid-input fixture에 대한 recovery 결과를 동등성 증거로 제시해야 한다.

### Harness Core 검증 기준
- Core public schema에 host SDK type, `Codex`, `OpenCode`, `tmux` 전용 enum이 없다.
- TypeScript package는 OpenCode adapter가 v1 필수일 때만 존재하며, adapter schema drift는 adapter boundary test에서 검출된다.
- JSONL event log replay로 session/workflow/task snapshot이 결정적으로 재생된다. [추정]
- snapshot JSON이 replay/inspection용 요약 상태로 산출된다.
- 마지막 줄이 손상된 JSONL event log는 마지막 줄을 버리는 방식으로 복구 가능하다.
- `omx-runtime-core`의 tmux outcome string을 core event name으로 복사하지 않았다.

근거: `/reference/oh-my-codex/crates/omx-runtime-core/src/lib.rs:96-114`는 tmux-specific outcome strings를 보여준다.  
이 인용에서 도출된 것: 새 core event vocabulary는 이 문자열을 상속하면 안 된다.

### Config & Migration 검증 기준
- Invalid section이 있어도 valid section은 partial load된다.
- User config base + project config override + array union merge가 테스트된다.
- Migration은 backup 또는 sidecar를 남기고, write failure 시 in-memory config를 반환한다.

근거: `/reference/oh-my-openagent/src/plugin-config.ts:48-132`와 `/reference/oh-my-openagent/src/shared/migration/config-migration.ts:151-215`는 partial load와 migration write/backup behavior를 보여준다.  
이 인용에서 도출된 것: 이 behavior가 새 loader acceptance criterion이다.

### Provider & Model Routing 검증 기준
- Fallback chain은 string/object/mixed syntax를 모두 normalize한다.
- Provider-specific params는 core schema가 아니라 provider adapter params로 격리된다.
- OpenAI default model string이 config 없이 hardcoded default로 노출되지 않는다.

근거: `/reference/oh-my-openagent/src/config/schema/fallback-models.ts:3-29`와 `/reference/oh-my-codex/src/config/models.ts:87-90`는 각각 fallback syntax와 OpenAI default constants를 보여준다.  
이 인용에서 도출된 것: fallback syntax는 채택하고 hardcoded OpenAI default는 거부한다.

### Agent Registry 검증 기준
- `agents.architect`, `agents.executor`, `agents.custom-name` 같은 arbitrary key가 schema-valid하다.
- Typo detection은 warning으로 제공하되 schema rejection으로 막지 않는다. [추정]
- Agent permissions/tools/skills/modelClass가 Tool/Skill/Provider registry에 대해 검증된다.

근거: `/reference/oh-my-openagent/src/config/schema/agent-overrides.ts:58-75`는 fixed key schema를 보여준다.  
이 인용에서 도출된 것: arbitrary key acceptance가 새 registry의 명확한 차별 검증 기준이다.

### Tool Registry 검증 기준
- Core registry test가 `@opencode-ai/plugin` 없이 실행된다.
- Host adapter projection test가 OpenCode ToolDefinition으로 변환한다.
- max_tools trimming과 disabled_tools filtering이 host-neutral manifest에 적용된다.

근거: `/reference/oh-my-openagent/src/plugin/tool-registry.ts:82-142`와 `/reference/oh-my-openagent/src/plugin/types.ts:1-17`는 trimming behavior와 OpenCode type dependency를 보여준다.  
이 인용에서 도출된 것: behavior는 유지하되 dependency는 adapter로 격리한다.

### Skill Registry 검증 기준
- Builtin/user/project/config/plugin source를 deterministic order로 merge한다.
- MCP collision filtering이 테스트된다.
- Available skill list는 internal definition과 별도로 projection된다.

근거: `/reference/oh-my-openagent/src/plugin/skill-context.ts:50-132`는 skill filtering, discovery, merge, projection 흐름을 보여준다.  
이 인용에서 도출된 것: 이 흐름이 새 skill registry acceptance criterion이다.

### Mux Layer 검증 기준
- `MuxAdapter` trait에는 tmux-specific type이 없다.
- `TmuxAdapter`는 existing tmux send/capture/liveness behavior를 통과한다.
- Dummy `ProcessAdapter`는 unsupported operation에서 `MuxError::Unsupported`를 반환한다. [추정]
- Submit press count는 orchestrator가 정하고 mux는 받은 `SubmitPolicy`만 실행한다.

근거: `/reference/oh-my-codex/crates/omx-mux/src/types.rs:37-76`, `/reference/oh-my-codex/crates/omx-mux/src/types.rs:271-295`, `/reference/oh-my-codex/crates/omx-mux/src/tmux.rs:103-130`는 SubmitPolicy, Unsupported, adapter execution을 보여준다.  
이 인용에서 도출된 것: policy/mechanism split이 mux acceptance criterion이다.

### Shell Runner 검증 기준
- argv execution은 shell metacharacter parsing 없이 실행된다.
- Large stdout/stderr는 line/byte 기준으로 truncate된다.
- Summary backend failure 시 raw output fallback을 보존한다.
- Codex CLI binary 없이 unit tests가 실행된다.

근거: `/reference/oh-my-codex/crates/omx-sparkshell/src/exec.rs:29-48`, `/reference/oh-my-codex/crates/omx-sparkshell/src/prompt.rs:170-223`, `/reference/oh-my-codex/crates/omx-sparkshell/src/main.rs:64-76`, `/reference/oh-my-codex/crates/omx-sparkshell/src/codex_bridge.rs:122-148`는 argv execution, truncation, fallback, Codex bridge dependency를 보여준다.  
이 인용에서 도출된 것: shell runner는 exec/prompt behavior를 유지하고 Codex bridge 없이 검증되어야 한다.

### Explore Runner 검증 기준
- Non-allowlisted command는 reject된다.
- Shell wrapper는 `&&`, `||`, `;`, pipe, redirects, command substitution을 reject한다.
- Backend model invocation은 Provider & Model Routing interface를 사용한다.
- Fallback use는 result metadata에 남는다.

근거: `/reference/oh-my-codex/crates/omx-explore/src/main.rs:20-23`, `/reference/oh-my-codex/crates/omx-explore/src/main.rs:831-855`, `/reference/oh-my-codex/crates/omx-explore/src/main.rs:103-145`, `/reference/oh-my-codex/crates/omx-explore/src/main.rs:265-313`는 allowlist, shell rejection, fallback, Codex invocation을 보여준다.  
이 인용에서 도출된 것: allowlist/fallback은 keep, Codex invocation은 replace가 검증 기준이다.

### Host Adapters 검증 기준
- OpenCode adapter만 `@opencode-ai/plugin`을 import한다.
- Codex adapter만 Codex hook event names를 안다.
- Adapter는 raw host event를 `HarnessEventEnvelope`로 변환하는 pure mapping test를 가진다. [추정]

근거: `/reference/oh-my-openagent/src/index.ts:1-3`, `/reference/oh-my-openagent/src/plugin/types.ts:1-17`, `/reference/oh-my-codex/src/config/codex-hooks.ts:3-9`는 OpenCode SDK type dependency와 Codex hook event names를 보여준다.  
이 인용에서 도출된 것: host-specific knowledge는 adapter package에만 있어야 한다.

### Compatibility Views 검증 기준
- Core state가 authoritative이고 host view는 삭제 후 재생성 가능하다.
- Session-scoped view가 root-scoped view보다 우선한다.
- Host view가 stale root state를 resurrect하지 않는다. [추정]

근거: `/reference/oh-my-codex/docs/STATE_MODEL.md:12-47`는 authoritative files, compatibility layer, session precedence, stale root handling을 설명한다.  
이 인용에서 도출된 것: 새 compatibility views도 같은 authority/precedence 원칙을 만족해야 한다.

## 8. Self-check — 이 plan이 self-contained 한가

- 컴포넌트별 신규/차용/폐기 판단이 `1. 아키텍처 개요` 표와 각 컴포넌트 `keep/change/remove`에 명시되어 있다.
- 차용 대상의 정확한 코드 위치가 `/reference/...:line-line` 형태로 각 컴포넌트에 들어 있다.
- 각 결정의 trade-off와 rejected alternative가 컴포넌트별 및 `4. 거부된 대안`에 들어 있다.
- plan에 없는 결정이 다음 단계에서 새로 필요할 수 있는 지점은 `6. 미해결 질문`에 노출되어 있다.
- 모든 확신 불가능/코드 미확인 항목은 `[추정]` 또는 `[미검증]` 태그를 달았다.
- 합의 전제 5개는 `0. 가정한 사실`에 코드 인용과 “이 인용에서 도출된 것” 줄로 미러링했다.

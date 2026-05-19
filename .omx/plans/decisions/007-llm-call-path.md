# DEC-007 — v1 LLM 호출 경로

시점: Config & Migration/DEC-001 정정 후 LLM 호출 주체 결정

## 가설 검증 결과 — OmO/OmX LLM 호출 주체

| 코드베이스 | LLM 호출 주체 | 직접 provider API 호출 여부 | 코드 인용 | 이 인용에서 도출된 것 |
|---|---|---|---|---|
| OmO | OpenCode server / OpenCode SDK client가 session prompt를 처리한다. OmO는 `client.session.promptAsync` / `prompt`에 요청을 넣는다. | OmO 자체가 OpenAI/Anthropic/Gemini API를 직접 호출하는 경로는 확인되지 않았다. package deps도 `@opencode-ai/plugin`, `@opencode-ai/sdk`가 있고 OpenAI/Anthropic/Gemini SDK dependency는 보이지 않는다. | `/reference/oh-my-openagent/src/cli/run/server-connection.ts:1-5`, `/reference/oh-my-openagent/src/cli/run/server-connection.ts:35-42`, `/reference/oh-my-openagent/src/cli/run/server-connection.ts:52-58`, `/reference/oh-my-openagent/package.json:57-73` | OmO CLI는 OpenCode server/client를 만들거나 붙고, provider API dependency를 직접 소유하지 않는다. |
| OmO | Delegate/background task는 OpenCode session prompt API에 agent/model/tool/options payload를 전달한다. | provider/model id는 body에 담지만 호출 실행은 OpenCode session API가 한다. | `/reference/oh-my-openagent/src/tools/delegate-task/sync-prompt-sender.ts:54-67`, `/reference/oh-my-openagent/src/tools/delegate-task/sync-prompt-sender.ts:81-103`, `/reference/oh-my-openagent/src/features/background-agent/manager.ts:1037-1058`, `/reference/oh-my-openagent/src/features/background-agent/manager.ts:1058-1076` | OmO는 LLM provider SDK를 부르는 대신 host session에 prompt를 주입한다. |
| OmO | fallback/retry도 OpenCode session을 abort하고 prompt/promptAsync를 다시 호출한다. | retry logic은 OmO가 하지만 실제 LLM call은 host session API 뒤에 있다. | `/reference/oh-my-openagent/src/plugin/event.ts:150-168`, `/reference/oh-my-openagent/src/plugin/event.ts:294-315` | OmO는 “다른 누군가가 LLM 호출하는 경로”에 개입한다. |
| OmX | `omx-sparkshell`은 `codex exec` 자식 프로세스를 실행하고 stdin으로 prompt를 넣는다. | OmX가 provider API를 직접 호출하지 않는다. `codex` 바이너리가 모델 선택, 인증, provider 호출을 소유한다. | `/reference/oh-my-codex/crates/omx-sparkshell/src/codex_bridge.rs:58-67`, `/reference/oh-my-codex/crates/omx-sparkshell/src/codex_bridge.rs:122-148`, `/reference/oh-my-codex/crates/omx-sparkshell/src/codex_bridge.rs:176-205` | OmX sparkshell은 LLM client가 아니라 Codex CLI process bridge다. |
| OmX | `omx-explore`도 `codex exec`를 read-only sandbox/low reasoning/restricted env로 실행한다. | LLM 호출은 Codex CLI가 한다. OmX는 sandbox/env/prompt/output 파일을 조립한다. | `/reference/oh-my-codex/crates/omx-explore/src/main.rs:265-313` | OmX explore도 worker CLI 위임 패턴이다. |

공통점:
- OmO와 OmX 모두 “harness가 provider API를 직접 호출”하기보다, host/session/worker CLI에 prompt를 주입하고 그 실행 경로를 관찰·보정한다.
- OmO는 OpenCode SDK/server session API에 끼어든다.
- OmX는 Codex CLI process에 끼어든다.
- 차이: OmO는 long-lived host server/session lifecycle에 붙고, OmX sparkshell/explore는 command invocation 단위 child process를 띄운다.

## Rust 생태계 검증 결과 — 후보 (다)의 실질성

| 항목 | 확인 결과 | 판단 |
|---|---|---|
| `async-openai` | docs.rs는 Rust library for OpenAI이며 OpenAI/compatible/Azure 설정을 제공한다고 설명한다. 출처: https://docs.rs/async-openai/latest/async_openai/ | 단일 provider 또는 OpenAI-compatible 중심. 후보 (가)의 일부 부품이지 multi-provider 통합 자체는 아니다. |
| Anthropic Rust SDK류 | `anthropic_sdk` / `anthropic-ai-sdk` / `anthropic-rs` 등 여러 unofficial Anthropic SDK가 있다. 출처: https://docs.rs/anthropic-sdk-rust , https://lib.rs/crates/anthropic-ai-sdk | 단일 provider SDK 선택지가 있지만 fragmented하다. 후보 (가)의 통합 비용이 큼. |
| `genai` | lib.rs는 OpenAI, Anthropic, Gemini, xAI, Ollama, Groq, DeepSeek, Cohere 등을 지원하는 Multi-AI Providers Library라고 설명한다. 출처: https://lib.rs/crates/genai | 후보 (다)의 실질 후보. 다만 2025년 기준 alpha/rc 이력이 있고, v1 핵심 dependency로 채택하려면 별도 proof-of-concept 필요. |
| `llm` crate | lib.rs는 OpenAI, Anthropic, Ollama, DeepSeek, xAI, Google 등 multi-backend unified API를 제공한다고 설명한다. 출처: https://lib.rs/crates/llm | 후보 (다)의 실질 후보. 다만 crate name history/범위가 넓어 dependency risk review 필요. |

판정:
- 후보 (다)는 “존재하지 않음”은 아니다. `genai`, `llm` 같은 multi-provider abstraction 후보가 있다.
- 그러나 v1 core dependency로 바로 고정하기에는 [미검증]이다. 최소 POC가 필요하다: OpenAI-compatible, Anthropic, Gemini 각각 text + tool/function calling + streaming + structured output 중 v1 필요 범위를 같은 trait로 안정적으로 처리하는지 확인해야 한다.

## 후보 (가) — Rust core가 직접 호출 (provider SDK들 통합)

trade-off:
- 장점: 외부 CLI/host에 의존하지 않아 LLM request/response/event schema를 core가 완전히 통제한다.
- 장점: host adapter와 LLM 호출 경로가 분리되어 OpenCode/Codex/Claude Code adapter가 모두 같은 위치가 된다.
- 장점: event log에는 raw request/response/fallback/tool-call metadata를 원하는 granularity로 남길 수 있다.
- 단점: async-openai + Anthropic SDK + Gemini SDK 또는 raw HTTP를 묶는 provider abstraction을 직접 작성해야 한다.
- 단점: auth, rate limit, retry, streaming, tool calling, structured output, model-specific params를 core가 직접 떠안는다.
- 단점: OmO/OmX가 실제로 선택한 “host/worker에 끼어드는” 패턴과 다르므로 selective extraction 이점이 줄어든다.

추천 + 근거:
- 추천하지 않음.
- 근거: OmO/OmX 검증 결과 둘 다 직접 provider API 호출이 아니라 OpenCode session 또는 Codex CLI에 위임한다. 후보 (가)는 greenfield 순수성은 높지만 v1 범위가 급격히 커진다.

깨지는 조건:
- v1이 provider-neutral direct API를 반드시 제공해야 함.
- worker CLI 설치/인증/버전 의존을 받아들일 수 없음.
- LLM 요청/응답 원문을 host/worker가 숨겨서 event log 요구를 만족하지 못함.

영향 범위:
- `Provider & Model Routing`이 v1 최우선 컴포넌트가 된다.
- `Host Adapters` 우선순위는 LLM 호출과 독립된다.
- `Shell Runner`/`Explore Runner`는 provider runner API를 직접 호출해야 한다.
- `OPEN-006`은 OpenCode adapter가 v1 필수일 때만 별도 처리된다.

## 후보 (나) — 워커 CLI에 위임 (OmX 패턴)

trade-off:
- 장점: OmX `codex_bridge`/`invoke_codex`와 같은 검증된 패턴을 따른다: Rust core는 prompt/sandbox/env/timeout/output contract를 관리하고, worker CLI가 인증/provider/model 호출을 처리한다.
- 장점: v1에서 Rust multi-provider SDK 통합을 미룰 수 있다.
- 장점: 사용자 로컬에 이미 설정된 Codex/Claude Code 인증·모델 권한·세션 동작을 활용할 수 있다.
- 장점: OmO와도 구조적으로 닮았다. OmO는 OpenCode session에 prompt를 넣고, OmX는 Codex CLI process에 prompt를 넣는다. 둘 다 “LLM 호출 경로 주변에서 orchestration”한다.
- 단점: worker CLI의 CLI flags, stdout/stderr, exit code, sandbox semantics, output format 변화에 민감하다.
- 단점: event log는 worker CLI가 노출하는 관찰 가능한 결과만 담는다. provider raw response/token/tool-call detail이 필요하면 worker CLI adapter에 의존한다.
- 단점: Codex worker를 첫 worker로 잡으면 OpenAI/Codex 종속 인상이 강해진다. Claude Code worker를 첫 worker로 잡으면 OmX Rust 코드 차용과 거리가 생긴다.
- 단점: interactive worker와 one-shot worker의 차이를 core가 명확히 모델링해야 한다. sparkshell/explore는 one-shot `codex exec`지만 agent harness는 long-running worker/session도 필요할 수 있다.

추천 + 근거:
- 추천: 후보 (나), 첫 worker는 Codex CLI.
- 근거: 현재 코드 검증상 OmX가 실제로 가진 재사용 가능한 LLM path는 `codex exec` process bridge이고, OmO도 직접 provider 호출이 아니라 host session 위임이다. 따라서 v1은 worker CLI 위임으로 시작하는 것이 selective extraction에 가장 충실하다.

깨지는 조건 — 실제로 망가지는 시나리오:
- Worker CLI output contract drift: `codex exec`가 stdout 형식, `-o` output file behavior, stderr error string, exit status semantics를 바꾸면 summary/explore parser가 실패하거나 false success를 기록한다.
- Worker CLI flag drift: `--sandbox read-only`, `model_reasoning_effort="low"`, `model_instructions_file`, `--skip-git-repo-check`, `-C`, `-m/-s` 같은 flags가 바뀌면 실행 자체가 깨진다.
- Auth/session invisibility: worker CLI 내부 인증 만료, account switching, org/project 권한 문제를 core가 구조화된 error로 받지 못하면 fallback/routing/event log가 부정확해진다.
- Streaming/tool-call opacity: worker CLI가 tool call deltas, token usage, structured output schema violations를 원시 event로 노출하지 않으면 core가 provider-neutral event log를 만들 수 없다.
- Sandbox mismatch: core가 read-only/workspace-write 같은 policy를 기록해도 worker CLI sandbox가 실제로 다르게 동작하거나 platform별로 다르면 security boundary가 문서와 달라진다.
- Process lifecycle failure: child process hang, partial stdout, killed process after timeout, orphaned subprocess, stdin close race가 생기면 retry/replay/idempotency가 어렵다. OmX `codex_bridge`도 timeout kill과 stdout/stderr thread join을 직접 다룬다.
- Version skew: 사용자의 로컬 Codex/Claude Code CLI version마다 flags/model names/output behavior가 달라 reproducible harness behavior가 깨진다.
- Vendor-neutrality perception: 첫 worker가 Codex면 “provider-neutral core”라도 v1 UX가 OpenAI/Codex 종속으로 보인다.

영향 범위:
- `Host Adapters`: 첫 host adapter는 Codex hook adapter가 자연스럽다. Codex worker CLI를 호출하고 Codex hook lifecycle을 관찰하는 path가 같은 product surface에 있기 때문이다.
- `OPEN-006`: Codex CLI first면 폐기한다. TypeScript workspace는 v1에 필요 없다. OpenCode adapter가 나중에 필요할 때 단일 TS package OPEN을 새로 연다.
- `Provider & Model Routing`: v1은 direct SDK routing이 아니라 worker CLI capability/routing으로 축소한다.
- `Shell Runner`/`Explore Runner`: OmX codex_bridge/invoke_codex pattern을 selective extraction한다.
- `Mux Layer`: interactive/long-running worker CLI 지원 시 process/pty/tmux adapter semantics가 중요해진다.
- `Compatibility Views`: worker CLI version, command args, sandbox policy, output contract를 event log에 기록해야 한다.

## 후보 (다) — 통합 라이브러리 의존 (Rust 측 Vercel AI SDK 같은 것)

trade-off:
- 장점: direct provider SDK를 직접 모두 감싸는 것보다 빠르게 provider-neutral call surface를 얻을 수 있다.
- 장점: host adapter와 LLM 호출 경로가 분리되어 (가)와 비슷하게 host 우선순위가 낮아진다.
- 단점: Rust multi-provider LLM library 자체가 빠르게 변하는 영역이다. `genai`, `llm` 후보는 있지만 v1 핵심 dependency로 쓰려면 maturity/security/feature parity 검증이 필요하다.
- 단점: library abstraction이 tool calling/streaming/structured output/model-specific params를 충분히 보존하지 못하면 결국 escape hatch를 많이 만들게 된다.
- 단점: dependency가 제공하는 provider set과 error model이 harness public contract를 사실상 결정할 수 있다.

추천 + 근거:
- 추천하지 않음, POC 후 v2 후보로 둔다.
- 근거: `genai`와 `llm`은 실질 후보지만 [미검증]이다. v1은 OmO/OmX 공통 패턴인 위임형 LLM path를 먼저 구현하는 편이 코드 근거와 맞다.

깨지는 조건:
- candidate library가 v1 필수 provider(OpenAI/Anthropic/Gemini)와 필수 기능(streaming/tool/structured output)을 안정적으로 모두 지원함이 POC로 확인됨.
- worker CLI dependency를 제품상 허용할 수 없음.
- direct provider observability가 v1 acceptance로 요구됨.

영향 범위:
- `Provider & Model Routing`이 library adapter 중심으로 바뀐다.
- `Host Adapters` 결정은 LLM 호출 경로와 분리된다.
- `OPEN-006`은 OpenCode adapter 필요 여부와만 연결된다.
- `Shell Runner`/`Explore Runner`는 Codex bridge 제거 후 library call로 대체된다.

## 기존 OPEN-007 처리

기존 `.omx/plans/open/007-v1-adapter-priority.md`의 “host adapter priority” 축은 폐기하고, 이 파일의 “v1 LLM 호출 경로” 축으로 대체한다.

종속 관계:
- 후보 (가) 채택 → host adapter는 LLM 호출과 독립되어 별도 결정.
- 후보 (나) 채택 → 첫 worker CLI가 첫 host adapter를 사실상 유도. Codex worker면 Codex hook adapter가 자연스럽다.
- 후보 (다) 채택 → host adapter는 LLM 호출과 독립되어 별도 결정.


## DECISION 섹션

선택:
- 후보 (나) — 워커 CLI 위임.
- v1 워커 = Codex CLI, Claude Code.

결정 근거:
- DeM filesystem-as-truth와 정합한다.
- JSONL family 일관성을 유지한다.
- OmO/OmX 검증 결과 둘 다 harness가 provider API를 직접 호출하지 않고 host/session/worker CLI에 prompt를 주입하는 패턴이다.

의존한 결정:
- DEC-001 정정 — Rust core 중심, TS는 OpenCode adapter가 v1 필수일 때만.
- DEC-002 — JSONL + snapshot JSON.
- DEC-004 정정 — Config & Migration은 Rust crate.

이 결정에 의존하는 후속:
- Two-mode harness spec 고정.
- Codex CLI worker adapter scaffold.
- Claude Code worker adapter scaffold.
- DeM trace input JSONL path contract.
- Langfuse LLM call trace layer.
- OPEN-006 v1 폐기 / v1.5 부활 기록.

spec 영향:
- `## 0. 가정한 사실`에 Two-mode harness, JSONL family trace input, Langfuse LLM call trace, ontology 결정 정책을 추가한다.
- `## Host Adapters`의 v1 우선순위를 host adapter가 아니라 LLM 호출 경로 기준으로 갱신한다.
- `## 6. 미해결 질문`의 Host adapter priority 항목을 DEC-007로 정리한다.

깨지는 조건:
- Mode B plugin의 OpenCode lifecycle hook이 우리 trace 떨어뜨리기에 부족 — 그러면 OpenCode SQLite adapter 추가 필요.
- v1 acceptance가 Mode B 검증 요구 — Mode A v1 결정 흔들림.
- LLM call layer와 session layer 분리가 DeM 분석에서 비효율 — 통합 모델 재검토.

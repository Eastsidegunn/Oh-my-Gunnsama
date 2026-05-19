# OPEN L1 — v1 어댑터 우선순위

시점: Config & Migration/DEC-001 정정 후 OPEN-006 처리 분기

## 막힌 이유

DEC-001이 Rust core 중심으로 정정되면서 TypeScript는 OpenCode adapter가 v1 필수일 때만 필요해졌다. 따라서 OPEN-006(TypeScript workspace root)은 v1 adapter priority가 결정되기 전에는 처리할 수 없다.

## 후보 A — OpenCode adapter v1 필수

trade-off:
- 장점: OmO의 좋은 사용성 패턴을 가장 직접적으로 검증할 수 있다.
- 장점: OpenCode plugin lifecycle을 실제 adapter boundary로 격리하는 설계를 빠르게 확인할 수 있다.
- 단점: TypeScript package가 v1에 필요해지고, OPEN-006을 “한 package만 두는 최소 npm 설정”으로 다시 처리해야 한다.

## 후보 B — Codex adapter v1 필수

trade-off:
- 장점: OmX의 기존 Codex/CLI 흐름과 사용 맥락이 가까워 Rust core 중심 진행이 단순하다.
- 장점: TypeScript workspace 없이 v1을 진행할 가능성이 높다.
- 단점: OmO 사용성 패턴은 config/registry 수준에서만 검증되고 OpenCode adapter 검증은 뒤로 밀린다.

## 후보 C — Claude Code adapter v1 필수

trade-off:
- 장점: OmO가 Claude/OpenCode skill discovery를 고려한 흔적을 adapter 설계에 반영할 수 있다.
- 단점: 현재 합의 전제와 차용 근거에서 Claude Code adapter가 가장 강한 v1 필수 이유는 아직 부족하다.

## 추천 + 근거

추천: 후보 B — Codex adapter v1 필수.

근거: DEC-001 정정 후 substrate는 Rust core 중심이고, OmX 차용 대상도 Rust crate/CLI 패턴이다. v1에서 TypeScript workspace를 피하면 Config & Migration을 Rust crate로 바로 진행할 수 있다. OpenCode adapter는 OmO 사용성 차용 검증에는 유리하지만 TS package 결정을 다시 열어야 한다.

## 깨지는 조건

- v1 acceptance가 OpenCode plugin host에서 실제로 동작해야 함.
- OmO 사용성 검증이 config/registry pattern만으로 부족하고 OpenCode adapter 실행 검증이 필수임.
- Codex adapter가 OpenAI 종속 문제를 다시 강화해 greenfield/vendor-neutral 목표와 충돌함.

## 영향 범위

- spec 갱신: `## Host Adapters`의 v1 우선순위를 갱신한다.
- spec 갱신: `## 6. 미해결 질문 (open questions)`의 Host adapter priority 항목을 OPEN-007/DEC-007로 연결한다.
- 재개 작업: 후보 B/C 결정 시 OPEN-006 폐기 후 Rust `harness-config` scaffold를 재개한다. 후보 A 결정 시 OPEN-006을 “OpenCode adapter 단일 TS package”로 단순화해 재산출한다.
- 후속 OPEN/작업 가정: Config & Migration Rust crate 작업은 OpenCode adapter가 v1 필수인지에 따라 TypeScript package 동시 scaffold 필요 여부가 갈린다.

## 출처 확인

- `/reference/oh-my-openagent/src/index.ts:1-3`와 `/reference/oh-my-openagent/src/plugin/types.ts:1-4`는 OmO adapter 경계가 OpenCode plugin type에 묶여 있음을 보여준다.
- `/reference/oh-my-openagent/src/tools/background-task/create-background-task.ts:1-5`와 `/reference/oh-my-openagent/src/hooks/context-window-monitor.ts:1-5`는 OmO tool/hook 구현이 OpenCode plugin SDK에 넓게 묶여 있음을 보여준다.
- `/reference/oh-my-codex/crates/omx-runtime-core/src/lib.rs:17-43`와 `/reference/oh-my-codex/crates/omx-mux/src/types.rs:235-295`는 OmX 차용 후보가 Rust command/event vocabulary와 mux trait임을 보여준다.

이 인용에서 도출된 것: OpenCode adapter를 v1 필수로 잡으면 TS package가 필요하고, Codex/Claude Code adapter를 먼저 잡으면 Rust core 중심 진행이 유지된다.

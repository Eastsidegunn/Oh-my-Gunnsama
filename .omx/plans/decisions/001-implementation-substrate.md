# DEC-001 — implementation substrate

시점: Harness Core/구현 substrate 선택

## 선택

Rust core 중심. TypeScript는 OpenCode adapter가 v1 필수일 때 그 adapter package 한 곳에만 둔다.

## 후보

### 후보 A — Rust 단독

trade-off:
- 장점: OmX의 Rust crate 차용 범위와 언어가 맞아 core state/replay, mux, shell/explore 계열 extraction이 단순하다.
- 단점: OmO의 JSONC/Zod config, schema, registry UX 차용을 부정하거나 TypeScript 생태계를 억지로 Rust로 재작성하게 된다.

### 후보 B — TypeScript 단독

trade-off:
- 장점: OmO의 JSONC/Zod config, schema, tool/skill registry pattern과 같은 language substrate라 빠르게 UX 계층을 만들 수 있다.
- 단점: OmX의 Rust crate 차용 범위와 맞지 않아 mux/shell/explore/core extraction을 wrapper 수준이 아니라 재작성으로 밀어낸다.

### 후보 C — Rust core + TypeScript bridge

trade-off:
- 장점: OmX 차용은 Rust 쪽에서 살리고, OmO 차용은 TypeScript bridge/config/registry 쪽에서 살린다.
- 장점: selective extraction 원칙을 구현 substrate에 반영한다.
- 단점: TypeScript bridge 산출물 생성과 Rust/TypeScript schema drift 방지 장치가 v1 필수 작업이 된다.

## 결정 근거

Rust 단독/TypeScript 단독은 selective extraction을 부정한다. OmX의 차용 대상은 Rust이고 OmO의 JSONC/schema 차용 대상은 TypeScript다.

## 깨지는 조건

- TypeScript bridge 산출물 생성 불가.
- core/bridge schema drift.

## 영향 범위

- spec 갱신: `## Harness Core / ### 인접 계약`의 Bridge 계약이 Rust core + TypeScript bridge를 가정한다.
- spec 갱신: Harness Core 검증 기준에는 TypeScript bridge 산출물 생성과 core/bridge schema drift 검출이 포함되어야 한다.
- 재개 작업: Harness Core substrate scaffold, Rust core schema 작성, TypeScript bridge generation 작업을 재개한다.
- 후속 OPEN/작업 가정: DEC-003의 “OmX `RuntimeCommand`/`RuntimeEvent` selective extraction을 L2로 결정”은 Rust core 쪽에서 진행한다고 가정한다.
- 후속 OPEN/작업 가정: Host Adapters, Compatibility Views, Provider & Model Routing, Tool Registry, Skill Registry는 TypeScript bridge를 통해 core schema를 소비한다고 가정한다.
- 후속 OPEN/작업 가정: build/test toolchain은 Rust와 TypeScript 양쪽 산출물을 검증한다고 가정한다.

## 대안

- Rust 단독: OmO 차용을 재작성으로 바꾸므로 거부.
- TypeScript 단독: OmX Rust 차용을 재작성으로 바꾸므로 거부.

## spec 환류 제안

Harness Core 구현 substrate는 Rust core + TypeScript bridge로 고정한다. bridge generation 검증은 Harness Core 검증 기준에 포함한다.


## DECISION 섹션

의존한 결정:
- greenfield core + selective extraction.
- OmO = OpenCode 플러그인 그 자체, 분리 가능 라이브러리 아님.
- OmX 차용 대상은 Rust crate 중심이고 OmO JSONC/schema 차용 대상은 TypeScript 중심이다.

이 결정에 의존하는 후속:
- Harness Core scaffold.
- TypeScript bridge generation.
- Host Adapters가 TypeScript bridge를 소비하는 작업.
- OmX `RuntimeCommand`/`RuntimeEvent` selective extraction L2 결정.

spec 영향:
- `## Harness Core / ### 인접 계약` Bridge 계약.
- Harness Core 검증 기준의 bridge generation 및 schema drift 증거.

깨지는 조건:
- TypeScript bridge 산출물 생성 불가.
- core/bridge schema drift.


## 정정 — 2026-04-30

정정 선택:
- substrate = Rust core 중심.
- TypeScript는 OpenCode adapter가 v1 필수일 때 그 adapter package 한 곳에만 둔다.
- Config & Migration은 TypeScript package가 아니라 Rust crate(예: `harness-config`)로 둔다.

정정 근거:
- OmO의 OpenCode 결합은 plugin/tool/hook/agent SDK 경계에 넓게 퍼져 있으나, Config & Migration 핵심 패턴은 OpenCode plugin SDK 직접 의존이 아니다.
- `/reference/oh-my-openagent/src/index.ts:1-3`, `/reference/oh-my-openagent/src/plugin/types.ts:1-4`, `/reference/oh-my-openagent/src/tools/background-task/create-background-task.ts:1-5`, `/reference/oh-my-openagent/src/hooks/context-window-monitor.ts:1-5`는 plugin/tool/hook 쪽 OpenCode SDK 결합을 보여준다.
- `/reference/oh-my-openagent/src/plugin-config.ts:1-15`, `/reference/oh-my-openagent/src/shared/jsonc-parser.ts:1-5`, `/reference/oh-my-openagent/src/shared/migration/config-migration.ts:1-7`, `/reference/oh-my-openagent/src/config/schema/oh-my-opencode-config.ts:1-25`는 Config & Migration 핵심이 OpenCode plugin SDK 직접 import가 아니라 Node/TypeScript/Zod/jsonc-parser/local helper 기반임을 보여준다.
- [추정] JSONC 파싱, config 발견/머지, legacy migration, schema는 Rust ecosystem(`serde`, `serde_json`, `schemars`, JSONC parser crate 등)으로 동등 구현 가능하다.

정정으로 폐기/변경되는 후속:
- TypeScript bridge generation을 core 기본 전제로 두지 않는다.
- `packages/config` TypeScript package 전제는 폐기한다.
- OPEN-006은 v1 OpenCode adapter 필수 여부가 결정될 때까지 보류한다.

spec 영향:
- `## Harness Core / Bridge 계약`에서 broad TypeScript bridge 전제를 제거한다.
- `## Config & Migration / 구현 위치`를 Rust crate로 갱신한다.
- `## 3. 의존 그래프`에서 Config & Migration dependency를 Rust crate 기준으로 갱신한다.

깨지는 조건:
- Rust에서 JSONC/schema/migration 동등 구현이 불가능함.
- v1 OpenCode adapter가 필수이고 TS package를 adapter 하나로 제한할 수 없음.
- Rust crate 중심 구조가 OmO pattern 차용 비용을 계획 대비 2배 이상 키움.

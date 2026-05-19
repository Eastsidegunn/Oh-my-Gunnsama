# DEC-004 — Config & Migration package layout

시점: Config & Migration/첫 구현 위치 선택

## 막힌 이유

Config & Migration은 TypeScript/Zod 기반 차용 컴포넌트다. 하지만 현재 repo에는 `package.json`, `tsconfig.json`, `packages/`, `src/` 같은 구현 substrate가 없고, spec은 Config & Migration 구현 위치를 명시하지 않는다. 이 결정은 Config & Migration 안에만 갇히지 않고 Provider & Model Routing, Agent Registry, Tool Registry, Skill Registry, Host Adapters의 TypeScript code 위치와 import 경계를 정하므로 L1이다.

## 후보 A — `packages/config` 독립 TypeScript package

trade-off:
- 장점: Config & Migration 경계가 명확하고, 나중에 Provider/Agent/Tool/Skill registry가 별도 package로 붙기 쉽다.
- 장점: OmO에서 차용하는 JSONC/Zod/partial-load/migration 패턴을 TypeScript package 안에 격리할 수 있다.
- 단점: 초기 monorepo/package 설정이 필요하고, package 간 import/test 설정을 먼저 잡아야 한다.

## 후보 B — `packages/harness-ts/src/config` shared TypeScript package 내부 모듈

trade-off:
- 장점: TypeScript bridge/config/registry/adapter를 한 package 안에서 시작하므로 초기 설정이 후보 A보다 단순하다.
- 장점: DEC-001의 TypeScript bridge 소비자 계층을 한 곳에 모을 수 있다.
- 단점: Config & Migration 경계가 약해져서 Tool/Skill/Host adapter 코드와 결합될 위험이 있다.

## 후보 C — repo root `src/config` 단일 TypeScript app 구조

trade-off:
- 장점: 가장 빠르게 파일을 만들 수 있고 초기 package 설정이 단순하다.
- 단점: greenfield harness가 여러 component package로 커질 때 migration 비용이 크다.
- 단점: DEC-001의 Rust core + TypeScript bridge 구조와 package boundary가 모호해진다.

## 추천 + 근거

추천: 후보 A — `packages/config` 독립 TypeScript package.

근거: Config & Migration은 OmO 패턴 차용이 가장 강한 TypeScript 컴포넌트이며, normalized config를 Provider & Model Routing, Agent Registry, Tool Registry, Skill Registry에 공급하는 선행 dependency다. 독립 package가 계약과 테스트 fixture를 가장 명확히 한다.

## 깨지는 조건

- TypeScript package workspace를 둘 수 없는 repo 운영 제약이 있음.
- Config & Migration과 TypeScript bridge를 같은 package에 둬야 하는 build constraint가 있음.
- 초기 구현 속도가 package boundary보다 더 중요한 우선순위로 바뀜.

## 영향 범위

- spec 갱신: `## Config & Migration / ### 인접 계약`에 구현 위치와 package boundary를 추가해야 한다.
- spec 갱신: `## 3. 의존 그래프`에서 Config & Migration이 normalized config를 제공하는 package 이름을 명시해야 한다.
- 재개 작업: Config & Migration scaffold, JSONC parse dependency 선택, schema/test fixture 작성 작업이 재개된다.
- 후속 OPEN/작업 가정: Provider & Model Routing, Agent Registry, Tool Registry, Skill Registry는 Config & Migration package의 `HarnessConfig`/`ConfigLoadResult`를 import한다고 가정한다.
- 후속 OPEN/작업 가정: TypeScript bridge package와 Config & Migration package의 관계를 다음 작업에서 확인해야 한다.

## 출처 확인

- `/reference/oh-my-openagent/src/plugin-config.ts:92-132`는 JSONC parse, schema safeParse, partial parse fallback을 보여준다.
- `/reference/oh-my-openagent/src/plugin-config.ts:135-193`는 user/project merge와 disable 목록 union을 보여준다.
- `/reference/oh-my-openagent/src/shared/migration/config-migration.ts:75-100`와 `/reference/oh-my-openagent/src/shared/migration/config-migration.ts:151-215`는 sidecar/backup/write 실패 시 in-memory migration을 보여준다.
- `/reference/oh-my-openagent/src/config/schema/oh-my-opencode-config.ts:27-79`는 OmO schema 전체를 그대로 차용하면 OmO/OpenCode 기능명이 public API가 됨을 보여준다.

이 인용에서 도출된 것: Config & Migration은 TypeScript package에서 패턴 차용하되 schema는 새로 작성해야 하며, 구현 위치 결정이 선행되어야 한다.


## DECISION 섹션

선택:
- 후보 A — `packages/config` 독립 TypeScript package.

결정 근거:
- 후속 registry들이 이 package를 import하는 구조라 경계 분명한 자리가 맞다.
- Config & Migration은 OmO 패턴 차용이 가장 강한 TypeScript 컴포넌트이며, normalized config를 Provider & Model Routing, Agent Registry, Tool Registry, Skill Registry에 공급하는 선행 dependency다.

의존한 결정:
- DEC-001: Rust core + TypeScript bridge.
- Config & Migration은 OmO의 JSONC/Zod/partial-load/migration pattern을 차용하되 schema는 새로 작성한다.

이 결정에 의존하는 후속:
- Config & Migration scaffold.
- JSONC parse dependency 선택.
- `HarnessConfig` / `ConfigLoadResult` schema 작성.
- Provider & Model Routing, Agent Registry, Tool Registry, Skill Registry가 `packages/config`를 import하는 작업.

spec 영향:
- `## Config & Migration / ### 인접 계약`에 구현 위치와 package boundary를 추가한다.
- `## 3. 의존 그래프`에서 Config & Migration이 `packages/config`의 normalized config를 제공한다고 명시한다.

깨지는 조건:
- TypeScript package workspace를 둘 수 없는 repo 운영 제약이 있음.
- Config & Migration과 TypeScript bridge를 같은 package에 둬야 하는 build constraint가 있음.
- 초기 구현 속도가 package boundary보다 더 중요한 우선순위로 바뀜.


## 정정 — 2026-04-30

정정 선택:
- 기존 선택 `packages/config` 독립 TypeScript package는 폐기한다.
- Config & Migration 구현 위치는 Rust crate(예: `crates/harness-config`)로 정정한다.

정정 근거:
- DEC-001 정정: substrate = Rust core 중심이며 TS는 OpenCode adapter가 v1 필수일 때 그 adapter package 한 곳에만 둔다.
- Config & Migration 차용은 OmO TypeScript 코드 직접 이식이 아니라 JSONC parse, partial recovery, merge, migration sidecar/backup pattern의 selective extraction이다.

의존한 결정:
- DEC-001 정정 — Rust core 중심.

이 결정에 의존하는 후속:
- Rust `harness-config` scaffold.
- Rust JSONC parser/schema/migration crate 선택.
- Provider & Model Routing, Agent Registry, Tool Registry, Skill Registry가 Rust config crate 또는 generated config schema를 소비하는 구조.

spec 영향:
- `## Config & Migration / ### 인접 계약` 구현 위치.
- `## 3. 의존 그래프` Config & Migration package/crate 표현.

깨지는 조건:
- Rust config crate에서 OmO partial-load/migration behavior 동등성을 만들 수 없음.
- v1 OpenCode adapter가 Config & Migration을 TS package로 직접 소비해야 함.

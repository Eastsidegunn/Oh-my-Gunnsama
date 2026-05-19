# OPEN L1 — TypeScript workspace root

시점: Config & Migration/scaffold 전 workspace 결정

## 막힌 이유

DEC-004는 `packages/config` 독립 TypeScript package를 선택했고, DEC-005는 `jsonc-parser` dependency를 선택했다. 하지만 현재 repo에는 root `package.json`, workspace manifest, lockfile, `tsconfig.json`이 없다. `packages/config`를 실제로 scaffold하려면 package manager/workspace 경계를 정해야 한다. 이 결정은 Config & Migration만이 아니라 Provider & Model Routing, Agent Registry, Tool Registry, Skill Registry, Host Adapters의 TypeScript package import/test/build 경로에 영향을 주므로 L1이다.

## 후보 A — npm workspaces root + `packages/config`

trade-off:
- 장점: Node 기본 package manager라 추가 workspace 파일이 적고, root `package.json`의 `workspaces`만으로 시작 가능하다.
- 장점: `packages/config` 독립 package와 후속 registry package import 구조를 표현할 수 있다.
- 단점: monorepo ergonomics는 pnpm보다 약하고, dependency hoisting/lockfile 정책을 별도 관리해야 한다.

## 후보 B — pnpm workspace root + `packages/config`

trade-off:
- 장점: monorepo package 경계와 workspace dependency 관리가 명확하다.
- 장점: 후속 `packages/*` 구조가 늘어날 때 가장 예측 가능하다.
- 단점: `pnpm-workspace.yaml`과 pnpm 사용 전제가 추가되고, repo 운영자가 pnpm을 원하지 않으면 깨진다.

## 후보 C — root workspace 없이 `packages/config` package-local scaffold만 작성

trade-off:
- 장점: 가장 작은 변경으로 `packages/config` 파일을 만들 수 있다.
- 단점: 후속 registry들이 `packages/config`를 import하는 구조가 root에서 검증되지 않는다.
- 단점: DEC-004의 “후속 registry들이 이 package를 import”한다는 결정의 build/test 증거가 약해진다.

## 추천 + 근거

추천: 후보 A — npm workspaces root + `packages/config`.

근거: repo가 비어 있고 package manager 선호가 spec에 없으므로, 가장 표준적인 root `package.json` + `workspaces`가 최소 전제다. pnpm은 더 강한 monorepo 선택이지만 운영 제약을 새로 만든다. package-local만 만들면 DEC-004의 후속 import 구조를 검증하기 어렵다.

## 깨지는 조건

- repo 운영자가 pnpm/bun/yarn 같은 특정 package manager를 요구함.
- root `package.json`을 두면 안 되는 repo 운영 제약이 있음.
- TypeScript packages를 root workspace가 아니라 각 package 독립 배포 단위로만 관리해야 함.

## 영향 범위

- spec 갱신: `## Config & Migration / ### 인접 계약`의 구현 위치 아래에 root workspace 방식을 추가해야 한다.
- spec 갱신: `## 3. 의존 그래프`에서 `packages/config` import 검증이 root workspace build/test를 통해 수행된다고 명시해야 한다.
- 재개 작업: root `package.json`, `tsconfig.base.json`, `packages/config/package.json`, `packages/config/tsconfig.json`, `packages/config/src/*` scaffold 작성이 재개된다.
- 후속 OPEN/작업 가정: Provider & Model Routing, Agent Registry, Tool Registry, Skill Registry, Host Adapters는 root workspace의 `packages/*` package로 추가된다고 가정한다.
- 후속 OPEN/작업 가정: TypeScript bridge package도 root workspace 안의 별도 package 또는 generated artifact consumer로 배치된다고 가정한다.

## 출처 확인

- `/reference/oh-my-openagent/src/shared/jsonc-parser.ts:24-58`는 Config & Migration이 차용할 JSONC parse behavior가 TypeScript dependency 기반임을 보여준다.
- `/reference/oh-my-openagent/package.json:69`, `/reference/oh-my-openagent/package.json:79-80`는 OmO가 `jsonc-parser`, `typescript`, `zod` dependency를 package manifest로 관리함을 보여준다.
- `.omx/plans/decisions/004-config-migration-package-layout.md`는 `packages/config` 독립 TypeScript package와 후속 registry import 구조를 결정했다.

이 인용에서 도출된 것: `packages/config` scaffold는 package manifest와 workspace-level import/test 경계 결정이 선행되어야 한다.


## 상태 — 보류

보류 이유:
- DEC-001이 Rust core 중심으로 정정되었고, Config & Migration은 Rust crate로 이동한다.
- TypeScript 영역은 v1 OpenCode adapter가 필수일 때만 필요하다.

처리 규칙:
- OPEN-007에서 v1 OpenCode adapter가 필수로 결정되면 OPEN-006은 “한 package만 두는 최소 npm 설정, workspace 불필요”로 단순화한다.
- OPEN-007에서 OpenCode adapter가 v1 필수가 아니면 OPEN-006은 폐기한다.

의존한 결정:
- DEC-001 정정.
- DEC-004 정정.

깨지는 조건:
- TypeScript package가 OpenCode adapter 외에도 v1 필수가 됨.


## DEC-007 이후 처리

상태:
- v1 폐기.
- v1.5 Mode B(OpenCode plugin)에서 부활 가능.

근거:
- DEC-007은 v1을 Mode A standalone worker CLI spawn으로 결정했다.
- TypeScript package는 Mode B OpenCode plugin(v1.5)에서만 필요하다.

처리:
- v1 scaffold에서는 TypeScript workspace를 만들지 않는다.
- Mode B v1.5 작업이 시작될 때 “OpenCode adapter 단일 TS package, workspace 불필요 또는 최소 npm 설정” 축으로 새 OPEN을 만든다.

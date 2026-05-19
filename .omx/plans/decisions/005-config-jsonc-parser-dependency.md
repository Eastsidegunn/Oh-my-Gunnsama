# DEC-005 — Config & Migration JSONC parse dependency

시점: Config & Migration/JSONC parse dependency 선택

## 선택

`jsonc-parser`를 `packages/config`의 JSONC parser dependency로 사용한다.

## 대안

- 직접 JSONC parser 작성: comment/trailing comma/BOM/error offset behavior를 직접 맞춰야 하므로 거부.
- `JSON.parse` + comment stripping helper: 문자열 내부 URL, escape, offset error reporting을 안전하게 처리하기 어려워 거부.
- 다른 JSON5/JSONC parser dependency: OmO 차용 출처와 동작 동등성 fixture를 맞추는 비용이 커져 거부.

## 근거

- `/reference/oh-my-openagent/src/shared/jsonc-parser.ts:24-42`는 `parseJsonc`가 BOM을 제거하고 `jsonc-parser`의 `parse`를 `allowTrailingComma: true`, `disallowComments: false`로 호출한 뒤 parse error를 `SyntaxError`로 올리는 것을 보여준다.
- `/reference/oh-my-openagent/src/shared/jsonc-parser.ts:44-58`는 safe parse가 data/null과 offset/length 포함 errors를 반환함을 보여준다.
- `/reference/oh-my-openagent/package.json:69`는 OmO가 `jsonc-parser` dependency를 사용함을 보여준다.

이 인용에서 도출된 것: Config & Migration의 JSONC parse behavior 차용은 `jsonc-parser`를 쓰는 것이 출처 동작 동등성 증거를 만들기 가장 쉽다.

## 의존 결정

- DEC-004: Config & Migration package layout은 `packages/config` 독립 TypeScript package다.
- Config & Migration spec: JSONC parse와 partial parse를 OmO pattern에서 차용한다.

## 의존한 결정

- DEC-004: `packages/config` 독립 TypeScript package.

## 이 결정에 의존하는 후속

- `packages/config/package.json` dependency 작성.
- `packages/config/src/jsonc.ts` 작성.
- Config load tests에서 comment/trailing comma/BOM/error fixture 작성.

## spec 영향

- `## Config & Migration / ### 차용 출처`에 OmO `shared/jsonc-parser.ts` 출처를 추가할 수 있다.
- `## 5. 차용 비용 표 / 차용 항목별 출처`의 Config load/merge/partial parse 항목에 JSONC parser behavior 출처를 보강할 수 있다.

## 깨지는 조건

- repo 운영 정책상 `jsonc-parser` dependency 추가가 금지됨.
- `jsonc-parser`가 target runtime/bundler에서 사용 불가함.
- JSONC parse error shape를 OmO와 다르게 설계해야 하는 요구가 생김.


## 정정 — 2026-04-30

정정 선택:
- `jsonc-parser` TypeScript dependency 결정은 폐기/보류한다.
- Config & Migration이 Rust crate로 이동하므로 Rust JSONC parser crate 선택을 별도 결정해야 한다.

의존한 결정:
- DEC-001 정정 — Rust core 중심.
- DEC-004 정정 — Config & Migration은 Rust crate.

이 결정에 의존하는 후속:
- Rust JSONC parser crate 후보 조사/결정.
- OmO `parseJsonc` behavior(comment/trailing comma/BOM/error shape)에 대한 Rust 동등성 fixture 작성.

spec 영향:
- `## Config & Migration / ### 차용 출처`는 OmO JSONC behavior를 유지하되 TypeScript dependency를 전제로 하지 않는다.

깨지는 조건:
- Rust JSONC parser로 OmO 동작 동등성 fixture를 만족할 수 없음.

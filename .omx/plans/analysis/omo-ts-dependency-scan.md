# OmO TS 종속 직접검증 — 2026-04-30

시점: Config & Migration/DEC-001 정정 요청 검증

## 검증 명령 요약

- `reference/oh-my-openagent/src` 아래 `.ts/.tsx` 파일 수: 1811.
- `@opencode-ai/plugin` 문자열 포함 파일 수: 전체 196, non-test 150.
- `@opencode-ai/sdk` 문자열 포함 파일 수: 전체 48, non-test 38.

## 코드 인용

- `/reference/oh-my-openagent/src/index.ts:1-3`는 OmO plugin entry가 `@opencode-ai/plugin` 타입을 직접 import함을 보여준다.
- `/reference/oh-my-openagent/src/plugin/types.ts:1-4`는 plugin context/type이 `@opencode-ai/plugin`의 `Plugin`에서 파생됨을 보여준다.
- `/reference/oh-my-openagent/src/tools/background-task/create-background-task.ts:1-5`는 tool 구현이 `@opencode-ai/plugin`의 `tool`, `PluginInput`, `ToolDefinition`에 직접 묶여 있음을 보여준다.
- `/reference/oh-my-openagent/src/hooks/context-window-monitor.ts:1-5`는 hook 구현도 `PluginInput`을 통해 OpenCode plugin SDK에 묶여 있음을 보여준다.
- `/reference/oh-my-openagent/package.json:64-80`는 OmO가 `@opencode-ai/sdk`, `jsonc-parser`, `typescript`, `zod`를 package dependency/devDependency로 사용함을 보여준다.
- `/reference/oh-my-openagent/src/plugin-config.ts:1-15`는 Config loader가 `fs`, `path`, local schema/parser/migration helper를 import하고 `@opencode-ai/plugin`을 직접 import하지 않음을 보여준다.
- `/reference/oh-my-openagent/src/shared/jsonc-parser.ts:1-5`는 JSONC parser helper가 Node fs/path, `jsonc-parser`, local plugin identity에 의존함을 보여준다.
- `/reference/oh-my-openagent/src/shared/migration/config-migration.ts:1-7`는 migration helper가 Node fs, logger, atomic write, migration helpers/sidecar에 의존함을 보여준다.
- `/reference/oh-my-openagent/src/config/schema/oh-my-opencode-config.ts:1-25`는 schema가 `zod`와 local schema modules에 의존함을 보여준다.

## 이 인용에서 도출된 것

OmO의 OpenCode 결합은 plugin/tool/hook/agent SDK 경계에 넓게 퍼져 있다. 반면 Config & Migration의 핵심 패턴(JSONC parse, config discovery/merge, legacy migration, schema validation)은 OpenCode plugin SDK 자체가 아니라 Node/TypeScript/Zod/jsonc-parser 구현에 묶여 있다. 따라서 차용 대상은 TypeScript 코드 직접 이식이 아니라 Rust에서 동등 패턴을 재구현하는 것으로 정정 가능하다.

## 불확실성

- [추정] JSONC 파싱, schema generation, validation은 Rust ecosystem(`serde`, `serde_json`, `schemars`, JSONC parser crate 등)으로 동등 구현 가능하다. 실제 crate 선택은 별도 결정이 필요하다.
- [추정] TS는 v1 OpenCode adapter가 필수일 때만 필요하다. v1 adapter priority가 아직 결정되지 않았다.

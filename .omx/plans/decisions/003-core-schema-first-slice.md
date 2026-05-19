# DEC-003 — core schema 1차

시점: Harness Core/core schema 1차 선택

## 선택

폐기 — 잘못된 질문.

## 결정 근거

- raw event 처리는 DeM 담당이다.
- Harness Core는 자기 schema를 새로 발명하지 않는다.
- OmX `omx-runtime-core`의 `RuntimeCommand`/`RuntimeEvent` 이름 목록이 이미 존재한다.
- selective extraction 원칙상 작업에 필요한 것만 차용한다.
- v1 lock은 탁상공론이다.

## OmO event/command schema 식별 결과

OmO에는 Harness Core가 차용할 compact command/event schema에 해당하는 정의가 확인되지 않았다.

- `/reference/oh-my-openagent/src/plugin/event.ts:141-148`는 OpenCode hook 입력 타입에서 event input을 가져와 handler를 만든다.
- `/reference/oh-my-openagent/src/plugin/event.ts:242-272`는 들어온 event를 다수 hook으로 fan-out한다.
- `/reference/oh-my-openagent/src/plugin/event.ts:370-406`는 `session.created`를 OpenCode session event로 처리한다.
- `/reference/oh-my-openagent/src/features/background-agent/constants.ts:29-38`는 `type: string`과 optional properties의 loose event shape만 둔다.
- `/reference/oh-my-openagent/src/features/tmux-subagent/session-created-event.ts:19-27`는 `session.created`용 coercion shape를 둔다.
- `/reference/oh-my-openagent/src/openclaw/runtime-dispatch.ts:22-31`는 raw session event 이름을 notification-friendly alias로 매핑한다.
- `/reference/oh-my-openagent/src/tools/slashcommand/types.ts:3-20`와 `/reference/oh-my-openagent/src/config/schema/commands.ts:3-14`는 slash/builtin command metadata이며 runtime command schema가 아니다.

이 인용에서 도출된 것: OmO의 event/command 관련 코드는 OpenCode hook event 수신, feature-local event 처리, slash command metadata에 묶여 있고, Harness Core의 command/event vocabulary로 차용할 단일 schema가 아니다.

## OmX 비교

- `/reference/oh-my-codex/crates/omx-runtime-core/src/lib.rs:17-43`는 authority/dispatch/replay/snapshot/mailbox command/event 이름 목록을 compact하게 제공한다.

이 인용에서 도출된 것: Harness Core의 selective extraction 후보는 OmO의 loose hook event shape보다 OmX `RuntimeCommand`/`RuntimeEvent` 이름 목록이다.

## 처리

spec §2 Harness Core의 `CoreCommand`/`CoreEvent` 항목을 “OmX omx-runtime-core의 RuntimeCommand/RuntimeEvent 중 selective extraction 대상으로 코드 짜며 L2 결정”으로 갱신한다.

## 깨지는 조건

- OmO에서 compact runtime command/event schema가 추가로 발견됨.
- Harness Core가 DeM이 아니라 raw event normalization을 직접 소유해야 함.
- OmX `RuntimeCommand`/`RuntimeEvent` selective extraction이 작업 중 실제 구현 경로와 맞지 않음.

## 영향 범위

- Harness Core
- Host Adapters
- Compatibility Views
- DeM/JANUS 연계
- generated schema/bindings

## 대안

- session/snapshot first slice를 L1에서 고정: Harness Core가 자기 schema를 발명하게 되므로 거부.
- OmO event shape 차용: OpenCode hook event와 feature-local 처리에 묶여 있으므로 거부.
- OmX 전체 schema 고정: selective extraction이 아니라 wholesale extraction이 되므로 거부.

## spec 환류 제안

Harness Core는 새 `CoreCommand`/`CoreEvent` union을 L1에서 발명하지 않는다. OmX `RuntimeCommand`/`RuntimeEvent` 중 실제 구현에 필요한 이름과 payload만 L2로 결정한다.

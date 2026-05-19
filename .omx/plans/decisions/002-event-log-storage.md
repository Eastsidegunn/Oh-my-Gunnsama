# DEC-002 — event log storage 모델

Status: Superseded by DEC-009 for OmG v1 storage. Historical context only.

시점: Harness Core/event log storage 모델 선택

## 선택

JSONL + snapshot JSON.

## 후보

### 후보 A — JSONL event log + snapshot JSON

trade-off:
- 장점: event log를 ground truth로 유지하고, snapshot은 사람이 읽을 수 있는 요약 상태로 남길 수 있다.
- 장점: DeM이 세션 외부 observer로 단일 writer를 담당한다는 전제와 맞다.
- 단점: raw event log 직접 query, multi-writer, 강한 crash recovery 요구가 생기면 깨진다.

### 후보 B — SQLite event store + snapshot table

trade-off:
- 장점: query/index/transaction이 필요할 때 강하다.
- 단점: JANUS 같은 query 소비자는 raw event log를 직접 query하지 않고 DeM이 가공한 형태를 사용한다는 전제에서 과하다.

### 후보 C — embedded KV event store

trade-off:
- 장점: Rust-native persistence와 key-range access가 가능하다.
- 단점: 사람이 읽기 어렵고 TypeScript/adapter tooling의 직접 inspection UX가 떨어진다.

## 결정 근거

DeM은 세션 외부 observer이며 single-writer다. JANUS 같은 query 소비자는 raw event log를 직접 query하지 않고 DeM이 가공한 형태를 사용한다. event log는 ground truth다. JANUS와 DeM은 이 프로젝트와 sibling directory에 있는 DeM과 JANUS 프로젝트를 의미한다.

## 깨지는 조건

- raw event log 직접 query 필요.
- multi-writer 필요.
- crash recovery가 “마지막 줄 버리기”보다 강해야 함.

## 영향 범위

- spec 갱신: `## Harness Core / ### 인접 계약`의 Storage 계약이 JSONL event log + snapshot JSON을 가정한다.
- spec 갱신: Harness Core 검증 기준에는 JSONL replay 결정성, snapshot JSON 산출, 마지막 줄 손상 처리 증거가 포함되어야 한다.
- 재개 작업: Harness Core persistence scaffold, event append API, snapshot write/read API, replay fixture 작업을 재개한다.
- 후속 OPEN/작업 가정: Compatibility Views는 raw event log를 직접 query하지 않고 snapshot 또는 DeM 가공 산출을 소비한다고 가정한다.
- 후속 OPEN/작업 가정: DeM/JANUS 연계는 DeM이 raw log를 관찰/가공하고 JANUS가 가공 결과를 소비한다고 가정한다.
- 후속 OPEN/작업 가정: SQLite/embedded KV migration은 v1 필수 작업이 아니라 깨지는 조건 발생 시 재검토 대상으로 둔다.

## 대안

- SQLite: raw event log query가 v1 필수가 아니므로 거부.
- embedded KV: inspection UX와 bridge tooling 비용 때문에 거부.

## spec 환류 제안

Harness Core persistence는 JSONL event log와 snapshot JSON을 기본으로 고정한다. DeM/JANUS 연동은 raw log query가 아니라 DeM 가공 산출 소비로 설계한다.


## DECISION 섹션

의존한 결정:
- DeM은 세션 외부 observer이며 single-writer다.
- JANUS 같은 query 소비자는 raw event log 직접 query가 아니라 DeM 가공 산출을 사용한다.
- event log는 ground truth다.

이 결정에 의존하는 후속:
- Harness Core persistence scaffold.
- JSONL event append/replay 구현.
- snapshot JSON write/read 구현.
- Compatibility Views의 read model 설계.
- DeM/JANUS 연계 경계 설계.

spec 영향:
- `## Harness Core / ### 인접 계약` Storage 계약.
- Harness Core 검증 기준의 replay/snapshot/crash-corruption 증거.

깨지는 조건:
- raw event log 직접 query 필요.
- multi-writer 필요.
- crash recovery가 “마지막 줄 버리기”보다 강해야 함.

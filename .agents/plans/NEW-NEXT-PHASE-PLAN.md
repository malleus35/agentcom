# New Next Phase Plan — agentcom v2 Hardening Rebaseline

> 작성일: 2026-03-17
> 기준 브랜치: `develop`
> 기준 버전: `v0.2.3`
> 목적: 현재 전체 코드 기준으로 PH5~PH9 계획을 다시 정렬하고, 이미 끝난 작업/부분 완료 작업을 반영한 실행 가능한 후속 계획을 정의한다.

---

## 개요

기존 `NEXT-PHASE-PLAN.md`는 `feature/P12-user-endpoint` 시점의 정적 분석 결과를 기반으로 작성되었지만, 이후 `P12 user endpoint`와 `PH10 priority-review-policy`가 반영되면서 일부 가정이 더 이상 맞지 않게 되었다.

특히 MCP 도구 집합과 태스크 관련 표면적은 이미 확장되었고, 반대로 JSON-RPC 에러 응답 일관성, UDS timeout/retry, shutdown timeout 처리, terminal state reopen 같은 production-readiness 이슈는 여전히 남아 있다.

이 문서는 다음 원칙으로 재작성한다.

- 이미 구현된 항목은 새 Phase 작업에서 제외하고 상태로만 기록한다.
- 부분 완료 항목은 남은 차이만 정의한다.
- `CLI-first, MCP는 선택적 어댑터` 원칙에 맞춰 PH8 범위를 축소한다.
- production 장애 가능성이 큰 항목부터 우선순위를 재배치한다.

---

## 현재 상태 요약

### 이미 반영된 후속 작업

다음 항목은 기존 계획 작성 이후 이미 코드에 반영되었다.

| 영역 | 현재 상태 | 근거 |
|------|-----------|------|
| MCP task update | 완료 | `internal/mcp/tools.go`, `internal/mcp/handler.go` |
| MCP task review tools | 완료 | `approve_task`, `reject_task` in `internal/mcp/tools.go` |
| MCP human communication | 완료 | `send_to_user`, `get_user_messages` in `internal/mcp/tools.go` |
| priority / reviewer / review_policy | 완료 | `internal/task/model.go`, `internal/task/policy.go`, `internal/mcp/handler.go` |
| onboard/wizard 테스트 | 부분 완료 | `internal/onboard/wizard_test.go`, `internal/onboard/result_test.go` |
| structured user error 기반 | 부분 완료 | `internal/cli/errors.go` |

### 아직 남아 있는 핵심 리스크

| 영역 | 현재 상태 | 영향도 |
|------|-----------|--------|
| MCP JSON-RPC 에러 응답 일관성 | 일부 경로만 `error` 필드 사용 | Critical |
| MCP handler 파라미터 검증 | handler별 품질 편차 존재 | High |
| task terminal state reopen/retry | 미지원 | High |
| `up` shutdown timeout | `context.Background()` 잔존 | High |
| UDS accept/read timeout | 미구현 | High |
| UDS client backoff/jitter | 미구현 | Medium |
| 고아 runtime/socket 정리 | 부분 구현 | Medium |
| runtime config externalization | 미구현 | Medium |
| MCP-CLI parity 재정의 | 기존 계획 과대 | Medium |
| query / transport lifecycle / MCP error 테스트 | 미흡 | Medium |

---

## 상태 레이블

- `done`: 계획 의도가 현재 코드에 이미 충족됨
- `partial`: 일부 구현됐지만 핵심 차이가 남아 있음
- `open`: 아직 구현되지 않음
- `reframed`: 원래 계획은 맞지만 범위/우선순위를 다시 정의해야 함

---

## PH5: MCP & Task Protocol Corrections

> 우선순위: Critical
> 목표: JSON-RPC 응답 일관성 회복 + task 상태머신의 남은 결함 수정

### PH5 상태

| 태스크 | 상태 | 메모 |
|-------|------|------|
| PH5-01 MCP 에러 응답 JSON-RPC 정렬 | done | unknown tool=`-32601`, runtime tool error=`-32000`, error path에서 `result` 제거 + roundtrip 테스트 반영 |
| PH5-02 MCP 파라미터 검증 강화 | done | handler-level JSON/type/required/status/reference validation을 `invalidParamsError`로 정렬하고 regression matrix + manual QA 반영 |
| PH5-03 terminal state reopen/retry | done | `completed -> pending|cancelled`, `failed -> pending|cancelled`, `cancelled -> pending` 지원 + reviewer 회귀 테스트/CLI manual QA 반영 |

### PH5-01: MCP 에러 응답을 전 경로에서 JSON-RPC `error`로 통일
- **대상**: `internal/mcp/server.go`
- **현재 상태**:
  - invalid request / invalid params / unknown tool / tool runtime error가 모두 JSON-RPC `error`를 사용
  - success path만 `result`를 사용하고 error path는 `result`를 포함하지 않음
- **완료 내용**:
  - unknown tool -> `error.code = -32601`
  - invalid params -> `error.code = -32602` 유지
  - tool execution errors -> `error.code = -32000`으로 정렬
  - `internal/mcp/server_test.go`에 unknown tool / invalid params / tool runtime error roundtrip 검증 추가
- **검증**:
  - `go test ./internal/mcp/... -count=1`
  - `go test ./... -count=1`
  - `go build ./...`
  - 수동 QA: `go run ./cmd/agentcom mcp-server` + unknown tool 호출로 `-32601` 응답 확인
- **소요 공수**: 약 2h

### PH5-02: MCP handler 파라미터 검증을 전 handler에 통일
- **대상**: `internal/mcp/handler.go`
- **완료 상태**:
  - `list_agents`, `send_message`, `send_to_user`, `get_user_messages`, `broadcast`, `delegate_task`, `list_tasks`, `get_status`의 JSON unmarshal/type mismatch를 `invalidParamsError`로 통일
  - message/user/delegate 계열의 required-field 오류와 caller-input agent reference 실패를 `-32602` 경로로 승격
  - `list_tasks.status`에 명시적 status validation 추가, `list_tasks.assignee`와 `create_task.assigned_to/created_by`의 permissive fallback은 기존 동작 유지
- **검증**:
  - `internal/mcp/server_test.go` invalid-params/runtime boundary matrix 추가
  - manual MCP STDIO QA로 `-32602` 승격 케이스와 `-32000` 유지 케이스 확인
- **실소요 공수**: 약 3h

### PH5-03: terminal 상태 재전이(reopen/retry/resurrect) 지원
- **대상**: `internal/task/model.go`, `internal/task/model_test.go`, 필요 시 `internal/task/manager_test.go`
- **완료 상태**:
  - `completed -> pending|cancelled`
  - `failed -> pending|cancelled`
  - `cancelled -> pending`
  - `IsTerminal()` semantics 유지, reviewer-aware `in_progress -> completed -> blocked` 동작 유지
- **검증**:
  - `internal/task/model_test.go`, `internal/task/manager_test.go` reopen/retry/resurrect + reviewer regression 추가
  - `go test ./internal/task/... -count=1`
  - `go test ./... -count=1`
  - `go build ./...`
  - manual QA: CLI에서 real task를 `completed -> pending`으로 재오픈해 JSON 출력 확인
- **실소요 공수**: 약 1h

**PH5 예상 잔여 공수: 0h**

---

## PH6: Runtime Reliability & Resource Safety

> 우선순위: High
> 목표: `up/down`, UDS, runtime 정리 경로에서 실제 운영 장애를 줄인다.

### PH6 상태

| 태스크 | 상태 | 메모 |
|-------|------|------|
| PH6-01 supervisor child health monitoring | done | stale child heartbeat 탐지 + fail-fast shutdown 추가 |
| PH6-02 shutdown timeout context | done | timeout-backed cleanup helper로 3경로 정리 |
| PH6-03 UDS accept/read timeout | done | periodic accept deadline + read deadline 추가 |
| PH6-04 UDS retry backoff/jitter | done | bounded retry backoff+jitter 회귀 테스트 반영 |
| PH6-05 rate limit / queue overflow protection | done | per-agent rate limit, inbox FIFO cleanup, duplicate broadcast throttle 추가 |
| PH6-06 orphan runtime/socket cleanup | done | stale runtime auto cleanup + dead socket cleanup 추가 |
| PH6-07 agent name validation | done | regex + reserved `user` name 검증 추가 |
| PH6-08 SQLite runtime health checks | done | `HealthCheck()` helper와 journal/integrity 검증 추가 |

### PH6-01: Supervisor 자식 프로세스 liveness 모니터링
- **대상**: `internal/cli/up.go`
- **완료 상태**: runtime state active agent를 heartbeat와 연결해 stale child를 감지하고 fail-fast shutdown 수행
- **검증**: `TestCollectStaleRuntimeAgents`, full CLI/package tests, PH6 stale-runtime manual QA
- **실소요 공수**: 약 2h

### PH6-02: shutdown 경로의 무기한 블로킹 제거
- **대상**: `internal/cli/up.go`
- **완료 상태**: timeout-backed cleanup helper로 defer / force / graceful shutdown 경로 정리
- **검증**: `TestRunWithCleanupTimeout*`, `agentcom --json down --force` manual QA
- **실소요 공수**: 약 30min

### PH6-03: UDS server accept/read deadline 도입
- **대상**: `internal/transport/uds.go`
- **완료 상태**: periodic accept deadline과 connection read deadline 추가, timeout 경로는 조용히 회수
- **검증**: `TestHandleConnectionReturnsAfterReadTimeout`, transport/full suite
- **실소요 공수**: 약 1.5h

### PH6-04: UDS client retry를 exponential backoff + jitter로 개선
- **대상**: `internal/transport/uds.go`
- **완료 상태**: bounded retry backoff+jitter 추가, retry timing regression 테스트 반영
- **검증**: `TestClientSendRetriesWithBackoff`, transport/full suite
- **실소요 공수**: 약 1.5h

### PH6-05: 메시지 rate limit / overflow 보호
- **대상**: `internal/message/router.go`, `internal/db/message.go`
- **완료 상태**: per-agent rate limit, inbox FIFO cleanup, duplicate broadcast throttle를 router 정책으로 추가
- **검증**: router regression 3종 + full suite
- **실소요 공수**: 약 3h

### PH6-06: stale runtime / orphan socket 정리 강화
- **대상**: `internal/agent/registry.go`, `internal/cli/up.go`
- **완료 상태**: dead agent socket cleanup과 stale runtime auto cleanup 구현, stale supervisor 재시작 경로를 수동 QA로 검증
- **검증**: `TestHandleExistingRuntimeStateRemovesStaleState`, `TestRegistryMarkInactive`, stale runtime manual QA
- **실소요 공수**: 약 2h

### PH6-07: agent name validation 추가
- **대상**: `internal/agent/registry.go`, `internal/cli/register.go`, MCP message/task resolution 진입점
- **완료 상태**: regex + reserved `user` name 정책을 registration path에 추가
- **검증**: `TestRegistryRegisterValidatesAgentName`, agent/full suite
- **실소요 공수**: 약 45min

### PH6-08: SQLite runtime health check 도입
- **대상**: `internal/db/sqlite.go`
- **완료 상태**: `HealthCheck()` helper로 ping, journal mode, integrity check surface 추가
- **검증**: `TestHealthCheck*`, db/full suite
- **실소요 공수**: 약 1h

**PH6 예상 잔여 공수: 0h**

---

## PH7: Runtime Configuration & Observability

> 우선순위: Medium
> 목표: PH6에서 필요한 timeout/retry 수치가 확정된 뒤 외부화와 운영 가시성을 보강한다.

### PH7 상태

| 태스크 | 상태 | 메모 |
|-------|------|------|
| PH7-01 runtime config externalization | done | runtime config/env override surface 추가 및 runtime wiring 연결 |
| PH7-02 structured user errors rollout | done | representative CLI user error paths를 structured format으로 정렬 |
| PH7-03 transport logging rebalance | done | fallback/failure log visibility 조정 및 e2e parsing 보강 |
| PH7-04 supervisor signal expansion | done | `SIGUSR1` state dump, `SIGHUP` no-op 처리 추가 |

### PH7-01: 하드코딩된 runtime 값 외부화
- **대상**: `internal/config/`, `internal/transport/uds.go`, `internal/agent/heartbeat.go`, `internal/transport/fallback.go`, `internal/cli/up.go`
- **완료 상태**: `RuntimeConfig`와 env override를 도입하고 timeout / retry / interval 값을 실제 runtime wiring에 연결
- **검증**: `TestLoadRuntimeUsesDefaults`, `TestLoadRuntimeUsesEnvOverrides`, full suite, invalid env manual QA
- **실소요 공수**: 약 3h

### PH7-02: 구조화 user error 패턴 확대 적용
- **대상**: `internal/cli/task.go`, `internal/cli/register.go`, 기존 `internal/cli/errors.go` surface
- **완료 상태**: 대표 user-facing 입력 오류(unknown assignee, invalid register name)를 `Error/Reason/Hint` 형식으로 정렬
- **검증**: `TestTaskCreateUsesStructuredErrorForUnknownAssignee`, `TestRegisterUsesStructuredErrorForInvalidName`, manual CLI QA
- **실소요 공수**: 약 2h

### PH7-03: transport/message 로그 레벨 재조정
- **대상**: `internal/message/router.go`, `cmd/agentcom/e2e_test.go`
- **완료 상태**: fallback은 INFO, transport send failure와 broadcast failure는 WARN으로 조정했고 combined output parser를 보강해 JSON CLI 회귀를 방지
- **검증**: `go test ./cmd/agentcom -count=1`, full suite
- **실소요 공수**: 약 1h

### PH7-04: supervisor 시그널 핸들링 확장
- **대상**: `internal/cli/up.go`, `internal/cli/up_test.go`
- **완료 상태**: `SIGUSR1`에서 runtime state dump, `SIGHUP`은 no-op로 수용
- **검증**: `TestHandleSupervisorSignal*`, `kill -USR1 <supervisor_pid>` manual QA
- **실소요 공수**: 약 1h

**PH7 예상 잔여 공수: 0h**

---

## PH8: MCP Surface Rebaseline

> 우선순위: Medium
> 목표: CLI-first 원칙에 맞춰 정말 필요한 parity gap만 메운다.

### PH8 상태

| 태스크 | 상태 | 메모 |
|-------|------|------|
| 기존 PH8-02 `update_task` | done | 이미 구현됨 |
| 기존 계획 외 `approve_task` / `reject_task` | done | 이미 구현됨 |
| 기존 계획 외 `send_to_user` / `get_user_messages` | done | 이미 구현됨 |
| inbox / health / deregister / doctor / version / user_reply | open | 남은 parity gap |

### PH8-01: `inbox` MCP 도구 추가
- CLI 대응: `agentcom inbox`
- **예상 공수**: 2h

### PH8-02: `health` MCP 도구 추가
- CLI 대응: `agentcom health`
- **예상 공수**: 1h

### PH8-03: `deregister` MCP 도구 추가
- CLI 대응: `agentcom deregister`
- **예상 공수**: 1h

### PH8-04: `doctor` MCP 도구 추가
- CLI 대응: `agentcom doctor`
- **예상 공수**: 1.5h

### PH8-05: `version` MCP 도구 추가
- CLI 대응: `agentcom version`
- **예상 공수**: 30min

### PH8-06: `user_reply` MCP 도구 추가
- CLI 대응: `agentcom user reply`
- **예상 공수**: 1.5h

### PH8 범위 규칙

- 새 MCP 도구는 CLI에 이미 존재하고 셸 없는 런타임에서 실제 가치가 있는 것만 추가한다.
- `list_tasks`, `get_status`, `update_task`, `approve_task`, `reject_task`, `send_to_user`, `get_user_messages`는 완료 항목으로 취급한다.

**PH8 예상 잔여 공수: 7.5h**

---

## PH9: Targeted Test Closure

> 우선순위: Medium
> 목표: 변경 예정 영역과 현재 테스트 공백을 직접 메운다.

### PH9 상태

| 태스크 | 상태 | 메모 |
|-------|------|------|
| onboard package tests | partial | wizard/result는 있으나 huh/prompter/template_definition 전용 테스트 부족 |
| task/query tests | open | `internal/task/query.go` 전용 테스트 없음 |
| transport lifecycle tests | open | roundtrip/stale socket/poller는 있으나 lifecycle 부족 |
| MCP error tests | partial | 일부 invalid params만 있으나 full matrix 부족 |

### PH9-01: onboard 보강 테스트
- **대상**: `internal/onboard/huh_prompter.go`, `internal/onboard/prompter.go`, `internal/onboard/template_definition.go`
- **현재 상태**: `wizard_test.go`, `result_test.go`만 존재
- **예상 공수**: 2h

### PH9-02: `internal/task/query.go` 전용 테스트 추가
- **대상**: `ListAll`, `ListByStatus`, `ListByAssignee`, `FindByID`, `NewQuery`
- **현재 상태**: manager 테스트를 통한 간접 커버만 존재
- **예상 공수**: 2h

### PH9-03: transport / heartbeat lifecycle 테스트 추가
- **대상**: `internal/transport/uds.go`, `internal/transport/fallback.go`, `internal/agent/heartbeat.go`
- **현재 상태**: roundtrip / stale socket / poller delivery까지만 존재
- **예상 공수**: 3h

### PH9-04: MCP 에러 응답 테스트 매트릭스 강화
- **대상**: `internal/mcp/server_test.go`
- **현재 상태**:
  - invalid priority 일부 테스트 존재
  - unknown tool / runtime tool error / empty params / malformed request matrix 부족
- **의존성**: PH5
- **예상 공수**: 2h

**PH9 예상 잔여 공수: 9h**

---

## 권장 실행 순서

### Wave 1 — 즉시 착수 (production-readiness blocker)

| 순서 | 태스크 | 이유 |
|------|--------|------|
| 1 | PH5-01 | MCP 호환성 직접 복구 |
| 2 | PH5-02 | error mapping 일관성 확보 |
| 3 | PH5-03 | task retry/reopen 결함 해소 |
| 4 | PH6-02 | shutdown hang 리스크 제거 |

### Wave 2 — 런타임 안정성 핵심

| 순서 | 태스크 | 이유 |
|------|--------|------|
| 5 | PH6-03 | UDS server hang 방지 |
| 6 | PH6-04 | 전송 안정성 향상 |
| 7 | PH6-06 | stale runtime/socket 정리 |
| 8 | PH6-01 | 자식 프로세스 health monitoring |

### Wave 3 — 운영 보호장치 / 설정

| 순서 | 태스크 | 이유 |
|------|--------|------|
| 9 | PH6-05 | 메시지 폭주 보호 |
| 10 | PH6-07 | 입력 품질 강화 |
| 11 | PH7-01 | runtime tuning 외부화 |
| 12 | PH7-03 | 운영 로그 가시성 확보 |

### Wave 4 — 표면 정리 / 테스트 마감

| 순서 | 태스크 | 이유 |
|------|--------|------|
| 13 | PH7-02 | 사용자-facing 에러 UX 정리 |
| 14 | PH8-01~06 | 남은 MCP parity gap 해소 |
| 15 | PH6-08 | DB health path 보강 |
| 16 | PH7-04 | signal 운영성 보강 |
| 17 | PH9-01~04 | 변경 영역 회귀 방지 |

---

## 병렬 실행 전략

다음 조합이 현재 기준으로 가장 안전하다.

```text
Worktree A: PH5-01 ~ PH5-03
Worktree B: PH6-02 ~ PH6-04
Worktree C: PH6-06 / PH6-01
Worktree D: PH7-01 / PH7-03 (PH6 주요 timeout 값이 정리된 뒤)
Worktree E: PH8 / PH9 (PH5 완료 후)
```

주의 사항:

- PH7-01은 PH6에서 timeout/retry 값이 확정되기 전 너무 일찍 시작하지 않는다.
- PH9-04는 PH5 완료 후 진행한다.
- PH8은 CLI-first 원칙상 “필요한 도구만” 유지하며 범위를 확장하지 않는다.

---

## 총 잔여 규모

| Phase | 남은 태스크 수 | 예상 잔여 공수 |
|-------|----------------|----------------|
| PH5 | 0 | 0h |
| PH6 | 0 | 0h |
| PH7 | 0 | 0h |
| PH8 | 6 | 7.5h |
| PH9 | 4 | 9h |
| **총계** | **10** | **약 16.5h** |

차이 설명:

- 기존 계획의 26개 중 `PH8-02 update_task`는 이미 완료되어 제외했다.
- 대신 기존 계획 밖에서 이미 완료된 MCP review/human communication 도구는 “완료 상태”로만 기록했다.
- 일부 partial 항목은 남은 작업만 다시 정의했기 때문에 체감 난이도는 원 계획보다 약간 낮아질 수 있다.

---

## 완료 기준

- [x] PH5 완료: MCP error path가 모두 JSON-RPC `error`를 사용하고, terminal state reopen/retry가 가능하다.
- [x] PH6 완료: `up/down`, UDS, runtime cleanup 경로에 운영상 치명적인 무기한 블로킹/누수 경로가 없다.
- [x] PH7 완료: timeout/retry/interval 값이 외부화되고 운영 로그/에러 UX가 일관적이다.
- [ ] PH8 완료: 남은 필수 CLI parity MCP 도구가 추가된다.
- [ ] PH9 완료: onboard/query/transport lifecycle/MCP error matrix 테스트가 보강된다.
- [ ] `go test ./...` 통과
- [ ] `go build ./...` 통과
- [ ] 수동 QA: `agentcom up` -> direct send/broadcast -> user flow -> `agentcom down` -> stale recovery 시나리오 확인

# Next Phase Plan — agentcom v2 Hardening & Feature Expansion

> 작성일: 2026-03-17
> 기준 브랜치: `feature/P12-user-endpoint` (P0~P12, PH1~PH4 완료)
> 목적: production-readiness 확보 + 새로운 기능 확장

---

## 개요

P0~P12와 PH1~PH4를 통해 핵심 기능(CLI, MCP, up/down 라이프사이클, user endpoint, 템플릿 시스템, doctor, skill 시스템)이 구현 완료되었다. 이 계획은 **코드베이스 정적 분석 결과**를 바탕으로 production-readiness를 확보하고, 실사용 시나리오에서 필요한 새 기능을 추가한다.

### 분석 결과 요약

| 영역 | 현 상태 | 발견된 문제 수 |
|------|---------|---------------|
| MCP 스펙 준수 | JSON-RPC 2.0 에러 형식 미준수 | 1 (Critical) |
| 프로세스 관리 | 고아 프로세스, 헬스 모니터링 부재 | 3 (High) |
| 에러 처리 | 구조화 에러 7.6% 적용 (353개 중 27개) | 1 (High) |
| 메시지 안정성 | Rate limiting 없음, 큐 오버플로 보호 없음 | 2 (High) |
| 설정 유연성 | 타임아웃 등 7개 값 하드코딩 | 1 (Medium) |
| 테스트 | onboard 패키지 0%, task/query 미테스트 | 4 (Medium) |
| MCP-CLI 패리티 | MCP 9개 도구 / CLI 18개 명령 (50%) | 1 (Medium) |
| 태스크 상태머신 | 터미널 상태 재전이 불가 | 1 (Medium) |
| 트랜스포트 | UDS 타임아웃 부재, 재시도 1회 | 2 (Medium) |
| 에이전트 검증 | 이름 유효성 검사 없음, 소켓 누적 | 2 (Low) |

---

## Phase 구조

기존 Phase 넘버링(P0~P12, PH1~PH4)과 충돌하지 않도록 **PH5~PH9** 체계를 사용한다.

```
PH5: MCP & Protocol Compliance     (필수 — 스펙 위반 수정)
PH6: Reliability & Robustness      (필수 — 프로세스/전송/DB 안정성)
PH7: Configuration & Observability  (권장 — 설정 외부화 + 로깅)
PH8: MCP Feature Parity            (권장 — MCP 도구 확장)
PH9: Test Coverage & Quality       (권장 — 미테스트 영역 보강)
```

Phase 간 의존성:
- PH5 → PH8 (MCP 에러 형식 수정 후 새 도구 추가)
- PH6은 독립 실행 가능
- PH7은 독립 실행 가능
- PH9은 PH5~PH7 완료 후 실행 권장

---

## PH5: MCP & Protocol Compliance

> **우선순위: CRITICAL** — JSON-RPC 2.0 스펙 위반은 MCP 클라이언트 호환성을 깨뜨림

### PH5-01: MCP 에러 응답을 JSON-RPC 2.0 규격으로 전환
- **파일**: `internal/mcp/server.go` (lines 183-197)
- **현상**: 도구 핸들러 에러를 `result` 필드에 `isError: true`로 감싸서 반환 — JSON-RPC 2.0은 `error` 필드를 요구
- **수정**:
  - `newToolResult(..., true)` → `newErrorResponse(req.ID, code, message)` 전환
  - 에러 코드 체계: `-32601` (unknown tool), `-32602` (invalid params), `-32000~-32099` (도구별 에러)
- **검증**: MCP roundtrip 테스트에서 에러 응답 `error` 필드 존재 + `result` 필드 부재 확인
- **예상 공수**: 2h

### PH5-02: MCP 파라미터 유효성 검증 추가
- **파일**: `internal/mcp/handler.go`
- **현상**: `handleListAgents`의 `alive_only` 타입 미검증, `handleBroadcast`의 payload JSON 미검증, `handleCreateTask`의 priority 값 미검증
- **수정**:
  - 각 handler에 `validateParams()` 함수 추가
  - 필수 파라미터 누락 → `-32602 Invalid params` 에러
  - 잘못된 값 → 구체적 에러 메시지
- **검증**: 잘못된 파라미터로 MCP 호출 시 적절한 에러 코드+메시지 반환 테스트
- **예상 공수**: 3h

### PH5-03: 태스크 상태머신 터미널 상태 전이 확장
- **파일**: `internal/task/model.go` (lines 17-39)
- **현상**: `completed`, `failed`, `cancelled` 상태에서 어떤 전이도 불가
- **수정**:
  ```go
  StatusCompleted: {StatusPending: {}, StatusCancelled: {}},
  StatusFailed:    {StatusPending: {}, StatusCancelled: {}},
  StatusCancelled: {StatusPending: {}},
  ```
- **검증**: 터미널→비터미널 전이 테스트 추가 (reopen, retry, resurrect 시나리오)
- **예상 공수**: 1h

**PH5 총 예상 공수: 6h**

---

## PH6: Reliability & Robustness

> **우선순위: HIGH** — 프로세스 누수, 메시지 유실, 무한 행은 production에서 치명적

### PH6-01: Supervisor 자식 프로세스 헬스 모니터링
- **파일**: `internal/cli/up.go` (lines 441-476)
- **현상**: supervisor가 자식 exit만 감지, 행(hang) 프로세스 미감지
- **수정**:
  - 주기적(30s) heartbeat DB 조회로 자식 liveness 확인
  - 3회 연속 heartbeat 미확인 → 프로세스 재시작 또는 강제 종료
  - `--no-restart` 플래그로 자동 재시작 비활성화 옵션
- **검증**: 자식 프로세스 행 시뮬레이션 테스트
- **예상 공수**: 4h

### PH6-02: `context.Background()` 셧다운 경로 수정
- **파일**: `internal/cli/up.go` (lines 416, 615, 626)
- **현상**: `deregisterUserPseudoAgent(context.Background(), ...)`로 무기한 블로킹 가능
- **수정**: `context.WithTimeout(context.Background(), 5*time.Second)` 패턴으로 전환
- **검증**: 셧다운 시 5초 타임아웃 후 강제 종료 확인
- **예상 공수**: 30min

### PH6-03: UDS Accept/Read 타임아웃 추가
- **파일**: `internal/transport/uds.go` (lines 128, 157)
- **현상**: `listener.Accept()`와 `decoder.Decode()`에 타임아웃 없음 → 슬로우 클라이언트가 서버 고루틴 블로킹
- **수정**:
  - `listener.(*net.UnixListener).SetDeadline()` 주기적 갱신 (acceptLoop)
  - `conn.SetReadDeadline()` 설정 (handleConnection)
  - 타임아웃 시 정상 재시도 루프
- **검증**: 슬로우 클라이언트 연결 시 서버가 행되지 않는 테스트
- **예상 공수**: 2h

### PH6-04: UDS 클라이언트 재시도 개선 (exponential backoff + jitter)
- **파일**: `internal/transport/uds.go` (lines 179-200)
- **현상**: 재시도 1회, 즉시 재시도, 백오프/지터 없음
- **수정**:
  - 재시도 3회: 100ms → 200ms → 400ms + ±50ms 지터
  - 최종 실패 시 WARN 레벨 로그
  - 폴백(SQLite) 전환 시 INFO 레벨 로그
- **검증**: 네트워크 지연 시뮬레이션 테스트
- **예상 공수**: 2h

### PH6-05: 메시지 Rate Limiting & 큐 오버플로 보호
- **파일**: `internal/message/router.go`, `internal/db/message.go`
- **현상**: 무제한 메시지 수용, SQLite 무한 증가
- **수정**:
  - 에이전트별 최대 인박스 크기 도입 (기본 10,000)
  - 초과 시 FIFO 자동 정리 (가장 오래된 읽은 메시지부터 삭제)
  - 에이전트별 초당 메시지 제한 (기본 100/s)
  - 브로드캐스트 쓰로틀링 (동일 topic 5초 내 중복 차단)
- **검증**: 제한 초과 시 적절한 에러 메시지 반환 테스트
- **예상 공수**: 4h

### PH6-06: 고아 프로세스 / 소켓 파일 정리
- **파일**: `internal/agent/registry.go`, `internal/cli/up.go`
- **현상**:
  - `MarkInactive()` 시 소켓 파일 미삭제
  - supervisor 크래시 후 런타임 상태 파일과 실제 프로세스 불일치
- **수정**:
  - `MarkInactive()` 내에서 `os.Remove(agent.SocketPath)` 추가
  - `agentcom up` 시작 시 stale runtime state 감지 → 자동 정리 또는 경고
  - `agentcom down --cleanup` 플래그: stale PID/소켓 강제 정리
- **검증**: 강제 kill 후 `up` 재실행 시 정상 동작 테스트
- **예상 공수**: 3h

### PH6-07: 에이전트 이름 유효성 검증
- **파일**: `internal/agent/registry.go` (line 41)
- **현상**: 빈 문자열, 특수문자, 예약어("user") 허용
- **수정**:
  - `^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$` 패턴 검증
  - "user" 예약어 차단
  - CLI `register`와 MCP `send_message`에서 동일 검증 적용
- **검증**: 유효/무효 이름 테이블 드리븐 테스트
- **예상 공수**: 1h

### PH6-08: SQLite 런타임 헬스 체크
- **파일**: `internal/db/sqlite.go`
- **현상**: 연결 시점에만 Ping, 이후 헬스 체크 없음
- **수정**:
  - `PRAGMA integrity_check` 주기적 실행 (5분 간격, 선택적)
  - WAL 모드 사후 검증: `PRAGMA journal_mode` 확인
  - 연결 풀 health check 콜백 등록
- **검증**: DB 파일 손상 시뮬레이션 후 에러 감지 테스트
- **예상 공수**: 2h

**PH6 총 예상 공수: 18.5h**

---

## PH7: Configuration & Observability

> **우선순위: MEDIUM** — 운영 환경 유연성 + 디버깅 용이성

### PH7-01: 하드코딩 값 설정 외부화
- **대상 값**:
  | 값 | 현재 위치 | 기본값 | 환경변수 |
  |---|---|---|---|
  | SQLite busy timeout | `db/sqlite.go:14` | 5000ms | `AGENTCOM_SQLITE_BUSY_TIMEOUT` |
  | UDS dial timeout | `transport/uds.go:18` | 5s | `AGENTCOM_UDS_DIAL_TIMEOUT` |
  | UDS write timeout | `transport/uds.go:19` | 5s | `AGENTCOM_UDS_WRITE_TIMEOUT` |
  | Stale dial timeout | `transport/uds.go:20` | 1s | `AGENTCOM_STALE_DIAL_TIMEOUT` |
  | Heartbeat interval | `agent/heartbeat.go:11` | 10s | `AGENTCOM_HEARTBEAT_INTERVAL` |
  | Stale threshold | `agent/registry.go:19` | 30s | `AGENTCOM_HEARTBEAT_STALE` |
  | Poller interval | `transport/fallback.go:25` | 5s | `AGENTCOM_POLLER_INTERVAL` |
  | Supervisor startup timeout | `cli/up.go:382` | 5s | `AGENTCOM_SUPERVISOR_TIMEOUT` |
  | Graceful shutdown timeout | `cli/up.go:170` | 10s | `AGENTCOM_SHUTDOWN_TIMEOUT` |
- **수정**: `internal/config/` 패키지에 `RuntimeConfig` 구조체 추가, 환경변수 → config 파일 → 기본값 우선순위 체인
- **검증**: 환경변수 오버라이드 테스트
- **예상 공수**: 4h

### PH7-02: 구조화 에러 패턴 CLI 전체 적용
- **현상**: `errors.go`의 What/Why/How 패턴이 `agents.go`에서만 사용 (7.6% 적용률)
- **대상**: `init.go`, `up.go`, `task.go`, `skill.go`, `register.go`, `root.go` 등 사용자-대면 에러 경로
- **수정**:
  - 사용자가 직접 보는 에러(cobra RunE 반환값)만 구조화 에러로 전환
  - 내부 로직 에러는 기존 `fmt.Errorf` 유지
  - 대상: ~50개 사용자-대면 에러 경로
- **검증**: 대표 에러 시나리오별 What/Why/How 출력 확인 테스트
- **예상 공수**: 6h

### PH7-03: 메시지 전송 로그 레벨 조정
- **파일**: `internal/message/router.go`, `internal/transport/uds.go`
- **현상**: UDS 실패/폴백 전환이 DEBUG 레벨 — production에서 보이지 않음
- **수정**:
  - UDS 전송 최종 실패 → WARN
  - SQLite 폴백 전환 → INFO
  - 성공적 직접 전송 → DEBUG (유지)
- **예상 공수**: 1h

### PH7-04: Supervisor 시그널 핸들링 확장
- **파일**: `internal/cli/up.go` (lines 183-184)
- **현상**: SIGINT/SIGTERM만 처리, SIGHUP(설정 리로드) 미처리
- **수정**:
  - `SIGHUP` 핸들러 추가: 설정 리로드 (PH7-01 완료 후)
  - `SIGUSR1` 핸들러: 상태 덤프 (현재 자식 프로세스 상태를 로그에 출력)
- **의존성**: PH7-01
- **예상 공수**: 2h

**PH7 총 예상 공수: 13h**

---

## PH8: MCP Feature Parity

> **우선순위: MEDIUM** — MCP가 주 통합점이라면 CLI 기능의 대부분을 노출해야 함

### PH8-01: `inbox` MCP 도구 추가
- 기능: 에이전트별 인박스 조회 (unread 필터, from 필터)
- CLI 대응: `agentcom inbox --agent <name> [--unread] [--from <id>]`
- **예상 공수**: 2h

### PH8-02: `update_task` MCP 도구 추가
- 기능: 태스크 상태/결과 업데이트
- CLI 대응: `agentcom task update <id> --status <status> --result <result>`
- **예상 공수**: 1.5h

### PH8-03: `health` MCP 도구 추가
- 기능: 등록된 에이전트 헬스 조회
- CLI 대응: `agentcom health`
- **예상 공수**: 1h

### PH8-04: `deregister` MCP 도구 추가
- 기능: 에이전트 등록 해제
- CLI 대응: `agentcom deregister <name>`
- **예상 공수**: 1h

### PH8-05: `doctor` MCP 도구 추가
- 기능: 환경 진단 결과 반환
- CLI 대응: `agentcom doctor`
- **예상 공수**: 1.5h

### PH8-06: `version` MCP 도구 추가
- 기능: 빌드 메타데이터 반환
- CLI 대응: `agentcom version`
- **예상 공수**: 30min

### PH8-07: `user_reply` MCP 도구 추가
- 기능: 사용자가 에이전트에게 응답 (MCP 클라이언트에서 직접 응답)
- CLI 대응: `agentcom user reply <agent> <payload>`
- **예상 공수**: 1.5h

**PH8 총 예상 공수: 9h**

---

## PH9: Test Coverage & Quality

> **우선순위: MEDIUM** — 미테스트 영역은 리팩토링/확장 시 회귀 위험

### PH9-01: onboard 패키지 테스트 추가
- **대상**: `huh_prompter.go` (2개 공개 함수), `prompter.go` (인터페이스), `template_definition.go`
- **방법**: mock prompter로 wizard flow 테스트, 결과값 검증
- **예상 공수**: 3h

### PH9-02: task/query 함수 테스트 추가
- **대상**: `ListAll`, `ListByStatus`, `ListByAssignee`, `FindByID`, `NewQuery`
- **방법**: in-memory SQLite + 테이블 드리븐 테스트
- **예상 공수**: 2h

### PH9-03: transport 고루틴 라이프사이클 테스트
- **대상**: `heartbeat.go` Start/Stop, `fallback.go` Start/Stop, `uds.go` acceptLoop 종료
- **방법**: context 취소 후 고루틴 정리 확인, 리소스 누수 검증
- **예상 공수**: 3h

### PH9-04: MCP 에러 응답 테스트 강화
- **대상**: PH5-01/PH5-02에서 수정된 에러 경로
- **방법**: 잘못된 파라미터, 존재하지 않는 도구, 빈 요청 등 에지 케이스
- **의존성**: PH5
- **예상 공수**: 2h

**PH9 총 예상 공수: 10h**

---

## 실행 우선순위 및 일정

### 즉시 실행 (Blocking — production 사용 전 필수)

| 순서 | 태스크 | Phase | 공수 | 의존성 |
|------|--------|-------|------|--------|
| 1 | MCP 에러 응답 JSON-RPC 전환 | PH5-01 | 2h | 없음 |
| 2 | context.Background() 셧다운 수정 | PH6-02 | 30min | 없음 |
| 3 | MCP 파라미터 검증 | PH5-02 | 3h | PH5-01 |
| 4 | 태스크 상태머신 확장 | PH5-03 | 1h | 없음 |

### 높은 우선순위 (1주 내)

| 순서 | 태스크 | Phase | 공수 | 의존성 |
|------|--------|-------|------|--------|
| 5 | UDS Accept/Read 타임아웃 | PH6-03 | 2h | 없음 |
| 6 | UDS 재시도 개선 | PH6-04 | 2h | 없음 |
| 7 | Rate limiting & 오버플로 보호 | PH6-05 | 4h | 없음 |
| 8 | 고아 프로세스/소켓 정리 | PH6-06 | 3h | 없음 |
| 9 | Supervisor 헬스 모니터링 | PH6-01 | 4h | PH6-06 |

### 중간 우선순위 (2주 내)

| 순서 | 태스크 | Phase | 공수 | 의존성 |
|------|--------|-------|------|--------|
| 10 | 하드코딩 값 설정 외부화 | PH7-01 | 4h | 없음 |
| 11 | 구조화 에러 전체 적용 | PH7-02 | 6h | 없음 |
| 12 | 로그 레벨 조정 | PH7-03 | 1h | 없음 |
| 13 | 에이전트 이름 검증 | PH6-07 | 1h | 없음 |
| 14 | SQLite 헬스 체크 | PH6-08 | 2h | PH7-01 |
| 15 | MCP 도구 확장 (7개) | PH8-01~07 | 9h | PH5-01 |

### 낮은 우선순위 (릴리스 전)

| 순서 | 태스크 | Phase | 공수 | 의존성 |
|------|--------|-------|------|--------|
| 16 | Supervisor 시그널 확장 | PH7-04 | 2h | PH7-01 |
| 17 | onboard 테스트 | PH9-01 | 3h | 없음 |
| 18 | task/query 테스트 | PH9-02 | 2h | PH5-03 |
| 19 | transport 고루틴 테스트 | PH9-03 | 3h | PH6-03 |
| 20 | MCP 에러 테스트 강화 | PH9-04 | 2h | PH5 |

---

## 총 규모

| Phase | 태스크 수 | 예상 공수 |
|-------|-----------|-----------|
| PH5: MCP & Protocol | 3 | 6h |
| PH6: Reliability | 8 | 18.5h |
| PH7: Config & Observability | 4 | 13h |
| PH8: MCP Feature Parity | 7 | 9h |
| PH9: Test Coverage | 4 | 10h |
| **총계** | **26** | **56.5h** |

---

## 병렬 실행 가능성

독립 워크트리에서 병렬 작업 가능한 조합:

```
워크트리 A: PH5 (MCP 스펙) → PH8 (MCP 확장)
워크트리 B: PH6 (안정성) — 독립
워크트리 C: PH7 (설정/옵저버빌리티) — 독립
워크트리 D: PH9 (테스트) — PH5/PH6 완료 후
```

---

## 새 기능 아이디어 (범위 외 — 향후 검토)

이 계획 범위에는 포함하지 않지만, 향후 확장 시 검토할 기능들:

1. **메시지 구독 (pub/sub)**: 토픽 기반 구독으로 폴링 없이 메시지 수신
2. **웹 대시보드**: 브라우저에서 에이전트 상태/메시지/태스크 모니터링
3. **원격 에이전트 지원**: UDS → TCP 확장으로 다중 머신 통신
4. **메시지 TTL & 자동 아카이브**: 오래된 메시지 자동 정리/아카이브
5. **플러그인 시스템**: 커스텀 도구/핸들러 등록 메커니즘
6. **태스크 의존성 그래프**: `blocked_by` 기반 자동 스케줄링
7. **에이전트 능력 기반 라우팅**: capability 매칭으로 자동 태스크 배정
8. **감사 로그**: 모든 상태 변경 이력 추적
9. **goreleaser 자동화**: release workflow에 arm64 + Homebrew/Scoop 자동 갱신 통합
10. **MCP SSE 전송**: STDIO 외 SSE(Server-Sent Events) 전송 지원

---

## 커밋 컨벤션

```
{type}({scope}): PH{N}-{NN} {설명}

fix(mcp): PH5-01 JSON-RPC 2.0 규격 에러 응답으로 전환
feat(transport): PH6-03 UDS accept/read 타임아웃 추가
refactor(config): PH7-01 하드코딩 타임아웃 설정 외부화
feat(mcp): PH8-01 inbox MCP 도구 추가
test(task): PH9-02 query 함수 테이블 드리븐 테스트 추가
```

---

## 완료 기준

이 계획의 전체 완료 기준:

- [ ] PH5 전체 완료: MCP JSON-RPC 2.0 스펙 준수, 파라미터 검증, 상태머신 확장
- [ ] PH6 전체 완료: 프로세스 헬스 모니터링, 타임아웃, rate limiting, 고아 정리, 이름 검증, DB 헬스
- [ ] PH7 전체 완료: 설정 외부화, 구조화 에러, 로그 레벨, 시그널 핸들링
- [ ] PH8 전체 완료: 7개 MCP 도구 추가 (CLI 기능의 90%+ MCP 노출)
- [ ] PH9 전체 완료: onboard, task/query, transport, MCP 에러 테스트
- [ ] `go test ./...` 전체 통과
- [ ] `go build ./...` 클린 빌드
- [ ] 수동 QA: `agentcom up` → 메시지 교환 → rate limit 테스트 → `agentcom down` 시나리오

# MEMORY.md — 작업 기억

> 세션 간 컨텍스트를 유지하기 위한 파일. 작업 시작/종료 시 반드시 읽고 업데이트한다.

## 현재 상태

- **Phase**: 7 완료
- **마지막 작업**: Wave 8 테스트 추가 + Wave 9 E2E/README/release/CI 마무리
- **다음 작업**: 커밋 또는 후속 polish가 필요하면 현재 상태 기준으로 진행

## 완료된 태스크

- P0-01~P0-05 완료
- P1-01~P1-09 완료
- P2-01~P2-09 완료
- P3-01~P3-11 완료
- P4-01~P4-09 완료
- P5-01~P5-03 완료
- P6-01~P6-12 완료
- P7-01~P7-04 완료

## 이번 세션에서 마무리한 작업

- `internal/db/agent.go`: `InsertAgent`가 preset ID를 덮어쓰지 않도록 수정
- `internal/db/*_test.go`: DB CRUD 테스트 추가
- `internal/agent/registry_test.go`: register/deregister/heartbeat/stale detection 테스트 추가
- `internal/transport/transport_test.go`: UDS roundtrip, stale socket, fallback poller 테스트 추가
- `internal/task/manager_test.go`: 상태 전이와 manager/query 테스트 추가
- `internal/mcp/server_test.go`: STDIO JSON-RPC handshake + tool roundtrip 테스트 추가
- `cmd/agentcom/e2e_test.go`: 실제 바이너리 기반 E2E 시나리오 추가
- `internal/cli/init.go`: `agentcom init --agents-md` 지원 추가
- `README.md`: 설치, 퀵스타트, CLI/MCP 사용법, 아키텍처 문서화
- `.goreleaser.yml`: 릴리스 설정 추가
- `.github/workflows/ci.yml`: lint/test/build CI 추가

## 설계 결정 로그

| 날짜 | 결정 | 이유 |
|------|------|------|
| 2026-03-12 | SQLite3를 유일한 외부 의존성으로 채택 | 외부 데몬 불필요, WAL 모드로 동시성 확보, 단일 파일 관리 |
| 2026-03-12 | 에이전트 type을 자유 문자열로 | 미래 에이전트 도구에 수정 없이 대응, 유연성 극대화 |
| 2026-03-12 | MessagePack 대신 JSON 직렬화 | 디버깅 편의성 우선, 성능 차이 무시할 수준 (로컬 IPC) |
| 2026-03-12 | CGO 사용 (mattn/go-sqlite3) | modernc.org/sqlite도 고려했으나 mattn이 더 성숙하고 WAL 지원 안정적 |
| 2026-03-12 | message/task 테이블의 agent foreign key 제거 | agent deregister 이후에도 message/task history를 유지하고 E2E 흐름을 막지 않기 위해 |

## 발견된 이슈

- 기존 메모의 PRD 경로 표기는 `.agents/plan/PRD.md`였지만 실제 경로는 `.agents/plans/PRD.md`
- agent 삭제 시 message/task 외래 키가 deregister를 막는 문제를 확인했고, 초기 스키마에서 제약 제거로 수정 완료

## 메모

- PRD 경로: `.agents/plans/PRD.md`
- 전체 태스크 수: 62개
- root 커맨드에 `mcp-server` 등록 완료
- 전체 테스트 통과: `go test ./...`
- 전체 빌드 통과: `go build ./...`

# AGENTS.md — agentcom 프로젝트

## 프로젝트 개요

agentcom은 병렬 AI 코딩 에이전트 세션 간 실시간 커뮤니케이션을 위한 Go CLI 도구다.
핵심 의존성은 SQLite3(WAL 모드)이며, Unix Domain Socket으로 실시간 전송한다.

## 기술 스택

- Go 1.22+ (CGO 활성화 — mattn/go-sqlite3)
- SQLite3 WAL 모드 (유일한 외부 의존성)
- cobra (CLI), bubbletea + lipgloss (TUI, v0.2)
- JSON-RPC 2.0 over STDIO (MCP 서버)

## 프로젝트 구조

```
agentcom/
├── cmd/agentcom/main.go         # 엔트리포인트
├── internal/
│   ├── cli/                     # cobra 명령 (init, register, send, task 등)
│   ├── db/                      # SQLite 연결, 마이그레이션, CRUD
│   ├── transport/               # UDS 서버/클라이언트, 폴백
│   ├── agent/                   # 레지스트리, 하트비트
│   ├── message/                 # 엔벨로프, 라우터, 인박스
│   ├── task/                    # 상태 머신, 매니저
│   ├── mcp/                     # MCP JSON-RPC 서버
│   └── config/                  # 경로, 환경변수
├── Makefile
├── AGENTS.md
├── .agents/
│   ├── plan/PRD.md              # 전체 PRD 및 태스크 분해
│   ├── MEMORY.md                # 작업 기억: 결정사항, 이슈, 컨텍스트
│   └── skills/golang.md         # Go 코딩 컨벤션 및 패턴
├── go.mod
└── go.sum
```

## .agents 폴더 규칙

### .agents/plan/
PRD.md에 전체 태스크가 Phase별로 분해되어 있다. 작업 시작 전 반드시 읽는다.
태스크 ID(P0-01 ~ P7-04)를 커밋 메시지와 브랜치명에 사용한다.

### .agents/MEMORY.md
세션 간 유지해야 할 정보를 기록한다:
- 내린 설계 결정과 이유 (예: "MessagePack 대신 JSON 선택 — 디버깅 우선")
- 발견한 이슈와 해결 방법
- 다음 세션에서 이어야 할 작업 컨텍스트
- 현재 Phase와 완료/미완료 태스크 목록

작업 시작 시 반드시 읽고, 작업 종료 시 반드시 업데이트한다.

### .agents/skills/
golang.md에 이 프로젝트의 Go 코딩 규칙이 정의되어 있다:
- 에러는 `fmt.Errorf("패키지.함수: %w", err)` 형식으로 래핑
- 테스트는 테이블 기반 (Table-Driven Tests)
- 인터페이스는 사용하는 쪽에서 정의 (consumer-side)
- context.Context는 첫 번째 파라미터
- 공개 함수에 GoDoc 주석 필수

## Git-Flow 워크플로우

### 브랜치 전략

```
main              ← 릴리스 가능 상태만
└── develop       ← 통합 브랜치, feature가 머지되는 곳
    ├── feature/P1-01-sqlite-connection
    ├── feature/P2-01-agent-registry
    └── feature/P3-01-uds-server
```

- `main`: 릴리스 태그만 존재. develop에서 머지.
- `develop`: 모든 feature의 통합점. CI 통과 필수.
- `feature/{태스크ID}-{설명}`: 개별 태스크 단위 브랜치.

### Worktree 기반 병렬 작업

각 에이전트(또는 병렬 세션)는 독립 worktree에서 작업한다:

```bash
# worktree 생성 (develop에서 분기)
git worktree add ../agentcom-P1-01 -b feature/P1-01-sqlite-connection develop

# 작업 완료 후
cd ../agentcom-P1-01
git add -A && git commit -m "feat(db): P1-01 SQLite 연결 관리자 구현"
git push origin feature/P1-01-sqlite-connection

# develop에 머지
cd ../agentcom  # 메인 worktree
git checkout develop
git merge --no-ff feature/P1-01-sqlite-connection
git branch -d feature/P1-01-sqlite-connection

# worktree 정리
git worktree remove ../agentcom-P1-01
```

### 커밋 컨벤션

```
{type}({scope}): {태스크ID} {설명}

feat(db): P1-01 SQLite 연결 관리자 구현
feat(agent): P2-01 에이전트 등록 로직 구현
fix(transport): P3-02 UDS 타임아웃 미처리 버그 수정
test(db): P1-09 데이터 레이어 단위 테스트 추가
docs: P7-02 README 작성
```

type: feat, fix, test, refactor, docs, chore

## 작업 프로토콜

### 1. 작업 시작

```
1. .agents/MEMORY.md 읽기 — 이전 컨텍스트 파악
2. .agents/plan/PRD.md 읽기 — 현재 태스크 확인
3. worktree 생성 + feature 브랜치 체크아웃
4. 작업 시작
```

### 2. 작업 중

```
- 하나의 feature 브랜치에서 하나의 태스크(또는 밀접 관련 태스크)만 작업
- 의존성이 있는 태스크는 순서대로 (PRD의 의존성 컬럼 참조)
- 테스트 작성 후 구현 (가능하면 TDD)
- `go test ./...` 통과 확인 후 커밋
```

### 3. 작업 완료

```
1. 모든 테스트 통과 확인: go test ./...
2. lint 통과: golangci-lint run
3. 커밋 + 푸시
4. develop에 머지 (--no-ff)
5. worktree 정리
6. .agents/MEMORY.md 업데이트 — 완료 태스크, 결정사항, 다음 작업
```

## Phase별 작업 순서

```
Phase 0: 부트스트랩       → P0-01 ~ P0-05 (프로젝트 초기화, CI)
Phase 1: SQLite 레이어    → P1-01 ~ P1-09 (DB, 스키마, CRUD, 테스트)
Phase 2: 에이전트 레지스트리 → P2-01 ~ P2-09 (등록, 하트비트, CLI)
Phase 3: 메시지 전송       → P3-01 ~ P3-11 (UDS, 라우팅, 폴백, CLI)
Phase 4: 태스크 관리       → P4-01 ~ P4-09 (상태머신, 위임, CLI)
Phase 5: 모니터링          → P5-01 ~ P5-03 (status, health, version)
Phase 6: MCP 서버          → P6-01 ~ P6-12 (JSON-RPC, 도구, 테스트)
Phase 7: 통합/릴리스       → P7-01 ~ P7-04 (E2E, README, 릴리스)
```

Phase 내 태스크는 의존성 순서를 따른다. Phase 간은 순차 진행이 원칙이되,
독립적인 Phase(예: P4와 P5)는 별도 worktree에서 병렬 가능하다.

## 핵심 설계 원칙

1. **SQLite가 유일한 진실의 원천**: 모든 상태는 SQLite에 영속화. 크래시 후 복구 가능.
2. **에이전트 유형 자유**: type 필드는 자유 문자열. enum 제한 없음.
3. **CLI-first**: CLI가 주 인터페이스. MCP 서버는 셸 없는 런타임(IDE 플러그인 등)용 선택적 어댑터.
4. **실패에 안전**: UDS 실패 → SQLite 폴백. 하트비트 없음 → dead 마킹.
5. **테스트 필수**: 핵심 로직 80%+ 커버리지. DB 테스트는 in-memory SQLite.

## 빌드 & 실행

```bash
make build          # → ./bin/agentcom
make test           # go test ./...
make lint           # golangci-lint run
make install        # go install ./cmd/agentcom
```

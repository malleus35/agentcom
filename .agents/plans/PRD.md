# agentcom PRD (Product Requirements Document)

> 병렬 AI 코딩 에이전트 세션 간 커뮤니케이션을 위한 CLI 도구

## 개요

agentcom은 터미널 기반 AI 코딩 에이전트(Claude Code, Codex CLI, OpenCode, Aider 등)가 별도 세션에서 실시간으로 소통할 수 있게 해주는 에이전트 불문(agent-agnostic) CLI 도구다. 에이전트 유형은 사전 정의되지 않으며 사용자가 자유롭게 등록할 수 있다.

## 기술 스택

- **언어**: Go 1.22+
- **핵심 의존성**: SQLite3 (mattn/go-sqlite3, CGO)
- **CLI 프레임워크**: cobra
- **IPC**: Unix Domain Socket (net 표준 라이브러리)
- **직렬화**: JSON (encoding/json 표준)
- **TUI**: bubbletea + lipgloss (v0.2)
- **MCP**: JSON-RPC 2.0 over STDIO

## 아키텍처 결정

### SQLite3를 유일한 외부 의존성으로

모든 상태(에이전트 레지스트리, 메시지, 태스크, 잠금)를 단일 SQLite 파일로 관리한다. WAL 모드를 기본으로 활성화하여 동시 읽기/쓰기를 지원하며, busy_timeout=5000ms로 경합을 완화한다. 별도 데몬이나 메시지 브로커 없이 SQLite가 메시지 영속화와 상태 조율을 모두 담당한다.

### 에이전트 유형의 자유 등록

에이전트 유형을 enum으로 제한하지 않는다. `--type` 플래그에 임의의 문자열을 허용한다. claude-code, codex, aider, opencode는 물론 custom-bot, my-agent 등 어떤 이름이든 등록 가능하다. 이를 통해 미래에 등장할 새로운 에이전트 도구에도 수정 없이 대응한다.

### 전송 레이어 이중화

주 전송: Unix Domain Socket (`~/.agentcom/sockets/{agent-id}.sock`)
폴백: SQLite 폴링 (UDS 불가 환경 또는 Windows)
에이전트 등록 시 소켓을 생성하고, 메시지 전송 실패 시 자동으로 SQLite 인박스 테이블에 기록하여 수신 에이전트가 폴링으로 수신한다.

---

## 디렉토리 구조

```
agentcom/
├── cmd/                    # CLI 엔트리포인트
│   └── agentcom/
│       └── main.go
├── internal/
│   ├── cli/                # cobra 명령 정의
│   │   ├── root.go
│   │   ├── init.go
│   │   ├── register.go
│   │   ├── deregister.go
│   │   ├── list.go
│   │   ├── send.go
│   │   ├── broadcast.go
│   │   ├── task.go
│   │   ├── status.go
│   │   ├── health.go
│   │   ├── mcp.go
│   │   └── version.go
│   ├── db/                 # SQLite 레이어
│   │   ├── sqlite.go       # 연결, 마이그레이션, PRAGMA
│   │   ├── migrations.go   # 스키마 버전 관리
│   │   ├── agent.go        # 에이전트 CRUD
│   │   ├── message.go      # 메시지 CRUD
│   │   └── task.go         # 태스크 CRUD
│   ├── transport/          # IPC 전송
│   │   ├── uds.go          # Unix Domain Socket 서버/클라이언트
│   │   ├── listener.go     # 수신 메시지 핸들러
│   │   └── fallback.go     # SQLite 폴링 폴백
│   ├── agent/              # 에이전트 도메인 로직
│   │   ├── registry.go     # 등록, 발견, 하트비트
│   │   └── heartbeat.go    # 백그라운드 하트비트 고루틴
│   ├── message/            # 메시지 도메인 로직
│   │   ├── envelope.go     # 메시지 구조체, 직렬화
│   │   ├── router.go       # 포인트투포인트, 브로드캐스트 라우팅
│   │   └── inbox.go        # 인박스 조회, 미수신 메시지
│   ├── task/               # 태스크 도메인 로직
│   │   ├── model.go        # 태스크 구조체, 상태 머신
│   │   ├── manager.go      # 생성, 할당, 상태 전이
│   │   └── query.go        # 목록, 필터, 칸반 뷰
│   ├── mcp/                # MCP 서버
│   │   ├── server.go       # JSON-RPC STDIO 핸들러
│   │   ├── tools.go        # MCP 도구 정의
│   │   └── handler.go      # 도구별 핸들러
│   └── config/             # 설정
│       └── config.go       # 경로, 환경변수, 기본값
├── go.mod
├── go.sum
├── Makefile
├── AGENTS.md
└── .agents/
    ├── plan/
    │   └── PRD.md
    ├── MEMORY.md
    └── skills/
        └── golang.md
```

---

## 태스크 분해

### Phase 0: 프로젝트 부트스트랩

| ID | 태스크 | 설명 | 의존성 |
|----|--------|------|--------|
| P0-01 | Go 모듈 초기화 | `go mod init github.com/{user}/agentcom`, .gitignore 작성 | - |
| P0-02 | Makefile 작성 | build, test, lint, clean, install 타겟 정의 | P0-01 |
| P0-03 | cobra root 명령 설정 | `cmd/agentcom/main.go` + `internal/cli/root.go`, `--json` 글로벌 플래그, 버전 출력 | P0-01 |
| P0-04 | config 패키지 작성 | `~/.agentcom/` 기본 경로, `AGENTCOM_HOME` 환경변수 오버라이드, 디렉토리 자동 생성 | P0-01 |
| P0-05 | CI 설정 | GitHub Actions: lint (golangci-lint), test, build matrix (linux/darwin/windows) | P0-02 |

### Phase 1: SQLite 데이터 레이어

| ID | 태스크 | 설명 | 의존성 |
|----|--------|------|--------|
| P1-01 | SQLite 연결 관리자 | `db/sqlite.go`: Open/Close, WAL 모드, busy_timeout=5000, foreign_keys=ON | P0-04 |
| P1-02 | 마이그레이션 엔진 | `db/migrations.go`: schema_version 테이블, 순차 마이그레이션 실행, 롤백 없는 전진만 | P1-01 |
| P1-03 | agents 테이블 스키마 | id TEXT PK, name TEXT UNIQUE, type TEXT, pid INTEGER, socket_path TEXT, capabilities TEXT (JSON 배열), workdir TEXT, status TEXT, registered_at, last_heartbeat | P1-02 |
| P1-04 | messages 테이블 스키마 | id TEXT PK, from_agent TEXT, to_agent TEXT (NULL=broadcast), type TEXT, topic TEXT, payload TEXT (JSON), correlation_id TEXT, created_at, delivered_at, read_at | P1-02 |
| P1-05 | tasks 테이블 스키마 | id TEXT PK, title TEXT, description TEXT, status TEXT, priority TEXT, assigned_to TEXT, created_by TEXT, blocked_by TEXT (JSON 배열), result TEXT, created_at, updated_at | P1-02 |
| P1-06 | 에이전트 CRUD | `db/agent.go`: Insert, Update, Delete, FindByName, FindByID, ListAll, ListAlive, UpdateHeartbeat | P1-03 |
| P1-07 | 메시지 CRUD | `db/message.go`: Insert, FindByID, ListForAgent, ListUnread, MarkDelivered, MarkRead, ListByCorrelation | P1-04 |
| P1-08 | 태스크 CRUD | `db/task.go`: Insert, Update, FindByID, ListAll, ListByStatus, ListByAssignee, UpdateStatus | P1-05 |
| P1-09 | 데이터 레이어 단위 테스트 | 모든 CRUD 함수에 대한 테이블 기반 테스트, in-memory SQLite 사용 | P1-06~08 |

### Phase 2: 에이전트 레지스트리

| ID | 태스크 | 설명 | 의존성 |
|----|--------|------|--------|
| P2-01 | 에이전트 등록 로직 | `agent/registry.go`: Register(name, type, caps, workdir) → ID 생성 (nanoid), PID 자동 감지, 중복 이름 거부 | P1-06 |
| P2-02 | 에이전트 해제 로직 | Deregister(nameOrID): DB 삭제 + 소켓 파일 정리 | P2-01 |
| P2-03 | 하트비트 고루틴 | `agent/heartbeat.go`: 10초 간격 last_heartbeat 업데이트, 컨텍스트 기반 종료 | P2-01 |
| P2-04 | 비활성 감지 | 30초 이상 하트비트 없으면 status='dead' 설정, `kill -0 <pid>`로 프로세스 생존 교차 확인 | P2-03 |
| P2-05 | `init` CLI 명령 | `cli/init.go`: `~/.agentcom/` 디렉토리 생성, DB 초기화, sockets/ 디렉토리 생성, 성공 메시지 출력 | P1-01 |
| P2-06 | `register` CLI 명령 | `cli/register.go`: --name (필수), --type (필수, 자유 문자열), --cap (쉼표 구분), --workdir (기본=cwd) | P2-01 |
| P2-07 | `deregister` CLI 명령 | `cli/deregister.go`: 이름 또는 ID로 해제, 확인 프롬프트 (--force로 스킵) | P2-02 |
| P2-08 | `list` CLI 명령 | `cli/list.go`: 테이블 형식 (name, type, status, pid, last_heartbeat), --json 지원, --alive 필터 | P2-01 |
| P2-09 | 레지스트리 단위 테스트 | 등록, 중복 거부, 해제, 하트비트 타임아웃, 비활성 감지 | P2-01~04 |

### Phase 3: 메시지 전송 (UDS)

| ID | 태스크 | 설명 | 의존성 |
|----|--------|------|--------|
| P3-01 | UDS 서버 | `transport/uds.go`: 에이전트 소켓 리스닝, 연결 수락, 고루틴당 1 연결, JSON 프레임 수신 | P0-04 |
| P3-02 | UDS 클라이언트 | 대상 에이전트 소켓에 연결, JSON 프레임 전송, 타임아웃(5초), 자동 재시도(1회) | P3-01 |
| P3-03 | 메시지 엔벨로프 | `message/envelope.go`: Message 구조체 (id, from, to, type, topic, payload, correlation_id, timestamp), JSON 직렬화 | - |
| P3-04 | 메시지 라우터 | `message/router.go`: Send(to, msg) → 레지스트리에서 소켓 조회 → UDS 전송 시도 → 실패 시 SQLite 인박스 기록 | P3-02, P1-07 |
| P3-05 | 브로드캐스트 라우터 | Broadcast(msg, topic) → 활성 에이전트 목록 조회 → 각각에 Send, 실패한 것만 인박스 | P3-04 |
| P3-06 | 인박스 폴링 폴백 | `transport/fallback.go`: 5초 간격 미수신 메시지 폴링, 수신 시 delivered_at 업데이트 | P1-07 |
| P3-07 | 수신 메시지 핸들러 | `transport/listener.go`: UDS 수신 시 콜백 실행 (stdout 출력, 향후 확장 포인트) | P3-01 |
| P3-08 | `send` CLI 명령 | `cli/send.go`: agentcom send <target> "message" [--type request|notification] [--topic X] [--correlation-id Y] | P3-04 |
| P3-09 | `broadcast` CLI 명령 | `cli/broadcast.go`: agentcom broadcast "message" [--topic X] | P3-05 |
| P3-10 | `inbox` CLI 명령 | `cli/inbox.go`: agentcom inbox [--unread] [--from X] 수신 메시지 목록 | P1-07 |
| P3-11 | 전송 통합 테스트 | UDS 서버/클라이언트 왕복, 폴백 동작, 타임아웃 | P3-01~07 |

### Phase 4: 태스크 관리

| ID | 태스크 | 설명 | 의존성 |
|----|--------|------|--------|
| P4-01 | 태스크 상태 머신 | `task/model.go`: pending → assigned → in_progress → blocked → completed / failed, 유효 전이만 허용 | - |
| P4-02 | 태스크 생성 로직 | `task/manager.go`: Create(title, desc, priority, assignee, blockers) → ID 생성 | P1-08, P4-01 |
| P4-03 | 태스크 위임 로직 | Delegate(taskID, targetAgent): 상태 assigned로 전이 + 대상 에이전트에 알림 메시지 자동 전송 | P4-02, P3-04 |
| P4-04 | 태스크 상태 업데이트 | UpdateStatus(taskID, newStatus, result): 상태 전이 검증 + updated_at 갱신 | P4-01 |
| P4-05 | `task create` CLI | agentcom task create "제목" [--desc X] [--assign Y] [--priority low|medium|high|critical] [--blocked-by Z] | P4-02 |
| P4-06 | `task list` CLI | agentcom task list [--status X] [--assignee Y] [--json], 기본은 칸반 스타일 테이블 | P1-08 |
| P4-07 | `task update` CLI | agentcom task update <id> --status X [--result "결과 설명"] | P4-04 |
| P4-08 | `task delegate` CLI | agentcom task delegate <id> --to <agent> | P4-03 |
| P4-09 | 태스크 단위 테스트 | 상태 머신 전이, 잘못된 전이 거부, 위임 메시지 전송 | P4-01~04 |

### Phase 5: 모니터링 명령

| ID | 태스크 | 설명 | 의존성 |
|----|--------|------|--------|
| P5-01 | `status` CLI 명령 | agentcom status: 에이전트 수, 활성/비활성, 미수신 메시지 수, 진행 중 태스크 요약 | P2-01, P1-07, P1-08 |
| P5-02 | `health` CLI 명령 | agentcom health: 각 에이전트 생존 확인 (PID + 소켓 + 하트비트), OK/DEAD/STALE 판정 | P2-04 |
| P5-03 | `version` CLI 명령 | agentcom version: 버전, 빌드 날짜, Go 버전, OS/arch, DB 경로 출력 | P0-03 |

### Phase 6: MCP 서버

| ID | 태스크 | 설명 | 의존성 |
|----|--------|------|--------|
| P6-01 | JSON-RPC STDIO 핸들러 | `mcp/server.go`: stdin 읽기, JSON-RPC 파싱, method 라우팅, stdout 응답 | - |
| P6-02 | initialize 핸드셰이크 | MCP initialize/initialized 핸드셰이크 구현, capabilities 선언 (tools) | P6-01 |
| P6-03 | tools/list 구현 | 사용 가능한 도구 목록 반환: list_agents, send_message, broadcast, create_task, list_tasks, delegate_task, get_status | P6-02 |
| P6-04 | list_agents 도구 | 활성 에이전트 목록 반환 (JSON) | P2-01 |
| P6-05 | send_message 도구 | 대상 에이전트에 메시지 전송, 결과 반환 | P3-04 |
| P6-06 | broadcast 도구 | 브로드캐스트 메시지 전송 | P3-05 |
| P6-07 | create_task 도구 | 태스크 생성, ID 반환 | P4-02 |
| P6-08 | delegate_task 도구 | 태스크 위임 | P4-03 |
| P6-09 | list_tasks 도구 | 태스크 목록 (필터 옵션) | P1-08 |
| P6-10 | get_status 도구 | 시스템 상태 요약 | P5-01 |
| P6-11 | `mcp-server` CLI 명령 | agentcom mcp-server: STDIO 모드로 MCP 서버 시작, --register 옵션으로 자신도 에이전트 등록 | P6-01~10 |
| P6-12 | MCP 통합 테스트 | stdin/stdout 파이프로 전체 핸드셰이크 + 도구 호출 라운드트립 | P6-01~10 |

### Phase 7: 통합 및 릴리스

| ID | 태스크 | 설명 | 의존성 |
|----|--------|------|--------|
| P7-01 | E2E 시나리오 테스트 | 2개 에이전트 등록 → 메시지 교환 → 태스크 위임 → 상태 확인 → 해제, 전체 흐름 | P2~P6 |
| P7-02 | README.md 작성 | 설치, 퀵스타트, CLI 레퍼런스, MCP 설정 예시, 아키텍처 다이어그램 | P7-01 |
| P7-03 | goreleaser 설정 | .goreleaser.yml: linux/darwin/windows, amd64/arm64 매트릭스 빌드, Homebrew tap | P0-02 |
| P7-04 | AGENTS.md 템플릿 생성기 | `agentcom init --agents-md`: 프로젝트 루트에 agentcom 사용법이 포함된 AGENTS.md 생성 | P2-05 |

---

## SQLite 스키마 (v1)

```sql
-- 마이그레이션 v1
PRAGMA journal_mode = WAL;
PRAGMA busy_timeout = 5000;
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS agents (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    type        TEXT NOT NULL,           -- 자유 문자열: "claude-code", "my-bot" 등
    pid         INTEGER,
    socket_path TEXT,
    capabilities TEXT DEFAULT '[]',      -- JSON 배열
    workdir     TEXT,
    status      TEXT NOT NULL DEFAULT 'alive',  -- alive | dead
    registered_at TEXT NOT NULL DEFAULT (datetime('now')),
    last_heartbeat TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS messages (
    id              TEXT PRIMARY KEY,
    from_agent      TEXT NOT NULL,
    to_agent        TEXT,                -- NULL = broadcast
    type            TEXT NOT NULL DEFAULT 'notification',  -- request | response | notification
    topic           TEXT,
    payload         TEXT DEFAULT '{}',   -- JSON
    correlation_id  TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    delivered_at    TEXT,
    read_at         TEXT,
    FOREIGN KEY (from_agent) REFERENCES agents(id)
);

CREATE TABLE IF NOT EXISTS tasks (
    id          TEXT PRIMARY KEY,
    title       TEXT NOT NULL,
    description TEXT,
    status      TEXT NOT NULL DEFAULT 'pending',  -- pending|assigned|in_progress|blocked|completed|failed
    priority    TEXT NOT NULL DEFAULT 'medium',    -- low|medium|high|critical
    assigned_to TEXT,
    created_by  TEXT,
    blocked_by  TEXT DEFAULT '[]',                 -- JSON 배열 of task IDs
    result      TEXT,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (assigned_to) REFERENCES agents(id),
    FOREIGN KEY (created_by) REFERENCES agents(id)
);

CREATE INDEX idx_messages_to_agent ON messages(to_agent, delivered_at);
CREATE INDEX idx_messages_correlation ON messages(correlation_id);
CREATE INDEX idx_tasks_status ON tasks(status);
CREATE INDEX idx_tasks_assignee ON tasks(assigned_to);
CREATE INDEX idx_agents_status ON agents(status);
```

---

## 비기능 요구사항

- **시작 시간**: < 50ms (CGO SQLite 포함)
- **메시지 지연**: UDS 경유 < 5ms, 폴백 폴링 < 5초
- **동시 에이전트**: 최소 20개 동시 등록 지원
- **바이너리 크기**: < 20MB (linux/amd64)
- **테스트 커버리지**: 핵심 로직 (db, agent, message, task) 80% 이상

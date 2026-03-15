# P10 PRD — agents 테이블 project 컬럼 추가

## Goal

`agents` 테이블에 `project TEXT NOT NULL DEFAULT ''` 컬럼을 추가하여 프로젝트별 에이전트 격리를 지원한다.
단일 글로벌 DB(`~/.agentcom/agentcom.db`)를 유지하면서 프로젝트 단위 필터링과 크로스 프로젝트 통신을 모두 가능하게 한다.

## 배경

현재 agentcom은 모든 프로젝트의 에이전트를 하나의 DB에 저장하며, `workdir` 컬럼으로만 간접 구분 가능하다.
문제점:
- `name`이 글로벌 UNIQUE → 다른 프로젝트에서 같은 이름(예: `alpha`) 사용 불가
- `agentcom list`가 모든 프로젝트의 에이전트를 섞어서 표시
- `workdir`는 같은 프로젝트 내 worktree별로도 달라질 수 있어 프로젝트 식별자로 부적합

## 확정된 설계 결정

| 항목 | 결정 | 이유 |
|------|------|------|
| project 컬럼 범위 | `agents` 테이블에만 추가 | messages/tasks는 agent JOIN으로 필터 가능. 스키마 변경 최소화 |
| UNIQUE 제약 | `UNIQUE(name)` → `UNIQUE(name, project)` | 다른 프로젝트에서 동일 이름 허용 |
| 기본값 | `DEFAULT ''` (빈 문자열) | 기존 데이터/동작 100% 하위 호환 |
| 프로젝트 등록 | `agentcom init`에 통합 | 별도 서브커맨드 없이 init 한 번으로 완료 |
| 마커 파일 | 프로젝트 루트에 `.agentcom.json` | `{"project":"myapp"}` — 이후 명령에서 자동 읽기 |
| project 해석 우선순위 | `--project` 플래그 > `.agentcom.json` > `''` | 명시적 override 항상 우선 |

## Scope

- `agents` 테이블에 `project` 컬럼 추가 (ALTER TABLE 마이그레이션)
- `UNIQUE(name, project)` 복합 유니크 제약
- `.agentcom.json` 프로젝트 마커 파일 읽기/쓰기
- `agentcom init`에서 프로젝트명 감지/등록/`.agentcom.json` 생성
- 전 CLI 명령에 `--project` 글로벌 플래그 + `--all-projects` 추가
- MCP 도구에 `project` 파라미터 추가
- 기존 동작 100% 하위 호환

## Non-Goals

- `messages`/`tasks` 테이블에 `project` 컬럼 추가
- 프로젝트 삭제/이름 변경 CLI
- 프로젝트별 별도 DB 파일
- `.git` root 기반 자동 감지 (사용자가 명시적으로 지정)

---

## 태스크 분해

### Phase 10-1: DB 스키마 마이그레이션

| ID | 태스크 | 설명 | 의존성 |
|----|--------|------|--------|
| P10-01-01 | 마이그레이션 SQL 작성 | `migrations.go`에 `ALTER TABLE agents ADD COLUMN project TEXT NOT NULL DEFAULT ''` 추가 | - |
| P10-01-02 | 기존 UNIQUE 인덱스 교체 | SQLite는 ALTER로 UNIQUE 변경 불가. 새 `UNIQUE INDEX idx_agents_name_project ON agents(name, project)` 생성. 기존 `agents` 테이블의 `name UNIQUE` 제약은 CREATE TABLE IF NOT EXISTS로 인해 기존 테이블에선 유지되므로, 테이블 재생성 마이그레이션 또는 새 인덱스 + 트리거 전략 결정 필요 | P10-01-01 |
| P10-01-03 | 마이그레이션 버전 관리 도입 | 현재 `CREATE TABLE IF NOT EXISTS`만 사용. `schema_version` 테이블 또는 `user_version` PRAGMA로 마이그레이션 순서 추적. 이미 적용된 마이그레이션 재실행 방지 | P10-01-01 |
| P10-01-04 | 마이그레이션 단위 테스트 | 빈 DB에서 전체 마이그레이션 적용 테스트 + 기존 스키마 DB에서 ALTER 마이그레이션만 적용되는 테스트 | P10-01-01~03 |

#### P10-01-02 서브태스크: UNIQUE 제약 교체 전략

SQLite는 `ALTER TABLE ... DROP CONSTRAINT` / `ALTER TABLE ... ADD CONSTRAINT`를 지원하지 않는다.
기존 `name TEXT NOT NULL UNIQUE` 제약을 `UNIQUE(name, project)`로 변경하려면 테이블 재생성이 필요하다.

| ID | 서브태스크 | 설명 | 의존성 |
|----|-----------|------|--------|
| P10-01-02a | 테이블 재생성 마이그레이션 SQL 작성 | `CREATE TABLE agents_new (... project TEXT NOT NULL DEFAULT '', UNIQUE(name, project))` → `INSERT INTO agents_new SELECT ..., '' FROM agents` → `DROP TABLE agents` → `ALTER TABLE agents_new RENAME TO agents` | P10-01-01 |
| P10-01-02b | 인덱스 재생성 | 테이블 재생성 후 `idx_agents_status` 인덱스 재생성 | P10-01-02a |
| P10-01-02c | 트랜잭션 안전성 확보 | 전체 재생성 과정을 단일 트랜잭션으로 감싸기. 실패 시 원본 테이블 보존 | P10-01-02a |

---

### Phase 10-2: DB 모델 + CRUD 수정

| ID | 태스크 | 설명 | 의존성 |
|----|--------|------|--------|
| P10-02-01 | Agent struct에 Project 필드 추가 | `db/agent.go`: `Agent` struct에 `Project string` 필드 추가 | P10-01 완료 |
| P10-02-02 | InsertAgent 수정 | INSERT 쿼리에 `project` 컬럼 추가. `agent.Project` 값 바인딩 | P10-02-01 |
| P10-02-03 | UpdateAgent 수정 | UPDATE 쿼리의 SET 절에 `project = ?` 추가 | P10-02-01 |
| P10-02-04 | scanAgent 수정 | SELECT 결과에서 `project` 컬럼 스캔 추가 | P10-02-01 |
| P10-02-05 | FindAgentByName → FindAgentByNameAndProject | 기존 `WHERE name = ?`를 `WHERE name = ? AND project = ?`로 변경. 기존 `FindAgentByName` 시그니처는 호환 래퍼로 유지하거나 제거 후 호출부 전부 수정 | P10-02-04 |
| P10-02-06 | ListAllAgents에 project 필터 추가 | `ListAgentsByProject(ctx, project)` 추가 또는 기존 함수에 optional filter 파라미터 추가 | P10-02-04 |
| P10-02-07 | ListAliveAgents에 project 필터 추가 | 위와 동일 패턴 | P10-02-04 |
| P10-02-08 | isUniqueNameViolation 에러 메시지 개선 | UNIQUE(name, project) 위반 시 "agent name already exists in this project" 메시지로 변경 | P10-02-02 |
| P10-02-09 | 기존 DB 쿼리 전수 검사 | `status.go`, `health.go` 등에서 직접 SQL을 실행하는 곳에 project 필터 누락 여부 확인 | P10-02-06 |
| P10-02-10 | DB CRUD 단위 테스트 수정 | 모든 기존 agent 테스트에 project 필드 반영 + 새 테스트 케이스 추가: 같은 name/다른 project 동시 등록 허용, 같은 name/같은 project 거부 | P10-02-01~08 |

---

### Phase 10-3: 프로젝트 마커 파일 (.agentcom.json)

| ID | 태스크 | 설명 | 의존성 |
|----|--------|------|--------|
| P10-03-01 | ProjectConfig 구조체 정의 | `config/project.go` 신규 파일. `ProjectConfig { Project string }` + JSON 태그 | - |
| P10-03-02 | WriteProjectConfig 함수 | 지정 디렉토리에 `.agentcom.json` 작성. 이미 존재하면 에러 반환 (덮어쓰기 방지) | P10-03-01 |
| P10-03-03 | LoadProjectConfig 함수 | 현재 디렉토리부터 상위로 올라가며 `.agentcom.json` 탐색. 없으면 빈 ProjectConfig 반환 | P10-03-01 |
| P10-03-04 | ResolveProject 함수 | 우선순위: `--project` 플래그 > `.agentcom.json`의 project > `""`. 최종 project 문자열 반환 | P10-03-03 |
| P10-03-05 | 프로젝트명 검증 | 허용 문자: 소문자, 숫자, 하이픈, 언더스코어. 빈 문자열 허용 (레거시). 최대 64자 | P10-03-01 |
| P10-03-06 | 단위 테스트 | `.agentcom.json` 읽기/쓰기/탐색/검증 테스트. 파일 없는 경우, 잘못된 JSON, 디렉토리 상위 탐색 | P10-03-01~05 |

---

### Phase 10-4: Agent 레지스트리 수정

| ID | 태스크 | 설명 | 의존성 |
|----|--------|------|--------|
| P10-04-01 | Registry.Register 시그니처 변경 | `Register(ctx, name, agentType, caps, workdir)` → `Register(ctx, name, agentType, caps, workdir, project)` | P10-02 완료 |
| P10-04-02 | Registry.FindByName 시그니처 변경 | `FindByName(ctx, name)` → `FindByName(ctx, name, project)`. project 파라미터 추가 | P10-02-05 |
| P10-04-03 | Registry.Deregister 수정 | name으로 찾을 때 project도 함께 사용 | P10-04-02 |
| P10-04-04 | Registry.ListAll / ListAlive 수정 | project 필터 전달. `""` 이면 전체, 값 있으면 해당 project만 | P10-02-06~07 |
| P10-04-05 | Registry.MarkInactive 수정 | ListAliveAgents가 project 파라미터를 받으면 반영. 또는 전체 alive에 대해 동작 (project 무관) | P10-04-04 |
| P10-04-06 | 레지스트리 단위 테스트 수정 | project 파라미터 추가 반영 + 크로스 프로젝트 시나리오 테스트 | P10-04-01~05 |

---

### Phase 10-5: CLI — 글로벌 플래그 + init 수정

| ID | 태스크 | 설명 | 의존성 |
|----|--------|------|--------|
| P10-05-01 | root.go에 `--project` 글로벌 플래그 추가 | `pf.StringVar(&projectFlag, "project", "", "Project name override")`. appContext에 `project string` 필드 추가 | P10-03-04 |
| P10-05-02 | root.go에 `--all-projects` 글로벌 플래그 추가 | `pf.BoolVar(&allProjects, "all-projects", false, "Show resources from all projects")`. project 필터를 무시하는 모드 | P10-05-01 |
| P10-05-03 | initApp에서 project 해석 | `config.ResolveProject(projectFlag)` 호출 → `app.project`에 저장. 모든 하위 명령이 참조 | P10-05-01, P10-03-04 |
| P10-05-04 | init.go — 프로젝트명 감지 로직 | `agentcom init` 실행 시 현재 디렉토리명을 추천 프로젝트명으로 제안. 사용자 확인 후 `.agentcom.json` 생성 | P10-03-02, P10-03-05 |
| P10-05-05 | init.go — 비대화형 모드 지원 | `agentcom init --project myapp`: 대화형 프롬프트 없이 직접 `.agentcom.json` 생성. `--json` 출력에 `project` 필드 추가 | P10-05-04 |
| P10-05-06 | init.go — 기존 `.agentcom.json` 존재 시 처리 | 이미 `.agentcom.json`이 있으면 "project already configured: myapp" 메시지 + skip. `--force`로 덮어쓰기 허용 | P10-05-04 |
| P10-05-07 | init wizard에 project 단계 추가 | P9에서 확장된 기본 wizard 앞단에 project name 단계 추가. 추천 이름 기본값 제공 | P10-05-04 |
| P10-05-08 | init CLI 단위 테스트 | `.agentcom.json` 생성 확인, 중복 생성 방지, `--project` 플래그 동작, JSON 출력 형식 | P10-05-04~07 |

---

### Phase 10-6: CLI — 기존 명령어 수정

| ID | 태스크 | 설명 | 의존성 |
|----|--------|------|--------|
| P10-06-01 | register.go 수정 | `app.project`를 `registry.Register()`에 전달. JSON 출력에 `project` 필드 추가. 텍스트 출력에도 project 표시 | P10-04-01, P10-05-03 |
| P10-06-02 | deregister.go 수정 | name으로 에이전트 찾을 때 `app.project` 사용. `--all-projects` 시 name만으로 검색 (프로젝트 무관 해제) | P10-04-03, P10-05-02 |
| P10-06-03 | list.go 수정 | `ListAll`/`ListAlive`에 `app.project` 전달. `--all-projects` 시 전체. 테이블/JSON 출력에 `project` 컬럼 추가 | P10-04-04, P10-05-02 |
| P10-06-04 | send.go 수정 | `--from`과 target 에이전트 이름 해석 시 `app.project` 사용. 다른 프로젝트 에이전트에게 보내려면 `--project` override 필요 | P10-04-02, P10-05-03 |
| P10-06-05 | broadcast.go 수정 | broadcast 대상을 `app.project` 내 alive 에이전트로 한정. `--all-projects` 시 전체 alive | P10-04-04, P10-05-02 |
| P10-06-06 | inbox.go 수정 | `--agent` 해석 시 `app.project` 사용 | P10-04-02, P10-05-03 |
| P10-06-07 | task.go — create 수정 | `--creator`, `--assign` 에이전트 해석 시 `app.project` 사용 | P10-04-02, P10-05-03 |
| P10-06-08 | task.go — list 수정 | `--assignee` 해석 시 `app.project` 사용 | P10-04-02, P10-05-03 |
| P10-06-09 | task.go — delegate 수정 | `--to` 에이전트 해석 시 `app.project` 사용 | P10-04-02, P10-05-03 |
| P10-06-10 | status.go 수정 | 집계 쿼리에 project 필터 조건 추가. `--all-projects` 시 전체 집계. JSON/텍스트 출력에 `project` 표시 | P10-02-09, P10-05-02 |
| P10-06-11 | health.go 수정 | 에이전트 목록 조회 시 project 필터 적용 | P10-04-04, P10-05-02 |
| P10-06-12 | CLI 명령어 통합 테스트 | 각 명령이 project 필터를 올바르게 적용하는지 검증. 크로스 프로젝트 시나리오 포함 | P10-06-01~11 |

---

### Phase 10-7: MCP 서버 수정

| ID | 태스크 | 설명 | 의존성 |
|----|--------|------|--------|
| P10-07-01 | MCP Server에 project 컨텍스트 전달 | `NewServer(db, cfg)` → `NewServer(db, cfg, project)` 또는 Server struct에 project 필드 추가 | P10-05-03 |
| P10-07-02 | list_agents 도구에 project 파라미터 추가 | `inputSchema`에 `project` (optional string) 추가. 미지정 시 서버의 기본 project 사용 | P10-07-01 |
| P10-07-03 | send_message 도구에 project 파라미터 추가 | from/to 에이전트 해석 시 project 사용 | P10-07-01 |
| P10-07-04 | broadcast 도구에 project 파라미터 추가 | broadcast 대상 필터에 project 적용 | P10-07-01 |
| P10-07-05 | create_task 도구에 project 파라미터 추가 | creator/assignee 해석 시 project 사용 | P10-07-01 |
| P10-07-06 | delegate_task 도구에 project 파라미터 추가 | target 에이전트 해석 시 project 사용 | P10-07-01 |
| P10-07-07 | list_tasks 도구에 project 파라미터 추가 | assignee 해석 시 project 사용 | P10-07-01 |
| P10-07-08 | get_status 도구에 project 파라미터 추가 | 집계 쿼리에 project 필터 적용 | P10-07-01 |
| P10-07-09 | mcp-server CLI 명령 수정 | `--register` 시 `app.project` 전달 | P10-07-01 |
| P10-07-10 | MCP 단위 테스트 수정 | 모든 도구 호출에 project 파라미터 포함 테스트 + project 미지정 시 기본 동작 테스트 | P10-07-01~09 |

---

### Phase 10-8: 통합 테스트 + 문서

| ID | 태스크 | 설명 | 의존성 |
|----|--------|------|--------|
| P10-08-01 | E2E 시나리오 — 단일 프로젝트 | init(project=myapp) → register alpha → register beta → send → inbox → status → deregister. 전부 동일 project 내 | P10-06 완료 |
| P10-08-02 | E2E 시나리오 — 멀티 프로젝트 격리 | project-A에 alpha 등록, project-B에 alpha 등록 (같은 이름). list --project project-A에 alpha 1개만 표시. list --all-projects에 alpha 2개 표시 | P10-06 완료 |
| P10-08-03 | E2E 시나리오 — 크로스 프로젝트 통신 | project-A의 alpha가 project-B의 beta에게 send (--project override 사용). 메시지 수신 확인 | P10-06-04 |
| P10-08-04 | E2E 시나리오 — 레거시 호환 | `.agentcom.json` 없는 디렉토리에서 기존 명령 모두 정상 동작 (project=''). 기존 DB 마이그레이션 후 정상 동작 | P10-01 완료 |
| P10-08-05 | README.md 업데이트 | project 관련 사용법 추가: init 시 project 설정, --project 플래그, --all-projects, .agentcom.json 설명 | P10-08-01~04 |
| P10-08-06 | AGENTS.md 템플릿 업데이트 | `writeProjectAgentsMD()` 내용에 project 관련 워크플로 추가 | P10-08-05 |
| P10-08-07 | MEMORY.md 업데이트 | 설계 결정 로그에 project 관련 결정 기록. 완료 태스크 목록 갱신 | P10-08-01~06 |

---

## 의존성 그래프

```
P10-01 (스키마 마이그레이션)
  └─ P10-02 (DB 모델 + CRUD)
       ├─ P10-03 (.agentcom.json) ← 독립, P10-01과 병렬 가능
       ├─ P10-04 (에이전트 레지스트리)
       │    └─ P10-05 (CLI 글로벌 + init) ← P10-03 필요
       │         └─ P10-06 (CLI 기존 명령)
       │              └─ P10-07 (MCP 서버)
       │                   └─ P10-08 (통합 테스트 + 문서)
       └─ (P10-03은 P10-02와 독립적으로 진행 가능)
```

병렬 가능 구간:
- **P10-01 + P10-03**: 스키마 마이그레이션과 .agentcom.json 로직은 독립적
- **P10-06 내부**: 각 CLI 명령 수정은 서로 독립적 (단, P10-05 완료 후)
- **P10-07 내부**: 각 MCP 도구 수정은 서로 독립적 (단, P10-07-01 완료 후)

## 스키마 변경 상세

### 현재 스키마 (v1)

```sql
CREATE TABLE agents (
    id             TEXT PRIMARY KEY,
    name           TEXT NOT NULL UNIQUE,
    type           TEXT NOT NULL,
    pid            INTEGER,
    socket_path    TEXT,
    capabilities   TEXT DEFAULT '[]',
    workdir        TEXT,
    status         TEXT NOT NULL DEFAULT 'alive',
    registered_at  TEXT NOT NULL DEFAULT (datetime('now')),
    last_heartbeat TEXT NOT NULL DEFAULT (datetime('now'))
);
```

### 목표 스키마 (v2)

```sql
CREATE TABLE agents (
    id             TEXT PRIMARY KEY,
    name           TEXT NOT NULL,
    type           TEXT NOT NULL,
    pid            INTEGER,
    socket_path    TEXT,
    capabilities   TEXT DEFAULT '[]',
    workdir        TEXT,
    project        TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'alive',
    registered_at  TEXT NOT NULL DEFAULT (datetime('now')),
    last_heartbeat TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(name, project)
);

CREATE INDEX idx_agents_status ON agents(status);
CREATE INDEX idx_agents_project ON agents(project);
```

### 마이그레이션 SQL (테이블 재생성)

```sql
-- Step 1: 새 테이블 생성
CREATE TABLE agents_new (
    id             TEXT PRIMARY KEY,
    name           TEXT NOT NULL,
    type           TEXT NOT NULL,
    pid            INTEGER,
    socket_path    TEXT,
    capabilities   TEXT DEFAULT '[]',
    workdir        TEXT,
    project        TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'alive',
    registered_at  TEXT NOT NULL DEFAULT (datetime('now')),
    last_heartbeat TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(name, project)
);

-- Step 2: 데이터 복사
INSERT INTO agents_new (id, name, type, pid, socket_path, capabilities, workdir, project, status, registered_at, last_heartbeat)
SELECT id, name, type, pid, socket_path, capabilities, workdir, '', status, registered_at, last_heartbeat
FROM agents;

-- Step 3: 기존 테이블 삭제
DROP TABLE agents;

-- Step 4: 이름 변경
ALTER TABLE agents_new RENAME TO agents;

-- Step 5: 인덱스 재생성
CREATE INDEX idx_agents_status ON agents(status);
CREATE INDEX idx_agents_project ON agents(project);
```

## .agentcom.json 파일 형식

```json
{
  "project": "myapp"
}
```

- 위치: 프로젝트 루트 디렉토리 (`.git`이 있는 곳과 동일 레벨 권장)
- 탐색: 현재 디렉토리부터 상위로 올라가며 `.agentcom.json` 검색 (최대 깊이 제한 권장: 10레벨)
- `.gitignore`에 추가 권장 여부: 프로젝트별 판단 (공유해도 무방한 메타데이터)

## 하위 호환성 체크리스트

- [ ] 기존 DB (project 컬럼 없음) → 마이그레이션 후 모든 agents의 project = ''
- [ ] `.agentcom.json` 없는 디렉토리 → project = '' → 기존과 동일 동작
- [ ] `--project` 미지정 + `.agentcom.json` 없음 → 모든 에이전트 표시 (레거시 모드)
- [ ] 기존 `agentcom register --name alpha --type codex` → project=''로 등록 (변경 없음)
- [ ] MCP 도구 호출 시 project 파라미터 미지정 → 서버 기본 project 사용 (= '')
- [ ] 기존 테스트 전부 통과 (project=''로 동작)

## 예상 작업량

| Phase | 태스크 수 | 예상 난이도 |
|-------|----------|------------|
| 10-1: DB 스키마 | 4 (+3 서브) | 중 (SQLite 테이블 재생성, 마이그레이션 버전 관리) |
| 10-2: DB 모델 | 10 | 하 (기계적 수정, 테스트 다수) |
| 10-3: 마커 파일 | 6 | 하 (독립적, 단순 JSON 읽기/쓰기) |
| 10-4: 레지스트리 | 6 | 하 (시그니처 변경 + 전달) |
| 10-5: CLI init | 8 | 중 (P9 wizard 이후 project 단계 추가) |
| 10-6: CLI 기존 | 12 | 하~중 (각 명령별 1-2줄 수정, 통합 테스트) |
| 10-7: MCP | 10 | 하 (패턴 반복) |
| 10-8: 통합/문서 | 7 | 하 (E2E + 문서) |
| **합계** | **63 (+3 서브)** | |

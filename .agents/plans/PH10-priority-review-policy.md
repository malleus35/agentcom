# PH10: Priority Enforcement + Review Policy System — PRD

> 작성일: 2026-03-17
> 브랜치: `feature/PH10-priority-review-policy`
> 선행 의존: P12 user-endpoint (완료)
> 관련 문서: PH10-architectural-decisions.md, PH10-review-system-analysis.md, PH10-user-task-approver-design.md

## 1. 목표

agentcom 태스크 시스템에 **고정된 우선순위 레벨 강제**와 **우선순위 기반 리뷰어 자동 할당 정책**을 추가한다.

### 핵심 요구사항

1. `priority`는 `low | medium | high | critical` 4단계로 **명시적으로 강제** (자유 문자열 금지)
2. `high` 이상이면 **선언적으로 reviewer 필수** 추가
3. `reviewer`는 `user`, 에이전트 이름, 에이전트 ID로 판별 (자유 문자열)
4. priority 기준은 **고정** (코드에 내장), review policy 임계값은 **template별 변경 가능**
5. `blocked → completed` 전환 추가 (리뷰 완료 후 직접 완료 허용)

## 2. 우선순위 정의 (고정)

업계 표준 조사 결과 (JIRA, Linear, GitHub, PagerDuty, Temporal, Asana, Camunda, CrewAI, LangGraph, AutoGen, Cordum)를 종합한 분류 기준:

| 레벨 | 값 | 기준 | 예시 |
|------|-----|------|------|
| **Critical** | `critical` | **비가역적 AND 고영향**: 외부 시스템 상태 변경, DB 데이터 변경/삭제/마이그레이션, 외부 API 호출 (결제/이메일/알림), 보안 자격증명 접근/변경, 프로덕션 인프라 변경, 돌이킬 수 없는 작업 | DB 스키마 마이그레이션, 결제 API 호출, 시크릿 로테이션, 프로덕션 배포 |
| **High** | `high` | **비가역적 OR 고영향 (하나 이상)**: 구조적 변경, 다수 에이전트/모듈에 영향, 우회 방법 없는 차단 가능성, SLA 위반 가능성 | 아키텍처 리팩토링, API 스키마 변경, 인프라 설정 변경, 다중 파일 대규모 변경 |
| **Medium** | `medium` | **가역적 + 제한된 영향**: 진행에 영향이 있으나 우회 가능, 단일 모듈 범위 | 기능 구현, 버그 수정, 설정 변경, 문서 업데이트 |
| **Low** | `low` | **완전 가역적 + 최소 영향**: 코드 품질 개선, 리팩토링, 테스트 추가, 코스메틱 변경 | 변수명 변경, 주석 추가, 린트 수정, 의존성 업데이트 |

### 분류 결정 축 (JIRA Impact×Urgency 모델 + PagerDuty SEV 기준 종합)

```
                    비가역적              가역적
                    ┌────────────────────┬────────────────────┐
  고영향            │    CRITICAL        │       HIGH         │
  (다수 에이전트,    │  (외부 연동,        │  (구조적 변경,      │
   SLA 위반,        │   데이터 손실 위험)  │   차단 가능)        │
   데이터 노출)     │                    │                    │
                    ├────────────────────┼────────────────────┤
  저영향            │       HIGH         │   MEDIUM / LOW     │
  (단일 모듈,       │  (되돌리기 어려움)   │  (일반 작업)        │
   우회 가능)       │                    │                    │
                    └────────────────────┴────────────────────┘
```

### 근거: 업계 비교

| 시스템 | Critical에 해당 | High에 해당 | 자동 승인 게이트 |
|--------|-----------------|-------------|-----------------|
| **PagerDuty** | SEV-1: 시스템 중단 + 데이터 노출 | SEV-2: 다수 사용자 기능 불가 | SEV-1/2 = 자동 IC 페이징 |
| **JIRA** | Highest: 진행 차단 | High: 진행 차단 가능성 | Impact × Urgency 매트릭스 |
| **Linear** | Urgent: 24h SLA + 즉시 이메일 | High: 1주 SLA | Urgent = 자동 이메일 에스컬레이션 |
| **GitHub** | P0: 전체 작업 중단 | P1: 주요 기능 불가 | 없음 |
| **Temporal** | PriorityKey=1: 최우선 실행 | PriorityKey=2: 높은 우선순위 | 없음 (실행 순서만) |
| **Camunda** | priority=100: User Task 지정 | priority=75 | BPMN User Task = 자동 정지 |
| **Cordum** | prod writes + secrets + network | high-risk packs | risk_tags 기반 자동 승인 요구 |

> **agentcom 차별점**: 어떤 멀티에이전트 프레임워크도 priority → review를 네이티브로 연결하지 않는다 (CrewAI: 정적 boolean, LangGraph: 노드 기반 interrupt, AutoGen: 에이전트 구성 시 결정). agentcom은 priority 레벨에 기반한 선언적 리뷰 정책을 template 수준에서 설정 가능하게 하는 최초 구현이 된다.

## 3. Review Policy 구조

### 3.1 Template-Level Review Policy (template.json 확장)

```json
{
  "name": "company",
  "description": "...",
  "roles": [...],
  "review_policy": {
    "require_review_above": "high",
    "default_reviewer": "user",
    "rules": [
      {
        "priority": "critical",
        "reviewer": "user"
      },
      {
        "priority": "high",
        "reviewer": "review"
      }
    ]
  }
}
```

**필드 설명:**

| 필드 | 타입 | 필수 | 설명 |
|------|------|------|------|
| `require_review_above` | string | Y | 이 우선순위 이상이면 reviewer 필수. `"high"` = high, critical에 적용. `"critical"` = critical만 적용 |
| `default_reviewer` | string | N | rules에 매칭되는 항목이 없을 때 기본 리뷰어. 미지정 시 `"user"` |
| `rules` | array | N | 우선순위별 구체적 리뷰어 지정. 더 구체적인 규칙이 default보다 우선 |
| `rules[].priority` | string | Y | 대상 우선순위 (`critical`, `high`) |
| `rules[].reviewer` | string | Y | 리뷰어 (`user`, 에이전트 이름, 에이전트 ID) |

### 3.2 Review Policy 적용 로직

```
태스크 생성 (priority 지정)
  ↓
priority 검증 (low|medium|high|critical만 허용)
  ↓
review_policy 조회 (active template의 review_policy)
  ↓
priority >= require_review_above?
  ├─ YES: reviewer 결정 (rules 매칭 → default_reviewer → "user" fallback)
  │   ↓
  │   태스크에 reviewer 필드 설정
  │   태스크 상태를 blocked로 전환 (리뷰 대기)
  │   reviewer에게 알림 메시지 발송
  │   ↓
  │   리뷰어 승인 → blocked → completed (또는 blocked → in_progress)
  │   리뷰어 거부 → blocked → pending (재작업)
  │
  └─ NO: 일반 태스크 생성 (reviewer 없음, 상태 = pending)
```

### 3.3 Reviewer 값 규칙

- `"user"` → 인간 오퍼레이터 (pseudo-agent)
- 에이전트 이름 (예: `"review"`, `"architect"`) → 이름으로 에이전트 검색
- 에이전트 ID (예: `"agt_xxx"`) → ID로 직접 참조
- 빈 문자열 → template default_reviewer 적용
- reviewer가 등록되지 않은 경우 → 경고 출력, 태스크는 생성되지만 blocked 전환은 건너뜀

### 3.4 Review Policy가 없는 경우 동작 (하위 호환성)

- template에 `review_policy` 미정의 → **기존과 동일 동작** (reviewer 없음, priority 검증만 적용)
- review_policy가 있지만 task의 priority가 임계값 미만 → 일반 태스크
- `--reviewer` CLI 플래그로 명시적 지정 → template policy와 무관하게 적용

## 4. 상태 전이 변경

### 현재 상태 전이 (model.go)

```
blocked → {in_progress, pending, cancelled}
```

### 변경 후

```
blocked → {in_progress, pending, cancelled, completed}
```

`blocked → completed` 추가 이유: 리뷰어가 블로킹된 태스크를 직접 승인하고 완료 처리할 수 있어야 함.

## 5. 기존 버그 수정 (이번 구현에 포함)

### 5.1 CLI `task update` 전이 검증 우회 (task.go:218-222)

현재: raw SQL `UPDATE tasks SET status=?, result=?` 직접 실행
수정: `Manager.UpdateStatus()`를 호출하여 `ValidateTransition()` 거쳐야 함

### 5.2 DB `InsertTask()` 이중 ID 생성 (db/task.go:30-35)

현재: `Manager.Create()`에서 `tsk_<id>` 생성 후, `InsertTask()`에서 다시 `tsk_<id>` 덮어쓰기
수정: `InsertTask()`는 이미 ID가 있으면 그대로 사용, 없을 때만 생성

### 5.3 MCP `update_task` 도구 부재

현재: 에이전트가 MCP로 태스크 생성은 가능하나 상태 업데이트 불가
수정: `update_task` MCP 도구 추가

## 6. Task Breakdown

### Phase A: Priority Validation (3 tasks, ~8 subtasks)

#### Task A-1: Priority 모델 정의 및 검증 함수 추가
**파일**: `internal/task/model.go`
**예상 시간**: 30분

- **A-1-1**: `ValidPriorities` 상수 슬라이스 정의 (`["low", "medium", "high", "critical"]`)
- **A-1-2**: `ValidatePriority(priority string) error` 함수 구현 — 허용되지 않은 값이면 `ErrInvalidPriority` 반환
- **A-1-3**: `ComparePriority(a, b string) int` 함수 구현 — low=0, medium=1, high=2, critical=3 순서로 비교
- **A-1-4**: `PriorityLevel(priority string) int` 헬퍼 함수 — 문자열을 정수 레벨로 변환

**테스트 (A-1-T)**:
- 유효한 4개 값 통과 확인
- 빈 문자열, 임의 문자열, 대문자 변형 거부 확인
- 비교 함수: low < medium < high < critical 순서 확인
- 에지 케이스: 동일 값 비교 = 0

#### Task A-2: 기존 코드에 Priority 검증 삽입
**파일**: `internal/task/manager.go`, `internal/cli/task.go`, `internal/mcp/handler.go`
**예상 시간**: 1시간

- **A-2-1**: `Manager.Create()`에 `ValidatePriority()` 호출 추가 — 빈 문자열은 `"medium"` 기본값, 무효 값은 에러 반환
- **A-2-2**: CLI `task create` 명령의 `--priority` 플래그를 ValidPriorities 기반 enum 힌트로 업데이트
- **A-2-3**: MCP `handleCreateTask()`에 priority 검증 추가 — 무효 값 시 JSON-RPC 에러 응답
- **A-2-4**: CLI `task update`의 raw SQL을 `Manager.UpdateStatus()` 호출로 교체 (**버그 5.1 수정**)

**테스트 (A-2-T)**:
- Manager.Create()에 무효 priority 전달 시 에러 확인
- Manager.Create()에 빈 priority → "medium" 기본값 확인
- CLI `task create --priority invalid` → 에러 메시지 확인
- CLI `task update`가 ValidateTransition()을 거치는지 확인
- MCP createTask에 무효 priority → JSON-RPC 에러 확인

#### Task A-3: DB InsertTask() ID 이중 생성 수정
**파일**: `internal/db/task.go`
**예상 시간**: 20분

- **A-3-1**: `InsertTask()`에서 `task.ID`가 비어있지 않으면 ID 생성 건너뛰기
- **A-3-2**: `task.ID`가 비어있을 때만 `tsk_<nanoid>` 생성 (하위 호환성)

**테스트 (A-3-T)**:
- 미리 ID가 설정된 Task → InsertTask() 후 원래 ID 유지 확인
- ID 비어있는 Task → InsertTask() 후 `tsk_` 프리픽스 ID 자동 생성 확인

### Phase B: Reviewer 필드 및 상태 전이 (3 tasks, ~10 subtasks)

#### Task B-1: DB 스키마에 reviewer 컬럼 추가
**파일**: `internal/db/migrations.go`, `internal/db/task.go`
**예상 시간**: 45분

- **B-1-1**: 새 마이그레이션 추가: `ALTER TABLE tasks ADD COLUMN reviewer TEXT`
- **B-1-2**: `Task` 구조체에 `Reviewer string` 필드 추가
- **B-1-3**: `scanTask()`에 reviewer 컬럼 스캔 추가 (nullable)
- **B-1-4**: `InsertTask()`, `UpdateTask()` 쿼리에 reviewer 포함
- **B-1-5**: `UpdateTaskStatus()`에 reviewer 업데이트 옵션 추가 (선택적)

**테스트 (B-1-T)**:
- 마이그레이션 후 reviewer 컬럼 존재 확인
- reviewer가 있는 Task 삽입/조회 확인
- reviewer가 없는 Task → NULL 저장, 빈 문자열 반환 확인
- 기존 DB에서 마이그레이션 후 기존 태스크의 reviewer = NULL 확인

#### Task B-2: blocked → completed 전이 추가
**파일**: `internal/task/model.go`
**예상 시간**: 15분

- **B-2-1**: `validTransitions[StatusBlocked]`에 `StatusCompleted` 추가

**테스트 (B-2-T)**:
- `ValidateTransition("blocked", "completed")` → nil 확인
- 기존 전이 (blocked → in_progress, pending, cancelled) 여전히 동작 확인
- 다른 터미널 전이는 변경 없음 확인

#### Task B-3: Manager에 reviewer 로직 통합
**파일**: `internal/task/manager.go`
**예상 시간**: 1시간

- **B-3-1**: `Manager.Create()` 시그니처에 `reviewer string` 파라미터 추가
- **B-3-2**: reviewer가 지정된 태스크는 생성 시 `status = "blocked"` 으로 설정 (리뷰 대기)
- **B-3-3**: `Manager.ApproveTask(ctx, taskID, reviewerID, result)` 메서드 추가 — reviewer 확인 후 blocked → completed 전이
- **B-3-4**: `Manager.RejectTask(ctx, taskID, reviewerID, reason)` 메서드 추가 — reviewer 확인 후 blocked → pending 전이 (재작업)

**테스트 (B-3-T)**:
- reviewer 지정 태스크 생성 → status = "blocked" 확인
- reviewer 미지정 태스크 → status = "pending" (기존 동작 유지)
- ApproveTask: 올바른 reviewer → blocked → completed 확인
- ApproveTask: 잘못된 reviewer → 에러 확인
- RejectTask: blocked → pending 확인
- RejectTask: 비-blocked 태스크 → 에러 확인

### Phase C: Review Policy — Template 통합 (4 tasks, ~14 subtasks)

#### Task C-1: ReviewPolicy 타입 정의
**파일**: `internal/cli/agents.go` (또는 새 파일 `internal/task/policy.go`)
**예상 시간**: 30분

- **C-1-1**: `ReviewPolicy` 구조체 정의
  ```go
  type ReviewPolicy struct {
      RequireReviewAbove string       `json:"require_review_above"`
      DefaultReviewer    string       `json:"default_reviewer,omitempty"`
      Rules              []PolicyRule `json:"rules,omitempty"`
  }
  type PolicyRule struct {
      Priority string `json:"priority"`
      Reviewer string `json:"reviewer"`
  }
  ```
- **C-1-2**: `ReviewPolicy.ResolveReviewer(priority string) (string, bool)` 메서드 — 주어진 priority에 대한 reviewer 결정. `bool`은 리뷰 필요 여부
- **C-1-3**: `ReviewPolicy.Validate() error` 메서드 — require_review_above가 유효한 priority인지, rules의 priority가 유효한지 검증

**테스트 (C-1-T)**:
- ResolveReviewer: high priority + require_review_above="high" → reviewer 반환, true
- ResolveReviewer: medium priority + require_review_above="high" → "", false
- ResolveReviewer: critical priority + rules에 critical 매칭 → 해당 rule의 reviewer
- ResolveReviewer: high priority + rules에 매칭 없음 → default_reviewer
- Validate: require_review_above="invalid" → 에러
- Validate: rules[].priority="invalid" → 에러

#### Task C-2: templateDefinition에 ReviewPolicy 통합
**파일**: `internal/cli/agents.go`, `internal/cli/template_store.go`, `internal/onboard/template_definition.go`
**예상 시간**: 45분

- **C-2-1**: `templateDefinition` 구조체에 `ReviewPolicy *ReviewPolicy` 필드 추가 (JSON/YAML 직렬화 지원)
- **C-2-2**: `onboard.TemplateDefinition`에도 동일 필드 추가
- **C-2-3**: built-in 템플릿(`company`, `oh-my-opencode`)에 기본 review_policy 추가
  - company: `require_review_above: "high"`, default_reviewer: `"review"`, critical: `"user"`
  - oh-my-opencode: `require_review_above: "high"`, default_reviewer: `"momus"`, critical: `"user"`
- **C-2-4**: `validateCustomTemplateDefinition()`에 review_policy 검증 추가

**테스트 (C-2-T)**:
- built-in 템플릿 로드 시 review_policy 필드 존재 확인
- custom 템플릿 JSON 파싱 시 review_policy 역직렬화 확인
- review_policy가 없는 기존 template.json → nil로 로드 (하위 호환)
- 무효한 review_policy → 검증 에러

#### Task C-3: 태스크 생성 파이프라인에 Review Policy 적용
**파일**: `internal/cli/task.go`, `internal/mcp/handler.go`
**예상 시간**: 1.5시간

- **C-3-1**: CLI `task create`에 `--reviewer` 플래그 추가 (명시적 리뷰어 지정)
- **C-3-2**: CLI `task create` 로직에 review policy 조회 추가:
  1. `--reviewer` 명시적 지정 → 그 값 사용
  2. 미지정 → active template의 review_policy 조회
  3. policy 매칭 → 자동 reviewer 설정
  4. policy 없음 → reviewer 없이 진행
- **C-3-3**: reviewer가 결정되면 `Manager.Create()`에 reviewer 전달
- **C-3-4**: 태스크 생성 후 reviewer에게 알림 메시지 자동 발송 (`send` 활용)
- **C-3-5**: MCP `handleCreateTask()`에 동일한 review policy 로직 추가
- **C-3-6**: MCP `handleCreateTask()`의 `create_task` 도구 스키마에 `reviewer` 파라미터 추가

**테스트 (C-3-T)**:
- CLI: `--reviewer user` → reviewer="user", status="blocked" 확인
- CLI: `--priority high` (reviewer 미지정, policy 있음) → 자동 reviewer 설정 확인
- CLI: `--priority low` (policy 있음) → reviewer 없음 확인
- CLI: reviewer 지정 시 알림 메시지 발송 확인
- MCP: reviewer 파라미터 전달 확인
- MCP: priority 기반 자동 reviewer 확인
- 하위 호환: review_policy 없는 프로젝트에서 기존 동작 유지 확인

#### Task C-4: CLI 태스크 승인/거부 커맨드 추가
**파일**: `internal/cli/task.go`
**예상 시간**: 1시간

- **C-4-1**: `agentcom task approve <task-id> --as <reviewer> [--result "승인 사유"]` 서브커맨드 추가
- **C-4-2**: `agentcom task reject <task-id> --as <reviewer> [--reason "거부 사유"]` 서브커맨드 추가
- **C-4-3**: `--as` 플래그 — reviewer 식별 (user, agent name, agent ID)
- **C-4-4**: 각 커맨드에서 `Manager.ApproveTask()` / `Manager.RejectTask()` 호출
- **C-4-5**: JSON 출력 지원

**테스트 (C-4-T)**:
- `task approve` 정상 흐름 확인
- `task approve` 잘못된 reviewer → 에러 확인
- `task reject` 정상 흐름 확인
- 비-blocked 태스크에 approve/reject → 에러 확인
- JSON 출력 형식 확인

### Phase D: MCP update_task 도구 추가 (1 task, ~4 subtasks)

#### Task D-1: MCP update_task 도구 구현
**파일**: `internal/mcp/tools.go`, `internal/mcp/handler.go`
**예상 시간**: 45분

- **D-1-1**: `tools.go`에 `update_task` 도구 정의 추가 (schema: task_id, status, result, reviewer)
- **D-1-2**: `handler.go`에 `handleUpdateTask()` 핸들러 구현 — `Manager.UpdateStatus()` 활용
- **D-1-3**: `handleUpdateTask()`에 approve/reject 특수 상태 지원 (status="approved" → ApproveTask, status="rejected" → RejectTask)
- **D-1-4**: 서버의 tool 라우팅 맵에 `update_task` 등록

**테스트 (D-1-T)**:
- MCP roundtrip: update_task → 상태 변경 확인
- MCP: 무효한 전이 → JSON-RPC 에러 확인
- MCP: approve 흐름 확인
- MCP: reject 흐름 확인

### Phase E: 테스트 보강 및 통합 검증 (2 tasks, ~6 subtasks)

#### Task E-1: 단위 테스트 종합 검증
**파일**: `internal/task/manager_test.go`, `internal/task/model_test.go`, `internal/db/task_test.go`
**예상 시간**: 1시간

- **E-1-1**: model_test.go — priority 검증, 비교, 전이 테이블 전체 커버
- **E-1-2**: manager_test.go — reviewer 기반 생성, 승인/거부 플로우
- **E-1-3**: db/task_test.go — reviewer 컬럼 CRUD, 마이그레이션 검증

#### Task E-2: E2E 시나리오 테스트
**파일**: `cmd/agentcom/e2e_test.go`
**예상 시간**: 1시간

- **E-2-1**: 전체 흐름 E2E: task create --priority high → auto-reviewer → blocked → approve → completed
- **E-2-2**: review policy 없는 프로젝트 E2E: 기존 동작 100% 호환 확인
- **E-2-3**: MCP 경유 E2E: create_task (high priority) → update_task (approve) → completed

### Phase F: 문서 및 마무리 (2 tasks, ~6 subtasks)

#### Task F-1: README 업데이트
**파일**: `README.md`, `README.ko.md`, `README.ja.md`, `README.zh.md`
**예상 시간**: 45분

- **F-1-1**: Priority 레벨 표 추가 (critical/high/medium/low 정의)
- **F-1-2**: Review Policy 설명 및 template.json 예시 추가
- **F-1-3**: `task approve` / `task reject` 커맨드 문서 추가
- **F-1-4**: MCP `update_task` 도구 문서 추가

#### Task F-2: AGENTS.md 및 MEMORY.md 업데이트
**파일**: `AGENTS.md`, `.agents/MEMORY.md`
**예상 시간**: 20분

- **F-2-1**: AGENTS.md line 166 "MCP 퍼스트" → "CLI-first, MCP는 셸 없는 런타임용 선택적 어댑터" 변경
- **F-2-2**: MEMORY.md에 PH10 완료 기록, 설계 결정 로그 업데이트

## 7. 구현 순서 및 의존성

```
Phase A (Priority Validation)
  ├── A-1: Priority 모델 (독립)
  ├── A-2: 기존 코드 검증 삽입 (A-1 의존)
  └── A-3: InsertTask ID 수정 (독립)

Phase B (Reviewer + State Machine)
  ├── B-1: DB 스키마 변경 (독립)
  ├── B-2: blocked→completed 전이 (독립)
  └── B-3: Manager reviewer 로직 (A-1, B-1, B-2 의존)

Phase C (Review Policy — Template)
  ├── C-1: ReviewPolicy 타입 (A-1 의존)
  ├── C-2: Template 통합 (C-1 의존)
  ├── C-3: 생성 파이프라인 (B-3, C-2 의존)
  └── C-4: 승인/거부 CLI (B-3 의존)

Phase D (MCP update_task)
  └── D-1: MCP 도구 (B-3 의존)

Phase E (테스트)
  ├── E-1: 단위 테스트 (Phase A~D 완료 후)
  └── E-2: E2E 테스트 (Phase A~D 완료 후)

Phase F (문서)
  ├── F-1: README (Phase A~D 완료 후)
  └── F-2: AGENTS.md/MEMORY.md (전체 완료 후)
```

### 병렬 실행 가능 그룹

- **Wave 1**: A-1 + A-3 + B-1 + B-2 (모두 독립)
- **Wave 2**: A-2 + B-3 + C-1 (Wave 1 결과 의존)
- **Wave 3**: C-2 + C-4 + D-1 (Wave 2 결과 의존)
- **Wave 4**: C-3 (Wave 3 결과 의존)
- **Wave 5**: E-1 + E-2 (Wave 4 완료 후)
- **Wave 6**: F-1 + F-2 (Wave 5 완료 후)

## 8. 영향받는 파일 목록 (총 ~15개)

| 파일 | 변경 유형 | Phase |
|------|----------|-------|
| `internal/task/model.go` | 수정 (priority 검증, 전이 추가) | A, B |
| `internal/task/manager.go` | 수정 (priority 검증, reviewer 로직) | A, B |
| `internal/task/policy.go` | **신규** (ReviewPolicy 타입) | C |
| `internal/task/manager_test.go` | 수정 (새 테스트) | E |
| `internal/task/model_test.go` | **신규** (priority/전이 테스트) | E |
| `internal/db/migrations.go` | 수정 (reviewer 컬럼 마이그레이션) | B |
| `internal/db/task.go` | 수정 (reviewer 필드, ID 수정) | A, B |
| `internal/db/task_test.go` | 수정 (reviewer CRUD 테스트) | E |
| `internal/cli/task.go` | 수정 (priority 검증, reviewer 플래그, approve/reject) | A, C |
| `internal/cli/agents.go` | 수정 (ReviewPolicy in templateDefinition) | C |
| `internal/cli/template_store.go` | 수정 (review_policy 검증) | C |
| `internal/mcp/handler.go` | 수정 (priority 검증, update_task) | A, D |
| `internal/mcp/tools.go` | 수정 (update_task 도구 정의) | D |
| `internal/onboard/template_definition.go` | 수정 (ReviewPolicy 필드) | C |
| `cmd/agentcom/e2e_test.go` | 수정 (E2E 시나리오) | E |
| `README.md` + 3 translations | 수정 (문서) | F |
| `AGENTS.md` | 수정 (CLI-first) | F |
| `.agents/MEMORY.md` | 수정 (완료 기록) | F |

## 9. 추정 일정

| Phase | 구현 | 테스트 | 합계 |
|-------|------|--------|------|
| A: Priority Validation | 1.5h | 1h | 2.5h |
| B: Reviewer + State | 2h | 1h | 3h |
| C: Review Policy | 3.5h | 1.5h | 5h |
| D: MCP update_task | 0.75h | 0.5h | 1.25h |
| E: 테스트 보강 | — | 2h | 2h |
| F: 문서 | 1h | — | 1h |
| **합계** | **8.75h** | **6h** | **~14.75h** |

## 10. 성공 기준

- [ ] `agentcom task create --priority invalid` → 에러 (low/medium/high/critical만 허용)
- [ ] `agentcom task create --priority high` (company template) → reviewer="review", status="blocked"
- [ ] `agentcom task create --priority critical` (company template) → reviewer="user", status="blocked"
- [ ] `agentcom task create --priority medium` → reviewer 없음, status="pending" (기존 동작)
- [ ] `agentcom task approve <id> --as review` → blocked → completed
- [ ] `agentcom task reject <id> --as review` → blocked → pending
- [ ] MCP `create_task` + `update_task` 전체 흐름 동작
- [ ] review_policy 없는 기존 프로젝트에서 100% 하위 호환
- [ ] `go test ./...` 전체 통과
- [ ] `go build ./...` 전체 통과

## 11. 리스크 및 완화

| 리스크 | 영향 | 완화 |
|--------|------|------|
| 기존 태스크에 reviewer=NULL → 기존 blocked 태스크가 예상과 다르게 동작 | 중 | 마이그레이션이 기존 데이터를 변경하지 않음 (ALTER TABLE ADD COLUMN) |
| template.json에 review_policy가 없는 기존 프로젝트 | 낮음 | nil 체크로 기존 동작 보존 |
| CLI task update 전이 검증 강화로 기존 스크립트 호환성 | 중 | 기존 무효 전이를 허용하던 것이 버그였으므로 의도적 Breaking Change |
| Manager.Create() 시그니처 변경 | 낮음 | 호출부 전체 업데이트 (CLI, MCP, 테스트) |
| InsertTask() ID 로직 변경 | 낮음 | 기존 ID 없는 호출은 이전과 동일하게 자동 생성 |

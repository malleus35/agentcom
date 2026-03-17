# PH10: Priority Enforcement & Review Policy — Final PRD

> **상태**: 최종 확정 (2026-03-17)
> **브랜치**: `feature/PH10-priority-review-policy`
> **추정 공수**: ~16h (6 Phase, 18 Tasks, ~62 Subtasks)
> **선행 조건**: P12 user-endpoint 완료 (commit `54b460c`)

---

## 1. 목표

agentcom의 태스크 시스템에 **우선순위 강제 검증**과 **선택적 리뷰 게이트**를 추가한다.

### 1.1 핵심 요구사항

| # | 요구사항 | 상세 |
|---|----------|------|
| R1 | Priority 4단계 강제 | `low`, `medium`, `high`, `critical` — 코드 레벨 enum, 입력 검증 필수 |
| R2 | Reviewer 필드 추가 | 태스크에 `reviewer` 문자열 필드 추가. 에이전트 이름/ID 또는 `"user"` |
| R3 | 선택적 리뷰 게이트 | reviewer가 있는 태스크만 `in_progress → blocked` 강제 전이. reviewer 없으면 기존 흐름 유지 |
| R4 | Review Policy 템플릿 통합 | 템플릿에 `review_policy` 선언. priority 임계값 이상이면 reviewer 자동 지정 |
| R5 | 기존 버그 수정 | CLI raw SQL bypass, DB double ID generation, delegate 상태 미설정 |

### 1.2 범위 밖 (명시적 제외)

- 에이전트 리뷰 자동화 로직 (retry, cascading, guardrail) — agentcom은 통신 인프라, 실행 엔진 아님
- 리뷰 타임아웃 — 업계 표준에 따라 human review는 무한 대기 (auto-approval은 리뷰 목적 무효화)
- `review_chain` 태스크 컬럼 — 워크플로우 그래프/별도 테이블이 올바른 추상화
- `pending_review` 별도 상태 — 기존 `blocked` 상태 재활용이 최소 변경

---

## 2. 아키텍처 결정 (확정)

### 2.1 Priority 정의 (고정, 코드 임베딩)

| Level | 기준 | 예시 |
|-------|------|------|
| **Critical** | 비가역적(Irreversible) **AND** 고영향(High-impact): 외부 시스템 변경, DB 변이, 외부 API 호출, 보안 자격 증명 | DB 마이그레이션, 결제 API, 시크릿 로테이션, 프로덕션 배포 |
| **High** | 비가역적 **OR** 고영향 (하나 이상): 구조적 변경, 다중 에이전트 영향 | 아키텍처 리팩터, API 스키마 변경, 인프라 설정 |
| **Medium** | 가역적 + 제한적 영향: 단일 모듈 범위 | 기능 구현, 버그 수정, 설정 변경, 문서 업데이트 |
| **Low** | 완전 가역적 + 최소 영향 | 변수 이름 변경, 주석, lint 수정, 의존성 업데이트 |

**업계 비교**: PagerDuty SEV-1/2 경계(Major Incident), JIRA Blocker/Critical, Linear Urgent/High와 일관된 2축 분류(가역성 × 영향 범위).

### 2.2 결정 테이블

| 결정 | 선택 | 근거 |
|------|------|------|
| `review_chain` 컬럼 | **거부** | Temporal, Camunda, Airflow 등 어떤 프로덕션 시스템도 review_chain을 태스크 테이블 컬럼으로 두지 않음. 워크플로우 그래프/별도 테이블이 올바른 추상화 (ICML 2025 "LLM-as-Judge" 논문 참조) |
| MCP 포지셔닝 | **CLI-first** | MCP 9개 도구 모두 CLI 대응 존재, 고유 기능 0, 토큰 비용 4-32x (Scalekit 벤치마크), 셸 없는 런타임에서만 필요 |
| `reviewer` 필드 타입 | **자유 문자열** | 에이전트 ID + "user" 모두 지원, agentcom "에이전트 유형 자유" 원칙 부합, 추가 비용 0 |
| 에이전트 리뷰 자동화 | **범위 밖** | agentcom은 라우팅 + 상태 전환 제어만 담당 |
| `blocked → completed` 전이 | **추가** | model.go validTransitions 1줄 변경. 리뷰 승인 후 직접 완료 가능 |
| Human 리뷰 타임아웃 | **없음 (무한 대기)** | JIRA, Linear, GitHub, Asana, Temporal, Camunda 모두 human task에 타임아웃 없음 |
| 리뷰 게이트 방식 | **선택적 (Option B)** | reviewer 필드가 있는 태스크만 리뷰 게이트 적용. 모든 태스크 강제(Option A)는 approval fatigue 유발 |
| `in_progress → completed` (reviewer 있을 때) | **차단** | reviewer 지정 태스크는 반드시 `blocked` 거쳐야 `completed` 전환 가능 |

### 2.3 상태 전이 변경

**현재 validTransitions**:
```
pending → assigned, in_progress, cancelled
assigned → in_progress, blocked, pending, cancelled
in_progress → completed, failed, blocked
blocked → in_progress, pending, cancelled
```

**변경 후**:
```
pending → assigned, in_progress, cancelled
assigned → in_progress, blocked, pending, cancelled
in_progress → completed*, failed, blocked
blocked → in_progress, pending, cancelled, completed  ← 추가
```

`*` `in_progress → completed`: reviewer 없으면 허용, reviewer 있으면 차단 (blocked 경유 필수)

### 2.4 Review Policy 템플릿 스키마

```json
{
  "review_policy": {
    "require_review_above": "high",
    "default_reviewer": "user",
    "rules": [
      { "priority": "critical", "reviewer": "user" },
      { "priority": "high", "reviewer": "review" }
    ]
  }
}
```

- `require_review_above`: 이 priority 이상이면 reviewer 자동 지정 (포함)
- `default_reviewer`: rules에 매칭 안 될 때 기본 reviewer
- `rules`: priority별 reviewer 오버라이드. 우선순위 높은 규칙이 먼저 매칭
- 템플릿에 `review_policy` 없으면 기존 동작 유지 (하위 호환)

### 2.5 시퀀스 다이어그램

```
Agent A                 agentcom                  Reviewer (user/agent)
  │                        │                           │
  │  task create           │                           │
  │  (priority=high)       │                           │
  ├───────────────────────>│                           │
  │                        │ apply review_policy       │
  │                        │ → reviewer="user"         │
  │                        │ status=pending            │
  │  <─ created ───────────┤                           │
  │                        │                           │
  │  task update           │                           │
  │  status=in_progress    │                           │
  ├───────────────────────>│                           │
  │                        │ ValidateTransition OK     │
  │  <─ updated ───────────┤                           │
  │                        │                           │
  │  task update           │                           │
  │  status=completed      │                           │
  ├───────────────────────>│                           │
  │                        │ BLOCKED: reviewer exists  │
  │                        │ → force status=blocked    │
  │  <─ blocked ───────────┤                           │
  │                        │                           │
  │                        │  "task needs review"      │
  │                        ├──────────────────────────>│
  │                        │                           │
  │                        │  approve/reject           │
  │                        │<──────────────────────────┤
  │                        │                           │
  │                        │ status=completed/failed   │
  │  <─ final status ──────┤                           │
```

---

## 3. 현재 코드베이스 상태 (PH10 관련)

### 3.1 Priority 핸들링 — 검증 없음

| 위치 | 현재 동작 | 문제 |
|------|-----------|------|
| `internal/task/model.go` | 상태 상수만 정의, priority 상수 없음 | enum 없음 |
| `internal/task/manager.go:38-41` | 빈 문자열이면 "medium" 기본값 | 임의 문자열 허용 |
| `internal/cli/task.go:110` | `--priority` 플래그, 기본 "medium" | 검증 없음 |
| `internal/mcp/handler.go:314-317` | 기본 "medium" | 검증 없음 |
| `internal/db/migrations.go:39` | `priority TEXT NOT NULL DEFAULT 'medium'` | DB 레벨 enum 없음 |

### 3.2 Reviewer 필드 — 존재하지 않음

- `db.Task` 구조체: `Reviewer` 필드 없음
- DB 스키마: `reviewer` 컬럼 없음
- "reviewer"는 템플릿 역할 이름으로만 존재 (company 템플릿의 "review" 역할)

### 3.3 알려진 버그 5개

| # | 버그 | 위치 | 영향 |
|---|------|------|------|
| B1 | CLI `task update` — ValidateTransition 우회 | `task.go:218-222` | 임의 상태 전환 가능 |
| B2 | DB `InsertTask` — double ID generation | `db/task.go:31-35` | Manager가 생성한 ID 덮어씀 |
| B3 | CLI `task delegate` — 상태 미설정 | `task.go:276-280` | delegate 후 상태가 "assigned"로 안 바뀜 |
| B4 | CLI `task create` — Manager 우회 | `task.go:70-81` | raw SQL INSERT, 향후 검증 로직 미적용 |
| B5 | MCP `update_task` 도구 없음 | `tools.go` | 태스크 상태 변경 불가 |

### 3.4 템플릿 시스템 — review_policy 없음

| 구조체 | 위치 | 현재 필드 |
|--------|------|-----------|
| `templateRole` | `agents.go:28-35` | name, description, agent_name, agent_type, communicates_with, responsibilities |
| `TemplateRole` (onboard) | `template_definition.go:12-19` | 동일 |
| `templateManifestRole` (runtime) | `up.go:33-38` | name, agent_name, agent_type |

**통합 지점**: `templateDefinition` 레벨에 `ReviewPolicy` 추가 필요 (role 레벨 아님).

---

## 4. 태스크 분해

### Phase A: Priority Validation (강제 검증) — ~2.5h

#### Task A-1: Priority 상수 및 검증 함수 (internal/task/model.go) — 0.5h

- **A-1-1**: `const` 블록에 `PriorityLow`, `PriorityMedium`, `PriorityHigh`, `PriorityCritical` 상수 추가
- **A-1-2**: `var ValidPriorities = map[string]struct{}` 맵 정의 (4개 키)
- **A-1-3**: `func ValidatePriority(p string) error` 구현 — 빈 문자열은 "medium" 반환이 아니라 에러, 대소문자 정규화 후 ValidPriorities 검사
- **A-1-4**: `func NormalizePriority(p string) string` 구현 — strings.ToLower + strings.TrimSpace
- **A-1-5**: 단위 테스트 추가 (`model_test.go`) — 유효값 4개, 무효값 ("urgent", "ASAP", "", "HIGH" 등), 정규화 테스트

#### Task A-2: Manager.Create에 Priority 검증 통합 (internal/task/manager.go) — 0.5h

- **A-2-1**: `Create()` 함수에서 기존 "빈 문자열이면 medium" 로직을 `NormalizePriority()` + `ValidatePriority()` 호출로 교체
- **A-2-2**: 무효 priority 입력 시 `fmt.Errorf("task.Manager.Create: %w", err)` 반환
- **A-2-3**: 기존 `manager_test.go`에 무효 priority 케이스 추가 (테이블 기반)

#### Task A-3: CLI task create에 Priority 검증 추가 (internal/cli/task.go) — 0.75h

- **A-3-1**: `newTaskCreateCmd()` 실행부에 `task.ValidatePriority()` 호출 추가 (raw SQL INSERT 전)
- **A-3-2**: 무효 priority 시 structured user error 반환 (`errors.go` 패턴 사용)
- **A-3-3**: `--priority` 플래그 help text에 허용값 명시: `"low|medium|high|critical (default: medium)"`
- **A-3-4**: CLI 테스트에 무효 priority 입력 검증 케이스 추가

#### Task A-4: MCP create_task에 Priority 검증 추가 (internal/mcp/handler.go) — 0.5h

- **A-4-1**: `handleCreateTask()`에서 priority 파라미터 추출 후 `task.ValidatePriority()` 호출
- **A-4-2**: 무효 priority 시 JSON-RPC error response 반환 (code -32602 Invalid params)
- **A-4-3**: MCP 테스트에 무효 priority 케이스 추가 (`server_test.go`)

#### Task A-5: Priority 비교 유틸리티 (internal/task/model.go) — 0.25h

- **A-5-1**: `var priorityOrder = map[string]int{"low": 0, "medium": 1, "high": 2, "critical": 3}` 정의
- **A-5-2**: `func ComparePriority(a, b string) int` 구현 — a가 높으면 +1, 같으면 0, 낮으면 -1
- **A-5-3**: `func PriorityAtLeast(p string, threshold string) bool` 구현 — `priorityOrder[p] >= priorityOrder[threshold]`
- **A-5-4**: 단위 테스트 추가 (모든 조합 테이블 기반)

---

### Phase B: Reviewer Field & State Machine (리뷰어 + 상태 전이) — ~3h

#### Task B-1: DB 스키마 마이그레이션 — reviewer 컬럼 추가 (internal/db/migrations.go) — 0.5h

- **B-1-1**: `user_version` 기반 다음 마이그레이션 번호 할당 (현재 최신 확인 후)
- **B-1-2**: `ALTER TABLE tasks ADD COLUMN reviewer TEXT DEFAULT ''` 마이그레이션 SQL 추가
- **B-1-3**: 마이그레이션 테스트 추가 — 빈 DB에서 전체 마이그레이션, 기존 DB에서 증분 마이그레이션 둘 다 검증
- **B-1-4**: 기존 데이터가 빈 reviewer로 마이그레이션되는지 검증

#### Task B-2: Task 구조체에 Reviewer 필드 추가 (internal/db/task.go) — 0.75h

- **B-2-1**: `Task` 구조체에 `Reviewer string` 필드 추가
- **B-2-2**: `InsertTask()` SQL에 `reviewer` 컬럼 포함
- **B-2-3**: `UpdateTask()` SQL에 `reviewer` 컬럼 포함
- **B-2-4**: `scanTask()` / row scan에 `reviewer` 필드 포함
- **B-2-5**: **B2 버그 수정**: `InsertTask()` double ID generation — `if task.ID == ""` 체크 추가
- **B-2-6**: 기존 DB 테스트에 reviewer 필드 포함 케이스 추가

#### Task B-3: Manager에 Reviewer 로직 추가 (internal/task/manager.go) — 0.75h

- **B-3-1**: `Create()` 파라미터에 `reviewer string` 추가
- **B-3-2**: reviewer가 빈 문자열이 아니면 Task.Reviewer에 설정
- **B-3-3**: `UpdateStatus()` 수정 — reviewer가 있는 태스크에서 `in_progress → completed` 시도 시 자동으로 `blocked`로 전환하고 안내 메시지 포함
- **B-3-4**: `func (m *Manager) ApproveTask(ctx, taskID, result string) error` 신규 — `blocked → completed` 전이 실행, reviewer 확인
- **B-3-5**: `func (m *Manager) RejectTask(ctx, taskID, result string) error` 신규 — `blocked → failed` 전이 실행, reviewer 확인
- **B-3-6**: Manager 테스트에 reviewer 관련 시나리오 전체 추가 (reviewer 있는/없는 태스크의 상태 전이 차이, approve/reject)

#### Task B-4: State Machine 전이 추가 (internal/task/model.go) — 0.25h

- **B-4-1**: `validTransitions[StatusBlocked]`에 `StatusCompleted: {}` 추가
- **B-4-2**: `func ValidateTransitionWithReviewer(from, to, reviewer string) error` 구현 — reviewer 있을 때 `in_progress → completed` 차단
- **B-4-3**: 단위 테스트 추가 — `blocked → completed` 허용, reviewer 있을 때 `in_progress → completed` 차단

#### Task B-5: CLI task 명령 수정 (internal/cli/task.go) — 0.75h

- **B-5-1**: `task create`에 `--reviewer` 플래그 추가
- **B-5-2**: **B1 버그 수정**: `task update` raw SQL을 `Manager.UpdateStatus()` 호출로 교체
- **B-5-3**: **B3 버그 수정**: `task delegate` raw SQL을 `Manager.Delegate()` 호출로 교체 (상태 "assigned" 자동 설정)
- **B-5-4**: **B4 버그 수정**: `task create` raw SQL을 `Manager.Create()` 호출로 교체
- **B-5-5**: `agentcom task approve <task-id> [--result "..."]` 서브커맨드 추가
- **B-5-6**: `agentcom task reject <task-id> [--result "..."]` 서브커맨드 추가
- **B-5-7**: CLI 테스트에 reviewer 플래그, approve/reject 명령 테스트 추가

---

### Phase C: Review Policy — Template Integration — ~3.5h

#### Task C-1: ReviewPolicy 타입 정의 (internal/task/policy.go — 신규) — 0.75h

- **C-1-1**: `policy.go` 파일 생성
- **C-1-2**: `type ReviewPolicyRule struct { Priority string; Reviewer string }` 정의
- **C-1-3**: `type ReviewPolicy struct { RequireReviewAbove string; DefaultReviewer string; Rules []ReviewPolicyRule }` 정의
- **C-1-4**: `func (p *ReviewPolicy) Validate() error` 구현 — RequireReviewAbove가 유효 priority인지, Rules의 priority/reviewer 검증
- **C-1-5**: `func (p *ReviewPolicy) ResolveReviewer(priority string) string` 구현 — priority가 임계값 이상이면 rules 매칭 후 reviewer 반환, 미매칭이면 default_reviewer, 임계값 미만이면 빈 문자열
- **C-1-6**: 단위 테스트 (`policy_test.go`) — Validate 성공/실패, ResolveReviewer 전체 매트릭스 (critical→user, high→review, medium→빈, low→빈, rules 없을 때 default, 빈 policy)

#### Task C-2: Manager.Create에 ReviewPolicy 통합 (internal/task/manager.go) — 0.5h

- **C-2-1**: `Create()` 파라미터에 `policy *ReviewPolicy` 추가 (nil 허용 — 하위 호환)
- **C-2-2**: reviewer가 명시적으로 지정되지 않았고 policy가 있으면 `policy.ResolveReviewer(priority)`로 자동 지정
- **C-2-3**: 명시적 reviewer가 있으면 policy 무시 (명시적 지정 우선)
- **C-2-4**: Manager 테스트에 policy 통합 시나리오 추가

#### Task C-3: Template 구조체에 ReviewPolicy 추가 (internal/cli/agents.go, internal/onboard/template_definition.go) — 0.75h

- **C-3-1**: `templateDefinition` 구조체에 `ReviewPolicy *task.ReviewPolicy` 필드 추가 (json tag: `"review_policy,omitempty"`)
- **C-3-2**: `onboard.TemplateDefinition` 구조체에 동일 필드 추가
- **C-3-3**: `templateDefinitionFromOnboard()` 변환 함수에 ReviewPolicy 매핑 추가
- **C-3-4**: 내장 템플릿 (`company`, `oh-my-opencode`)에 기본 review_policy 정의 추가:
  - `company`: `require_review_above: "high", default_reviewer: "user", rules: [{critical, user}, {high, review}]`
  - `oh-my-opencode`: `require_review_above: "high", default_reviewer: "user", rules: [{critical, user}, {high, review}]`
- **C-3-5**: 기존 templateDefinition 테스트에 review_policy 포함 검증 추가

#### Task C-4: Template 로딩/저장에 ReviewPolicy 반영 (internal/cli/template_store.go) — 0.5h

- **C-4-1**: `loadCustomTemplate()` — JSON 언마샬 시 review_policy 자동 로드됨 (구조체 필드 추가만으로 충분, 검증)
- **C-4-2**: `saveCustomTemplate()` — JSON 마샬 시 review_policy 자동 저장됨 (검증)
- **C-4-3**: `validateCustomTemplateDefinition()`에 review_policy 검증 추가 — policy가 있으면 `policy.Validate()` 호출
- **C-4-4**: 테스트에 review_policy 포함된 custom template 저장/로드 라운드트립 검증 추가

#### Task C-5: Template Import에 ReviewPolicy 지원 (internal/cli/template_import.go) — 0.5h

- **C-5-1**: `templateDefinitionFromMap()` — `review_policy` 키 파싱, map → ReviewPolicy 변환
- **C-5-2**: YAML/JSON import 시 review_policy 포함 template 라운드트립 테스트
- **C-5-3**: `template export`에서 review_policy 포함 YAML 출력 검증

#### Task C-6: CLI task create에서 ReviewPolicy 적용 (internal/cli/task.go) — 0.5h

- **C-6-1**: `task create` 실행부에서 현재 프로젝트의 활성 템플릿 로드
- **C-6-2**: 템플릿에 review_policy가 있으면 `policy.ResolveReviewer(priority)` 호출
- **C-6-3**: `--reviewer` 명시 지정이 있으면 policy 결과 무시
- **C-6-4**: CLI 테스트에 policy 기반 reviewer 자동 지정 시나리오 추가

#### Task C-7: MCP create_task에서 ReviewPolicy 적용 (internal/mcp/handler.go) — 0.5h

- **C-7-1**: `handleCreateTask()`에서 project의 활성 템플릿 review_policy 로드
- **C-7-2**: reviewer 파라미터가 없으면 policy 기반 자동 지정
- **C-7-3**: MCP 테스트에 policy 기반 reviewer 자동 지정 시나리오 추가

---

### Phase D: MCP update_task Tool (신규) — ~1.5h

#### Task D-1: update_task 도구 정의 (internal/mcp/tools.go) — 0.25h

- **D-1-1**: `update_task` 도구 스키마 정의 — required: `task_id`, `status`; optional: `result`, `project`
- **D-1-2**: 도구 목록에 등록

#### Task D-2: update_task 핸들러 구현 (internal/mcp/handler.go) — 0.75h

- **D-2-1**: `handleUpdateTask()` 함수 구현
- **D-2-2**: `Manager.UpdateStatus()` 호출 (ValidateTransition + reviewer 검증 포함)
- **D-2-3**: 에러 시 적절한 JSON-RPC error response (상태 전이 실패, 존재하지 않는 태스크 등)
- **D-2-4**: 성공 시 updated task 정보 반환

#### Task D-3: approve_task, reject_task 도구 (internal/mcp/tools.go, handler.go) — 0.5h

- **D-3-1**: `approve_task` 스키마 정의 — required: `task_id`; optional: `result`, `project`
- **D-3-2**: `reject_task` 스키마 정의 — 동일
- **D-3-3**: `handleApproveTask()` — `Manager.ApproveTask()` 호출
- **D-3-4**: `handleRejectTask()` — `Manager.RejectTask()` 호출
- **D-3-5**: 도구 목록에 등록, MCP 테스트 추가

---

### Phase E: Test Coverage — ~3h

#### Task E-1: model_test.go 강화 — 1h

- **E-1-1**: ValidatePriority 전체 매트릭스 (유효 4개, 무효 8개+, 대소문자, 빈 문자열, 공백)
- **E-1-2**: NormalizePriority 테스트 (대소문자 혼합, 앞뒤 공백)
- **E-1-3**: ComparePriority / PriorityAtLeast 전체 조합 테스트
- **E-1-4**: ValidateTransition 기존 테스트에 `blocked → completed` 추가
- **E-1-5**: ValidateTransitionWithReviewer 전체 시나리오 (reviewer 유/무 × from/to 조합)

#### Task E-2: manager_test.go 강화 — 0.75h

- **E-2-1**: Create with valid/invalid priority 테스트
- **E-2-2**: Create with reviewer 테스트
- **E-2-3**: Create with policy (reviewer 자동 지정) 테스트
- **E-2-4**: UpdateStatus with reviewer — in_progress → completed 차단 → blocked 전환 테스트
- **E-2-5**: ApproveTask / RejectTask 정상/에러 테스트

#### Task E-3: db/task_test.go 강화 — 0.5h

- **E-3-1**: InsertTask — reviewer 필드 저장/조회 테스트
- **E-3-2**: InsertTask — ID가 비어있을 때만 생성 확인 (B2 버그 수정 검증)
- **E-3-3**: UpdateTask — reviewer 필드 업데이트 테스트
- **E-3-4**: 마이그레이션 후 기존 데이터 reviewer 기본값 확인

#### Task E-4: policy_test.go 추가 — 0.5h

- **E-4-1**: ReviewPolicy.Validate() — 유효/무효 정책 (잘못된 priority 임계값, 빈 reviewer 등)
- **E-4-2**: ResolveReviewer — 전체 priority × policy 조합 매트릭스
- **E-4-3**: nil policy 안전성 테스트
- **E-4-4**: 빈 policy (모든 필드 비어있음) 테스트

#### Task E-5: E2E 테스트 추가 (cmd/agentcom/e2e_test.go) — 0.25h

- **E-5-1**: 무효 priority로 `task create` 실행 → 에러 확인
- **E-5-2**: reviewer가 있는 태스크의 전체 lifecycle (create → in_progress → blocked → completed) 확인
- **E-5-3**: `task approve` / `task reject` 명령 E2E 확인

---

### Phase F: Documentation — ~2.5h

#### Task F-1: README 업데이트 (README.md + 3개 번역) — 1.5h

- **F-1-1**: Task 명령 섹션에 `--reviewer`, `--priority` 허용값, `task approve`, `task reject` 추가
- **F-1-2**: MCP 도구 목록에 `update_task`, `approve_task`, `reject_task` 추가
- **F-1-3**: Template 섹션에 `review_policy` 스키마 설명 추가
- **F-1-4**: Priority 정의 테이블 추가 (4단계 + 기준 + 예시)
- **F-1-5**: Human Operator 섹션에 리뷰 워크플로우 예제 추가
- **F-1-6**: README.ko.md, README.ja.md, README.zh.md 동일 반영

#### Task F-2: AGENTS.md 업데이트 — 0.5h

- **F-2-1**: 핵심 설계 원칙에 "Priority는 4단계 고정 enum" 추가
- **F-2-2**: 태스크 관리 설명에 reviewer + review policy 언급
- **F-2-3**: PH10 완료 반영

#### Task F-3: MEMORY.md 업데이트 — 0.5h

- **F-3-1**: 현재 Phase를 PH10 완료로 변경
- **F-3-2**: 완료된 태스크 목록에 PH10 추가
- **F-3-3**: 설계 결정 로그에 PH10 결정사항 추가
- **F-3-4**: 다음 작업을 PH5~PH9 또는 다음 priority로 갱신

---

## 5. 의존성 그래프

```
Phase A ─────────────────────────────────────────────────────────┐
  A-1 (priority constants) ──┬─> A-2 (manager) ──> A-3 (CLI)   │
                             │                 ──> A-4 (MCP)    │
                             └─> A-5 (comparisons)              │
                                                                │
Phase B ─────────────────────────────────────────────────────────┤
  B-1 (migration) ──> B-2 (task struct) ──> B-3 (manager)      │
  B-4 (state machine) ──────────────────────> B-3              │
                                              B-3 ──> B-5 (CLI)│
                                                                │
Phase C ─────────────────────────────────────────────────────────┤
  A-5 ──> C-1 (policy type) ──> C-2 (manager integration)      │
                                C-3 (template struct)           │
                                C-3 ──> C-4 (store)            │
                                C-3 ──> C-5 (import)           │
  B-5 + C-2 ──> C-6 (CLI apply)                                │
  B-5 + C-2 ──> C-7 (MCP apply)                                │
                                                                │
Phase D ─────────────────────────────────────────────────────────┤
  B-3 ──> D-1 (tool def) ──> D-2 (handler)                     │
  B-3 ──> D-3 (approve/reject tools)                            │
                                                                │
Phase E ──────────────────────────────────── (all above) ───────┤
  E-1 (model tests) ← A-1, A-5, B-4                            │
  E-2 (manager tests) ← A-2, B-3, C-2                          │
  E-3 (db tests) ← B-1, B-2                                    │
  E-4 (policy tests) ← C-1                                     │
  E-5 (e2e tests) ← B-5, C-6, D-2, D-3                        │
                                                                │
Phase F ──────────────────────────────────── (all above) ───────┘
  F-1 (README), F-2 (AGENTS.md), F-3 (MEMORY.md)
```

**Phase A와 B는 독립적으로 병렬 가능** (A는 model.go priority 영역, B는 DB/state machine 영역).
Phase C는 A-5와 B-5에 의존. Phase D는 B-3에 의존. Phase E/F는 전체 의존.

---

## 6. 영향 받는 파일 목록

### 수정 파일 (15개)

| 파일 | Phase | 변경 내용 |
|------|-------|-----------|
| `internal/task/model.go` | A, B | Priority 상수/검증, 상태 전이 추가 |
| `internal/task/manager.go` | A, B, C | Priority 검증, reviewer 로직, policy 통합 |
| `internal/db/migrations.go` | B | reviewer 컬럼 마이그레이션 |
| `internal/db/task.go` | B | Task 구조체 + reviewer 필드 + InsertTask ID 버그 수정 |
| `internal/cli/task.go` | A, B, C | --reviewer/--priority 플래그, approve/reject, raw SQL → Manager 호출 |
| `internal/cli/agents.go` | C | templateDefinition에 ReviewPolicy, 내장 템플릿 policy 추가 |
| `internal/cli/template_store.go` | C | review_policy 검증 추가 |
| `internal/cli/template_import.go` | C | review_policy 파싱 추가 |
| `internal/onboard/template_definition.go` | C | TemplateDefinition에 ReviewPolicy 추가 |
| `internal/mcp/handler.go` | A, C, D | Priority 검증, update/approve/reject 핸들러 |
| `internal/mcp/tools.go` | D | update_task, approve_task, reject_task 도구 정의 |
| `README.md` | F | Priority, reviewer, review_policy, 새 명령 문서 |
| `README.ko.md` | F | 동일 |
| `README.ja.md` | F | 동일 |
| `README.zh.md` | F | 동일 |

### 신규 파일 (2개)

| 파일 | Phase | 내용 |
|------|-------|------|
| `internal/task/policy.go` | C | ReviewPolicy 타입, Validate, ResolveReviewer |
| `internal/task/policy_test.go` | E | ReviewPolicy 단위 테스트 |

### 수정 테스트 파일 (5개)

| 파일 | Phase |
|------|-------|
| `internal/task/model_test.go` | E |
| `internal/task/manager_test.go` | E |
| `internal/db/task_test.go` | E |
| `internal/db/migrations_test.go` | B |
| `cmd/agentcom/e2e_test.go` | E |

---

## 7. 성공 기준

| # | 기준 | 검증 방법 |
|---|------|-----------|
| S1 | `task create --priority urgent` → 에러 | CLI 테스트 + E2E |
| S2 | `task create --priority HIGH` → 정규화되어 "high"로 저장 | DB 테스트 |
| S3 | reviewer 있는 태스크에서 `in_progress → completed` 시도 → `blocked`로 전환 | Manager 테스트 |
| S4 | reviewer 없는 태스크에서 `in_progress → completed` → 정상 동작 | Manager 테스트 |
| S5 | `blocked → completed` 전이 허용 | Model 테스트 |
| S6 | `task approve <id>` → `blocked → completed` | CLI 테스트 + E2E |
| S7 | `task reject <id>` → `blocked → failed` | CLI 테스트 + E2E |
| S8 | 템플릿 review_policy로 high 태스크에 reviewer 자동 지정 | Manager 테스트 |
| S9 | review_policy 없는 템플릿 → 기존 동작 유지 (하위 호환) | 기존 테스트 통과 |
| S10 | MCP update_task, approve_task, reject_task 작동 | MCP 테스트 |
| S11 | CLI raw SQL 버그 3개 수정 (B1, B3, B4) | CLI 테스트 |
| S12 | DB InsertTask double ID 버그 수정 (B2) | DB 테스트 |
| S13 | `go test ./...` 전체 통과 | CI |
| S14 | `go build ./...` 통과 | CI |

---

## 8. 리스크

| 리스크 | 영향 | 완화 |
|--------|------|------|
| CLI raw SQL → Manager 호출 변경 시 기존 동작 깨짐 | 높음 | TDD — 변경 전 기존 동작 테스트 먼저 추가 |
| DB 마이그레이션 실패 (기존 DB 호환) | 높음 | in-memory + 파일 기반 마이그레이션 테스트 양쪽 추가 |
| review_policy 파싱 실패로 template load 깨짐 | 중간 | omitempty + nil 체크로 하위 호환 보장 |
| MCP 새 도구 추가로 initialize 핸드셰이크 변경 | 낮음 | tools/list에 추가만, 기존 도구 변경 없음 |
| `in_progress → blocked` 자동 전환이 직관적이지 않을 수 있음 | 중간 | 에러 메시지에 "reviewer가 있어 리뷰 필요" 안내 포함 |

---

## 9. 일정 추정

| Phase | 추정 시간 | 비고 |
|-------|-----------|------|
| A: Priority Validation | 2.5h | A-1, A-2 병렬 가능 |
| B: Reviewer + State Machine | 3h | B-1, B-4 병렬 가능 |
| C: Review Policy | 3.5h | C-3, C-4, C-5 병렬 가능 |
| D: MCP update_task | 1.5h | |
| E: Tests | 3h | Phase별 TDD 진행 시 이 시간 감소 |
| F: Documentation | 2.5h | |
| **합계** | **~16h** | TDD 병행 시 실질 ~14h |

---

## 10. 업계 포지셔닝 비교

| 기능 | agentcom (PH10) | CrewAI | AutoGen | LangGraph | Temporal | Camunda |
|------|-----------------|--------|---------|-----------|----------|---------|
| Priority → Review Gate | ✅ 선언적 | ❌ | ❌ | ❌ | ⚠️ 수동 | ✅ DMN |
| 4단계 Priority | ✅ 고정 enum | ❌ 없음 | ❌ 없음 | ❌ 없음 | ❌ 없음 | ✅ 유연 |
| Human Review | ✅ user pseudo-agent | ⚠️ input() | ⚠️ input() | ⚠️ interrupt | ✅ signal | ✅ user task |
| Template-level Policy | ✅ review_policy | ❌ | ❌ | ❌ | ❌ | ✅ BPMN |
| Review Timeout | ❌ 무한 대기 | N/A | N/A | N/A | ✅ 선택적 | ✅ 에스컬레이션 |

**차별화**: 어떤 멀티에이전트 프레임워크도 natively priority → review gate를 연결하지 않음. PagerDuty의 SEV 경계 패턴이 가장 유사한 아날로그.

---

## 부록 A: 현재 태스크 DB 스키마 (참조)

```sql
CREATE TABLE IF NOT EXISTS tasks (
    id          TEXT PRIMARY KEY,
    title       TEXT NOT NULL,
    description TEXT,
    status      TEXT NOT NULL DEFAULT 'pending',
    priority    TEXT NOT NULL DEFAULT 'medium',
    assigned_to TEXT,
    created_by  TEXT,
    blocked_by  TEXT DEFAULT '[]',
    result      TEXT,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_assignee ON tasks(assigned_to);
```

## 부록 B: PH10 후 스키마 (예상)

```sql
CREATE TABLE IF NOT EXISTS tasks (
    id          TEXT PRIMARY KEY,
    title       TEXT NOT NULL,
    description TEXT,
    status      TEXT NOT NULL DEFAULT 'pending',
    priority    TEXT NOT NULL DEFAULT 'medium',
    reviewer    TEXT DEFAULT '',              -- 추가
    assigned_to TEXT,
    created_by  TEXT,
    blocked_by  TEXT DEFAULT '[]',
    result      TEXT,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
```

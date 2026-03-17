# PH10: 리뷰 시스템 설계 종합 분석

> 상태: **설계 검토 완료** — 사용자 결정 대기
> 작성일: 2025-03-17
> 기반: 외부 프레임워크 비교 분석 (CrewAI, AutoGen, LangGraph, Temporal, Camunda, GitHub Actions, JIRA, Linear, OpenHands)

---

## 분석 대상 3가지 설계 질문

1. **리뷰어 모델**: 에이전트도 리뷰어가 될 수 있되, user 리뷰가 필요하면 에이전트 리뷰를 포함하는 cascading 구조
2. **필수 리뷰 게이트**: 모든 태스크의 `completed` 전환을 `blocked → completed`로만 가능하게 할지
3. **타임아웃 정책**: 승인 타임아웃 없이 최종 리뷰 후 진행 / 에이전트 태스크는 리뷰 없이 가능 / user 리뷰는 타임아웃 없이

---

## 1. 리뷰어 모델: Cascading Agent → Human Review

### 1.1 외부 프레임워크 비교

| 프레임워크 | 리뷰어 모델 | 핵심 패턴 |
|-----------|------------|----------|
| **CrewAI** | `LLMGuardrail` — 별도 에이전트가 output 검증, 실패 시 retry loop (max N), 최종 `human_input=True` | 에이전트 리뷰 → 인간 에스컬레이션 |
| **AutoGen** | `SocietyOfMindAgent` — writer + editor 내부 팀, `TextMentionTermination("APPROVE")` | 에이전트 간 peer review |
| **AutoGen** | `SelectorGroupChat` — custom function으로 동적 리뷰어 선정 | 역할 기반 선택 |
| **LangGraph** | Conditional edge + AI reviewer node → `interrupt()` for human | AI 필터 → 인간 게이트 |
| **OpenHands** | Trajectory-level critic model (non-blocking, post-hoc selection) | 비동기 품질 평가 |

### 1.2 공통 패턴: **Cascading Review**

모든 프로덕션 프레임워크에서 관찰되는 지배적 패턴:

```
에이전트 작업 완료
  → AI 리뷰어가 자동 검증 (1-3초, 동기)
    → 통과: 완료 또는 인간 리뷰로 에스컬레이션
    → 실패: 에이전트에게 재작업 요청 (max N회)
      → N회 초과: 인간 에스컬레이션
```

**핵심 인사이트**: AI 리뷰는 인간 리뷰의 **대체가 아니라 필터**. 인간에게 도달하는 리뷰 건수를 줄이는 역할.

### 1.3 agentcom 적용 설계

#### 스키마

```sql
ALTER TABLE tasks ADD COLUMN reviewer TEXT;       -- 최종 리뷰어 (agent ID 또는 "user")
ALTER TABLE tasks ADD COLUMN review_chain TEXT;    -- JSON: ["agt_ai_reviewer", "user"] (optional, cascading 순서)
```

**`reviewer`**: 현재 리뷰 담당자. 단일 값.
**`review_chain`** (선택): cascading 순서를 명시할 경우. 에이전트 리뷰 통과 시 다음 리뷰어로 자동 전환.

#### 흐름

```
# Case 1: 에이전트만 리뷰 (자동)
task create --reviewer agent-qa     → 에이전트 리뷰만, 통과 시 completed

# Case 2: 인간만 리뷰
task create --reviewer user         → 인간 리뷰만

# Case 3: Cascading (에이전트 → 인간)
task create --reviewer user --review-chain '["agent-qa","user"]'
  → agent-qa가 먼저 리뷰 → 통과 시 reviewer가 user로 전환 → 인간 리뷰
  → agent-qa 실패 시 → 에이전트에게 재작업 반환
```

#### 권장사항

**Phase 1 (PH10)**: `reviewer` 컬럼만 구현. 단일 리뷰어 (에이전트 OR 인간).
**Phase 2 (PH11+)**: `review_chain` 추가하여 cascading 지원. AI 리뷰 자동화.

**이유**: Cascading은 AI reviewer agent의 자동 판단 로직이 필요하고, 이는 agentcom의 현재 범위(통신 인프라)를 넘어선다. 단일 reviewer로 기본 게이트를 확보한 후, 상위 레이어에서 cascading 오케스트레이션을 구현하는 것이 안전하다.

---

## 2. 필수 리뷰 게이트: `blocked → completed`만 허용할지

### 2.1 외부 프레임워크 비교

| 시스템 | 필수 리뷰? | 정책 |
|--------|-----------|------|
| **GitHub Actions** | **선택적** — environment-scoped job만 review gate, build/test는 자동 | per-environment protection rules |
| **JIRA** | **선택적** — `In Review` 상태는 workflow별 optional | issue type별 워크플로우 |
| **Linear** | **선택적** — `In Review`는 custom state, 팀별 설정 | 팀 정책 |
| **Camunda** | **선택적** — Service Task는 auto-complete, User Task만 인간 필요 | task type별 분기 |
| **Temporal** | **조건부** — risk-based: 고위험이면 `wait_for_signal`, 아니면 auto-approve | 런타임 조건 |

### 2.2 핵심 발견: **어떤 프로덕션 시스템도 ALL tasks에 필수 리뷰를 강제하지 않는다**

> **"Approval fatigue"**: 모든 태스크에 리뷰를 요구하면 리뷰어가 기계적으로 승인하게 됨. 진짜 중요한 리뷰의 품질이 하락하는 역효과.
>
> — GitHub, JIRA, Camunda 문서 공통 경고

### 2.3 agentcom 현재 상태 영향 분석

**현재 상태 머신** (`model.go`):
```go
StatusInProgress: {
    StatusCompleted: {},  // ← 이 전환을 제거하면?
    StatusFailed:    {},
    StatusBlocked:   {},
}
StatusBlocked: {
    StatusInProgress: {},
    StatusPending:    {},
    StatusCancelled:  {},
    // StatusCompleted: {} ← 이 전환을 추가해야 함
}
```

**Option A: `in_progress → completed` 제거 (모든 태스크 필수 리뷰)**

영향:
- `manager_test.go` 2개 테스트 깨짐 (line 50, 122-124)
- 모든 에이전트 워크플로우가 `in_progress → blocked → completed`를 강제당함
- reviewer가 없는 단순 태스크도 반드시 blocked를 거쳐야 함
- **CLI `task update --status completed`가 in_progress에서 직접 불가** → 모든 기존 사용자 스크립트 파손

**Option B: `reviewer` 필드 기반 선택적 게이트 (권장)**

```go
// manager.go — UpdateStatus 내
if to == StatusCompleted && from == StatusInProgress {
    task := getTask(id)
    if task.Reviewer != "" {
        return fmt.Errorf("task has reviewer %q; must transition to blocked for review first", task.Reviewer)
    }
    // reviewer 없으면 직접 completed 허용
}
```

영향:
- `in_progress → completed` 전환 유지 (reviewer 없는 태스크)
- `in_progress → completed` 차단 (reviewer 있는 태스크 → blocked 강제)
- `blocked → completed` 전환 추가 (리뷰 승인)
- 기존 테스트/스크립트 비파손
- reviewer 있는 태스크만 리뷰 게이트 적용

**Option C: `in_progress → completed` 전면 제거 + system auto-approve (하이브리드)**

```go
// reviewer 없는 태스크: in_progress → blocked (system auto-transition) → completed (system auto-approve)
// reviewer 있는 태스크: in_progress → blocked → (인간/에이전트 리뷰) → completed
```

영향:
- 모든 태스크가 일관된 흐름 (`blocked` 거침)
- reviewer 없으면 시스템이 즉시 자동 approve → 실질적으로 직접 completed와 동일
- 감사 로그(audit trail)에 모든 태스크의 blocked→completed 기록이 남음
- **복잡도 증가**: auto-approve 로직 필요, 이벤트 2개 발생 (사실상 불필요한 중간 단계)

### 2.4 권장: **Option B — 선택적 리뷰 게이트**

| 기준 | Option A (전면 필수) | Option B (선택적) | Option C (하이브리드) |
|------|---------------------|-------------------|---------------------|
| 기존 호환성 | ❌ 파손 | ✅ 유지 | ⚠️ 부분 변경 |
| 리뷰 피로도 | ❌ 높음 | ✅ 낮음 | ✅ 낮음 |
| 구현 복잡도 | 낮음 | **낮음** | 중간 |
| 감사 추적 | ✅ 전수 | ⚠️ reviewer 태스크만 | ✅ 전수 |
| 업계 정합성 | ❌ 없음 | ✅ GitHub/JIRA/Camunda | ⚠️ 유사한 사례 적음 |

**결론**:
- `reviewer` 필드가 있는 태스크: `in_progress → completed` **차단**, `in_progress → blocked → completed` 강제
- `reviewer` 필드가 없는 태스크: `in_progress → completed` **허용** (기존 동작 유지)
- `blocked → completed` 전환 **추가** (model.go에 1줄)

---

## 3. 타임아웃 정책

### 3.1 외부 프레임워크 비교

| 시스템 | 인간 리뷰 타임아웃 | 에이전트 리뷰 타임아웃 | 기본 정책 |
|--------|-------------------|---------------------|----------|
| **Temporal** | **무한 대기** (기본) | N/A (동기 실행) | `wait_for_signal` — 타임아웃 없이 시그널 대기 |
| **Camunda** | **무한 대기** (기본) + optional 에스컬레이션 타이머 | N/A (Service Task = 동기) | non-interrupting timer → 알림, interrupting timer → 재할당 |
| **GitHub PRs** | **타임아웃 없음** | N/A | PR 리뷰는 무한 대기. 자동 승인 없음. |
| **LangGraph** | **무한 대기** | N/A (동기 노드) | `interrupt()` → 프로세스 일시정지, 인간 입력까지 대기 |
| **CrewAI** | **무한 대기** | 동기 (1-3초) | `human_input=True` → 무한 대기 |
| **Linear/JIRA** | **타임아웃 없음** | N/A | 이슈 상태는 명시적 전환만 |

### 3.2 핵심 발견

> **어떤 성숙한 시스템도 인간 리뷰에 타임아웃을 적용하지 않는다.**
>
> 자동 승인은 리뷰 게이트의 존재 이유를 부정한다.

**에이전트 리뷰**는 동기/인라인 (1-3초):
- LLMGuardrail (CrewAI): 함수 호출 → 즉시 반환
- AI reviewer node (LangGraph): 노드 실행 → 즉시 다음 edge
- 타임아웃 개념 자체가 불필요 (동기 호출이므로)

**인간 리뷰**는 비동기/무기한:
- 인간이 응답할 때까지 태스크는 `blocked` 상태로 유지
- 다른 태스크는 영향 없이 병렬 진행 가능
- 시스템 리소스 소비 없음 (단순 DB 상태)

### 3.3 Camunda 에스컬레이션 패턴 (참고)

타임아웃 대신 **에스컬레이션 타이머**를 사용하는 유일한 프레임워크:

```
인간 리뷰 대기 중 (blocked)
  → 24h 경과: non-interrupting timer 발동 → 알림 재전송 (리마인더)
  → 72h 경과: interrupting timer 발동 → 다른 리뷰어에게 재할당
```

- 태스크를 자동 승인하지 않음
- 타임아웃이 아니라 에스컬레이션
- agentcom에서 구현하려면 별도 스케줄러 필요 → 현재 범위 밖

### 3.4 agentcom 적용 설계

#### 즉시 구현 (PH10)

```
인간 리뷰: 타임아웃 없음. blocked 상태로 무한 대기. user approve/reject만.
에이전트 리뷰: 동기. reviewer가 에이전트면 approve CLI/MCP 호출로 즉시 처리.
reviewer 없는 태스크: 리뷰 불필요. in_progress → completed 직접 가능.
```

#### 향후 확장 (PH11+)

```sql
ALTER TABLE tasks ADD COLUMN review_deadline TEXT;   -- ISO 8601 timestamp (optional)
ALTER TABLE tasks ADD COLUMN escalation_to TEXT;      -- 에스컬레이션 대상 agent ID
```

- `review_deadline` 설정 시 → 스케줄러가 주기적 체크
- 기한 초과 → `escalation_to`에게 재할당 또는 알림 재전송
- **자동 승인 아님** — 에스컬레이션만

### 3.5 권장 정책 요약

| 리뷰어 타입 | 타임아웃 | 행동 |
|------------|---------|------|
| **없음** (reviewer=NULL) | N/A | `in_progress → completed` 직접 가능 |
| **에이전트** (reviewer=agt_xxx) | 없음 (동기 호출) | 에이전트가 approve/reject CLI/MCP 호출. 미응답 시 = 에이전트 장애 (별도 처리) |
| **인간** (reviewer=user) | **없음** (무한 대기) | `user approve` / `user reject`까지 blocked 유지 |
| **인간 + 에스컬레이션** (PH11+) | 선택적 | `review_deadline` 초과 시 리마인더 또는 재할당 |

---

## 4. 종합 설계 권장안

### 4.1 상태 머신 변경

```go
// model.go — 변경사항

StatusInProgress: {
    StatusCompleted: {},  // 유지 (reviewer 없는 태스크용)
    StatusFailed:    {},
    StatusBlocked:   {},
},
StatusBlocked: {
    StatusInProgress: {},
    StatusPending:    {},
    StatusCancelled:  {},
    StatusCompleted:  {},  // 추가 (리뷰 승인)
},
```

**+ Manager.UpdateStatus() 가드 추가**:
```go
// reviewer가 있는 태스크의 in_progress → completed 차단
if to == StatusCompleted && from == StatusInProgress && task.Reviewer != "" {
    return ErrReviewRequired  // "task has reviewer; must go through blocked state"
}
```

### 4.2 스키마 변경

```sql
-- PH10 (최소)
ALTER TABLE tasks ADD COLUMN reviewer TEXT;
CREATE INDEX IF NOT EXISTS idx_tasks_reviewer ON tasks(reviewer);

-- PH11+ (선택)
ALTER TABLE tasks ADD COLUMN review_chain TEXT;     -- JSON array
ALTER TABLE tasks ADD COLUMN review_deadline TEXT;   -- ISO 8601
ALTER TABLE tasks ADD COLUMN escalation_to TEXT;     -- agent ID
```

### 4.3 CLI/MCP 변경

| 항목 | 변경 | Phase |
|------|------|-------|
| `task create --reviewer <agent\|user>` | 새 플래그 | PH10 |
| `create_task` MCP에 `reviewer` 파라미터 | 추가 | PH10 |
| `update_task` MCP 도구 | **신규** (기존 갭 해소) | PH10 |
| `user approve <task-id>` | 새 CLI 명령 | PH10 |
| `user reject <task-id> --reason "..."` | 새 CLI 명령 | PH10 |
| `user tasks` | 새 CLI 명령 (blocked + reviewer=user 조회) | PH10 |
| assignee human 가드 | CLI + MCP 양쪽 | PH10 |
| `task create --review-chain '["agt_qa","user"]'` | Cascading 지원 | PH11+ |
| 에스컬레이션 타이머 | 스케줄러 + 타이머 | PH11+ |

### 4.4 흐름 요약

```
[reviewer 없는 태스크]
  pending → assigned → in_progress → completed          (기존과 동일)

[reviewer 있는 태스크 — 에이전트 리뷰]
  pending → assigned → in_progress → blocked             (에이전트 작업 완료)
    → 리뷰 에이전트가 approve → completed                 (동기, 1-3초)
    → 리뷰 에이전트가 reject → in_progress               (재작업)

[reviewer 있는 태스크 — 인간 리뷰]
  pending → assigned → in_progress → blocked             (에이전트 작업 완료)
    → 시스템이 user inbox에 자동 알림
    → 인간이 user approve → completed                    (무한 대기)
    → 인간이 user reject → in_progress                   (재작업)

[reviewer 있는 태스크 — Cascading (PH11+)]
  pending → assigned → in_progress → blocked             (에이전트 작업 완료)
    → AI 리뷰어가 자동 검증 (통과) → reviewer를 user로 전환
    → 인간이 user approve → completed
    → AI 리뷰어가 자동 검증 (실패) → in_progress (재작업)
```

---

## 5. 외부 프레임워크 대비 agentcom 포지셔닝

| 특성 | CrewAI | AutoGen | LangGraph | Temporal | Camunda | **agentcom (PH10)** |
|------|--------|---------|-----------|----------|---------|---------------------|
| 인간 리뷰 | `human_input=True` | UserProxy | `interrupt()` | Signal | User Task | `reviewer` 필드 + `user approve` |
| 에이전트 리뷰 | LLMGuardrail | SocietyOfMind | AI reviewer node | Worker | Service Task | `reviewer=agt_xxx` + `approve` |
| Cascading | ✅ (guardrail→human) | ✅ (내부팀) | ✅ (conditional edge) | ❌ | ❌ | ⏳ PH11+ |
| 필수 리뷰 | ❌ 선택적 | ❌ 선택적 | ❌ 선택적 | ❌ 조건부 | ❌ task type별 | ✅ **reviewer 필드 기반 선택적** |
| 타임아웃 | ❌ 없음 | ❌ 없음 | ❌ 없음 | ❌ 기본 없음 | ⏳ 에스컬레이션 | ❌ 없음 (PH11+ 에스컬레이션) |

---

## 6. 리스크 및 완화

| 리스크 | 영향 | 완화 |
|--------|------|------|
| CLI `task update`가 ValidateTransition()을 우회 (line 218-222) | reviewer 가드가 CLI에서 무효화 | **PH10-05에서 반드시 수정** — CLI도 Manager.UpdateStatus() 경유하도록 |
| `update_task` MCP 부재 (기존 갭) | 에이전트가 태스크 상태를 MCP로 변경 불가 | **PH10-03에서 해소** |
| reviewer 없이 `in_progress → completed` 직접 가능 | 실수로 리뷰 우회 가능 | reviewer 있으면 차단 + 로그 경고 |
| 인간 리뷰 무한 대기 시 태스크 장기 방치 | 워크플로우 정체 | PH11+ 에스컬레이션 타이머로 해결 |

---

## 7. 결론

### 즉시 구현 (PH10)

1. **`reviewer` 컬럼 추가** — 단일 리뷰어, 에이전트 또는 인간
2. **`blocked → completed` 전환 추가** — 리뷰 승인 경로
3. **`in_progress → completed` 조건부 차단** — reviewer 있으면 blocked 거쳐야 함
4. **`update_task` MCP 도구 신규 추가** — 기존 갭 해소
5. **CLI `task update`가 Manager 경유하도록 수정** — 가드 우회 방지
6. **`user approve/reject/tasks` CLI 추가** — 인간 리뷰 인터페이스
7. **인간 리뷰: 타임아웃 없음** — blocked 상태로 무한 대기

### 향후 확장 (PH11+)

1. **`review_chain`** — Cascading agent → human review
2. **에스컬레이션 타이머** — 장기 방치 방지
3. **AI 자동 리뷰어** — LLMGuardrail 패턴 적용

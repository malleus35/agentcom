# PH10: User-Task Approver 모델 설계 분석

> 상태: **설계 검토 중** (구현 전)
> 작성일: 2025-03-17

---

## 1. 배경 및 현재 상태

### 현재 코드 구조

**상태 머신** (`internal/task/model.go`):
```
pending → assigned, in_progress, cancelled
assigned → in_progress, blocked, pending, cancelled
in_progress → completed, failed, blocked
blocked → in_progress, pending, cancelled
```
7개 상태, terminal: `completed`, `failed`, `cancelled`

**Task 스키마** (`internal/db/migrations.go`):
```sql
CREATE TABLE tasks (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    priority TEXT NOT NULL DEFAULT 'medium',
    assigned_to TEXT,      -- 에이전트 ID (FK 없음)
    created_by TEXT,       -- 에이전트 ID (FK 없음)
    blocked_by TEXT DEFAULT '[]',  -- JSON array of task IDs
    result TEXT,
    created_at TEXT,
    updated_at TEXT
);
```

**현재 문제**: Task 시스템에 `type="human"` 가드가 전혀 없음
- `task create --assign user` → 성공 (user pseudo-agent ID로 resolve됨)
- `task delegate --to user` → 성공
- Message 시스템은 `human` 타입을 broadcast에서 제외하는 가드가 있음
- Task에는 동등한 가드 없음

---

## 2. Approver 모델 상세 설계

### 2.1 핵심 개념

```
에이전트가 태스크를 완료한 후 → 인간 승인이 필요하면 → blocked 상태 전환
→ 인간 인박스에 자동 알림 → 인간이 approve/reject → 태스크 상태 전환
```

인간은 **태스크 ASSIGNEE가 아니라 REVIEWER**. 작업을 직접 수행하지 않고, 에이전트가 완료한 결과를 게이팅한다.

### 2.2 상태 머신 전환 규칙 분석

#### Option A: 기존 `blocked` 상태 재활용

현재 `blocked`는 이미 존재하고, `in_progress → blocked`와 `blocked → in_progress/pending/cancelled` 전환이 정의되어 있음.

```
in_progress → blocked (reviewer 지정된 태스크 완료 시)
blocked → completed (인간 approve)
blocked → in_progress (인간 reject + 에이전트 재작업)
blocked → cancelled (인간 cancel)
```

**문제**: `blocked → completed` 전환이 현재 **없음**.

현재 validTransitions:
```go
StatusBlocked: {
    StatusInProgress: {},  // 있음
    StatusPending:    {},  // 있음
    StatusCancelled:  {},  // 있음
    // StatusCompleted 없음!
}
```

**변경 필요**: `blocked → completed` 전환 추가.

이건 논란의 여지가 없는 변경임:
- Approver 모델 없이도 `blocked → completed`는 합리적 (차단 해소 후 바로 완료)
- 기존 `blocked_by` (태스크 간 차단)에서도 유효한 전환

#### Option B: 새 상태 `awaiting_review` 추가

```
in_progress → awaiting_review (에이전트가 작업 완료 + reviewer 있음)
awaiting_review → completed (approve)
awaiting_review → in_progress (reject)
awaiting_review → cancelled (cancel)
```

**장점**: `blocked`(기술적 차단)와 `awaiting_review`(인간 대기)의 의미가 명확히 분리
**단점**: 새 상태 추가 → 스키마 변경 없지만 모든 상태 관련 코드(CLI, MCP, 쿼리) 업데이트 필요

#### 권장: **Option A** (blocked 재활용)

이유:
1. 스키마 변경 최소화 (전환 규칙 1줄 추가)
2. `blocked`의 의미가 이미 "진행 불가 — 외부 입력 필요"
3. 인간 대기도 "차단"의 일종 — 의미적으로 정확
4. 나중에 구분이 필요하면 `result` 필드에 `"blocked_reason": "awaiting_human_review"` 기록 가능

### 2.3 스키마 변경

#### 최소 변경안 (tasks 테이블)

```sql
-- reviewer 컬럼 추가 (migration)
ALTER TABLE tasks ADD COLUMN reviewer TEXT;
CREATE INDEX IF NOT EXISTS idx_tasks_reviewer ON tasks(reviewer);
```

`reviewer`는 optional. 값이 있으면 해당 태스크가 완료 시 인간 승인 필요.

#### 왜 reviewer 컬럼인가?

대안: `blocked_by`에 에이전트 ID를 넣는 방식
→ 거부. `blocked_by`는 태스크 ID 배열이며, 에이전트 ID와 혼용하면 의미가 오염됨.

대안: `metadata` JSON 필드에 reviewer 정보 저장
→ 가능하지만, 쿼리가 어려움 (`WHERE json_extract(metadata, '$.reviewer') = ?`)
→ 인덱싱도 복잡. 별도 컬럼이 SQLite에서 가장 효율적.

---

## 3. MCP 도구 필요성 분석

### 현재 MCP 도구 (9개)

| 도구 | 용도 |
|------|------|
| `list_agents` | 에이전트 목록 |
| `send_message` | 에이전트 간 메시지 |
| `send_to_user` | 에이전트 → 인간 메시지 |
| `get_user_messages` | 인간 → 에이전트 응답 읽기 |
| `broadcast` | 전체 브로드캐스트 |
| `create_task` | 태스크 생성 |
| `delegate_task` | 태스크 위임 |
| `list_tasks` | 태스크 목록 |
| `get_status` | 시스템 상태 |

### 질문: "새 MCP 도구가 실제로 필요한가?"

#### 시나리오 분석

**에이전트(MCP 클라이언트)가 인간 승인을 요청하는 흐름:**

1. 에이전트가 태스크 생성: `create_task(title="...", reviewer="user")`
2. 에이전트가 작업 완료: 기존 `update_task` MCP 도구가 없음!

**발견: `update_task` MCP 도구가 없다.**

현재 MCP에 태스크 상태 업데이트 도구가 없음:
- CLI에는 `task update --status ...`가 있음 (task.go:204)
- MCP handler에는 `handleUpdateTask`가 없음

이건 Approver 모델 이전에 이미 존재하는 갭:
- 에이전트가 MCP로 태스크를 생성할 수 있지만
- 상태를 업데이트할 방법이 없음 (delegate만 가능)

#### 결론: 필요한 MCP 도구

| 도구 | 필요성 | 이유 |
|------|--------|------|
| `update_task` | **필수 (Approver 무관)** | 기존 갭. 에이전트가 태스크 상태를 업데이트하려면 반드시 필요 |
| `request_human_approval` | **불필요** | `create_task(reviewer="user")` + `update_task(status="blocked")` 조합으로 충분 |
| `get_approval_status` | **불필요** | `list_tasks(status="blocked", assignee=...)` 로 이미 조회 가능 |

**`update_task`만 추가하면 된다.** 전용 approval 도구는 기존 도구 조합으로 대체 가능.

단, `create_task`에 `reviewer` 파라미터 추가는 필요:
```json
{
  "name": "create_task",
  "inputSchema": {
    "properties": {
      "reviewer": {"type": "string"},  // 추가
      // ... 기존 필드들
    }
  }
}
```

---

## 4. CLI 변경 사항

### 4.1 가드 추가

```go
// task.go — resolveAgentID 또는 create/delegate 시
// assignee가 type="human"이면 거부
if agent.Type == "human" {
    return "", fmt.Errorf("cannot assign task to human agent; use --reviewer instead")
}
```

적용 위치:
- `newTaskCreateCmd()` — `--assign` 플래그 resolve 후
- `newTaskDelegateCmd()` — `--to` 플래그 resolve 후
- `handleCreateTask()` (MCP) — `assigned_to` resolve 후
- `handleDelegateTask()` (MCP) — `to` resolve 후

### 4.2 새 CLI 명령

```bash
# 태스크 생성 시 reviewer 지정
agentcom task create "Implement feature" --assign plan --reviewer user --priority high

# 인간이 승인 대기 태스크 조회
agentcom user tasks                    # reviewer=user인 blocked 태스크 목록

# 인간이 승인/거부
agentcom user approve <task-id>                           # blocked → completed
agentcom user reject <task-id> --reason "수정 필요"        # blocked → in_progress
```

### 4.3 자동 알림 연결

에이전트가 `task update <id> --status blocked`로 전환할 때:
- `reviewer` 필드가 있으면 → 자동으로 reviewer에게 메시지 전송
- 메시지 내용: 태스크 ID, 제목, 결과(result) 포함

이건 `task.Manager.UpdateStatus()` 또는 CLI 레벨에서 처리 가능.

---

## 5. 전체 흐름도

```
[에이전트]                        [시스템]                       [인간]
    |                                |                            |
    |-- create_task(reviewer=user) ->|                            |
    |   status: pending              |                            |
    |                                |                            |
    |-- update_task(in_progress) --->|                            |
    |   status: in_progress          |                            |
    |                                |                            |
    |-- (작업 수행...)               |                            |
    |                                |                            |
    |-- update_task(blocked,         |                            |
    |   result="작업 완료, 검토 요청")|                           |
    |   status: blocked              |                            |
    |                                |-- [자동] send_to_user -->  |
    |                                |   "태스크 tsk_xxx 검토 요청"|
    |                                |                            |
    |                                |                  user inbox |
    |                                |                  user tasks |
    |                                |                            |
    |                                |  <-- user approve tsk_xxx--|
    |                                |   status: completed        |
    |                                |                            |
    |   (또는)                       |                            |
    |                                |  <-- user reject tsk_xxx --|
    |                                |   status: in_progress      |
    |                                |   result: "수정 필요: ..." |
    |                                |                            |
    |<-- (알림/폴링으로 상태 확인)    |                            |
    |-- (재작업 후 다시 blocked)      |                            |
```

---

## 6. 구현 태스크 분해

| ID | 태스크 | 의존성 | 예상 시간 |
|----|--------|--------|-----------|
| PH10-01 | `reviewer` 컬럼 추가 (migration) | 없음 | 0.5h |
| PH10-02 | `blocked → completed` 전환 규칙 추가 | 없음 | 0.25h |
| PH10-03 | `update_task` MCP 도구 추가 | 없음 | 1.5h |
| PH10-04 | `create_task` MCP에 `reviewer` 파라미터 추가 | PH10-01 | 0.5h |
| PH10-05 | assignee human 가드 추가 (CLI + MCP) | 없음 | 1h |
| PH10-06 | `task create --reviewer` CLI 플래그 | PH10-01 | 0.5h |
| PH10-07 | `user tasks` CLI 명령 (blocked + reviewer=user 조회) | PH10-01 | 1h |
| PH10-08 | `user approve / reject` CLI 명령 | PH10-02 | 1.5h |
| PH10-09 | blocked 전환 시 자동 reviewer 알림 | PH10-01 | 1h |
| PH10-10 | 테스트: 상태 전환 + 가드 + 알림 | PH10-01~09 | 2h |
| PH10-11 | MCP 도구 설명 업데이트 + README 반영 | PH10-03~04 | 0.5h |

**총 예상: ~10.25h**

---

## 7. 미결 질문

### Q1: `reviewer`가 에이전트일 수도 있나?
현재 설계는 `reviewer=user`(인간)만 가정. 에이전트 간 코드 리뷰도 허용할 것인가?
→ 허용하면 가드 로직이 달라짐 (human 체크 불필요)
→ **권장**: 타입 제한 없이 reviewer 필드 자체는 범용으로, approve/reject CLI만 user 서브커맨드 아래 배치

### Q2: 자동 blocked 전환 vs 명시적 전환?
에이전트가 `in_progress → completed`로 가려 할 때 reviewer가 있으면 시스템이 자동으로 `blocked`로 돌릴 것인가?
→ **권장**: 명시적. 에이전트가 직접 `blocked`로 전환 + result에 작업 결과 기록. 암묵적 인터셉트는 디버깅 어려움.

### Q3: 승인 타임아웃?
인간이 오래 응답하지 않으면? 현재 설계에는 타임아웃 없음.
→ **Phase 1에서는 없음**. PH6의 타임아웃 인프라가 완성된 후 추가 가능.

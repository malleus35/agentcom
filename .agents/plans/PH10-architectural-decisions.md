# PH10: 아키텍처 결정 종합 문서

> 상태: **결정 완료** — 구현 대기
> 작성일: 2026-03-17
> 기반: 외부 프레임워크 비교, 웹서치 조사, 학술 논문 분석

---

## 목차

1. [review_chain 필요성 결론](#1-review_chain-필요성-결론)
2. [MCP vs CLI 결론](#2-mcp-vs-cli-결론)
3. [Agent-as-Reviewer 필요성 결론](#3-agent-as-reviewer-필요성-결론)

---

## 1. review_chain 필요성 결론

### 결론: **불필요 — 태스크 레벨 필드로 부적합**

### 근거

**외부 프레임워크 조사 결과** (CrewAI, AutoGen, LangGraph, Temporal, Camunda, GitHub Actions, JIRA, Linear, ServiceNow, Wagtail):

- **어떤 프로덕션 시스템도 `review_chain`을 태스크 테이블의 일급 컬럼으로 두지 않는다.**
- 순차적 리뷰가 필요한 경우 모든 시스템이 다음 중 하나를 사용:
  1. **워크플로우 그래프 엣지** (LangGraph, Temporal)
  2. **태스크 시퀀스** (JIRA, Linear — 별개 이슈로 연결)
  3. **별도 승인 레코드 테이블** (Camunda, ServiceNow)

**핵심 논리**:
- `review_chain`은 태스크의 속성이 아니라 **워크플로우의 속성**이다.
- 태스크 테이블에 JSON 배열로 넣으면 쿼리 어려움, 인덱싱 불가, 스키마 경직.
- 만약 다단계 리뷰가 필요해지면 별도 `approval_records` 테이블이 올바른 추상화.

### 결정

- **PH10 스키마에서 `review_chain` 컬럼 완전 제거.** 지연이 아니라 거부.
- 단일 `reviewer` 필드만 유지. 에이전트 ID 또는 `"user"` 값.
- 미래에 cascading이 필요하면 별도 엔티티(`approval_step` 등)로 모델링.

---

## 2. MCP vs CLI 결론

### 결론: **CLI-first, MCP는 선택적 어댑터**

### 현재 상태 분석

| 항목 | CLI | MCP |
|------|-----|-----|
| 도구 수 | 31개 (전체 라이프사이클) | 9개 (에이전트 통신 + 태스크만) |
| 고유 기능 | `init`, `up`, `down`, `doctor`, `skill`, `template` 등 | **없음** — 모든 MCP 도구가 CLI 대응 존재 |
| 토큰 비용 | 낮음 (직접 실행) | **4-32x 비쌈** (Scalekit 벤치마크, 75회 측정) |
| 신뢰성 | 100% (로컬 바이너리) | 원격 시 28% 실패율 (인증/네트워크) |
| 실행 환경 | 터미널 필수 | STDIO (셸 없는 런타임 가능) |

### MCP가 여전히 필요한 유일한 시나리오

1. **셸 없는 런타임**: Electron, 모바일, 브라우저 샌드박스에서 agentcom 사용
2. **MCP 클라이언트 에코시스템**: Claude Desktop, Cursor 등 MCP 프로토콜을 네이티브로 지원하는 도구에서 직접 호출
3. **프로세스 내 통합**: Go 라이브러리가 아닌 JSON-RPC 프로토콜로 다른 언어 런타임에서 호출

### 결정

- **AGENTS.md의 "MCP 퍼스트" (line 166)를 "CLI-first"로 변경.**
- MCP 서버는 유지하되 포지셔닝 변경: "주 통합점" → "셸 없는 런타임용 선택적 어댑터".
- MCP에 CLI 기능을 동기화할 의무 없음. MCP는 에이전트 통신 도구 중심으로 유지.
- 향후 PH5-PH8에서 MCP 확장 계획은 범위 축소 검토 필요.

### 필요 조치

- [ ] `AGENTS.md` line 166: `"MCP 퍼스트"` → `"CLI-first, MCP는 셸 없는 런타임용 선택적 어댑터"`
- [ ] `MEMORY.md`에 TODO로 기록

---

## 3. Agent-as-Reviewer 필요성 결론

### 결론: **구현 가치 있음 — 단, Phase 1에서는 최소 구현으로**

### 조사 근거

#### 3.1 학술 연구: Agent-as-a-Judge (ICML 2025)

Zhuge et al. (2025)의 "Agent-as-a-Judge" 프레임워크 (ICML 2025 게재):

- **에이전트가 에이전트를 평가하는 패턴**은 학술적으로 검증된 패러다임.
- LLM-as-a-Judge의 자연스러운 확장으로, 최종 결과물뿐 아니라 **중간 과정 피드백** 제공이 가능.
- 코드 생성 태스크에서 적용 시 인간 평가와의 상관관계가 기존 평가 방법보다 높음.
- 핵심 한계: **동일 모델이 생성하고 평가하면 self-serving bias 발생** → 별도 모델/에이전트가 리뷰해야 효과적.

#### 3.2 프로덕션 코딩 에이전트의 리뷰 패턴

| 에이전트 | 자기 수정 | 별도 리뷰어 에이전트 | 인간 리뷰 | 품질 메커니즘 |
|---------|----------|-------------------|---------|------------|
| **Devin** | ✅ (내부 self-correction) | ❌ (없음) | ✅ (PR 리뷰) | PR 기반, 인간 머지 필수 |
| **Claude Code** | ✅ (자기 수정 루프) | ❌ (없음) | ✅ (사용자 확인) | 사용자 피드백 루프 |
| **OpenHands** | ✅ (trajectory critic) | ⚠️ (post-hoc selection, 비동기) | ✅ (PR 리뷰) | Trajectory-level 품질 평가 |
| **SWE-Agent** | ✅ (Think-Act loop) | ❌ (없음) | ✅ (벤치마크/PR) | ACI 기반 자기 수정 |
| **CrewAI** | ✅ (retry loop) | ✅ (`LLMGuardrail`) | ✅ (`human_input=True`) | **Cascading: AI 필터 → 인간** |
| **AutoGen** | ✅ | ✅ (`SocietyOfMindAgent`) | ✅ (UserProxy) | **Peer review 내부 팀** |
| **LangGraph** | ✅ | ✅ (AI reviewer node) | ✅ (`interrupt()`) | **Conditional edge 기반** |

#### 3.3 핵심 발견

**발견 1: 프로덕션 코딩 에이전트 (Devin, Claude Code, SWE-Agent)는 별도 리뷰어 에이전트를 사용하지 않는다.**

- 이들은 **자기 수정**(self-correction)에 의존하고, 최종 품질 게이트는 **인간 리뷰**(PR 머지)로 처리.
- 별도 리뷰어 에이전트는 없고, 인간이 PR에서 직접 확인.
- Devin 연례 성과 리뷰(Cognition, 2025): "codebase understanding은 시니어급이나 execution은 주니어급" — 인간 리뷰가 필수적인 이유.

**발견 2: 멀티에이전트 프레임워크 (CrewAI, AutoGen, LangGraph)는 에이전트 리뷰를 적극 사용한다.**

- 이들은 에이전트 리뷰를 **인간 리뷰의 필터**로 사용 (대체가 아님).
- CrewAI `LLMGuardrail`: 별도 에이전트가 output 검증 → 실패 시 retry → N회 초과 시 인간 에스컬레이션.
- AutoGen `SocietyOfMindAgent`: writer + editor 내부 팀 구조에서 peer review.
- LangGraph: conditional edge로 AI reviewer → human interrupt 연결.

**발견 3: "Review, Refine, Repeat" (Chakraborty et al., 2025) — iterative decoding에서 AI reviewer가 best-of-N 선택을 수행.**

- 반복 생성 + AI 평가 + 동적 선택이 단순 단일 생성보다 유의미하게 품질 향상.
- 이는 "리뷰" 보다는 "선택/필터링"에 가까움.
- agentcom의 태스크 리뷰 모델과는 다른 추상화 레벨.

**발견 4: AI 코딩 에이전트 PR 수락률 데이터 (Pinna et al., 2026)**

- 7,156개 PR 분석: Devin만 수락률 상승 추세 (+0.77%/주), 나머지는 정체.
- 이는 AI 에이전트 output의 품질이 아직 인간 리뷰 없이는 불안정하다는 방증.
- **AI 리뷰어가 인간 리뷰 부담을 줄이는 데 가치가 있을 수 있음을 시사.**

#### 3.4 agentcom에 대한 함의

**agentcom은 통신 인프라이지 실행 엔진이 아니다.** agentcom의 `reviewer` 필드는:

1. **리뷰 요청을 라우팅**하는 역할 (누가 리뷰해야 하는지 기록)
2. **상태 전환을 제어**하는 역할 (reviewer가 approve/reject해야 completed 가능)
3. 실제 **리뷰 로직 자체를 실행하지 않음** (그것은 리뷰어 에이전트/인간의 몫)

따라서 `reviewer=agt_xxx`를 지원하는 것은:
- **스키마/API 비용 거의 0**: `reviewer` 필드는 이미 에이전트 ID 또는 "user"를 받도록 설계됨.
- **추가 복잡도 거의 0**: 상태 전환 가드는 reviewer 값의 존재 여부만 확인, 값이 에이전트인지 인간인지는 구분할 필요 없음.
- **확장성 확보**: 멀티에이전트 프레임워크가 agentcom 위에서 AI reviewer 패턴을 구축할 수 있는 기반 제공.

#### 3.5 비판적 분석: agent-review가 불필요할 수 있는 논거

| 논거 | 반론 |
|------|------|
| 프로덕션 코딩 에이전트는 별도 리뷰어를 쓰지 않는다 | agentcom은 단일 에이전트가 아니라 멀티에이전트 오케스트레이션 도구. CrewAI/AutoGen 패턴에 더 가깝다. |
| 자기 수정(self-correction)으로 충분하다 | Agent-as-a-Judge 연구가 self-serving bias를 지적. 별도 리뷰어가 더 효과적. |
| 인간 리뷰만으로 충분하다 | 맞지만, 인간 리뷰 부담을 줄이는 AI 필터가 있으면 더 효율적. |
| 구현 복잡도가 높다 | **반박: agentcom에서는 복잡도 0.** reviewer 필드가 "user"든 "agt_xxx"든 동일하게 동작. |

#### 3.6 최종 결론

**`reviewer` 필드는 에이전트 ID도 지원해야 한다.** 이유:

1. **추가 비용 0**: reviewer 필드의 타입 체크만 없으면 됨 (이미 자유 문자열).
2. **agentcom의 정체성 부합**: "에이전트 유형 자유" 설계 원칙과 일치.
3. **멀티에이전트 패턴 지원**: CrewAI/AutoGen 스타일 워크플로우를 agentcom 위에서 구축 가능.
4. **인간 전용으로 제한할 이유 없음**: 에이전트가 approve CLI/MCP 호출을 할 수 있고, 인간은 `user approve`를 사용. 두 경로 모두 자연스러움.

**단, reviewer가 에이전트일 때의 자동화 로직 (retry loop, cascading, guardrail 패턴)은 agentcom 범위 밖.** agentcom은 리뷰 요청 라우팅과 상태 전환 제어만 담당하고, 실제 리뷰 판단은 상위 레이어(사용 측 에이전트 프레임워크)의 책임.

### 결정

- **`reviewer` 필드는 에이전트 ID + "user" 모두 지원.**
- reviewer 값에 대한 타입 제한 없음 (자유 문자열, 기존 설계 원칙 준수).
- 에이전트 리뷰어의 approve/reject는 기존 `task update --status` 또는 향후 `update_task` MCP를 통해 수행.
- 자동 리뷰 로직 (retry, cascading, guardrail)은 agentcom 범위 밖. 사용 측에서 구현.

---

## 4. 종합 결정 요약

| 항목 | 결정 | 상태 |
|------|------|------|
| `review_chain` 컬럼 | **거부** — 태스크 레벨 필드로 부적합 | 확정 |
| MCP 포지셔닝 | **CLI-first** — MCP는 셸 없는 런타임용 어댑터 | 확정, AGENTS.md 수정 필요 |
| `reviewer` 필드 범위 | **에이전트 ID + "user" 모두 지원** | 확정 |
| 에이전트 리뷰 자동화 | **agentcom 범위 밖** — 상위 레이어 책임 | 확정 |
| `blocked → completed` 전환 | **추가** (model.go 1줄) | PH10 구현 대기 |
| 인간 리뷰 타임아웃 | **없음** (무한 대기) | 확정 |
| `update_task` MCP 도구 | **신규 추가 필요** (기존 갭) | PH10 구현 대기 |
| CLI `task update` 가드 우회 | **수정 필요** (line 218-222) | PH10 구현 대기 |

---

## 5. PH10 구현 범위 최종 정리

### 스키마

```sql
ALTER TABLE tasks ADD COLUMN reviewer TEXT;
CREATE INDEX IF NOT EXISTS idx_tasks_reviewer ON tasks(reviewer);
```

### 상태 머신

```go
// model.go
StatusBlocked: {
    StatusInProgress: {},
    StatusPending:    {},
    StatusCancelled:  {},
    StatusCompleted:  {},  // 추가
},
```

```go
// manager.go — UpdateStatus 가드
if to == StatusCompleted && from == StatusInProgress && task.Reviewer != "" {
    return ErrReviewRequired
}
```

### CLI

- `task create --reviewer <agent|user>` — 새 플래그
- `task update` — Manager.UpdateStatus() 경유하도록 수정
- `user approve <task-id>` — 인간 리뷰 승인
- `user reject <task-id> --reason "..."` — 인간 리뷰 거부
- `user tasks` — blocked + reviewer=user 태스크 조회

### MCP

- `create_task` — `reviewer` 파라미터 추가
- `update_task` — **신규 도구** (상태 변경 + 가드 적용)

### 제외 (범위 밖)

- `review_chain` 컬럼
- 자동 리뷰 로직 (retry, cascading, guardrail)
- 에스컬레이션 타이머
- 리뷰 데드라인

---

## 참조

### 학술 논문
- Zhuge et al. (2025). "Agent-as-a-Judge: Evaluate Agents with Agents." ICML 2025, PMLR 267:80569-80611.
- Chakraborty et al. (2025). "Review, Refine, Repeat: Understanding Iterative Decoding of AI Agents with Dynamic Evaluation and Selection." arXiv:2504.01931v2.
- Pinna et al. (2026). "Comparing AI Coding Agents: A Task-Stratified Analysis of Pull Request Acceptance." arXiv:2602.08915.

### 프레임워크/도구 문서
- CrewAI: LLMGuardrail, human_input patterns
- AutoGen: SocietyOfMindAgent, SelectorGroupChat
- LangGraph: Conditional edges, interrupt() for human-in-the-loop
- Temporal: wait_for_signal, activity timeout policies
- Camunda: User Task, Service Task, escalation timers
- OpenHands: Trajectory critic model, SWE-bench evaluation

### 제품 리뷰/데이터
- Cognition (2025). "Devin's 2025 Performance Review: Learnings From 18 Months of Agents At Work."
- MIT (2025). "The 2025 AI Agent Index: Documenting Technical and Safety Features of Deployed Agentic AI Systems."
- Scalekit MCP benchmark: CLI vs MCP token cost comparison (75 runs)

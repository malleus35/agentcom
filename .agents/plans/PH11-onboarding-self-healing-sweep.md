# PH11: Onboarding & Self-Healing Sweep

> **상태**: 초안 (2026-04-06, Descartes minor fix 12건 반영 v1.1)
> **브랜치**: `feature/PH11-onboarding-self-healing`
> **우선순위**: High
> **추정 공수**: ~35h (4 Track, 27 Tasks, ~79 Subtasks)
> **선행 조건**: PH10 Priority & Review Policy 머지 완료, PH8 MCP surface rebaseline 완료

---

## 0. 변증법적 배경 (Why this cycle)

### 0.1 정(Thesis) — 현재 상태
- PH8~PH10을 거치며 agentcom은 **라우팅 + 상태 전환**의 코어 정합성은 확보함.
- 그러나 신규 사용자는 `init → skill → spawn → task` 경로를 스스로 조립해야 하고,
  환경 드리프트(marker 블록, skill schema, stale socket)는 감지만 되고 **복구는 수동**임.
- PRD 잔여 항목(E4 custom template, E5 SKILL.md 확장)은 계속 다음 사이클로 미뤄지며 부채화됨.

### 0.2 반(Antithesis) — 이상 상태
- `agentcom quickstart` 한 번으로 **5분 안에 2-agent 대화가 눈앞에서 흐르는** 상태.
- `agentcom doctor --fix`가 감지한 드리프트 중 안전한 것은 **자동 복구**.
- PRD 잔여 항목은 이 사이클에서 **클로징**되어 다음 사이클이 새 축(관측/TUI 등)으로 전진 가능.

### 0.3 합(Synthesis) — 이번 사이클
성장(A: Onboarding), 안정(B: Self-Healing), PRD 잔여 클로징을 **한 사이클에 묶는다**.
각 트랙은 독립적으로 머지 가능하지만, 머지 게이트 E2E는 네 트랙이 하나의 사용자 여정
(`quickstart → doctor → doctor --fix → quickstart 재실행`)으로 수렴하는지 검증한다.

---

## 1. 목표 & 범위

### 1.1 핵심 목표

| # | 목표 | 측정 |
|---|------|------|
| G1 | 신규 사용자 TTFM(Time-To-First-Message) < 5분 | **측정 필수 — T2.7.1을 필수 subtask로 승격**. `quickstart` 실행~첫 메시지 흐름까지 자동 측정 |
| G2 | 감지된 드리프트 중 safe 항목 100% 자동 복구 | `doctor --fix` dry-run diff = actual apply diff |
| G3 | PRD E4/E5 잔여 클로징 | PRD 체크리스트 E4, E5 항목 DONE |
| G4 | install-failure 신호 수집 채널 오픈 | GitHub issue template + release 스모크 테스트 green |

### 1.2 범위 밖 (명시적 연기)

| 항목 | 사유 | 재평가 시점 |
|------|------|-------------|
| β1 CGO 제거 | 신호 부족, 현 사용자 영향 미미 | 6개월 후 이슈 집계 재검토 |
| β3 watch TUI | quickstart 사용자 요구 선행 필요 | quickstart 사용 데이터 수집 후 |
| doctor --fix unsafe 항목 | 백업/롤백 인프라 성숙 필요 | PH12 후반 |
| MCP `doctor_fix` 노출 | safe-only 강제 정책 검증 선행 | Track 3 완료 후 별도 논의 |

---

## 2. 트랙 개요

| Track | 이름 | 역할 | 예상 공수 | Owner 힌트 |
|-------|------|------|-----------|------------|
| T1 | PRD 잔여 클로징 (E4, E5) | 부채 상환 | ~6.9h | Kant / Wittgenstein |
| T2 | β2 Quickstart (메인) | 성장 | ~11.2h | Spinoza / Kant |
| T3 | β4 doctor --fix (사이드) | 안정 | ~13.1h | Descartes / Spinoza |
| T4 | Spinoza 안전망 | 신호 수집 | ~4.0h | Spinoza |
| E2E | 머지 게이트 | 검증 | ~3.5h | Descartes |
| **합계** | | | **~38.7h** (여유 20% 포함 ~46h) | |

---

## 3. Track 1 — PRD 잔여 클로징

### T1.1 (PRD E4) — Custom Template advanced 모드 단순화

**현재**: `internal/cli/init_prompter.go` advanced 모드가 템플릿당 8개 필드 × N 역할. 중간 취소 시 전부 재입력.
**목표**: 핵심 필드만 남기고 나머지는 sane default, 중간 취소 시 진행상황을 `.agentcom/wizard-state.json`으로 저장·복원.

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|----------|-----------|------|------|
| T1.1.1 | `internal/cli/init_prompter.go` | advanced 모드 필드 목록을 core/optional로 분리 (`coreFields []string`, `optionalFields []string`) | [ ] core 3~4개로 축소 [ ] optional은 기본값 주입 | `go test ./internal/cli/...` | 1h |
| T1.1.2 | `internal/cli/init_prompter.go` | `applyDefaults(role Role)` 헬퍼 신설 | [ ] optional 필드 누락 시 default 자동 주입 [ ] 단위 테스트 | `go test -run TestApplyDefaults` | 0.5h |
| T1.1.3 | `internal/cli/wizard_state.go` (신규) | `WizardState` 구조체 + `Save/Load/Clear` 정의 (파일: `.agentcom/wizard-state.json`) | [ ] JSON 직렬화 [ ] schema_version 필드 포함 | `go test ./internal/cli/ -run TestWizardState` | 1h |
| T1.1.4 | `internal/cli/init_prompter.go` | 중간 취소(SIGINT, ESC) hook → `WizardState.Save()` | [ ] ctrl-C 시 상태 저장 [ ] 재실행 시 "resume?" 프롬프트 | **expect/PTY 골든 테스트로 자동화**(`test/pty/init_resume_test.go`). 실패 시 §13.4 Manual QA Checklist에 항목 등재 | 1h |
| T1.1.5 | `internal/cli/init_prompter.go` | 성공 완료 시 `WizardState.Clear()` 호출 | [ ] init 성공 후 wizard-state.json 삭제 | **PTY 골든 테스트 포함 (T1.1.4와 동일 파일)** | 0.3h |
| T1.1.6 | `internal/cli/init_prompter_test.go` | resume 시나리오 골든 테스트 | [ ] save→load→apply 3단계 모두 커버 | `go test ./internal/cli/...` | 0.7h |
| T1.1.7 | `docs/prd/` 해당 섹션 | E4 체크박스 DONE, 변경 요약 링크 | [ ] PRD 문서 업데이트 | 수동 | 0.2h |

**T1.1 의존성**: T1.1.1 → T1.1.2 → T1.1.4. T1.1.3은 T1.1.1과 병렬. T1.1.6은 나머지 전부 완료 후.

### T1.2 (PRD E5) — 루트 SKILL.md 확장 + schema_version

**현재**: `internal/cli/skill.go renderSkillContent()` 약 10줄. communication protocol, workflow, JSON 예시 없음.
**목표**: 60줄 이상, `communication / workflow / examples` 섹션 + `schema_version: 1` frontmatter.

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|----------|-----------|------|------|
| T1.2.1 | `internal/cli/skill.go` | `renderSkillContent()` frontmatter에 `schema_version: 1` 추가 | [ ] 기존 테스트 업데이트 [ ] frontmatter parser 호환 | `go test ./internal/cli/ -run TestSkill` | 0.5h |
| T1.2.2 | `internal/cli/skill.go` | `## Communication Protocol` 섹션 추가 (메시지 타입, 수신자 명명 규약) | [ ] 최소 15줄 | 골든 파일 diff | 0.7h |
| T1.2.3 | `internal/cli/skill.go` | `## Workflow` 섹션 추가 (task 수명주기, reviewer 게이트 언급) | [ ] 최소 15줄 [ ] PH10 reviewer 정책 반영 | 골든 파일 diff | 0.7h |
| T1.2.4 | `internal/cli/skill.go` | `## Examples` 섹션 — JSON 메시지/태스크 예시 3개 | [ ] 각 예시 valid JSON | `go test -run TestSkillExamplesParse` | 1h |
| T1.2.5 | `internal/cli/skill_test.go` | `TestSkillSchemaVersion`, `TestSkillSections` 신규 | [ ] 섹션 3개 존재 [ ] schema_version=1 | `go test ./internal/cli/...` | 0.5h |
| T1.2.6 | `internal/doctor/skill_check.go` | schema_version 불일치 시 warn, 누락 시 error | [ ] doctor가 schema_version 확인 [ ] 메시지 명확 | `go test ./internal/doctor/...` | 0.6h |
| T1.2.7 | `docs/prd/` E5 | DONE 체크 | [ ] 문서 업데이트 | 수동 | 0.2h |

**T1.2 의존성**: T1.2.1 → (T1.2.2‖T1.2.3‖T1.2.4) → T1.2.5 → T1.2.6.

---

## 4. Track 2 — β2 Quickstart (메인)

### 결정이 필요한 항목 (Track 2 착수 전 합의)

| 결정 | 후보 | 기본 |
|------|------|------|
| T2.3 mock agent 형태 | (a) 실프로세스 `agentcom mock-agent` (b) in-process goroutine | **(a) 실프로세스** — 실제 소켓 경로를 검증 가능, quickstart가 doctor의 실제 상태를 그대로 노출 |
| sandbox 경로 | `$XDG_DATA_HOME/agentcom/sandbox` vs `./.agentcom-quickstart` | **XDG** 기본, `--here` 플래그로 override |

### T2.1 — `agentcom quickstart` 명령 신설

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|----------|-----------|------|------|
| T2.1.1 | `internal/cli/quickstart.go` (신규) | `NewQuickstartCmd()` cobra 커맨드 + 플래그(`--here`, `--keep`, `--no-color`) | [ ] `agentcom quickstart --help` 출력 | `go build && ./agentcom quickstart --help` | 0.7h |
| T2.1.2 | `internal/cli/root.go` | root에 quickstart 등록 | [ ] `agentcom --help`에 노출 | 수동 | 0.2h |
| T2.1.3 | `internal/cli/quickstart.go` | 상위 `Run()` 골격: `scaffold → spawn → demo → hint → cleanup` 5단계 | [ ] 각 단계 함수 스텁 존재 | `go vet ./...` | 0.5h |

### T2.2 — 데모 워크스페이스 자동 scaffold

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|----------|-----------|------|------|
| T2.2.1 | `internal/cli/quickstart_scaffold.go` | sandbox 디렉토리 결정 로직 (XDG / `--here`) | [ ] 기존 dir 존재 시 안전하게 재사용 or 재생성 확인 프롬프트 [ ] **`wizard-state.json` 디렉토리 키 형식과 충돌하지 않음 (sandbox 경로가 wizard-state 키로 인코딩될 때 콜리전 없음을 단위 테스트로 증명)** | 단위 테스트 | 0.6h |
| T2.2.2 | `internal/cli/quickstart_scaffold.go` | sandbox 안에 `.agentcom/` 구조 초기화 (init 내부 함수 재사용) | [ ] `.agentcom/skill.md`, `config.toml` 생성 | 단위 테스트 | 0.8h |
| T2.2.3 | `internal/cli/quickstart_scaffold.go` | 2개 role(`planner`, `worker`) 템플릿 파일 자동 생성 | [ ] AGENTS.md 2개, SKILL.md 2개 | 단위 테스트 | 0.6h |
| T2.2.4 | `internal/cli/quickstart_scaffold_test.go` | 골든 디렉토리 비교 테스트 | [ ] 파일 트리 snapshot 일치 | `go test ./internal/cli/ -run TestQuickstartScaffold` | 0.6h |

### T2.3 — mock agent 2개 자동 spawn (실프로세스 결정)

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|----------|-----------|------|------|
| T2.3.1 | `internal/cli/mock_agent.go` (신규) | 숨김 커맨드 `agentcom __mock-agent --role <name>` | [ ] hidden flag 설정 [ ] stdin/stdout 로깅 | 수동 실행 | 0.8h |
| T2.3.2 | `internal/cli/mock_agent.go` | 메시지 수신 시 역할별 응답 템플릿 (planner: 태스크 분해, worker: 완료 보고) | [ ] A→B→A 왕복 가능 | 단위 테스트 | 1h |
| T2.3.3 | `internal/cli/quickstart_spawn.go` | `os/exec`로 mock agent 2개 백그라운드 spawn, pid 추적 | [ ] 정상 spawn [ ] cleanup 시 kill | 단위 테스트 | 1h |
| T2.3.4 | `internal/cli/quickstart_spawn.go` | ready-probe(소켓 생성까지 polling, timeout 5s) | [ ] timeout 초과 시 clear error | 단위 테스트 | 0.6h |

### T2.4 — 데모 task A→B→A 라이브 가시화

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|----------|-----------|------|------|
| T2.4.1 | `internal/cli/quickstart_demo.go` | 초기 태스크 생성 (planner에게 "Build a hello feature") | [ ] 태스크 등록 성공 | 단위 테스트 | 0.4h |
| T2.4.2 | `internal/cli/quickstart_demo.go` | 이벤트 구독 → 색상/아이콘 포함 라이브 출력 | [ ] 메시지 흐름이 3단계 이상 눈에 보임 | 수동 QA + §13.4 Manual QA Checklist 등재 | 1h |
| T2.4.3 | `internal/cli/quickstart_demo.go` | 최대 20s 타임아웃 — 초과 시 graceful 종료 + doctor 추천 | [ ] 타임아웃 경로 테스트 | 단위 테스트 | 0.5h |

### T2.5 — 종료 메시지 next-step hint 3줄 + doctor 권유

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|----------|-----------|------|------|
| T2.5.1 | `internal/cli/quickstart.go` | 종료 hint 3줄(`agentcom init`, `agentcom spawn`, `agentcom doctor`) | [ ] 정확히 3줄 [ ] doctor는 `--fix` 아님 | 골든 테스트 | 0.3h |
| T2.5.2 | `internal/cli/quickstart.go` | 실패 경로 시 `agentcom doctor` 자동 추천 문구 | [ ] stderr에 노출 | **stderr 골든 테스트로 자동화** (`quickstart_failpath_test.go`) | 0.2h |

### T2.6 — sandbox 디렉토리 doctor 화이트리스트 등록

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|----------|-----------|------|------|
| T2.6.1 | `internal/doctor/whitelist.go` (신규 또는 기존 doctor.go) | sandbox 경로 자동 인식 → doctor가 `[sandbox]` 라벨로 구분 | [ ] sandbox 내 drift는 warning으로 격하 | 단위 테스트 | 0.8h |
| T2.6.2 | `internal/doctor/whitelist_test.go` | 화이트리스트 테스트 | [ ] 일반 경로와 sandbox 경로 분류 | `go test ./internal/doctor/...` | 0.4h |

### T2.7 — TTFM 측정 인프라 (G1 게이트, 필수)

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|----------|-----------|------|------|
| T2.7.1 | `internal/cli/quickstart.go` | start→first-message timestamp 기록, `--metrics` 플래그로 출력. **G1 게이트: 측정값이 완료 기준 §13에 기록됨** | [ ] 초 단위 표시 [ ] G1 측정 산출물 제공 | E2E에서 값 파싱 | 0.5h |
| T2.7.2 | `internal/cli/quickstart.go` | opt-in 익명 로컬 기록 `.agentcom/metrics.jsonl` (전송 X) | [ ] 로컬 파일만 | 수동 | 0.5h |

**Track 2 의존성**:
- T2.1.1 → T2.1.2 → T2.1.3
- T2.2.* ← T2.1.3
- T2.3.1 → T2.3.2, T2.3.3 → T2.3.4
- T2.4.* ← (T2.2 완료 AND T2.3 완료)
- T2.5.* ← T2.4.1
- T2.6.* ← T2.2.1 (경로 결정만 필요)
- T2.7.* ← T2.4.2 (first message hook 필요)

---

## 5. Track 3 — β4 doctor --fix (사이드)

### T3.1 — `doctor --fix` 플래그 + dry-run 기본

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|----------|-----------|------|------|
| T3.1.1 | `internal/cli/doctor.go` | `--fix`, `--yes`, `--dry-run`(기본 true when --fix) 플래그 | [ ] `--fix`만 주면 dry-run | `./agentcom doctor --fix` + **expect 스크립트 자동화** (`test/pty/doctor_fix_flags_test.go`) | 0.5h |
| T3.1.2 | `internal/cli/doctor.go` | dry-run 출력 포맷 (planned diff) | [ ] diff가 명확히 파일·라인 단위로 보임 | 골든 테스트 | 0.7h |

### T3.2 — fixer 인터페이스 정의

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|----------|-----------|------|------|
| T3.2.1 | `internal/doctor/fixer/fixer.go` (신규) | `type Fixer interface { ID() string; Safe() bool; Plan() Diff; Apply() error; Rollback() error; Idempotent() bool }` | [ ] 인터페이스 및 godoc | `go vet ./...` | 0.8h |
| T3.2.2 | `internal/doctor/fixer/registry.go` | `Register()` / `All()` / `BySafety()` | [ ] 테스트 존재 | `go test ./internal/doctor/fixer/...` | 0.5h |

### T3.3 — marker block fixer (template_files drift)

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|----------|-----------|------|------|
| T3.3.1 | `internal/doctor/fixer/marker_block.go` | 기존 marker 검출 로직 재사용해 `Plan/Apply` 구현 | [ ] Safe=true [ ] apply 후 drift=0 | 단위 테스트 | 1.2h |
| T3.3.2 | `internal/doctor/fixer/marker_block_test.go` | 단위 테스트(정상/충돌/이미_해결) | [ ] 3 케이스 green | `go test` | 0.6h |

### T3.4 — skill drift fixer (schema_version 매칭 시만)

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|----------|-----------|------|------|
| T3.4.1 | `internal/doctor/fixer/skill_drift.go` | frontmatter schema_version가 일치할 때만 Safe=true, 아니면 Safe=false | [ ] version mismatch 시 --fix에서 제외 [ ] **schema_version mismatch 시 `Safe()==false` AND dry-run 출력에 사유(예: "skipped: schema_version v1 != v2")가 명시됨** | 단위 테스트 + 골든 출력 | 1h |
| T3.4.2 | `internal/doctor/fixer/skill_drift_test.go` | v1↔v1 apply, v1↔v2 skip 시나리오 | [ ] 두 케이스 green [ ] skip 사유 문자열 검증 | `go test` | 0.5h |

**선행**: T1.2.1 (schema_version 도입) 필수.

### T3.5 — stale socket fixer

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|----------|-----------|------|------|
| T3.5.1 | `internal/doctor/fixer/stale_socket.go` | 죽은 pid 참조 소켓 파일 제거 | [ ] 활성 소켓 건드리지 않음 [ ] Safe=true | 단위 테스트 | 0.8h |
| T3.5.2 | `internal/doctor/fixer/stale_socket_test.go` | mock 소켓 생성·pid 가짜 PID | [ ] 2 케이스 green | `go test` | 0.5h |

### T3.6 — `.agentcom/backup/<ts>/` 백업 인프라

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|----------|-----------|------|------|
| T3.6.1 | `internal/doctor/backup/backup.go` | `SnapshotFiles([]string) (BackupID, error)` | [ ] 타임스탬프 디렉토리 생성 | 단위 테스트 | 0.8h |
| T3.6.2 | `internal/doctor/backup/backup.go` | `Restore(BackupID) error` | [ ] 복원 후 원본 일치 | 단위 테스트 | 0.6h |
| T3.6.3 | `internal/doctor/backup/retention.go` | 오래된 백업 N개 이상 시 cleanup(기본 N=10) | [ ] 보존 정책 테스트 | 단위 테스트 | 0.4h |
| **T3.6.4** | `internal/doctor/backup/restore_integration_test.go` (신규) | **Restore 통합 테스트 — fixer apply 후 의도적 실패를 주입하고 `Restore`를 호출해 원본 파일 바이트 단위 일치 검증** | [ ] 최소 2 fixer (marker_block, skill_drift)로 시나리오 실행 [ ] restore 후 SHA256 원본 일치 [ ] backup 디렉토리는 retention 정책에 따라 정리됨 | `go test ./internal/doctor/backup/... -run TestRestoreIntegration` | 1h |

### T3.7 — 멱등성 테스트

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|----------|-----------|------|------|
| T3.7.1 | `internal/doctor/fixer/idempotency_test.go` | 각 fixer 2회 apply 시 2번째 Plan()이 empty diff | [ ] 모든 safe fixer 통과 | `go test -run TestIdempotency` | 1h |

### T3.8 — doctor 출력 메시지를 quickstart 어휘로 재정렬

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|----------|-----------|------|------|
| T3.8.1 | `internal/cli/doctor.go` | 출력 라벨 재조정 (quickstart 용어 통일) | [ ] quickstart hint와 어휘 일치 | 골든 테스트 | 0.5h |
| T3.8.2 | `internal/cli/doctor_test.go` | 골든 출력 테스트 업데이트 | [ ] green | `go test ./internal/cli/...` | 0.3h |

### T3.9 — confirm/--yes 정책 + MCP safe-only 강제

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|----------|-----------|------|------|
| T3.9.1 | `internal/cli/doctor.go` | `--fix` 실제 적용은 `--yes` 혹은 interactive confirm 필수 | [ ] non-tty && no --yes → abort | **expect/PTY 골든 테스트 자동화** (`test/pty/doctor_confirm_test.go`) | 0.6h |
| T3.9.2 | `internal/mcp/...` (관련 서버 파일) | MCP 노출되는 doctor 경로가 있다면 `Safe()==true`만 허용. 없다면 문서화만 | [ ] unsafe fixer MCP에서 거부 | 단위 테스트 | 0.8h |

**Track 3 의존성**:
- T3.2.* → (T3.3, T3.4, T3.5)
- T3.6.* ← T3.2.1 (Apply가 backup 사용)
- T3.6.4 ← T3.6.1, T3.6.2, T3.3, T3.4
- T3.3~T3.5 → T3.7
- T3.1.* ← T3.2.1
- T3.4.* ← **T1.2.1** (schema_version)
- T3.8.* ← T3.1.2
- T3.9.* ← T3.1, T3.2

---

## 6. Track 4 — Spinoza 안전망

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|----------|-----------|------|------|
| T4.1.1 | `internal/cli/doctor.go` | `--report-install-failure` opt-in 섹션 (OS, shell, installer, 에러 로그 수집) | [ ] 개인정보 제외 문서화 | **스냅샷 테스트로 자동화** (`doctor_report_test.go`, T4.1.2와 동일), interactive 확인은 §13.4 Manual QA Checklist | 1h |
| T4.1.2 | `internal/cli/doctor_report_test.go` | 생성 리포트 스냅샷 | [ ] green | `go test` | 0.4h |
| T4.2.1 | `.github/ISSUE_TEMPLATE/install-failure.yml` | GH 이슈 템플릿(OS/버전/installer/log 필드) | [ ] GH UI에서 렌더 | GH 미리보기 | 0.5h |
| T4.2.2 | `.github/ISSUE_TEMPLATE/config.yml` | 카테고리 등록 | [ ] 목록에 노출 | 수동 | 0.2h |
| T4.3.1 | `.github/workflows/release.yml` | Scoop install 후 `agentcom --version` 스모크 스텝 추가 | [ ] fail 시 release 중단 | CI run | 1h |
| T4.3.2 | `.github/workflows/release.yml` | Homebrew/Tarball 동일 스모크 (최소 1종) | [ ] green | CI run | 0.6h |
| T4.3.3 | `docs/release-runbook.md` | 스모크 실패 대응 절차 기록 | [ ] 문서 존재 | 수동 | 0.3h |

**Track 4 의존성**: 거의 없음 (T4.1은 `doctor.go` 접촉하므로 Track 3와 머지 충돌 주의).

---

## 7. 머지 게이트 — E2E 시나리오

### 7.1 E2E 테스트 파일 (신규)

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|----------|-----------|------|------|
| E2E.1 | `test/e2e/quickstart_selfheal_test.go` (신규) | 시나리오: `quickstart → doctor → 인위적 drift 주입 → doctor --fix --yes → quickstart 재실행` | [ ] 각 단계 exit 0 [ ] 전 구간 60s 이내 [ ] **drift 주입 → `--fix --yes` → 의도적 실패(예: apply 중 panic 주입) → backup restore → 정상 복귀 경로 포함** | `go test ./test/e2e/... -run TestQuickstartSelfHeal` | 2h |
| E2E.2 | `test/e2e/helpers.go` | temp HOME / XDG 격리 헬퍼 | [ ] 병렬 안전 | `go test` | 0.8h |
| E2E.3 | `.github/workflows/ci.yml` | e2e job 추가 (**ubuntu-latest, macos-latest, windows-latest**). windows는 최소 mock agent smoke 수준이라도 포함 | [ ] main PR에서 3 OS green (windows는 smoke-only 허용) | CI run | 0.7h |

**E2E 선행**: Track 1, 2, 3 모두 머지된 이후.

---

## 8. 의존성 그래프 (요약)

```
T1.1 (template wizard) ─┐
                        ├──► (독립 병렬)
T1.2.1 (schema_version) ┘
         │
         └────► T3.4 (skill drift fixer)

T2.1 ► T2.2 ► T2.4 ► T2.5
       T2.3 ┘        T2.7
       T2.4.2 ───────► T2.7   (first-message hook, G1 게이트)
       T2.6 (T2.2.1만 선행)

T3.1 ┐
T3.2 ┼──► T3.3, T3.5 ──► T3.7
T3.6 ┘       T3.4  ──┘
     └──► T3.6.4 (restore integration) ← T3.3/T3.4 apply
             └── needs T1.2.1
T3.1.2 ► T3.8
T3.1 + T3.2 ► T3.9

T4.* : 거의 독립 (T4.1은 doctor.go 충돌 주의)

(T1, T2, T3 머지) ► E2E ► 머지 게이트 통과
```

---

## 9. 병렬 실행 가능한 Worktree 조합 (Wave)

### Wave 1 — 기반 (병렬 4 worktree)

| Worktree | 담당 | 충돌 위험 |
|----------|------|-----------|
| `wt-t1-1-wizard` | T1.1 전체 | `internal/cli/init_prompter.go` |
| `wt-t1-2-skill` | T1.2 전체 | `internal/cli/skill.go` |
| `wt-t3-fixer-iface` | T3.1, T3.2, T3.6 | `internal/doctor/*` 신규 패키지 |
| `wt-t4-safety` | T4.1, T4.2, T4.3 | `doctor.go` (T4.1) → Wave 2 이전 머지 권장 |

**Wave 1 게이트**: T1.2.1 머지(schema_version) + T3.2.1 머지(Fixer interface).

### Wave 2 — 확장 (병렬 3 worktree)

| Worktree | 담당 | 선행 |
|----------|------|------|
| `wt-t2-quickstart` | T2.1 ~ T2.7 전체 | Wave 1 T1.* |
| `wt-t3-fixers` | T3.3, T3.4, T3.5, T3.6.4, T3.7, T3.8, T3.9 | Wave 1 T3.2, T3.6.1/2, T1.2.1 |
| `wt-t1-2-skill-doctor` | T1.2.6 (doctor schema check) | Wave 1 T1.2.1 |

### Wave 3 — 머지 게이트 (단일 worktree)

| Worktree | 담당 | 선행 |
|----------|------|------|
| `wt-e2e-gate` | E2E.1 ~ E2E.3 | Wave 2 전부 |

---

## 10. 총 예상 공수

| Track | Subtasks | 공수 |
|-------|----------|------|
| T1 (PRD 클로징) | 14 | 6.9h |
| T2 (Quickstart) | 18 | 11.2h |
| T3 (doctor --fix) | 19 | 13.1h |
| T4 (안전망) | 7 | 4.0h |
| E2E 게이트 | 3 | 3.5h |
| **합계** | **61** | **~38.7h** |

여유(리뷰·리워크 20%) 포함 시 **~46h**.

---

## 11. 중간 체크포인트

**사이클 절반(공수 50%, 약 ~23h 소진 시점)에서 점검**:

- [ ] β4 진단(fixer) 항목 수가 **>5**이면 T3.4/T3.5 중 일부를 **PH12로 분할**
- [ ] Track 2 TTFM 측정(T2.7)이 지연되면 **G1 재정의 필요** (필수 게이트이므로 연기 불가)
- [ ] E2E.1이 60s 안에 수렴 실패 시 타임아웃 완화 vs 성능 개선 결정
- [ ] **수동 QA 항목 잔여 수 집계, 자동화 가능한 것은 골든/PTY 테스트로 전환** (§13.4 Manual QA Checklist와 cross-check)

체크포인트 담당: Hegel(플래너), Descartes(검증 가능성 판단).

---

## 12. 연기 항목 (명시적 기록)

| 항목 | 사유 | 재평가 조건 |
|------|------|-------------|
| β1 CGO 제거 | 신호 수집 중 | +6개월 후 install-failure 리포트 >10건 시 재개 |
| β3 watch TUI | 사용 데이터 부족 | quickstart 월간 실행 수 >100 이후 |
| doctor --fix unsafe 항목 | 백업/롤백 성숙 필요 | PH12 |
| MCP `doctor_fix` tool | safe-only 검증 선행 | Track 3 완료 retro 후 |

---

## 13. 완료 기준 체크리스트

### 13.1 기능 완료
- [ ] `agentcom quickstart` 실행 → 5분 이내 A→B→A 메시지 흐름 가시화
- [ ] `agentcom quickstart --here` 로컬 디렉토리 scaffold 동작
- [ ] mock agent 2개 spawn 후 cleanup 시 프로세스 누수 없음
- [ ] `agentcom doctor --fix` dry-run 출력이 실제 apply diff와 1:1 일치
- [ ] **`agentcom doctor --fix` 실패 시 backup으로 자동 복원되어 원본 상태가 바이트 단위 보존됨**
- [ ] safe fixer 3종(marker block, skill drift, stale socket) 모두 멱등
- [ ] `.agentcom/backup/<ts>/` 백업 생성 및 Restore 동작
- [ ] Custom Template advanced 모드 core 필드 4개 이하로 축소
- [ ] `.agentcom/wizard-state.json` 저장·복원 시나리오 동작
- [ ] 루트 SKILL.md ≥60줄, `schema_version: 1` 포함, 3개 섹션 존재
- [ ] G1: TTFM 측정값이 `--metrics` 출력으로 제공되고 5분 미만

### 13.2 품질 게이트
- [ ] `go test ./...` 전부 green
- [ ] E2E `TestQuickstartSelfHeal` green (ubuntu, macos, windows-smoke)
- [ ] `go vet ./...` clean
- [ ] 신규 패키지 godoc 존재 (`internal/doctor/fixer`, `internal/doctor/backup`)
- [ ] 멱등성 테스트 (T3.7.1) green
- [ ] **Restore 회귀 시나리오 green** (T3.6.4, E2E.1 restore 경로)
- [ ] **PH10 reviewer 정책 골든 회귀 green** (T1.2.3 workflow 섹션이 PH10 골든을 깨뜨리지 않음 확인)

### 13.3 문서·릴리스
- [ ] PRD E4, E5 DONE 표시
- [ ] `docs/release-runbook.md` 스모크 실패 대응 절차 업데이트
- [ ] `.github/ISSUE_TEMPLATE/install-failure.yml` 렌더 확인
- [ ] Scoop 스모크 테스트 release.yml 반영 및 dry-run green
- [ ] `docs/debates/PH11-onboarding-self-healing.md` retro 작성

### 13.4 Manual QA Checklist (자동화 불가 항목 집약)

자동화 시도 후 PTY/expect로도 커버되지 않은 항목만 여기 남긴다. 각 항목은 체크포인트(§11)에서 잔여 수를 집계한다.

- [ ] `agentcom init` 중 실제 터미널에서 ctrl-C → 재실행 시 resume 프롬프트 UX (T1.1.4 보조)
- [ ] quickstart 라이브 출력의 색상/아이콘이 실제 터미널에서 가독성 확보 (T2.4.2)
- [ ] `--report-install-failure` interactive 플로우의 개인정보 검토 (T4.1.1)

### 13.5 연기 기록
- [ ] 본 문서 §12 연기 항목이 다음 사이클 입구에서 재평가되도록 `NEXT-PHASE.md` 링크

---

## 14. 리뷰 요청

- **Kant**: T2.4 라이브 출력의 정보 밀도·색상 정책, T2.5 hint 3줄의 어휘 선정
- **Socrates**: "TTFM < 5분"이 진짜 사용자 가치인지, 측정 없이도 G1이 성립하는지 — 본 개정에서 **측정 필수로 전환**
- **Descartes**: T3.7 멱등성 정의가 "Plan() empty diff"로 충분한지, E2E 시나리오의 반증 가능성, T3.6.4 restore 통합 범위
- **Spinoza**: T2.3 mock agent 실프로세스 결정이 OS별(특히 Windows) 신뢰 가능한지, E2E.3 windows-latest smoke 범위
- **Wittgenstein**: T1.2 SKILL.md 섹션 경계(communication vs workflow)의 언어 명확성

---

## 15. 변경 이력

| 날짜 | 작성자 | 비고 |
|------|--------|------|
| 2026-04-06 | Hegel | 초안 작성, 합 가설 v2 기반 |
| 2026-04-06 | Hegel | v1.1 — Descartes APPROVED WITH MINOR FIXES 12건 반영 (G1 측정 필수화, T2.7 필수 승격, T3.6.4 restore 통합 테스트 신설, E2E windows 포함, wizard-state 충돌 방지, schema_version mismatch 출력, PTY 자동화 전환, Manual QA Checklist §13.4 신설, 품질 게이트 2건 추가) |

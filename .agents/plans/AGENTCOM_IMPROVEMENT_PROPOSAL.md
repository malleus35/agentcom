# Agentcom CLI 개선 제안서 (PRD)

> **Version**: 1.0  
> **Date**: 2026-03-16  
> **Status**: Draft  
> **Author**: Automated Analysis  
> **Scope**: agentcom v0.1.6 CLI UX/워크플로우 전면 개선

---

## 목차

1. [Executive Summary](#executive-summary)
2. [분석 방법론](#분석-방법론)
3. [3-관점 종합 분석](#3-관점-종합-분석)
4. [Epic 1: AGENTS.md/CLAUDE.md Append 동작 수정](#epic-1)
5. [Epic 2: 스킬 간 상호연결(Interconnection) 개선](#epic-2)
6. [Epic 3: Init 시 프로젝트명 명시적 입력](#epic-3)
7. [Epic 4: Custom Template 생성 단순화](#epic-4)
8. [Epic 5: 스킬 문서(SKILL.md) 품질 강화](#epic-5)
9. [Epic 6: 추가 발견 사항 및 부가 개선](#epic-6)
10. [우선순위 및 구현 로드맵](#우선순위-및-구현-로드맵)

---

## Executive Summary

agentcom CLI는 멀티 에이전트 협업을 위한 프로젝트 스캐폴딩/오케스트레이션 도구로, 현재 v0.1.6입니다. 소스코드 전수 분석 결과, **5개의 핵심 개선 영역**과 **1개의 추가 발견 사항**이 확인되었습니다.

### 핵심 발견 요약

| # | 영역 | 심각도 | 현재 상태 |
|---|------|--------|----------|
| 1 | AGENTS.md Append 미지원 | 🔴 Critical | 파일 존재 시 에러 발생, append 로직 전무 |
| 2 | 스킬 간 상호연결 | 🟡 Major | 통신 그래프 존재하나, 검증 미비 + 자기참조 버그 |
| 3 | Init 프로젝트명 | 🟡 Major | 빈 문자열 허용, 현재 사용자가 작업 중 |
| 4 | Custom Template 복잡도 | 🟠 High | 6개 역할 기준 47개 입력 필드 필요 |
| 5 | SKILL.md 문서 품질 | 🟠 High | 10줄짜리 루트 스킬, 구체적 가이드 부재 |
| 6 | 추가: 자기참조 에스컬레이션 | 🟡 Major | architect가 자기 자신에게 에스컬레이션 |

---

## 분석 방법론

### 분석 대상 소스 파일

```
agentcom/internal/cli/
├── instruction.go      ← AGENTS.md/CLAUDE.md 생성 로직 (append 버그 위치: line 245)
├── init_prompter.go    ← Wizard 및 Custom Template 위저드 (line 275-368)
├── init_setup.go       ← Setup executor
├── agents.go           ← 템플릿 정의, 스킬 렌더링 (line 355-535)
├── skill.go            ← 스킬 파일 생성
├── up.go               ← 에이전트 라이프사이클
├── template_store.go   ← 커스텀 템플릿 영속화
agentcom/internal/onboard/
├── template_definition.go ← TemplateDefinition, TemplateRole 구조체
```

### 생성된 출력물 분석 (db workspace)

```
thehackerton/db/
├── .agentcom.json          ← project:"" (빈 문자열 확인)
├── AGENTS.md               ← 19줄, 제네릭 워크플로우
├── CLAUDE.md               ← AGENTS.md와 동일
├── .agentcom/templates/company/
│   ├── template.json       ← 112줄 전체 company 템플릿
│   └── COMMON.md           ← 9줄
└── .opencode/skills/agentcom/
    ├── SKILL.md             ← 10줄 (루트, 너무 간략)
    └── oh-my-opencode-*/SKILL.md  ← 각 24줄 (6개 역할)
```

---

## 3-관점 종합 분석

### 관점 1: 프로젝트 관리자/감독자 (Supervisor)

프로젝트를 에이전트에 위임하고 결과를 감독하는 사용자의 관점입니다.

**Pain Points:**

1. **재초기화 불가능** (Epic 1): 팀 구성 변경이나 역할 추가 시 `agentcom init`을 다시 실행하면 AGENTS.md가 이미 존재한다며 실패합니다. 관리자는 수동으로 파일을 삭제하거나 편집해야 하며, 이는 기존 설정을 날릴 위험이 있습니다.

2. **에이전트 간 협업 투명성 부족** (Epic 2): 통신 그래프가 존재하지만, 어떤 에이전트가 실제로 누구와 소통하는지 런타임에서 확인할 수 없습니다. 감독자 입장에서 에이전트 간 메시지 흐름이 불투명합니다.

3. **프로젝트 식별 혼란** (Epic 3): 여러 프로젝트를 관리할 때 프로젝트명이 비어있으면 어떤 프로젝트의 에이전트인지 구분이 불가능합니다.

4. **팀 구성 시간 과다** (Epic 4): 6개 역할의 커스텀 팀을 구성하는 데 47개 필드를 입력해야 합니다. 실험적으로 다양한 팀 구성을 시도하기가 현실적으로 불가능합니다.

5. **위임 품질 불확실** (Epic 5): 스킬 문서가 너무 간략해서 에이전트가 실제로 역할을 제대로 수행하는지 판단하기 어렵습니다.

### 관점 2: AI 에이전트 (Agent Consumer)

스킬을 로드하고 지시사항에 따라 동작하는 AI 에이전트의 관점입니다.

**Pain Points:**

1. **지시사항 불완전** (Epic 1): AGENTS.md에 기존 프로젝트 컨텍스트가 있는데, 새 agentcom 설정이 추가되지 않고 실패하면, 에이전트는 협업 프레임워크를 인식하지 못한 채 동작합니다.

2. **통신 대상 모호** (Epic 2): SKILL.md의 `## Communication` 섹션이 `<sender>`, `<target>` 같은 플레이스홀더를 사용합니다. 에이전트가 실제로 누구에게 메시지를 보내야 하는지 추론해야 합니다.

3. **자기참조 에스컬레이션** (Epic 6): architect 역할의 SKILL.md에 "Escalate blockers to `plan` and `architect`"라고 되어있어, 자기 자신에게 에스컬레이션하라는 무의미한 지시가 포함됩니다.

4. **협업 프로토콜 부재** (Epic 5): 언제 독립적으로 작업하고, 언제 다른 에이전트와 소통해야 하는지에 대한 구체적 가이드라인이 없습니다. JSON 메시지 포맷이나 태스크 생성 예시도 없습니다.

5. **공통 지시사항 적용 혼란** (Epic 1): COMMON.md와 AGENTS.md의 관계, 어떤 것이 우선하는지가 불명확합니다.

### 관점 3: CLI 사용자 (End User)

터미널에서 직접 agentcom 명령을 실행하는 사용자의 관점입니다.

**Pain Points:**

1. **에러 메시지 불친절** (Epic 1): "instruction file already exists"라는 에러만 표시되고, `--force` 옵션이나 append 가능성에 대한 안내가 없습니다.

2. **위저드 피로감** (Epic 4): 커스텀 템플릿 생성 시 반복적인 폼 입력(역할당 7개 필드)은 CLI 사용자에게 매우 불편합니다. 중간에 취소하면 진행 상황이 모두 유실됩니다.

3. **프로젝트명 누락 무경고** (Epic 3): `agentcom init` 시 프로젝트명을 비워도 경고 없이 진행되어, 나중에 `agentcom status`에서 프로젝트 식별이 안 됩니다.

4. **템플릿 차이 불투명** (Epic 4): `company`와 `oh-my-opencode` 템플릿이 동일한 6개 역할을 가지는데, 차이점이 무엇인지 CLI에서 확인할 수 없습니다.

5. **스킬 동작 확인 불가** (Epic 5): `agentcom skill list`로 스킬 목록은 보이지만, 각 스킬이 실제로 어떤 역할을 하는지 CLI에서 확인할 수 있는 상세 설명이 부족합니다.

---

<a id="epic-1"></a>
## Epic 1: AGENTS.md/CLAUDE.md Append 동작 수정

> **심각도**: 🔴 Critical  
> **영향 범위**: 모든 사용자 (재초기화 시 100% 실패)  
> **근본 원인**: `instruction.go:245-247` — `writeInstructionFile()` 함수가 파일 존재 시 무조건 에러 반환  
> **현재 동작**: `os.Stat(path)` 성공 → `return fmt.Errorf("instruction file already exists: %s", path)`

### Task 1.1: Append 로직 구현

**목표**: 기존 instruction 파일이 존재할 때, 내용을 덮어쓰지 않고 파일 끝에 agentcom 설정을 추가

#### Subtask 1.1.1: 기존 파일 감지 및 분기 로직

- **파일**: `internal/cli/instruction.go` — `writeInstructionFile()` 함수
- **변경 내용**:
  - `os.Stat(path)` 성공 시 에러 반환 대신 append 모드로 분기
  - 새 함수 `appendInstructionFile(path, content string) error` 추가
  - 기존 파일 끝에 `\n\n` 구분자 + agentcom 블록 추가
- **수락 기준**:
  - [x] 파일이 없으면 기존대로 새로 생성
  - [x] 파일이 있으면 기존 내용 보존 + 마지막 줄에 내용 추가
  - [x] 기존 파일의 trailing newline 유무와 관계없이 정상 동작

#### Subtask 1.1.2: 중복 추가 방지 (Idempotency)

- **변경 내용**:
  - agentcom이 추가하는 블록에 고유 마커 삽입: `<!-- AGENTCOM:START -->` / `<!-- AGENTCOM:END -->`
  - append 전에 기존 파일에서 마커 존재 여부 확인
  - 마커가 이미 있으면: 해당 블록만 교체 (update 모드)
  - 마커가 없으면: 파일 끝에 추가 (append 모드)
- **수락 기준**:
  - [x] 동일 명령 2회 실행 시 블록이 중복되지 않음
  - [x] 마커 블록 내용이 변경되었으면 최신 내용으로 업데이트
  - [x] 마커 블록 외부의 사용자 커스텀 내용은 절대 수정하지 않음

#### Subtask 1.1.3: `--force` 플래그 동작 범위 확대

- **파일**: `internal/cli/init_setup.go`
- **현재 동작**: `--force`는 `.agentcom.json`에만 적용
- **변경 내용**:
  - `--force` 적용 시 instruction 파일도 전체 덮어쓰기 옵션 추가
  - `--force` 없이도 append는 기본 동작으로 진행
  - `--force`와 append의 관계를 명확히 구분:
    - `--force` 없음 + 파일 존재: append (마커 기반)
    - `--force` 있음 + 파일 존재: 전체 덮어쓰기
- **수락 기준**:
  - [x] `--force` 없이 재실행 시 기존 내용 보존 + append
  - [x] `--force`로 재실행 시 파일 전체 재생성
  - [x] 도움말(`--help`)에 동작 설명 추가

### Task 1.2: 에이전트 디렉토리별 분기 처리

**목표**: 서로 다른 에이전트(codex, opencode, amp, devin)가 동일 파일(AGENTS.md)을 공유하는 구조에서의 충돌 방지

#### Subtask 1.2.1: `seenPaths` 맵 개선

- **파일**: `internal/cli/instruction.go` — `generateInstructionFiles()` 함수
- **현재 동작**: `seenPaths` 맵으로 동일 경로 중복 쓰기 방지 (같은 init 실행 내)
- **변경 내용**:
  - 동일 경로에 대한 내용 병합 로직 추가 (여러 에이전트 타입이 같은 파일을 가리키는 경우)
  - 각 에이전트 타입별 섹션을 구분하여 작성
- **수락 기준**:
  - [x] codex + opencode가 모두 AGENTS.md를 가리킬 때, 두 에이전트의 설정이 모두 포함
  - [x] 에이전트별 섹션이 명확히 구분됨

### Task 1.3: 에러 메시지 및 UX 개선

#### Subtask 1.3.1: 사용자 친화적 에러 메시지

- **변경 내용**:
  - 현재: `"instruction file already exists: %s"` (어떤 액션도 안내하지 않음)
  - 변경: append 모드가 기본이므로 에러 대신 info 로그 출력
  - `"Found existing %s — appending agentcom configuration (use --force to overwrite)"`
- **수락 기준**:
  - [x] 파일 존재 시 에러가 아닌 정보 메시지 표시
  - [x] `--force` 옵션 존재를 안내

#### Subtask 1.3.2: Dry-run 모드 지원

- **변경 내용**:
  - `agentcom init --dry-run` 플래그 추가
  - 실제 파일 변경 없이 어떤 파일에 무엇이 추가/생성될지 미리보기
- **수락 기준**:
  - [x] `--dry-run` 실행 시 변경 예정 사항만 stdout에 출력
  - [x] 실제 파일 시스템에는 어떤 변경도 없음

### Task 1.4: 테스트

#### Subtask 1.4.1: 유닛 테스트

- 빈 디렉토리에서 init → 파일 새로 생성 확인
- 기존 AGENTS.md가 있는 디렉토리에서 init → append 확인
- 마커가 이미 있는 파일에서 init → update 확인 (중복 없음)
- `--force`로 init → 전체 덮어쓰기 확인
- 기존 파일 내용이 보존되는지 확인

#### Subtask 1.4.2: 통합 테스트

- 실제 프로젝트 디렉토리(기존 AGENTS.md 포함)에서 end-to-end init 테스트
- 멀티 에이전트(codex + opencode) 동시 init 시나리오

---

<a id="epic-2"></a>
## Epic 2: 스킬 간 상호연결(Interconnection) 개선

> **심각도**: 🟡 Major  
> **영향 범위**: 에이전트 간 협업 품질  
> **근본 원인**: `agents.go:396-403` — 통신 그래프가 정적이며 검증 로직 없음  
> **발견된 버그**: architect → architect 자기참조 에스컬레이션 (`agents.go:379`)

### Task 2.1: 통신 그래프 검증 로직 추가

#### Subtask 2.1.1: 대칭성 검증 (Symmetric Validation)

- **파일**: `internal/cli/agents.go` — `builtInTemplateDefinitions()`
- **현재 상태**: `communicationMap`이 대칭적으로 설정되어 있으나, 코드로 검증하지 않음
- **변경 내용**:
  - 템플릿 빌드 시 통신 그래프 대칭성 자동 검증 함수 추가
  - A → B가 있으면 B → A도 존재하는지 확인
  - 비대칭 발견 시 경고 로그 출력 + 자동 보정 옵션
- **수락 기준**:
  - [x] 비대칭 통신 관계 자동 감지
  - [x] 빌드 타임에 경고 출력
  - [x] `--strict` 모드에서는 비대칭 시 에러로 중단

#### Subtask 2.1.2: 자기참조 에스컬레이션 제거

- **파일**: `internal/cli/agents.go` — `renderRoleSkillContent()` (line 379)
- **현재 코드**: 모든 역할에 하드코딩된 `"Escalate blockers to plan and architect"`
- **변경 내용**:
  - 현재 역할이 `architect`이면 에스컬레이션 대상에서 자기 자신 제외
  - 현재 역할이 `plan`이면 에스컬레이션 대상에서 자기 자신 제외
  - 에스컬레이션 대상을 `communicationMap`에서 동적으로 결정
- **수락 기준**:
  - [x] architect SKILL.md에 "Escalate to architect" 문구 없음
  - [x] plan SKILL.md에 "Escalate to plan" 문구 없음
  - [x] 각 역할의 에스컬레이션 대상이 자기 자신을 포함하지 않음

#### Subtask 2.1.3: 고아 역할(Orphan Role) 감지

- **변경 내용**:
  - 통신 그래프에서 어떤 역할과도 연결되지 않은 역할 감지
  - 커스텀 템플릿에서도 고아 역할 경고
- **수락 기준**:
  - [x] 연결 없는 역할 생성 시 경고 메시지 출력
  - [x] `agentcom agents template <name>` 명령에서 통신 그래프 시각화

### Task 2.2: SKILL.md 통신 섹션 구체화

#### Subtask 2.2.1: 플레이스홀더 제거 및 실명 사용

- **파일**: `internal/cli/agents.go` — `renderRoleSkillContent()`
- **현재 상태**: `<sender>`, `<target>` 플레이스홀더 사용
- **변경 내용**:
  - 렌더링 시 실제 역할 이름과 에이전트 이름으로 치환
  - `communicatesWith` 배열의 각 역할에 대해 구체적 소통 시나리오 생성
- **수락 기준**:
  - [x] 생성된 SKILL.md에 `<sender>`, `<target>` 문자열 없음
  - [x] 실제 역할명이 사용됨 (예: "frontend에게 UI 컴포넌트 작업 요청")

#### Subtask 2.2.2: 협업 프로토콜 명세 추가

- **변경 내용**:
  - 각 역할 SKILL.md에 협업 프로토콜 섹션 추가:
    - **요청(Request)**: 다른 역할에게 작업을 요청하는 방법
    - **응답(Response)**: 요청에 응답하는 방법
    - **에스컬레이션(Escalation)**: 블로커 발생 시 상위 역할에 에스컬레이션하는 방법
    - **보고(Report)**: 작업 완료/진행 상황 보고 방법
  - 각 항목에 `agentcom send` 명령 예시 포함
- **수락 기준**:
  - [x] 모든 역할 SKILL.md에 4가지 협업 프로토콜 섹션 존재
  - [x] 각 섹션에 실행 가능한 CLI 명령 예시 포함

### Task 2.3: 런타임 통신 검증

#### Subtask 2.3.1: `agentcom verify` 명령 추가

- **변경 내용**:
  - 새 서브커맨드: `agentcom verify`
  - 현재 프로젝트의 통신 그래프 무결성 검증
  - 검증 항목: 대칭성, 자기참조 없음, 고아 역할 없음, 모든 역할의 SKILL.md 존재
- **수락 기준**:
  - [x] 정상 상태에서 `agentcom verify` → "All checks passed"
  - [x] 비정상 상태에서 구체적 문제점과 수정 방법 안내

### Task 2.4: 테스트

#### Subtask 2.4.1: 통신 그래프 검증 테스트

- 대칭 그래프에서 검증 통과 확인
- 비대칭 그래프에서 경고 발생 확인
- 자기참조 있는 경우 감지 확인
- 고아 역할 감지 확인

---

<a id="epic-3"></a>
## Epic 3: Init 시 프로젝트명 명시적 입력

> **심각도**: 🟡 Major  
> **영향 범위**: 멀티 프로젝트 관리 시 식별 불가  
> **근본 원인**: `init_prompter.go:40-51` — `defaultWizardProjectName()`이 빈 문자열 허용  
> **참고**: ⚠️ 사용자가 현재 직접 작업 중인 항목

### Task 3.1: 프로젝트명 필수 입력 강제

#### Subtask 3.1.1: Wizard에서 프로젝트명 필수화

- **파일**: `internal/cli/init_prompter.go` — `defaultWizardProjectName()` 및 wizard 폼
- **변경 내용**:
  - 프로젝트명 입력 필드에 validation 추가: 빈 문자열 거부
  - 디렉토리명 기반 기본값은 유지하되, 사용자가 명시적으로 확인해야 진행
  - 빈 프로젝트명으로 진행 불가하도록 폼 레벨 검증
- **수락 기준**:
  - [x] 빈 프로젝트명으로 init 진행 불가
  - [x] 디렉토리명 기반 기본 제안값 제공
  - [x] 사용자가 기본값을 수정할 수 있음

#### Subtask 3.1.2: CLI 플래그로 프로젝트명 전달

- **변경 내용**:
  - `agentcom init --project <name>` 플래그를 필수 옵션으로 격상
  - 플래그 없이 실행 시 wizard에서 물어봄 (현재 동작 유지)
  - 비대화형(non-interactive) 모드에서는 `--project` 필수
- **수락 기준**:
  - [x] `agentcom init --project myapp` → 프로젝트명 "myapp" 설정
  - [x] 비대화형 모드에서 `--project` 없으면 에러

#### Subtask 3.1.3: 기존 프로젝트 마이그레이션

- **변경 내용**:
  - `.agentcom.json`에 `project: ""` 인 경우 감지
  - `agentcom status` 실행 시 경고: "프로젝트명이 설정되지 않았습니다"
  - `agentcom config set project <name>` 명령으로 사후 설정 가능
- **수락 기준**:
  - [x] 빈 프로젝트명 감지 및 경고
  - [x] 사후 프로젝트명 설정 가능

### Task 3.2: 프로젝트명 활용 범위 확대

#### Subtask 3.2.1: 생성 파일에 프로젝트명 반영

- **변경 내용**:
  - AGENTS.md 생성 시 프로젝트명 포함: `"# [ProjectName] Agent Configuration"`
  - SKILL.md 생성 시 프로젝트 컨텍스트 포함
  - `agentcom status` 출력에 프로젝트명 표시
- **수락 기준**:
  - [x] 생성된 AGENTS.md 첫 줄에 프로젝트명 포함
  - [x] `agentcom status`에 프로젝트명 표시

### Task 3.3: 테스트

#### Subtask 3.3.1: 프로젝트명 검증 테스트

- 빈 프로젝트명으로 init 시도 → 거부 확인
- `--project` 플래그로 init → 정상 설정 확인
- 기존 빈 프로젝트명 → 경고 메시지 확인
- `agentcom config set project` → 사후 설정 확인

---

<a id="epic-4"></a>
## Epic 4: Custom Template 생성 단순화

> **심각도**: 🟠 High  
> **영향 범위**: 커스텀 팀 구성 UX  
> **근본 원인**: `init_prompter.go:275-368` — `runCustomTemplateWizard()` 47개 필드 (6역할 기준)  
> **목표**: 역할 이름만 입력 → 자동 생성

### Task 4.1: 간소화된 위저드 플로우 설계

#### Subtask 4.1.1: 1-Step 역할명 입력 방식

- **파일**: `internal/cli/init_prompter.go` — `runCustomTemplateWizard()` 대체
- **변경 내용**:
  - **현재 (47 필드)**:
    ```
    Form 1: 템플릿명, 설명, 참조 (3개)
    Form 2: [역할명, 설명, 에이전트명, 에이전트타입, 책임(CSV), 소통대상(CSV), 추가?] × N (7×N개)
    Form 3: 공통 제목, 공통 본문 (2개)
    ```
  - **변경 후 (최소 2 필드)**:
    ```
    Form 1: 템플릿명 (1개, 선택적 — 미입력 시 자동 생성)
    Form 2: 역할명 목록 (콤마 구분 또는 하나씩 추가) (1개)
    ```
  - 나머지 필드는 역할명 기반으로 자동 생성:
    - `description`: 역할명 기반 기본 설명 (예: "frontend" → "Frontend development and UI implementation")
    - `agentName`: 역할명을 PascalCase로 변환 (예: "frontend" → "Frontend")
    - `agentType`: 기본값 "specialist"
    - `responsibilities`: 역할명 기반 기본 책임 목록 (사전 정의 맵 활용)
    - `communicatesWith`: 전체 역할 간 기본 연결 (메시형 그래프)
    - `commonTitle`, `commonBody`: 기본 템플릿 사용
- **수락 기준**:
  - [x] 역할명만 입력하면 전체 템플릿 생성 가능
  - [x] 생성된 템플릿이 현재와 동일한 구조 및 품질
  - [x] 6개 역할 기준 입력 필드: 최대 2개 (템플릿명 + 역할 목록)

#### Subtask 4.1.2: 역할명 → 메타데이터 자동 생성 엔진

- **변경 내용**:
  - 새 파일: `internal/cli/role_defaults.go`
  - 사전 정의된 역할 메타데이터 맵:
    ```go
    var roleDefaults = map[string]RoleMetadata{
        "frontend":  {Description: "Frontend development...", Responsibilities: [...]},
        "backend":   {Description: "Backend services...", Responsibilities: [...]},
        "plan":      {Description: "Project planning...", Responsibilities: [...]},
        "review":    {Description: "Code review...", Responsibilities: [...]},
        "architect": {Description: "System architecture...", Responsibilities: [...]},
        "design":    {Description: "UI/UX design...", Responsibilities: [...]},
        "qa":        {Description: "Quality assurance...", Responsibilities: [...]},
        "devops":    {Description: "Infrastructure...", Responsibilities: [...]},
        "security":  {Description: "Security audit...", Responsibilities: [...]},
    }
    ```
  - 사전 정의되지 않은 역할명: 제네릭 기본값 생성 + 경고
  - `communicatesWith` 자동 생성: 모든 역할을 서로 연결 (기본 메시 토폴로지)
- **수락 기준**:
  - [x] 사전 정의 역할: 풍부한 메타데이터로 자동 생성
  - [x] 미정의 역할: 합리적 기본값으로 생성 + 사용자 경고
  - [x] 통신 그래프 자동 생성 (대칭적)

#### Subtask 4.1.3: Advanced 모드 유지

- **변경 내용**:
  - 기존 상세 위저드를 `--advanced` 플래그 뒤에 보존
  - `agentcom init --advanced`: 기존 47-필드 위저드 실행
  - `agentcom init` (기본): 간소화 위저드 실행
- **수락 기준**:
  - [x] `--advanced` 없이 init → 간소화 위저드
  - [x] `--advanced`로 init → 기존 상세 위저드 (하위 호환)
  - [x] 두 방식 모두 동일한 출력 품질

### Task 4.2: 사후 편집 지원

#### Subtask 4.2.1: `agentcom template edit` 명령

- **변경 내용**:
  - 기존 커스텀 템플릿의 역할 추가/제거/수정 명령 추가
  - `agentcom template edit <name> add-role <rolename>`: 역할 추가
  - `agentcom template edit <name> remove-role <rolename>`: 역할 제거
  - 변경 시 자동으로 통신 그래프 업데이트 + SKILL.md 재생성
- **수락 기준**:
  - [x] 역할 추가 후 통신 그래프에 자동 반영
  - [x] 역할 제거 후 관련 참조 자동 정리
  - [x] 변경된 SKILL.md 자동 재생성

#### Subtask 4.2.2: 템플릿 비교 명령

- **변경 내용**:
  - `agentcom agents template diff <template1> <template2>`: 두 템플릿 차이 비교
  - 역할 목록, 통신 그래프, 공통 설정의 차이를 시각적으로 표시
- **수락 기준**:
  - [x] `company`와 `oh-my-opencode` 비교 시 차이점 명확히 표시
  - [x] 커스텀 템플릿 간 비교도 지원

### Task 4.3: 비대화형 모드 지원

#### Subtask 4.3.1: YAML/JSON 기반 템플릿 정의

- **변경 내용**:
  - `agentcom init --from-file template.yaml` 지원
  - 최소 YAML 형식:
    ```yaml
    name: my-team
    roles: [frontend, backend, plan, review]
    ```
  - 완전한 YAML 형식 (advanced):
    ```yaml
    name: my-team
    description: "Custom team for project X"
    roles:
      - name: frontend
        description: "Custom frontend role"
        responsibilities: [...]
    ```
- **수락 기준**:
  - [x] 최소 YAML (역할명 목록만)로 템플릿 생성 가능
  - [x] 완전한 YAML로 세밀한 제어 가능
  - [x] CI/CD 파이프라인에서 비대화형으로 사용 가능

### Task 4.4: 테스트

#### Subtask 4.4.1: 간소화 위저드 테스트

- 역할명 3개 입력 → 전체 템플릿 생성 확인
- 사전 정의/미정의 역할 혼합 입력 테스트
- `--advanced` 모드 하위 호환성 테스트
- YAML 파일 기반 init 테스트

---

<a id="epic-5"></a>
## Epic 5: 스킬 문서(SKILL.md) 품질 강화

> **심각도**: 🟠 High  
> **영향 범위**: 에이전트가 역할을 수행하는 품질에 직접적 영향  
> **근본 원인**: 루트 SKILL.md 10줄, 역할별 SKILL.md 24줄 — 구체적 가이드 부재  
> **핵심 문제**: 에이전트가 "무엇을 해야 하는지"는 알려주지만 "어떻게 해야 하는지"는 알려주지 않음

### Task 5.1: 루트 agentcom SKILL.md 강화

#### Subtask 5.1.1: 프레임워크 개요 섹션 추가

- **파일**: `internal/cli/agents.go` — 루트 SKILL.md 템플릿
- **현재 상태**: 3개 불릿포인트 (10줄)
- **변경 내용**: 다음 섹션 추가
  - `## Overview`: agentcom 프레임워크가 무엇이고, 왜 사용하는지
  - `## Architecture`: 에이전트 구조 (역할, 통신 그래프, 공통 설정)
  - `## Communication Protocol`: 에이전트 간 소통 방식
  - `## Task Lifecycle`: 태스크 생성 → 할당 → 진행 → 완료의 전체 흐름
  - `## Quick Reference`: 자주 사용하는 명령어 표
- **수락 기준**:
  - [x] 루트 SKILL.md가 최소 60줄 이상
  - [x] AI 에이전트가 읽고 바로 동작할 수 있는 수준의 구체성
  - [x] 실행 가능한 CLI 명령 예시 포함

#### Subtask 5.1.2: 메시지 포맷 명세

- **변경 내용**:
  - 에이전트 간 메시지의 표준 JSON 포맷 정의:
    ```json
    {
      "from": "backend",
      "to": "frontend",
      "type": "request|response|escalation|report",
      "priority": "high|medium|low",
      "subject": "API endpoint ready for integration",
      "body": "The /api/users endpoint is now available...",
      "context": {"task_id": "...", "related_files": [...]}
    }
    ```
  - `agentcom send` 명령과의 매핑 설명
- **수락 기준**:
  - [x] 메시지 포맷이 명확히 정의됨
  - [x] 에이전트가 해당 포맷으로 메시지를 작성할 수 있는 수준

#### Subtask 5.1.3: 의사결정 플로우차트

- **변경 내용**:
  - "언제 독립 작업 vs 소통 vs 에스컬레이션" 판단 기준 추가:
    ```
    독립 작업: 자신의 역할 범위 내, 다른 역할 의존성 없음
    소통 필요: 다른 역할의 산출물이 필요하거나, 자신의 산출물이 다른 역할에 영향
    에스컬레이션: 자신의 역할로 해결 불가, 아키텍처 결정 필요, 우선순위 충돌
    ```
- **수락 기준**:
  - [x] 에이전트가 독립/소통/에스컬레이션을 스스로 판단할 수 있는 기준 제공

### Task 5.2: 역할별 SKILL.md 강화

#### Subtask 5.2.1: 역할별 구체적 워크플로우 추가

- **파일**: `internal/cli/agents.go` — `renderRoleSkillContent()`
- **현재 상태**: 각 24줄, 제네릭 책임 목록 + 간략한 통신 섹션
- **변경 내용**: 각 역할에 다음 추가:
  - `## Workflow`: 해당 역할의 전형적 작업 흐름 (단계별)
  - `## Examples`: 실제 시나리오 기반 행동 예시 (최소 2개)
  - `## Anti-patterns`: 하지 말아야 할 것 (역할별 맞춤)
  - `## Handoff Protocol`: 다른 역할에 작업을 넘기는 구체적 방법
- **수락 기준**:
  - [x] 각 역할 SKILL.md가 최소 50줄 이상
  - [x] 모든 역할에 Workflow, Examples, Anti-patterns, Handoff 섹션 존재
  - [x] 예시가 해당 역할에 맞게 구체적 (제네릭하지 않음)

#### Subtask 5.2.2: 역할별 도구 권장 사항

- **변경 내용**:
  - 각 역할에 권장 도구/명령 섹션 추가:
    ```
    ## Recommended Tools
    - File operations: Read, Write, Edit
    - Code search: Grep, AST-grep
    - Diagnostics: lsp_diagnostics
    - Communication: agentcom send, agentcom task create
    ```
  - 역할에 따라 도구 우선순위 차별화 (예: review → diagnostics 우선, frontend → browser 우선)
- **수락 기준**:
  - [x] 역할별 도구 권장 목록이 차별화됨
  - [x] 도구와 역할의 관계가 명확

#### Subtask 5.2.3: 통신 섹션 실명화 및 시나리오화

- **변경 내용**:
  - 현재: `Primary contacts: <role1>, <role2>` (플레이스홀더)
  - 변경: 실제 역할명 사용 + 시나리오별 소통 예시
    ```markdown
    ## Communication

    ### Primary Contacts
    - **backend** (Backend Engineer): API 인터페이스 협의, 데이터 모델 확인
    - **design** (UI Designer): 디자인 시안 수령, 구현 피드백

    ### Communication Scenarios
    1. **새 API 필요시**: backend에게 요청
       ```
       agentcom send --to backend --subject "New API endpoint needed" --body "..."
       ```
    2. **디자인 확인 필요시**: design에게 확인
       ...
    ```
- **수락 기준**:
  - [x] 모든 플레이스홀더 제거
  - [x] 최소 2개의 구체적 소통 시나리오 포함
  - [x] 각 시나리오에 실행 가능한 명령 예시

### Task 5.3: COMMON.md 강화

#### Subtask 5.3.1: 팀 전체 규칙 및 프로토콜 추가

- **현재 상태**: 9줄, "Work collaboratively..." 수준의 제네릭 문구
- **변경 내용**:
  - 팀 전체에 적용되는 코딩 표준 섹션
  - 커밋 메시지 규칙
  - 브랜치 전략
  - 충돌 해결 프로토콜
  - 우선순위 체계 설명
- **수락 기준**:
  - [x] COMMON.md가 최소 30줄 이상
  - [x] 팀 전체에 적용 가능한 구체적 규칙 포함

### Task 5.4: 문서 품질 검증 도구

#### Subtask 5.4.1: `agentcom skill validate` 명령

- **변경 내용**:
  - 스킬 문서의 품질 검증 명령 추가
  - 검증 항목:
    - 최소 줄 수 충족 여부
    - 필수 섹션 존재 여부 (## Communication, ## Workflow 등)
    - 플레이스홀더 잔존 여부
    - CLI 명령 예시 포함 여부
- **수락 기준**:
  - [x] 검증 통과/실패를 섹션별로 리포트
  - [x] 실패 항목에 대한 수정 가이드 제공

### Task 5.5: 테스트

#### Subtask 5.5.1: 문서 품질 테스트

- 강화된 루트 SKILL.md가 60줄 이상인지 확인
- 각 역할 SKILL.md가 50줄 이상인지 확인
- 플레이스홀더(`<sender>`, `<target>`) 잔존 여부 확인
- `agentcom skill validate` 명령 동작 확인

---

<a id="epic-6"></a>
## Epic 6: 추가 발견 사항 및 부가 개선

> **심각도**: 🟡 Major ~ 🟢 Minor  
> **분석 과정에서 추가로 발견된 개선 기회**

### Task 6.1: 에러 처리 및 사용자 경험

#### Subtask 6.1.1: 통일된 에러 메시지 포맷

- **변경 내용**:
  - 모든 에러 메시지에 다음 포함:
    - 무엇이 실패했는지 (What)
    - 왜 실패했는지 (Why)
    - 어떻게 해결할 수 있는지 (How)
  - 예시:
    ```
    Error: Cannot initialize project — AGENTS.md already exists at ./AGENTS.md
    Hint: Run 'agentcom init' again to append configuration, or use '--force' to overwrite.
    ```
- **수락 기준**:
  - [x] 모든 사용자 대면 에러에 What/Why/How 포함
  - [x] 일관된 에러 출력 포맷

#### Subtask 6.1.2: `agentcom doctor` 명령

- **변경 내용**:
  - 프로젝트 설정 상태 진단 명령
  - 검진 항목:
    - `.agentcom.json` 존재 및 유효성
    - 프로젝트명 설정 여부
    - 모든 역할의 SKILL.md 존재 여부
    - 통신 그래프 무결성
    - AGENTS.md/CLAUDE.md 존재 및 agentcom 블록 포함 여부
    - 에이전트 프로세스 상태 (`agentcom up` 후)
  - 출력: 체크리스트 형태로 ✅/❌ 표시
- **수락 기준**:
  - [x] 모든 검진 항목에 대해 pass/fail 표시
  - [x] fail 항목에 대한 수정 명령 안내

### Task 6.2: 템플릿 시스템 개선

#### Subtask 6.2.1: 빌트인 템플릿 차별화

- **현재 문제**: `company`와 `oh-my-opencode` 템플릿이 동일한 6개 역할 (이름만 다름)
- **변경 내용**:
  - 각 템플릿의 고유한 특성을 COMMON.md와 SKILL.md에 반영
  - `company`: 기업 환경 맞춤 (코드리뷰 필수, PR 기반 워크플로우)
  - `oh-my-opencode`: opencode 에코시스템 최적화 (MCP 도구, OMX 스킬 연동)
  - `agentcom agents template --list`에 각 템플릿의 차이점 요약 표시
- **수락 기준**:
  - [x] 두 템플릿이 실질적으로 다른 워크플로우 제공
  - [x] `--list`에서 차이점 확인 가능

#### Subtask 6.2.2: 템플릿 내보내기/가져오기

- **변경 내용**:
  - `agentcom agents template export <name> > template.yaml`: 현재 템플릿을 YAML로 내보내기
  - `agentcom agents template import template.yaml`: YAML에서 템플릿 가져오기
  - 팀 간 템플릿 공유 지원
- **수락 기준**:
  - [x] export → import 왕복 시 정보 손실 없음
  - [x] YAML 형식이 사람이 읽고 편집 가능한 수준

### Task 6.3: 에이전트 라이프사이클 개선

#### Subtask 6.3.1: `agentcom status` 출력 강화

- **현재 상태**: 에이전트 수, 메시지 수 표시
- **변경 내용**:
  - 프로젝트명 표시
  - 활성 템플릿 표시
  - 역할별 에이전트 상태 (alive/dead)
  - 미읽 메시지를 역할별로 분류
  - 마지막 통신 시각
- **수락 기준**:
  - [x] `agentcom status`가 프로젝트 전체 상태를 한눈에 파악 가능

---

## 우선순위 및 구현 로드맵

### Phase 1: Critical Fixes (즉시)

| 순서 | Epic | Task | 예상 공수 | 의존성 |
|------|------|------|----------|--------|
| 1 | Epic 1 | Task 1.1 (Append 로직) | 4h | 없음 |
| 2 | Epic 1 | Task 1.3 (에러 메시지) | 1h | Task 1.1 |
| 3 | Epic 2 | Task 2.1.2 (자기참조 제거) | 1h | 없음 |
| 4 | Epic 3 | Task 3.1 (프로젝트명 필수) | 2h | ⚠️ 사용자 작업 중 |

### Phase 2: Core Improvements (1주)

| 순서 | Epic | Task | 예상 공수 | 의존성 |
|------|------|------|----------|--------|
| 5 | Epic 4 | Task 4.1 (간소화 위저드) | 6h | 없음 |
| 6 | Epic 4 | Task 4.1.2 (자동 생성 엔진) | 4h | Task 4.1 |
| 7 | Epic 2 | Task 2.1 (그래프 검증) | 3h | 없음 |
| 8 | Epic 2 | Task 2.2 (통신 섹션 구체화) | 3h | Task 2.1 |
| 9 | Epic 1 | Task 1.2 (에이전트별 분기) | 3h | Task 1.1 |

### Phase 3: Documentation & Polish (2주)

| 순서 | Epic | Task | 예상 공수 | 의존성 |
|------|------|------|----------|--------|
| 10 | Epic 5 | Task 5.1 (루트 SKILL.md) | 4h | 없음 |
| 11 | Epic 5 | Task 5.2 (역할별 SKILL.md) | 8h | Task 5.1 |
| 12 | Epic 5 | Task 5.3 (COMMON.md) | 2h | 없음 |
| 13 | Epic 4 | Task 4.2 (사후 편집) | 4h | Task 4.1 |
| 14 | Epic 4 | Task 4.3 (비대화형 YAML) | 3h | Task 4.1.2 |

### Phase 4: Enhanced UX (3주)

| 순서 | Epic | Task | 예상 공수 | 의존성 |
|------|------|------|----------|--------|
| 15 | Epic 6 | Task 6.1 (에러 처리) | 4h | Phase 1 |
| 16 | Epic 6 | Task 6.2 (템플릿 차별화) | 6h | Phase 2 |
| 17 | Epic 6 | Task 6.3 (status 강화) | 3h | Task 3.1 |
| 18 | Epic 2 | Task 2.3 (verify 명령) | 3h | Task 2.1 |
| 19 | Epic 5 | Task 5.4 (validate 명령) | 3h | Task 5.2 |
| 20 | Epic 1 | Task 1.3.2 (dry-run) | 2h | Task 1.1 |

### 총 예상 공수

| Phase | 공수 | 누적 |
|-------|------|------|
| Phase 1: Critical Fixes | 8h | 8h |
| Phase 2: Core Improvements | 19h | 27h |
| Phase 3: Documentation | 21h | 48h |
| Phase 4: Enhanced UX | 21h | 69h |
| **총합** | **69h** | — |

---

## 부록: 소스코드 참조 인덱스

| 파일 | 라인 | 내용 |
|------|------|------|
| `instruction.go` | 245-247 | `writeInstructionFile()` — append 버그 위치 |
| `init_prompter.go` | 40-51 | `defaultWizardProjectName()` — 빈 프로젝트명 허용 |
| `init_prompter.go` | 275-368 | `runCustomTemplateWizard()` — 47필드 위저드 |
| `agents.go` | 355-381 | `renderRoleSkillContent()` — SKILL.md 렌더링 |
| `agents.go` | 379 | 하드코딩된 에스컬레이션 문구 (자기참조 버그) |
| `agents.go` | 395-535 | `builtInTemplateDefinitions()` — 빌트인 템플릿 정의 |
| `agents.go` | 396-403 | `communicationMap` — 통신 그래프 정의 |
| `init_setup.go` | — | `--force` 플래그 적용 범위 (`.agentcom.json`만) |
| `template_store.go` | — | 커스텀 템플릿 영속화 |
| `template_definition.go` | — | `TemplateDefinition`, `TemplateRole` 구조체 |

---

> **문서 끝** — 이 개선 제안서는 agentcom v0.1.6 소스코드 전수 분석과 db workspace 생성 출력물 검증을 기반으로 작성되었습니다. 모든 발견 사항은 구체적인 소스코드 라인 참조로 뒷받침됩니다.

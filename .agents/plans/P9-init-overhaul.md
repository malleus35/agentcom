# P9 PRD — `agentcom init` 커맨드 대규모 개편

## Goal

`agentcom init`을 **원스톱 온보딩 허브**로 재설계한다.
사용자가 `agentcom init` 한 번으로 환경 초기화, agent별 instruction 파일 생성, 프로젝트 템플릿 스캐폴딩까지 모두 마칠 수 있어야 한다.

## Motivation

1. 현재 `--setup`으로 분리된 wizard가 기본 UX여야 한다 — 대부분의 사용자가 인터랙티브 환경에서 init을 실행한다.
2. `--agents-md`는 단일 `AGENTS.md`만 생성하지만, 실제로는 각 agent tool이 고유한 instruction 파일(CLAUDE.md, .cursorrules 등)을 사용한다.
3. `--template`은 built-in 템플릿 선택만 가능하고 사용자 정의 템플릿 생성을 지원하지 않는다.
4. 이 세 가지를 하나의 통합 wizard 플로우로 합쳐야 한다.

## Scope

### In-Scope

- `agentcom init` 기본 동작을 wizard UI로 변경
- `--setup` 플래그 제거 (breaking change)
- `--batch` / `--non-interactive` 플래그 추가 (CI/스크립트용)
- `--agents-md` 플래그를 agent별 instruction 파일 생성으로 확장
- `--template` 플래그에 사용자 정의 템플릿 생성 wizard 추가
- 통합 wizard 플로우 설계 및 구현
- 기존 테스트 업데이트 및 신규 테스트 추가

### Non-Goals

- full-screen TUI dashboard (wizard 범위만)
- remote template registry (로컬 저장소만)
- agent 자동 감지 (사용자가 명시적으로 선택)
- MCP 서버 변경

---

## Agent Instruction File 매핑

각 AI coding agent tool은 프로젝트 루트에서 고유한 instruction 파일을 읽는다.
`--agents-md`는 사용자가 선택한 agent에 맞는 파일을 생성해야 한다.

> **조사 기준일**: 2026-03-15 | 출처: 공식 문서, GitHub, 커뮤니티 검증

### 핵심 인사이트

1. **AGENTS.md가 사실상 크로스 툴 표준** — Codex, OpenCode, Devin, Amp, Goose, Gemini, Copilot, Cursor, Windsurf, Roo Code, Kilo Code, Augment, Aider 등 15+ agent가 네이티브로 읽음 (AAIF 표준, 60,000+ 저장소 채택)
2. **고유 파일이 있는 agent도 AGENTS.md를 함께 읽는 경우가 많음** — Claude(예정), Gemini, Augment 등
3. **단일파일 → 디렉토리 마이그레이션 트렌드** — Cursor, Windsurf, Cline, Roo Code, Augment 등

### Tier 1 — 고유 Instruction File 보유 Agent (검증 완료)

| Agent | Primary File | Secondary/Legacy | 포맷 | 자동 로드 | 비고 |
|---|---|---|---|---|---|
| Claude Code | `CLAUDE.md` | `~/.claude/CLAUDE.md` (글로벌) | Markdown | ✅ | 계층적: 글로벌→프로젝트→서브디렉토리 |
| Codex CLI | `AGENTS.md` | `~/.codex/AGENTS.md` (글로벌) | Markdown | ✅ | AGENTS.md 표준 원조 |
| Gemini CLI | `GEMINI.md` | `.gemini/GEMINI.md` | Markdown | ✅ | AGENTS.md도 읽음 |
| Cursor | `.cursor/rules/*.mdc` | `.cursorrules` (deprecated) | MDC | ✅ | `.cursorrules`는 deprecated, `.mdc`에 `alwaysApply`, glob 패턴 지원 |
| GitHub Copilot | `.github/copilot-instructions.md` | `.github/instructions/*.instructions.md` | Markdown | ✅ | 조직/저장소/경로별 3단계 |
| Windsurf | `.windsurfrules` | `.windsurf/rules/*.md` (권장) | Markdown | ✅ | `.windsurfrules` 6,000자 제한, 디렉토리 방식 권장 |
| Cline | `.clinerules` | `.clinerules/` (디렉토리, 권장) | Markdown | ✅ | 글로벌: `~/Documents/Cline/Rules/` |
| Roo Code | `.roorules` | `.roo/rules/*.md` (권장) | Markdown | ✅ | 모드별: `.roorules-{modeSlug}` |
| Amazon Q | `.amazonq/rules/*.md` | — | Markdown | ✅ | 다중 파일, 파일명 자유 |
| Augment Code | `.augment/rules/*.md` | `.augment-guidelines` (레거시) | Markdown | ✅ | AGENTS.md, CLAUDE.md도 계층적으로 읽음 |
| Continue | `.continue/rules/*.md` | `~/.continue/rules/` (글로벌) | Markdown | ✅ | Hub Rules도 지원 |
| Kilo Code | `.kilocode/rules/*.md` | `.kilocode/rules-{modeSlug}/` | Markdown | ✅ | v7.0.41에서 `.opencode`→`.kilo` 마이그레이션 |
| Trae | `.trae/project_rules.md` | `.trae/user_rules.md` (글로벌) | Markdown | ✅ | UI "Rules" 탭으로 관리 |
| Goose | `.goosehints` | `AGENTS.md` (저장소 전체) | Markdown | ✅ | 두 가지 모두 지원 |

### Tier 2 — AGENTS.md 표준 채택 Agent

| Agent | Instruction File | 비고 |
|---|---|---|
| OpenCode | `AGENTS.md` | 공식 문서에서 primary로 명시 |
| Amp | `AGENTS.md` | AGENTS.md 표준 공동 창시자 |
| Devin | `AGENTS.md` | `.devin/wiki.json`은 별도 Wiki 시스템 |
| Aider | `CONVENTIONS.md` (관행) | `--read` 플래그 또는 `.aider.conf.yml`의 `read:` 항목으로 지정 |

### Tier 3 — 기타 Agent (범용 AGENTS.md fallback)

나머지 agent(antigravity, clawdbot, factory, mux, neovate 등)는 범용 `AGENTS.md` 또는 해당 agent의 스킬 디렉토리에 instruction 파일을 생성한다.

### 생성 우선순위

```
Priority 1 (필수): AGENTS.md             → 최대 호환성 (15+ agent)
Priority 2 (권장): CLAUDE.md             → Claude Code 전용
Priority 3 (권장): GEMINI.md             → Gemini CLI 전용
Priority 4 (선택): .cursor/rules/*.mdc   → Cursor 전용 (MDC 포맷)
Priority 5 (선택): .github/copilot-instructions.md → Copilot 전용
Priority 6 (선택): .windsurfrules        → Windsurf 전용
Priority 7 (선택): .clinerules           → Cline 전용
Priority 8 (선택): .roorules             → Roo Code 전용
Priority 9 (선택): .amazonq/rules/       → Amazon Q 전용
Priority 10 (선택): .augment/rules/      → Augment 전용
Priority 11 (선택): .continue/rules/     → Continue 전용
Priority 12 (선택): .kilocode/rules/     → Kilo Code 전용
Priority 13 (선택): .trae/project_rules.md → Trae 전용
Priority 14 (선택): .goosehints          → Goose 전용
```

### MEMORY.md 등가물

| Agent | Memory 시스템 | 방식 | 비고 |
|---|---|---|---|
| Claude Code | `~/.claude/projects/<hash>/MEMORY.md` | 자동 (Auto-Memory) | 백그라운드 자동 기록/로드 |
| Augment Code | Augment Memories (내장) | 자동 | Rules로 import 가능 |
| Goose | Memory Extension (MCP 기반) | 플러그인 | 별도 활성화 필요 |
| Devin | Devin Wiki (자동 인덱싱) | 자동 | `.devin/wiki.json`으로 커스터마이즈 |
| Codex CLI | `.agents/MEMORY.md` | 수동 (agentcom 관행) | 커뮤니티 관행 |
| 범용 | `.agentcom/MEMORY.md` | 수동 | agentcom 기본값 |
| 기타 대부분 | 없음 | 수동 AGENTS.md 업데이트 | — |

---

## UX 플로우 설계

### 기본 모드 (Interactive TTY)

```
$ agentcom init

╭─ agentcom setup ──────────────────────────╮
│                                           │
│  Step 1: Environment                      │
│  ├─ Home directory: ~/.agentcom           │
│                                           │
│  Step 2: Agent Tools                      │
│  ├─ Select agents you use:                │
│  │  [x] Claude Code                       │
│  │  [x] Codex CLI                         │
│  │  [ ] Cursor                            │
│  │  [x] GitHub Copilot                    │
│  │  [ ] Windsurf                          │
│  │  ... (show all / search)               │
│                                           │
│  Step 3: Project Instructions             │
│  ├─ Generate instruction files?  [Yes]    │
│  ├─ Generate MEMORY files?       [No]     │
│                                           │
│  Step 4: Template                         │
│  ├─ Select template:                      │
│  │  ( ) None                              │
│  │  ( ) Company                           │
│  │  ( ) Oh-My-OpenCode                    │
│  │  ( ) Create custom template...         │
│                                           │
│  Step 5: Confirm                          │
│  ├─ Review & Apply                        │
│                                           │
╰───────────────────────────────────────────╯
```

### Batch 모드 (Non-Interactive)

```bash
# 기존 동작 유지 (비대화형)
agentcom init --batch
agentcom init --batch --agents-md claude,codex,cursor
agentcom init --batch --template company
```

### 직접 플래그 모드 (단일 기능 실행)

```bash
# instruction 파일만 생성
agentcom init --agents-md claude,codex,github-copilot

# 특정 템플릿만 스캐폴드
agentcom init --template company

# 커스텀 템플릿 생성 wizard
agentcom init --template custom
```

---

## 태스크 분해

### Phase 9-A: 리서치 및 기반 작업

#### P9-01: Agent Instruction File 리서치 및 매핑 정의

##### P9-01-01 Agent별 instruction file 컨벤션 리서치
- Tier 1 agent 10개의 공식 문서/소스 코드에서 instruction file 경로 확인
- Tier 2 agent 6개의 instruction file 경로 확인
- 각 agent의 MEMORY.md 등가물 존재 여부 확인
- 결과를 본 PRD의 매핑 테이블에 반영

##### P9-01-02 Instruction file 매핑 데이터 모델 정의
- `instructionFileDefinition` 구조체 설계
  - `AgentID string`
  - `FileName string` (예: "CLAUDE.md", ".cursorrules")
  - `RelativePath string` (프로젝트 루트 기준)
  - `Format string` ("markdown" | "yaml" | "json" | "mdc")
  - `SupportsMemory bool`
  - `MemoryFileName string`
  - `MemoryRelativePath string`
- `instructionFileDefinitions` 슬라이스 정의 (skill.go의 `skillAgentDefinitions`와 유사)

##### P9-01-03 Instruction file 렌더링 함수 설계
- `renderInstructionContent(agentID string, projectName string) string` — agent별 instruction 파일 내용 생성
- `renderMemoryContent(agentID string) string` — agent별 memory 파일 내용 생성
- agent 공통 섹션(agentcom workflow)과 agent 고유 섹션 분리 설계

##### P9-01-04 Instruction file 매핑 유닛 테스트
- 모든 agent의 instruction file 경로 해석 테스트
- 지원되지 않는 agent 에러 핸들링 테스트
- 렌더링 함수 출력 검증 테스트

---

### Phase 9-B: Wizard 기본 UI 전환

#### P9-02: `--setup` 제거 및 wizard 기본화

##### P9-02-01 `--batch` 플래그 추가
- `init.go`에 `--batch` bool 플래그 추가
- `--batch` 지정 시 기존 비대화형 init 로직 실행
- `--json` 지정 시 자동으로 `--batch` 동작 적용

##### P9-02-02 기본 동작을 wizard로 전환
- `init.go`의 RunE 분기 변경: TTY 감지 시 wizard 실행이 기본
- `--batch`가 없고 TTY인 경우 → wizard
- `--batch`이거나 non-TTY인 경우 → 기존 비대화형
- 기존 `setup` 변수/분기 제거

##### P9-02-03 `--setup` 플래그 제거
- `init.go`에서 `--setup` 플래그 선언 제거
- `--setup` 관련 조건문 제거
- `--accessible`는 독립 플래그로 유지 (wizard 모드에서 자동 적용)

##### P9-02-04 root.go pre-run 우회 로직 정리
- 기존 `--setup` 기반 app 초기화 우회 조건을 wizard 기본 동작에 맞게 변경
- wizard 모드에서는 사용자가 home dir를 선택한 뒤 초기화하도록 유지

##### P9-02-05 기존 init 테스트 업데이트
- `init_test.go` / `cli_test.go`에서 `--setup` 참조 제거
- `--batch` 플래그 테스트 추가
- TTY 감지 기반 wizard 진입 테스트 추가
- `--json` 시 batch 자동 적용 테스트

##### P9-02-06 init_setup.go 리팩터링
- `isInteractiveInput` → `shouldRunWizard(cmd)` 로 통합
- batch 플래그, JSON 모드, TTY 상태를 조합한 단일 판별 함수
- 기존 `newOnboardPrompter` 시그니처 유지 (테스트 주입용)

---

### Phase 9-C: `--agents-md` 오버홀

#### P9-03: Agent별 instruction 파일 생성 시스템

##### P9-03-01 `instructionFileDefinition` 구조체 및 레지스트리 구현
- `internal/cli/instruction.go` 신규 파일 생성
- `instructionFileDefinition` 구조체 정의 (P9-01-02에서 설계한 모델)
- Tier 1/2/3 agent별 instruction file 매핑 슬라이스 구현
- `findInstructionDefinition(agentID) (instructionFileDefinition, bool)` 함수

##### P9-03-02 Instruction file 렌더링 엔진 구현
- `renderInstructionContent(agentID, projectContext)` 구현
- agent 공통 agentcom workflow 섹션
- agent 고유 섹션 (예: Claude → "## Claude Code 특화 지침")
- 포맷별 렌더링 (Markdown, YAML frontmatter, MDC, JSON)

##### P9-03-03 Memory file 렌더링 엔진 구현
- `renderMemoryContent(agentID)` 구현
- agent별 memory 파일 구조 생성
- 기존 `AGENTS.md`의 MEMORY.md 관련 내용 연계

##### P9-03-04 `writeAgentInstructions(projectDir, agentIDs)` 함수 구현
- 선택된 agent 목록을 받아 각 agent의 instruction 파일 생성
- 기존 파일 존재 시 건너뛰기 (또는 `--force` 옵션)
- 생성된 파일 경로 목록 반환
- 에러 시 부분 생성 결과와 함께 반환

##### P9-03-05 `writeAgentMemoryFiles(projectDir, agentIDs)` 함수 구현
- 선택된 agent 중 memory 지원 agent의 memory 파일 생성
- 기존 파일 존재 시 건너뛰기

##### P9-03-06 `--agents-md` 플래그 시맨틱 변경
- 기존: `--agents-md` (bool) → 단일 AGENTS.md 생성
- 변경: `--agents-md <agent1,agent2,...|all>` (string) → agent별 instruction 파일 생성
- `--agents-md` (값 없음) → wizard에서 agent 선택 프롬프트
- `--agents-md all` → 모든 agent의 instruction 파일 생성
- `--agents-md claude,codex,cursor` → 특정 agent만
- 하위 호환: `--agents-md` 단독 사용 시 기존처럼 동작하되 wizard에서 확장

##### P9-03-07 기존 `writeProjectAgentsMD` 함수 리팩터링
- 기존 AGENTS.md 생성 로직을 `writeAgentInstructions`의 codex 타겟으로 통합
- 기존 AGENTS.md 내용을 codex용 instruction template로 이전
- 하위 호환성을 위해 codex 선택 시 기존과 동일한 AGENTS.md 생성 보장

##### P9-03-08 instruction 파일 생성 유닛 테스트
- agent별 올바른 파일 경로 생성 테스트
- agent별 올바른 내용 렌더링 테스트
- 기존 파일 보호 (덮어쓰지 않음) 테스트
- 에러 핸들링 테스트
- `--agents-md` 플래그 파싱 테스트

##### P9-03-09 memory 파일 생성 유닛 테스트
- memory 지원 agent 필터링 테스트
- memory 파일 내용 검증 테스트
- 기존 파일 보호 테스트

---

### Phase 9-D: 커스텀 템플릿 Wizard

#### P9-04: 사용자 정의 템플릿 생성 시스템

##### P9-04-01 커스텀 템플릿 데이터 모델 확장
- `templateDefinition`은 기존 구조 그대로 재사용
- `validTemplates` 맵을 동적 검증으로 변경 (하드코딩 → 런타임 체크)
- 커스텀 템플릿 이름 검증 규칙 정의 (skillNamePattern과 유사)

##### P9-04-02 커스텀 템플릿 저장/로드 로직 구현
- `internal/cli/template_store.go` 신규 파일
- `saveCustomTemplate(projectDir, definition)` — `.agentcom/templates/{name}/template.json`에 저장
- `loadCustomTemplates(projectDir)` — `.agentcom/templates/` 하위의 `template.json` 파일 스캔
- `mergeTemplateDefinitions(builtIn, custom)` — built-in + custom 템플릿 통합

##### P9-04-03 `builtInTemplateDefinitions()` 확장
- 기존 built-in 정의 유지
- `allTemplateDefinitions(projectDir)` 함수 추가: built-in + custom 통합
- `resolveTemplateDefinition(name)` 호출 시 custom도 탐색

##### P9-04-04 커스텀 템플릿 생성 wizard — Step 1: 기본 정보
- 템플릿 이름 입력 (이름 검증 포함)
- 설명 입력
- 참조(reference) 입력 (선택)

##### P9-04-05 커스텀 템플릿 생성 wizard — Step 2: 역할 정의
- 역할 추가 루프 (최소 1개, 최대 N개)
- 각 역할: 이름, 설명, agent 이름, agent 유형 입력
- 각 역할: 책임사항(responsibilities) 입력 (콤마 구분 또는 여러 줄)
- 사전 정의된 역할 프리셋 제공 (frontend, backend, plan, review, architect, design)

##### P9-04-06 커스텀 템플릿 생성 wizard — Step 3: 커뮤니케이션 맵
- 각 역할 간 소통 대상 설정
- 기본값: 모든 역할이 서로 소통
- 선택적 제한 가능 (특정 역할만 선택)

##### P9-04-07 커스텀 템플릿 생성 wizard — Step 4: 공통 지침
- CommonBody 텍스트 에디터 입력 또는 기본 템플릿 제공
- 기본값: 최소한의 agentcom workflow 지침
- 선택: 기존 built-in 템플릿의 CommonBody를 기반으로 수정

##### P9-04-08 커스텀 템플릿 생성 wizard — Step 5: 확인 및 적용
- 전체 설정 요약 표시
- 확인 후 `template.json` + `COMMON.md` 저장
- 스캐폴드 파일 생성 (기존 `writeTemplateScaffold` 재사용)
- 생성 결과 출력

##### P9-04-09 `--template custom` 플래그 처리 구현
- `--template custom` → 커스텀 템플릿 생성 wizard 진입
- `--template <name>` → 기존 동작 유지 (built-in + custom 모두 탐색)
- `--template` (값 없음) → wizard에서 템플릿 선택 프롬프트 (built-in + custom + "새로 만들기")

##### P9-04-10 커스텀 템플릿 생성 테스트
- 템플릿 이름 검증 테스트
- 템플릿 저장/로드 라운드트립 테스트
- built-in + custom 통합 목록 테스트
- wizard 각 step의 validation 테스트
- 스캐폴드 생성 결과 검증 테스트

##### P9-04-11 커스텀 템플릿 목록/삭제 CLI 추가
- `agentcom agents template --list` — built-in + custom 통합 목록
- `agentcom agents template --delete <name>` — custom 템플릿 삭제 (built-in 삭제 불가)
- 삭제 시 확인 프롬프트

---

### Phase 9-E: 통합 Wizard 플로우

#### P9-05: 통합 init wizard 조립

##### P9-05-01 wizard Result 모델 확장
- `onboard.Result`에 필드 추가:
  - `SelectedAgents []string` — 사용자가 선택한 agent 목록
  - `WriteInstructions bool` — instruction 파일 생성 여부
  - `WriteMemory bool` — memory 파일 생성 여부
  - `CustomTemplate *templateDefinition` — 커스텀 템플릿 생성 시 정의
- `onboard.ApplyReport`에 필드 추가:
  - `InstructionFiles []string` — 생성된 instruction 파일 경로
  - `MemoryFiles []string` — 생성된 memory 파일 경로
  - `CustomTemplatePath string` — 커스텀 템플릿 저장 경로

##### P9-05-02 wizard Validation 규칙 확장
- `Result.Validate()` 확장:
  - `SelectedAgents`가 비어있어도 유효 (instruction 파일 미생성)
  - `WriteInstructions=true`이면 `SelectedAgents` 최소 1개 필요
  - `CustomTemplate != nil`이면 커스텀 템플릿 validation 실행
- 커스텀 템플릿 이름이 built-in과 충돌하지 않는지 검증

##### P9-05-03 HuhPrompter 5-step 플로우 구현
- Step 1: Environment (기존 유지)
  - Home directory 입력
- Step 2: Agent Tools (신규)
  - Multi-select 체크박스로 사용 중인 agent 선택
  - 검색/필터 지원
  - Tier 1 agent 상단 배치
- Step 3: Project Instructions (신규)
  - 선택된 agent의 instruction 파일 생성 여부 확인
  - Memory 파일 생성 여부 확인
- Step 4: Template (기존 확장)
  - 기존 built-in 선택지 유지
  - "Create custom template..." 선택지 추가
  - 커스텀 선택 시 P9-04의 sub-wizard 진입
- Step 5: Confirm (기존 확장)
  - 선택 요약에 agent, instruction, memory, template 정보 포함

##### P9-05-04 initSetupExecutor 확장
- `Apply()` 메서드에 instruction/memory 파일 생성 로직 추가
- 커스텀 템플릿 생성 시 저장 로직 추가
- 결과 report에 모든 생성 파일 포함
- 에러 발생 시 부분 생성 결과 반환

##### P9-05-05 init.go RunE 통합 분기 정리
- wizard 모드: 확장된 HuhPrompter → initSetupExecutor
- batch 모드: `--agents-md`, `--template` 플래그 기반 직접 실행
- JSON 모드: 확장된 결과 구조체로 출력
- 각 플래그의 독립 실행과 wizard 연동 모두 지원

##### P9-05-06 통합 wizard E2E 시나리오 테스트
- wizard 진입 → agent 선택 → instruction 생성 → template 선택 → 완료 전체 흐름
- batch 모드와 wizard 모드의 동일 결과 검증
- 커스텀 템플릿 생성 후 재사용 시나리오
- 기존 파일이 있는 프로젝트에서의 안전한 실행 검증

---

### Phase 9-F: 테스트 및 품질

#### P9-06: 테스트 보강

##### P9-06-01 instruction 파일 통합 테스트
- 여러 agent 조합의 instruction 파일 동시 생성 테스트
- 프로젝트 루트와 상대 경로 일관성 검증
- 기존 skill 파일과의 충돌 없음 검증

##### P9-06-02 커스텀 템플릿 통합 테스트
- 커스텀 템플릿 생성 → 사용 → 삭제 전체 라이프사이클
- built-in과 custom 공존 시나리오
- 잘못된 template.json 파일 핸들링

##### P9-06-03 하위 호환성 테스트
- `agentcom init --batch` 가 기존 `agentcom init` 동작과 동일한지 검증
- `agentcom init --batch --agents-md` (값 없이) → 기존 AGENTS.md 생성
- `agentcom init --batch --template company` → 기존과 동일한 scaffold 생성

##### P9-06-04 E2E init 시나리오 테스트 확장
- `cmd/agentcom/e2e_test.go`에 새로운 init 시나리오 추가
- batch 모드: instruction + template 조합
- wizard 모드: mock prompter 기반 흐름 검증

---

### Phase 9-G: 문서화

#### P9-07: 문서 업데이트

##### P9-07-01 README 업데이트
- Quickstart 섹션에 새로운 wizard 기본 동작 반영
- `--setup` 제거, `--batch` 추가 문서화
- `--agents-md` 새 시맨틱 문서화
- `--template custom` 워크플로우 문서화

##### P9-07-02 README 번역 동기화
- README.ko.md, README.ja.md, README.zh.md 업데이트

##### P9-07-03 MEMORY.md 업데이트
- P9 완료 태스크 기록
- 설계 결정 로그 추가
- 다음 작업 컨텍스트 기록

---

## 의존성 그래프

```
P9-01 (리서치)
  └─> P9-03 (instruction 파일 시스템)
        └─> P9-05 (통합 wizard)

P9-02 (wizard 기본화)
  └─> P9-05 (통합 wizard)

P9-04 (커스텀 템플릿)
  └─> P9-05 (통합 wizard)

P9-05 (통합 wizard)
  └─> P9-06 (테스트)
  └─> P9-07 (문서)
```

### 병렬 작업 가능 구간

- **P9-01** (리서치) + **P9-02** (wizard 기본화): 독립적, 병렬 가능
- **P9-03** (instruction) + **P9-04** (커스텀 템플릿): P9-01 완료 후 병렬 가능
- **P9-06** + **P9-07**: P9-05 완료 후 병렬 가능

---

## 태스크 요약

| Task ID | 설명 | Subtask 수 | 의존성 |
|---|---|---|---|
| P9-01 | Agent instruction file 리서치 및 매핑 정의 | 4 | 없음 |
| P9-02 | `--setup` 제거 및 wizard 기본화 | 6 | 없음 |
| P9-03 | Agent별 instruction 파일 생성 시스템 | 9 | P9-01 |
| P9-04 | 사용자 정의 템플릿 생성 시스템 | 11 | 없음 |
| P9-05 | 통합 init wizard 조립 | 6 | P9-02, P9-03, P9-04 |
| P9-06 | 테스트 보강 | 4 | P9-05 |
| P9-07 | 문서 업데이트 | 3 | P9-05 |
| **합계** | | **43 subtasks** | |

---

## Breaking Changes

1. `--setup` 플래그 제거 → 기존 스크립트에서 `--setup` 사용 중이면 에러
   - **마이그레이션**: `--setup`을 deprecated alias로 유지 후 1 버전 뒤 제거 가능
2. `--agents-md` 시맨틱 변경 → bool에서 string으로
   - **마이그레이션**: 값 없이 `--agents-md` 단독 사용 시 기존 AGENTS.md 생성 유지

## QA

- `go test ./...`
- `go build ./...`
- `golangci-lint run`
- 수동: `AGENTCOM_HOME=$(mktemp -d)/home go run ./cmd/agentcom init`
- 수동: `printf '' | go run ./cmd/agentcom init --batch` (비대화형)
- 수동: `go run ./cmd/agentcom init --batch --agents-md claude,codex`
- 수동: `go run ./cmd/agentcom init --batch --template custom` (에러 확인)

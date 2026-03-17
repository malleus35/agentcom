# Init Template Selected Skill Targets Plan

## Goal

`agentcom init --template ...` 실행 시 template scaffold가 모든 기본 agent(`claude`, `codex`, `gemini`, `opencode`)용 shared/role SKILL을 생성하지 않고, 사용자가 실제로 선택한 agent만 반영하도록 정리한다.

## Current Findings

- onboarding/wizard는 선택 agent를 `onboard.Result.SelectedAgents`로 보존한다 (`internal/onboard/result.go`).
- instruction 파일 생성 경로는 이미 `SelectedAgents`를 사용한다 (`internal/cli/init_setup.go`).
- template scaffold 경로는 선택값을 받지 않고 `writeTemplateScaffold(projectDir, templateName, mode)`만 호출한다 (`internal/cli/init.go`, `internal/cli/init_setup.go`).
- 실제 skill target 결정은 `resolveTemplateSkillTargets()`가 하드코딩된 4개 agent를 반환해서 발생한다 (`internal/cli/skill.go`).
- `writeTemplateScaffold()`와 `previewTemplateScaffold()`가 모두 이 하드코딩 경로를 사용한다 (`internal/cli/agents.go`, `internal/cli/init_setup.go`).

## Scope

### In Scope

- template scaffold 대상 agent 집합을 선택값 기반으로 바꾸기
- wizard/batch/dry-run/force 흐름 모두 동일 규칙 적용
- shared skill + role skill + preview + tests 정합성 맞추기
- 선택값이 없을 때의 fallback 정책 정의

### Out of Scope

- instruction file 렌더링 자체 변경
- template 내용 문구 개편
- 새로운 agent 지원 추가

## Decisions To Lock

- template scaffold의 skill target source는 "현재 init 실행에서 확정된 selected agents"로 통일한다.
- template-only batch 실행처럼 선택값이 없을 때는 명시적 fallback이 필요하다. 추천 기본값은 `claude,codex,gemini,opencode` 유지이되, 사용자 선택이 있으면 그것이 항상 우선한다.
- dry-run과 실제 write가 완전히 같은 target 계산 함수를 공유해야 한다.

## Tasks

### T1. Target Resolution Model 정리

#### T1.1 선택 agent 기반 template target resolver 추가
- `resolveTemplateSkillTargets()`를 대체/확장할 함수 설계
- 입력: scope, selectedAgents, skillName
- 출력: dedupe된 `skillTarget[]`
- alias 정규화 필요 시 기존 agent 정의 재사용

#### T1.2 fallback 정책 명문화
- wizard에서 선택 agent가 있는 경우: 그 목록만 사용
- `--template` 단독 batch 실행: 기본 4개 유지 여부를 helper에 명시
- `--agents-md`와 `--template`를 함께 쓴 경우: 동일 selectedAgents 공유

### T2. Init Flow 배선 변경

#### T2.1 non-interactive init 경로 연결
- `internal/cli/init.go`에서 template scaffold 호출부가 selectedAgents를 넘기도록 시그니처 변경
- `dry-run` preview도 동일 인자를 사용하도록 변경

#### T2.2 wizard init 경로 연결
- `internal/cli/init_setup.go`에서 `onboard.Result.SelectedAgents`를 template scaffold 단계까지 전달
- instruction/memory/template가 서로 다른 agent 집합을 쓰지 않도록 정렬

#### T2.3 helper 시그니처 정리
- `writeTemplateScaffold(...)`
- `previewTemplateScaffold(...)`
- 관련 호출부 (`template_edit.go`, `doctor.go` 등) 영향 범위 점검

### T3. Scaffold Writer / Preview 정합성 수정

#### T3.1 shared skill 생성 범위 제한
- `agentcom/SKILL.md` 생성도 선택된 agent에만 수행
- unselected agent의 `.gemini`/`.agents`/`.opencode` 디렉터리가 생기지 않도록 보장

#### T3.2 role skill 생성 범위 제한
- 각 role별 target 계산에 동일 selectedAgents 적용
- generated paths/preview actions가 실제 write 결과와 일치하도록 유지

#### T3.3 dedupe 및 stable ordering 유지
- 같은 실제 path를 공유하는 agent가 생겨도 marker/append semantics가 깨지지 않도록 확인
- 테스트 비교를 위해 결과 path 정렬 유지

### T4. 회귀 테스트 추가

#### T4.1 wizard 선택 반영 테스트
- `claude`, `opencode`만 선택했을 때 `.claude/...`와 `.opencode/...`만 생기는 케이스 추가
- `.gemini/...`와 `.agents/...`가 생성되지 않는지 검증

#### T4.2 batch template-only fallback 테스트
- `agentcom init --batch --template company`의 기본 동작을 고정
- 선택값이 없을 때 어떤 agent set이 생성되는지 명시적으로 테스트

#### T4.3 dry-run preview 테스트
- preview가 실제 생성 대상과 동일한 agent/path만 노출하는지 검증

#### T4.4 re-init / force 테스트
- append 모드 재실행과 `--force`에서 selected target만 overwrite/update 되는지 검증

### T5. 문서 반영

#### T5.1 README init/template 설명 업데이트
- template scaffold가 "지원 agent 전체"가 아니라 "선택 agent 또는 기본 fallback" 기준임을 명시

#### T5.2 JSON / CLI 출력 기대치 검토
- `generated_files` 예시가 달라지면 관련 문서/테스트 함께 수정

## Acceptance Criteria

- `claude,opencode`만 선택한 init에서 `.gemini/skills/...`와 기타 미선택 agent skill 디렉터리가 생성되지 않는다.
- wizard, batch, dry-run, force 모두 같은 target 집합을 사용한다.
- template scaffold preview 결과와 실제 write 결과가 path 단위로 일치한다.
- template-only batch fallback 동작이 테스트와 문서에 명시된다.

## Verification

- `go test ./internal/cli/... -run 'TestInit.*Template|TestWriteTemplateScaffold|Test.*DryRun.*Template|Test.*Force.*Template' -count=1`
- 필요 시 `agentcom --json init --batch --agents-md claude,opencode --template company --dry-run` 수동 확인

# P8-01 PRD - onboard/setup wizard

## Goal

`agentcom init --setup`로 최소한의 대화형 온보딩을 제공한다. 범위는 onboard/setup 단계만이며, full-screen TUI는 포함하지 않는다.

## Scope

- `huh` 기반 3-step wizard 도입
- 기존 `agentcom init` 비대화형 동작 유지
- 현재 작업 디렉토리에 대한 `AGENTS.md` 생성 여부 선택
- 내장 템플릿(`company`, `oh-my-opencode`) 선택
- 선택한 홈 디렉토리 기준으로 DB/소켓 디렉토리 초기화

## Non-Goals

- first-run 자동 실행
- dashboard / chat / full-screen TUI
- setup resume
- 기존 `init --template`, `init --agents-md` 의미 변경

## UX outline

### Step 1 - Environment

- home directory 입력
- 절대 경로 검증

### Step 2 - Project scaffold

- template 선택: `none`, `company`, `oh-my-opencode`
- `AGENTS.md` 생성 여부 확인

### Step 3 - Confirm

- 선택 내용 요약 표시
- 최종 확인

## Acceptance criteria

- `agentcom init --setup`는 인터랙티브 wizard를 실행한다.
- wizard 완료 후 선택한 home directory에 DB와 sockets 디렉토리가 준비된다.
- template 선택 시 기존 scaffold 로직과 동일한 파일이 생성된다.
- `AGENTS.md` 선택 시 현재 작업 디렉토리에 파일이 생성된다.
- `agentcom init`는 기존처럼 계속 동작한다.
- non-interactive stdin 에서 `--setup`은 명확한 에러로 종료한다.

## Task breakdown

### P8-01-01 문서/계획 동기화

- `.agents/MEMORY.md`에 현재 작업 컨텍스트 추가
- 본 PRD 작성 및 추적 기준 확정

### P8-01-02 onboard 도메인 모델 추가

- `internal/onboard/result.go`
- `Result` / `ApplyReport` 구조 정의
- validation 규칙 추가

### P8-01-03 onboard detection/defaults 추가

- `internal/onboard/detect.go`
- 기본 home dir 계산
- first-run 판별 함수 추가

### P8-01-04 wizard 인터페이스/조립 추가

- `internal/onboard/prompter.go`
- `Prompter`, `Applier`, `Wizard` 정의
- prompter -> validate -> apply 흐름 구성

### P8-01-05 huh prompter 구현

- `internal/onboard/huh_prompter.go`
- `Input`, `Select`, `Confirm` 기반 3-step form 구성
- accessible 모드 지원
- stdin/stdout 주입 지원

### P8-01-06 init --setup executor 구현

- 선택된 home dir 기준 config 생성
- DB open/migrate 수행
- 기존 `writeProjectAgentsMD`, `writeTemplateScaffold` 재사용
- 결과 파일 목록 반환

### P8-01-07 CLI wiring

- `internal/cli/init.go`에 `--setup`, `--accessible` 추가
- `internal/cli/root.go`에서 `init --setup` 시 기존 app 초기화 우회
- non-interactive guard 추가

### P8-01-08 테스트 추가

- `internal/onboard/result_test.go`
- `internal/onboard/detect_test.go`
- `internal/onboard/wizard_test.go`
- `internal/cli/init_test.go` 또는 기존 cli 테스트에 setup 경로 추가

### P8-01-09 문서 업데이트

- README Quickstart / init usage에 `init --setup` 반영
- accessible 및 non-interactive 제약 문서화

## Subtasks

### P8-01-02 subtasks

- P8-01-02-a `Result` 필드 정의
- P8-01-02-b template whitelist 정의
- P8-01-02-c validation 에러 메시지 정리
- P8-01-02-d `ApplyReport` JSON shape 정의

### P8-01-03 subtasks

- P8-01-03-a env 기반 home dir 기본값 계산
- P8-01-03-b `agentcom.db` 존재 여부로 first-run 판별
- P8-01-03-c setup 기본값 생성

### P8-01-05 subtasks

- P8-01-05-a Step 1 group 작성
- P8-01-05-b Step 2 group 작성
- P8-01-05-c Step 3 group 작성
- P8-01-05-d abort/error 매핑

### P8-01-06 subtasks

- P8-01-06-a home dir별 config 생성 helper 작성
- P8-01-06-b DB init helper 작성
- P8-01-06-c optional AGENTS.md 생성 연결
- P8-01-06-d optional template scaffold 연결

### P8-01-07 subtasks

- P8-01-07-a `setup` 플래그 추가
- P8-01-07-b `accessible` 플래그 추가
- P8-01-07-c root pre-run skip 조건 추가
- P8-01-07-d interactive terminal guard 추가

### P8-01-08 subtasks

- P8-01-08-a validation table tests
- P8-01-08-b defaults/detect tests
- P8-01-08-c wizard orchestration tests
- P8-01-08-d init setup command tests

## QA

- `go test ./...`
- `go build ./...`
- manual: `AGENTCOM_HOME=$(mktemp -d)/home go run ./cmd/agentcom init --setup`
- manual: 비대화형 stdin 에서 `printf '' | go run ./cmd/agentcom init --setup` 실패 확인
